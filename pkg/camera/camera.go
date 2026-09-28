// Package camera - one Nanit camera: its websocket connection, its state,
// its stream and the commands it accepts. The HTTP API and MQTT both drive a
// camera through these methods.
package camera

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/message"
	"github.com/andrewshaodev/nanit-web/pkg/mqtt"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// ErrNotConnected - the camera's websocket isn't up, so it can't take commands
var ErrNotConnected = errors.New("camera not connected")

// Options - how the bridge is set up, as far as its cameras care
type Options struct {
	// RTMP - nil when the RTMP server is off
	RTMP         *RTMPOptions
	EventPolling EventPollingOptions
}

// RTMPOptions - the RTMP server the camera streams to
type RTMPOptions struct {
	// IP:Port under which the camera can reach the RTMP server
	PublicAddr string
	// Ask the camera to stream as soon as it connects, and again if the
	// stream drops
	AutoStart bool
}

// EventPollingOptions - polling Nanit for motion and sound events
type EventPollingOptions struct {
	Enabled        bool
	Interval       time.Duration
	MessageTimeout time.Duration
}

// Deps - the bridge's parts a camera works with
type Deps struct {
	State *baby.StateManager
	Nanit *client.NanitClient
	HLS   transcoder
	// MQTT - nil when MQTT is off
	MQTT *mqtt.Connection
}

// requester - the part of a websocket connection that commands need, so
// tests can check what a command sends without a camera
type requester interface {
	SendRequest(reqType client.RequestType, request *client.Request) func(time.Duration) (*client.Response, error)
}

// Camera - one camera, and the baby profile it belongs to
type Camera struct {
	Baby baby.Baby
	opts Options
	deps Deps

	mu   sync.RWMutex
	conn requester // nil while the websocket is down

	// For the stream loop; nil without RTMP
	streamEvents   chan streamEvent
	streamRetryMin time.Duration
	streamRetryMax time.Duration
}

// New - a camera that isn't running yet; Run connects it
func New(b baby.Baby, opts Options, deps Deps) *Camera {
	c := &Camera{Baby: b, opts: opts, deps: deps, streamRetryMin: streamRetryMin, streamRetryMax: streamRetryMax}
	if opts.RTMP != nil {
		c.streamEvents = make(chan streamEvent, streamEventQueueSize)
	}
	return c
}

// UID - the baby profile's UID, which names the camera throughout the bridge
func (c *Camera) UID() string {
	return c.Baby.UID
}

// State - the camera's current state
func (c *Camera) State() *baby.State {
	return c.deps.State.GetBabyState(c.UID())
}

// requester - the live connection, or nil
func (c *Camera) requester() requester {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *Camera) setConnection(conn requester) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

// Run keeps the camera connected until ctx ends
func (c *Camera) Run(ctx utils.GracefulContext) {
	// The websocket carries the readings and the commands; without RTMP or
	// MQTT there's nothing to use them for
	if c.opts.RTMP == nil && c.deps.MQTT == nil {
		<-ctx.Done()
		return
	}

	uid := c.UID()
	ws := client.NewWebsocketConnectionManager(uid, c.Baby.CameraUID, c.deps.Nanit, c.deps.State)

	ws.WithReadyConnection(func(conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
		c.setConnection(conn)
		defer c.setConnection(nil)
		c.runWebsocket(conn, childCtx)
	})

	if c.opts.RTMP != nil {
		ctx.RunAsChild(c.runStream)
	}

	if c.opts.EventPolling.Enabled {
		ctx.RunAsChild(c.pollMessages)
	}

	ctx.RunAsChild(func(childCtx utils.GracefulContext) {
		ws.RunWithinContext(childCtx)
	})

	<-ctx.Done()
}

// autoStart - whether the camera should be asked to stream by itself
func (c *Camera) autoStart() bool {
	return c.opts.RTMP != nil && c.opts.RTMP.AutoStart
}

// pollMessages checks Nanit for new motion and sound events until ctx ends
func (c *Camera) pollMessages(ctx utils.GracefulContext) {
	for {
		c.pollMessagesOnce()

		select {
		case <-ctx.Done():
			return
		case <-time.After(c.opts.EventPolling.Interval):
		}
	}
}

func (c *Camera) pollMessagesOnce() {
	uid := c.UID()
	newMessages, err := c.deps.Nanit.FetchNewMessages(uid, c.opts.EventPolling.MessageTimeout)
	if err != nil {
		log.Error().Err(err).Str("baby_uid", uid).Msg("Failed to fetch new messages")
		return
	}

	for _, msg := range newMessages {
		switch msg.Type {
		case message.SoundEventMessageType:
			c.deps.State.NotifySoundSubscribers(uid, time.Time(msg.Time))
		case message.MotionEventMessageType:
			c.deps.State.NotifyMotionSubscribers(uid, time.Time(msg.Time))
		}
	}
}

