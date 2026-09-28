package history

// The types here are part of the HTTP API, and the dashboard's TypeScript
// is generated from them (see tygo.yaml). Keep API-facing types in this file.

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
	Mode  string `json:"mode" tstype:"'day' | 'night' | 'unknown'"` // ModeDay, ModeNight or ModeUnknown
}

// SensorReading represents a point-in-time sensor measurement
type SensorReading struct {
	ID                 int64    `json:"id"`
	BabyUID            string   `json:"baby_uid"`
	Timestamp          int64    `json:"timestamp"`
	TemperatureCelsius *float64 `json:"temperature_celsius,omitempty"`
	HumidityPercent    *float64 `json:"humidity_percent,omitempty"`
	IsNight            *bool    `json:"is_night,omitempty"`
	CreatedAt          int64    `json:"created_at"`
}

// Event represents a motion or sound event
type Event struct {
	ID        int64  `json:"id"`
	BabyUID   string `json:"baby_uid"`
	Timestamp int64  `json:"timestamp"`
	EventType string `json:"event_type" tstype:"'motion' | 'sound'"`
	CreatedAt int64  `json:"created_at"`
}

// StateChange represents a change in baby state (night light, standby)
type StateChange struct {
	ID         int64  `json:"id"`
	BabyUID    string `json:"baby_uid"`
	Timestamp  int64  `json:"timestamp"`
	StateType  string `json:"state_type"` // "night_light" or "standby"
	StateValue bool   `json:"state_value"`
	CreatedAt  int64  `json:"created_at"`
}

// HistoricalSummary provides aggregated data for a time period
type HistoricalSummary struct {
	BabyUID             string   `json:"baby_uid"`
	StartTime           int64    `json:"start_time"`
	EndTime             int64    `json:"end_time"`
	AvgTemperature      *float64 `json:"avg_temperature,omitempty"`
	MinTemperature      *float64 `json:"min_temperature,omitempty"`
	MaxTemperature      *float64 `json:"max_temperature,omitempty"`
	AvgHumidity         *float64 `json:"avg_humidity,omitempty"`
	MinHumidity         *float64 `json:"min_humidity,omitempty"`
	MaxHumidity         *float64 `json:"max_humidity,omitempty"`
	MotionEventCount    int64    `json:"motion_event_count"`
	SoundEventCount     int64    `json:"sound_event_count"`
	NightLightChanges   int64    `json:"night_light_changes"`
	StandbyChanges      int64    `json:"standby_changes"`
	DayModeMinutes      int64    `json:"day_mode_minutes"`
	NightModeMinutes    int64    `json:"night_mode_minutes"`
	DayModePercentage   float64  `json:"day_mode_percentage"`
	NightModePercentage float64  `json:"night_mode_percentage"`
}

// DayNightAnalytics provides detailed day/night mode analysis
type DayNightAnalytics struct {
	BabyUID               string           `json:"baby_uid"`
	StartTime             int64            `json:"start_time"`
	EndTime               int64            `json:"end_time"`
	TotalMinutes          int64            `json:"total_minutes"`
	DayModeMinutes        int64            `json:"day_mode_minutes"`
	NightModeMinutes      int64            `json:"night_mode_minutes"`
	UnknownModeMinutes    int64            `json:"unknown_mode_minutes"`
	DayModePercentage     float64          `json:"day_mode_percentage"`
	NightModePercentage   float64          `json:"night_mode_percentage"`
	UnknownModePercentage float64          `json:"unknown_mode_percentage"`
	ModeTransitions       int64            `json:"mode_transitions"`
	DayNightChanges       []DayNightChange `json:"day_night_changes"`
	// Periods - the window as consecutive day, night and unknown stretches
	Periods []DayNightPeriod `json:"periods"`
}

// DayNightChange represents a transition between day and night mode
type DayNightChange struct {
	Timestamp    int64 `json:"timestamp"`
	FromNight    bool  `json:"from_night"`
	ToNight      bool  `json:"to_night"`
	DurationMins int64 `json:"duration_mins"`
}
