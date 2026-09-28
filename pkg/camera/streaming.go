package camera

// Asking the camera to stream to the bridge, and the HLS transcoding that
// follows it

import (
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

func requestLocalStreaming(babyUID string, targetURL string, streamingStatus client.Streaming_Status, conn *client.WebsocketConnection, stateManager *baby.StateManager) {
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

		_, err := awaitResponse(30 * time.Second)

		if err != nil {
			if err.Error() == "Forbidden: Number of Mobile App connections above limit, declining connection" {
				log.Warn().Err(err).Msg("Too many app connections, will retry via background monitor...")
				stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_RequestFailed))
				return // Exit and let the retry monitor handle it
			} else if err.Error() != "Request timeout" {
				if stateManager.GetBabyState(babyUID).GetStreamState() == baby.StreamState_Alive {
					log.Info().Err(err).Msg("Failed to request local streaming, but stream seems to be alive from previous run")
				} else if stateManager.GetBabyState(babyUID).GetStreamState() == baby.StreamState_Unhealthy {
					log.Error().Err(err).Msg("Failed to request local streaming and stream seems to be dead")
					stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_RequestFailed))
				} else {
					log.Warn().Err(err).Msg("Failed to request local streaming, awaiting stream health check")
					stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_RequestFailed))
				}

				return
			}

			if !stateManager.GetBabyState(babyUID).GetIsWebsocketAlive() {
				return
			}

			log.Warn().Msg("Streaming request timeout, trying again")

		} else {
			log.Info().Msg("Local streaming successfully requested")
			stateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_Requested))
			return
		}
	}
}

// shouldRequestStream - whether the cam still needs to be asked to publish.
//
// Asking for a stream the cam is already publishing makes it open a second
// publisher connection, and registering that one closes every subscriber of the
// first, so a stream that is already alive is left alone.
func shouldRequestStream(state *baby.State) bool {
	return state.GetStreamState() != baby.StreamState_Alive
}

// shouldReleaseStreamResources - whether a websocket ending means the stream is
// really over, rather than a reconnect the cam publishes straight through.
//
// The cam publishes over its own connection, so a dropped websocket says
// nothing about the stream. A websocket that is still alive means we are
// shutting down on purpose; a stream that is no longer alive means the cam
// stopped on its own. Either way there is nothing left to keep running.
func shouldReleaseStreamResources(state *baby.State) bool {
	return state.GetIsWebsocketAlive() || state.GetStreamState() != baby.StreamState_Alive
}

// autoStartStreaming automatically starts RTMP streaming and HLS transcoding when a baby comes online
func (c *Camera) autoStartStreaming(conn *client.WebsocketConnection) {
	babyUID := c.UID()
	// Give the WebSocket connection a moment to fully establish
	time.Sleep(2 * time.Second)

	// Get the RTMP URL for this baby
	streamURL := c.LocalStreamURL()
	if streamURL == "" {
		log.Error().Str("baby_uid", babyUID).Msg("Cannot auto-start streaming: no RTMP URL available")
		return
	}

	// Asking for a stream the cam is already publishing makes it open a second
	// publisher connection, and registering that one closes every subscriber of
	// the first. After a reconnect the existing stream is usually still running,
	// so the request is only worth making when it is not.
	if !shouldRequestStream(c.deps.State.GetBabyState(babyUID)) {
		log.Info().
			Str("baby_uid", babyUID).
			Msg("Cam is already publishing, leaving the existing stream alone")
	} else {
		log.Info().
			Str("baby_uid", babyUID).
			Str("rtmp_url", streamURL).
			Msg("Auto-starting RTMP streaming")

		requestLocalStreaming(babyUID, streamURL, client.Streaming_STARTED, conn, c.deps.State)
	}

	// Start HLS transcoding for instant playback
	if c.deps.HLS != nil {
		// Give RTMP stream a moment to establish before starting HLS transcoding
		go func() {
			time.Sleep(3 * time.Second)

			// A transcoder left running against a still-live stream is already
			// producing what we would be restarting it for.
			if transcoder, exists := c.deps.HLS.GetTranscoder(babyUID); exists && transcoder.IsRunning() {
				log.Debug().Str("baby_uid", babyUID).Msg("HLS transcoding is already running")
				return
			}

			if err := c.deps.HLS.StartTranscoding(babyUID, streamURL); err != nil {
				log.Error().
					Err(err).
					Str("baby_uid", babyUID).
					Msg("Failed to auto-start HLS transcoding")
			} else {
				log.Info().
					Str("baby_uid", babyUID).
					Msg("Auto-started HLS transcoding for instant playback")
			}
		}()
	}
}

