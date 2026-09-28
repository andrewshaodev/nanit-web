package baby

// The types here are part of the HTTP API, and the dashboard's TypeScript
// is generated from them (see tygo.yaml). Keep API-facing types in this file.

// DeviceInfo - struct holding device information from Nanit API responses
type DeviceInfo struct {
	FirmwareVersion      *string  `json:"firmware_version,omitempty"`
	HardwareVersion      *string  `json:"hardware_version,omitempty"`
	DeviceMode           *string  `json:"device_mode,omitempty"`
	MountingMode         *int32   `json:"mounting_mode,omitempty"`
	WiFiNetwork          *string  `json:"wifi_network,omitempty"`
	WiFiSignal           *int32   `json:"wifi_signal,omitempty"`    // dBm
	WiFiFrequency        *int32   `json:"wifi_frequency,omitempty"` // MHz
	WiFiBand             *string  `json:"wifi_band,omitempty"`
	NightVision          *bool    `json:"night_vision,omitempty"`
	Volume               *int32   `json:"volume,omitempty"`
	SleepMode            *bool    `json:"sleep_mode,omitempty"`
	StatusLight          *bool    `json:"status_light,omitempty"`
	MicMute              *bool    `json:"mic_mute,omitempty"`
	AntiFlicker          *string  `json:"anti_flicker,omitempty"`
	StreamingError       *string  `json:"streaming_error,omitempty"`
	UpgradeDownloaded    *bool    `json:"upgrade_downloaded,omitempty"`
	AvailableSoundtracks []string `json:"available_soundtracks,omitempty"`

	// Sensor thresholds
	TempLowThreshold      *int32 `json:"temp_low_threshold,omitempty"`
	TempHighThreshold     *int32 `json:"temp_high_threshold,omitempty"`
	HumidityLowThreshold  *int32 `json:"humidity_low_threshold,omitempty"`
	HumidityHighThreshold *int32 `json:"humidity_high_threshold,omitempty"`

	// Stream configuration
	MobileBitrate    *int32 `json:"mobile_bitrate,omitempty"`
	MobileFPS        *int32 `json:"mobile_fps,omitempty"`
	DVRBitrate       *int32 `json:"dvr_bitrate,omitempty"`
	DVRFPS           *int32 `json:"dvr_fps,omitempty"`
	AnalyticsBitrate *int32 `json:"analytics_bitrate,omitempty"`
	AnalyticsFPS     *int32 `json:"analytics_fps,omitempty"`

	// Metadata
	LastUpdated *int64 `json:"last_updated,omitempty"` // Unix timestamp when device info was last updated
}
