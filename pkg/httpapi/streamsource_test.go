package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/streamsource"
	"github.com/stretchr/testify/assert"
)

// Only a viewer has a stream server to set
func TestStreamSourceIsForViewers(t *testing.T) {
	app, mux := newTestServer(t)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", "/api/streaming/source", nil)))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestStreamSourceSet(t *testing.T) {
	app, _ := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1", Name: "Baby"}})
	app.StreamSource = streamsource.Open(filepath.Join(t.TempDir(), "stream_source.json"))
	mux := app.Handler()

	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("PUT", "/api/streaming/source", strings.NewReader(body))))
		return w
	}
	w := put(`{"host":"rtsp://monitor.homelab.house:8554","port":0,"key":""}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = put(`{"host":"monitor.homelab.house","port":0,"key":"KEY"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"port":8554`)
	assert.Contains(t, w.Body.String(), `"baby_uid":"baby1"`)

	// The info shows the address with the key masked
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", "/api/streaming/info", nil)))
	assert.Contains(t, w.Body.String(), `"stream_from":"rtsp://nanit:xxxxx@monitor.homelab.house:8554/local/{baby_uid}"`)
	assert.NotContains(t, w.Body.String(), "KEY")
}