// autoStopStreaming releases streaming resources once the websocket handler ends.
//
// The cam publishes RTMP over its own connection, which a websocket drop does
// not touch. Tearing the stream down here closed every subscriber and cut off
// anything consuming the stream, once per reconnect, so a cam that is still
// publishing is now left exactly as it is. Telling the cam to stop on a
// deliberate shutdown is runWebsocket's cleanup, which still does it while the
// socket can still carry the request.
func (c *Camera) autoStopStreaming() {
	babyUID := c.UID()
	if !shouldReleaseStreamResources(c.deps.State.GetBabyState(babyUID)) {
		log.Info().
			Str("baby_uid", babyUID).
			Msg("Websocket ended while the cam is still publishing, leaving the stream up")
		return
	}

	// Nothing is feeding the transcoder any more, so it would only linger.
	if c.deps.HLS != nil {
		c.deps.HLS.StopTranscoding(babyUID)
		log.Info().Str("baby_uid", babyUID).Msg("Stopped HLS transcoding")
	}
}

// streamingRetryMonitor continuously monitors and retries failed streaming connections
func (c *Camera) streamingRetryMonitor(ctx utils.GracefulContext) {
	babyUID := c.UID()
	retryInterval := 60 * time.Second // Retry every 60 seconds
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	log.Info().
		Str("baby_uid", babyUID).
		Dur("retry_interval", retryInterval).
		Msg("Starting streaming retry monitor")

	for {
		select {
		case <-ticker.C:
			// Check if we should retry streaming
			if c.shouldRetryStreaming() {
				if conn, ok := c.requester().(*client.WebsocketConnection); ok && conn != nil {
					log.Info().
						Str("baby_uid", babyUID).
						Msg("Retrying streaming connection due to previous failure")

					go c.retryStreaming(conn)
				}
			}

		case <-ctx.Done():
			log.Info().
				Str("baby_uid", babyUID).
				Msg("Streaming retry monitor stopped")
			return
		}
	}
}

// shouldRetryStreaming determines if we should retry streaming for a baby
func (c *Camera) shouldRetryStreaming() bool {
	babyUID := c.UID()
	// Only retry if RTMP auto-start is enabled
	if !c.autoStart() {
		return false
	}

	babyState := c.deps.State.GetBabyState(babyUID)

	// Only retry if:
	// 1. WebSocket is alive (connection exists)
	// 2. Stream request failed (connection limit or other failure)
	// 3. Stream is not currently alive (no active stream)
	return babyState.GetIsWebsocketAlive() &&
		babyState.GetStreamRequestState() == baby.StreamRequestState_RequestFailed &&
		babyState.GetStreamState() != baby.StreamState_Alive
}

// retryStreaming attempts to restart streaming after a failure
func (c *Camera) retryStreaming(conn *client.WebsocketConnection) {
	babyUID := c.UID()
	streamURL := c.LocalStreamURL()
	if streamURL == "" {
		log.Error().Str("baby_uid", babyUID).Msg("Cannot retry streaming: no RTMP URL available")
		return
	}

	log.Info().
		Str("baby_uid", babyUID).
		Str("rtmp_url", streamURL).
		Msg("Retrying RTMP streaming and HLS transcoding")

	// Reset the failed state before retrying
	c.deps.State.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_NotRequested))

	// Retry RTMP streaming
	requestLocalStreaming(babyUID, streamURL, client.Streaming_STARTED, conn, c.deps.State)

	// Start HLS transcoding if not already running
	if c.deps.HLS != nil {
		if transcoder, exists := c.deps.HLS.GetTranscoder(babyUID); !exists || !transcoder.IsRunning() {
			// Give RTMP stream a moment to establish before starting HLS transcoding
			go func() {
				time.Sleep(3 * time.Second)

				if err := c.deps.HLS.StartTranscoding(babyUID, streamURL); err != nil {
					log.Error().
						Err(err).
						Str("baby_uid", babyUID).
						Msg("Failed to start HLS transcoding during retry")
				} else {
					log.Info().
						Str("baby_uid", babyUID).
						Msg("Started HLS transcoding during streaming retry")
				}
			}()
		}
	}
}
