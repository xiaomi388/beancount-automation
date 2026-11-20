package dump

import (
	"bytes"
	"strings"
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func ptr[T any](v T) *T { return &v }

func security(id, ticker, typ string) plaid.Security {
	s := plaid.Security{SecurityId: id}
	s.TickerSymbol.Set(ptr(ticker))
	s.Type.Set(ptr(typ))
	return s
}

func invTxn(id, acct, sec, date, typ, sub, name string, qty, amount, price float32) plaid.InvestmentTransaction {
	t := plaid.InvestmentTransaction{
		InvestmentTransactionId: id, AccountId: acct, Date: date, Type: typ, Subtype: sub,
		Name: name, Quantity: qty, Amount: amount, Price: price,
	}
	t.SecurityId.Set(ptr(sec))
	t.IsoCurrencyCode.Set(ptr("USD"))
	return t
}

func holding(acct, sec string, qty, price float32, asOf string) plaid.Holding {
	h := plaid.Holding{AccountId: acct, SecurityId: sec, Quantity: qty, InstitutionPrice: price}
	h.InstitutionPriceAsOf.Set(ptr(asOf))
	return h
}

// stockPlanOwner builds one Item with a stock plan account and a brokerage
// account. Sync copies every Item transaction onto every account, so both
// accounts carry all transactions; dump must keep only their own.
func stockPlanOwner() types.Owner {
	all := map[string]plaid.InvestmentTransaction{}
	for _, t := range []plaid.InvestmentTransaction{
		invTxn("v1", "sp", "goog", "2026-01-28", "cash", "withdrawal", "RESTRICTED STOCK DEPOSIT", 10, 0, 1),
		invTxn("v2", "sp", "goog", "2026-02-27", "cash", "withdrawal", "RESTRICTED STOCK DEPOSIT", 5, 0, 1),
		invTxn("d1", "sp", "goog", "2026-03-16", "cash", "dividend", "DIVIDEND", -12.5, -12.5, 1),
		invTxn("s1", "sp", "goog", "2026-04-01", "sell", "sell", "SALE", -3, -900, 300),
		invTxn("i1", "br", "usd", "2026-02-15", "cash", "interest", "BANK INT", 0, -0.02, 1),
	} {
		all[t.InvestmentTransactionId] = t
	}
	secs := map[string]plaid.Security{
		"goog": security("goog", "GOOG", "equity"),
		"usd":  security("usd", "CUR:USD", "cash"),
	}
	sp := types.InvestmentAccount{
		AccoutBase:   plaid.AccountBase{AccountId: "sp", Name: "Equity Awards", Type: plaid.ACCOUNTTYPE_INVESTMENT},
		Securities:   secs,
		Transactions: all,
		Holdings: []plaid.Holding{
			holding("sp", "goog", 112, 280.5, "2026-11-18"),
			holding("sp", "usd", 912.5, 1, "2026-11-18"),
		},
	}
	sp.AccoutBase.Subtype.Set(ptr(plaid.ACCOUNTSUBTYPE_STOCK_PLAN))
	sp.AccoutBase.Balances.IsoCurrencyCode.Set(ptr("USD"))
	sp.AccoutBase.Balances.Current.Set(ptr(float32(32328.5)))
	br := types.InvestmentAccount{
		AccoutBase:   plaid.AccountBase{AccountId: "br", Name: "Individual", Type: plaid.ACCOUNTTYPE_INVESTMENT},
		Securities:   secs,
		Transactions: all,
		Holdings:     []plaid.Holding{holding("br", "usd", 0.02, 1, "2026-11-18")},
	}
	br.AccoutBase.Subtype.Set(ptr(plaid.ACCOUNTSUBTYPE_BROKERAGE))
	br.AccoutBase.Balances.IsoCurrencyCode.Set(ptr("USD"))
	br.AccoutBase.Balances.Current.Set(ptr(float32(0.02)))
	return types.Owner{
		Name: "Alice",
		InvestmentInstitutions: []types.InvestmentInstitution{{
			InstitutionBase:    types.InstitutionBase{Name: "Schwab"},
			InvestmentAccounts: []types.InvestmentAccount{sp, br},
		}},
	}
}

const testPrices = `date,symbol,price,source
2025-12-31,GOOG,190,yahoo-close
2026-01-28,GOOG,200,yahoo-close
2026-02-27,GOOG,210,yahoo-close
2026-02-27,GOOG,209.5,schwab
`

func renderOwner(t *testing.T, owner types.Owner, pricesCSV string) (string, processResult) {
	t.Helper()
	book, err := parsePriceBook(strings.NewReader(pricesCSV))
	if err != nil {
		t.Fatal(err)
	}
	res, err := processTransactions([]types.Owner{owner}, types.PostprocessConfig{}, book, "2026-11-18")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeTransactions(&buf, res.Transactions, res.Accounts); err != nil {
		t.Fatal(err)
	}
	if err := writeStockPlanDirectives(&buf, res.Prices, res.Balances); err != nil {
		t.Fatal(err)
	}
	return buf.String(), res
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in output:\n%s", w, out)
		}
	}
}

