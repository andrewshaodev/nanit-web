package oidcauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/oidcauth/oidctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const redirect = "https://monitor.example.com" + CallbackPath

func TestValidate(t *testing.T) {
	for _, bad := range []Config{{Issuer: "id.example.com", ClientID: "c"}, {Issuer: "https://id.example.com"}, {Issuer: "ftp://x", ClientID: "c"}} {
		assert.Error(t, bad.Validate(), bad.Issuer)
	}
	c := Config{Issuer: " https://id.example.com/ ", ClientID: " c ", ClientSecret: " s "}
	require.NoError(t, c.Validate())
	assert.Equal(t, Config{Issuer: "https://id.example.com", ClientID: "c", ClientSecret: "s"}, c)
}

// A draft only signs people in once a test sign-in with it has worked
func TestTestSignInTurnsOn(t *testing.T) {
	idp := oidctest.New(t, "nanit")
	path := filepath.Join(t.TempDir(), "sso.json")
	s := Open(path)
	assert.False(t, s.Enabled())
	_, _, err := s.Start(context.Background(), redirect, "/", false)
	assert.Error(t, err, "nothing to sign in with yet")

	require.NoError(t, s.SaveDraft(context.Background(), Config{Issuer: idp.URL + "/", ClientID: "nanit", ClientSecret: "secret"}))
	assert.False(t, s.Enabled(), "a draft isn't on")
	active, draft, secretSet := s.Settings()
	assert.Nil(t, active)
	assert.Equal(t, Config{Issuer: idp.URL, ClientID: "nanit"}, draft, "the secret isn't given out")
	assert.True(t, secretSet)

	authURL, state, err := s.Start(context.Background(), redirect, "/settings", true)
	require.NoError(t, err)
	back := idp.Authorize(t, authURL)
	res, err := s.Finish(context.Background(), state, back.Query().Get("code"))
	require.NoError(t, err)
	assert.True(t, res.Test)
	assert.Equal(t, Identity{Subject: "user-1", Email: "parent@example.com", Name: "A Parent"}, res.Identity)
	assert.True(t, s.Enabled())
	assert.True(t, Open(path).Enabled(), "kept through a restart")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Now an ordinary sign-in works, once per state
	authURL, state, err = s.Start(context.Background(), redirect, "/settings", false)
	require.NoError(t, err)
	back = idp.Authorize(t, authURL)
	res, err = s.Finish(context.Background(), state, back.Query().Get("code"))
	require.NoError(t, err)
	assert.False(t, res.Test)
	assert.Equal(t, "/settings", res.ReturnTo)
	_, err = s.Finish(context.Background(), state, back.Query().Get("code"))
	assert.Error(t, err)

	// Off, keeping the draft; a blank secret keeps the saved one
	require.NoError(t, s.Disable())
	assert.False(t, s.Enabled())
	require.NoError(t, s.SaveDraft(context.Background(), Config{Issuer: idp.URL, ClientID: "nanit"}))
	_, _, secretSet = s.Settings()
	assert.True(t, secretSet)
}

// A test that fails leaves SSO as it was
func TestFailedTestLeavesItOff(t *testing.T) {
	idp := oidctest.New(t, "nanit")
	s := Open(filepath.Join(t.TempDir(), "sso.json"))
	require.NoError(t, s.SaveDraft(context.Background(), Config{Issuer: idp.URL, ClientID: "nanit"}))
	_, state, err := s.Start(context.Background(), redirect, "/settings", true)
	require.NoError(t, err)
	res, err := s.Finish(context.Background(), state, "wrong-code")
	assert.Error(t, err)
	assert.True(t, res.Test)
	assert.False(t, s.Enabled())
}

// A code for one sign-in can't finish another: the PKCE verifier and nonce
// are the other sign-in's
func TestCodeFromAnotherSignIn(t *testing.T) {
	idp := oidctest.New(t, "nanit")
	s := Open(filepath.Join(t.TempDir(), "sso.json"))
	require.NoError(t, s.SaveDraft(context.Background(), Config{Issuer: idp.URL, ClientID: "nanit"}))
	firstURL, _, err := s.Start(context.Background(), redirect, "/", true)
	require.NoError(t, err)
	_, second, err := s.Start(context.Background(), redirect, "/", true)
	require.NoError(t, err)
	_, err = s.Finish(context.Background(), second, idp.Authorize(t, firstURL).Query().Get("code"))
	assert.Error(t, err)
	assert.False(t, s.Enabled())
}

func TestSaveDraftChecksTheProvider(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "sso.json"))
	err := s.SaveDraft(context.Background(), Config{Issuer: "http://127.0.0.1:1", ClientID: "nanit"})
	assert.ErrorContains(t, err, "couldn't reach")
}
