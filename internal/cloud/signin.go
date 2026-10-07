package cloud

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/nonlabhq/r3v/internal/keyring"
)

// SignIn signs this computer in to service through the browser: open is
// given the address to show (the browser opens it); the person signs in
// there and confirms, and the browser comes back to a one-time address on
// this computer with a code, exchanged here for a session of the app's own
// (loopback redirect with PKCE). The session goes in the credential store.
// Waits up to 10 minutes unless ctx ends sooner.
func SignIn(ctx context.Context, service string, open func(address string) error) (*Me, error) {
	verifier := random(32)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state := random(16)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		w.Header().Set("content-type", "text/html; charset=utf-8")
		if q.Get("state") != state || q.Get("code") == "" {
			w.WriteHeader(400)
			io.WriteString(w, page("This sign-in wasn't started here", "Start signing in from R3V again."))
			return
		}
		io.WriteString(w, page("Signed in", "You can close this tab and go back to R3V."))
		select {
		case done <- result{code: q.Get("code")}:
		default:
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	start := fmt.Sprintf("%s/desktop/start?port=%d&state=%s&challenge=%s",
		service, ln.Addr().(*net.TCPAddr).Port, url.QueryEscape(state), url.QueryEscape(challenge))
	if err := open(start); err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}
	var res result
	select {
	case res = <-done:
	case <-ctx.Done():
		return nil, errors.New("signing in took too long, or was cancelled")
	}

	var tok struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := call(service, "", "POST", "/v1/desktop/token", map[string]string{"code": res.code, "verifier": verifier}, &tok); err != nil {
		return nil, err
	}
	if tok.Token == "" {
		return nil, errors.New("R3V-Cloud didn't give a session")
	}
	if err := keyring.Set(target(service), "R3V", tok.Token); err != nil {
		return nil, fmt.Errorf("keeping the session: %w", err)
	}
	return GetMe(service)
}

func random(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func page(title, text string) string {
	return `<!doctype html><meta charset="utf-8"><title>` + html.EscapeString(title) + ` · R3V</title>` +
		`<body style="font:16px/1.5 system-ui,sans-serif;max-width:26rem;margin:4rem auto;padding:0 1rem">` +
		`<h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(text) + `</p>`
}
