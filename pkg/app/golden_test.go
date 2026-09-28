package app

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/golden from the current handlers")

// The JSON each endpoint the dashboard reads returns, for a fixed state.
// The frontend depends on these shapes, so changing a handler must leave
// them as they are (or change the golden file on purpose, with the
// frontend). Values that vary between runs are zeroed first.
var goldenRequests = map[string]string{
	"status":        "/api/status",
	"babies":        "/api/babies",
	"streaming":     "/api/streaming/info",
	"device_info":   "/api/device-info/baby1",
	"auth_status":   "/api/auth/status",
	"stream_status": "/api/stream/status/baby1",
	"health":        "/api/health/baby1",
	"sensor":        "/api/history/sensor/baby1?start=0&end=4000000000",
	"events":        "/api/history/events/baby1?start=0&end=4000000000",
	"summary":       "/api/history/summary/baby1?start=0&end=4000000000",
	"day_night":     "/api/history/day-night/baby1?start=0&end=4000000000",
	"webauth":       "/api/webauth/status",
	"liveness":      "/health",
	"readiness":     "/ready",
}

// varying - keys whose values change from run to run
var varying = map[string]bool{
	"timestamp": true, "created_at": true, "start": true, "end": true,
	"auth_time": true, "uptime": true, "last_updated": true,
	"last_video_packet_time": true,
}

func zeroVarying(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, val := range v {
			if varying[k] {
				v[k] = 0
			} else {
				v[k] = zeroVarying(val)
			}
		}
	case []any:
		for i := range v {
			v[i] = zeroVarying(v[i])
		}
	}
	return v
}

func goldenState(t *testing.T) (*App, *http.ServeMux) {
	app, mux := newTestServer(t)
	dir := t.TempDir()
	tracker, err := history.NewTracker(dir, true)
	require.NoError(t, err)
	t.Cleanup(func() { tracker.Close() })
	app.HistoryTracker = tracker
	app.Opts.RTMP = &RTMPOpts{PublicAddr: "192.0.2.1:1935", AutoStart: true}
	require.NoError(t, os.WriteFile(app.Opts.SessionFile, []byte("{}"), 0600))

	app.SessionStore.SetBabies([]baby.Baby{{UID: "baby1", Name: "Baby", CameraUID: "cam1"}})
	app.SessionStore.SetRefreshToken("refresh")
	state := baby.NewState().SetTemperatureMilli(23500).SetHumidityMilli(51200).SetIsNight(false).
		SetNightLight(true).SetStandby(false).SetWebsocketAlive(true).SetStreamState(baby.StreamState_Alive)
	app.BabyStateManager.Update("baby1", *state)

	require.NoError(t, tracker.TrackSensorData("baby1", *state))
	require.NoError(t, tracker.TrackEvent("baby1", "motion", 1_790_000_000))
	return app, mux
}

func TestGoldenResponses(t *testing.T) {
	app, mux := goldenState(t)

	for name, path := range goldenRequests {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, signedIn(t, app, httptest.NewRequest("GET", path, nil)))
			require.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "application/json"), "%s: %s", path, w.Body.String())

			var body any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
			got, err := json.MarshalIndent(map[string]any{"status": w.Code, "body": zeroVarying(body)}, "", "  ")
			require.NoError(t, err)

			file := filepath.Join("testdata", "golden", name+".json")
			if *updateGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
				require.NoError(t, os.WriteFile(file, append(got, '\n'), 0o644))
				return
			}
			want, err := os.ReadFile(file)
			require.NoError(t, err, "run go test ./pkg/app -run Golden -update-golden to create it")
			assert.Equal(t, string(bytes.TrimSpace(want)), string(got))
		})
	}
}
