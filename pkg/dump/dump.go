package dump

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
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
	// Booking is an optional Beancount booking method for the open directive
	// (e.g. "FIFO" for accounts holding lots at cost).
	Booking string `json:"booking,omitempty"`
	// NoPad skips the pad directive when transactions already reconcile to
	// Balance; Beancount reports an unused pad as an error.
	NoPad bool `json:"no_pad,omitempty"`
}

func (a Account) ToString() string {
	if a.Type == "Equity" {
		return "Equity:" + strings.Join(a.Category, ":")
	}
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
	// Postings, when set, are written verbatim instead of the To/From pair.
	// Used for entries with commodities at cost (stock plan accounts).
	Postings []string `json:"postings,omitempty"`
}

func investTxnToChangeAccount(account types.InvestmentAccount, txn plaid.InvestmentTransaction) Account {
	amount := deriveInvestTxnAmount(account, txn)
	typ := "Expenses"
	if amount > 0 {
		typ = "Income"
	}

	return Account{
		Type:     typ,
		Country:  account.AccoutBase.Balances.GetIsoCurrencyCode(),
		Category: sanitizeCategories(strings.Title(txn.Type), strings.Title(txn.Subtype)),
	}
}

func sanitizeCategories(parts ...string) []string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = string(re.ReplaceAll([]byte(p), nil))
		if p == "" {
			p = "Unknown"
		}
		out = append(out, p)
	}
	return out
}

