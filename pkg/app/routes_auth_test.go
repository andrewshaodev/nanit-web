package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/indiefan/home_assistant_nanit/pkg/baby"
	"github.com/indiefan/home_assistant_nanit/pkg/webauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With a dashboard password set, only what's needed to load the page and
// sign in is reachable without a session. Most of the API used to be open,
// including the live video, the camera controls, Nanit sign-in and reset,
// and history reset.
func TestRoutesRequireAuthWhenPasswordSet(t *testing.T) {
	dir := t.TempDir()
	wa := webauth.NewWebAuth(filepath.Join(dir, "password.json"))
	require.NoError(t, wa.SetPassword("correct horse battery staple"))

	app := &App{Opts: Opts{WebAuth: WebAuthOpts{Enabled: true}}, WebAuth: wa}
	// setupAPIRoutes registers on the default mux, so it can only run once
	setupAPIRoutes(nil, DataDirectories{VideoDir: dir, LogDir: dir}, baby.NewStateManager(), app)

	protected := []string{
		"/api/status", "/api/babies", "/api/streaming/info",
		"/api/control/night-light", "/api/control/standby", "/api/sound/baby1",
		"/api/device-info/baby1",
		"/api/auth/login", "/api/auth/verify-2fa", "/api/auth/status", "/api/auth/reset",
		"/api/webauth/set-password", "/api/webauth/change-password", "/api/webauth/remove-password",
		"/api/stream/hls/baby1/playlist.m3u8", "/api/stream/start/baby1", "/api/stream/stop/baby1", "/api/stream/status/baby1",
		"/api/history/sensor/baby1", "/api/history/events/baby1", "/api/history/summary/baby1",
		"/api/history/day-night/baby1", "/api/history/reset/baby1",
		"/api/health/baby1",
		"/video/anything.mp4",
	}
	for _, path := range protected {
		for _, method := range []string{"GET", "POST"} {
			w := httptest.NewRecorder()
			http.DefaultServeMux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			assert.Equal(t, http.StatusUnauthorized, w.Code, "%s %s should need a session", method, path)
		}
	}

	for _, path := range []string{"/health", "/api/webauth/status"} {
		w := httptest.NewRecorder()
		http.DefaultServeMux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "%s should be reachable signed out", path)
	}

	// /log wrote any POST body to disk; it's gone
	w := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(w, httptest.NewRequest("POST", "/log", nil))
	assert.NotEqual(t, http.StatusNoContent, w.Code)

	// And a signed-in session gets through
	session, err := wa.CreateSession()
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/streaming/info", nil)
	req.AddCookie(&http.Cookie{Name: "nanit_session", Value: session})
	w = httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
