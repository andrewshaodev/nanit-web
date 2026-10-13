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

func newService(url string) (*Service, *recorder) {
	rec := &recorder{}
	return New(Opts{URL: url, Cameras: map[string]string{"nursery": "bb36bfe3"}}, rec), rec
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
	s, rec := newService("")
	assert.Equal(t, "frigate/reviews", s.ReviewsTopic())

	s.HandleReview([]byte(`{"type":"new","after":{"id":"1700000000.1-abc","camera":"nursery","start_time":1700000000.1,"end_time":null,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"end","after":{"id":"1700000000.1-abc","camera":"nursery","start_time":1700000000.1,"end_time":1700000060.5,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x1","camera":"driveway","start_time":1,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x2","camera":"nursery","start_time":1,"severity":"detection","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"x3","camera":"nursery","start_time":1,"severity":"alert","data":{"audio":["speech"]}}}`))
	s.HandleReview([]byte(`{"type":"new","after":{"id":"../x","camera":"nursery","start_time":1,"severity":"alert","data":{"audio":["crying"]}}}`))
	s.HandleReview([]byte(`not json`))

	require.Len(t, rec.cries, 2)
	assert.Equal(t, history.Cry{ReviewID: "1700000000.1-abc", BabyUID: "bb36bfe3", Start: 1700000000.1}, rec.cries[0])
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

	s, rec := newService(srv.URL + "/")
	require.NoError(t, s.Sync(context.Background(), time.Unix(1000, 0)))
	assert.Equal(t, "after=1000&cameras=nursery&limit=1000&severity=alert", query)
	require.Len(t, rec.cries, 1)
	assert.Equal(t, "a", rec.cries[0].ReviewID)
}

func TestClip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/review/a/clip.mp4" {
			http.NotFound(w, r)
			return
		}
		assert.Equal(t, "30", r.URL.Query().Get("padding"))
		w.Write([]byte("mp4"))
	}))
	defer srv.Close()

	s, _ := newService(srv.URL)
	body, err := s.Clip(context.Background(), "a")
	require.NoError(t, err)
	body.Close()

	_, err = s.Clip(context.Background(), "missing")
	assert.ErrorContains(t, err, "404")
	_, err = s.Clip(context.Background(), "../a")
	assert.Error(t, err)
}
