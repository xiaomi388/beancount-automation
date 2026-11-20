package dump

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

// Stock plan (RSU) accounts hold real shares. Plaid reports a vest as a
// "RESTRICTED STOCK DEPOSIT" with quantity = shares, price 1 and amount 0, so
// the generic USD-only investment path writes the share count as dollars.
// This file renders those accounts with shares as a commodity held at cost.

const shareEpsilon = 1e-3

var symbolSanitizer = regexp.MustCompile(`[^A-Z0-9._-]`)

type priceDirective struct {
	Date   string
	Symbol string
	Price  string
}

type unitBalance struct {
	Account string
	Amount  string
	Symbol  string
}

type stockPlanOutput struct {
	Transactions []BeancountTransaction
	Accounts     []Account
	Prices       []priceDirective
	Balances     []unitBalance
}

func isStockPlanAccount(account types.InvestmentAccount) bool {
	return account.AccoutBase.GetSubtype() == plaid.ACCOUNTSUBTYPE_STOCK_PLAN
}

// accountInvestTxns returns only the account's own transactions. Sync stores
// every transaction of an Item on every account of that Item.
func accountInvestTxns(account types.InvestmentAccount) []plaid.InvestmentTransaction {
	out := make([]plaid.InvestmentTransaction, 0, len(account.Transactions))
	for _, t := range account.Transactions {
		if t.AccountId == account.AccoutBase.AccountId {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].InvestmentTransactionId < out[j].InvestmentTransactionId
	})
	return out
}

func isCashSecurity(sec plaid.Security) bool {
	if sec.Type.Get() != nil && *sec.Type.Get() == "cash" {
		return true
	}
	if sec.IsCashEquivalent.Get() != nil && *sec.IsCashEquivalent.Get() {
		return true
	}
	return strings.HasPrefix(sec.GetTickerSymbol(), "CUR:")
}

// shareSymbol returns a Beancount commodity for a non-cash security.
func shareSymbol(account types.InvestmentAccount, securityID *string) (string, bool) {
	if securityID == nil {
		return "", false
	}
	sec, ok := account.Securities[*securityID]
	if !ok || isCashSecurity(sec) {
		return "", false
	}
	sym := symbolSanitizer.ReplaceAllString(strings.ToUpper(sec.GetTickerSymbol()), "")
	if sym == "" || sym[0] < 'A' || sym[0] > 'Z' {
		return "", false
	}
	if len(sym) > 24 {
		sym = sym[:24]
	}
	return sym, true
}

