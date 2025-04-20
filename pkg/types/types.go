package types

import (
	"github.com/plaid/plaid-go/plaid"
)

type InstitutionType string

const (
	InstitutionTypeTransaction = InstitutionType("transactions")
	InstitutionTypeInvestment  = InstitutionType("investments")
)

type Config struct {
	ClientID    string `yaml:"clientID"`
	Secret      string `yaml:"secret"`
	Environment string `yaml:"environment"`
}

type Owner struct {
	Name string `json:"name"`

	TransactionInstitutions []TransactionInstitution `json:"transactionInstitutions"`
	InvestmentInstitutions  []InvestmentInstitution  `json:"investmentInstitutions"`
}

func (o Owner) TransactionInstitution(name string) (TransactionInstitution, bool) {
	for _, inst := range o.TransactionInstitutions {
		if inst.InstitutionBase.Name == name {
			return inst, true
		}
	}

	return TransactionInstitution{}, false
}

func (o Owner) CreateOrUpdateTransactionInstitution(inst TransactionInstitution) Owner {
	for i := range o.TransactionInstitutions {
		if inst.InstitutionBase.Name == o.TransactionInstitutions[i].InstitutionBase.Name {
			o.TransactionInstitutions[i] = inst
			return o
		}
	}

	o.TransactionInstitutions = append(o.TransactionInstitutions, inst)
	return o
}

func (o Owner) InvestmentInstitution(name string) (InvestmentInstitution, bool) {
	for _, inst := range o.InvestmentInstitutions {
		if inst.InstitutionBase.Name == name {
			return inst, true
		}
	}

	return InvestmentInstitution{}, false
}

func (o Owner) CreateOrUpdateInvestmentInstitution(inst InvestmentInstitution) Owner {
	for i := range o.InvestmentInstitutions {
		if inst.InstitutionBase.Name == o.InvestmentInstitutions[i].InstitutionBase.Name {
			o.InvestmentInstitutions[i] = inst
			return o
		}
	}

	o.InvestmentInstitutions = append(o.InvestmentInstitutions, inst)
	return o
}

func GetOwner(owners []Owner, name string) (Owner, bool) {
	for _, owner := range owners {
		if owner.Name == name {
			return owner, true
		}
	}

	return Owner{}, false
}

func CreateOrUpdateOwner(owners []Owner, owner Owner) []Owner {
	for i := range owners {
		if owners[i].Name == owner.Name {
			owners[i] = owner
			return owners
		}
	}

	owners = append(owners, owner)
	return owners
}

type InstitutionBase struct {
	Name        string `json:"name"`
	AccessToken string `json:"accessToken"`
	Cursor      string `json:"cursor"`
}

type TransactionInstitution struct {
	InstitutionBase     InstitutionBase      `json:"institutionBase"`
	TransactionAccounts []TransactionAccount `json:"transactionAccounts"`
}

func (ti TransactionInstitution) TransactionAccount(id string) (TransactionAccount, bool) {
	for _, account := range ti.TransactionAccounts {
		if account.AccoutBase.AccountId == id {
			return account, true
		}
	}

	return TransactionAccount{}, false
}

func (ti TransactionInstitution) CreateOrUpdateTransactionAccount(account TransactionAccount) TransactionInstitution {
	for i := range ti.TransactionAccounts {
		if ti.TransactionAccounts[i].AccoutBase.AccountId == account.AccoutBase.AccountId {
			ti.TransactionAccounts[i] = account
			return ti
		}
	}

	ti.TransactionAccounts = append(ti.TransactionAccounts, account)
	return ti
}

func (ti TransactionInstitution) CreateOrUpdateTransactionAccountBase(accountBase plaid.AccountBase) TransactionInstitution {
	for i := range ti.TransactionAccounts {
		if ti.TransactionAccounts[i].AccoutBase.AccountId == accountBase.AccountId {
			ti.TransactionAccounts[i].AccoutBase = accountBase
			return ti
		}
	}

	ti.TransactionAccounts = append(ti.TransactionAccounts, TransactionAccount{
		AccoutBase:   accountBase,
		Transactions: map[string]plaid.Transaction{},
	})
	return ti
}

func (ti TransactionInstitution) CreateOrUpdateTransactionAccountBases(accountBases []plaid.AccountBase) TransactionInstitution {
	for _, acb := range accountBases {
		ti = ti.CreateOrUpdateTransactionAccountBase(acb)
	}

	return ti
}

type TransactionAccount struct {
	AccoutBase   plaid.AccountBase            `json:"accountBase"`
	Transactions map[string]plaid.Transaction `json:"transactions"`
}

type InvestmentInstitution struct {
	InstitutionBase    InstitutionBase     `json:"InstitutionBase"`
	InvestmentAccounts []InvestmentAccount `json:"InvestmentAccounts"`
}

type InvestmentAccount struct {
	AccoutBase   plaid.AccountBase                      `json:"accountBase"`
	Holdings     []plaid.Holding                        `json:"holdings"`
	Securities   map[string]plaid.Security              `json:"securities"`
	Transactions map[string]plaid.InvestmentTransaction `json:"transactions"`
}

func (ii InvestmentInstitution) InvestmentAccount(id string) (InvestmentAccount, bool) {
	for _, account := range ii.InvestmentAccounts {
		if account.AccoutBase.AccountId == id {
			return account, true
		}
	}

	return InvestmentAccount{}, false
}

func (ii InvestmentInstitution) CreateOrUpdateInvestmentAccount(account InvestmentAccount) InvestmentInstitution {
	for i := range ii.InvestmentAccounts {
		if ii.InvestmentAccounts[i].AccoutBase.AccountId == account.AccoutBase.AccountId {
			ii.InvestmentAccounts[i] = account
			return ii
		}
	}

	ii.InvestmentAccounts = append(ii.InvestmentAccounts, account)
	return ii
}

func (ii InvestmentInstitution) CreateOrUpdateInvestmentAccountBase(accountBase plaid.AccountBase) InvestmentInstitution {
	for i := range ii.InvestmentAccounts {
		if ii.InvestmentAccounts[i].AccoutBase.AccountId == accountBase.AccountId {
			ii.InvestmentAccounts[i].AccoutBase = accountBase
			return ii
		}
	}

	ii.InvestmentAccounts = append(ii.InvestmentAccounts, InvestmentAccount{
		AccoutBase:   accountBase,
		Transactions: map[string]plaid.InvestmentTransaction{},
	})
	return ii
}

func (ii InvestmentInstitution) CreateOrUpdateInvestmentAccountBases(accountBases []plaid.AccountBase) InvestmentInstitution {
	for _, acb := range accountBases {
		ii = ii.CreateOrUpdateInvestmentAccountBase(acb)
	}

	return ii
}
