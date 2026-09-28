package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrewshaodev/nanit-web/pkg/app"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

func ensureDataDirectories() (app.DataDirectories, error) {
	relDataDir := utils.EnvVarStr("NANIT_DATA_DIR", "/data")

	absDataDir, filePathErr := filepath.Abs(relDataDir)
	if filePathErr != nil {
		log.Error().Str("path", relDataDir).Err(filePathErr).Msg("Unable to retrieve absolute file path")
		return app.DataDirectories{}, fmt.Errorf("failed to get absolute path for data directory '%s': %w", relDataDir, filePathErr)
	}

	// Create base data directory if it does not exist
	if _, err := os.Stat(absDataDir); os.IsNotExist(err) {
		log.Warn().Str("dir", absDataDir).Msg("Data directory does not exist, creating")
		mkdirErr := os.MkdirAll(absDataDir, 0755)
		if mkdirErr != nil {
			log.Error().Str("path", absDataDir).Err(mkdirErr).Msg("Unable to create data directory")
			return app.DataDirectories{}, fmt.Errorf("failed to create data directory '%s': %w", absDataDir, mkdirErr)
		}
	}

	// Create data dir skeleton
	for _, subdirName := range []string{"history"} {
		absSubdir := filepath.Join(absDataDir, subdirName)

		if _, err := os.Stat(absSubdir); os.IsNotExist(err) {
			mkdirErr := os.Mkdir(absSubdir, 0755)
			if mkdirErr != nil {
				log.Error().Str("path", absSubdir).Err(mkdirErr).Msg("Unable to create subdirectory")
				return app.DataDirectories{}, fmt.Errorf("failed to create subdirectory '%s': %w", absSubdir, mkdirErr)
			} else {
				log.Info().Str("dir", absSubdir).Msgf("Directory created ./%v", subdirName)
			}
		}
	}

	return app.DataDirectories{
		BaseDir:    absDataDir,
		HistoryDir: filepath.Join(absDataDir, "history"),
	}, nil
}
