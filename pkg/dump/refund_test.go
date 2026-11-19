package dump

import (
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func txnAccount(typ plaid.AccountType) types.TransactionAccount {
	a := types.TransactionAccount{AccoutBase: plaid.AccountBase{AccountId: "a", Name: "Card", Type: typ}}
	usd := "USD"
	a.AccoutBase.Balances.IsoCurrencyCode.Set(&usd)
	return a
}

func plaidTxn(amount float32, cats ...string) plaid.Transaction {
	return plaid.Transaction{Amount: amount, Category: cats}
}

func TestCardRefundsAndCreditsReduceExpenses(t *testing.T) {
	card := txnAccount(plaid.ACCOUNTTYPE_CREDIT)
	cases := []struct {
		name string
		txn  plaid.Transaction
		want string
	}{
		{"statement credit", plaidTxn(-97.51, "Travel", "Lodging"), "Expenses:USD:Travel:Lodging"},
		{"merchant refund", plaidTxn(-14.32, "Shops", "Department Stores"), "Expenses:USD:Shops:DepartmentStores"},
		{"no category", plaidTxn(-5), "Expenses:USD:Unknown"},
		{"card payment", plaidTxn(-162.71, "Payment", "Credit Card"), "Income:USD:Payment:CreditCard"},
		{"transfer", plaidTxn(-543.03, "Transfer", "Credit"), "Income:USD:Transfer:Credit"},
		{"purchase", plaidTxn(20.99, "Food and Drink", "Restaurants"), "Expenses:USD:FoodandDrink:Restaurants"},
	}
	for _, c := range cases {
		if got := txnToChangeAccount(card, c.txn).ToString(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestDepositoryInflowStaysIncome(t *testing.T) {
	chk := txnAccount(plaid.ACCOUNTTYPE_DEPOSITORY)
	if got := txnToChangeAccount(chk, plaidTxn(-5208.53, "Transfer", "Payroll")).ToString(); got != "Income:USD:Transfer:Payroll" {
		t.Fatalf("got %s", got)
	}
	if got := txnToChangeAccount(chk, plaidTxn(-20, "Shops")).Type; got != "Income" {
		t.Fatalf("depository refund type changed: %s", got)
	}
}

func TestRuleCanSwitchIncomeExpenseOnly(t *testing.T) {
	liab := Account{Type: "Liabilities", Owner: "A", Country: "USD", Institution: "Amex", PlaidAccountType: "Credit", Name: "Card"}
	inc := Account{Type: "Income", Country: "USD", Category: []string{"Shops", "DiscountStores"}}
	txn := BeancountTransaction{Desc: "DEALSCASHBACK", FromAccount: inc, ToAccount: liab}
	rule := types.KeywordRule{
		Match: types.MatchCriteria{Description: types.TextCriteria{Contains: []string{"CASHBACK"}}},
		Set: types.SetMutations{
			FromAccount: types.AccountMutation{Type: "Expenses"},
			ToAccount:   types.AccountMutation{Type: "Expenses"},
		},
	}
	got := applyCategoryRules([]BeancountTransaction{txn}, []types.KeywordRule{rule})[0]
	if got.FromAccount.Type != "Expenses" {
		t.Errorf("income side not switched: %s", got.FromAccount.ToString())
	}
	if got.ToAccount.Type != "Liabilities" {
		t.Errorf("liability account was changed: %s", got.ToAccount.ToString())
	}
	rule.Set.FromAccount.Type = "Assets"
	if got := applyCategoryRules([]BeancountTransaction{txn}, []types.KeywordRule{rule})[0]; got.FromAccount.Type != "Income" {
		t.Errorf("invalid type applied: %s", got.FromAccount.Type)
	}
}
