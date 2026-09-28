package history

import "database/sql"

// recordingGapLimit - silence after which the bridge is taken to have not
// been recording. Sensor readings normally arrive every 30-60s, while the
// cam's day/night mode is only reported when it changes, so a mode can only
// be carried forward across time the bridge was actually listening.
const recordingGapLimit int64 = 10 * 60

// Day/night timeline modes
const (
	ModeDay     = "day"
	ModeNight   = "night"
	ModeUnknown = "unknown"
)

// DayNightPeriod - a stretch of the timeline in one mode
type DayNightPeriod struct {
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Mode  string `json:"mode"` // ModeDay, ModeNight or ModeUnknown
}

// modeReading - a sensor reading's time, and the day/night mode it reported
// if it reported one
type modeReading struct {
	timestamp int64
	isNight   sql.NullBool
}

func modeName(isNight sql.NullBool) string {
	switch {
	case !isNight.Valid:
		return ModeUnknown
	case isNight.Bool:
		return ModeNight
	default:
		return ModeDay
	}
}

// buildDayNightTimeline - splits [start, end] into day, night and unknown
// periods.
//
// initial is the last mode reported before start, if any. Between readings
// the last reported mode carries forward, except across a gap longer than
// gapLimit, which is unknown: nothing was being recorded. Time before any
// mode is known is unknown too, rather than borrowing the first mode seen.
func buildDayNightTimeline(start, end int64, initial sql.NullBool, readings []modeReading, gapLimit int64) ([]DayNightPeriod, []DayNightChange) {
	var periods []DayNightPeriod
	var changes []DayNightChange

	add := func(from, to int64, mode string) {
		if to <= from {
			return
		}
		if n := len(periods); n > 0 && periods[n-1].Mode == mode && periods[n-1].End == from {
			periods[n-1].End = to
			return
		}
		periods = append(periods, DayNightPeriod{Start: from, End: to, Mode: mode})
	}

	current := initial
	modeSince := start
	cursor := start
	for _, reading := range readings {
		if reading.timestamp < start || reading.timestamp > end {
			continue
		}

		mode := modeName(current)
		if reading.timestamp-cursor > gapLimit {
			mode = ModeUnknown
		}
		add(cursor, reading.timestamp, mode)
		cursor = reading.timestamp

		if reading.isNight.Valid && (!current.Valid || current.Bool != reading.isNight.Bool) {
			if current.Valid {
				changes = append(changes, DayNightChange{
					Timestamp:    reading.timestamp,
					FromNight:    current.Bool,
					ToNight:      reading.isNight.Bool,
					DurationMins: (reading.timestamp - modeSince) / 60,
				})
			}
			current = reading.isNight
			modeSince = reading.timestamp
		}
	}

	mode := modeName(current)
	if end-cursor > gapLimit {
		mode = ModeUnknown
	}
	add(cursor, end, mode)

	return periods, changes
}
