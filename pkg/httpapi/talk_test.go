package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/talk"
	"github.com/stretchr/testify/assert"
)

// Talking needs the RTMP server, which the camera plays from, and the
// camera connected; both are answered before the WebSocket opens
func TestTalkNeedsRTMPAndACamera(t *testing.T) {
	app, mux := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})

	get := func(path string) int {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", path, nil)))
		return w.Code
	}

	assert.Equal(t, http.StatusServiceUnavailable, get("/api/talk/baby1"), "no RTMP")

	app.Talk = talk.New()
	app.Config.RTMPPublicAddr = "bridge:1935"
	assert.Equal(t, http.StatusServiceUnavailable, get("/api/talk/baby1"), "no camera")
	assert.Equal(t, http.StatusNotFound, get("/api/talk/unknown"))
}
