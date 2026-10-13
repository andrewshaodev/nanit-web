package httpapi

import (
	"math"
	"net/http"
	"strconv"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/frigate"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
)

// Crying spells from Frigate, and their recordings. Frigate isn't reachable
// from the browser (it's on the Docker network, without a password), so the
// recordings come through here, behind the dashboard password.

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

// handleCryVOD - a spell's recording, as HLS from Frigate. until fixes the
// end of a spell still going when the player opens it, so the playlist and
// the segments it names all come from the same recording.
func (s *Server) handleCryVOD(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	id, file := r.PathValue("review"), r.PathValue("file")
	until, err := strconv.ParseInt(r.PathValue("until"), 10, 64)
	if s.Frigate == nil || err != nil || !frigate.ValidReviewID(id) || !frigate.ValidVODFile(file) {
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
	// Saved before the camera was kept, and not seen in Frigate since
	if c.Camera == "" {
		writeError(w, http.StatusNotFound, "no_recording", "Frigate's camera for this crying spell isn't known")
		return
	}

	start := int64(math.Floor(c.Start)) - frigate.ClipPadding
	end := until
	if c.End != nil {
		end = int64(math.Ceil(*c.End)) + frigate.ClipPadding
	}
	end = max(end, start+frigate.ClipPadding)
	s.Frigate.ServeVOD(w, r, c.Camera, start, end, file)
}
