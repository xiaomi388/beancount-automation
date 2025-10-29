package link

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
)

//go:embed static/link.html
var linkPage []byte

type tokenServer struct {
	srv       *http.Server
	mux       *http.ServeMux
	linkToken string
	tokenMu   sync.Mutex
	token     string
	done      chan struct{}
}

func newTokenServer(linkToken string) *tokenServer {
	ts := &tokenServer{
		linkToken: linkToken,
		mux:       http.NewServeMux(),
		done:      make(chan struct{}),
	}

	ts.mux.HandleFunc("/token", ts.handleToken)
	ts.mux.HandleFunc("/link-token", ts.handleLinkToken)
	ts.mux.HandleFunc("/", ts.handleLink)

	ts.srv = &http.Server{Handler: ts.mux}
	return ts
}

func (ts *tokenServer) start(ctx context.Context) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		if err := ts.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Println("token server error:", err)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = ts.shutdown(context.Background())
	}()

	return "http://" + ln.Addr().String(), nil
}

func (ts *tokenServer) shutdown(ctx context.Context) error {
	return ts.srv.Shutdown(ctx)
}

func (ts *tokenServer) waitForToken(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-ts.done:
		ts.tokenMu.Lock()
		defer ts.tokenMu.Unlock()
		return ts.token, nil
	}
}

func (ts *tokenServer) handleLink(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/link" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(linkPage)
}

func (ts *tokenServer) handleLinkToken(w http.ResponseWriter, r *http.Request) {
	type response struct {
		LinkToken string `json:"link_token"`
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{LinkToken: ts.linkToken})
}

func (ts *tokenServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("public_token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	ts.tokenMu.Lock()
	ts.token = token
	ts.tokenMu.Unlock()

	select {
	case <-ts.done:
	default:
		close(ts.done)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
