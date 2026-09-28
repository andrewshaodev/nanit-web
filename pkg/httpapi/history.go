package httpapi

import (
	"net/http"
	"strconv"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/rs/zerolog/log"
)

// The recorded readings and events, for the dashboard's charts. The range
// is the start and end query parameters (see timeRange).

func (s *Server) handleHistorySensor(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	start, end := timeRange(r)
	// Sampled to suit the length of the range
	readings, err := s.History.GetSensorReadingsWithSampling(b.UID, start, end)
	if err != nil {
		historyError(w, err, b, "sensor data")
		return
	}
	writeJSON(w, apitypes.SensorHistoryResponse{
		BabyUID: b.UID, StartTime: start, EndTime: end, Readings: readings, Count: len(readings),
	})
}

func (s *Server) handleHistoryEvents(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	start, end := timeRange(r)
	eventType := r.URL.Query().Get("type")
	limit := 500
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 5000 {
		limit = n
	}

	events, err := s.History.GetEvents(b.UID, start, end, eventType, limit)
	if err != nil {
		historyError(w, err, b, "event data")
		return
	}
	writeJSON(w, apitypes.EventsHistoryResponse{
		BabyUID: b.UID, StartTime: start, EndTime: end, EventType: eventType, Events: events, Count: len(events),
	})
}

func (s *Server) handleHistorySummary(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	start, end := timeRange(r)
	summary, err := s.History.GetSummary(b.UID, start, end)
	if err != nil {
		historyError(w, err, b, "summary data")
		return
	}
	writeJSON(w, summary)
}

func (s *Server) handleHistoryDayNight(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	start, end := timeRange(r)
	dayNight, err := s.History.GetDayNightAnalytics(b.UID, start, end)
	if err != nil {
		historyError(w, err, b, "day/night data")
		return
	}
	writeJSON(w, apitypes.DayNightResponse{BabyUID: b.UID, StartTime: start, EndTime: end, DayNight: dayNight})
}

func (s *Server) handleHistoryReset(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	if _, err := s.History.ResetData(b.UID); err != nil {
		log.Error().Err(err).Str("baby_uid", b.UID).Msg("Failed to reset history data")
		writeError(w, http.StatusInternalServerError, "reset_failed", "Failed to reset history data")
		return
	}
	log.Info().Str("baby_uid", b.UID).Msg("History data reset successfully")
	writeJSON(w, apitypes.HistoryResetResponse{Success: true, BabyUID: b.UID, Message: "History data reset successfully"})
}

func historyError(w http.ResponseWriter, err error, b baby.Baby, what string) {
	log.Error().Err(err).Str("baby_uid", b.UID).Msgf("Failed to get %s", what)
	writeError(w, http.StatusInternalServerError, "history_failed", "Failed to retrieve "+what)
}
