package link

import (
	"context"
	"fmt"

	"github.com/xiaomi388/beancount-automation/pkg/persistence"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func Relink(ownerName string, instName string, instType types.InstitutionType) error {
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
		return fmt.Errorf("owner %s not existed", ownerName)
	}

	var accessToken string
	switch instType {
	case types.InstitutionTypeTransaction:
		inst, ok := owner.TransactionInstitution(instName)
		if !ok {
			return fmt.Errorf("inst %s not existed", instName)
		}
		accessToken = inst.InstitutionBase.AccessToken
	case types.InstitutionTypeInvestment:
		inst, ok := owner.InvestmentInstitution(instName)
		if !ok {
			return fmt.Errorf("inst %s not existed", instName)
		}
		accessToken = inst.InstitutionBase.AccessToken
	default:
		panic(fmt.Sprintf("unsupported institution type: %s", instType))
	}

	ctx := context.Background()

	c := plaidclient.New(config.ClientID, config.Secret, config.Environment)
	linkToken, err := createLinkToken(ctx, c, nil, &accessToken)
	if err != nil {
		return fmt.Errorf("failed to create link token: %w", err)
	}

	if _, err := launchLinkFlow(ctx, linkToken); err != nil {
		return fmt.Errorf("failed to launch link flow: %w", err)
	}

	return nil
}