func txnToChangeAccount(account types.TransactionAccount, txn plaid.Transaction) Account {
	typ := "Expenses"
	if txn.GetAmount() < 0 && !isCardRefundOrCredit(account, txn) {
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

// isCardRefundOrCredit reports whether money coming back to a credit card is
// a merchant refund or a statement credit (not a card payment or transfer).
// Those reduce spending in their category instead of counting as income.
func isCardRefundOrCredit(account types.TransactionAccount, txn plaid.Transaction) bool {
	if account.AccoutBase.GetType() != plaid.ACCOUNTTYPE_CREDIT || txn.GetAmount() >= 0 {
		return false
	}
	cats := txn.GetCategory()
	if len(cats) == 0 {
		return true
	}
	switch cats[0] {
	case "Payment", "Transfer", "Interest":
		return false
	}
	return true
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

func txnAccountToBeanCountBalanceAccount(owner types.Owner, inst types.InstitutionBase, account types.TransactionAccount) Account {
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

func investAccountToBeanCountBalanceAccount(owner types.Owner, inst types.InstitutionBase, account types.InvestmentAccount) Account {
	name := string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(account.AccoutBase.Name), nil))
	balanceAccount := Account{
		Type:             "Assets",
		Owner:            owner.Name,
		Country:          account.AccoutBase.Balances.GetIsoCurrencyCode(),
		Institution:      inst.Name,
		PlaidAccountType: strings.Title(string(account.AccoutBase.Type)),
		Name:             name,
		Balance:          account.AccoutBase.Balances.GetCurrent(),
	}

	for _, txn := range accountInvestTxns(account) {
		if leftDateBeforeRightDate(txn.Date, balanceAccount.FirstTransactionDate) {
			balanceAccount.FirstTransactionDate = txn.Date
		}
	}

	return balanceAccount
}

func dumpTransactions(cfg types.Config, owners []types.Owner, w io.Writer) error {
	prices, err := LoadPriceBook(cfg.StockPlan.PricesFile)
	if err != nil {
		return err
	}

	res, err := processTransactions(owners, cfg.Postprocess, prices, time.Now().Format("2006-01-02"))
	if err != nil {
		return err
	}

	if err := writeTransactions(w, res.Transactions, res.Accounts); err != nil {
		return err
	}

	return writeStockPlanDirectives(w, res.Prices, res.Balances)
}

type processResult struct {
	Transactions []BeancountTransaction
	Accounts     map[string]Account
	Prices       []priceDirective
	Balances     []unitBalance
}

func processTransactions(owners []types.Owner, postCfg types.PostprocessConfig, prices PriceBook, today string) (processResult, error) {
	var bcTxns []BeancountTransaction
	var stockPlanTxns []BeancountTransaction
	var priceDirs []priceDirective
	var unitBals []unitBalance
	accounts := make(map[string]Account)

	for _, owner := range owners {
		for _, inst := range owner.TransactionInstitutions {
			for _, account := range inst.TransactionAccounts {
				balanceAccount := txnAccountToBeanCountBalanceAccount(owner, inst.InstitutionBase, account)
				accounts[balanceAccount.ToString()] = balanceAccount
				for _, txn := range account.Transactions {
					changeAccount := txnToChangeAccount(account, txn)
					accounts[changeAccount.ToString()] = changeAccount

					bcTxn := txnToBeancountTransaction(owner, balanceAccount, changeAccount, txn)
					bcTxns = append(bcTxns, bcTxn)
				}
			}
		}

		for _, inst := range owner.InvestmentInstitutions {
			for _, account := range inst.InvestmentAccounts {
				balanceAccount := investAccountToBeanCountBalanceAccount(owner, inst.InstitutionBase, account)
				if isStockPlanAccount(account) {
					balanceAccount.Balance = stockPlanCash(account)
					balanceAccount.Booking = "FIFO"
					accounts[balanceAccount.ToString()] = balanceAccount
					sp := processStockPlanAccount(owner, account, balanceAccount, prices, today)
					stockPlanTxns = append(stockPlanTxns, sp.Transactions...)
					for _, a := range sp.Accounts {
						accounts[a.ToString()] = a
					}
					priceDirs = append(priceDirs, sp.Prices...)
					unitBals = append(unitBals, sp.Balances...)
					continue
				}
				accounts[balanceAccount.ToString()] = balanceAccount
				for _, txn := range accountInvestTxns(account) {
					changeAccount := investTxnToChangeAccount(account, txn)
					accounts[changeAccount.ToString()] = changeAccount

					bcTxn := investTxnToBeancountTransaction(owner, account, balanceAccount, changeAccount, txn)
					bcTxns = append(bcTxns, bcTxn)
				}
			}
		}
	}

	bcTxns = applyPostprocessTransactions(bcTxns, postCfg)

	for _, bcTxn := range bcTxns {
		accounts[bcTxn.FromAccount.ToString()] = bcTxn.FromAccount
		accounts[bcTxn.ToAccount.ToString()] = bcTxn.ToAccount
	}

	// Stock plan entries carry their own postings and skip transfer merging.
	bcTxns = append(bcTxns, stockPlanTxns...)

	markReconciledAccounts(accounts, bcTxns)

	return processResult{
		Transactions: bcTxns,
		Accounts:     accounts,
		Prices:       dedupePrices(priceDirs),
		Balances:     unitBals,
	}, nil
}

// markReconciledAccounts sets NoPad on asset accounts whose USD postings
// already sum to the asserted balance (only To/From style entries count).
func markReconciledAccounts(accounts map[string]Account, txns []BeancountTransaction) {
	sums := map[string]float64{}
	touched := map[string]bool{}
	for _, t := range txns {
		if len(t.Postings) > 0 {
			for name := range accounts {
				for _, p := range t.Postings {
					if strings.HasPrefix(p, name+" ") || p == name {
						touched[name] = true // has cost/unit postings; leave padding alone
					}
				}
			}
			continue
		}
		if t.Unit != "USD" {
			continue
		}
		sums[t.ToAccount.ToString()] += float64(t.Amount)
		sums[t.FromAccount.ToString()] -= float64(t.Amount)
	}
	for name, a := range accounts {
		if a.Type != "Assets" || touched[name] {
			continue
		}
		if s, ok := sums[name]; ok && math.Abs(s-float64(a.Balance)) < 0.005 {
			a.NoPad = true
			accounts[name] = a
		}
	}
}

func dedupePrices(in []priceDirective) []priceDirective {
	seen := map[string]bool{}
	out := make([]priceDirective, 0, len(in))
	for _, p := range in {
		k := p.Date + " " + p.Symbol
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Symbol != out[j].Symbol {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].Date < out[j].Date
	})
	return out
}

func writeStockPlanDirectives(w io.Writer, prices []priceDirective, balances []unitBalance) error {
	if len(prices) == 0 && len(balances) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for _, p := range prices {
		if _, err := fmt.Fprintf(w, "%s price %s %s USD\n", p.Date, p.Symbol, p.Price); err != nil {
			return err
		}
	}
	for _, b := range balances {
		if _, err := fmt.Fprintf(w, "2999-01-01 balance %s %s %s\n", b.Account, b.Amount, b.Symbol); err != nil {
			return err
		}
	}
	return nil
}

func deriveInvestTxnAmount(account types.InvestmentAccount, txn plaid.InvestmentTransaction) float32 {
	amount := txn.Amount
	if amount == 0 {
		amount = txn.Price * txn.Quantity
		if amount == 0 {
			amount = txn.Quantity * (*account.Securities[*txn.SecurityId.Get()].ClosePrice.Get())
		}
	}

	return amount
}

func investTxnToBeancountTransaction(owner types.Owner, investAccount types.InvestmentAccount, balanceAccount, changeAccount Account, txn plaid.InvestmentTransaction) BeancountTransaction {
	amount := deriveInvestTxnAmount(investAccount, txn)
	var fa, ta *Account
	if amount < 0 {
		fa = &balanceAccount
		ta = &changeAccount
	} else {
		fa = &changeAccount
		ta = &balanceAccount
	}

	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	bcTxn := BeancountTransaction{
		Date:        txn.Date,
		Desc:        string(re.ReplaceAll([]byte(txn.GetName()), nil)),
		FromAccount: *fa,
		ToAccount:   *ta,
		Metadata: map[string]string{
			"id": txn.GetInvestmentTransactionId(),
		},
		Tags:   []string{},
		Unit:   txn.GetIsoCurrencyCode(),
		Amount: float32(math.Abs(float64(amount))),
	}
	if txn.Amount > 0 {
		bcTxn.Metadata["payer"] = owner.Name
	}

	return bcTxn
}

func txnToBeancountTransaction(owner types.Owner, balanceAccount, changeAccount Account, txn plaid.Transaction) BeancountTransaction {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
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

	return bcTxn
}

func writeTransactions(w io.Writer, bcTxns []BeancountTransaction, accounts map[string]Account) error {
	for _, bcTxn := range bcTxns {
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

					type Data struct {
						Owner       string
						Institution string
						Holding     plaid.Holding
						Security    plaid.Security
					}

					if err := template.Must(template.New("holding").Funcs(template.FuncMap{
						"Deref":   func(s *string) string { return *s },
						"Replace": func(s string) string { return string(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAll([]byte(s), nil)) },
					}).Parse(holdingTemplate)).Execute(w, Data{
						Owner:       owner.Name,
						Institution: inst.InstitutionBase.Name,
						Holding:     holding,
						Security:    account.Securities[holding.SecurityId],
					}); err != nil {
						return fmt.Errorf("failed to generate holding for %#v: %w", holding, err)
					}

				}
			}
		}
	}

	return nil
}

func Dump() error {
	config, err := persistence.LoadConfig(persistence.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := persistence.NewStore(config.Storage)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	defer store.Close()

	owners, err := store.LoadOwners()
	if err != nil {
		return fmt.Errorf("failed to load owners: %w", err)
	}

	var buf bytes.Buffer
	w := io.Writer(&buf)

	if err := dumpTransactions(config, owners, w); err != nil {
		return fmt.Errorf("failed to dump transactions: %w", err)
	}

	// if err := dumpHoldings(owners, w); err != nil {
	// 	return fmt.Errorf("failed to dump holdings: %w", err)
	// }

	if err := os.WriteFile(persistence.DefaultBeancountPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write beancount file: %w", err)
	}

	fmt.Printf("Successfully generated beancount file: %q.\n", persistence.DefaultBeancountPath)
	return nil
}