func fmtUnits(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

// unitsValue returns the quantity as written in the ledger. float64(float32)
// carries binary noise (8.399 -> 8.39900016), which can flip a half-cent
// rounding and leave the vest posting 0.005 USD out of balance.
func unitsValue(v float32) float64 {
	f, _ := strconv.ParseFloat(fmtUnits(v), 64)
	return f
}

func fmtPrice(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func fmtUSD(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64)
}

func categoryAccount(typ, country string, parts ...string) Account {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	cat := make([]string, 0, len(parts))
	for _, p := range parts {
		p = string(re.ReplaceAll([]byte(strings.Title(p)), nil))
		if p == "" {
			p = "Unknown"
		}
		cat = append(cat, p)
	}
	return Account{Type: typ, Country: country, Category: cat}
}

func dayBefore(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

// processStockPlanAccount renders one stock plan account. balanceAccount is the
// Assets account; its Balance must already hold the cash balance only.
func processStockPlanAccount(owner types.Owner, account types.InvestmentAccount, balanceAccount Account, prices PriceBook, today string) stockPlanOutput {
	var out stockPlanOutput
	acct := balanceAccount.ToString()
	ccy := account.AccoutBase.Balances.GetIsoCurrencyCode()
	if ccy == "" {
		ccy = "USD"
	}

	txns := accountInvestTxns(account)
	unitsFromTxns := map[string]float32{}
	usedSymbols := map[string]bool{}

	addAccount := func(a Account) { out.Accounts = append(out.Accounts, a) }

	for _, t := range txns {
		meta := map[string]string{"id": t.InvestmentTransactionId}
		re := regexp.MustCompile(`[^a-zA-Z0-9]`)
		bc := BeancountTransaction{
			Date:     t.Date,
			Desc:     string(re.ReplaceAll([]byte(t.Name), nil)),
			Metadata: meta,
			Tags:     []string{},
			Unit:     ccy,
		}

		sym, isShare := shareSymbol(account, t.SecurityId.Get())
		// Plaid puts the dollar amount of a dividend or fee in Quantity as well,
		// so cash and fee types are cash movements even with a share security.
		cashType := strings.EqualFold(t.Type, "cash") || strings.EqualFold(t.Type, "fee")
		switch {
		case isShare && t.Quantity > 0 && t.Amount == 0:
			// Vest / share deposit without cash: income at fair market value.
			usedSymbols[sym] = true
			unitsFromTxns[sym] += t.Quantity
			income := categoryAccount("Income", ccy, "Compensation", "RSU", sym)
			addAccount(income)
			if p, ok := prices.OnOrBefore(sym, t.Date); ok {
				meta["fmv_date"] = p.Date
				meta["fmv_source"] = sourceOr(p.Source, "prices file")
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s {%s %s}", acct, fmtUnits(t.Quantity), sym, fmtPrice(p.Price), ccy),
					fmt.Sprintf("%s %s %s", income.ToString(), fmtUSD(-unitsValue(t.Quantity)*p.Price), ccy),
				}
			} else {
				meta["fmv"] = "missing"
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s", acct, fmtUnits(t.Quantity), sym),
					fmt.Sprintf("%s %s %s", income.ToString(), fmtUnits(-t.Quantity), sym),
				}
			}
		case cashType && t.Amount != 0:
			bc.Postings = cashPostings(acct, ccy, t, &out)
		case isShare && t.Quantity < 0 && strings.EqualFold(t.Type, "sell"):
			usedSymbols[sym] = true
			unitsFromTxns[sym] += t.Quantity
			gains := categoryAccount("Income", ccy, "CapitalGains", sym)
			addAccount(gains)
			bc.Postings = []string{
				fmt.Sprintf("%s %s %s {} @ %s %s", acct, fmtUnits(t.Quantity), sym, fmtUnits(t.Price), ccy),
				fmt.Sprintf("%s %s %s", acct, fmtUSD(-float64(t.Amount)), ccy),
			}
			if fee := t.Fees.Get(); fee != nil && *fee != 0 {
				fees := categoryAccount("Expenses", ccy, "Fees", "Brokerage")
				addAccount(fees)
				bc.Postings = append(bc.Postings, fmt.Sprintf("%s %s %s", fees.ToString(), fmtUSD(float64(*fee)), ccy))
			}
			bc.Postings = append(bc.Postings, gains.ToString())
		case isShare && t.Quantity != 0:
			// Buys and share transfers in or out.
			usedSymbols[sym] = true
			unitsFromTxns[sym] += t.Quantity
			if t.Quantity > 0 && t.Amount != 0 {
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s {%s %s}", acct, fmtUnits(t.Quantity), sym, fmtUnits(t.Price), ccy),
					fmt.Sprintf("%s %s %s", acct, fmtUSD(-float64(t.Amount)), ccy),
				}
				break
			}
			transfers := Account{Type: "Equity", Category: []string{"Transfers", "Shares"}}
			addAccount(transfers)
			if t.Quantity < 0 {
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s {}", acct, fmtUnits(t.Quantity), sym),
					transfers.ToString(),
				}
			} else {
				cost := float64(t.Price)
				if p, ok := prices.OnOrBefore(sym, t.Date); ok {
					cost = p.Price
				}
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s {%s %s}", acct, fmtUnits(t.Quantity), sym, fmtPrice(cost), ccy),
					transfers.ToString(),
				}
			}
		case t.Amount != 0:
			bc.Postings = cashPostings(acct, ccy, t, &out)
		default:
			continue
		}
		if t.Amount < 0 || (isShare && t.Quantity > 0) {
			meta["payer"] = owner.Name
		}
		out.Transactions = append(out.Transactions, bc)
	}

	// Shares held now vs shares seen in transactions: the rest vested before
	// Plaid's history window and is opened as a placeholder lot.
	held := map[string]float32{}
	for _, h := range account.Holdings {
		sym, ok := shareSymbol(account, &h.SecurityId)
		if !ok {
			continue
		}
		held[sym] += h.Quantity
		usedSymbols[sym] = true
		if d := h.InstitutionPriceAsOf.Get(); d != nil && h.InstitutionPrice > 0 {
			out.Prices = append(out.Prices, priceDirective{Date: *d, Symbol: sym, Price: fmtUnits(h.InstitutionPrice)})
		}
	}

	openDate := today
	if len(txns) > 0 {
		openDate = dayBefore(txns[0].Date)
	}
	syms := make([]string, 0, len(held))
	for s := range held {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	for _, sym := range syms {
		qty := held[sym]
		missing := qty - unitsFromTxns[sym]
		if math.Abs(float64(missing)) > shareEpsilon {
			bc := BeancountTransaction{
				Date:     openDate,
				Desc:     "Opening shares before Plaid history",
				Metadata: map[string]string{},
				Tags:     []string{},
				Unit:     ccy,
			}
			if p, ok := prices.OnOrBefore(sym, openDate); ok && missing > 0 {
				bc.Metadata["basis"] = "placeholder"
				bc.Metadata["fmv_date"] = p.Date
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s {%s %s}", acct, fmtUnits(missing), sym, fmtPrice(p.Price), ccy),
					"Equity:OpenBalance",
				}
			} else {
				bc.Metadata["basis"] = "unknown"
				bc.Postings = []string{
					fmt.Sprintf("%s %s %s", acct, fmtUnits(missing), sym),
					fmt.Sprintf("Equity:OpenBalance %s %s", fmtUnits(-missing), sym),
				}
			}
			out.Transactions = append([]BeancountTransaction{bc}, out.Transactions...)
		}
		out.Balances = append(out.Balances, unitBalance{Account: acct, Amount: fmtUnits(qty), Symbol: sym})
	}

	for sym := range usedSymbols {
		for _, p := range prices[sym] {
			out.Prices = append(out.Prices, priceDirective{Date: p.Date, Symbol: sym, Price: fmtPrice(p.Price)})
		}
	}

	return out
}

// cashPostings books a cash movement. Plaid amount > 0 means cash left the
// account, < 0 means cash came in (dividends, interest, sale proceeds).
func cashPostings(acct, ccy string, t plaid.InvestmentTransaction, out *stockPlanOutput) []string {
	delta := -float64(t.Amount)
	typ := "Expenses"
	if delta > 0 {
		typ = "Income"
	}
	other := categoryAccount(typ, ccy, t.Type, t.Subtype)
	out.Accounts = append(out.Accounts, other)
	return []string{
		fmt.Sprintf("%s %s %s", acct, fmtUSD(delta), ccy),
		other.ToString(),
	}
}

func sourceOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// stockPlanCash returns the cash held in a stock plan account (the USD balance
// to assert), excluding the market value of shares.
func stockPlanCash(account types.InvestmentAccount) float32 {
	var cash float32
	for _, h := range account.Holdings {
		sec, ok := account.Securities[h.SecurityId]
		if ok && isCashSecurity(sec) {
			cash += h.Quantity * h.InstitutionPrice
		}
	}
	return cash
}
