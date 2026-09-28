package app

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/message"
	"github.com/andrewshaodev/nanit-web/pkg/mqtt"
	"github.com/andrewshaodev/nanit-web/pkg/rtmpserver"
	"github.com/andrewshaodev/nanit-web/pkg/session"
	"github.com/andrewshaodev/nanit-web/pkg/streaming"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/andrewshaodev/nanit-web/pkg/webauth"
	"github.com/rs/zerolog/log"
)

// App - application container
type App struct {
	Opts             Opts
	SessionStore     *session.Store
	BabyStateManager *baby.StateManager
	RestClient       *client.NanitClient
	MQTTConnection   *mqtt.Connection
	HLSManager       *streaming.HLSManager
	HistoryTracker   *history.Tracker
	WebAuth          *webauth.WebAuth
	connections      map[string]*client.WebsocketConnection
	connectionsMutex sync.RWMutex
	mainContext      utils.GracefulContext // Store main application context

	// Set once RTMP, MQTT and the cameras have been started. Signing in
	// again used to start them all a second time.
	servicesStarted atomic.Bool
}

// NewApp - constructor
func NewApp(opts Opts) (*App, error) {
	sessionStore, err := session.InitSessionStore(opts.SessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session store: %w", err)
	}

	instance := &App{
		Opts:             opts,
		BabyStateManager: baby.NewStateManager(),
		SessionStore:     sessionStore,
		RestClient: &client.NanitClient{
			Email:        opts.NanitCredentials.Email,
			Password:     opts.NanitCredentials.Password,
			RefreshToken: opts.NanitCredentials.RefreshToken,
			SessionStore: sessionStore,
		},
		HLSManager:  streaming.NewHLSManager(opts.DataDirectories.BaseDir + "/hls"),
		WebAuth:     webauth.NewWebAuth(opts.WebAuth.PasswordFile),
		connections: make(map[string]*client.WebsocketConnection),
	}

	if opts.MQTT != nil {
		instance.MQTTConnection = mqtt.NewConnection(*opts.MQTT)
	}

	// Initialize historical data tracker
	if historyTracker, err := history.NewTracker(opts.DataDirectories.HistoryDir, opts.History.Enabled); err != nil {
		log.Error().Err(err).Msg("Failed to initialize historical data tracker")
		// Continue without historical tracking
		instance.HistoryTracker = &history.Tracker{}
	} else {
		instance.HistoryTracker = historyTracker
	}

	return instance, nil
}

// Run - application main loop
func (app *App) Run(ctx utils.GracefulContext) {
	// Store main context for later use
	app.mainContext = ctx

	// Set up historical data tracking callback
	app.setupHistoryTracking()

	// Close the history database and stop ffmpeg on shutdown, however the
	// app started. This used to be set up only after a dashboard sign-in,
	// so a normal boot never cleaned up.
	ctx.RunAsChild(func(childCtx utils.GracefulContext) {
		<-childCtx.Done()
		log.Info().Msg("Shutting down application...")
		if app.HLSManager != nil {
			app.HLSManager.StopAll()
		}
		if app.HistoryTracker != nil {
			if err := app.HistoryTracker.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close history tracker")
			}
		}
		log.Info().Msg("Application cleanup completed")
	})
	// Check if we have valid authentication
	hasValidAuth := false
	if app.SessionStore != nil && app.SessionStore.RefreshToken() != "" {
		// Try to authorize - if it fails, we'll run in web-only mode
		defer func() {
			if r := recover(); r != nil {
				log.Warn().Interface("error", r).Msg("Authorization failed, running in web-only mode")
				hasValidAuth = false
			}
		}()

		if err := app.RestClient.MaybeAuthorize(false); err != nil {
			log.Error().Err(err).Msg("Authentication failed")
			hasValidAuth = false
		} else {
			if _, err := app.RestClient.EnsureBabies(); err != nil {
				log.Error().Err(err).Msg("Failed to fetch babies")
				hasValidAuth = false
			} else {
				hasValidAuth = true
			}
		}
	} else {
		log.Info().Msg("No valid authentication found - running in web-only mode for initial setup")
	}

	// Always start HTTP server for web UI (including setup)
	go ServeReact(app.BabyStateManager, app)

	// Only start RTMP/MQTT/WebSocket if we have valid auth
	if hasValidAuth {
		app.startServices(ctx, app.SessionStore.Babies())
		log.Info().Msg("All services started with authentication")
	} else {
		log.Info().Msg("Web server started - visit http://localhost:8080/setup to configure authentication")
	}

	<-ctx.Done()
}

