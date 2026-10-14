// Package oidctest - a fake OpenID Connect provider for tests: discovery,
// keys, and a token endpoint that checks the PKCE verifier and signs ID
// tokens.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Provider - the fake provider. Authorize stands in for the user signing in
// at it.
type Provider struct {
	*httptest.Server
	ClientID string
	// Subject - who signs in
	Subject string

	key   *rsa.PrivateKey
	mu    sync.Mutex
	codes map[string]grant
}

type grant struct {
	nonce, challenge, redirectURI string
}

// New - a provider for clientID, closed when the test ends
func New(t *testing.T, clientID string) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{ClientID: clientID, Subject: "user-1", key: key, codes: map[string]grant{}}
	mux := http.NewServeMux()
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Close)

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                p.URL,
			"authorization_endpoint":                p.URL + "/authorize",
			"token_endpoint":                        p.URL + "/token",
			"jwks_uri":                              p.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.mu.Lock()
		g, ok := p.codes[r.Form.Get("code")]
		delete(p.codes, r.Form.Get("code"))
		p.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge || r.Form.Get("redirect_uri") != g.redirectURI {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 3600,
			"id_token": p.idToken(t, g.nonce),
		})
	})
	return p
}

func (p *Provider) idToken(t *testing.T, nonce string) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: p.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	token, err := jwt.Signed(signer).Claims(map[string]any{
		"iss": p.URL, "sub": p.Subject, "aud": p.ClientID, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"email": "parent@example.com", "name": "A Parent",
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// Authorize signs the user in at authURL, as the browser would, and
// returns the callback URL the provider sends it back to
func (p *Provider) Authorize(t *testing.T, authURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != p.ClientID || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected authorization request %s", authURL)
	}
	code := "code-" + q.Get("state")[:8]
	p.mu.Lock()
	p.codes[code] = grant{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), redirectURI: q.Get("redirect_uri")}
	p.mu.Unlock()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		t.Fatal(err)
	}
	back.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	return back
}
