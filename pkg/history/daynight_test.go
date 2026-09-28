package history

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	noMode = sql.NullBool{}
	night  = sql.NullBool{Bool: true, Valid: true}
	day    = sql.NullBool{Bool: false, Valid: true}
)

// sensor readings every 30s over [from, to), reporting mode only at the first
func readingsEvery30s(from, to int64, mode sql.NullBool) []modeReading {
	var out []modeReading
	for ts := from; ts < to; ts += 30 {
		r := modeReading{timestamp: ts}
		if ts == from {
			r.isNight = mode
		}
		out = append(out, r)
	}
	return out
}

// Recording started partway through the window. The time before it used to
// be filled with the first mode seen, showing a full day of night that was
// never observed.
func TestTimeBeforeRecordingIsUnknown(t *testing.T) {
	readings := readingsEvery30s(80000, 86400, night)

	periods, changes := buildDayNightTimeline(0, 86400, noMode, readings, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{
		{Start: 0, End: 80000, Mode: ModeUnknown},
		{Start: 80000, End: 86400, Mode: ModeNight},
	}, periods)
	assert.Empty(t, changes)
}

// The cam reports its mode only when it changes, so the last one carries
// forward across ordinary readings
func TestModeCarriesForwardAndTransitions(t *testing.T) {
	readings := append(readingsEvery30s(0, 3600, day), readingsEvery30s(3600, 7200, night)...)

	periods, changes := buildDayNightTimeline(0, 7200, noMode, readings, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{
		{Start: 0, End: 3600, Mode: ModeDay},
		{Start: 3600, End: 7200, Mode: ModeNight},
	}, periods)
	assert.Equal(t, []DayNightChange{{Timestamp: 3600, FromNight: false, ToNight: true, DurationMins: 60}}, changes)
}

// A mode reported before the window opens applies from its start
func TestInitialModeFromBeforeWindow(t *testing.T) {
	readings := readingsEvery30s(0, 3600, noMode)

	periods, _ := buildDayNightTimeline(0, 3600, day, readings, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{{Start: 0, End: 3600, Mode: ModeDay}}, periods)
}

// While the bridge was down there is nothing to carry the mode across
func TestRecordingGapIsUnknown(t *testing.T) {
	readings := append(readingsEvery30s(0, 1800, night), readingsEvery30s(7200, 9000, noMode)...)

	periods, changes := buildDayNightTimeline(0, 9000, noMode, readings, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{
		{Start: 0, End: 1770, Mode: ModeNight},
		{Start: 1770, End: 7200, Mode: ModeUnknown},
		{Start: 7200, End: 9000, Mode: ModeNight},
	}, periods)
	assert.Empty(t, changes, "a gap is not a transition")
}

// Recording that stopped before the window ends leaves the rest unknown
func TestRecordingStoppedBeforeEnd(t *testing.T) {
	readings := readingsEvery30s(0, 1800, day)

	periods, _ := buildDayNightTimeline(0, 7200, noMode, readings, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{
		{Start: 0, End: 1770, Mode: ModeDay},
		{Start: 1770, End: 7200, Mode: ModeUnknown},
	}, periods)
}

func TestNoReadingsIsAllUnknown(t *testing.T) {
	periods, _ := buildDayNightTimeline(0, 3600, night, nil, recordingGapLimit)

	assert.Equal(t, []DayNightPeriod{{Start: 0, End: 3600, Mode: ModeUnknown}}, periods)
}
