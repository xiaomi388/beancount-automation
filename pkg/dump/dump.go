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
	"github.com/xiaomi388/beancount-automation/pkg/transaction"
)

// TODO: make gen folder configurable
const genPath = "./plaid_gen.beancount"

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
	Date        string   `json:"date"`
	Payee       string   `json:"payee"`
	Desc        string   `json:"desc"`
	Tags        []string `json:"tags"`
	ToAccount   Account  `json:"to_account"`
	FromAccount Account  `json:"from_account"`
	Amount      float32  `json:"amount"`
	Unit        string   `json:"unit"`
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

func Dump() error {
	txnsMap, err := transaction.Load(transaction.DBPath)
	if err != nil {
		return fmt.Errorf("failed to load transaction from db: %w", err)
	}

    var txns []transaction.Transaction
    for _, txn := range txnsMap {
        txns = append(txns, txn)
    }

	var buf bytes.Buffer
	w := io.Writer(&buf)

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
		bcTxns = append(bcTxns, BeancountTransaction{
			Date:        txn.Transaction.Date,
			Payee:       string(re.ReplaceAll([]byte(txn.Transaction.GetMerchantName()), nil)),
			Desc:        string(re.ReplaceAll([]byte(txn.Transaction.GetName()), nil)),
			FromAccount: *fa,
			ToAccount:   *ta,
			Unit:        txn.Transaction.GetIsoCurrencyCode(),
			Amount:      float32(math.Abs(float64(txn.Transaction.Amount))),
			Tags:        []string{"owner-" + txn.Owner},
		})
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

	os.WriteFile(genPath, buf.Bytes(), 0644)
	return nil
}
