// Package apitypes - the JSON the HTTP API sends and accepts. The
// dashboard's TypeScript types are generated from these (see tygo.yaml),
// so changing one changes the frontend's too.
package apitypes

import (
	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/streaming"
)

// ErrorResponse - every error the API sends: a short code, and a message
// for people
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// MessageResponse - an action that went through
type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Baby - a baby profile on the Nanit account, with its camera
type Baby struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	CameraUID string `json:"camera_uid"`
}

// BabyStatus - a camera's live readings and connection
type BabyStatus struct {
	UID            string  `json:"uid"`
	Name           string  `json:"name"`
	CameraUID      string  `json:"camera_uid"`
	Temperature    float64 `json:"temperature"`
	Humidity       float64 `json:"humidity"`
	IsNight        *bool   `json:"is_night" tstype:"boolean | null"`
	NightLight     bool    `json:"night_light"`
	Standby        bool    `json:"standby"`
	WebsocketAlive bool    `json:"websocket_alive"`
	// 0 unknown, 1 unhealthy, 2 alive
	StreamState int32 `json:"stream_state"`
}

// StatusResponse - GET /api/status
type StatusResponse struct {
	Timestamp int64        `json:"timestamp"`
	Babies    []BabyStatus `json:"babies"`
}

// BabiesResponse - GET /api/babies
type BabiesResponse struct {
	Babies []Baby `json:"babies"`
	Count  int    `json:"count"`
}

// RTMPInfo - where the RTMP stream is served, when it is
type RTMPInfo struct {
	Enabled     bool   `json:"enabled"`
	PublicAddr  string `json:"public_addr,omitempty"`
	URLTemplate string `json:"url_template,omitempty"`
}

// HLSInfo - where the dashboard's video comes from
type HLSInfo struct {
	Enabled     bool   `json:"enabled"`
	URLTemplate string `json:"url_template"`
}

// StreamingInfoResponse - GET /api/streaming/info
type StreamingInfoResponse struct {
	RTMP RTMPInfo `json:"rtmp"`
	HLS  HLSInfo  `json:"hls"`
}

// ControlRequest - POST /api/control/night-light and /api/control/standby
type ControlRequest struct {
	BabyUID string `json:"baby_uid"`
	Action  string `json:"action" tstype:"'toggle'"`
}

// ControlResponse - a control command that was sent
type ControlResponse struct {
	Success   bool   `json:"success"`
	BabyUID   string `json:"baby_uid"`
	Control   string `json:"control"`
	Action    string `json:"action"`
	Timestamp int64  `json:"timestamp"`
}

// DeviceAlert - an error or warning about a camera
type DeviceAlert struct {
	Type     string `json:"type" tstype:"'error' | 'warning'"`
	Message  string `json:"message"`
	Category string `json:"category"`
}

// ConnectionStatus - a camera's connection, for its device page
type ConnectionStatus struct {
	WebsocketAlive bool   `json:"websocket_alive"`
	StreamState    string `json:"stream_state" tstype:"'connected' | 'unhealthy' | 'unknown'"`
}

// DeviceInfoResponse - GET /api/device-info/{uid}
type DeviceInfoResponse struct {
	BabyUID          string           `json:"baby_uid"`
	BabyName         string           `json:"baby_name"`
	CameraUID        string           `json:"camera_uid"`
	Timestamp        int64            `json:"timestamp"`
	DeviceInfo       *baby.DeviceInfo `json:"device_info" tstype:"baby.DeviceInfo"`
	ConnectionStatus ConnectionStatus `json:"connection_status"`
	Alerts           []DeviceAlert    `json:"alerts"`
}

// LoginRequest - POST /api/auth/login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse - Nanit either sent a code (MFAToken, Channel), or signed
// in straight away (SignedIn)
type LoginResponse struct {
	Success     bool   `json:"success"`
	SignedIn    bool   `json:"signed_in,omitempty"`
	MFAToken    string `json:"mfa_token,omitempty"`
	Channel     string `json:"channel,omitempty"` // "sms" or "email", as Nanit says
	PhoneSuffix string `json:"phone_suffix,omitempty"`
	Message     string `json:"message"`
}

// Verify2FARequest - POST /api/auth/verify-2fa
type Verify2FARequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	MFAToken string `json:"mfa_token"`
	MFACode  string `json:"mfa_code"`
	Channel  string `json:"channel"`
}

// AuthStatusResponse - GET /api/auth/status: the Nanit sign-in
type AuthStatusResponse struct {
	Authenticated   bool   `json:"authenticated"`
	Message         string `json:"message"`
	Email           string `json:"email"`
	BabiesCount     int    `json:"babies_count"`
	ServicesRunning bool   `json:"services_running"`
	AuthTime        *int64 `json:"auth_time,omitempty"`
}

// WebAuthStatusResponse - GET /api/webauth/status: the dashboard password
type WebAuthStatusResponse struct {
	PasswordProtectionEnabled bool `json:"password_protection_enabled"`
	PasswordSet               bool `json:"password_set"`
	Authenticated             bool `json:"authenticated"`
}

// PasswordRequest - POST /api/webauth/login, set-password, remove-password
type PasswordRequest struct {
	Password string `json:"password"`
}

// ChangePasswordRequest - POST /api/webauth/change-password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// StreamRequest - POST /api/stream/start and /api/stream/stop
type StreamRequest struct {
	BabyUID string `json:"baby_uid"`
}

