package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/rs/zerolog/log"
)

// handleControl - POST /api/control/night-light and /api/control/standby,
// with {"baby_uid", "action": "toggle"}
func (s *Server) handleControl(control string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req apitypes.ControlRequest
		if !decode(w, r, &req) {
			return
		}
		if req.BabyUID == "" {
			writeError(w, http.StatusBadRequest, "missing_baby_uid", "baby_uid is required")
			return
		}
		b, ok := s.baby(req.BabyUID)
		if !ok {
			writeError(w, http.StatusNotFound, "unknown_baby", "No baby with that UID on this account")
			return
		}
		if req.Action != "toggle" {
			writeError(w, http.StatusBadRequest, "invalid_action", "The only action is toggle")
			return
		}
		cam, ok := s.camera(w, b)
		if !ok {
			return
		}

		toggle := cam.ToggleNightLight
		if control == "standby" {
			toggle = cam.ToggleStandby
		}
		on, err := toggle()
		if !cameraOK(w, err) {
			return
		}
		log.Info().Str("baby_uid", b.UID).Str("control", control).Bool("new_state", on).Msg("Toggle command sent")

		writeJSON(w, apitypes.ControlResponse{
			Success:   true,
			BabyUID:   b.UID,
			Control:   control,
			Action:    req.Action,
			Timestamp: time.Now().Unix(),
		})
	}
}

// cameraOK answers a command's error, if there was one
func cameraOK(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, camera.ErrNotConnected):
		writeError(w, http.StatusServiceUnavailable, "not_connected", "Camera not connected")
	case errors.Is(err, camera.ErrNoRTMP):
		writeError(w, http.StatusServiceUnavailable, "rtmp_disabled", err.Error())
	default:
		writeError(w, http.StatusBadGateway, "camera_error", err.Error())
	}
	return false
}

// The camera's built-in sounds (white noise, birds, waves...), played over
// its speaker. Nothing here runs on its own: every command is an explicit
// API call, and the bridge never starts or resumes a sound by itself.

func (s *Server) handleSoundStatus(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	s.sound(w, b, func(cam *camera.Camera) (camera.SoundStatus, error) { return cam.SoundStatus() })
}

func (s *Server) handleSoundPlay(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	var req struct {
		Track           string `json:"track"`
		DurationSeconds int32  `json:"duration_seconds"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Track == "" {
		writeError(w, http.StatusBadRequest, "missing_track", "Expected JSON with a track name")
		return
	}
	s.sound(w, b, func(cam *camera.Camera) (camera.SoundStatus, error) {
		return cam.PlaySound(req.Track, req.DurationSeconds)
	})
}

func (s *Server) handleSoundStop(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	s.sound(w, b, func(cam *camera.Camera) (camera.SoundStatus, error) { return cam.StopSound() })
}

func (s *Server) handleSoundVolume(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	var req struct {
		Volume *int32 `json:"volume"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Volume == nil {
		writeError(w, http.StatusBadRequest, "missing_volume", "Expected JSON with a volume (0-100)")
		return
	}
	s.sound(w, b, func(cam *camera.Camera) (camera.SoundStatus, error) { return cam.SetVolume(*req.Volume) })
}

// sound runs a sound command and answers with the status after it
func (s *Server) sound(w http.ResponseWriter, b baby.Baby, command func(*camera.Camera) (camera.SoundStatus, error)) {
	cam, ok := s.camera(w, b)
	if !ok {
		return
	}
	status, err := command(cam)
	if cameraOK(w, err) {
		writeJSON(w, status)
	}
}
