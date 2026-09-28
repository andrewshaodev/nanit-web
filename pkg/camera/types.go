package camera

// The types here are part of the HTTP API, and the dashboard's TypeScript
// is generated from them (see tygo.yaml). Keep API-facing types in this file.

// SoundPlayback - what the camera is playing
type SoundPlayback struct {
	Playing bool   `json:"playing"`
	Track   string `json:"track,omitempty"`
}

// SoundStatus - the camera's sounds, what's playing and its speaker volume
type SoundStatus struct {
	Tracks   []string       `json:"tracks"`
	Playback *SoundPlayback `json:"playback" tstype:"SoundPlayback | null"`
	Volume   *int32         `json:"volume" tstype:"number | null"`
	// Errors - reads that failed, so a partial answer is still useful
	Errors []string `json:"errors,omitempty"`
}
