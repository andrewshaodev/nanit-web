package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var liveTestStream = relay.Stream{H264: &relay.H264{
	SPS: []byte{
		0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
		0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
		0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9, 0x20,
	},
	PPS: []byte{0x08, 0x06, 0x07, 0x08},
}}

// startLive - the API on a real listener, with a relay hub and one baby
func startLive(t *testing.T) (*Server, *relay.Hub, string) {
	t.Helper()
	s, _ := newTestServer(t)
	s.Sessions.SetBabies([]baby.Baby{{UID: "baby1"}})
	hub := relay.NewHub(s.State)
	s.Relay = hub
	stopping := make(chan struct{})
	s.Stopping = stopping
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(func() { close(stopping); srv.Close() })
	return s, hub, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dialLive(t *testing.T, s *Server, base, uid string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	id, err := s.WebAuth.CreateSession()
	require.NoError(t, err)
	header := http.Header{"Cookie": {sessionCookie + "=" + id}}
	conn, res, err := websocket.DefaultDialer.Dial(base+"/api/stream/live/"+uid, header)
	if conn != nil {
		t.Cleanup(func() { conn.Close() })
	}
	return conn, res, err
}

// feed publishes video frames, a keyframe every 10, until stopped
func feed(t *testing.T, hub *relay.Hub) *relay.Publisher {
	t.Helper()
	p := hub.Publish("baby1", liveTestStream)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
			pts := time.Duration(i) * 100 * time.Millisecond
			au, key := [][]byte{{0x41, 0x9a, 0x02, 0x03}}, false
			if i%10 == 0 {
				au, key = [][]byte{{0x65, 0x88, 0x84, 0x00, 0x33}}, true
			}
			p.Broadcast(relay.Frame{PTS: pts, DTS: pts, Video: au, Keyframe: key})
		}
	}()
	t.Cleanup(func() { close(stop); <-done })
	return p
}

func readLiveMessage(t *testing.T, conn *websocket.Conn) apitypes.LiveMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	kind, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, kind)
	var m apitypes.LiveMessage
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

func readBinary(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	kind, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.BinaryMessage, kind)
	return data
}

// The player gets the MIME type, the init segment, then fragments
func TestLiveSendsTheStream(t *testing.T) {
	s, hub, base := startLive(t)
	feed(t, hub)
	require.Eventually(t, func() bool { return hub.Publishing("baby1") }, time.Second, 10*time.Millisecond)

	conn, _, err := dialLive(t, s, base, "baby1")
	require.NoError(t, err)

	m := readLiveMessage(t, conn)
	assert.Equal(t, "stream", m.Type)
	assert.Equal(t, `video/mp4; codecs="avc1.42c028"`, m.MIME)

	var init fmp4.Init
	require.NoError(t, init.Unmarshal(bytes.NewReader(readBinary(t, conn))))
	require.Len(t, init.Tracks, 1)

	var last uint64
	for i := 0; i < 5; i++ {
		var parts fmp4.Parts
		require.NoError(t, parts.Unmarshal(readBinary(t, conn)))
		require.Len(t, parts, 1)
		base := parts[0].Tracks[0].BaseTime
		if i == 0 {
			assert.Zero(t, base, "the first fragment starts at zero")
			assert.False(t, parts[0].Tracks[0].Samples[0].IsNonSyncSample, "and at a keyframe")
		} else {
			assert.Greater(t, base, last)
		}
		last = base
	}
}

// Before the camera streams the player is told to wait, and the stream
// follows once it starts
func TestLiveWaitsForTheCamera(t *testing.T) {
	s, hub, base := startLive(t)
	conn, _, err := dialLive(t, s, base, "baby1")
	require.NoError(t, err)
	assert.Equal(t, "waiting", readLiveMessage(t, conn).Type)

	feed(t, hub)
	assert.Equal(t, "stream", readLiveMessage(t, conn).Type)
}

// When the camera's stream ends the player is told, so it reconnects
func TestLiveClosesWhenTheStreamEnds(t *testing.T) {
	s, hub, base := startLive(t)
	p := feed(t, hub)
	require.Eventually(t, func() bool { return hub.Publishing("baby1") }, time.Second, 10*time.Millisecond)
	conn, _, err := dialLive(t, s, base, "baby1")
	require.NoError(t, err)
	readLiveMessage(t, conn)
	readBinary(t, conn)

	p.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			assert.True(t, websocket.IsCloseError(err, liveStreamEnded), "got %v", err)
			return
		}
	}
}

// Only the account's babies, and only when signed in
func TestLiveNeedsAKnownBabyAndASession(t *testing.T) {
	s, _, base := startLive(t)
	_, res, err := dialLive(t, s, base, "unknown")
	require.Error(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	_, res, err = websocket.DefaultDialer.Dial(base+"/api/stream/live/baby1", nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}
