package link

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestTokenServer(t *testing.T) {
	ts := newTokenServer("link-token")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseURL, err := ts.start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer ts.shutdown(context.Background())

	httpClient := http.Client{Timeout: 2 * time.Second}

	// Verify link token endpoint
	resp, err := httpClient.Get(baseURL + "/link-token")
	if err != nil {
		t.Fatalf("failed to request link-token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// Send public token
	_, err = httpClient.PostForm(baseURL+"/token", url.Values{"public_token": {"public"}})
	if err != nil {
		t.Fatalf("failed to POST token: %v", err)
	}

	token, err := ts.waitForToken(context.Background())
	if err != nil {
		t.Fatalf("waitForToken returned error: %v", err)
	}
	if token != "public" {
		t.Fatalf("expected token 'public', got %q", token)
	}
}
