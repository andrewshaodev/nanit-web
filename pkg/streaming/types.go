package streaming

// The types here are part of the HTTP API, and the dashboard's TypeScript
// is generated from them (see tygo.yaml). Keep API-facing types in this file.

// StreamStatus represents the current state of the transcoder
type StreamStatus string

const (
	StatusStarting   StreamStatus = "starting"
	StatusConnecting StreamStatus = "connecting"
	StatusStreaming  StreamStatus = "streaming"
	StatusError      StreamStatus = "error"
	StatusStopped    StreamStatus = "stopped"
)

// StreamError represents different types of streaming errors
type StreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code"`
}
