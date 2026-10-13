package httpapi

import (
	"net/http"
	"net/http/httptest"
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

func TestCriesAndRecordings(t *testing.T) {
	var paths []string
	frigateAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write([]byte("#EXTM3U"))
	}))
	defer frigateAPI.Close()

	app, _ := newTestServer(t)
	tracker, err := history.NewTracker(t.TempDir(), true)
	require.NoError(t, err)
	t.Cleanup(func() { tracker.Close() })
	app.History = tracker
	app.Frigate, err = frigate.New(frigate.Opts{URL: frigateAPI.URL, Cameras: map[string]string{"nursery": "baby1"}}, tracker)
	require.NoError(t, err)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}, {UID: "baby2"}})
	mux := app.Handler()

	end, oldEnd := 1700000060.4, 1600000060.0
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "old", BabyUID: "baby1", Camera: "nursery", Start: 1700000000.5, End: &end}))
	// A later message without the end doesn't lose it
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "old", BabyUID: "baby1", Camera: "nursery", Start: 1700000000.5}))
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "now", BabyUID: "baby1", Camera: "nursery", Start: 1700000100}))
	require.NoError(t, tracker.RecordCry(history.Cry{ReviewID: "nocamera", BabyUID: "baby1", Start: 1600000000, End: &oldEnd}))

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", path, nil)))
		return w
	}

	w := get("/api/cries/baby1?start=1699999000&end=1700001000")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"enabled":true,"baby_uid":"baby1","start_time":1699999000,"end_time":1700001000,"cries":[
		{"review_id":"now","baby_uid":"baby1","start":1700000100,"end":null},
		{"review_id":"old","baby_uid":"baby1","start":1700000000.5,"end":1700000060.4}
	]}`, w.Body.String())

	// An ended spell: padded either side, whatever until says
	w = get("/api/cries/baby1/old/vod/1800000000/index.m3u8")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "#EXTM3U", w.Body.String())
	assert.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	// Still crying: up to until, so every file comes from the same recording
	assert.Equal(t, http.StatusOK, get("/api/cries/baby1/now/vod/1700000200/init-v1-a1.mp4").Code)
	assert.Equal(t, []string{
		"/vod/nursery/start/1699999970/end/1700000091/index.m3u8",
		"/vod/nursery/start/1700000070/end/1700000200/init-v1-a1.mp4",
	}, paths)

	// Another baby's spell, an unknown one, one without its camera, and
	// files that aren't Frigate's
	for _, path := range []string{
		"/api/cries/baby2/old/vod/0/index.m3u8",
		"/api/cries/baby1/missing/vod/0/index.m3u8",
		"/api/cries/baby1/nocamera/vod/0/index.m3u8",
		"/api/cries/baby1/old/vod/0/clip.mov",
		"/api/cries/baby1/old/vod/soon/index.m3u8",
	} {
		assert.Equal(t, http.StatusNotFound, get(path).Code, path)
	}
	assert.Len(t, paths, 2)
}
