package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/rs/zerolog/log"
)

// The camera's built-in sounds (white noise, birds, waves...), played over its
// speaker. The protocol fields were worked out by maddijoyce/nanit-web.
//
// Nothing here runs on its own: every command is an explicit API call, and the
// bridge never starts or resumes a sound by itself.

const (
	soundRequestTimeout = 10 * time.Second

	// soundDurationForever - Playback.duration for "keep playing"; the Nanit
	// app loops a sound this way
	soundDurationForever int32 = -1

	minVolume int32 = 0
	maxVolume int32 = 100
)

type soundPlayback struct {
	Playing bool   `json:"playing"`
	Track   string `json:"track,omitempty"`
}

type soundStatus struct {
	Tracks   []string       `json:"tracks"`
	Playback *soundPlayback `json:"playback"`
	Volume   *int32         `json:"volume"`
	// Errors - reads that failed, so a partial answer is still useful
	Errors []string `json:"errors,omitempty"`
}

// sound request helpers, all against a live camera connection

func requestSound(conn *client.WebsocketConnection, reqType client.RequestType, req *client.Request) (*client.Response, error) {
	res, err := conn.SendRequest(reqType, req)(soundRequestTimeout)
	if err != nil {
		return nil, err
	}
	if code := res.GetStatusCode(); code != 0 && code != 200 {
		return nil, fmt.Errorf("camera answered %d: %s", code, res.GetStatusMessage())
	}
	return res, nil
}

func trackName(s *client.Soundtrack) string {
	if s == nil {
		return ""
	}
	return s.GetName()
}

func readSoundStatus(conn *client.WebsocketConnection) soundStatus {
	status := soundStatus{Tracks: []string{}}

	if res, err := requestSound(conn, client.RequestType_GET_SOUNDTRACKS, &client.Request{}); err != nil {
		status.Errors = append(status.Errors, "tracks: "+err.Error())
	} else {
		for _, track := range res.GetSoundtracks() {
			if name := track.GetName(); name != "" {
				status.Tracks = append(status.Tracks, name)
			}
		}
	}

	if res, err := requestSound(conn, client.RequestType_GET_PLAYBACK, &client.Request{}); err != nil {
		status.Errors = append(status.Errors, "playback: "+err.Error())
	} else if pb := res.GetPlayback(); pb != nil {
		playing := pb.GetStatus() == client.Playback_STARTED
		track := trackName(pb.GetSelectedSoundtrack())
		if track == "" {
			track = trackName(pb.GetSoundtrack())
		}
		status.Playback = &soundPlayback{Playing: playing, Track: track}
	}

	if res, err := requestSound(conn, client.RequestType_GET_SETTINGS, &client.Request{
		// GetSettings_: protoc-gen-go's name, avoiding the GetSettings() getter
		GetSettings_: &client.GetSettings{All: boolRef(true)},
	}); err != nil {
		status.Errors = append(status.Errors, "volume: "+err.Error())
	} else if settings := res.GetSettings(); settings != nil && settings.Volume != nil {
		volume := settings.GetVolume()
		status.Volume = &volume
	}

	return status
}

func boolRef(v bool) *bool { return &v }

// playDuration - the Playback.duration for a request: -1 (forever) unless a
// positive number of seconds was asked for
func playDuration(seconds int32) int32 {
	if seconds > 0 {
		return seconds
	}
	return soundDurationForever
}

func clampVolume(level int32) int32 {
	if level < minVolume {
		return minVolume
	}
	if level > maxVolume {
		return maxVolume
	}
	return level
}

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

	conn := app.getConnection(babyUID)
	if conn == nil {
		http.Error(w, "Camera not connected", http.StatusServiceUnavailable)
		return
	}

	sublog := log.With().Str("baby_uid", babyUID).Logger()

	switch {
	case action == "" && r.Method == http.MethodGet:
		writeJSON(w, readSoundStatus(conn))

	case action == "play" && r.Method == http.MethodPost:
		var body struct {
			Track           string `json:"track"`
			DurationSeconds int32  `json:"duration_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Track == "" {
			http.Error(w, "Expected JSON with a track name", http.StatusBadRequest)
			return
		}
		duration := playDuration(body.DurationSeconds)
		sublog.Info().Str("track", body.Track).Int32("duration_seconds", duration).Msg("Playing sound")
		if _, err := requestSound(conn, client.RequestType_PUT_PLAYBACK, &client.Request{
			Playback: &client.Playback{
				Status:             client.Playback_STARTED.Enum(),
				Duration:           &duration,
				Soundtrack:         &client.Soundtrack{Type: int32Ref(0), Name: &body.Track},
				SelectedSoundtrack: &client.Soundtrack{Type: int32Ref(0), Name: &body.Track},
			},
		}); err != nil {
			sublog.Warn().Err(err).Msg("Playing sound failed")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, readSoundStatus(conn))

	case action == "stop" && r.Method == http.MethodPost:
		sublog.Info().Msg("Stopping sound")
		if _, err := requestSound(conn, client.RequestType_PUT_PLAYBACK, &client.Request{
			Playback: &client.Playback{Status: client.Playback_STOPPED.Enum()},
		}); err != nil {
			sublog.Warn().Err(err).Msg("Stopping sound failed")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, readSoundStatus(conn))

	case action == "volume" && r.Method == http.MethodPost:
		var body struct {
			Volume *int32 `json:"volume"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Volume == nil {
			http.Error(w, "Expected JSON with a volume (0-100)", http.StatusBadRequest)
			return
		}
		volume := clampVolume(*body.Volume)
		sublog.Info().Int32("volume", volume).Msg("Setting speaker volume")
		if _, err := requestSound(conn, client.RequestType_PUT_SETTINGS, &client.Request{
			Settings: &client.Settings{Volume: &volume},
		}); err != nil {
			sublog.Warn().Err(err).Msg("Setting volume failed")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, readSoundStatus(conn))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func int32Ref(v int32) *int32 { return &v }

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
