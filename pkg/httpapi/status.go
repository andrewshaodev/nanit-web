package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/streaming"
)

// hlsURLTemplate - where the dashboard's video for a baby is served
const hlsURLTemplate = "/api/stream/hls/{baby_uid}/playlist.m3u8"

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	res := apitypes.StatusResponse{Timestamp: time.Now().Unix(), Babies: []apitypes.BabyStatus{}}
	for _, b := range s.babies() {
		state := s.State.GetBabyState(b.UID)
		res.Babies = append(res.Babies, apitypes.BabyStatus{
			UID:            b.UID,
			Name:           b.Name,
			CameraUID:      b.CameraUID,
			Temperature:    state.GetTemperature(),
			Humidity:       state.GetHumidity(),
			IsNight:        state.IsNight,
			NightLight:     state.GetNightLight(),
			Standby:        state.GetStandby(),
			WebsocketAlive: state.GetIsWebsocketAlive(),
			StreamState:    int32(state.GetStreamState()),
		})
	}
	writeJSON(w, res)
}

func (s *Server) handleBabies(w http.ResponseWriter, r *http.Request) {
	babies := s.babies()
	res := apitypes.BabiesResponse{Babies: make([]apitypes.Baby, 0, len(babies)), Count: len(babies)}
	for _, b := range babies {
		res.Babies = append(res.Babies, apitypes.Baby{UID: b.UID, Name: b.Name, CameraUID: b.CameraUID})
	}
	writeJSON(w, res)
}

// The RTMP server listens on the address configured through NANIT_RTMP_ADDR,
// which is rarely the host/port the dashboard itself is served from. Clients
// have to be told the real address rather than guessing it from the browser
// location.
func (s *Server) handleStreamingInfo(w http.ResponseWriter, r *http.Request) {
	res := apitypes.StreamingInfoResponse{
		RTMP: apitypes.RTMPInfo{Enabled: s.Config.RTMPPublicAddr != ""},
		HLS:  apitypes.HLSInfo{Enabled: true, URLTemplate: hlsURLTemplate},
	}
	if res.RTMP.Enabled {
		res.RTMP.PublicAddr = s.Config.RTMPPublicAddr
		// The baby UID left as a placeholder, so clients can fill it in
		res.RTMP.URLTemplate = camera.LocalStreamURL(s.Config.RTMPPublicAddr, "{baby_uid}")
	}
	writeJSON(w, res)
}

func (s *Server) handleDeviceInfo(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	state := s.State.GetBabyState(b.UID)
	deviceInfo := state.GetDeviceInfo()

	// Empty rather than nil, so the JSON is [] rather than null
	alerts := []apitypes.DeviceAlert{}
	alert := func(kind, category, message string) {
		alerts = append(alerts, apitypes.DeviceAlert{Type: kind, Message: message, Category: category})
	}

	if !state.GetIsWebsocketAlive() {
		alert("error", "connectivity", "Camera is disconnected from Nanit servers")
	}
	if deviceInfo.StreamingError != nil && *deviceInfo.StreamingError != "" {
		alert("error", "streaming", *deviceInfo.StreamingError)
	}
	if state.StreamState != nil {
		switch *state.StreamState {
		case baby.StreamState_Unhealthy:
			alert("warning", "streaming", "Video streaming is experiencing issues")
		case baby.StreamState_Unknown:
			alert("warning", "streaming", "Video stream status unknown")
		}
	}
	// Streaming blocked by too many mobile apps
	if state.GetStreamRequestState() == baby.StreamRequestState_RequestFailed {
		alert("warning", "connection_limit", "Streaming blocked: Too many Nanit mobile apps connected. Close the official Nanit app on your phone/tablet to enable streaming here.")
	}
	if deviceInfo.SleepMode != nil && *deviceInfo.SleepMode {
		alert("warning", "device_state", "Camera is in sleep mode")
	}
	if deviceInfo.UpgradeDownloaded != nil && *deviceInfo.UpgradeDownloaded {
		alert("warning", "firmware", "Firmware update available for installation")
	}

	writeJSON(w, apitypes.DeviceInfoResponse{
		BabyUID:    b.UID,
		BabyName:   b.Name,
		CameraUID:  b.CameraUID,
		Timestamp:  time.Now().Unix(),
		DeviceInfo: deviceInfo,
		ConnectionStatus: apitypes.ConnectionStatus{
			WebsocketAlive: state.GetIsWebsocketAlive(),
			StreamState:    connectionStreamState(state.StreamState),
		},
		Alerts: alerts,
	})
}

