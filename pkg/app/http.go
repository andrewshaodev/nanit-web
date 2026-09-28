package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/httpapi"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// serveHTTP runs the dashboard and API until ctx ends, then gives requests
// in flight up to 5 s to finish. If the server can't start (the port is in
// use, say) the app stops: it used to carry on without a dashboard.
func (app *App) serveHTTP(ctx utils.GracefulContext) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", app.Opts.HTTPPort),
		Handler: app.apiServer().Handler(),
		// The server had no timeouts, so a slow or stalled client held its
		// connection open indefinitely
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Reading the camera's sound settings takes up to three 10 s requests
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info().Int("port", app.Opts.HTTPPort).Msg("Starting HTTP server")
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		log.Error().Err(err).Int("port", app.Opts.HTTPPort).Msg("HTTP server failed")
		app.fail(fmt.Errorf("HTTP server: %w", err))
		return
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Msg("HTTP server didn't shut down cleanly")
	}
	log.Info().Msg("HTTP server stopped")
}

// apiServer - the dashboard and API, over the app's parts
func (app *App) apiServer() *httpapi.Server {
	config := httpapi.Config{
		MQTTEnabled: app.MQTTConnection != nil,
		SessionFile: app.Opts.SessionFile,
		WebDir:      app.Opts.WebDir,
	}
	if app.Opts.RTMP != nil {
		config.RTMPPublicAddr = app.Opts.RTMP.PublicAddr
	}
	return &httpapi.Server{
		Config:    config,
		Sessions:  app.SessionStore,
		State:     app.BabyStateManager,
		Cameras:   app.Cameras,
		HLS:       app.HLSManager,
		History:   app.HistoryTracker,
		WebAuth:   app.WebAuth,
		Nanit:     app.RestClient,
		SignedIn:  app.StartMonitoringServices,
		StartedAt: app.startedAt,
	}
}
