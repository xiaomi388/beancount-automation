package dump

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/holding"
	"github.com/xiaomi388/beancount-automation/pkg/transaction"
)

type Account struct {
	Type             string   `json:"type"`
	Owner            string   `json:"owner"`
	Country          string   `json:"country"`
	Institution      string   `json:"institution"`
	PlaidAccountType string   `json:"plaid_account_type"`
	Name             string   `json:"name"`
	Category         []string `json:"category"`
}

func (a Account) ToString() string {
	if a.Type == "Expenses" || a.Type == "Income" {
		return fmt.Sprintf("%s:%s:%s", a.Type, a.Country, strings.Join(a.Category, ":"))

	} else {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s", a.Type, a.Owner, a.Country, a.Institution, a.PlaidAccountType, a.Name)
	}
}

type ChangeAccount struct {
	Type     string   `json:"type"`
	Country  string   `json:"country"`
	Category []string `json:"category"`
}

func (ca ChangeAccount) ToString() string {
	return fmt.Sprintf("%s:%s:%s", ca.Type, ca.Country, strings.Join(ca.Category, ":"))
}

type BeancountTransaction struct {
	Date        string            `json:"date"`
	Payee       string            `json:"payee"`
	Desc        string            `json:"desc"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	ToAccount   Account           `json:"to_account"`
	FromAccount Account           `json:"from_account"`
	Amount      float32           `json:"amount"`
	Unit        string            `json:"unit"`
}

func txnToChangeAccount(txn transaction.Transaction) Account {
	typ := "Expenses"
	if txn.Transaction.GetAmount() < 0 {
		typ = "Income"
	}

	categories := txn.Transaction.GetCategory()
	for i := range categories {
		categories[i] = string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(categories[i]), nil))
	}

	return Account{
		Type:     typ,
		Country:  txn.Account.Balances.GetIsoCurrencyCode(),
		Category: txn.Transaction.GetCategory(),
	}
}

func txnToBalanceAccount(txn transaction.Transaction) Account {
	plaidAccountType := txn.Account.GetType()
	typ := "Liabilities"
	if plaidAccountType != plaid.ACCOUNTTYPE_CREDIT {
		typ = "Assets"
	}
	name := string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(txn.Account.Name), nil))

	return Account{
		Type:             typ,
		Owner:            txn.Owner,
		Country:          txn.Account.Balances.GetIsoCurrencyCode(),
		Institution:      txn.Institution,
		PlaidAccountType: strings.Title(string(plaidAccountType)),
		Name:             name,
	}
}

func dumpTransactions(cfg *config.Config, w io.Writer) error {
	txnsMap, err := transaction.Load(cfg.TransactionDBPath)
	if err != nil {
		return fmt.Errorf("failed to load transaction from db: %w", err)
	}

	var txns []transaction.Transaction
	for _, txn := range txnsMap {
		txns = append(txns, txn)
	}

	var bcTxns []BeancountTransaction
	for _, txn := range txns {
		re := regexp.MustCompile(`[^a-zA-Z0-9]`)

		ba := txnToBalanceAccount(txn)
		ca := txnToChangeAccount(txn)

		var fa, ta *Account
		if txn.Transaction.Amount > 0 {
			fa = &ba
			ta = &ca
		} else {
			fa = &ca
			ta = &ba
		}
		bcTxn := BeancountTransaction{
			Date:        txn.Transaction.Date,
			Payee:       string(re.ReplaceAll([]byte(txn.Transaction.GetMerchantName()), nil)),
			Desc:        string(re.ReplaceAll([]byte(txn.Transaction.GetName()), nil)),
			FromAccount: *fa,
			ToAccount:   *ta,
			Metadata: map[string]string{
				"id": txn.Transaction.GetTransactionId(),
			},
			Tags:   []string{},
			Unit:   txn.Transaction.GetIsoCurrencyCode(),
			Amount: float32(math.Abs(float64(txn.Transaction.Amount))),
		}
		if txn.Transaction.Amount > 0 {
			bcTxn.Metadata["payer"] = txn.Owner
		}
		bcTxns = append(bcTxns, bcTxn)
	}

	bcTxns, err = modify(txns, bcTxns)
	if err != nil {
		return fmt.Errorf("failed to modify transactions: %w", err)
	}

	accounts := map[string]Account{}
	for _, bcTxn := range bcTxns {
		accounts[bcTxn.FromAccount.ToString()] = bcTxn.FromAccount
		accounts[bcTxn.ToAccount.ToString()] = bcTxn.ToAccount
		if err := template.Must(template.New("transaction").Parse(transactionTemplate)).Execute(w, bcTxn); err != nil {
			return fmt.Errorf("failed to generate transaction: %w", err)
		}
	}

	if err := template.Must(template.New("open-account").Parse(openAccountTemplate)).Execute(w, accounts); err != nil {
		return fmt.Errorf("failed to generate open balance account: %w", err)
	}

	return nil
}

func dumpHoldings(cfg *config.Config, w io.Writer) error {
	holdings, err := holding.Load(cfg.HoldingDBPath)
	if err != nil {
		return fmt.Errorf("failed to load holdings from %q: %w", cfg.HoldingDBPath, err)
	}

	for _, holding := range holdings {
        if holding.Holding.InstitutionPriceAsOf.Get() == nil {
            continue
        }

        if err := template.Must(template.New("holding").Funcs(template.FuncMap{
            "Deref": func(s *string) string { return *s },
            "Replace": func(s string) string { return string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(s), nil)) },
        }).Parse(holdingTemplate)).Execute(w, holding); err != nil {
            return fmt.Errorf("failed to generate holding for %#v: %w", holding, err)
        }

	}

    return nil
}

func Dump() error {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	var buf bytes.Buffer
	w := io.Writer(&buf)

    if err := dumpTransactions(cfg, w); err != nil {
        return fmt.Errorf("failed to dump transactions: %w", err)

    }

    if err := dumpHoldings(cfg, w); err != nil {
        return fmt.Errorf("failed to dump holdings: %w", err)
    }

	os.WriteFile(cfg.DumpPath, buf.Bytes(), 0644)
	fmt.Printf("Successfully generated beancount file: %q.\n", cfg.DumpPath)
	return nil
}