// StreamResponse - a stream start or stop that went through
type StreamResponse struct {
	Success bool   `json:"success"`
	BabyUID string `json:"baby_uid"`
	HLSURL  string `json:"hls_url,omitempty"`
	Message string `json:"message"`
}

// StreamErrorInfo - why a stream isn't available
type StreamErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// StreamStatusResponse - GET /api/stream/status/{uid}. Status is
// "blocked" or "not_found" (with a Message), or the transcoder's status,
// with its details.
type StreamStatusResponse struct {
	BabyUID     string                 `json:"baby_uid"`
	Status      string                 `json:"status"`
	Message     string                 `json:"message,omitempty"`
	StreamError *StreamErrorInfo       `json:"stream_error,omitempty"`
	IsRunning   *bool                  `json:"is_running,omitempty"`
	StartTime   string                 `json:"start_time,omitempty"`
	RetryCount  *int                   `json:"retry_count,omitempty"`
	MaxRetries  *int                   `json:"max_retries,omitempty"`
	Error       *streaming.StreamError `json:"error,omitempty"`
	Uptime      *float64               `json:"uptime,omitempty"`
	HasFiles    *bool                  `json:"has_files,omitempty"`
}

// HLSErrorResponse - why a stream file couldn't be served
type HLSErrorResponse struct {
	Error       string                 `json:"error"`
	Message     string                 `json:"message"`
	Status      string                 `json:"status,omitempty"`
	File        string                 `json:"file,omitempty"`
	StreamError *streaming.StreamError `json:"stream_error,omitempty"`
}

// SensorHistoryResponse - GET /api/history/sensor/{uid}
type SensorHistoryResponse struct {
	BabyUID   string                  `json:"baby_uid"`
	StartTime int64                   `json:"start_time"`
	EndTime   int64                   `json:"end_time"`
	Readings  []history.SensorReading `json:"readings"`
	Count     int                     `json:"count"`
}

// EventsHistoryResponse - GET /api/history/events/{uid}
type EventsHistoryResponse struct {
	BabyUID   string          `json:"baby_uid"`
	StartTime int64           `json:"start_time"`
	EndTime   int64           `json:"end_time"`
	EventType string          `json:"event_type"`
	Events    []history.Event `json:"events"`
	Count     int             `json:"count"`
}

// DayNightResponse - GET /api/history/day-night/{uid}
type DayNightResponse struct {
	BabyUID   string                     `json:"baby_uid"`
	StartTime int64                      `json:"start_time"`
	EndTime   int64                      `json:"end_time"`
	DayNight  *history.DayNightAnalytics `json:"day_night" tstype:"history.DayNightAnalytics"`
}

// HistoryResetResponse - DELETE /api/history/reset/{uid}
type HistoryResetResponse struct {
	Success bool   `json:"success"`
	BabyUID string `json:"baby_uid"`
	Message string `json:"message"`
}

// WebsocketHealth - the camera's connection to Nanit
type WebsocketHealth struct {
	Status string `json:"status" tstype:"'connected' | 'disconnected'"`
	Alive  bool   `json:"alive"`
}

// RTMPHealth - the camera's stream to the bridge
type RTMPHealth struct {
	Status              string `json:"status" tstype:"'active' | 'connected_no_video' | 'unhealthy' | 'inactive'"`
	StreamState         string `json:"stream_state" tstype:"'alive' | 'unhealthy' | 'unknown'"`
	ActivelyStreaming   bool   `json:"actively_streaming"`
	LastVideoPacketTime *int64 `json:"last_video_packet_time" tstype:"number | null"`
}

// HLSHealth - the transcoder feeding the dashboard
type HLSHealth struct {
	Status    string                 `json:"status" tstype:"'streaming' | 'connecting' | 'starting' | 'error' | 'stopped' | 'unknown'"`
	IsRunning bool                   `json:"is_running"`
	Error     *streaming.StreamError `json:"error,omitempty"`
}

// HealthDetails - each part of a camera's video path
type HealthDetails struct {
	Websocket WebsocketHealth `json:"websocket"`
	RTMP      RTMPHealth      `json:"rtmp"`
	HLS       HLSHealth       `json:"hls"`
}

// HealthResponse - GET /api/health/{uid}
type HealthResponse struct {
	BabyUID       string        `json:"baby_uid"`
	OverallHealth string        `json:"overall_health" tstype:"'healthy' | 'degraded' | 'connected_no_video' | 'starting' | 'unhealthy'"`
	Details       HealthDetails `json:"details"`
	Timestamp     int64         `json:"timestamp"`
}

// LivenessResponse - GET /health
type LivenessResponse struct {
	Status    string  `json:"status"`
	Timestamp int64   `json:"timestamp"`
	Uptime    float64 `json:"uptime"`
	// The commit the bridge was built from, for builds CI made
	Version string `json:"version,omitempty"`
}

// ServiceReadiness - one part of the bridge, for /ready
type ServiceReadiness struct {
	Ready     bool   `json:"ready"`
	Message   string `json:"message"`
	BabyCount *int   `json:"baby_count,omitempty"`
}

// ReadinessResponse - GET /ready
type ReadinessResponse struct {
	Status    string                      `json:"status" tstype:"'ready' | 'not_ready'"`
	Timestamp int64                       `json:"timestamp"`
	Services  map[string]ServiceReadiness `json:"services"`
}
