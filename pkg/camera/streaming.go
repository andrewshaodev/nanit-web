package camera

// Getting the camera to stream to the bridge, and transcoding what it sends
// for the dashboard.
//
// One loop per camera does all of it, driven by what actually happens: the
// websocket coming up or going down, the camera starting or stopping
// publishing to the RTMP server, a request from the API, and a retry timer.
// It replaces fixed sleeps ahead of each step, a monitor that re-checked
// every 60 s, and two separate requests on every connect.

import (
	"errors"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

const (
	// streamRequestTimeout - how long the camera has to answer a request
	streamRequestTimeout = 30 * time.Second
	// A failed request is retried after streamRetryMin, doubling up to
	// streamRetryMax. Nanit's app-connection limit is the usual failure, and
	// it lifts only when a phone closes the app.
	streamRetryMin = time.Minute
	streamRetryMax = 15 * time.Minute

	streamEventQueueSize = 64
)

// errConnectionLimit - Nanit turned the stream down because too many apps
// are connected to the account
var errConnectionLimit = errors.New("too many app connections")

// ErrNoRTMP - the bridge runs without its RTMP server, so there's nowhere to
// stream to
var ErrNoRTMP = errors.New("RTMP is not enabled")

// transcoder - the HLS side, which turns the RTMP stream into what the
// dashboard plays
type transcoder interface {
	StartTranscoding(babyUID, rtmpURL string) error
	StopTranscoding(babyUID string)
	IsTranscoding(babyUID string) bool
}

type streamEvent interface{ isStreamEvent() }

type (
	// the websocket came up (conn) or went down (nil)
	connectionChanged struct{ conn requester }
	// the camera started or stopped publishing to the RTMP server
	publishingChanged struct{ publishing bool }
	// the API asked for the stream
	streamRequested struct{ reply chan error }
	// a stream request finished
	requestFinished struct{ err error }
)

func (connectionChanged) isStreamEvent() {}
func (publishingChanged) isStreamEvent() {}
func (streamRequested) isStreamEvent()   {}
func (requestFinished) isStreamEvent()   {}

// sendStreamEvent - never blocks: the websocket handler and the state
// subscription both send, and neither may stall
func (c *Camera) sendStreamEvent(e streamEvent) {
	if c.streamEvents == nil {
		return
	}
	select {
	case c.streamEvents <- e:
	default:
		log.Error().Str("baby_uid", c.UID()).Msgf("Stream event queue is full, dropping %T", e)
	}
}

// RequestStream asks the camera to stream to the bridge now, whatever
// NANIT_RTMP_AUTO_START says. The transcoder starts once the camera is
// publishing (straight away if it already is).
func (c *Camera) RequestStream() error {
	if c.streamEvents == nil {
		return ErrNoRTMP
	}
	reply := make(chan error, 1)
	c.sendStreamEvent(streamRequested{reply})
	select {
	case err := <-reply:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("stream loop did not answer")
	}
}

// runStream is the camera's stream loop, until ctx ends
func (c *Camera) runStream(ctx utils.GracefulContext) {
	uid := c.UID()
	sublog := log.With().Str("baby_uid", uid).Logger()

	// Publishing starts and stops show up as the stream state the RTMP
	// server sets. The first call is the current state.
	unsubscribe := c.deps.State.Subscribe(func(babyUID string, state baby.State) {
		if babyUID == uid && state.StreamState != nil {
			c.sendStreamEvent(publishingChanged{*state.StreamState == baby.StreamState_Alive})
		}
	})
	defer unsubscribe()

	var (
		conn       requester
		publishing bool
		requesting bool
		retry      <-chan time.Time
		backoff    = c.streamRetryMin
	)

	startTranscoding := func() {
		if c.deps.HLS == nil || c.deps.HLS.IsTranscoding(uid) {
			return
		}
		if err := c.deps.HLS.StartTranscoding(uid, c.LocalStreamURL()); err != nil {
			sublog.Error().Err(err).Msg("Failed to start HLS transcoding")
		} else {
			sublog.Info().Msg("Started HLS transcoding")
		}
	}

	request := func(why string) {
		if conn == nil || requesting {
			return
		}
		// The state is the authority on publishing. Its changes reach this
		// loop from the subscription's goroutine, so a connect can arrive
		// ahead of the stream coming up, and asking a camera that already
		// publishes makes it open a second stream, cutting off every viewer.
		if c.State().GetStreamState() == baby.StreamState_Alive {
			publishing = true
		}
		if publishing {
			startTranscoding()
			return
		}
		requesting = true
		retry = nil
		sublog.Info().Str("reason", why).Msg("Asking the camera to stream")
		go func(conn requester) {
			c.sendStreamEvent(requestFinished{requestLocalStreaming(uid, c.LocalStreamURL(), client.Streaming_STARTED, conn, c.deps.State)})
		}(conn)
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-retry:
			retry = nil
			request("retrying a failed request")

		case e := <-c.streamEvents:
			switch e := e.(type) {
			case connectionChanged:
				conn = e.conn
				if conn == nil {
					// The camera publishes over its own connection, which a
					// websocket drop doesn't touch, so the stream stays up
					retry = nil
				} else if c.autoStart() {
					request("camera connected")
				}

			case publishingChanged:
				if e.publishing == publishing {
					continue
				}
				publishing = e.publishing
				if publishing {
					backoff = c.streamRetryMin
					retry = nil
					startTranscoding()
				} else {
					// Nothing is feeding the transcoder any more
					if c.deps.HLS != nil {
						c.deps.HLS.StopTranscoding(uid)
					}
					if c.autoStart() {
						request("stream stopped")
					}
				}

			case streamRequested:
				switch {
				case publishing:
					startTranscoding()
					e.reply <- nil
				case conn == nil:
					e.reply <- ErrNotConnected
				default:
					request("asked for from the API")
					e.reply <- nil
				}

			case requestFinished:
				requesting = false
				if e.err == nil || publishing {
					continue
				}
				// Only a camera asked to stream by itself keeps being asked
				if c.autoStart() && conn != nil {
					sublog.Warn().Err(e.err).Dur("retry_in", backoff).Msg("Stream request failed, will retry")
					retry = time.After(backoff)
					backoff = min(backoff*2, c.streamRetryMax)
				}
			}
		}
	}
}

