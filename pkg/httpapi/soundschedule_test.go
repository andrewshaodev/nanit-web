package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/soundschedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Schedules are the bridge's settings: saved and read with no camera
func TestSoundSchedulesAPI(t *testing.T) {
	app, mux := newTestServer(t)
	app.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})
	app.SoundSchedules = soundschedule.Open(filepath.Join(t.TempDir(), "s.json"))
	app.Config.StreamServer = true

	call := func(method, body string) (*httptest.ResponseRecorder, soundschedule.Schedules) {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest(method, "/api/sound/baby1/schedules", strings.NewReader(body))))
		var res soundschedule.Schedules
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		return w, res
	}

	w, res := call("GET", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, res.Schedules)
	assert.True(t, res.Active)
	assert.NotEmpty(t, res.TimeZone)

	w, res = call("PUT", `{"schedules":[{"enabled":true,"track":"White Noise.wav","start":"19:30","end":"06:30","days":[]}]}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, res.Schedules, 1)
	assert.NotEmpty(t, res.Schedules[0].ID)

	w, _ = call("PUT", `{"schedules":[{"enabled":true,"track":"White Noise.wav","start":"19:30","end":"19:30"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	_, res = call("GET", "")
	assert.Len(t, res.Schedules, 1)
}
