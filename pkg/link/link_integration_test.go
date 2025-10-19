package link

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func TestLinkAddsTransactionInstitutionIntegration(t *testing.T) {
	// Save originals to restore after test because we modify globals.
	origGetAccessToken := getAccessTokenFn

	getAccessTokenFn = func(clientID, secret, env string, product *plaid.Products) (string, error) {
		return "test-access-token", nil
	}

	t.Cleanup(func() {
		getAccessTokenFn = origGetAccessToken
	})

	tempDir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	copyTestFile(t, filepath.Join(cwd, "testdata", "config.yaml"), filepath.Join(tempDir, "config.yaml"))
	copyTestFile(t, filepath.Join(cwd, "testdata", "owners.yaml"), filepath.Join(tempDir, "owners.yaml"))

	if err := Link("alice", "chase", types.InstitutionTypeTransaction); err != nil {
		t.Fatalf("Link returned error: %v", err)
	}

	ownersData, err := os.ReadFile(filepath.Join(tempDir, "owners.yaml"))
	if err != nil {
		t.Fatalf("failed to read owners: %v", err)
	}

	var owners []types.Owner
	if err := json.Unmarshal(ownersData, &owners); err != nil {
		t.Fatalf("failed to unmarshal owners: %v", err)
	}

	if len(owners) != 1 {
		t.Fatalf("expected 1 owner, got %d", len(owners))
	}

	inst, ok := owners[0].TransactionInstitution("chase")
	if !ok {
		t.Fatalf("expected chase institution to exist")
	}
	if inst.InstitutionBase.AccessToken != "test-access-token" {
		t.Fatalf("unexpected access token: %s", inst.InstitutionBase.AccessToken)
	}
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read testdata %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}
