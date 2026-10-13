// Package streamauth - the optional password on the bridge's RTSP and RTMP
// outputs, set from the dashboard. RTSP players sign in as User with the
// key as the password; RTMP players add ?key= to the address. The camera's
// own RTMP push is never asked for it, so changing the key can't stop the
// stream.
package streamauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// User - the RTSP user name; only the key is secret
const User = "nanit"

// Settings - whether the password is on, and the key
type Settings struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key,omitempty"`
}

// Store - the settings, kept in a file in the data folder
type Store struct {
	path string

	mu       sync.Mutex
	settings Settings
	// changed is closed, and replaced, whenever the settings change
	changed chan struct{}
	// internal - a key for the bridge's own HLS converter, new each start
	// and never shown, so it can read the RTMP output with the password on
	internal string
}

// Open loads the settings from path. A missing or unreadable file means the
// password is off.
func Open(path string) *Store {
	s := &Store{path: path, changed: make(chan struct{}), internal: newKey()}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Error().Err(err).Str("path", path).Msg("Failed to read the stream password; streams are open")
		}
		return s
	}
	if err := json.Unmarshal(data, &s.settings); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read the stream password; streams are open")
		s.settings = Settings{}
	}
	// Enabled without a key would lock everyone out
	if s.settings.Key == "" {
		s.settings.Enabled = false
	}
	return s
}

// Get - the current settings
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// Changed - a channel closed the next time the settings change, so open
// streams can be closed and their players made to sign in again
func (s *Store) Changed() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changed
}

// SetEnabled turns the password on or off. Turning it on the first time
// makes a key.
func (s *Store) SetEnabled(enabled bool) (Settings, error) {
	return s.update(func(next *Settings) {
		next.Enabled = enabled
		if enabled && next.Key == "" {
			next.Key = newKey()
		}
	})
}

// Regenerate replaces the key; players with the old one are refused
func (s *Store) Regenerate() (Settings, error) {
	return s.update(func(next *Settings) {
		next.Key = newKey()
	})
}

func (s *Store) update(change func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.settings
	change(&next)
	data, err := json.Marshal(next)
	if err != nil {
		return s.settings, err
	}
	if err := utils.WriteFileAtomic(s.path, data); err != nil {
		return s.settings, err
	}
	s.settings = next
	close(s.changed)
	s.changed = make(chan struct{})
	return next, nil
}

// Required - the key players must give, and whether one is needed. A nil
// store never needs one.
func (s *Store) Required() (key string, required bool) {
	if s == nil {
		return "", false
	}
	settings := s.Get()
	return settings.Key, settings.Enabled
}

// Allows - whether key opens the streams: always, while the password is
// off, and the internal key always
func (s *Store) Allows(key string) bool {
	want, required := s.Required()
	if !required {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(want)) == 1 ||
		subtle.ConstantTimeCompare([]byte(key), []byte(s.internal)) == 1
}

// InternalKey - the key the bridge itself uses to read its RTMP output; ""
// for a nil store. It stays the same when the stream password changes.
func (s *Store) InternalKey() string {
	if s == nil {
		return ""
	}
	return s.internal
}

// newKey - 26 random letters and digits (128 bits), which need no escaping
// in a URL
func newKey() string {
	return rand.Text()
}
