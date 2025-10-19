package sync

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func TestSyncUpdatesTransactionInstitutionIntegration(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}

	server := newPlaidTestServer(t, origDir)
	defer server.Close()

	origEnv, ok := plaidclient.Environment("Sandbox")
	plaidclient.SetEnvironment("Sandbox", plaid.Environment(server.URL))
	t.Cleanup(func() {
		if ok {
			plaidclient.SetEnvironment("Sandbox", origEnv)
		}
	})

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	copySyncTestFile(t, filepath.Join(origDir, "testdata", "config.yaml"), filepath.Join(tempDir, "config.yaml"))
	copySyncTestFile(t, filepath.Join(origDir, "testdata", "owners.yaml"), filepath.Join(tempDir, "owners.yaml"))

	if err := Sync(); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	ownersData, err := os.ReadFile(filepath.Join(tempDir, "owners.yaml"))
	if err != nil {
		t.Fatalf("failed to read owners: %v", err)
	}

	goldenPath := filepath.Join(origDir, "testdata", "owners_after_sync.json")
	if os.Getenv("UPDATE_SYNC_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, ownersData, 0644); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
	}
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if !jsonEqual(ownersData, goldenData) {
		t.Fatalf("owners.yaml mismatch golden file\n got: %s\nwant: %s", ownersData, goldenData)
	}

	var owners []types.Owner
	if err := json.Unmarshal(ownersData, &owners); err != nil {
		t.Fatalf("failed to unmarshal owners: %v", err)
	}

	if len(owners) != 1 {
		t.Fatalf("expected 1 owner, got %d", len(owners))
	}

	inst, ok := owners[0].TransactionInstitution("mock-bank")
	if !ok {
		t.Fatalf("expected mock-bank institution to exist")
	}
	if inst.InstitutionBase.Cursor != "cursor-1" {
		t.Fatalf("expected cursor updated to cursor-1, got %s", inst.InstitutionBase.Cursor)
	}

	account, ok := inst.TransactionAccount("account-1")
	if !ok {
		t.Fatalf("expected account-1 to exist")
	}

	txn, ok := account.Transactions["txn-1"]
	if !ok {
		t.Fatalf("expected txn-1 to exist")
	}
	if txn.Name != "Coffee Shop" {
		t.Fatalf("unexpected transaction name: %s", txn.Name)
	}
	if txn.Amount != 4.99 {
		t.Fatalf("unexpected transaction amount: %v", txn.Amount)
	}

	invInst, ok := owners[0].InvestmentInstitution("mock-invest")
	if !ok {
		t.Fatalf("expected mock-invest institution to exist")
	}
	invAccount, ok := invInst.InvestmentAccount("invest-account-1")
	if !ok {
		t.Fatalf("expected invest-account-1 to exist")
	}
	if len(invAccount.Holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(invAccount.Holdings))
	}
	holding := invAccount.Holdings[0]
	if holding.SecurityId != "security-1" {
		t.Fatalf("unexpected holding security id: %s", holding.SecurityId)
	}
	if holding.Quantity != 10.5 {
		t.Fatalf("unexpected holding quantity: %v", holding.Quantity)
	}
	if invAccount.Securities == nil {
		t.Fatalf("expected securities map to be populated")
	}
	ticker := invAccount.Securities["security-1"].TickerSymbol.Get()
	if ticker == nil || *ticker != "TGF" {
		t.Fatalf("expected ticker TGF, got %+v", invAccount.Securities["security-1"].TickerSymbol)
	}
	if len(invAccount.Transactions) != 1 {
		t.Fatalf("expected 1 investment txn, got %d", len(invAccount.Transactions))
	}
	invTxn, ok := invAccount.Transactions["inv-txn-1"]
	if !ok {
		t.Fatalf("expected inv-txn-1 to exist")
	}
	if invTxn.Amount != -50.75 {
		t.Fatalf("unexpected investment transaction amount: %v", invTxn.Amount)
	}
}

func copySyncTestFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read testdata %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}

func newPlaidTestServer(t *testing.T, baseDir string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/accounts/get", func(w http.ResponseWriter, r *http.Request) {
		ensureMethod(t, r, http.MethodPost)
		ensureRequestBody(t, r.Body, filepath.Join(baseDir, "testdata", "server_accounts_request.json"))
		w.Header().Set("Content-Type", "application/json")
		serveJSON(t, w, filepath.Join(baseDir, "testdata", "server_accounts_response.json"))
	})

	mux.HandleFunc("/transactions/sync", func(w http.ResponseWriter, r *http.Request) {
		ensureMethod(t, r, http.MethodPost)
		ensureRequestBody(t, r.Body, filepath.Join(baseDir, "testdata", "server_transactions_request.json"))
		w.Header().Set("Content-Type", "application/json")
		serveJSON(t, w, filepath.Join(baseDir, "testdata", "server_transactions_response.json"))
	})

	mux.HandleFunc("/investments/holdings/get", func(w http.ResponseWriter, r *http.Request) {
		ensureMethod(t, r, http.MethodPost)
		ensureRequestBody(t, r.Body, filepath.Join(baseDir, "testdata", "server_investment_holdings_request.json"))
		w.Header().Set("Content-Type", "application/json")
		serveJSON(t, w, filepath.Join(baseDir, "testdata", "server_investment_holdings_response.json"))
	})

	mux.HandleFunc("/investments/transactions/get", func(w http.ResponseWriter, r *http.Request) {
		ensureMethod(t, r, http.MethodPost)
		ensureRequestBody(t, r.Body, filepath.Join(baseDir, "testdata", "server_investment_transactions_request.json"))
		w.Header().Set("Content-Type", "application/json")
		serveJSON(t, w, filepath.Join(baseDir, "testdata", "server_investment_transactions_response.json"))
	})

	return httptest.NewServer(mux)
}

func ensureMethod(t *testing.T, r *http.Request, method string) {
	t.Helper()
	if r.Method != method {
		t.Fatalf("unexpected method %s", r.Method)
	}
}

func ensureRequestBody(t *testing.T, body io.ReadCloser, expectedPath string) {
	t.Helper()
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	expected, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to load expected payload %s: %v", expectedPath, err)
	}
	if !jsonContains(raw, expected) {
		t.Fatalf("unexpected request payload:\n got: %s\nwant subset of: %s", raw, expected)
	}
}

func serveJSON(t *testing.T, w http.ResponseWriter, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	_, _ = w.Write(data)
}

func jsonContains(actualRaw, expectedRaw []byte) bool {
	var actual interface{}
	if err := json.Unmarshal(actualRaw, &actual); err != nil {
		return false
	}
	var expected interface{}
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		return false
	}
	return contains(actual, expected)
}

func contains(actual, expected interface{}) bool {
	switch exp := expected.(type) {
	case map[string]interface{}:
		actMap, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for key, expVal := range exp {
			actVal, ok := actMap[key]
			if !ok || !contains(actVal, expVal) {
				return false
			}
		}
		return true
	case []interface{}:
		actSlice, ok := actual.([]interface{})
		if !ok || len(actSlice) != len(exp) {
			return false
		}
		for i := range exp {
			if !contains(actSlice[i], exp[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(actual, expected)
	}
}

func jsonEqual(leftRaw, rightRaw []byte) bool {
	var left interface{}
	if err := json.Unmarshal(leftRaw, &left); err != nil {
		return false
	}
	var right interface{}
	if err := json.Unmarshal(rightRaw, &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}
