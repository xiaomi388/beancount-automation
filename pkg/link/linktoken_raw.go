package link

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/plaidclient"
)

// maxTransactionsDaysRequested is the longest history Plaid Transactions allows.
const maxTransactionsDaysRequested = 730

// createTransactionsLinkToken calls /link/token/create directly because the
// pinned plaid-go version has no transactions.days_requested field. Without it
// Plaid only backfills 90 days for a new Item.
func createTransactionsLinkToken(ctx context.Context, clientID, secret, env string, daysRequested int) (string, error) {
	base := string(plaid.Environment(env))
	if e, ok := plaidclient.Environment(env); ok {
		base = string(e)
	}

	body := map[string]any{
		"client_id":     clientID,
		"secret":        secret,
		"client_name":   "Beancount Automation",
		"language":      "en",
		"country_codes": []string{"US"},
		"user":          map[string]string{"client_user_id": "USERID"},
		"products":      []string{string(plaid.PRODUCTS_TRANSACTIONS)},
		"transactions":  map[string]int{"days_requested": daysRequested},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/link/token/create", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("link token request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		LinkToken    string `json:"link_token"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode link token response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || out.LinkToken == "" {
		return "", fmt.Errorf("link token create failed (HTTP %d): %s %s", resp.StatusCode, out.ErrorCode, out.ErrorMessage)
	}
	return out.LinkToken, nil
}