// requestLocalStreaming asks the camera to start or stop streaming to
// targetURL, and records the outcome in the state for the dashboard. A
// request that times out is sent again while the websocket is up.
func requestLocalStreaming(babyUID string, targetURL string, streamingStatus client.Streaming_Status, conn requester, stateManager *baby.StateManager) error {
	for {
		switch streamingStatus {
		case client.Streaming_STARTED:
			log.Info().Str("target", targetURL).Msg("Requesting local streaming")
		case client.Streaming_PAUSED:
			log.Info().Str("target", targetURL).Msg("Pausing local streaming")
		case client.Streaming_STOPPED:
			log.Info().Str("target", targetURL).Msg("Stopping local streaming")
		}

		awaitResponse := conn.SendRequest(client.RequestType_PUT_STREAMING, &client.Request{
			Streaming: &client.Streaming{
				Id:       client.StreamIdentifier(client.StreamIdentifier_MOBILE).Enum(),
				RtmpUrl:  utils.ConstRefStr(targetURL),
				Status:   client.Streaming_Status(streamingStatus).Enum(),
				Attempts: utils.ConstRefInt32(1),
			},
		})

		_, err := awaitResponse(streamRequestTimeout)
		if err == nil {
			log.Info().Msg("Local streaming successfully requested")
			stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_Requested))
			return nil
		}

		switch {
		case err.Error() == "Forbidden: Number of Mobile App connections above limit, declining connection":
			log.Warn().Err(err).Msg("Too many app connections to stream")
			stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_RequestFailed))
			return errConnectionLimit

		case err.Error() != "Request timeout":
			if stateManager.GetBabyState(babyUID).GetStreamState() == baby.StreamState_Alive {
				log.Info().Err(err).Msg("Failed to request local streaming, but stream seems to be alive from previous run")
				return nil
			}
			log.Warn().Err(err).Msg("Failed to request local streaming")
			stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_RequestFailed))
			return err

		case !stateManager.GetBabyState(babyUID).GetIsWebsocketAlive():
			return err
		}

		log.Warn().Msg("Streaming request timeout, trying again")
	}
}
