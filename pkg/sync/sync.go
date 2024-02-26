package sync

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/persistence"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func getTransactionAccounts(ctx context.Context, cli *plaid.APIClient, inst types.InstitutionBase) ([]plaid.AccountBase, error) {
	accountsGetRequest := plaid.NewAccountsGetRequest(inst.AccessToken)
	accountsGetResp, httpResp, err := cli.PlaidApi.AccountsGet(ctx).AccountsGetRequest(
		*accountsGetRequest,
	).Execute()
	if err != nil {
		logrus.Debug(httpResp.Body)
		return nil, fmt.Errorf("failed to execute account request: %w", err)
	}

	return accountsGetResp.GetAccounts(), nil
}

func Sync() error {
	ctx := context.Background()
	cfg, err := persistence.LoadConfig(persistence.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cli := plaidclient.New(cfg.ClientID, cfg.Secret, cfg.Environment)

	owners, err := persistence.LoadOwners(persistence.DefaultOwnerPath)
	if err != nil {
		return fmt.Errorf("failed to load owners: %w", err)
	}

	for _, owner := range owners {
		for _, inst := range owner.TransactionInstitutions {
			accountBases, err := getTransactionAccounts(ctx, cli, inst.InstitutionBase)
			if err != nil {
				return fmt.Errorf("failed to get accounts for %s:%s: %w", owner.Name, inst.InstitutionBase.Name, err)
			}
			inst = inst.CreateOrUpdateTransactionAccountBases(accountBases)
			if inst, err = syncTransactions(ctx, cli, inst); err != nil {
				return fmt.Errorf("failed to sync transactions: %w", err)
			}

			owner = owner.CreateOrUpdateTransactionInstitution(inst)
		}

		for _, inst := range owner.InvestmentInstitutions {
			if inst, err = syncInvestmentHoldings(ctx, cli, inst); err != nil {
				return fmt.Errorf("failed to sync transactions: %w", err)
			}

			owner = owner.CreateOrUpdateInvestmentInstitution(inst)
		}

		types.CreateOrUpdateOwner(owners, owner)
	}

	if err := persistence.DumpOwners(persistence.DefaultOwnerPath, owners); err != nil {
		return fmt.Errorf("failed to dump owners: %w", err)
	}
	fmt.Printf("Successfully synced all data to %q.\n", persistence.DefaultOwnerPath)
	return nil
}

func syncTransactions(ctx context.Context, cli *plaid.APIClient, inst types.TransactionInstitution) (types.TransactionInstitution, error) {
	hasMore := true
	cursor := inst.InstitutionBase.Cursor
	for hasMore {
		request := plaid.NewTransactionsSyncRequest(inst.InstitutionBase.AccessToken)
		if inst.InstitutionBase.Cursor != "" {
			request.SetCursor(cursor)
		}
		resp, _, err := cli.PlaidApi.TransactionsSync(
			ctx,
		).TransactionsSyncRequest(*request).Execute()

		if err != nil {
			return types.TransactionInstitution{}, fmt.Errorf("failed to execute sync request: %w", err)
		}

		for _, txn := range resp.GetAdded() {
			account, ok := inst.TransactionAccount(txn.AccountId)
			if !ok {
				continue
			}

			account.Transactions[txn.TransactionId] = txn
			inst = inst.CreateOrUpdateTransactionAccount(account)
		}

		for _, txn := range resp.GetModified() {
			account, ok := inst.TransactionAccount(txn.AccountId)
			if !ok {
				continue
			}

			account.Transactions[txn.TransactionId] = txn
			inst = inst.CreateOrUpdateTransactionAccount(account)
		}

		for _, txn := range resp.GetRemoved() {
			for _, account := range inst.TransactionAccounts {
				if _, ok := account.Transactions[*txn.TransactionId]; ok {
					delete(account.Transactions, *txn.TransactionId)
					inst = inst.CreateOrUpdateTransactionAccount(account)
					break
				}
			}
		}

		hasMore = resp.GetHasMore()

		// Update cursor to the next cursor and inst and dump the inst.
		cursor = resp.GetNextCursor()
		inst.InstitutionBase.Cursor = cursor
	}

	return inst, nil
}

func syncInvestmentHoldings(ctx context.Context, cli *plaid.APIClient, inst types.InvestmentInstitution) (types.InvestmentInstitution, error) {
	req := plaid.NewInvestmentsHoldingsGetRequest(inst.InstitutionBase.AccessToken)
	resp, httpResp, err := cli.PlaidApi.InvestmentsHoldingsGet(ctx).InvestmentsHoldingsGetRequest(*req).Execute()
	if err != nil {
		return types.InvestmentInstitution{}, fmt.Errorf("failed to execute get request: %w: %s", err, httpResp.Body)
	}

	accountBases := resp.GetAccounts()
	inst = inst.CreateOrUpdateInvestmentAccountBases(accountBases)

	accountIDToHoldings := map[string][]plaid.Holding{}
	securities := map[string]plaid.Security{}

	for _, s := range resp.GetSecurities() {
		securities[s.SecurityId] = s
	}

	for _, h := range resp.GetHoldings() {
		accountIDToHoldings[h.AccountId] = append(accountIDToHoldings[h.AccountId], h)
	}

	for accountID, holdings := range accountIDToHoldings {
		account, ok := inst.InvestmentAccount(accountID)
		if !ok {
			continue
		}

		account.Holdings = holdings
		account.Securities = securities
		inst = inst.CreateOrUpdateInvestmentAccount(account)
	}

	return inst, nil
}
