// Package httpapi - the dashboard's page and the HTTP API behind it.
//
// Routes are declared with their methods in Handler, and everything under
// /api/ except the dashboard-password endpoints sits behind the password.
// Responses are the types in apitypes; errors are always
// apitypes.ErrorResponse.
package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/session"
	"github.com/andrewshaodev/nanit-web/pkg/streaming"
	"github.com/andrewshaodev/nanit-web/pkg/webauth"
	"github.com/rs/zerolog/log"
)

// Config - how the bridge is set up, as far as the API tells
type Config struct {
	// RTMPPublicAddr - where cameras and players reach the RTMP server; ""
	// when it's off
	RTMPPublicAddr string
	MQTTEnabled    bool
	// SessionFile - the saved Nanit session
	SessionFile string
	// WebDir - the built dashboard (index.html and assets/)
	WebDir string
}

// Server - the dashboard and API, and the parts of the bridge they use
type Server struct {
	Config   Config
	Sessions *session.Store
	State    *baby.StateManager
	Cameras  *camera.Registry
	HLS      *streaming.HLSManager
	History  *history.Tracker
	WebAuth  *webauth.WebAuth
	Nanit    *client.NanitClient
	// SignedIn - called after a Nanit sign-in, to start the cameras
	SignedIn func()
	// StartedAt - for /health's uptime
	StartedAt time.Time
}

// sessionCookie - the dashboard session, set by /api/webauth/login
const sessionCookie = "nanit_session"

// Handler - every route, with its method
func (s *Server) Handler() http.Handler {
	// Behind the dashboard password
	api := http.NewServeMux()
	api.HandleFunc("GET /api/status", s.handleStatus)
	api.HandleFunc("GET /api/babies", s.handleBabies)
	api.HandleFunc("GET /api/streaming/info", s.handleStreamingInfo)
	api.HandleFunc("GET /api/device-info/{uid}", s.withBaby(s.handleDeviceInfo))
	api.HandleFunc("GET /api/health/{uid}", s.withBaby(s.handleHealth))

	api.HandleFunc("POST /api/control/night-light", s.handleControl("night-light"))
	api.HandleFunc("POST /api/control/standby", s.handleControl("standby"))
	api.HandleFunc("GET /api/sound/{uid}", s.withBaby(s.handleSoundStatus))
	api.HandleFunc("POST /api/sound/{uid}/play", s.withBaby(s.handleSoundPlay))
	api.HandleFunc("POST /api/sound/{uid}/stop", s.withBaby(s.handleSoundStop))
	api.HandleFunc("POST /api/sound/{uid}/volume", s.withBaby(s.handleSoundVolume))

	api.HandleFunc("POST /api/auth/login", s.handleNanitLogin)
	api.HandleFunc("POST /api/auth/verify-2fa", s.handleNanitVerify2FA)
	api.HandleFunc("GET /api/auth/status", s.handleNanitStatus)
	api.HandleFunc("DELETE /api/auth/reset", s.handleNanitReset)

	api.HandleFunc("POST /api/webauth/set-password", s.handleSetPassword)
	api.HandleFunc("POST /api/webauth/change-password", s.handleChangePassword)
	api.HandleFunc("POST /api/webauth/remove-password", s.handleRemovePassword)

	api.HandleFunc("GET /api/stream/hls/{uid}/{file}", s.withBaby(s.handleHLSFile))
	// The dashboard sends these with a trailing slash
	for _, path := range []string{"/api/stream/start", "/api/stream/start/{$}"} {
		api.HandleFunc("POST "+path, s.handleStreamStart)
	}
	for _, path := range []string{"/api/stream/stop", "/api/stream/stop/{$}"} {
		api.HandleFunc("POST "+path, s.handleStreamStop)
	}
	api.HandleFunc("GET /api/stream/status/{uid}", s.withBaby(s.handleStreamStatus))

	api.HandleFunc("GET /api/history/sensor/{uid}", s.withBaby(s.withHistory(s.handleHistorySensor)))
	api.HandleFunc("GET /api/history/events/{uid}", s.withBaby(s.withHistory(s.handleHistoryEvents)))
	api.HandleFunc("GET /api/history/summary/{uid}", s.withBaby(s.withHistory(s.handleHistorySummary)))
	api.HandleFunc("GET /api/history/day-night/{uid}", s.withBaby(s.withHistory(s.handleHistoryDayNight)))
	api.HandleFunc("DELETE /api/history/reset/{uid}", s.withBaby(s.withHistory(s.handleHistoryReset)))

	mux := http.NewServeMux()
	// Signed out, a request under /api/ gets 401 before the API's routing
	// can answer anything, a 404 or 405 included
	mux.Handle("/api/", s.requireAuth(api))

	// Open: what the page needs to load and sign in, and the health checks
	mux.HandleFunc("GET /api/webauth/status", s.handleWebAuthStatus)
	mux.HandleFunc("POST /api/webauth/login", s.handleWebAuthLogin)
	mux.HandleFunc("POST /api/webauth/logout", s.handleWebAuthLogout)
	mux.HandleFunc("GET /health", s.handleLiveness)
	mux.HandleFunc("GET /ready", s.handleReadiness)

	// Vite's build output. The file names under assets/ carry a content
	// hash, so a changed file gets a new URL and each one can be cached for
	// good.
	assets := http.FileServer(http.Dir(filepath.Join(s.Config.WebDir, "assets")))
	mux.Handle("GET /assets/", immutable(http.StripPrefix("/assets/", assets)))
	// Everything else is a client-side route: serve the app's single page.
	// It takes every method: "GET /" would clash with "/api/" in the mux.
	mux.HandleFunc("/", s.handleIndex)

	return jsonErrors(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	indexPath := filepath.Join(s.Config.WebDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		log.Error().Err(err).Str("path", indexPath).Msg("Frontend index.html not found")
		http.Error(w, "Frontend not built. Run 'bun run build' in the frontend directory.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	// Revalidate every load so a new build's asset URLs are picked up
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, indexPath)
}

// immutable marks responses as safe to cache forever
func immutable(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		h.ServeHTTP(w, r)
	})
}

