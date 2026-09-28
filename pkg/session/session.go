package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/andrewshaodev/nanit-web/pkg/baby"
)

// Revision - marks the version of the structure of a session file. Only files with equal revision will be loaded
// Note: you should increment this whenever you change the Session structure
const Revision = 3

// Session - application session data container
type Session struct {
	Revision            int         `json:"revision"`
	AuthToken           string      `json:"authToken"`
	AuthTime            time.Time   `json:"authTime"`
	Babies              []baby.Baby `json:"babies"`
	RefreshToken        string      `json:"refreshToken"`
	LastSeenMessageTime time.Time   `json:"lastSeenMessageTime"`
}

// Store - application session store context.
//
// The session is read by the websocket dialer and written by the REST client's
// authorization on different goroutines, so the session is kept private and
// reached through accessors: direct field access raced on the auth token.
type Store struct {
	Filename string

	mu      sync.RWMutex
	session *Session
}

// NewSessionStore - constructor
func NewSessionStore() *Store {
	return &Store{
		session: &Session{Revision: Revision},
	}
}

// AuthToken - currently held access token, empty if we never authorized
func (store *Store) AuthToken() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.session.AuthToken
}

// AuthTime - when the currently held access token was issued
func (store *Store) AuthTime() time.Time {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.session.AuthTime
}

// Credentials - access token together with its issue time.
//
// Read as a pair so a caller reasoning about the token's remaining life cannot
// observe a token from before a refresh alongside the issue time from after it.
func (store *Store) Credentials() (string, time.Time) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.session.AuthToken, store.session.AuthTime
}

// RefreshToken - token used to renew the session without a full re-login
func (store *Store) RefreshToken() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.session.RefreshToken
}

// SetRefreshToken - seeds the refresh token from configuration
func (store *Store) SetRefreshToken(refreshToken string) {
	store.mu.Lock()
	store.session.RefreshToken = refreshToken
	store.mu.Unlock()
}

// StoreCredentials - records the tokens returned by a login or a session renewal
func (store *Store) StoreCredentials(authToken string, refreshToken string, authTime time.Time) {
	store.mu.Lock()
	store.session.AuthToken = authToken
	store.session.RefreshToken = refreshToken
	store.session.AuthTime = authTime
	store.mu.Unlock()
}

// Babies - copy of the known babies
func (store *Store) Babies() []baby.Baby {
	store.mu.RLock()
	defer store.mu.RUnlock()

	babies := make([]baby.Baby, len(store.session.Babies))
	copy(babies, store.session.Babies)
	return babies
}

// SetBabies - records the baby list returned by the API
func (store *Store) SetBabies(babies []baby.Baby) {
	store.mu.Lock()
	store.session.Babies = babies
	store.mu.Unlock()
}

// LastSeenMessageTime - timestamp of the newest message we already processed
func (store *Store) LastSeenMessageTime() time.Time {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.session.LastSeenMessageTime
}

// SetLastSeenMessageTime - records how far through the message feed we got
func (store *Store) SetLastSeenMessageTime(t time.Time) {
	store.mu.Lock()
	store.session.LastSeenMessageTime = t
	store.mu.Unlock()
}

// Reset - drops all session data, as used when logging out
func (store *Store) Reset() {
	store.mu.Lock()
	store.session = &Session{Revision: Revision}
	store.mu.Unlock()
}

// Load - loads previous state from a file
func (store *Store) Load() error {
	if _, err := os.Stat(store.Filename); os.IsNotExist(err) {
		log.Info().Str("filename", store.Filename).Msg("No app session file found")
		return nil
	}

	f, err := os.Open(store.Filename)
	if err != nil {
		log.Error().Str("filename", store.Filename).Err(err).Msg("Unable to open app session file")
		return err
	}

	defer f.Close()

	session := &Session{}
	jsonErr := json.NewDecoder(f).Decode(session)
	if jsonErr != nil {
		log.Error().Str("filename", store.Filename).Err(jsonErr).Msg("Unable to decode app session file, using default session")
		// Don't return error for corrupted session files, just use default
		return nil
	}

	if session.Revision == Revision {
		store.mu.Lock()
		store.session = session
		store.mu.Unlock()
		log.Info().Str("filename", store.Filename).Msg("Loaded app session from the file")
	} else {
		log.Warn().Str("filename", store.Filename).Msg("App session file contains older revision of the state, ignoring")
	}

	return nil
}

// Save - stores current data in a file
func (store *Store) Save() error {
	if store.Filename == "" {
		return nil
	}

	log.Trace().Str("filename", store.Filename).Msg("Storing app session to the file")

	store.mu.RLock()
	data, jsonErr := json.Marshal(store.session)
	store.mu.RUnlock()

	if jsonErr != nil {
		log.Error().Str("filename", store.Filename).Err(jsonErr).Msg("Unable to marshal contents of app session file")
		return jsonErr
	}

	f, err := os.OpenFile(store.Filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Error().Str("filename", store.Filename).Err(err).Msg("Unable to open app session file for writing")
		return err
	}

	defer f.Close()

	_, writeErr := f.Write(data)
	if writeErr != nil {
		log.Error().Str("filename", store.Filename).Err(writeErr).Msg("Unable to write to app session file")
		return writeErr
	}

	return nil
}

// InitSessionStore - Initializes new application session store
func InitSessionStore(sessionFile string) (*Store, error) {
	sessionStore := NewSessionStore()

	// Load previous state of the application from session file
	if sessionFile != "" {

		absFileName, filePathErr := filepath.Abs(sessionFile)
		if filePathErr != nil {
			log.Error().Str("path", sessionFile).Err(filePathErr).Msg("Unable to retrieve absolute file path")
			return nil, filePathErr
		}

		sessionStore.Filename = absFileName
		if err := sessionStore.Load(); err != nil {
			log.Warn().Err(err).Msg("Failed to load session file, continuing with default session")
			// Don't return error - continue with default session
		}
	}

	return sessionStore, nil
}