// connectionStreamState - the device page's name for the stream state
func connectionStreamState(state *baby.StreamState) string {
	if state == nil {
		return "unknown"
	}
	switch *state {
	case baby.StreamState_Unhealthy:
		return "unhealthy"
	case baby.StreamState_Alive:
		return "connected"
	default:
		return "unknown"
	}
}

// healthStreamState - the health check's name for the stream state
func healthStreamState(state baby.StreamState) string {
	switch state {
	case baby.StreamState_Alive:
		return "alive"
	case baby.StreamState_Unhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	state := s.State.GetBabyState(b.UID)

	hls := apitypes.HLSHealth{Status: "stopped"}
	if transcoder, exists := s.HLS.GetTranscoder(b.UID); exists {
		hls.IsRunning = transcoder.IsRunning()
		status, streamErr := transcoder.GetStatus()
		hls.Error = streamErr
		if hls.IsRunning {
			switch status {
			case streaming.StatusStreaming, streaming.StatusConnecting, streaming.StatusStarting, streaming.StatusError:
				hls.Status = string(status)
			default:
				hls.Status = "unknown"
			}
		}
	}

	websocket := apitypes.WebsocketHealth{Status: "disconnected", Alive: state.GetIsWebsocketAlive()}
	if websocket.Alive {
		websocket.Status = "connected"
	}

	// Whether video is actually arriving, not just a publisher connected
	rtmp := apitypes.RTMPHealth{
		Status:              "inactive",
		StreamState:         healthStreamState(state.GetStreamState()),
		ActivelyStreaming:   state.IsActivelyStreaming(),
		LastVideoPacketTime: state.GetLastVideoPacketTime(),
	}
	switch {
	case rtmp.ActivelyStreaming:
		rtmp.Status = "active"
	case state.GetStreamState() == baby.StreamState_Alive:
		rtmp.Status = "connected_no_video"
	case state.GetStreamState() == baby.StreamState_Unhealthy:
		rtmp.Status = "unhealthy"
	}

	connected := websocket.Status == "connected"
	overall := "unhealthy"
	switch {
	case connected && rtmp.Status == "active" && hls.Status == "streaming":
		overall = "healthy"
	case connected && (rtmp.Status == "active" || hls.Status == "streaming"):
		overall = "degraded"
	case connected && rtmp.Status == "connected_no_video":
		overall = "connected_no_video"
	case connected || rtmp.Status == "active" || hls.IsRunning:
		overall = "starting"
	}

	writeJSON(w, apitypes.HealthResponse{
		BabyUID:       b.UID,
		OverallHealth: overall,
		Details:       apitypes.HealthDetails{Websocket: websocket, RTMP: rtmp, HLS: hls},
		Timestamp:     time.Now().Unix(),
	})
}

// handleLiveness - GET /health: the process is up
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, apitypes.LivenessResponse{
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
		Uptime:    time.Since(s.StartedAt).Seconds(),
	})
}

// handleReadiness - GET /ready: signed in to Nanit, with cameras
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	authReady := s.Sessions != nil && s.Sessions.RefreshToken() != ""
	babyCount := len(s.babies())
	rtmpReady := s.Config.RTMPPublicAddr != ""

	pick := func(ok bool, yes, no string) string {
		if ok {
			return yes
		}
		return no
	}
	res := apitypes.ReadinessResponse{
		Status:    "ready",
		Timestamp: time.Now().Unix(),
		Services: map[string]apitypes.ServiceReadiness{
			"authentication": {Ready: authReady, Message: pick(authReady, "Authentication configured", "No authentication configured")},
			"babies": {Ready: babyCount > 0, BabyCount: &babyCount,
				Message: pick(babyCount > 0, fmt.Sprintf("%d babies configured", babyCount), "No babies configured")},
			"rtmp": {Ready: rtmpReady, Message: pick(rtmpReady, "RTMP server configured", "RTMP server not configured")},
			"mqtt": {Ready: s.Config.MQTTEnabled, Message: pick(s.Config.MQTTEnabled, "MQTT configured", "MQTT not configured")},
		},
	}

	status := http.StatusOK
	if !authReady || babyCount == 0 {
		res.Status = "not_ready"
		status = http.StatusServiceUnavailable
	}
	writeJSONStatus(w, status, res)
}
