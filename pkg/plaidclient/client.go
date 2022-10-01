package plaidclient

import (
	"github.com/plaid/plaid-go/plaid"
)

func New(clientID, secret, env string) *plaid.APIClient {
	pcfg := plaid.NewConfiguration()
	pcfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	pcfg.AddDefaultHeader("PLAID-SECRET", secret)
	pcfg.UseEnvironment(map[string]plaid.Environment{
        "Development": plaid.Development,
        "Sandbox": plaid.Sandbox,
        "Production": plaid.Production,
    }[env])
	return plaid.NewAPIClient(pcfg)
}
