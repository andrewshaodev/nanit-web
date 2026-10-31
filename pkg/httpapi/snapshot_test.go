package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/snapshot"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A snapshot needs a dashboard session, or the stream password's key while
// it's on (in the query, or as basic auth's password), and only then says
// whether the baby exists or is streaming
func TestSnapshotAuth(t *testing.T) {
	app, mux := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})
	app.Snapshots = snapshot.New(relay.NewHub(nil))
	app.StreamAuth = streamauth.Open(filepath.Join(t.TempDir(), "stream_auth.json"))

	get := func(r *http.Request) int {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	req := func(path string) *http.Request { return httptest.NewRequest("GET", path, nil) }

	// The dashboard password is on (newTestServer), the stream password off
	assert.Equal(t, http.StatusUnauthorized, get(req("/api/snapshot/baby1")))
	assert.Equal(t, http.StatusUnauthorized, get(req("/api/snapshot/unknown")), "unknown UIDs aren't told apart signed out")
	assert.Equal(t, http.StatusUnauthorized, get(req("/api/snapshot/baby1?key=anything")), "no key opens it while the stream password is off")
	assert.Equal(t, http.StatusServiceUnavailable, get(signedIn(t, app, req("/api/snapshot/baby1"))), "not streaming")
	assert.Equal(t, http.StatusNotFound, get(signedIn(t, app, req("/api/snapshot/unknown"))))

	settings, err := app.StreamAuth.SetEnabled(true)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, get(req("/api/snapshot/baby1?key=wrong")))
	assert.Equal(t, http.StatusServiceUnavailable, get(req("/api/snapshot/baby1?key="+settings.Key)))
	basic := req("/api/snapshot/baby1")
	basic.SetBasicAuth("nanit", settings.Key)
	assert.Equal(t, http.StatusServiceUnavailable, get(basic))
	// The camera's own user name and password as basic auth (Home
	// Assistant sends them with every request) don't hide a key in the query
	both := req("/api/snapshot/baby1?key=" + settings.Key)
	both.SetBasicAuth("admin", "something else")
	assert.Equal(t, http.StatusServiceUnavailable, get(both))

	// With no dashboard password and the stream password off, it's open,
	// like the RTSP output
	require.NoError(t, app.WebAuth.RemovePassword())
	_, err = app.StreamAuth.SetEnabled(false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, get(req("/api/snapshot/baby1")))
}
