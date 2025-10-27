package dump

import (
	"strings"
	"time"

	"github.com/xiaomi388/beancount-automation/pkg/types"
)

type mergeOptions struct {
	enabled      bool
	sameOwner    bool
	crossOwner   bool
	maxDaysApart int
}

func applyPostprocessTransactions(transactions []BeancountTransaction, cfg types.PostprocessConfig) []BeancountTransaction {
	result := transactions

	if shouldApplyMerge(cfg.Merge) {
		result = mergeTransactions(result, buildMergeOptions(cfg.Merge))
	}

	if rules := resolveCategoryRules(cfg.Categorise); len(rules) > 0 {
		result = applyCategoryRules(result, rules)
	}

	return result
}

func shouldApplyMerge(cfg *types.MergeConfig) bool {
	if cfg == nil {
		return true
	}
	return boolValue(cfg.Enabled, true)
}

func buildMergeOptions(cfg *types.MergeConfig) mergeOptions {
	if cfg == nil {
		return mergeOptions{
			enabled:      true,
			sameOwner:    true,
			crossOwner:   true,
			maxDaysApart: 10,
		}
	}

	return mergeOptions{
		enabled:      boolValue(cfg.Enabled, true),
		sameOwner:    boolValue(cfg.SameOwner, true),
		crossOwner:   boolValue(cfg.CrossOwner, true),
		maxDaysApart: intValue(cfg.MaxDaysApart, 10),
	}
}

func resolveCategoryRules(cfg *types.CategoriseConfig) []types.KeywordRule {
	if cfg == nil {
		return nil
	}

	if !boolValue(cfg.Enabled, len(cfg.KeywordRules) > 0) {
		return nil
	}

	return cfg.KeywordRules
}

func applyCategoryRules(transactions []BeancountTransaction, rules []types.KeywordRule) []BeancountTransaction {
	result := make([]BeancountTransaction, len(transactions))
	copy(result, transactions)

	for idx, txn := range result {
		for _, rule := range rules {
			if matchesRule(txn, rule.Match) {
				result[idx] = applyMutations(txn, rule.Set)
				break
			}
		}
	}

	return result
}

func mergeTransactions(transactions []BeancountTransaction, opts mergeOptions) []BeancountTransaction {
	if !opts.enabled {
		return transactions
	}

	merged := make([]BeancountTransaction, 0, len(transactions))
	processed := make(map[int]bool, len(transactions))

	if opts.sameOwner {
		mergedSelf := mergeSelfTransfers(transactions, processed)
		merged = append(merged, mergedSelf...)
	}

	if opts.crossOwner {
		mergedCross := mergeCrossOwnerTransfers(transactions, processed, opts)
		merged = append(merged, mergedCross...)
	}

	for i, txn := range transactions {
		if !processed[i] {
			merged = append(merged, txn)
		}
	}

	return merged
}

func mergeSelfTransfers(transactions []BeancountTransaction, processed map[int]bool) []BeancountTransaction {
	result := make([]BeancountTransaction, 0)

	for i, fromTxn := range transactions {
		if processed[i] || fromTxn.FromAccount.Type != "Assets" {
			continue
		}

		for j, toTxn := range transactions {
			if processed[j] || j == i {
				continue
			}

			if !unitsMatch(fromTxn, toTxn) || fromTxn.Amount != toTxn.Amount {
				continue
			}

			if toTxn.ToAccount.Type != "Assets" && toTxn.ToAccount.Type != "Liabilities" {
				continue
			}

			if fromTxn.FromAccount.Owner != toTxn.ToAccount.Owner {
				continue
			}

			var mergedTxn BeancountTransaction

			if fromTxn.FromAccount.ToString() == toTxn.ToAccount.ToString() {
				mergedTxn = createSameAccountTransfer(fromTxn, toTxn)
			} else {
				mergedTxn = createMergedTransaction(fromTxn, toTxn, "self transfer")
			}

			result = append(result, mergedTxn)
			processed[i] = true
			processed[j] = true
			break
		}
	}

	return result
}

