package plaidclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/plaid/plaid-go/plaid"
)

// The pinned plaid-go (v1.10.0) rejects enum values Plaid added later, which
// aborts a whole /transactions/sync page. sanitizeTransport rewrites unknown
// values of such fields to null before the SDK decodes the response.
var enumFields = map[string]map[string]bool{
	"transaction_code": toSet(transactionCodes()),
}

func transactionCodes() []string {
	out := []string{}
	for _, v := range []plaid.TransactionCode{
		plaid.TRANSACTIONCODE_ADJUSTMENT, plaid.TRANSACTIONCODE_ATM, plaid.TRANSACTIONCODE_BANK_CHARGE,
		plaid.TRANSACTIONCODE_BILL_PAYMENT, plaid.TRANSACTIONCODE_CASH, plaid.TRANSACTIONCODE_CASHBACK,
		plaid.TRANSACTIONCODE_CHEQUE, plaid.TRANSACTIONCODE_DIRECT_DEBIT, plaid.TRANSACTIONCODE_INTEREST,
		plaid.TRANSACTIONCODE_PURCHASE, plaid.TRANSACTIONCODE_STANDING_ORDER, plaid.TRANSACTIONCODE_TRANSFER,
		plaid.TRANSACTIONCODE_NULL,
	} {
		out = append(out, string(v))
	}
	return out
}

func toSet(vals []string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

type sanitizeTransport struct{ base http.RoundTripper }

func (t sanitizeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return resp, nil
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(SanitizeJSON(raw)))
	return resp, nil
}

// SanitizeJSON nulls unknown enum values. It returns the input unchanged when
// nothing needs fixing or the body is not valid JSON.
func SanitizeJSON(raw []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keep numbers exactly as sent
	var v any
	if err := dec.Decode(&v); err != nil {
		return raw
	}
	if !walk(v) {
		return raw
	}
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

func walk(v any) bool {
	changed := false
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if allowed, ok := enumFields[k]; ok {
				if s, isStr := val.(string); isStr && !allowed[s] {
					x[k] = nil
					changed = true
					continue
				}
			}
			if walk(val) {
				changed = true
			}
		}
	case []any:
		for _, val := range x {
			if walk(val) {
				changed = true
			}
		}
	}
	return changed
}
