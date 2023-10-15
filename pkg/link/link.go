package link

import (
	"context"
	"fmt"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
)

func Link(owner string, institution string, accountType plaid.Products) error {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	if _, ok := cfg.Institution(institution, owner); ok {
		return fmt.Errorf("%s:%s already existed", owner, institution)
	}

	ctx := context.Background()

	c := plaidclient.New(cfg.ClientID, cfg.Secret, cfg.Environment)
	linkToken, err := createLinkToken(ctx, c, &accountType, nil)
	if err != nil {
		return fmt.Errorf("failed to create link token: %w", err)
	}

	if err := generateAuthPage(linkToken); err != nil {
		return fmt.Errorf("failed to generate auth page: %w", err)
	}

	publicToken, err := readPublicToken()
	if err != nil {
		return fmt.Errorf("failed to read public token: %w", err)
	}

	accessToken, err := exchangeAccessToken(ctx, c, publicToken)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	cfg.SetInstitution(config.Institution{
		Name:        institution,
		AccessToken: accessToken,
		Type:        accountType,
	}, owner)

	config.Dump(config.ConfigPath, cfg)
	return nil
}
