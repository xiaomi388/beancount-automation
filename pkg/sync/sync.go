package sync

import (
	"context"
	"fmt"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/transaction"
)

func getAllAccounts(ctx context.Context, cli *plaid.APIClient, cfg *config.Config) (map[string]plaid.AccountBase, error) {
    accounts := map[string]plaid.AccountBase{}

    for _, owner := range cfg.Owners {
        for _, inst := range owner.Institutions {
            accountsGetRequest := plaid.NewAccountsGetRequest(inst.AccessToken)
            accountsGetResp, _, err := cli.PlaidApi.AccountsGet(ctx).AccountsGetRequest(
              *accountsGetRequest,
            ).Execute()
            if err != nil {
                return nil, fmt.Errorf("failed to execute account request: %w", err)

            }

            for _, account := range accountsGetResp.GetAccounts() {
                accounts[account.GetAccountId()] = account
            }
        }
    }

    return accounts, nil
}

func Sync() error {
	ctx := context.Background()
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	txns, err := transaction.Load(transaction.DBPath)
    if err != nil {
        return fmt.Errorf("failed to load transactions from db: %w", err)
    }

	cli := plaidclient.New(cfg.ClientID, cfg.Secret)

    accounts, err := getAllAccounts(ctx, cli, cfg)
    if err != nil {
        return fmt.Errorf("failed to get accounts: %w", err)
    }

	for _, owner := range cfg.Owners {
        for _, inst := range owner.Institutions {
            hasMore := true
            cursor := inst.Cursor
            for hasMore {
                request := plaid.NewTransactionsSyncRequest(inst.AccessToken)
                if inst.Cursor != "" {
                    request.SetCursor(cursor)
                }
                resp, _, err := cli.PlaidApi.TransactionsSync(
                    ctx,
                ).TransactionsSyncRequest(*request).Execute()

                if err != nil {
                    return fmt.Errorf("failed to execute sync request: %w", err)
                }

                for _, txn := range resp.GetAdded() {
                    txns[txn.TransactionId] = transaction.New(txn, owner.Name, inst.Name, accounts[txn.AccountId])
                }

                for _, txn := range resp.GetModified() {
                    txns[txn.TransactionId] = transaction.New(txn, owner.Name, inst.Name, accounts[txn.AccountId])
                }

                for _, txn := range resp.GetRemoved() {
                    delete(txns, *txn.TransactionId)
                }

                hasMore = resp.GetHasMore()

                // Dump transaction
                if err := transaction.Dump(transaction.DBPath, txns); err != nil {
                    return fmt.Errorf("failed to dump transactions: %w", err)
                }

                // Update cursor to the next cursor and inst and dump the inst.
                cursor = resp.GetNextCursor()
                inst.Cursor = cursor
                if err := cfg.SetInstitution(inst, owner.Name); err != nil {
                    return fmt.Errorf("failed to set inst cursor: %w", err)
                }
                config.Dump(config.ConfigPath, cfg)
            }
        }
	}

	return nil
}
