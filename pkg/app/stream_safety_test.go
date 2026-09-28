package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
)

// Stream start and stop take the UID from the request body, and it names a
// folder that starting a stream empties. Only the account's cameras are
// accepted.
func TestStreamRequestsNeedAKnownBaby(t *testing.T) {
	app, mux := newTestServer(t)
	app.SessionStore.SetBabies([]baby.Baby{{UID: "baby1", Name: "Baby"}})

	for _, path := range []string{"/api/stream/start/", "/api/stream/stop/"} {
		for _, uid := range []string{"..", "../history", "unknown"} {
			body := strings.NewReader(`{"baby_uid":"` + uid + `"}`)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("POST", path, body)))
			assert.Equal(t, http.StatusNotFound, w.Code, "%s %s", path, uid)
		}
	}
	_, started := app.HLSManager.GetTranscoder("..")
	assert.False(t, started)
}

// Only the playlist and segments ffmpeg writes are served
func TestHLSServesOnlyStreamFiles(t *testing.T) {
	app, mux := newTestServer(t)
	for _, name := range []string{"session.json", "history.db", "playlist.m3u8.bak", "segment_x.ts"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", "/api/stream/hls/baby1/"+name, nil)))
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
}
