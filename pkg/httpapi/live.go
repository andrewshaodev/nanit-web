package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/mse"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// liveWriteTimeout - a player that takes longer than this to accept a
	// message has stalled
	liveWriteTimeout = 10 * time.Second
	// liveWaitFor - how long a player waits for the camera to start
	// streaming before it's told there's no stream
	liveWaitFor = 60 * time.Second
	livePoll    = 250 * time.Millisecond
)

// The live view's close codes (4000-4999 are for applications). The player
// reconnects after either.
const (
	// liveStreamEnded - the camera's stream ended or restarted, or the
	// player fell behind
	liveStreamEnded = 4000
	// liveNoStream - the camera didn't start streaming in liveWaitFor
	liveNoStream = 4001
)

// The browser sends the dashboard's cookie with the upgrade, and the
// upgrader's default origin check refuses other sites' pages
var liveUpgrader = websocket.Upgrader{WriteBufferSize: 64 * 1024}

// handleLive - the dashboard's live view, a WebSocket: a "waiting" message
// while the camera isn't streaming, then a "stream" message with the MIME
// type, the fMP4 init segment, and a fragment per video frame
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	if s.Relay == nil {
		writeError(w, http.StatusServiceUnavailable, "no_live_stream", "The live view isn't available")
		return
	}
	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader has answered
	}
	defer conn.Close()
	// The HTTP server's deadlines carry over to a hijacked connection; the
	// live view sets its own on each write
	_ = conn.SetReadDeadline(time.Time{})

	sublog := log.With().Str("baby_uid", b.UID).Str("remote", r.RemoteAddr).Logger()

	// The player sends nothing; reading is how its leaving is noticed
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	stream, sub, ok := s.waitForLiveStream(conn, b.UID, gone)
	if !ok {
		return
	}
	defer sub.Close()

	sublog.Debug().Msg("Live view started")
	code, reason := s.sendLive(conn, stream, sub, gone, sublog)
	if code != 0 {
		closeLive(conn, code, reason)
	}
	sublog.Debug().Str("reason", reason).Msg("Live view ended")
}

// waitForLiveStream - a subscription to the camera's stream, once there is
// one. ok is false if the player left, the bridge is stopping, or the wait
// ran out (the player is then told so).
func (s *Server) waitForLiveStream(conn *websocket.Conn, uid string, gone <-chan struct{}) (relay.Stream, *relay.Subscription, bool) {
	if stream, sub, ok := s.Relay.Subscribe(uid); ok {
		return stream, sub, true
	}
	if writeLive(conn, websocket.TextMessage, liveMessage(apitypes.LiveMessage{Type: "waiting"})) != nil {
		return relay.Stream{}, nil, false
	}
	deadline := time.After(liveWaitFor)
	tick := time.NewTicker(livePoll)
	defer tick.Stop()
	for {
		select {
		case <-gone:
			return relay.Stream{}, nil, false
		case <-s.Stopping:
			closeLive(conn, websocket.CloseGoingAway, "the bridge is stopping")
			return relay.Stream{}, nil, false
		case <-deadline:
			closeLive(conn, liveNoStream, "the camera isn't streaming")
			return relay.Stream{}, nil, false
		case <-tick.C:
			if stream, sub, ok := s.Relay.Subscribe(uid); ok {
				return stream, sub, true
			}
		}
	}
}

// sendLive - the stream to the player, until it ends or the player leaves.
// Returns the close code and reason to send, or 0 when the player has gone.
func (s *Server) sendLive(conn *websocket.Conn, stream relay.Stream, sub *relay.Subscription, gone <-chan struct{}, sublog zerolog.Logger) (int, string) {
	muxer, err := mse.New(stream)
	if err != nil {
		return liveStreamEnded, err.Error()
	}
	init, err := muxer.Init()
	if err != nil {
		sublog.Error().Err(err).Msg("Failed to make the live view's init segment")
		return websocket.CloseInternalServerErr, "init segment"
	}
	header := liveMessage(apitypes.LiveMessage{Type: "stream", MIME: muxer.MIME()})
	if writeLive(conn, websocket.TextMessage, header) != nil || writeLive(conn, websocket.BinaryMessage, init) != nil {
		return 0, "player gone"
	}

	for {
		select {
		case <-gone:
			return 0, "player gone"
		case <-s.Stopping:
			return websocket.CloseGoingAway, "the bridge is stopping"
		case f, ok := <-sub.Frames:
			if !ok {
				if sub.FellBehind() {
					return liveStreamEnded, "fell behind"
				}
				return liveStreamEnded, "the stream ended"
			}
			frag, err := muxer.Write(f)
			if err != nil {
				sublog.Debug().Err(err).Msg("Dropping a frame the live view couldn't package")
				continue
			}
			if frag == nil {
				continue
			}
			if writeLive(conn, websocket.BinaryMessage, frag) != nil {
				return 0, "player gone"
			}
		}
	}
}

func writeLive(conn *websocket.Conn, messageType int, data []byte) error {
	_ = conn.SetWriteDeadline(time.Now().Add(liveWriteTimeout))
	return conn.WriteMessage(messageType, data)
}

func closeLive(conn *websocket.Conn, code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
}

// liveMessage - a text message; it has only strings, so it always encodes
func liveMessage(m apitypes.LiveMessage) []byte {
	b, _ := json.Marshal(m)
	return b
}
