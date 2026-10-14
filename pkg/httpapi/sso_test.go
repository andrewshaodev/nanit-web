package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/oidcauth"
	"github.com/andrewshaodev/nanit-web/pkg/oidcauth/oidctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSSOServer - a test server with SSO available, no dashboard password,
// and a provider
func newSSOServer(t *testing.T) (*Server, http.Handler, *oidctest.Provider) {
	t.Helper()
	idp := oidctest.New(t, "nanit")
	app, _ := newTestServer(t)
	require.NoError(t, app.WebAuth.RemovePassword())
	app.SSO = oidcauth.Open(filepath.Join(t.TempDir(), "sso.json"))
	return app, app.Handler(), idp
}

func cookieNamed(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func serve(mux http.Handler, req *http.Request, cookies ...*http.Cookie) *http.Response {
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Host = "monitor.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w.Result()
}

func body(res *http.Response) string {
	var b bytes.Buffer
	_, _ = b.ReadFrom(res.Body)
	return b.String()
}

// Set up from Settings: saved as a draft, then turned on by a test sign-in,
// after which the dashboard needs signing in
func TestSSOSetUpFromSettings(t *testing.T) {
	_, mux, idp := newSSOServer(t)

	res := serve(mux, httptest.NewRequest("GET", "/api/settings/sso", nil))
	assert.JSONEq(t, `{"enabled":false,"active":null,"draft":{"issuer":"","client_id":""},"draft_secret_set":false,
		"callback_url":"https://monitor.example.com/api/webauth/oidc/callback"}`, body(res))

	// A provider that isn't there is refused
	res = serve(mux, httptest.NewRequest("PUT", "/api/settings/sso", bytes.NewBufferString(`{"issuer":"http://127.0.0.1:1","client_id":"nanit"}`)))
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)

	res = serve(mux, httptest.NewRequest("PUT", "/api/settings/sso", bytes.NewBufferString(`{"issuer":"`+idp.URL+`","client_id":"nanit","client_secret":"s"}`)))
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, body(res), `"draft_secret_set":true`)
	assert.Equal(t, http.StatusOK, serve(mux, httptest.NewRequest("GET", "/api/status", nil)).StatusCode, "still open: only a draft")

	// The test sign-in
	res = serve(mux, httptest.NewRequest("GET", "/api/settings/sso/test", nil))
	require.Equal(t, http.StatusFound, res.StatusCode)
	stateCookie := cookieNamed(res, ssoStateCookie)
	require.NotNil(t, stateCookie)
	assert.True(t, stateCookie.Secure)
	back := idp.Authorize(t, res.Header.Get("Location"))
	assert.Equal(t, "https://monitor.example.com"+oidcauth.CallbackPath, back.Scheme+"://"+back.Host+back.Path)
	res = serve(mux, httptest.NewRequest("GET", back.Path+"?"+back.RawQuery, nil), stateCookie)
	require.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, "/settings?sso=on", res.Header.Get("Location"))
	session := cookieNamed(res, sessionCookie)
	require.NotNil(t, session, "the tester stays signed in")

	// On: the API needs signing in, and the session works
	assert.Equal(t, http.StatusUnauthorized, serve(mux, httptest.NewRequest("GET", "/api/status", nil)).StatusCode)
	assert.Contains(t, body(serve(mux, httptest.NewRequest("GET", "/api/webauth/status", nil))), `"sso_enabled":true`)
	assert.Equal(t, http.StatusOK, serve(mux, httptest.NewRequest("GET", "/api/status", nil), session).StatusCode)
	// Changing settings and testing need signing in now
	assert.Equal(t, http.StatusUnauthorized, serve(mux, httptest.NewRequest("GET", "/api/settings/sso/test", nil)).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, serve(mux, httptest.NewRequest("DELETE", "/api/settings/sso", nil)).StatusCode)

	// An ordinary sign-in, in another browser, back to where it started;
	// the callback in a browser without the state cookie is refused
	res = serve(mux, httptest.NewRequest("GET", "/api/webauth/oidc/login?return_to=/settings", nil))
	stateCookie = cookieNamed(res, ssoStateCookie)
	back = idp.Authorize(t, res.Header.Get("Location"))
	refused := serve(mux, httptest.NewRequest("GET", back.Path+"?"+back.RawQuery, nil))
	assert.Equal(t, "/?sso_error=failed", refused.Header.Get("Location"))
	assert.Nil(t, cookieNamed(refused, sessionCookie))
	res = serve(mux, httptest.NewRequest("GET", back.Path+"?"+back.RawQuery, nil), stateCookie)
	assert.Equal(t, "/settings", res.Header.Get("Location"))
	require.NotNil(t, cookieNamed(res, sessionCookie))

	// Off again
	res = serve(mux, httptest.NewRequest("DELETE", "/api/settings/sso", nil), session)
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, http.StatusOK, serve(mux, httptest.NewRequest("GET", "/api/status", nil)).StatusCode)
}

func TestSSORefusedByProvider(t *testing.T) {
	app, mux, idp := newSSOServer(t)
	require.NoError(t, app.SSO.SaveDraft(t.Context(), oidcauth.Config{Issuer: idp.URL, ClientID: "nanit"}))
	res := serve(mux, httptest.NewRequest("GET", "/api/settings/sso/test", nil))
	stateCookie := cookieNamed(res, ssoStateCookie)
	back := idp.Authorize(t, res.Header.Get("Location"))
	res = serve(mux, httptest.NewRequest("GET", back.Path+"?error=access_denied&state="+back.Query().Get("state"), nil), stateCookie)
	assert.Equal(t, "/settings?sso_error=denied", res.Header.Get("Location"))
	assert.False(t, app.SSO.Enabled())
}

func TestSSOLoginWhileOff(t *testing.T) {
	_, mux, _ := newSSOServer(t)
	assert.Equal(t, http.StatusNotFound, serve(mux, httptest.NewRequest("GET", "/api/webauth/oidc/login", nil)).StatusCode)
}

func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{
		"/settings":            "/settings",
		"/?a=b":                "/?a=b",
		"":                     "/",
		"settings":             "/",
		"//evil.example":       "/",
		"/\\evil.example":      "/",
		"https://evil.example": "/",
	} {
		assert.Equal(t, want, localPath(in), in)
	}
	assert.Equal(t, "/?a=b&sso_error=x", ssoErrorURL("/?a=b", "x"))
}
