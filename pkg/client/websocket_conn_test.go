package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/indiefan/home_assistant_nanit/pkg/utils"
	"github.com/sacOO7/gowebsocket"
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

	socket := gowebsocket.Socket{Conn: clientConn}
	conn := NewWebsocketConnection(&socket)

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
