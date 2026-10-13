// Package streamsource - where a viewer instance takes its video from: the
// stream server's RTSP output, set from the dashboard. Saving it reconnects
// straight away. Each camera's pull reports back here, so the dashboard can
// say whether video is arriving, and why not.
package streamsource

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// DefaultPort - the stream server's RTSP port unless it says otherwise
const DefaultPort = 8554

// Source - the stream server. Key is its stream password, when it has one.
type Source struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Key  string `json:"key,omitempty"`
}

// Configured - whether there is a server to pull from
func (s Source) Configured() bool {
	return s.Host != ""
}

// URL - the RTSP address of one camera's stream on the server
func (s Source) URL(babyUID string) string {
	u := url.URL{Scheme: "rtsp", Host: net.JoinHostPort(s.Host, strconv.Itoa(s.port()))}
	if s.Key != "" {
		u.User = url.UserPassword(streamauth.User, s.Key)
	}
	// Appended rather than set as the path, which would escape a
	// {baby_uid} placeholder; real UIDs are letters and digits
	return u.String() + "/local/" + babyUID
}

// Redacted - the address for logs and the dashboard, with the key masked
func (s Source) Redacted(babyUID string) string {
	if !s.Configured() {
		return ""
	}
	masked := s
	if masked.Key != "" {
		masked.Key = "xxxxx"
	}
	return masked.URL(babyUID)
}

func (s Source) port() int {
	if s.Port == 0 {
		return DefaultPort
	}
	return s.Port
}

// Validate checks the fields, and fills in the default port
func (s *Source) Validate() error {
	s.Host = strings.TrimSpace(s.Host)
	s.Key = strings.TrimSpace(s.Key)
	if s.Host == "" {
		// Nothing to pull from: valid, and means no video
		s.Port, s.Key = 0, ""
		return nil
	}
	if strings.ContainsAny(s.Host, "/:@ ?#") {
		return errors.New("host is just a name or IP address, without rtsp:// or a port")
	}
	if s.Port == 0 {
		s.Port = DefaultPort
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port %d is out of range", s.Port)
	}
	return nil
}

// Status - how one camera's pull is going
type Status struct {
	// Receiving - the server's video is arriving
	Receiving bool
	// Error - why not, the last time it tried; "" while receiving
	Error string
	Since time.Time
}

// Store - the source, kept in a file in the data folder, and each camera's
// status
type Store struct {
	path string

	mu      sync.Mutex
	source  Source
	changed chan struct{}
	status  map[string]Status
}

// Open loads the source from path. A missing file means none is set.
func Open(path string) *Store {
	s := &Store{path: path, changed: make(chan struct{}), status: map[string]Status{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Error().Err(err).Str("path", path).Msg("Failed to read the stream server settings")
		}
		return s
	}
	if err := json.Unmarshal(data, &s.source); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read the stream server settings")
		s.source = Source{}
	}
	return s
}

// Get - the current source
func (s *Store) Get() Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.source
}

// Changed - a channel closed the next time the source changes
func (s *Store) Changed() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changed
}

// Set validates and saves a new source. Pulls reconnect to it.
func (s *Store) Set(next Source) (Source, error) {
	if err := next.Validate(); err != nil {
		return Source{}, err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return Source{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := utils.WriteFileAtomic(s.path, data); err != nil {
		return s.source, err
	}
	s.source = next
	// Old statuses describe the old server
	s.status = map[string]Status{}
	close(s.changed)
	s.changed = make(chan struct{})
	return next, nil
}

// SetStatus - a camera's pull reports how it's going
func (s *Store) SetStatus(babyUID string, receiving bool, err error) {
	st := Status{Receiving: receiving, Since: time.Now()}
	if err != nil {
		st.Error = Describe(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.status[babyUID]; ok && prev.Receiving == st.Receiving && prev.Error == st.Error {
		return
	}
	s.status[babyUID] = st
}

// Status - a camera's status, if it has tried
func (s *Store) Status(babyUID string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status[babyUID]
	return st, ok
}

// Describe - a pull error in words the dashboard can show
func Describe(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"):
		return "The stream server asks for a key, and this one is missing or wrong"
	case strings.Contains(msg, "404"):
		return "The stream server isn't receiving this camera's video right now"
	case strings.Contains(msg, "connection refused"):
		return "Nothing is listening on that host and port"
	case strings.Contains(msg, "no such host"):
		return "That host name doesn't resolve"
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "deadline exceeded"):
		return "The stream server didn't answer in time"
	default:
		return msg
	}
}
