package plaidclient

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/plaid/plaid-go/plaid"
)

func TestSanitizeUnknownTransactionCode(t *testing.T) {
	in := []byte(`{"added":[{"transaction_id":"a","amount":12.34,"transaction_code":"payment"},{"transaction_id":"b","amount":1,"transaction_code":"purchase"}],"next_cursor":"c"}`)
	out := SanitizeJSON(in)
	var resp plaid.TransactionsSyncResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("SDK still rejects sanitized body: %v", err)
	}
	if resp.Added[0].TransactionCode.IsSet() && resp.Added[0].TransactionCode.Get() != nil {
		t.Fatalf("unknown code not nulled")
	}
	if got := resp.Added[1].TransactionCode.Get(); got == nil || *got != plaid.TRANSACTIONCODE_PURCHASE {
		t.Fatalf("known code changed: %v", got)
	}
	if !strings.Contains(string(out), "12.34") {
		t.Fatalf("amount changed: %s", out)
	}
}

func TestSanitizeLeavesCleanBodyUntouched(t *testing.T) {
	in := []byte(`{"amount": 1.10, "transaction_code": null}`)
	if string(SanitizeJSON(in)) != string(in) {
		t.Fatal("clean body was rewritten")
	}
}
