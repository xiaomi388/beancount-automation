package plaidclient

import (
	"github.com/plaid/plaid-go/plaid"
)

func New(clientID, secret string) *plaid.APIClient {
	cfg := plaid.NewConfiguration()
	cfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	cfg.AddDefaultHeader("PLAID-SECRET", secret)
	cfg.UseEnvironment(plaid.Development)
	return plaid.NewAPIClient(cfg)
}