func (c *Camera) runWebsocket(conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
	uid := c.UID()
	stateManager := c.deps.State

	// Reading sensor data
	conn.RegisterMessageHandler(func(m *client.Message, conn *client.WebsocketConnection) {
		// Sensor request initiated by us on start (or some other client, we don't care)
		if *m.Type == client.Message_RESPONSE && m.Response != nil {
			if *m.Response.RequestType == client.RequestType_GET_SENSOR_DATA && len(m.Response.SensorData) > 0 {
				processSensorData(uid, m.Response.SensorData, stateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_CONTROL && m.Response.Control != nil {
				processLight(uid, m.Response.Control, stateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_SETTINGS && m.Response.Settings != nil {
				processStandby(uid, m.Response.Settings, stateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_STATUS && m.Response.Status != nil {
				processStatus(uid, m.Response.Status, stateManager)
			}
		} else

		// Communication initiated from a cam
		// Note: it sends the updates periodically on its own + whenever some significant change occurs
		if *m.Type == client.Message_REQUEST && m.Request != nil {
			if *m.Request.Type == client.RequestType_PUT_SENSOR_DATA && len(m.Request.SensorData_) > 0 {
				processSensorData(uid, m.Request.SensorData_, stateManager)
			} else if *m.Request.Type == client.RequestType_PUT_CONTROL && m.Request.Control != nil {
				processLight(uid, m.Request.Control, stateManager)
			} else if *m.Request.Type == client.RequestType_PUT_SETTINGS && m.Request.Settings != nil {
				processStandby(uid, m.Request.Settings, stateManager)
			}
		}
	})

	// MQTT switch commands for this camera, while it's connected. They call
	// the same methods as the HTTP API.
	if c.deps.MQTT != nil {
		unregister := c.deps.MQTT.RegisterBaby(uid, mqtt.CommandHandlers{
			NightLight: func(on bool) { _ = c.SetNightLight(on) },
			Standby:    func(on bool) { _ = c.SetStandby(on) },
		})
		defer unregister()
	}

	// Get the initial state of the light
	conn.SendRequest(client.RequestType_GET_CONTROL, &client.Request{GetControl_: &client.GetControl{
		NightLight: utils.ConstRefBool(true),
	}})

	// Ask for sensor data (initial request)
	conn.SendRequest(client.RequestType_GET_SENSOR_DATA, &client.Request{
		GetSensorData: &client.GetSensorData{
			All: utils.ConstRefBool(true),
		},
	})

	// Ask for status
	conn.SendRequest(client.RequestType_GET_STATUS, &client.Request{
		GetStatus_: &client.GetStatus{
			All: utils.ConstRefBool(true),
		},
	})

	// Ask for settings to get device configuration
	conn.SendRequest(client.RequestType_GET_SETTINGS, &client.Request{})

	// The stream loop takes it from here: asking the camera to stream (with
	// NANIT_RTMP_AUTO_START), and transcoding once it does
	c.sendStreamEvent(connectionChanged{conn})

	<-childCtx.Done()

	// Tell the loop first: stopping the stream below ends the camera's
	// publishing, which it would otherwise answer by asking for it again
	c.sendStreamEvent(connectionChanged{nil})

	// Stop local streaming on a deliberate shutdown, while the socket can
	// still carry the request
	if c.opts.RTMP != nil {
		state := stateManager.GetBabyState(uid)
		if state.GetIsWebsocketAlive() && state.GetStreamState() == baby.StreamState_Alive {
			requestLocalStreaming(uid, c.LocalStreamURL(), client.Streaming_STOPPED, conn, stateManager)
		}
	}
}

// localStreamURLTemplate - shape of the RTMP URL served by the built-in RTMP
// server, both for the cam to publish to and for clients (VLC, Home Assistant,
// ffmpeg) to subscribe to.
const localStreamURLTemplate = "rtmp://{publicAddr}/local/{babyUid}"

// LocalStreamURL - the RTMP URL for babyUID's stream on the bridge at
// publicAddr
func LocalStreamURL(publicAddr, babyUID string) string {
	return strings.NewReplacer("{publicAddr}", publicAddr, "{babyUid}", babyUID).Replace(localStreamURLTemplate)
}

// LocalStreamURL - where this camera streams to, or "" without RTMP
func (c *Camera) LocalStreamURL() string {
	if c.opts.RTMP == nil {
		return ""
	}
	return LocalStreamURL(c.opts.RTMP.PublicAddr, c.UID())
}
