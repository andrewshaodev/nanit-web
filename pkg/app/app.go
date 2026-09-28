package app

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/history"
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
	Cameras          *camera.Registry
	mainContext      utils.GracefulContext // Store main application context

	// Set once RTMP and MQTT have been started. Signing in again used to
	// start them, and every camera, a second time.
	servicesStarted atomic.Bool

	// fail stops the app with an error (set by Run)
	fail func(error)

	// State changes waiting to be written to the history database
	historyQueue chan historyUpdate
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
		HLSManager: streaming.NewHLSManager(opts.DataDirectories.BaseDir + "/hls"),
		WebAuth:    webauth.NewWebAuth(opts.WebAuth.PasswordFile),
	}

	if opts.MQTT != nil {
		instance.MQTTConnection = mqtt.NewConnection(*opts.MQTT)
	}

	cameraOpts := camera.Options{EventPolling: camera.EventPollingOptions{
		Enabled:        opts.EventPolling.Enabled,
		Interval:       opts.EventPolling.PollingInterval,
		MessageTimeout: opts.EventPolling.MessageTimeout,
	}}
	if opts.RTMP != nil {
		cameraOpts.RTMP = &camera.RTMPOptions{PublicAddr: opts.RTMP.PublicAddr, AutoStart: opts.RTMP.AutoStart}
	}
	instance.Cameras = camera.NewRegistry(cameraOpts, camera.Deps{
		State: instance.BabyStateManager,
		Nanit: instance.RestClient,
		HLS:   instance.HLSManager,
		MQTT:  instance.MQTTConnection,
	})

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

// Run - application main loop. It returns once ctx is cancelled and
// everything it started has shut down.
func (app *App) Run(ctx utils.GracefulContext) {
	// Failing the root context stops the whole app, and main then exits
	app.fail = ctx.Fail

	// Everything but the final cleanup runs under services, so shutdown can
	// wait for all of it before closing the history database
	services := ctx.RunAsChild(func(servicesCtx utils.GracefulContext) {
		// Signing in from the dashboard starts the cameras under this too
		app.mainContext = servicesCtx

		app.setupHistoryTracking(servicesCtx)

		servicesCtx.RunAsChild(app.serveHTTP)

		if app.hasSavedSession() {
			app.startServices(servicesCtx, app.SessionStore.Babies())
			log.Info().Msg("All services started with authentication")
		} else {
			log.Info().Int("port", app.Opts.HTTPPort).Msg("Waiting for sign-in: open the dashboard to connect a Nanit account")
		}

		<-servicesCtx.Done()
	})

	<-ctx.Done()
	log.Info().Msg("Shutting down application...")

	// Stop ffmpeg first, so it doesn't retry as the RTMP server goes away
	if app.HLSManager != nil {
		app.HLSManager.StopAll()
	}
	// Wait for the cameras to be told to stop streaming, the servers to
	// close and queued history to be written
	services.Wait()
	// The history database used to be closed alongside all that, while the
	// cameras could still be writing to it
	if app.HistoryTracker != nil {
		if err := app.HistoryTracker.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close history tracker")
		}
	}
	log.Info().Msg("Application cleanup completed")
}

// hasSavedSession reports whether a saved Nanit session still works:
// signed in (renewing the token if needed) and the cameras known
func (app *App) hasSavedSession() bool {
	if app.SessionStore == nil || app.SessionStore.RefreshToken() == "" {
		log.Info().Msg("No saved Nanit session")
		return false
	}
	if err := app.RestClient.MaybeAuthorize(false); err != nil {
		log.Error().Err(err).Msg("Saved Nanit session no longer works")
		return false
	}
	if _, err := app.RestClient.EnsureBabies(); err != nil {
		log.Error().Err(err).Msg("Failed to fetch babies")
		return false
	}
	return true
}

func (app *App) getLocalStreamURL(babyUID string) string {
	if app.Opts.RTMP != nil {
		return camera.LocalStreamURL(app.Opts.RTMP.PublicAddr, babyUID)
	}

	return ""
}

