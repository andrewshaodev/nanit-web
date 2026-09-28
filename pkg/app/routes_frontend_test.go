package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/session"
	"github.com/andrewshaodev/nanit-web/pkg/streaming"
	"github.com/andrewshaodev/nanit-web/pkg/webauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every request frontend/src/lib/api.ts makes, spelled exactly as it builds
// them. Keep this in step with api.ts: it is what guards the dashboard when
// routes change.
var frontendRequests = []struct{ method, path string }{
	{"GET", "/api/status"},
	{"GET", "/api/device-info/baby1"},
	{"GET", "/api/history/sensor/baby1?start=0&end=1&limit=10"},
	{"GET", "/api/history/summary/baby1?start=0&end=1"},
	{"GET", "/api/history/day-night/baby1?start=0&end=1"},
	{"DELETE", "/api/history/reset/baby1"},
	{"POST", "/api/control/night-light"},
	{"POST", "/api/control/standby"},
	{"POST", "/api/auth/login"},
	{"POST", "/api/auth/verify-2fa"},
	{"GET", "/api/auth/status"},
	{"DELETE", "/api/auth/reset"},
	{"POST", "/api/stream/start/"},
	{"POST", "/api/stream/stop/"},
	{"GET", "/api/stream/status/baby1"},
	{"GET", "/api/health/baby1"},
	{"GET", "/api/streaming/info"},
	{"GET", "/api/stream/hls/baby1/playlist.m3u8"},
	{"GET", "/api/sound/baby1"},
	{"POST", "/api/sound/baby1/play"},
	{"POST", "/api/sound/baby1/stop"},
	{"POST", "/api/sound/baby1/volume"},
	{"GET", "/api/webauth/status"},
	{"POST", "/api/webauth/login"},
	{"POST", "/api/webauth/logout"},
	{"POST", "/api/webauth/set-password"},
	{"POST", "/api/webauth/change-password"},
	{"POST", "/api/webauth/remove-password"},
}

// Public: the page has to load these before anyone has signed in
var publicRequests = map[string]bool{
	"/api/webauth/status": true,
	"/api/webauth/login":  true,
	"/api/webauth/logout": true,
}

// newTestServer is the app with a dashboard password, no cameras connected
// and history off, with its routes on a fresh mux. Files go in a temp dir.
func newTestServer(t *testing.T) (*App, *http.ServeMux) {
	t.Helper()
	dir := t.TempDir()
	wa := webauth.NewWebAuth(filepath.Join(dir, "password.json"))
	require.NoError(t, wa.SetPassword("correct horse battery staple"))
	tracker, err := history.NewTracker(dir, false)
	require.NoError(t, err)

	app := &App{
		Opts:             Opts{SessionFile: filepath.Join(dir, "session.json")},
		SessionStore:     session.NewSessionStore(),
		BabyStateManager: baby.NewStateManager(),
		HLSManager:       streaming.NewHLSManager(filepath.Join(dir, "hls")),
		HistoryTracker:   tracker,
		WebAuth:          wa,
	}
	mux := http.NewServeMux()
	setupAPIRoutes(mux, nil, app.BabyStateManager, app)
	return app, mux
}

// signedIn adds a valid dashboard session to r
func signedIn(t *testing.T, app *App, r *http.Request) *http.Request {
	t.Helper()
	session, err := app.WebAuth.CreateSession()
	require.NoError(t, err)
	r.AddCookie(&http.Cookie{Name: "nanit_session", Value: session})
	return r
}

// Each request reaches its handler with the method the frontend uses. The
// requests have no body and the app has no cameras, so the handlers answer
// 400/404/503, but never 405 or the mux's own 404.
func TestFrontendRequestsAreRouted(t *testing.T) {
	app, mux := newTestServer(t)

	for _, req := range frontendRequests {
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			// Signed out: 401, apart from the public routes
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(req.method, req.path, nil))
			if publicRequests[req.path] {
				assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}

			// Signed in: routed, with the method accepted
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest(req.method, req.path, nil)))
			assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code, w.Body.String())
			assert.NotEqual(t, "404 page not found\n", w.Body.String())
		})
	}
}
