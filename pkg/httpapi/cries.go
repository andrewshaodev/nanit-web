package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/frigate"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/rs/zerolog/log"
)

// Crying spells from Frigate, and their recordings. Frigate isn't reachable
// from the browser (it's on the Docker network, without a password), so the
// recordings come through here, behind the dashboard password.

// How long a downloaded recording is kept, and how long after a spell ends
// before its recording is complete: the clip runs ClipPadding past the end,
// and Frigate writes recordings in segments of about 10 s.
const (
	clipCacheAge   = 24 * time.Hour
	clipSettleTime = frigate.ClipPadding*time.Second + 30*time.Second
)

func (s *Server) handleCries(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	start, end := timeRange(r)
	res := apitypes.CriesResponse{BabyUID: b.UID, StartTime: start, EndTime: end, Cries: []history.Cry{}}
	if s.Frigate == nil {
		writeJSON(w, res)
		return
	}
	cries, err := s.History.GetCries(b.UID, start, end)
	if err != nil {
		historyError(w, err, b, "crying spells")
		return
	}
	res.Enabled = true
	res.Cries = cries
	writeJSON(w, res)
}

func (s *Server) handleCryClip(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	id := r.PathValue("review")
	if s.Frigate == nil || !frigate.ValidReviewID(id) {
		writeError(w, http.StatusNotFound, "unknown_cry", "No crying spell with that id")
		return
	}
	c, ok, err := s.History.GetCry(id)
	if err != nil {
		historyError(w, err, b, "crying spell")
		return
	}
	if !ok || c.BabyUID != b.UID {
		writeError(w, http.StatusNotFound, "unknown_cry", "No crying spell with that id")
		return
	}

	// A long recording to a phone can take longer than the server's usual
	// write timeout
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))

	// Until it's complete, the recording is passed straight through, and
	// can't be seeked
	if c.End == nil || time.Since(unixTime(*c.End)) < clipSettleTime {
		body, err := s.Frigate.Clip(r.Context(), id)
		if err != nil {
			clipError(w, err, id)
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "no-store")
		if _, err := io.Copy(w, body); err != nil {
			log.Debug().Err(err).Str("review_id", id).Msg("Recording not fully sent")
		}
		return
	}

	// Complete: kept here for a day, so the player can seek in it (Safari
	// won't play a video it can't ask for ranges of)
	path, err := s.cachedClip(r.Context(), id)
	if err != nil {
		clipError(w, err, id)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		clipError(w, err, id)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		clipError(w, err, id)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, id+".mp4", info.ModTime(), f)
}

// cachedClip - the path of a complete recording, downloaded from Frigate if
// it isn't kept here yet
func (s *Server) cachedClip(ctx context.Context, id string) (string, error) {
	if err := os.MkdirAll(s.ClipCacheDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.ClipCacheDir, id+".mp4")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	s.pruneClipCache()

	body, err := s.Frigate.Clip(ctx, id)
	if err != nil {
		return "", err
	}
	defer body.Close()

	// Written beside it and renamed, so a broken download is never served
	tmp, err := os.CreateTemp(s.ClipCacheDir, id+".*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("downloading the recording: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// pruneClipCache removes recordings downloaded over a day ago, and
// downloads that never finished
func (s *Server) pruneClipCache() {
	entries, err := os.ReadDir(s.ClipCacheDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < clipCacheAge {
			continue
		}
		if err := os.Remove(filepath.Join(s.ClipCacheDir, e.Name())); err != nil {
			log.Debug().Err(err).Str("file", e.Name()).Msg("Couldn't remove an old recording")
		}
	}
}

func clipError(w http.ResponseWriter, err error, id string) {
	log.Error().Err(err).Str("review_id", id).Msg("Failed to get a crying spell's recording from Frigate")
	writeError(w, http.StatusBadGateway, "clip_failed", "Couldn't get the recording from Frigate")
}

func unixTime(seconds float64) time.Time {
	return time.Unix(0, int64(seconds*float64(time.Second)))
}