func mergeCrossOwnerTransfers(transactions []BeancountTransaction, processed map[int]bool, opts mergeOptions) []BeancountTransaction {
	result := make([]BeancountTransaction, 0)

	for i, fromTxn := range transactions {
		if processed[i] || fromTxn.FromAccount.Type != "Assets" {
			continue
		}

		for j, toTxn := range transactions {
			if processed[j] || j == i {
				continue
			}

			if !unitsMatch(fromTxn, toTxn) || fromTxn.Amount != toTxn.Amount {
				continue
			}

			if toTxn.ToAccount.Type != "Assets" {
				continue
			}

			if fromTxn.FromAccount.Owner == toTxn.ToAccount.Owner {
				continue
			}

			if !datesWithinRange(fromTxn.Date, toTxn.Date, opts.maxDaysApart) {
				continue
			}

			payer := fromTxn.FromAccount.Owner
			payee := toTxn.ToAccount.Owner
			desc := "transfer " + payer + " -> " + payee
			mergedTxn := createMergedTransaction(fromTxn, toTxn, desc)
			result = append(result, mergedTxn)
			processed[i] = true
			processed[j] = true
			break
		}
	}

	return result
}

func createMergedTransaction(fromTxn, toTxn BeancountTransaction, desc string) BeancountTransaction {
	date := fromTxn.Date
	if toTxn.Date > date {
		date = toTxn.Date
	}

	metadata := make(map[string]string)
	if fromTxn.Metadata != nil {
		if id, ok := fromTxn.Metadata["id"]; ok {
			metadata["from_id"] = id
		}
	}
	if toTxn.Metadata != nil {
		if id, ok := toTxn.Metadata["id"]; ok {
			metadata["to_id"] = id
		}
	}

	payer := fromTxn.FromAccount.Owner
	if payer != "" {
		metadata["payer"] = payer
	}

	return BeancountTransaction{
		Date:        date,
		Payee:       toTxn.ToAccount.Owner,
		Desc:        desc,
		FromAccount: fromTxn.FromAccount,
		ToAccount:   toTxn.ToAccount,
		Metadata:    metadata,
		Tags:        []string{},
		Unit:        fromTxn.Unit,
		Amount:      fromTxn.Amount,
	}
}

func createSameAccountTransfer(fromTxn, toTxn BeancountTransaction) BeancountTransaction {
	syntheticFrom := fromTxn
	syntheticTo := toTxn

	syntheticFrom.FromAccount = pickAssetAccount(fromTxn.FromAccount, fromTxn.ToAccount)
	syntheticTo.ToAccount = pickAssetAccount(toTxn.ToAccount, toTxn.FromAccount)

	return createMergedTransaction(syntheticFrom, syntheticTo, "self transfer")
}

func pickAssetAccount(primary, secondary Account) Account {
	if primary.Type == "Assets" {
		return primary
	}
	if secondary.Type == "Assets" {
		return secondary
	}
	return primary
}

func matchesRule(txn BeancountTransaction, crit types.MatchCriteria) bool {
	if !matchesText(txn.Desc, crit.Description) {
		return false
	}
	if !matchesText(txn.Payee, crit.Payee) {
		return false
	}

	if len(crit.Metadata) > 0 {
		for key, tc := range crit.Metadata {
			value := ""
			if txn.Metadata != nil {
				value = txn.Metadata[key]
			}
			if !matchesText(value, tc) {
				return false
			}
		}
	}

	return true
}

func matchesText(value string, crit types.TextCriteria) bool {
	if crit.Equals != "" && value != crit.Equals {
		return false
	}

	if len(crit.Contains) > 0 {
		found := false
		for _, needle := range crit.Contains {
			if needle != "" && strings.Contains(value, needle) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func applyMutations(txn BeancountTransaction, set types.SetMutations) BeancountTransaction {
	if len(set.Tags) > 0 {
		txn.Tags = append([]string(nil), set.Tags...)
	}

	if len(set.ToAccount.Category) > 0 {
		txn.ToAccount.Category = append([]string(nil), set.ToAccount.Category...)
	}
	if set.ToAccount.Name != "" {
		txn.ToAccount.Name = set.ToAccount.Name
	}

	if len(set.FromAccount.Category) > 0 {
		txn.FromAccount.Category = append([]string(nil), set.FromAccount.Category...)
	}
	if set.FromAccount.Name != "" {
		txn.FromAccount.Name = set.FromAccount.Name
	}

	return txn
}

func copyStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func unitsMatch(a, b BeancountTransaction) bool {
	return a.Unit == b.Unit
}

func datesWithinRange(a, b string, maxDays int) bool {
	if maxDays <= 0 {
		return a == b
	}

	layout := "2006-01-02"
	at, errA := time.Parse(layout, a)
	bt, errB := time.Parse(layout, b)
	if errA != nil || errB != nil {
		return a == b
	}

	diff := at.Sub(bt)
	if diff < 0 {
		diff = -diff
	}

	return diff.Hours() <= float64(maxDays*24)
}

func boolValue(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func intValue(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}
