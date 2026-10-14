package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/talk"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	// talkStartTimeout - how long the dashboard has to say its sample rate
	talkStartTimeout = 5 * time.Second
	// talkPlayTimeout - how long the camera has to start playing once it
	// has agreed to
	talkPlayTimeout = 15 * time.Second
	// talkIdleTimeout - the microphone sends audio continuously, so this
	// long without any means the dashboard has gone
	talkIdleTimeout = 5 * time.Second
	// talkMaxDuration - a talk is cut off after this, in case the button
	// is left held
	talkMaxDuration = 5 * time.Minute
	// talkMaxMessage - one message of audio; a tenth of a second at 48 kHz
	// is under 10 kB
	talkMaxMessage = 256 * 1024
)

// The talk-back close codes, as the live view's (4000-4999)
const (
	// talkBusy - someone else is talking to this camera
	talkBusy = 4010
	// talkRefused - the camera didn't agree to play, or never started
	talkRefused = 4011
	// talkTooLong - talkMaxDuration ran out
	talkTooLong = 4012
)

// talking - the cameras someone is talking to, by baby UID: one at a time
var talking sync.Map

// handleTalk - talking through the camera's speaker, a WebSocket. The
// dashboard sends TalkStart, then the microphone's audio as binary PCM
// messages; it's told "connecting", then "talking" once the camera plays.
// Closing the socket ends the talk, and the camera is told to stop.
func (s *Server) handleTalk(w http.ResponseWriter, r *http.Request, b baby.Baby) {
	if s.Talk == nil || s.Config.RTMPPublicAddr == "" {
		writeError(w, http.StatusServiceUnavailable, "rtmp_disabled", "Talking needs the RTMP server, which the camera plays from")
		return
	}
	cam, ok := s.camera(w, b)
	if !ok {
		return
	}
	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader has answered
	}
	defer conn.Close()
	conn.SetReadLimit(talkMaxMessage)
	sublog := log.With().Str("baby_uid", b.UID).Str("remote", r.RemoteAddr).Logger()

	if _, busy := talking.LoadOrStore(b.UID, struct{}{}); busy {
		closeLive(conn, talkBusy, "someone is already talking to this camera")
		return
	}
	defer talking.Delete(b.UID)

	// What the dashboard records at
	var start apitypes.TalkStart
	_ = conn.SetReadDeadline(time.Now().Add(talkStartTimeout))
	if err := conn.ReadJSON(&start); err != nil {
		closeLive(conn, websocket.CloseUnsupportedData, "expected the sample rate first")
		return
	}
	sess, err := s.Talk.Start(start.SampleRate)
	if err != nil {
		sublog.Error().Err(err).Msg("Couldn't start talk-back")
		closeLive(conn, websocket.CloseInternalServerErr, err.Error())
		return
	}
	defer sess.Close()
	url := talk.URL(s.Config.RTMPPublicAddr, sess.SID)
	sublog = sublog.With().Str("sid", sess.SID).Logger()

	// Messages go out from here and from the reader below
	var writeMu sync.Mutex
	say := func(t string) {
		msg, _ := json.Marshal(apitypes.TalkMessage{Type: t})
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = writeLive(conn, websocket.TextMessage, msg)
	}
	end := func(code int, reason string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		closeLive(conn, code, reason)
	}

	// The audio, read while the camera is asked: what's said before it
	// plays is dropped, as nobody's listening yet
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(talkIdleTimeout))
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if kind == websocket.BinaryMessage {
				if err := sess.Write(data); err != nil {
					sublog.Warn().Err(err).Msg("Talk-back encoder stopped taking audio")
					return
				}
			}
		}
	}()

	say("connecting")
	playing, err := cam.StartTalk(url, sess.SID)
	if err != nil {
		sublog.Warn().Err(err).Msg("Camera refused talk-back")
		end(talkRefused, "the camera didn't agree to play: "+err.Error())
		return
	}
	defer func() {
		if err := cam.StopTalk(url, sess.SID); err != nil {
			sublog.Warn().Err(err).Msg("Camera didn't confirm talk-back stopped")
		}
		sublog.Info().Msg("Talk-back ended")
	}()

	select {
	case <-playing:
		sublog.Info().Msg("Camera playing talk-back")
		say("talking")
	case <-time.After(talkPlayTimeout):
		end(talkRefused, "the camera didn't start playing")
		return
	case <-gone:
		return
	case <-s.Stopping:
		end(websocket.CloseGoingAway, "the bridge is stopping")
		return
	}

	select {
	case <-gone:
	case <-time.After(talkMaxDuration):
		end(talkTooLong, "talking is limited to 5 minutes at a time")
	case <-s.Stopping:
		end(websocket.CloseGoingAway, "the bridge is stopping")
	}
}