func (app *App) handleBaby(baby baby.Baby, ctx utils.GracefulContext) {
	if app.Opts.RTMP != nil || app.MQTTConnection != nil {
		// Websocket connection
		ws := client.NewWebsocketConnectionManager(baby.UID, baby.CameraUID, app.RestClient, app.BabyStateManager)

		ws.WithReadyConnection(func(conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
			// Register connection
			app.registerConnection(baby.UID, conn)
			defer func() {
				app.unregisterConnection(baby.UID)
				// Gracefully stop streaming when WebSocket disconnects
				if app.Opts.RTMP != nil && app.Opts.RTMP.AutoStart {
					app.autoStopStreaming(baby.UID)
				}
			}()

			// Auto-start streaming if RTMP is enabled and auto-start is configured
			if app.Opts.RTMP != nil && app.Opts.RTMP.AutoStart {
				log.Info().Str("baby_uid", baby.UID).Msg("Auto-starting RTMP stream")
				go app.autoStartStreaming(baby.UID, conn)

				// Start persistent retry mechanism for failed connections
				go app.startStreamingRetryMonitor(baby.UID, childCtx)
			}

			app.runWebsocket(baby.UID, conn, childCtx)
		})

		if app.Opts.EventPolling.Enabled {
			ctx.RunAsChild(func(childCtx utils.GracefulContext) {
				app.pollMessages(baby.UID, app.BabyStateManager, childCtx)
			})
		}

		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			ws.RunWithinContext(childCtx)
		})
	}

	<-ctx.Done()
}

// pollMessages checks Nanit for new motion and sound events until ctx ends.
// It used to call itself after each wait, forever, ignoring shutdown.
func (app *App) pollMessages(babyUID string, babyStateManager *baby.StateManager, ctx utils.GracefulContext) {
	for {
		app.pollMessagesOnce(babyUID, babyStateManager)

		select {
		case <-ctx.Done():
			return
		case <-time.After(app.Opts.EventPolling.PollingInterval):
		}
	}
}

func (app *App) pollMessagesOnce(babyUID string, babyStateManager *baby.StateManager) {
	newMessages, err := app.RestClient.FetchNewMessages(babyUID, app.Opts.EventPolling.MessageTimeout)
	if err != nil {
		log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to fetch new messages")
		// Continue with empty messages rather than crash
		newMessages = []message.Message{}
	}

	for _, msg := range newMessages {
		switch msg.Type {
		case message.SoundEventMessageType:
			go babyStateManager.NotifySoundSubscribers(babyUID, time.Time(msg.Time))
			break
		case message.MotionEventMessageType:
			go babyStateManager.NotifyMotionSubscribers(babyUID, time.Time(msg.Time))
			break
		}
	}
}