func TestStockPlanVestsHeldAtCost(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), testPrices)
	acct := "Assets:Alice:USD:Schwab:Investment:EquityAwards"
	mustContain(t, out,
		acct+" 10 GOOG {200 USD}",
		"Income:USD:Compensation:RSU:GOOG -2000.00 USD",
		// later row for the same date (manual Schwab FMV) wins
		acct+" 5 GOOG {209.5 USD}",
		`fmv_source:"schwab"`,
		"open "+acct+` "FIFO"`,
	)
}

func TestStockPlanDividendIsIncomeNotShares(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), testPrices)
	mustContain(t, out, "Assets:Alice:USD:Schwab:Investment:EquityAwards 12.50 USD", "Income:USD:Cash:Dividend")
	if strings.Contains(out, "-12.5 GOOG") || strings.Contains(out, "Expenses:USD:Cash:Dividend") {
		t.Errorf("dividend booked as shares or expense:\n%s", out)
	}
}

func TestStockPlanSaleReducesLotsAndBooksGains(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), testPrices)
	mustContain(t, out,
		"Assets:Alice:USD:Schwab:Investment:EquityAwards -3 GOOG {} @ 300 USD",
		"Assets:Alice:USD:Schwab:Investment:EquityAwards 900.00 USD",
		"Income:USD:CapitalGains:GOOG",
	)
}

func TestStockPlanOpeningLotAndAssertions(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), testPrices)
	// held 112, transactions explain 10 + 5 - 3 = 12, so 100 pre-history shares
	mustContain(t, out,
		"2026-01-27 * \"\" \"Opening shares before Plaid history\"",
		`basis:"placeholder"`,
		"Assets:Alice:USD:Schwab:Investment:EquityAwards 100 GOOG {190 USD}",
		"2999-01-01 balance Assets:Alice:USD:Schwab:Investment:EquityAwards 112 GOOG",
		"2999-01-01 balance Assets:Alice:USD:Schwab:Investment:EquityAwards 912.5 USD",
		"2026-11-18 price GOOG 280.5 USD",
	)
}

func TestStockPlanMissingPriceKeepsUnits(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), "date,symbol,price\n")
	mustContain(t, out,
		"Assets:Alice:USD:Schwab:Investment:EquityAwards 10 GOOG\n",
		"Income:USD:Compensation:RSU:GOOG -10 GOOG",
		`fmv:"missing"`,
		`basis:"unknown"`,
	)
}

func TestInvestmentTxnsStayOnTheirOwnAccount(t *testing.T) {
	out, _ := renderOwner(t, stockPlanOwner(), testPrices)
	// the brokerage account must not receive the stock plan's dividend or vests
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Investment:Individual") && (strings.Contains(line, "12.5") || strings.Contains(line, "GOOG")) {
			t.Errorf("stock plan txn leaked onto brokerage account: %q", line)
		}
	}
	if strings.Count(out, `"RESTRICTEDSTOCKDEPOSIT"`) != 2 {
		t.Errorf("expected each vest once, got:\n%s", out)
	}
}

func TestReconciledAccountGetsNoPad(t *testing.T) {
	acct := Account{Type: "Assets", Owner: "Alice", Country: "USD", Institution: "Bank", PlaidAccountType: "Depository", Name: "Checking", Balance: 30}
	other := Account{Type: "Assets", Owner: "Alice", Country: "USD", Institution: "Bank", PlaidAccountType: "Depository", Name: "Savings", Balance: 99}
	income := Account{Type: "Income", Country: "USD", Category: []string{"Salary"}}
	accounts := map[string]Account{acct.ToString(): acct, other.ToString(): other, income.ToString(): income}
	txns := []BeancountTransaction{
		{FromAccount: income, ToAccount: acct, Amount: 50, Unit: "USD"},
		{FromAccount: acct, ToAccount: other, Amount: 20, Unit: "USD"},
	}
	markReconciledAccounts(accounts, txns)
	if !accounts[acct.ToString()].NoPad {
		t.Error("checking reconciles (50 - 20 = 30) but still pads")
	}
	if accounts[other.ToString()].NoPad {
		t.Error("savings does not reconcile (20 != 99) but pad was dropped")
	}
	var buf bytes.Buffer
	if err := writeTransactions(&buf, nil, accounts); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "pad "+acct.ToString()) || !strings.Contains(buf.String(), "pad "+other.ToString()) {
		t.Errorf("pad directives wrong:\n%s", buf.String())
	}
}

func TestInvestmentCategoryIsSanitized(t *testing.T) {
	acct := types.InvestmentAccount{}
	acct.AccoutBase.Balances.IsoCurrencyCode.Set(ptr("USD"))
	got := investTxnToChangeAccount(acct, plaid.InvestmentTransaction{Type: "fee", Subtype: "miscellaneous fee", Amount: 1.25})
	if !strings.HasSuffix(got.ToString(), ":Fee:MiscellaneousFee") {
		t.Fatalf("got %q", got.ToString())
	}
}

func TestVestIncomeUsesLedgerUnits(t *testing.T) {
	// 8.399 * 339.01 = 2847.34499; float32 noise made it round to .35.
	if got := fmtUSD(-unitsValue(8.399) * 339.01); got != "-2847.34" {
		t.Fatalf("income = %s, want -2847.34", got)
	}
}
