package httpapi

import (
	"net/http"

	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/streamsource"
	"github.com/rs/zerolog/log"
)

// streamFrom - a viewer's stream server, as an address with the key masked
// and {baby_uid} as a placeholder; "" on the server, or with none set
func (s *Server) streamFrom() string {
	if s.StreamSource == nil {
		return ""
	}
	return s.StreamSource.Get().Redacted("{baby_uid}")
}

func (s *Server) streamSourceResponse() apitypes.StreamSourceResponse {
	source := s.StreamSource.Get()
	res := apitypes.StreamSourceResponse{
		Host:    source.Host,
		Port:    source.Port,
		Key:     source.Key,
		Cameras: []apitypes.StreamSourceCamera{},
	}
	if res.Port == 0 {
		res.Port = streamsource.DefaultPort
	}
	for _, b := range s.babies() {
		cam := apitypes.StreamSourceCamera{BabyUID: b.UID, Name: b.Name}
		if st, ok := s.StreamSource.Status(b.UID); ok {
			cam.Receiving = st.Receiving
			cam.Error = st.Error
			cam.Since = st.Since.Unix()
		}
		res.Cameras = append(res.Cameras, cam)
	}
	return res
}

// viewerOnly - the stream server has no stream server of its own
func (s *Server) viewerOnly(w http.ResponseWriter) bool {
	if s.StreamSource == nil {
		writeError(w, http.StatusConflict, "stream_server", "This instance is the stream server; it takes video from the cameras")
		return false
	}
	return true
}

// handleStreamSourceGet - a viewer's stream server, and how each camera's
// video from it is going
func (s *Server) handleStreamSourceGet(w http.ResponseWriter, _ *http.Request) {
	if !s.viewerOnly(w) {
		return
	}
	writeJSON(w, s.streamSourceResponse())
}

// handleStreamSourceSet saves a viewer's stream server; its cameras
// reconnect to it straight away
func (s *Server) handleStreamSourceSet(w http.ResponseWriter, r *http.Request) {
	var req apitypes.StreamSourceRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.viewerOnly(w) {
		return
	}
	source, err := s.StreamSource.Set(streamsource.Source{Host: req.Host, Port: req.Port, Key: req.Key})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_stream_server", err.Error())
		return
	}
	log.Info().Str("from", source.Redacted("{baby_uid}")).Msg("Stream server changed")
	writeJSON(w, s.streamSourceResponse())
}
