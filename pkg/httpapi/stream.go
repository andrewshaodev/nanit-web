package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/rs/zerolog/log"
)

// hlsFileName matches the files ffmpeg writes for a stream (see ffmpegArgs)
var hlsFileName = regexp.MustCompile(`^(playlist\.m3u8|segment_\d+\.ts)$`)

// handleHLSFile - the dashboard's video: the playlist and its segments
func (s *Server) handleHLSFile(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	fileName := r.PathValue("file")
	if !hlsFileName.MatchString(fileName) {
		writeError(w, http.StatusBadRequest, "invalid_stream_path", "Invalid stream path")
		return
	}

	transcoder, exists := s.HLS.GetTranscoder(b.UID)
	if !exists {
		writeJSONStatus(w, http.StatusNotFound, apitypes.HLSErrorResponse{
			Error:   "no_transcoder",
			Message: "No stream transcoder found for this baby",
		})
		return
	}

	status, streamErr := transcoder.GetStatus()
	if !transcoder.IsRunning() {
		writeJSONStatus(w, http.StatusServiceUnavailable, apitypes.HLSErrorResponse{
			Error:       "transcoder_not_running",
			Message:     "Stream transcoder is not running",
			Status:      string(status),
			StreamError: streamErr,
		})
		return
	}

	filePath := filepath.Join(transcoder.GetHLSDir(), fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeJSONStatus(w, http.StatusNotFound, apitypes.HLSErrorResponse{
			Error:       "file_not_found",
			Message:     "HLS file not available yet",
			Status:      string(status),
			File:        fileName,
			StreamError: streamErr,
		})
		return
	}

	if strings.HasSuffix(fileName, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "max-age=3600")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	http.ServeFile(w, r, filePath)
}

// streamBaby reads a stream request's body, and checks its baby. The UID
// names a folder on disk, so only the account's own are accepted.
func (s *Server) streamBaby(w http.ResponseWriter, r *http.Request) (baby.Baby, bool) {
	var req apitypes.StreamRequest
	if !decode(w, r, &req) {
		return baby.Baby{}, false
	}
	b, ok := s.baby(req.BabyUID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_baby", "No baby with that UID on this account")
	}
	return b, ok
}

// handleStreamStart asks the camera to stream. HLS starts once it's
// publishing (straight away if it already is).
func (s *Server) handleStreamStart(w http.ResponseWriter, r *http.Request) {
	b, ok := s.streamBaby(w, r)
	if !ok {
		return
	}
	cam, ok := s.camera(w, b)
	if !ok {
		return
	}
	if !cameraOK(w, cam.RequestStream()) {
		return
	}
	log.Info().Str("baby_uid", b.UID).Msg("Stream requested")
	writeJSON(w, apitypes.StreamResponse{
		Success: true,
		BabyUID: b.UID,
		HLSURL:  fmt.Sprintf("/api/stream/hls/%s/playlist.m3u8", b.UID),
		Message: "Stream requested; video starts once the camera is streaming",
	})
}

func (s *Server) handleStreamStop(w http.ResponseWriter, r *http.Request) {
	b, ok := s.streamBaby(w, r)
	if !ok {
		return
	}
	s.HLS.StopTranscoding(b.UID)
	log.Info().Str("baby_uid", b.UID).Msg("HLS transcoding stopped")
	writeJSON(w, apitypes.StreamResponse{Success: true, BabyUID: b.UID, Message: "Stream stopped successfully"})
}

func (s *Server) handleStreamStatus(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	if s.State.GetBabyState(b.UID).GetStreamRequestState() == baby.StreamRequestState_RequestFailed {
		writeJSON(w, apitypes.StreamStatusResponse{
			BabyUID: b.UID,
			Status:  "blocked",
			Message: "Streaming blocked by connection limit",
			StreamError: &apitypes.StreamErrorInfo{
				Type:    "connection_limit",
				Message: "Too many Nanit mobile apps connected. Close the official Nanit app to enable streaming.",
			},
		})
		return
	}

	transcoder, exists := s.HLS.GetTranscoder(b.UID)
	if !exists {
		writeJSON(w, apitypes.StreamStatusResponse{BabyUID: b.UID, Status: "not_found", Message: "No transcoder found for this baby"})
		return
	}

	info := transcoder.GetDetailedInfo()
	res := apitypes.StreamStatusResponse{
		BabyUID:    b.UID,
		Status:     string(info.Status),
		IsRunning:  &info.IsRunning,
		StartTime:  info.StartTime.Format(time.RFC3339Nano),
		RetryCount: &info.RetryCount,
		MaxRetries: &info.MaxRetries,
		Error:      info.Error,
	}
	if info.IsRunning {
		res.Uptime = &info.Uptime
		res.HasFiles = &info.HasFiles
	}
	writeJSON(w, res)
}
