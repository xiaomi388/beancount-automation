package link

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

const removedOwnersJSON = `[
  {
    "name": "alice",
    "transactionInstitutions": [
      {
        "institutionBase": {"name": "chase", "accessToken": "", "cursor": "stale-cursor"},
        "transactionAccounts": [
          {"accountBase": {"account_id": "old-acct", "mask": "1234", "name": "CREDIT CARD", "type": "credit", "subtype": "credit card", "balances": {}}, "transactions": {}}
        ]
      },
      {
        "institutionBase": {"name": "boa", "accessToken": "live-token", "cursor": "c1"},
        "transactionAccounts": []
      }
    ],
    "investmentInstitutions": [
      {
        "InstitutionBase": {"name": "schwab", "accessToken": "", "cursor": ""},
        "InvestmentAccounts": []
      }
    ]
  }
]`

func setupRemovedOwners(t *testing.T) string {
	t.Helper()
	orig := getAccessTokenFn
	getAccessTokenFn = func(clientID, secret, env string, product *plaid.Products) (string, error) {
		return "new-token", nil
	}
	t.Cleanup(func() { getAccessTokenFn = orig })

	dir := t.TempDir()
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	copyTestFile(t, filepath.Join(cwd, "testdata", "config.yaml"), filepath.Join(dir, "config.yaml"))
	if err := os.WriteFile(filepath.Join(dir, "owners.yaml"), []byte(removedOwnersJSON), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func loadOwners(t *testing.T, dir string) []types.Owner {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "owners.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var owners []types.Owner
	if err := json.Unmarshal(data, &owners); err != nil {
		t.Fatal(err)
	}
	return owners
}

func TestLinkReusesRemovedTransactionEntry(t *testing.T) {
	dir := setupRemovedOwners(t)
	if err := Link("alice", "chase", types.InstitutionTypeTransaction); err != nil {
		t.Fatalf("Link: %v", err)
	}
	owners := loadOwners(t, dir)
	if n := len(owners[0].TransactionInstitutions); n != 2 {
		t.Fatalf("expected 2 transaction institutions (no duplicate), got %d", n)
	}
	inst, _ := owners[0].TransactionInstitution("chase")
	if inst.InstitutionBase.AccessToken != "new-token" {
		t.Fatalf("token not attached: %q", inst.InstitutionBase.AccessToken)
	}
	if inst.InstitutionBase.Cursor != "" {
		t.Fatalf("stale cursor kept: %q", inst.InstitutionBase.Cursor)
	}
	if len(inst.TransactionAccounts) != 1 || inst.TransactionAccounts[0].AccoutBase.AccountId != "old-acct" {
		t.Fatalf("historical account lost: %+v", inst.TransactionAccounts)
	}
}

func TestLinkReusesRemovedInvestmentEntry(t *testing.T) {
	dir := setupRemovedOwners(t)
	if err := Link("alice", "schwab", types.InstitutionTypeInvestment); err != nil {
		t.Fatalf("Link: %v", err)
	}
	owners := loadOwners(t, dir)
	if n := len(owners[0].InvestmentInstitutions); n != 1 {
		t.Fatalf("expected 1 investment institution, got %d", n)
	}
	inst, _ := owners[0].InvestmentInstitution("schwab")
	if inst.InstitutionBase.AccessToken != "new-token" {
		t.Fatalf("token not attached: %q", inst.InstitutionBase.AccessToken)
	}
}

func TestLinkRefusesLiveEntry(t *testing.T) {
	setupRemovedOwners(t)
	if err := Link("alice", "boa", types.InstitutionTypeTransaction); err == nil {
		t.Fatal("expected error when linking over a live Item")
	}
}

func TestCreateTransactionsLinkTokenSendsDaysRequested(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/link/token/create" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"link_token":"link-sandbox-123"}`))
	}))
	defer srv.Close()

	orig, _ := plaidclient.Environment("Sandbox")
	plaidclient.SetEnvironment("Sandbox", plaid.Environment(srv.URL))
	defer plaidclient.SetEnvironment("Sandbox", orig)

	tok, err := createTransactionsLinkToken(context.Background(), "cid", "sec", "Sandbox", 730)
	if err != nil {
		t.Fatalf("createTransactionsLinkToken: %v", err)
	}
	if tok != "link-sandbox-123" {
		t.Fatalf("token %q", tok)
	}
	txn, _ := got["transactions"].(map[string]any)
	if txn["days_requested"] != float64(730) {
		t.Fatalf("days_requested not sent: %v", got["transactions"])
	}
	if p, _ := got["products"].([]any); len(p) != 1 || p[0] != "transactions" {
		t.Fatalf("products %v", got["products"])
	}
}

func TestCreateTransactionsLinkTokenSurfacesPlaidError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_code":"INVALID_FIELD","error_message":"bad"}`))
	}))
	defer srv.Close()
	orig, _ := plaidclient.Environment("Sandbox")
	plaidclient.SetEnvironment("Sandbox", plaid.Environment(srv.URL))
	defer plaidclient.SetEnvironment("Sandbox", orig)

	if _, err := createTransactionsLinkToken(context.Background(), "cid", "sec", "Sandbox", 730); err == nil {
		t.Fatal("expected error")
	}
}
