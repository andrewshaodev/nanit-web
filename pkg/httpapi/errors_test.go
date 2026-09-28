package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every error under /api/ is the same JSON shape, the mux's own 404 and 405
// included, so the dashboard can always show why a request failed
func TestAPIErrorsAreJSON(t *testing.T) {
	app, mux := newTestServer(t)

	for _, tc := range []struct {
		method, path string
		want         int
		code         string
	}{
		{"GET", "/api/nope", http.StatusNotFound, "not_found"},
		{"DELETE", "/api/status", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"GET", "/api/device-info/unknown", http.StatusNotFound, "unknown_baby"},
		{"POST", "/api/control/standby", http.StatusBadRequest, "invalid_json"},
		{"GET", "/api/history/sensor/unknown", http.StatusNotFound, "unknown_baby"},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest(tc.method, tc.path, nil)))
		assert.Equal(t, tc.want, w.Code, "%s %s", tc.method, tc.path)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "%s %s", tc.method, tc.path)

		var body apitypes.ErrorResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "%s %s: %s", tc.method, tc.path, w.Body.String())
		assert.Equal(t, tc.code, body.Error, "%s %s", tc.method, tc.path)
		assert.NotEmpty(t, body.Message)
	}
}

// Signed out, the API says so before it says anything about the route
func TestSignedOutGets401BeforeAnythingElse(t *testing.T) {
	_, mux := newTestServer(t)
	for _, req := range []*http.Request{
		httptest.NewRequest("DELETE", "/api/status", nil),
		httptest.NewRequest("GET", "/api/nope", nil),
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "%s %s", req.Method, req.URL.Path)
	}
}

// Reached over HTTPS (here, through a proxy), the session cookie is only
// sent back over HTTPS
func TestSessionCookieIsSecureOverHTTPS(t *testing.T) {
	_, mux := newTestServer(t)
	for _, https := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/api/webauth/login", strings.NewReader(`{"password":"correct horse battery staple"}`))
		if https {
			req.Header.Set("X-Forwarded-Proto", "https")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, https, cookies[0].Secure, "https=%v", https)
		assert.True(t, cookies[0].HttpOnly)
	}
}

// The page itself is only ever fetched
func TestIndexOnlyTakesGET(t *testing.T) {
	_, mux := newTestServer(t)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/settings", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
