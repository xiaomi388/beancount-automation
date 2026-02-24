package persistence

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func newTestOwners() []types.Owner {
	return []types.Owner{
		{
			Name: "alice",
			TransactionInstitutions: []types.TransactionInstitution{
				{
					InstitutionBase: types.InstitutionBase{
						Name:        "bank-a",
						AccessToken: "tok-a",
						Cursor:      "cur-1",
					},
					TransactionAccounts: []types.TransactionAccount{
						{
							AccoutBase: plaid.AccountBase{
								AccountId: "acct-1",
								Name:      "Checking",
							},
							Transactions: map[string]plaid.Transaction{
								"txn-1": {
									TransactionId: "txn-1",
									AccountId:     "acct-1",
									Amount:        42.5,
									Name:          "Coffee Shop",
								},
								"txn-2": {
									TransactionId: "txn-2",
									AccountId:     "acct-1",
									Amount:        100.0,
									Name:          "Grocery Store",
								},
							},
						},
					},
				},
			},
			InvestmentInstitutions: []types.InvestmentInstitution{
				{
					InstitutionBase: types.InstitutionBase{
						Name:        "broker-a",
						AccessToken: "tok-inv-a",
						Cursor:      "",
					},
					InvestmentAccounts: []types.InvestmentAccount{
						{
							AccoutBase: plaid.AccountBase{
								AccountId: "inv-acct-1",
								Name:      "Brokerage",
							},
							Holdings: []plaid.Holding{
								{
									AccountId:  "inv-acct-1",
									SecurityId: "sec-1",
									Quantity:   10.0,
								},
							},
							Securities: map[string]plaid.Security{
								"sec-1": {
									SecurityId:   "sec-1",
									TickerSymbol: *plaid.NewNullableString(strPtr("AAPL")),
								},
							},
							Transactions: map[string]plaid.InvestmentTransaction{
								"inv-txn-1": {
									InvestmentTransactionId: "inv-txn-1",
									AccountId:               "inv-acct-1",
									Amount:                  500.0,
								},
							},
						},
					},
				},
			},
		},
	}
}

func strPtr(s string) *string {
	return &s
}

func TestSQLiteStoreRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	owners := newTestOwners()

	if err := store.DumpOwners(owners); err != nil {
		t.Fatalf("DumpOwners: %v", err)
	}

	loaded, err := store.LoadOwners()
	if err != nil {
		t.Fatalf("LoadOwners: %v", err)
	}

	// Compare via JSON since plaid types have unexported fields
	expected, _ := json.Marshal(owners)
	actual, _ := json.Marshal(loaded)

	if string(expected) != string(actual) {
		t.Errorf("round-trip mismatch.\nExpected: %s\nActual:   %s", string(expected), string(actual))
	}
}

func TestSQLiteStoreEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	loaded, err := store.LoadOwners()
	if err != nil {
		t.Fatalf("LoadOwners: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("expected empty, got %d owners", len(loaded))
	}
}

func TestSQLiteStoreOverwrite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Write initial data
	owners := newTestOwners()
	if err := store.DumpOwners(owners); err != nil {
		t.Fatalf("DumpOwners (first): %v", err)
	}

	// Overwrite with different data
	owners2 := []types.Owner{
		{
			Name:                    "bob",
			TransactionInstitutions: []types.TransactionInstitution{},
			InvestmentInstitutions:  []types.InvestmentInstitution{},
		},
	}
	if err := store.DumpOwners(owners2); err != nil {
		t.Fatalf("DumpOwners (second): %v", err)
	}

	loaded, err := store.LoadOwners()
	if err != nil {
		t.Fatalf("LoadOwners: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Name != "bob" {
		t.Errorf("expected single owner 'bob', got %+v", loaded)
	}
}