func (app *App) runWebsocket(babyUID string, conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
	// Reading sensor data
	conn.RegisterMessageHandler(func(m *client.Message, conn *client.WebsocketConnection) {
		// Sensor request initiated by us on start (or some other client, we don't care)
		if *m.Type == client.Message_RESPONSE && m.Response != nil {
			if *m.Response.RequestType == client.RequestType_GET_SENSOR_DATA && len(m.Response.SensorData) > 0 {
				processSensorData(babyUID, m.Response.SensorData, app.BabyStateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_CONTROL && m.Response.Control != nil {
				processLight(babyUID, m.Response.Control, app.BabyStateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_SETTINGS && m.Response.Settings != nil {
				processStandby(babyUID, m.Response.Settings, app.BabyStateManager)
			} else if *m.Response.RequestType == client.RequestType_GET_STATUS && m.Response.Status != nil {
				processStatus(babyUID, m.Response.Status, app.BabyStateManager)
			}
		} else

		// Communication initiated from a cam
		// Note: it sends the updates periodically on its own + whenever some significant change occurs
		if *m.Type == client.Message_REQUEST && m.Request != nil {
			if *m.Request.Type == client.RequestType_PUT_SENSOR_DATA && len(m.Request.SensorData_) > 0 {
				processSensorData(babyUID, m.Request.SensorData_, app.BabyStateManager)
			} else if *m.Request.Type == client.RequestType_PUT_CONTROL && m.Request.Control != nil {
				processLight(babyUID, m.Request.Control, app.BabyStateManager)
			} else if *m.Request.Type == client.RequestType_PUT_SETTINGS && m.Request.Settings != nil {
				processStandby(babyUID, m.Request.Settings, app.BabyStateManager)
			}
		}
	})

	if app.MQTTConnection != nil {
		unregister := app.MQTTConnection.RegisterBaby(babyUID, mqtt.CommandHandlers{
			NightLight: func(enabled bool) { sendLightCommand(enabled, conn) },
			Standby:    func(enabled bool) { sendStandbyCommand(enabled, conn) },
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

	// Ask for logs
	// conn.SendRequest(client.RequestType_GET_LOGS, &client.Request{
	// 	GetLogs: &client.GetLogs{
	// 		Url: utils.ConstRefStr("http://192.168.3.234:8080/log"),
	// 	},
	// })

	var cleanup func()

	// Local streaming
	if app.Opts.RTMP != nil {
		initializeLocalStreaming := func() {
			requestLocalStreaming(babyUID, app.getLocalStreamURL(babyUID), client.Streaming_STARTED, conn, app.BabyStateManager)
		}

		// Asking the cam to stream on its own is what NANIT_RTMP_AUTO_START
		// controls. These two requests used to ignore it, so setting it to false
		// still pointed the cam's stream at this bridge on every connect.
		autoStart := app.Opts.RTMP.AutoStart

		// Watch for stream liveness change
		unsubscribe := app.BabyStateManager.Subscribe(func(updatedBabyUID string, stateUpdate baby.State) {
			// Do another streaming request if stream just turned unhealthy
			if autoStart && updatedBabyUID == babyUID && stateUpdate.StreamState != nil && *stateUpdate.StreamState == baby.StreamState_Unhealthy {
				// Prevent duplicate request if we already received failure
				if app.BabyStateManager.GetBabyState(babyUID).GetStreamRequestState() != baby.StreamRequestState_RequestFailed {
					go initializeLocalStreaming()
				}
			}
		})

		cleanup = func() {
			// Stop listening for stream liveness change
			unsubscribe()

			// Stop local streaming
			state := app.BabyStateManager.GetBabyState(babyUID)
			if state.GetIsWebsocketAlive() && state.GetStreamState() == baby.StreamState_Alive {
				requestLocalStreaming(babyUID, app.getLocalStreamURL(babyUID), client.Streaming_STOPPED, conn, app.BabyStateManager)
			}
		}

		// Initialize local streaming upon connection if we know that the stream is not alive
		babyState := app.BabyStateManager.GetBabyState(babyUID)
		if autoStart && babyState.GetStreamState() != baby.StreamState_Alive {
			if babyState.GetStreamRequestState() != baby.StreamRequestState_Requested || babyState.GetStreamState() == baby.StreamState_Unhealthy {
				go initializeLocalStreaming()
			}
		}
	}

	<-childCtx.Done()
	if cleanup != nil {
		cleanup()
	}
}

// localStreamURLTemplate - shape of the RTMP URL served by the built-in RTMP
// server, both for the cam to publish to and for clients (VLC, Home Assistant,
// ffmpeg) to subscribe to.
const localStreamURLTemplate = "rtmp://{publicAddr}/local/{babyUid}"

func (app *App) getLocalStreamURL(babyUID string) string {
	if app.Opts.RTMP != nil {
		return strings.NewReplacer("{publicAddr}", app.Opts.RTMP.PublicAddr, "{babyUid}", babyUID).Replace(localStreamURLTemplate)
	}

	return ""
}

// getLocalStreamURLTemplate - local stream URL with the baby UID left as a
// "{baby_uid}" placeholder, so clients can render it for any baby
func (app *App) getLocalStreamURLTemplate() string {
	return app.getLocalStreamURL("{baby_uid}")
}

// Connection management methods for WebSocket connections
func (app *App) registerConnection(babyUID string, conn *client.WebsocketConnection) {
	app.connectionsMutex.Lock()
	defer app.connectionsMutex.Unlock()
	app.connections[babyUID] = conn
}

func (app *App) unregisterConnection(babyUID string) {
	app.connectionsMutex.Lock()
	defer app.connectionsMutex.Unlock()
	delete(app.connections, babyUID)
}

func (app *App) getConnection(babyUID string) *client.WebsocketConnection {
	app.connectionsMutex.RLock()
	defer app.connectionsMutex.RUnlock()
	return app.connections[babyUID]
}

// StartMonitoringServices - start all monitoring services after authentication
func (app *App) StartMonitoringServices() {
	// Use the main application context stored during Run()
	ctx := app.mainContext
	if ctx == nil {
		log.Error().Msg("Cannot start monitoring services: main context not available")
		return
	}
	if app.servicesStarted.Load() {
		log.Info().Msg("Signed in again; services are already running")
		return
	}
	log.Info().Msg("Starting monitoring services after authentication...")

	// Force refresh authorization and fetch babies (token may have expired since web auth)
	if err := app.RestClient.MaybeAuthorize(true); err != nil { // Force refresh
		log.Error().Err(err).Msg("Failed to refresh authorization")
		return
	}
	if _, err := app.RestClient.EnsureBabies(); err != nil {
		log.Error().Err(err).Msg("Failed to ensure babies after authorization")
		return
	}

	babies := app.SessionStore.Babies()
	if len(babies) == 0 {
		log.Warn().Msg("No babies found after authentication")
		return
	}

	log.Info().Int("babies_count", len(babies)).Msg("Found babies, starting services")
	app.startServices(ctx, babies)
	log.Info().Msg("All monitoring services started successfully")
}

// startServices starts the RTMP server, MQTT and a connection per camera,
// at most once
func (app *App) startServices(ctx utils.GracefulContext, babies []baby.Baby) {
	if !app.servicesStarted.CompareAndSwap(false, true) {
		return
	}

	if app.Opts.RTMP != nil {
		go func() {
			if err := rtmpserver.StartRTMPServer(app.Opts.RTMP.ListenAddr, app.BabyStateManager); err != nil {
				log.Error().Err(err).Msg("RTMP server failed to start or crashed")
			}
		}()
	}

	if app.MQTTConnection != nil {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			app.MQTTConnection.Run(app.BabyStateManager, childCtx)
		})
	}

	for _, babyInfo := range babies {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			app.handleBaby(babyInfo, childCtx)
		})
		log.Info().Str("baby_uid", babyInfo.UID).Str("name", babyInfo.Name).Msg("Started monitoring baby")
	}
}

// babies is the account's cameras, as last fetched from Nanit. Handlers
// read it per request: a list captured at startup stayed empty until a
// restart when the bridge started before sign-in.
func (app *App) babies() []baby.Baby {
	if app.SessionStore == nil {
		return nil
	}
	return app.SessionStore.Babies()
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
func (app *App) autoStartStreaming(babyUID string, conn *client.WebsocketConnection) {
	// Give the WebSocket connection a moment to fully establish
	time.Sleep(2 * time.Second)

	// Get the RTMP URL for this baby
	streamURL := app.getLocalStreamURL(babyUID)
	if streamURL == "" {
		log.Error().Str("baby_uid", babyUID).Msg("Cannot auto-start streaming: no RTMP URL available")
		return
	}

	// Asking for a stream the cam is already publishing makes it open a second
	// publisher connection, and registering that one closes every subscriber of
	// the first. After a reconnect the existing stream is usually still running,
	// so the request is only worth making when it is not.
	if !shouldRequestStream(app.BabyStateManager.GetBabyState(babyUID)) {
		log.Info().
			Str("baby_uid", babyUID).
			Msg("Cam is already publishing, leaving the existing stream alone")
	} else {
		log.Info().
			Str("baby_uid", babyUID).
			Str("rtmp_url", streamURL).
			Msg("Auto-starting RTMP streaming")

		requestLocalStreaming(babyUID, streamURL, client.Streaming_STARTED, conn, app.BabyStateManager)
	}

	// Start HLS transcoding for instant playback
	if app.HLSManager != nil {
		// Give RTMP stream a moment to establish before starting HLS transcoding
		go func() {
			time.Sleep(3 * time.Second)

			// A transcoder left running against a still-live stream is already
			// producing what we would be restarting it for.
			if transcoder, exists := app.HLSManager.GetTranscoder(babyUID); exists && transcoder.IsRunning() {
				log.Debug().Str("baby_uid", babyUID).Msg("HLS transcoding is already running")
				return
			}

			if err := app.HLSManager.StartTranscoding(babyUID, streamURL); err != nil {
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
func (app *App) autoStopStreaming(babyUID string) {
	if !shouldReleaseStreamResources(app.BabyStateManager.GetBabyState(babyUID)) {
		log.Info().
			Str("baby_uid", babyUID).
			Msg("Websocket ended while the cam is still publishing, leaving the stream up")
		return
	}

	// Nothing is feeding the transcoder any more, so it would only linger.
	if app.HLSManager != nil {
		app.HLSManager.StopTranscoding(babyUID)
		log.Info().Str("baby_uid", babyUID).Msg("Stopped HLS transcoding")
	}
}

// setupHistoryTracking configures historical data tracking for state changes
func (app *App) setupHistoryTracking() {
	if !app.HistoryTracker.IsEnabled() {
		log.Debug().Msg("Historical tracking disabled")
		return
	}

	// Set up callback to track state changes
	app.BabyStateManager.SetHistoryCallback(func(babyUID string, state baby.State) {
		// Track sensor data (temperature, humidity, night mode)
		if state.TemperatureMilli != nil || state.HumidityMilli != nil || state.IsNight != nil {
			if err := app.HistoryTracker.TrackSensorData(babyUID, state); err != nil {
				log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to track sensor data")
			}
		}

		// Track motion events
		if state.MotionTimestamp != nil {
			if err := app.HistoryTracker.TrackEvent(babyUID, "motion", int64(*state.MotionTimestamp)); err != nil {
				log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to track motion event")
			}
		}

		// Track sound events
		if state.SoundTimestamp != nil {
			if err := app.HistoryTracker.TrackEvent(babyUID, "sound", int64(*state.SoundTimestamp)); err != nil {
				log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to track sound event")
			}
		}

		// Track night light state changes
		if state.NightLight != nil {
			if err := app.HistoryTracker.TrackStateChange(babyUID, "night_light", *state.NightLight); err != nil {
				log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to track night light state change")
			}
		}

		// Track standby state changes
		if state.Standby != nil {
			if err := app.HistoryTracker.TrackStateChange(babyUID, "standby", *state.Standby); err != nil {
				log.Error().Err(err).Str("baby_uid", babyUID).Msg("Failed to track standby state change")
			}
		}
	})

	log.Info().Msg("Historical data tracking enabled")

	// Set up periodic cleanup if enabled
	if app.Opts.History.CleanupEnabled {
		app.setupHistoryCleanup()
	}
}

// setupHistoryCleanup starts a background routine for cleaning up old historical data
func (app *App) setupHistoryCleanup() {
	if !app.HistoryTracker.IsEnabled() {
		return
	}

	app.mainContext.RunAsChild(func(childCtx utils.GracefulContext) {
		ticker := time.NewTicker(24 * time.Hour) // Run cleanup daily
		defer ticker.Stop()

		log.Info().Int("retention_days", app.Opts.History.RetentionDays).
			Msg("Starting historical data cleanup routine")

		for {
			select {
			case <-ticker.C:
				if err := app.HistoryTracker.Cleanup(app.Opts.History.RetentionDays); err != nil {
					log.Error().Err(err).Msg("Failed to cleanup historical data")
				}

			case <-childCtx.Done():
				log.Info().Msg("Historical data cleanup routine stopped")
				return
			}
		}
	})
}

// startStreamingRetryMonitor continuously monitors and retries failed streaming connections
func (app *App) startStreamingRetryMonitor(babyUID string, ctx utils.GracefulContext) {
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
			if app.shouldRetryStreaming(babyUID) {
				conn := app.getConnection(babyUID)
				if conn != nil {
					log.Info().
						Str("baby_uid", babyUID).
						Msg("Retrying streaming connection due to previous failure")

					go app.retryStreaming(babyUID, conn)
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
func (app *App) shouldRetryStreaming(babyUID string) bool {
	// Only retry if RTMP auto-start is enabled
	if app.Opts.RTMP == nil || !app.Opts.RTMP.AutoStart {
		return false
	}

	babyState := app.BabyStateManager.GetBabyState(babyUID)

	// Only retry if:
	// 1. WebSocket is alive (connection exists)
	// 2. Stream request failed (connection limit or other failure)
	// 3. Stream is not currently alive (no active stream)
	return babyState.GetIsWebsocketAlive() &&
		babyState.GetStreamRequestState() == baby.StreamRequestState_RequestFailed &&
		babyState.GetStreamState() != baby.StreamState_Alive
}

// retryStreaming attempts to restart streaming after a failure
func (app *App) retryStreaming(babyUID string, conn *client.WebsocketConnection) {
	streamURL := app.getLocalStreamURL(babyUID)
	if streamURL == "" {
		log.Error().Str("baby_uid", babyUID).Msg("Cannot retry streaming: no RTMP URL available")
		return
	}

	log.Info().
		Str("baby_uid", babyUID).
		Str("rtmp_url", streamURL).
		Msg("Retrying RTMP streaming and HLS transcoding")

	// Reset the failed state before retrying
	app.BabyStateManager.Update(babyUID, *baby.NewState().SetStreamRequestState(baby.StreamRequestState_NotRequested))

	// Retry RTMP streaming
	requestLocalStreaming(babyUID, streamURL, client.Streaming_STARTED, conn, app.BabyStateManager)

	// Start HLS transcoding if not already running
	if app.HLSManager != nil {
		if transcoder, exists := app.HLSManager.GetTranscoder(babyUID); !exists || !transcoder.IsRunning() {
			// Give RTMP stream a moment to establish before starting HLS transcoding
			go func() {
				time.Sleep(3 * time.Second)

				if err := app.HLSManager.StartTranscoding(babyUID, streamURL); err != nil {
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
