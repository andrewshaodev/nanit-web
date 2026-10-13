package httpapi

import (
	"net/http"
	"net/url"

	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/rs/zerolog/log"
)

// streamPassword - the RTSP and RTMP password settings, as the API shows them
func (s *Server) streamPassword() apitypes.StreamPasswordInfo {
	info := apitypes.StreamPasswordInfo{User: streamauth.User}
	if s.StreamAuth != nil {
		settings := s.StreamAuth.Get()
		info.Enabled = settings.Enabled
		info.Key = settings.Key
	}
	return info
}

// rtspUserInfo - "user:key@" for an RTSP address while the password is on
func rtspUserInfo(p apitypes.StreamPasswordInfo) string {
	if !p.Enabled {
		return ""
	}
	return url.UserPassword(p.User, p.Key).String() + "@"
}

// handleStreamPasswordSet turns the stream password on or off. Turning it
// on the first time makes a key.
func (s *Server) handleStreamPasswordSet(w http.ResponseWriter, r *http.Request) {
	var req apitypes.StreamPasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.streamAuthAvailable(w) {
		return
	}
	if _, err := s.StreamAuth.SetEnabled(req.Enabled); err != nil {
		log.Error().Err(err).Msg("Failed to save the stream password")
		writeError(w, http.StatusInternalServerError, "save_failed", "Couldn't save the stream password")
		return
	}
	log.Info().Bool("enabled", req.Enabled).Msg("Stream password changed")
	writeJSON(w, s.streamPassword())
}

// handleStreamPasswordRegenerate replaces the key
func (s *Server) handleStreamPasswordRegenerate(w http.ResponseWriter, _ *http.Request) {
	if !s.streamAuthAvailable(w) {
		return
	}
	if _, err := s.StreamAuth.Regenerate(); err != nil {
		log.Error().Err(err).Msg("Failed to save the stream password")
		writeError(w, http.StatusInternalServerError, "save_failed", "Couldn't save the stream password")
		return
	}
	log.Info().Msg("Stream password key regenerated")
	writeJSON(w, s.streamPassword())
}

func (s *Server) streamAuthAvailable(w http.ResponseWriter) bool {
	if s.StreamAuth == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "The stream password isn't available")
		return false
	}
	return true
}
