package soundschedule

// Schedule - a sound the camera plays between two times of day
type Schedule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	// Track - the camera's sound, as SoundStatus lists it ("White Noise.wav")
	Track string `json:"track"`
	// Start and End - "HH:MM", 24-hour, on the bridge's clock. An End
	// before Start runs past midnight.
	Start string `json:"start"`
	End   string `json:"end"`
	// Days - the days it starts on, 0 for Sunday to 6 for Saturday; empty
	// for every day
	Days []int `json:"days"`
}

// Schedules - a camera's schedules, GET and PUT /api/sound/{uid}/schedules
type Schedules struct {
	Schedules []Schedule `json:"schedules"`
	// TimeZone - the bridge's, which the times are in (only in answers)
	TimeZone string `json:"time_zone,omitempty"`
	// UTCOffsetMinutes - the bridge's offset from UTC now, for the
	// dashboard to tell when its own clock differs (only in answers)
	UTCOffsetMinutes int `json:"utc_offset_minutes"`
	// Active - whether this bridge runs schedules: only the stream server
	// does, so two bridges don't both send them (only in answers)
	Active bool `json:"active"`
}
