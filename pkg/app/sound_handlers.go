package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/andrewshaodev/nanit-web/pkg/camera"
)

// The camera's built-in sounds (white noise, birds, waves...), played over its
// speaker. The protocol fields were worked out by maddijoyce/nanit-web.
//
// Nothing here runs on its own: every command is an explicit API call, and the
// bridge never starts or resumes a sound by itself.

// handleSoundAPI serves /api/sound/{baby_uid}[/play|/stop|/volume]
func handleSoundAPI(w http.ResponseWriter, r *http.Request, app *App) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sound/"), "/"), "/")
	babyUID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if babyUID == "" || len(parts) > 2 {
		http.Error(w, "Expected /api/sound/{baby_uid}[/play|/stop|/volume]", http.StatusNotFound)
		return
	}

	var cam *camera.Camera
	if app.Cameras != nil {
		cam, _ = app.Cameras.Get(babyUID)
	}
	if cam == nil {
		http.Error(w, "Camera not connected", http.StatusServiceUnavailable)
		return
	}

	var status camera.SoundStatus
	var err error
	switch {
	case action == "" && r.Method == http.MethodGet:
		status, err = cam.SoundStatus()

	case action == "play" && r.Method == http.MethodPost:
		var body struct {
			Track           string `json:"track"`
			DurationSeconds int32  `json:"duration_seconds"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil || body.Track == "" {
			http.Error(w, "Expected JSON with a track name", http.StatusBadRequest)
			return
		}
		status, err = cam.PlaySound(body.Track, body.DurationSeconds)

	case action == "stop" && r.Method == http.MethodPost:
		status, err = cam.StopSound()

	case action == "volume" && r.Method == http.MethodPost:
		var body struct {
			Volume *int32 `json:"volume"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil || body.Volume == nil {
			http.Error(w, "Expected JSON with a volume (0-100)", http.StatusBadRequest)
			return
		}
		status, err = cam.SetVolume(*body.Volume)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if errors.Is(err, camera.ErrNotConnected) {
		http.Error(w, "Camera not connected", http.StatusServiceUnavailable)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, status)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
