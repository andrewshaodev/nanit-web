package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/app"
	"github.com/andrewshaodev/nanit-web/pkg/mqtt"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/andrewshaodev/nanit-web/pkg/webauth"
	"github.com/rs/zerolog/log"
)

func main() {
	// Parse command line arguments
	var resetPassword = flag.Bool("reset-password", false, "Reset web password protection (removes password file)")
	flag.Parse()

	initLogger()
	logAppVersion()
	utils.LoadDotEnvFile()
	setLogLevel()

	// Handle CLI commands
	if *resetPassword {
		handleResetPassword()
		return
	}

	dirs, err := ensureDataDirectories()
	if err != nil {
		log.Error().Err(err).Msg("Failed to ensure data directories")
		os.Exit(1)
	}

	opts := app.Opts{
		NanitCredentials: app.NanitCredentials{
			Email:        utils.EnvVarStr("NANIT_EMAIL", ""),
			Password:     utils.EnvVarStr("NANIT_PASSWORD", ""),
			RefreshToken: utils.EnvVarStr("NANIT_REFRESH_TOKEN", ""),
		},
		SessionFile:     utils.EnvVarStr("NANIT_SESSION_FILE", filepath.Join(dirs.BaseDir, "session.json")),
		DataDirectories: dirs,
		HTTPPort:        utils.EnvVarInt("NANIT_HTTP_PORT", 8080),
		EventPolling: app.EventPollingOpts{
			// Event message polling disabled by default
			Enabled: utils.EnvVarBool("NANIT_EVENTS_POLLING", false),
			// 30 second default polling interval
			PollingInterval: utils.EnvVarSeconds("NANIT_EVENTS_POLLING_INTERVAL", 30*time.Second),
			// 300 second (5 min) default message timeout (unseen messages are ignored once they are this old)
			MessageTimeout: utils.EnvVarSeconds("NANIT_EVENTS_MESSAGE_TIMEOUT", 300*time.Second),
		},
		History: app.HistoryOpts{
			// Historical tracking enabled by default
			Enabled: utils.EnvVarBool("NANIT_HISTORY_ENABLED", true),
			// Keep data for 30 days by default
			RetentionDays: utils.EnvVarInt("NANIT_HISTORY_RETENTION_DAYS", 30),
			// Auto-cleanup enabled by default
			CleanupEnabled: utils.EnvVarBool("NANIT_HISTORY_CLEANUP_ENABLED", true),
		},
		WebAuth: app.WebAuthOpts{
			PasswordFile: passwordFile(dirs.BaseDir),
		},
	}

	if utils.EnvVarBool("NANIT_RTMP_ENABLED", true) {
		publicAddr := utils.EnvVarReqStr("NANIT_RTMP_ADDR")
		m := regexp.MustCompile("(:[0-9]+)$").FindStringSubmatch(publicAddr)
		if len(m) != 2 {
			log.Error().
				Str("value", publicAddr).
				Msg("Invalid NANIT_RTMP_ADDR format. Expected format: 'hostname:port' (e.g., '192.168.1.100:1935')")
			os.Exit(1)
		}

		opts.RTMP = &app.RTMPOpts{
			ListenAddr: m[1],
			PublicAddr: publicAddr,
			AutoStart:  utils.EnvVarBool("NANIT_RTMP_AUTO_START", true),
		}
	}

	if utils.EnvVarBool("NANIT_MQTT_ENABLED", false) {
		opts.MQTT = &mqtt.Opts{
			BrokerURL:   utils.EnvVarReqStr("NANIT_MQTT_BROKER_URL"),
			ClientID:    utils.EnvVarStr("NANIT_MQTT_CLIENT_ID", "nanit"),
			Username:    utils.EnvVarStr("NANIT_MQTT_USERNAME", ""),
			Password:    utils.EnvVarStr("NANIT_MQTT_PASSWORD", ""),
			TopicPrefix: utils.EnvVarStr("NANIT_MQTT_PREFIX", "nanit"),
		}
	}

	if opts.EventPolling.Enabled {
		log.Info().Msgf("Event polling enabled with an interval of %v", opts.EventPolling.PollingInterval)
	}

	// docker stop sends SIGTERM, then kills the process 10 s later. Only
	// Ctrl-C was handled, so in a container the camera, ffmpeg and SQLite
	// were never shut down cleanly.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	instance, err := app.NewApp(opts)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize application")
		os.Exit(1)
	}

	runner := utils.RunWithGracefulCancel(instance.Run)

	<-interrupt
	log.Warn().Msg("Received a stop signal, shutting down")

	waitForCleanup := make(chan struct{}, 1)

	go func() {
		runner.Cancel()
		close(waitForCleanup)
	}()

	select {
	case <-interrupt:
		log.Fatal().Msg("Received another interrupt signal, forcing termination without clean up")
	case <-waitForCleanup:
		log.Info().Msg("Clean exit")
		return
	}
}

// passwordFile is where the dashboard password is kept. It was always
// /data/web_password.json, whatever NANIT_DATA_DIR said.
func passwordFile(dataDir string) string {
	return filepath.Join(dataDir, "web_password.json")
}

// handleResetPassword removes the web password file (CLI command)
func handleResetPassword() {
	webAuth := webauth.NewWebAuth(passwordFile(utils.EnvVarStr("NANIT_DATA_DIR", "/data")))

	if !webAuth.IsPasswordSet() {
		fmt.Println("No password is currently set.")
		return
	}

	err := webAuth.RemovePassword()
	if err != nil {
		fmt.Printf("Error removing password: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Web password protection has been disabled successfully.")
	fmt.Println("You can now access the web interface without a password.")
}
