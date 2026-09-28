package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
)

// Sound commands need one of the account's cameras, connected
func TestSoundAPIRequiresAConnectedCamera(t *testing.T) {
	app, mux := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})

	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/sound/baby1", "", http.StatusServiceUnavailable},
		{"POST", "/api/sound/baby1/play", `{"track":"White Noise.wav"}`, http.StatusServiceUnavailable},
		{"POST", "/api/sound/baby1/play", `{}`, http.StatusBadRequest},
		{"GET", "/api/sound/unknown", "", http.StatusNotFound},
		{"GET", "/api/sound/baby1/play/extra", "", http.StatusNotFound},
		{"GET", "/api/sound/baby1/play", "", http.StatusMethodNotAllowed},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))))
		assert.Equal(t, tc.want, w.Code, "%s %s", tc.method, tc.path)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "%s %s", tc.method, tc.path)
	}
}
