package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/snapshot"
	"github.com/rs/zerolog/log"
)

// handleSnapshot - GET /api/snapshot/{uid}: a JPEG of the camera now, from
// its stream. Besides a dashboard session, the stream password's key opens
// it, as ?key=KEY or as the password of basic auth (user "nanit", as for
// RTSP), so Home Assistant can fetch it.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.snapshotAllowed(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="nanit"`)
		writeError(w, http.StatusUnauthorized, "authentication_required", "Sign in, or give the stream password's key")
		return
	}
	b, ok := s.baby(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_baby", "No baby with that UID on this account")
		return
	}
	if s.Snapshots == nil {
		writeError(w, http.StatusServiceUnavailable, "no_snapshots", "Snapshots aren't available")
		return
	}

	pic, err := s.Snapshots.Get(r.Context(), b.UID)
	if errors.Is(err, snapshot.ErrNoStream) {
		writeError(w, http.StatusServiceUnavailable, "no_stream", "The camera isn't streaming")
		return
	}
	if err != nil {
		if r.Context().Err() == nil {
			log.Warn().Err(err).Str("baby_uid", b.UID).Msg("Couldn't take a snapshot")
		}
		writeError(w, http.StatusInternalServerError, "snapshot_failed", "Couldn't take a picture")
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(pic.JPEG)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Last-Modified", pic.Taken.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Snapshot-Age-Ms", strconv.FormatInt(time.Since(pic.Taken).Milliseconds(), 10))
	_, _ = w.Write(pic.JPEG)
}

// snapshotAllowed - a dashboard session (or none needed), or the stream
// password's key while it's on, in either place: Home Assistant sends the
// camera's user name and password as basic auth with every request, so a
// key in the query must still count when that password isn't the key
func (s *Server) snapshotAllowed(r *http.Request) bool {
	if s.signedIn(r) {
		return true
	}
	queryKey := r.URL.Query().Get("key")
	user, password, hasBasic := r.BasicAuth()
	if _, required := s.StreamAuth.Required(); required {
		for _, key := range []string{queryKey, password} {
			if key != "" && s.StreamAuth.Allows(key) {
				return true
			}
		}
	}
	// Why it was refused, without the values, for setting up a client
	log.Info().
		Str("remote", r.RemoteAddr).
		Bool("query_key", queryKey != "").
		Bool("basic_auth", hasBasic).
		Str("basic_user", user).
		Msg("Snapshot refused: no session, and no valid stream password key")
	return false
}
