package plaidclient

import "github.com/plaid/plaid-go/plaid"

var environments = map[string]plaid.Environment{
	"Development": plaid.Development,
	"Sandbox":     plaid.Sandbox,
	"Production":  plaid.Production,
}

// SetEnvironment allows tests to override the base URL used for a Plaid environment name.
func SetEnvironment(name string, env plaid.Environment) {
	environments[name] = env
}

// Environment returns the configured environment mapping if it exists.
func Environment(name string) (plaid.Environment, bool) {
	env, ok := environments[name]
	return env, ok
}

func New(clientID, secret, env string) *plaid.APIClient {
	pcfg := plaid.NewConfiguration()
	pcfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	pcfg.AddDefaultHeader("PLAID-SECRET", secret)
	if plaidEnv, ok := environments[env]; ok {
		pcfg.UseEnvironment(plaidEnv)
	} else {
		pcfg.UseEnvironment(plaid.Environment(env))
	}
	return plaid.NewAPIClient(pcfg)
}
