package dump

import (
	"math"
	"sort"
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

	// Plaid data comes from maps, so input order is random. Sort first and
	// pair each outflow with the closest-dated inflow so output is stable.
	transactions = sortedTransactions(transactions)

	merged := make([]BeancountTransaction, 0, len(transactions))
	processed := make(map[int]bool, len(transactions))

	if opts.sameOwner {
		mergedSelf := mergeSelfTransfers(transactions, processed, opts)
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

func txnID(t BeancountTransaction) string {
	if t.Metadata == nil {
		return ""
	}
	return t.Metadata["id"]
}

func sortedTransactions(in []BeancountTransaction) []BeancountTransaction {
	out := make([]BeancountTransaction, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return txnID(out[i]) < txnID(out[j])
	})
	return out
}

type transferPair struct {
	from, to  int
	days      int
	nameMatch bool
}

// pairTransfers matches outflows (from an Assets account) with inflows using
// accept, then assigns pairs globally: smallest date gap first, then pairs
// whose descriptions name the other side's institution, then date and id.
// Greedy per-outflow matching paired recurring same-amount payments (rent and
// a savings transfer on the same day) with the wrong inflow.
func pairTransfers(transactions []BeancountTransaction, processed map[int]bool, maxDays int, accept func(from, to BeancountTransaction) bool) []transferPair {
	var cands []transferPair
	for i, from := range transactions {
		if processed[i] || from.FromAccount.Type != "Assets" {
			continue
		}
		for j, to := range transactions {
			if processed[j] || j == i || !unitsMatch(from, to) || from.Amount != to.Amount {
				continue
			}
			if !accept(from, to) {
				continue
			}
			d := daysApart(from.Date, to.Date)
			if maxDays >= 0 && d > maxDays {
				continue
			}
			cands = append(cands, transferPair{from: i, to: j, days: d, nameMatch: mentionsCounterparty(from, to)})
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		x, y := cands[a], cands[b]
		if x.days != y.days {
			return x.days < y.days
		}
		if x.nameMatch != y.nameMatch {
			return x.nameMatch
		}
		if transactions[x.from].Date != transactions[y.from].Date {
			return transactions[x.from].Date < transactions[y.from].Date
		}
		if x.from != y.from {
			return x.from < y.from
		}
		return x.to < y.to
	})
	var out []transferPair
	for _, c := range cands {
		if processed[c.from] || processed[c.to] {
			continue
		}
		processed[c.from] = true
		processed[c.to] = true
		out = append(out, c)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].from < out[b].from })
	return out
}

func mentionsCounterparty(from, to BeancountTransaction) bool {
	norm := func(s string) string { return strings.ToLower(s) }
	toInst, fromInst := norm(to.ToAccount.Institution), norm(from.FromAccount.Institution)
	return (toInst != "" && strings.Contains(norm(from.Desc), toInst)) ||
		(fromInst != "" && strings.Contains(norm(to.Desc), fromInst))
}

func daysApart(a, b string) int {
	layout := "2006-01-02"
	at, errA := time.Parse(layout, a)
	bt, errB := time.Parse(layout, b)
	if errA != nil || errB != nil {
		if a == b {
			return 0
		}
		return math.MaxInt32
	}
	d := at.Sub(bt).Hours() / 24
	if d < 0 {
		d = -d
	}
	return int(d)
}

func mergeSelfTransfers(transactions []BeancountTransaction, processed map[int]bool, opts mergeOptions) []BeancountTransaction {
	result := make([]BeancountTransaction, 0)
	pairs := pairTransfers(transactions, processed, opts.maxDaysApart, func(from, to BeancountTransaction) bool {
		if to.ToAccount.Type != "Assets" && to.ToAccount.Type != "Liabilities" {
			return false
		}
		return from.FromAccount.Owner == to.ToAccount.Owner
	})
	for _, p := range pairs {
		fromTxn, toTxn := transactions[p.from], transactions[p.to]
		if fromTxn.FromAccount.ToString() == toTxn.ToAccount.ToString() {
			result = append(result, createSameAccountTransfer(fromTxn, toTxn))
		} else {
			result = append(result, createMergedTransaction(fromTxn, toTxn, "self transfer"))
		}
	}
	return result
}

func mergeCrossOwnerTransfers(transactions []BeancountTransaction, processed map[int]bool, opts mergeOptions) []BeancountTransaction {
	result := make([]BeancountTransaction, 0)
	pairs := pairTransfers(transactions, processed, opts.maxDaysApart, func(from, to BeancountTransaction) bool {
		return to.ToAccount.Type == "Assets" && from.FromAccount.Owner != to.ToAccount.Owner
	})
	for _, p := range pairs {
		fromTxn, toTxn := transactions[p.from], transactions[p.to]
		desc := "transfer " + fromTxn.FromAccount.Owner + " -> " + toTxn.ToAccount.Owner
		result = append(result, createMergedTransaction(fromTxn, toTxn, desc))
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