// getLocalStreamURLTemplate - local stream URL with the baby UID left as a
// "{baby_uid}" placeholder, so clients can render it for any baby
func (app *App) getLocalStreamURLTemplate() string {
	return app.getLocalStreamURL("{baby_uid}")
}

// StartMonitoringServices - start all monitoring services after authentication
func (app *App) StartMonitoringServices() {
	// Use the main application context stored during Run()
	ctx := app.mainContext
	if ctx == nil {
		log.Error().Msg("Cannot start monitoring services: main context not available")
		return
	}
	log.Info().Msg("Starting monitoring services after authentication...")

	// Sign-in has just stored fresh tokens; renew only if they're somehow stale
	if err := app.RestClient.MaybeAuthorize(false); err != nil {
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

// startServices starts the RTMP server and MQTT, the first time, and a
// camera for each baby that doesn't have one running
func (app *App) startServices(ctx utils.GracefulContext, babies []baby.Baby) {
	if app.servicesStarted.CompareAndSwap(false, true) {
		app.startSharedServices(ctx)
	}
	app.Cameras.Sync(ctx, babies)
}

// startSharedServices - the RTMP server and MQTT, which all cameras share
func (app *App) startSharedServices(ctx utils.GracefulContext) {
	if app.Opts.RTMP != nil {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			// Logged rather than stopping the app: in a container that would
			// restart it, reconnecting to Nanit, in a loop
			if err := rtmpserver.Serve(childCtx, app.Opts.RTMP.ListenAddr, app.BabyStateManager); err != nil {
				log.Error().Err(err).Msg("RTMP server failed; video is unavailable")
			}
		})
	}

	if app.MQTTConnection != nil {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			app.MQTTConnection.Run(app.BabyStateManager, childCtx)
		})
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

// setupHistoryTracking configures historical data tracking for state changes
// historyUpdate - a state change to record
type historyUpdate struct {
	babyUID string
	state   baby.State
}

// historyQueueSize - how many state changes can wait to be written. Updates
// come a few a second at most, so a full queue means the database is stuck.
const historyQueueSize = 256

func (app *App) setupHistoryTracking(ctx utils.GracefulContext) {
	if !app.HistoryTracker.IsEnabled() {
		log.Debug().Msg("Historical tracking disabled")
		return
	}

	// Each state change used to be written from a goroutine of its own, so
	// writes piled up against SQLite's single writer. They're queued now,
	// for one writer.
	app.historyQueue = make(chan historyUpdate, historyQueueSize)
	app.BabyStateManager.SetHistoryCallback(func(babyUID string, state baby.State) {
		select {
		case app.historyQueue <- historyUpdate{babyUID, state}:
		default:
			log.Warn().Str("baby_uid", babyUID).Msg("History queue full, dropping an update")
		}
	})
	ctx.RunAsChild(app.writeHistory)

	log.Info().Msg("Historical data tracking enabled")

	// Set up periodic cleanup if enabled
	if app.Opts.History.CleanupEnabled {
		app.setupHistoryCleanup(ctx)
	}
}

// writeHistory records queued state changes until ctx ends, then writes
// what's still queued. Run closes the database only after this returns.
func (app *App) writeHistory(ctx utils.GracefulContext) {
	for {
		select {
		case update := <-app.historyQueue:
			app.recordHistory(update)
		case <-ctx.Done():
			for {
				select {
				case update := <-app.historyQueue:
					app.recordHistory(update)
				default:
					return
				}
			}
		}
	}
}

// recordHistory writes one state change to the history database
func (app *App) recordHistory(update historyUpdate) {
	babyUID, state := update.babyUID, update.state

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
}

// setupHistoryCleanup starts a background routine for cleaning up old historical data
func (app *App) setupHistoryCleanup(ctx utils.GracefulContext) {
	if !app.HistoryTracker.IsEnabled() {
		return
	}

	ctx.RunAsChild(func(childCtx utils.GracefulContext) {
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
