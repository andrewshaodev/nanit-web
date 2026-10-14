package camera

import (
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/rs/zerolog/log"
)

// SetNightLight - turns the night light on or off. The command is sent
// without waiting for the camera to answer, as it always was; the answer
// is logged.
func (c *Camera) SetNightLight(on bool) error {
	nightLight := client.Control_LIGHT_OFF
	if on {
		nightLight = client.Control_LIGHT_ON
	}
	return c.send("night light", on, client.RequestType_PUT_CONTROL, &client.Request{
		Control: &client.Control{NightLight: &nightLight},
	})
}

// ToggleNightLight - flips the night light from its last known state, and
// returns the state asked for
func (c *Camera) ToggleNightLight() (bool, error) {
	on := !c.State().GetNightLight()
	return on, c.SetNightLight(on)
}

// SetStandby - puts the camera in standby, or wakes it. Sent like the night
// light.
func (c *Camera) SetStandby(on bool) error {
	return c.send("standby", on, client.RequestType_PUT_SETTINGS, &client.Request{
		Settings: &client.Settings{SleepMode: &on},
	})
}

// ToggleStandby - flips standby from its last known state, and returns the
// state asked for
func (c *Camera) ToggleStandby() (bool, error) {
	on := !c.State().GetStandby()
	return on, c.SetStandby(on)
}

// send - a switch command, with its answer logged in the background
func (c *Camera) send(control string, on bool, reqType client.RequestType, req *client.Request) error {
	conn := c.requester()
	if conn == nil {
		return ErrNotConnected
	}

	sublog := log.With().Str("baby_uid", c.UID()).Str("control", control).Bool("on", on).Logger()
	sublog.Info().Msg("Sending camera command")
	awaitResponse := conn.SendRequest(reqType, req)
	go func() {
		if _, err := awaitResponse(commandTimeout); err != nil {
			sublog.Warn().Err(err).Msg("Camera command got no good answer")
		}
	}()
	return nil
}

// StartTalk - asks the camera to play the talk-back stream at url, session
// sid, through its speaker. Returns once the camera has agreed; playing is
// closed when it says it's playing.
func (c *Camera) StartTalk(url, sid string) (playing <-chan struct{}, err error) {
	conn := c.requester()
	if conn == nil {
		return nil, ErrNotConnected
	}
	ch := make(chan struct{})
	c.mu.Lock()
	if c.talkPlaying == nil {
		c.talkPlaying = map[string]chan struct{}{}
	}
	c.talkPlaying[sid] = ch
	c.mu.Unlock()

	log.Info().Str("baby_uid", c.UID()).Str("sid", sid).Msg("Asking the camera to play talk-back")
	if _, err := conn.SendRequest(client.RequestType_PUT_AUDIO_STREAMING, talkRequest(client.AudioStreaming_STARTED, url, sid))(commandTimeout); err != nil {
		c.forgetTalk(sid)
		return nil, err
	}
	return ch, nil
}

// StopTalk - tells the camera talk-back session sid is over
func (c *Camera) StopTalk(url, sid string) error {
	c.forgetTalk(sid)
	conn := c.requester()
	if conn == nil {
		return ErrNotConnected
	}
	_, err := conn.SendRequest(client.RequestType_PUT_AUDIO_STREAMING, talkRequest(client.AudioStreaming_STOPPED, url, sid))(commandTimeout)
	return err
}

func talkRequest(status client.AudioStreaming_Status, url, sid string) *client.Request {
	return &client.Request{AudioStreaming: &client.AudioStreaming{Status: status.Enum(), RtmpUrl: &url, SessionId: &sid}}
}

func (c *Camera) forgetTalk(sid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.talkPlaying, sid)
}

// talkStarted - the camera says it's playing talk-back session sid
func (c *Camera) talkStarted(sid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.talkPlaying[sid]; ok {
		close(ch)
		delete(c.talkPlaying, sid)
	}
}
