package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConnection - opens a real websocket against a server that reads and
// discards everything, and returns a connection wrapping the client end.
func newTestConnection(t *testing.T) (*WebsocketConnection, *httptest.Server) {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		for {
			if _, _, err := serverConn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	clientConn, _, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http://", "ws://", 1), nil)
	require.NoError(t, err)

	conn := NewWebsocketConnection(clientConn)

	t.Cleanup(func() {
		clientConn.Close()
		server.Close()
	})

	return conn, server
}

func keepaliveMessage() *Message {
	return &Message{Type: Message_Type(Message_KEEPALIVE).Enum()}
}

func TestSendMessageSucceedsOnLiveConnection(t *testing.T) {
	conn, _ := newTestConnection(t)

	assert.NoError(t, conn.SendMessage(keepaliveMessage()))
}

// TestSendMessageReportsWriteFailure - gowebsocket's SendBinary swallowed the
// write error, which is why a half-open connection could be written to forever
// without anything noticing. A dead connection must surface as an error.
func TestSendMessageReportsWriteFailure(t *testing.T) {
	conn, _ := newTestConnection(t)

	require.NoError(t, conn.Close())

	assert.Error(t, conn.SendMessage(keepaliveMessage()))
}

func TestCloseIsIdempotent(t *testing.T) {
	conn, _ := newTestConnection(t)

	require.NoError(t, conn.Close())
	// A second close reports the underlying error rather than panicking, which
	// matters because both the health watchdog and the attempt cleanup close.
	assert.NotPanics(t, func() { _ = conn.Close() })
}

func TestLastReceivedTracksIncomingMessages(t *testing.T) {
	conn, _ := newTestConnection(t)

	conn.lastReceivedUnixNano.Store(time.Now().Add(-time.Hour).UnixNano())
	require.True(t, time.Since(conn.LastReceived()) > 30*time.Minute)

	conn.MarkReceived()

	assert.True(t, time.Since(conn.LastReceived()) < time.Minute)
}

// TestTimedOutRequestReleasesHandler - a request that times out used to leave
// its response handler in the map for the life of the connection, so a
// long-lived connection leaked one entry per unanswered request.
func TestTimedOutRequestReleasesHandler(t *testing.T) {
	conn, _ := newTestConnection(t)

	awaitResponse := conn.SendRequest(RequestType_GET_STATUS, &Request{
		GetStatus_: &GetStatus{All: utils.ConstRefBool(true)},
	})

	conn.resHandlersMu.RLock()
	pending := len(conn.resHandlers)
	conn.resHandlersMu.RUnlock()
	require.Equal(t, 1, pending)

	_, err := awaitResponse(10 * time.Millisecond)
	assert.Error(t, err)

	conn.resHandlersMu.RLock()
	remaining := len(conn.resHandlers)
	conn.resHandlersMu.RUnlock()
	assert.Equal(t, 0, remaining, "timed out request should not leave a handler behind")
}

// A response can arrive just as its request times out, and frames are
// handled concurrently, so copies can arrive together too. The response
// channel used to be closed on timeout, and a send racing that close
// panics. Run with -race.
func TestResponseRacingTimeout(t *testing.T) {
	conn, _ := newTestConnection(t)

	for i := range 200 {
		awaitResponse := conn.SendRequest(RequestType_GET_STATUS, &Request{})
		res := &Response{
			RequestId:   utils.ConstRefInt32(int32(i + 1)),
			RequestType: RequestType_GET_STATUS.Enum(),
			StatusCode:  utils.ConstRefInt32(200),
		}

		done := make(chan struct{})
		for range 2 {
			go func() {
				conn.handleResponse(res)
				done <- struct{}{}
			}()
		}
		_, _ = awaitResponse(time.Duration(i%3) * time.Microsecond)
		<-done
		<-done
	}

	conn.resHandlersMu.RLock()
	defer conn.resHandlersMu.RUnlock()
	assert.Empty(t, conn.resHandlers)
}
