// The API's types are generated from the Go structs it sends (see
// tygo.yaml, and run `go generate ./pkg/httpapi/apitypes` after changing
// them). This file gives them the names the dashboard uses.
import type * as api from './generated/api'
import type * as baby from './generated/baby'
import type * as camera from './generated/camera'
import type * as history from './generated/history'

// A camera's live readings, as /api/status sends them. stream_state is the
// backend's number; see streamStatus()
export type Baby = api.BabyStatus
export type StatusResponse = api.StatusResponse

export type DeviceInfo = baby.DeviceInfo
export type DeviceAlert = api.DeviceAlert
export type DeviceInfoResponse = api.DeviceInfoResponse

// The camera's built-in sounds, from /api/sound/{baby_uid}
export type SoundStatus = camera.SoundStatus

export type SensorReading = history.SensorReading
export type SensorDataResponse = api.SensorHistoryResponse
export type HistorySummary = history.HistoricalSummary
export type DayNightChange = history.DayNightChange
export type DayNightAnalytics = history.DayNightAnalytics
export type DayNightPeriod = history.DayNightPeriod

export type ControlRequest = api.ControlRequest
export type ControlResponse = api.ControlResponse

export type LoginRequest = api.LoginRequest
export type LoginResponse = api.LoginResponse
export type Verify2FARequest = api.Verify2FARequest
export type Verify2FAResponse = api.MessageResponse
export type AuthStatusResponse = api.AuthStatusResponse
export type AuthResetResponse = api.MessageResponse

export type StreamStartRequest = api.StreamRequest
export type StreamStartResponse = api.StreamResponse
export type StreamStatusResponse = api.StreamStatusResponse
export type StreamError = api.StreamErrorInfo
export type StreamingInfoResponse = api.StreamingInfoResponse

export type WebAuthStatusResponse = api.WebAuthStatusResponse
export type WebAuthResponse = api.MessageResponse

export type HealthDetails = api.HealthDetails
export type HealthResponse = api.HealthResponse
