package asset

import "github.com/plaid/plaid-go/plaid"

type Asset struct {
	Owners []Owner `json:"owners"`
}

type Owner struct {
	Name        string        `json:"name"`
	Institution []Institution `json:"institutions"`
}

type Institution struct {
	Name     string             `json:"name"`
	Accounts []Account `json:"accounts"`
}

type Account struct {
	plaid.AccountBase `json:"account_base"`

	Holding                map[string]Holding               `json:"holding"`
	Transactions           map[string]Transaction           `json:"transactions"`
	InvestmentTransactions map[string]InvestmentTransaction `json:"investment_transactions"`
}

type Transaction struct {
	plaid.Transaction `json:"transaction"`
}

type InvestmentTransaction struct {
	plaid.InvestmentTransaction `json:"investment_transaction"`
}

type Holding struct {
	plaid.Holding `json:"holding"`

	Security plaid.Security `json:"security"`
}
