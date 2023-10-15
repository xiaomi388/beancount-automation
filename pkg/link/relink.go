package link

import (
	"context"
	"fmt"

	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
)

func Relink(owner string, institution string) error {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	inst, ok := cfg.Institution(institution, owner)
	if !ok {
		return fmt.Errorf("%s:%s not existed", owner, institution)
	}

	ctx := context.Background()

	c := plaidclient.New(cfg.ClientID, cfg.Secret, cfg.Environment)
	linkToken, err := createLinkToken(ctx, c, nil, &inst.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to create link token: %w", err)
	}

	if err := generateAuthPage(linkToken); err != nil {
		return fmt.Errorf("failed to generate auth page: %w", err)
	}

	return nil
}
