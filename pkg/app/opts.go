package app

import (
	"github.com/andrewshaodev/nanit-web/pkg/mqtt"
	"time"
)

// Opts - application run options
type Opts struct {
	NanitCredentials NanitCredentials
	SessionFile      string
	DataDirectories  DataDirectories
	HTTPPort         int
	// Version - the commit the binary was built from, when CI stamped it
	Version string
	// WebDir - the built dashboard (index.html and assets/)
	WebDir string
	MQTT   *mqtt.Opts
	RTMP   *RTMPOpts
	RTSP   *RTSPOpts
	// StreamServer - this instance asks the cameras to stream; only one
	// may. Others are viewers, taking the server's stream from the stream
	// server set in the dashboard.
	StreamServer bool
	EventPolling EventPollingOpts
	History      HistoryOpts
	WebAuth      WebAuthOpts
}

// NanitCredentials - user credentials for Nanit account
type NanitCredentials struct {
	Email        string
	Password     string
	RefreshToken string
}

// DataDirectories - dictionary of dir paths
type DataDirectories struct {
	BaseDir    string
	HistoryDir string
}

// RTMPOpts - options for RTMP streaming
// RTSPOpts - the RTSP output, for camera software
type RTSPOpts struct {
	// :Port, or IP:Port, to listen on
	ListenAddr string

	// IP:Port players use, shown in the dashboard; "" when unknown
	PublicAddr string
}

// RTMPOpts - options for RTMP streaming
type RTMPOpts struct {
	// IP:Port of the interface on which we should listen
	ListenAddr string

	// IP:Port under which can Cam reach the RTMP server
	PublicAddr string

	// Automatically start streaming when baby comes online
	AutoStart bool
}

type EventPollingOpts struct {
	Enabled         bool
	PollingInterval time.Duration
	MessageTimeout  time.Duration
}

// HistoryOpts - options for historical data tracking
type HistoryOpts struct {
	Enabled        bool
	RetentionDays  int
	CleanupEnabled bool
}

// WebAuthOpts - options for web interface authentication
type WebAuthOpts struct {
	PasswordFile string
}