// signedIn - whether r carries a valid dashboard session, or no password
// is set
func (s *Server) signedIn(r *http.Request) bool {
	if !s.WebAuth.IsPasswordSet() {
		return true
	}
	cookie, err := r.Cookie(sessionCookie)
	return err == nil && s.WebAuth.ValidateSession(cookie.Value)
}

// requireAuth - the dashboard password, when one is set
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.signedIn(r) {
			writeError(w, http.StatusUnauthorized, "authentication_required", "Please log in to access this resource")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// babyHandler - a handler for one of the account's babies
type babyHandler func(w http.ResponseWriter, r *http.Request, b baby.Baby)

// withBaby resolves {uid} to one of the account's babies, or answers 404.
// The UID reaches the camera, the history database and file paths, and
// most routes used to take any.
func (s *Server) withBaby(next babyHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := s.baby(r.PathValue("uid"))
		if !ok {
			writeError(w, http.StatusNotFound, "unknown_baby", "No baby with that UID on this account")
			return
		}
		next(w, r, b)
	}
}

// baby - the account's baby with uid, as last fetched from Nanit
func (s *Server) baby(uid string) (baby.Baby, bool) {
	for _, b := range s.babies() {
		if b.UID == uid {
			return b, true
		}
	}
	return baby.Baby{}, false
}

func (s *Server) babies() []baby.Baby {
	if s.Sessions == nil {
		return nil
	}
	return s.Sessions.Babies()
}

// withHistory answers 503 while history tracking is off
func (s *Server) withHistory(next babyHandler) babyHandler {
	return func(w http.ResponseWriter, r *http.Request, b baby.Baby) {
		if s.History == nil || !s.History.IsEnabled() {
			writeError(w, http.StatusServiceUnavailable, "history_disabled", "Historical tracking is disabled")
			return
		}
		next(w, r, b)
	}
}

// camera - the running camera for b, or a 503
func (s *Server) camera(w http.ResponseWriter, b baby.Baby) (*camera.Camera, bool) {
	if s.Cameras != nil {
		if cam, ok := s.Cameras.Get(b.UID); ok {
			return cam, true
		}
	}
	writeError(w, http.StatusServiceUnavailable, "not_connected", "Camera not connected")
	return nil, false
}

// decode reads a JSON body into v, or answers 400
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Expected a JSON body")
		return false
	}
	return true
}

// timeRange - the start and end query parameters, as Unix seconds or
// RFC 3339, defaulting to the last 24 hours
func timeRange(r *http.Request) (start, end int64) {
	end = time.Now().Unix()
	start = end - 24*60*60
	if v, ok := parseTime(r.URL.Query().Get("start")); ok {
		start = v
	}
	if v, ok := parseTime(r.URL.Query().Get("end")); ok {
		end = v
	}
	return start, end
}

func parseTime(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), true
	}
	return 0, false
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Debug().Err(err).Msg("Failed to write a JSON response")
	}
}

// writeError - the one shape every API error takes
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSONStatus(w, status, apitypes.ErrorResponse{Error: code, Message: message})
}

// jsonErrors turns the mux's own plain-text 404 and 405 under /api/ into
// the API's JSON error
func jsonErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&errorRewriter{ResponseWriter: w}, r)
	})
}

type errorRewriter struct {
	http.ResponseWriter
	rewritten bool
}

func (e *errorRewriter) WriteHeader(status int) {
	plain := strings.HasPrefix(e.Header().Get("Content-Type"), "text/plain")
	if plain && (status == http.StatusNotFound || status == http.StatusMethodNotAllowed) {
		e.rewritten = true
		e.Header().Del("X-Content-Type-Options")
		if status == http.StatusNotFound {
			writeError(e.ResponseWriter, status, "not_found", "No such API endpoint")
		} else {
			writeError(e.ResponseWriter, status, "method_not_allowed", "That method isn't allowed here")
		}
		return
	}
	e.ResponseWriter.WriteHeader(status)
}

func (e *errorRewriter) Write(b []byte) (int, error) {
	if e.rewritten {
		return len(b), nil // the mux's text, already replaced
	}
	return e.ResponseWriter.Write(b)
}
