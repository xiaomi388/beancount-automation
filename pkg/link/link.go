package link

import (
	"context"
	"fmt"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/persistence"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

var (
	getAccessTokenFn = getAccessToken
)

func Link(ownerName string, instName string, instType types.InstitutionType) error {
	config, err := persistence.LoadConfig(persistence.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	owners, err := persistence.LoadOwners(persistence.DefaultOwnerPath)
	if err != nil {
		return fmt.Errorf("failed to load owners: %w", err)
	}

	owner, ok := types.GetOwner(owners, ownerName)
	if !ok {
		owner = types.Owner{
			Name: ownerName,
		}
	}

	switch instType {
	case types.InstitutionTypeTransaction:
		owner, err = linkTransactionInstitution(owner, instName, config)
	case types.InstitutionTypeInvestment:
		owner, err = linkInvestmentInstitution(owner, instName, config)
	default:
		panic(fmt.Sprintf("unsupported institution type: %s", instType))
	}

	if err != nil {
		return fmt.Errorf("failed to link institution: %w", err)
	}

	owners = types.CreateOrUpdateOwner(owners, owner)

	if err := persistence.DumpOwners(persistence.DefaultOwnerPath, owners); err != nil {
		return fmt.Errorf("failed to dump owners: %w", err)
	}

	return nil
}

func getAccessToken(clientID string, secret string, env string, product *plaid.Products) (string, error) {
	ctx := context.Background()
	c := plaidclient.New(clientID, secret, env)
	linkToken, err := createLinkToken(ctx, c, product, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create link token: %w", err)
	}

	publicToken, err := launchLinkFlow(ctx, linkToken)
	if err != nil {
		return "", fmt.Errorf("failed to obtain public token: %w", err)
	}

	accessToken, err := exchangeAccessToken(ctx, c, publicToken)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return accessToken, nil
}

func linkTransactionInstitution(owner types.Owner, instName string, config types.Config) (types.Owner, error) {
	if _, ok := owner.TransactionInstitution(instName); ok {
		return types.Owner{}, fmt.Errorf("transaction institution %s:%s already existed", owner.Name, instName)
	}

	accessToken, err := getAccessTokenFn(config.ClientID, config.Secret, config.Environment, instTypeToPlaidProduct(types.InstitutionTypeTransaction))
	if err != nil {
		return types.Owner{}, fmt.Errorf("failed to get access token: %w", err)
	}

	owner.TransactionInstitutions = append(owner.TransactionInstitutions, types.TransactionInstitution{
		InstitutionBase: types.InstitutionBase{
			Name:        instName,
			AccessToken: accessToken,
		},
	})

	return owner, nil
}

func linkInvestmentInstitution(owner types.Owner, instName string, config types.Config) (types.Owner, error) {
	if _, ok := owner.InvestmentInstitution(instName); ok {
		return types.Owner{}, fmt.Errorf("investment institution %s:%s already existed", owner.Name, instName)
	}

	accessToken, err := getAccessTokenFn(config.ClientID, config.Secret, config.Environment, instTypeToPlaidProduct(types.InstitutionTypeInvestment))
	if err != nil {
		return types.Owner{}, fmt.Errorf("failed to get access token: %w", err)
	}

	owner.InvestmentInstitutions = append(owner.InvestmentInstitutions, types.InvestmentInstitution{
		InstitutionBase: types.InstitutionBase{
			Name:        instName,
			AccessToken: accessToken,
		},
	})

	return owner, nil
}

func instTypeToPlaidProduct(instType types.InstitutionType) *plaid.Products {
	products := plaid.Products("")
	switch instType {
	case types.InstitutionTypeTransaction:
		products = plaid.PRODUCTS_TRANSACTIONS
	case types.InstitutionTypeInvestment:
		products = plaid.PRODUCTS_INVESTMENTS
	default:
		panic(fmt.Sprintf("unsupported institution type: %s", instType))
	}

	return &products
}
