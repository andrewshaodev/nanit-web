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
