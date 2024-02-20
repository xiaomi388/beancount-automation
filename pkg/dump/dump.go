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
	"time"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/persistence"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

type Account struct {
	Type                 string   `json:"type"`
	Owner                string   `json:"owner"`
	Country              string   `json:"country"`
	Institution          string   `json:"institution"`
	PlaidAccountType     string   `json:"plaid_account_type"`
	Name                 string   `json:"name"`
	Category             []string `json:"category"`
	Balance              float32  `json:"balance"`
	FirstTransactionDate string   `json:"first_transaction_date"`
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

func txnToChangeAccount(owner types.Owner, inst types.InstitutionBase, account types.TransactionAccount, txn plaid.Transaction) Account {
	typ := "Expenses"
	if txn.GetAmount() < 0 {
		typ = "Income"
	}

	categories := txn.GetCategory()
	for i := range categories {
		categories[i] = string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(categories[i]), nil))
	}

	if len(categories) == 0 {
		categories = []string{"Unknown"}
	}

	return Account{
		Type:     typ,
		Country:  account.AccoutBase.Balances.GetIsoCurrencyCode(),
		Category: categories,
	}
}

func leftDateBeforeRightDate(left string, right string) bool {
	leftDate, err := time.Parse("2006-01-02", left)
	if err != nil {
		return false
	}

	rightDate, err := time.Parse("2006-01-02", right)
	if err != nil {
		return true
	}

	return leftDate.Before(rightDate)
}

func accountToBeanCountBalanceAccount(owner types.Owner, inst types.InstitutionBase, account types.TransactionAccount) Account {
	plaidAccountType := account.AccoutBase.GetType()
	typ := "Liabilities"
	if plaidAccountType != plaid.ACCOUNTTYPE_CREDIT {
		typ = "Assets"
	}
	name := string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(account.AccoutBase.Name), nil))
	balanceAccount := Account{
		Type:             typ,
		Owner:            owner.Name,
		Country:          account.AccoutBase.Balances.GetIsoCurrencyCode(),
		Institution:      inst.Name,
		PlaidAccountType: strings.Title(string(plaidAccountType)),
		Name:             name,
		Balance:          account.AccoutBase.Balances.GetAvailable(),
	}

	for _, txn := range account.Transactions {
		if leftDateBeforeRightDate(txn.Date, balanceAccount.FirstTransactionDate) {
			balanceAccount.FirstTransactionDate = txn.Date
		}
	}

	return balanceAccount
}

func dumpTransactions(owners []types.Owner, w io.Writer) error {
	var bcTxns []BeancountTransaction
	for _, owner := range owners {
		for _, inst := range owner.TransactionInstitutions {
			for _, account := range inst.TransactionAccounts {
				balanceAccount := accountToBeanCountBalanceAccount(owner, inst.InstitutionBase, account)
				for _, txn := range account.Transactions {
					re := regexp.MustCompile(`[^a-zA-Z0-9]`)

					changeAccount := txnToChangeAccount(owner, inst.InstitutionBase, account, txn)
					var fa, ta *Account
					if txn.Amount > 0 {
						fa = &balanceAccount
						ta = &changeAccount
					} else {
						fa = &changeAccount
						ta = &balanceAccount
					}

					bcTxn := BeancountTransaction{
						Date:        txn.Date,
						Payee:       string(re.ReplaceAll([]byte(txn.GetMerchantName()), nil)),
						Desc:        string(re.ReplaceAll([]byte(txn.GetName()), nil)),
						FromAccount: *fa,
						ToAccount:   *ta,
						Metadata: map[string]string{
							"id": txn.GetTransactionId(),
						},
						Tags:   []string{},
						Unit:   txn.GetIsoCurrencyCode(),
						Amount: float32(math.Abs(float64(txn.Amount))),
					}
					if txn.Amount > 0 {
						bcTxn.Metadata["payer"] = owner.Name
					}
					bcTxns = append(bcTxns, bcTxn)
				}
			}
		}
	}

	//bcTxns, err := modify(txns, bcTxns)
	//if err != nil {
	//	return fmt.Errorf("failed to modify transactions: %w", err)
	//}

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

func dumpHoldings(owners []types.Owner, w io.Writer) error {
	for _, owner := range owners {
		for _, inst := range owner.InvestmentInstitutions {
			for _, account := range inst.InvestmentAccounts {
				holdings := account.Holdings
				for _, holding := range holdings {
					if holding.InstitutionPriceAsOf.Get() == nil {
						continue
					}

					if err := template.Must(template.New("holding").Funcs(template.FuncMap{
						"Deref":   func(s *string) string { return *s },
						"Replace": func(s string) string { return string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(s), nil)) },
					}).Parse(holdingTemplate)).Execute(w, holding); err != nil {
						return fmt.Errorf("failed to generate holding for %#v: %w", holding, err)
					}

				}
			}
		}
	}

	return nil
}

func Dump() error {
	owners, err := persistence.LoadOwners(persistence.DefaultOwnerPath)
	if err != nil {
		return fmt.Errorf("failed to load owners: %w", err)
	}

	var buf bytes.Buffer
	w := io.Writer(&buf)

	if err := dumpTransactions(owners, w); err != nil {
		return fmt.Errorf("failed to dump transactions: %w", err)

	}

	if err := dumpHoldings(owners, w); err != nil {
		return fmt.Errorf("failed to dump holdings: %w", err)
	}

	os.WriteFile(persistence.DefaultBeancountPath, buf.Bytes(), 0644)
	fmt.Printf("Successfully generated beancount file: %q.\n", persistence.DefaultBeancountPath)
	return nil
}
