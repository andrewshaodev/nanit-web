package camera

import (
	"fmt"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// The camera's built-in sounds (white noise, birds, waves...), played over its
// speaker. The protocol fields were worked out by maddijoyce/nanit-web.
//
// Nothing here runs on its own: every command is an explicit API call, and the
// bridge never starts or resumes a sound by itself.

const (
	soundRequestTimeout = 10 * time.Second

	// commandTimeout - how long a switch command's answer is waited for
	commandTimeout = 10 * time.Second

	// soundDurationForever - Playback.duration for "keep playing"; the Nanit
	// app loops a sound this way
	soundDurationForever int32 = -1

	minVolume int32 = 0
	maxVolume int32 = 100
)

// sound request helpers, all against a live camera connection

func requestSound(conn requester, reqType client.RequestType, req *client.Request) (*client.Response, error) {
	res, err := conn.SendRequest(reqType, req)(soundRequestTimeout)
	if err != nil {
		return nil, err
	}
	if code := res.GetStatusCode(); code != 0 && code != 200 {
		return nil, fmt.Errorf("camera answered %d: %s", code, res.GetStatusMessage())
	}
	return res, nil
}

func trackName(s *client.Soundtrack) string {
	if s == nil {
		return ""
	}
	return s.GetName()
}

func readSoundStatus(conn requester) SoundStatus {
	status := SoundStatus{Tracks: []string{}}

	if res, err := requestSound(conn, client.RequestType_GET_SOUNDTRACKS, &client.Request{}); err != nil {
		status.Errors = append(status.Errors, "tracks: "+err.Error())
	} else {
		for _, track := range res.GetSoundtracks() {
			if name := track.GetName(); name != "" {
				status.Tracks = append(status.Tracks, name)
			}
		}
	}

	if res, err := requestSound(conn, client.RequestType_GET_PLAYBACK, &client.Request{}); err != nil {
		status.Errors = append(status.Errors, "playback: "+err.Error())
	} else if pb := res.GetPlayback(); pb != nil {
		playing := pb.GetStatus() == client.Playback_STARTED
		track := trackName(pb.GetSelectedSoundtrack())
		if track == "" {
			track = trackName(pb.GetSoundtrack())
		}
		status.Playback = &SoundPlayback{Playing: playing, Track: track}
	}

	if res, err := requestSound(conn, client.RequestType_GET_SETTINGS, &client.Request{
		// GetSettings_: protoc-gen-go's name, avoiding the GetSettings() getter
		GetSettings_: &client.GetSettings{All: utils.ConstRefBool(true)},
	}); err != nil {
		status.Errors = append(status.Errors, "volume: "+err.Error())
	} else if settings := res.GetSettings(); settings != nil && settings.Volume != nil {
		volume := settings.GetVolume()
		status.Volume = &volume
	}

	return status
}

// playDuration - the Playback.duration for a request: -1 (forever) unless a
// positive number of seconds was asked for
func playDuration(seconds int32) int32 {
	if seconds > 0 {
		return seconds
	}
	return soundDurationForever
}

func clampVolume(level int32) int32 {
	if level < minVolume {
		return minVolume
	}
	if level > maxVolume {
		return maxVolume
	}
	return level
}

// SoundStatus - the camera's sounds, what's playing and its speaker volume.
// Each is its own request, and a part that fails is listed in Errors.
func (c *Camera) SoundStatus() (SoundStatus, error) {
	conn := c.requester()
	if conn == nil {
		return SoundStatus{}, ErrNotConnected
	}
	return readSoundStatus(conn), nil
}

// PlaySound - plays track for seconds, or until stopped when seconds isn't
// positive, and returns the status after
func (c *Camera) PlaySound(track string, seconds int32) (SoundStatus, error) {
	conn := c.requester()
	if conn == nil {
		return SoundStatus{}, ErrNotConnected
	}
	duration := playDuration(seconds)
	sublog := log.With().Str("baby_uid", c.UID()).Logger()
	sublog.Info().Str("track", track).Int32("duration_seconds", duration).Msg("Playing sound")
	if _, err := requestSound(conn, client.RequestType_PUT_PLAYBACK, &client.Request{
		Playback: &client.Playback{
			Status:             client.Playback_STARTED.Enum(),
			Duration:           &duration,
			Soundtrack:         &client.Soundtrack{Type: utils.ConstRefInt32(0), Name: &track},
			SelectedSoundtrack: &client.Soundtrack{Type: utils.ConstRefInt32(0), Name: &track},
		},
	}); err != nil {
		sublog.Warn().Err(err).Msg("Playing sound failed")
		return SoundStatus{}, err
	}
	return readSoundStatus(conn), nil
}

// StopSound - stops what's playing, and returns the status after
func (c *Camera) StopSound() (SoundStatus, error) {
	conn := c.requester()
	if conn == nil {
		return SoundStatus{}, ErrNotConnected
	}
	sublog := log.With().Str("baby_uid", c.UID()).Logger()
	sublog.Info().Msg("Stopping sound")
	if _, err := requestSound(conn, client.RequestType_PUT_PLAYBACK, &client.Request{
		Playback: &client.Playback{Status: client.Playback_STOPPED.Enum()},
	}); err != nil {
		sublog.Warn().Err(err).Msg("Stopping sound failed")
		return SoundStatus{}, err
	}
	return readSoundStatus(conn), nil
}

// SetVolume - sets the speaker volume, clamped to 0-100, and returns the
// status after
func (c *Camera) SetVolume(volume int32) (SoundStatus, error) {
	conn := c.requester()
	if conn == nil {
		return SoundStatus{}, ErrNotConnected
	}
	volume = clampVolume(volume)
	sublog := log.With().Str("baby_uid", c.UID()).Logger()
	sublog.Info().Int32("volume", volume).Msg("Setting speaker volume")
	if _, err := requestSound(conn, client.RequestType_PUT_SETTINGS, &client.Request{
		Settings: &client.Settings{Volume: &volume},
	}); err != nil {
		sublog.Warn().Err(err).Msg("Setting volume failed")
		return SoundStatus{}, err
	}
	return readSoundStatus(conn), nil
}
