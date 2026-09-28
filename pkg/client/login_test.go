package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNanit stands in for Nanit's /login: a code is needed unless the
// request carries mfa_code "0123", and alice@example.com has no 2FA
func fakeNanit(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/login", r.URL.Path)
		assert.Equal(t, "1", r.Header.Get("nanit-api-version"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		switch {
		case body["password"] != "secret":
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		case body["email"] == "alice@example.com":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"access_token": "access", "refresh_token": "refresh"})
		case body["mfa_code"] == "0123":
			assert.Equal(t, "mfa-token", body["mfa_token"], "the challenge's token comes back")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"access_token": "access", "refresh_token": "refresh"})
		case body["mfa_code"] != "":
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid code"})
		default:
			w.WriteHeader(loginStatusMFARequired)
			json.NewEncoder(w).Encode(map[string]string{"mfa_token": "mfa-token", "channel": "sms", "phone_suffix": "42"})
		}
	}))
	t.Cleanup(server.Close)

	old := apiBaseURL
	apiBaseURL = server.URL
	t.Cleanup(func() { apiBaseURL = old })
}

func newTestClient(t *testing.T) *NanitClient {
	store := session.NewSessionStore()
	store.Filename = filepath.Join(t.TempDir(), "session.json")
	return &NanitClient{SessionStore: store}
}

// The dashboard's sign-in used to store the MFA token as the access token,
// and write the session file itself
func TestLoginWithCodeStoresTheTokens(t *testing.T) {
	fakeNanit(t)
	c := newTestClient(t)

	challenge, err := c.StartLogin("bob@example.com", "secret")
	require.NoError(t, err)
	assert.Equal(t, &LoginChallenge{MFAToken: "mfa-token", Channel: "sms", PhoneSuffix: "42"}, challenge)
	assert.Empty(t, c.SessionStore.AuthToken(), "nothing is stored until the code is in")

	require.NoError(t, c.FinishLogin("bob@example.com", "secret", challenge.MFAToken, "0123", challenge.Channel))
	assert.Equal(t, "access", c.SessionStore.AuthToken())
	assert.Equal(t, "refresh", c.SessionStore.RefreshToken())

	saved, err := session.InitSessionStore(c.SessionStore.Filename)
	require.NoError(t, err)
	assert.Equal(t, "refresh", saved.RefreshToken(), "the session is saved")
}

// Nanit answers 201 straight away for an account without 2FA. That used to
// be treated as a code request with no token, which the dashboard rejected.
func TestLoginWithout2FASignsInAtOnce(t *testing.T) {
	fakeNanit(t)
	c := newTestClient(t)

	challenge, err := c.StartLogin("alice@example.com", "secret")
	require.NoError(t, err)
	assert.Nil(t, challenge)
	assert.Equal(t, "access", c.SessionStore.AuthToken())
}

func TestLoginRejectionsCarryNanitsReason(t *testing.T) {
	fakeNanit(t)
	c := newTestClient(t)

	_, err := c.StartLogin("bob@example.com", "wrong")
	var rejected *LoginError
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, "Invalid credentials", rejected.Message)

	err = c.FinishLogin("bob@example.com", "secret", "mfa-token", "9999", "sms")
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, "Invalid code", rejected.Message)
	assert.Empty(t, c.SessionStore.RefreshToken())
}
