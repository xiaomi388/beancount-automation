package sync

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/config"
	"github.com/xiaomi388/beancount-automation/pkg/holding"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/transaction"
)

func getAllTxnAccounts(ctx context.Context, cli *plaid.APIClient, inst config.Institution) (map[string]plaid.AccountBase, error) {
	accounts := map[string]plaid.AccountBase{}

	accountsGetRequest := plaid.NewAccountsGetRequest(inst.AccessToken)
	accountsGetResp, httpResp, err := cli.PlaidApi.AccountsGet(ctx).AccountsGetRequest(
		*accountsGetRequest,
	).Execute()
	if err != nil {
		logrus.Debug(httpResp.Body)
		return nil, fmt.Errorf("failed to execute account request: %w", err)
	}

	for _, account := range accountsGetResp.GetAccounts() {
		accounts[account.GetAccountId()] = account
	}

	return accounts, nil
}

func Sync() error {
	ctx := context.Background()
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	txns, err := transaction.Load(cfg.TransactionDBPath)
	if err != nil {
		return fmt.Errorf("failed to load transactions from db: %w", err)
	}

	cli := plaidclient.New(cfg.ClientID, cfg.Secret, cfg.Environment)

	holdings := []holding.Holding{}

	for _, owner := range cfg.Owners {
		for _, inst := range owner.Institutions {
			switch inst.Type {
			case plaid.PRODUCTS_TRANSACTIONS:
				txnAccounts, err := getAllTxnAccounts(ctx, cli, inst)
				if err != nil {
					return fmt.Errorf("failed to get accounts for %s-%s: %w", owner.Name, inst.Name, err)
				}
				err = syncTransactions(ctx, cli, cfg, owner, inst, txns, txnAccounts)
				if err != nil {
					return fmt.Errorf("failed to sync transactions for %s:%s: %w", owner.Name, inst.Name, err)
				}
			case plaid.PRODUCTS_INVESTMENTS:
				hs, err := syncInvestmentHoldings(ctx, cli, owner, inst)
				if err != nil {
					return fmt.Errorf("failed to sync holdings for %s:%s : %w", owner.Name, inst.Name, err)
				}
				holdings = append(holdings, hs...)
			default:
				return fmt.Errorf("unsupported account type %s on %s:%s", inst.Type, owner.Name, inst.Name)
			}
		}
	}

	holding.Dump(cfg.HoldingDBPath, holdings)
	fmt.Printf("Successfully synced all data to %q and %q.\n", cfg.TransactionDBPath, cfg.HoldingDBPath)
	return nil
}

// TODO: refactor the sync funcs by adding a syncer struct
func syncTransactions(ctx context.Context, cli *plaid.APIClient, cfg *config.Config, owner config.Owner, inst config.Institution, txns map[string]transaction.Transaction, accounts map[string]plaid.AccountBase) error {

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
			txns[txn.TransactionId] = transaction.NewTransaction(txn, owner.Name, inst.Name, accounts[txn.AccountId])
		}

		for _, txn := range resp.GetModified() {
			txns[txn.TransactionId] = transaction.NewTransaction(txn, owner.Name, inst.Name, accounts[txn.AccountId])
		}

		for _, txn := range resp.GetRemoved() {
			delete(txns, *txn.TransactionId)
		}

		hasMore = resp.GetHasMore()

		// Dump transaction
		if err := transaction.Dump(cfg.TransactionDBPath, txns); err != nil {
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

	return nil
}

func syncInvestmentHoldings(ctx context.Context, cli *plaid.APIClient, owner config.Owner, inst config.Institution) ([]holding.Holding, error) {
	req := plaid.NewInvestmentsHoldingsGetRequest(inst.AccessToken)
	resp, httpResp, err := cli.PlaidApi.InvestmentsHoldingsGet(ctx).InvestmentsHoldingsGetRequest(*req).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to execute get request: %w: %s", err, httpResp.Body)
	}

	accounts := map[string]plaid.AccountBase{}
	accountList := resp.GetAccounts()
	for _, account := range accountList {
		accounts[account.AccountId] = account
	}

	securities := map[string]plaid.Security{}
	securityList := resp.GetSecurities()
	for _, security := range securityList {
		securities[security.SecurityId] = security
	}

	holdings := []holding.Holding{}
	for _, h := range resp.GetHoldings() {
		holdings = append(holdings, holding.New(h, securities[h.SecurityId], owner.Name, inst.Name, accounts[h.AccountId]))
	}

	return holdings, nil
}
