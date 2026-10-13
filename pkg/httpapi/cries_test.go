package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/frigate"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCriesWithoutFrigate(t *testing.T) {
	app, mux := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", "/api/cries/baby1", nil)))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
	assert.Contains(t, w.Body.String(), `"cries":[]`)
}

func TestCriesAndClips(t *testing.T) {
	var fetched atomic.Int32
	frigateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched.Add(1)
		w.Write([]byte("mp4 bytes"))
	}))
	defer frigateAPI.Close()

	app, _ := newTestServer(t)
	dir := t.TempDir()
	tracker, err := history.NewTracker(dir, true)
	require.NoError(t, err)
	t.Cleanup(func() { tracker.Close() })
	app.History = tracker
	app.Frigate = frigate.New(frigate.Opts{URL: frigateAPI.URL, Cameras: map[string]string{"nursery": "baby1"}}, tracker)
	app.ClipCacheDir = filepath.Join(dir, "clips")
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}, {UID: "baby2"}})
	mux := app.Handler()

	end := 1700000060.0
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "old", BabyUID: "baby1", Start: 1700000000, End: &end}))
	// A later message without the end doesn't lose it
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "old", BabyUID: "baby1", Start: 1700000000}))
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "now", BabyUID: "baby1", Start: 1700000100}))

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", path, nil)))
		return w
	}

	w := get("/api/cries/baby1?start=1699999000&end=1700001000")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"enabled":true,"baby_uid":"baby1","start_time":1699999000,"end_time":1700001000,"cries":[
		{"review_id":"now","baby_uid":"baby1","start":1700000100,"end":null},
		{"review_id":"old","baby_uid":"baby1","start":1700000000,"end":1700000060}
	]}`, w.Body.String())

	// A complete recording is downloaded once, and served with ranges
	w = get("/api/cries/baby1/old/clip.mp4")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "mp4 bytes", w.Body.String())
	assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
	w = get("/api/cries/baby1/old/clip.mp4")
	assert.Equal(t, "mp4 bytes", w.Body.String())
	assert.EqualValues(t, 1, fetched.Load())

	// Still crying: passed straight through, every time
	w = get("/api/cries/baby1/now/clip.mp4")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.EqualValues(t, 2, fetched.Load())

	// Another baby's spell, or an unknown one
	assert.Equal(t, http.StatusNotFound, get("/api/cries/baby2/old/clip.mp4").Code)
	assert.Equal(t, http.StatusNotFound, get("/api/cries/baby1/missing/clip.mp4").Code)
}
