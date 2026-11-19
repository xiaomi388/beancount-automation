package dump

import (
	"math/rand"
	"testing"
)

func acct(typ, owner, name string) Account {
	return Account{Type: typ, Owner: owner, Country: "USD", Institution: "Bank", PlaidAccountType: "Depository", Name: name}
}

func leg(id, date string, from, to Account, amt float32) BeancountTransaction {
	return BeancountTransaction{Date: date, Unit: "USD", Amount: amt, FromAccount: from, ToAccount: to, Metadata: map[string]string{"id": id}}
}

// Two identical monthly transfers: each outflow must pair with the inflow of
// the same month, whatever order the input arrives in.
func TestSelfTransferPairsClosestDateDeterministically(t *testing.T) {
	chk := acct("Assets", "A", "Checking")
	sav := acct("Assets", "A", "Savings")
	out := Account{Type: "Expenses", Country: "USD", Category: []string{"Transfer"}}
	in := Account{Type: "Income", Country: "USD", Category: []string{"Transfer"}}
	base := []BeancountTransaction{
		leg("o1", "2026-01-05", chk, out, 3400),
		leg("i1", "2026-01-06", in, sav, 3400),
		leg("o2", "2026-02-05", chk, out, 3400),
		leg("i2", "2026-02-06", in, sav, 3400),
	}
	var first []BeancountTransaction
	for n := 0; n < 50; n++ {
		in := append([]BeancountTransaction(nil), base...)
		rand.Shuffle(len(in), func(i, j int) { in[i], in[j] = in[j], in[i] })
		got := mergeTransactions(in, mergeOptions{enabled: true, sameOwner: true, crossOwner: true, maxDaysApart: 10})
		if len(got) != 2 {
			t.Fatalf("expected 2 merged transfers, got %d", len(got))
		}
		for _, m := range got {
			if m.Metadata["from_id"][1:] != m.Metadata["to_id"][1:] {
				t.Fatalf("paired across months: %v", m.Metadata)
			}
		}
		if first == nil {
			first = got
			continue
		}
		for k := range got {
			if got[k].Metadata["from_id"] != first[k].Metadata["from_id"] || got[k].Date != first[k].Date {
				t.Fatalf("output depends on input order")
			}
		}
	}
}

// Rent and a savings transfer leave checking on the same day for the same
// amount; only the savings transfer has a matching inflow.
func TestSameDaySameAmountPrefersNamedCounterparty(t *testing.T) {
	chk := acct("Assets", "A", "Checking")
	chk.Institution = "BOA"
	wf := acct("Assets", "A", "Cash")
	wf.Institution = "WealthFront"
	rent := Account{Type: "Expenses", Country: "USD", Category: []string{"Payment", "Rent"}}
	fin := Account{Type: "Expenses", Country: "USD", Category: []string{"Service", "Financial"}}
	dep := Account{Type: "Income", Country: "USD", Category: []string{"Transfer", "Deposit"}}
	var txns []BeancountTransaction
	for _, m := range []string{"01", "02", "03"} {
		r := leg("a-rent-"+m, "2026-"+m+"-01", chk, rent, 3400)
		r.Desc = "ZelleRecurringpaymentforRent"
		w := leg("z-wf-"+m, "2026-"+m+"-01", chk, fin, 3400)
		w.Desc = "WealthfrontDESEDIPYMNTS"
		in := leg("m-in-"+m, "2026-"+m+"-01", dep, wf, 3400)
		in.Desc = "BankofAmericaDeposit"
		txns = append(txns, r, w, in)
	}
	got := mergeTransactions(txns, mergeOptions{enabled: true, sameOwner: true, crossOwner: true, maxDaysApart: 10})
	rents, transfers := 0, 0
	for _, g := range got {
		switch {
		case g.Desc == "self transfer":
			transfers++
			if g.Metadata["from_id"][:4] != "z-wf" {
				t.Errorf("savings inflow paired with %s", g.Metadata["from_id"])
			}
		case g.ToAccount.ToString() == rent.ToString():
			rents++
		default:
			t.Errorf("unexpected leftover: %s %s", g.Desc, g.ToAccount.ToString())
		}
	}
	if rents != 3 || transfers != 3 {
		t.Fatalf("rents=%d transfers=%d, want 3 and 3", rents, transfers)
	}
}

func TestSelfTransferOutsideWindowNotMerged(t *testing.T) {
	chk := acct("Assets", "A", "Checking")
	sav := acct("Assets", "A", "Savings")
	out := Account{Type: "Expenses", Country: "USD", Category: []string{"Payment", "Rent"}}
	in := Account{Type: "Income", Country: "USD", Category: []string{"Transfer"}}
	got := mergeTransactions([]BeancountTransaction{
		leg("o", "2024-01-01", chk, out, 3400),
		leg("i", "2026-01-01", in, sav, 3400),
	}, mergeOptions{enabled: true, sameOwner: true, crossOwner: true, maxDaysApart: 10})
	if len(got) != 2 {
		t.Fatalf("legs two years apart were merged: %+v", got)
	}
}
