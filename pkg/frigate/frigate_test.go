package frigate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct{ cries []history.Cry }

func (r *recorder) RecordCry(c history.Cry) error {
	r.cries = append(r.cries, c)
	return nil
}

func newService(t *testing.T, url string) (*Service, *recorder) {
	rec := &recorder{}
	s, err := New(Opts{URL: url, Cameras: map[string]string{"nursery": "bb36bfe3"}}, rec)
	require.NoError(t, err)
	return s, rec
}

func TestNewChecksURL(t *testing.T) {
	for _, bad := range []string{"", "frigate:5000", "ftp://frigate"} {
		_, err := New(Opts{URL: bad}, &recorder{})
		assert.Error(t, err, bad)
	}
}

func TestParseCameras(t *testing.T) {
	got, err := ParseCameras(" nursery=bb36bfe3, travel=380fb4f2 ")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"nursery": "bb36bfe3", "travel": "380fb4f2"}, got)

	for _, bad := range []string{"", "nursery", "=bb36bfe3", "nursery=../x"} {
		_, err := ParseCameras(bad)
		assert.Error(t, err, bad)
	}
}

// Every message about a crying alert is saved, so the end arrives with the
// last one. Other cameras, detections and sounds are left out.
func TestHandleReview(t *testing.T) {
	s, rec := newService(t, "http://frigate:5000")
	assert.Equal(t, "frigate/reviews", s.ReviewsTopic())

	s.HandleReview([]byte(`{"type":"new","after":{"id":"1700000000.1-abc","camera":"nursery","start_time":1700000000.1,"end_time":null,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"end","after":{"id":"1700000000.1-abc","camera":"nursery","start_time":1700000000.1,"end_time":1700000060.5,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x1","camera":"driveway","start_time":1,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x2","camera":"nursery","start_time":1,"severity":"detection","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x3","camera":"nursery","start_time":1,"severity":"alert","data":{"audio":["speech"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"../x","camera":"nursery","start_time":1,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`not json`))

	require.Len(t, rec.cries, 2)
	assert.Equal(t, history.Cry{ReviewID: "1700000000.1-abc", BabyUID: "bb36bfe3", Camera: "nursery", Start: 1700000000.1}, rec.cries[0])
	require.NotNil(t, rec.cries[1].End)
	assert.Equal(t, 1700000060.5, *rec.cries[1].End)
}

// The review API fills in spells MQTT missed
func TestSync(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/review", r.URL.Path)
		query = r.URL.RawQuery
		w.Write([]byte(`[
			{"id":"a","camera":"nursery","start_time":10,"end_time":20,"severity":"alert","data":{"audio":["crying"]}},
			{"id":"b","camera":"nursery","start_time":30,"end_time":null,"severity":"alert","data":{"audio":[]}}
		]`))
	}))
	defer srv.Close()

	s, rec := newService(t, srv.URL+"/")
	require.NoError(t, s.Sync(context.Background(), time.Unix(1000, 0)))
	assert.Equal(t, "after=1000&cameras=nursery&limit=1000&severity=alert", query)
	require.Len(t, rec.cries, 1)
	assert.Equal(t, "a", rec.cries[0].ReviewID)
}

// Only the file's path and the player's headers reach Frigate, not the
// dashboard's cookie or the query
func TestServeVOD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vod/nursery/start/100/end/200/seg-1-v1-a1.m4s", r.URL.Path)
		assert.Empty(t, r.URL.RawQuery)
		assert.Empty(t, r.Header.Get("Cookie"))
		assert.Equal(t, "bytes=0-9", r.Header.Get("Range"))
		w.Header().Set("Cache-Control", "public")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("segment"))
	}))
	defer srv.Close()

	s, _ := newService(t, srv.URL)
	req := httptest.NewRequest("GET", "/api/cries/bb36bfe3/a/vod/0/seg-1-v1-a1.m4s?x=1", nil)
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("Range", "bytes=0-9")
	w := httptest.NewRecorder()
	s.ServeVOD(w, req, "nursery", 100, 200, "seg-1-v1-a1.m4s")
	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "segment", w.Body.String())
	assert.Equal(t, "private, max-age=3600", w.Header().Get("Cache-Control"))
}

func TestValidVODFile(t *testing.T) {
	for _, ok := range []string{"index.m3u8", "index-v1-a1.m3u8", "init-v1-a1.mp4", "seg-12-v1-a1.m4s"} {
		assert.True(t, ValidVODFile(ok), ok)
	}
	for _, bad := range []string{"", "../index.m3u8", "index.m3u8/x", "a.ts", "index"} {
		assert.False(t, ValidVODFile(bad), bad)
	}
}
