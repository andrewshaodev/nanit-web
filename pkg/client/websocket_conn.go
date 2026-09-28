package client

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/indiefan/home_assistant_nanit/pkg/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// WebsocketMessageHandler - message handler
type WebsocketMessageHandler func(*Message, *WebsocketConnection)

// WebsocketConnection - ready websocket connection
type WebsocketConnection struct {
	ws *websocket.Conn

	msgHandlersMu sync.RWMutex
	msgHandlers   []WebsocketMessageHandler

	resHandlersMu sync.RWMutex
	resHandlers   map[int32]unhandledRequest

	// writeMu - serialises writes to the underlying gorilla connection, which
	// supports only one concurrent writer
	writeMu sync.Mutex

	// lastReceivedUnixNano - when we last heard anything from the camera, used
	// to decide whether the connection still deserves the benefit of the doubt
	lastReceivedUnixNano atomic.Int64

	lastRequestID int32
}

// NewWebsocketConnection - constructor
func NewWebsocketConnection(ws *websocket.Conn) *WebsocketConnection {
	conn := &WebsocketConnection{
		ws:            ws,
		resHandlers:   make(map[int32]unhandledRequest),
		lastRequestID: 0,
	}

	conn.MarkReceived()
	return conn
}

// MarkReceived - records that a frame arrived from the camera
func (conn *WebsocketConnection) MarkReceived() {
	conn.lastReceivedUnixNano.Store(time.Now().UnixNano())
}

// LastReceived - when we last heard anything from the camera
func (conn *WebsocketConnection) LastReceived() time.Time {
	return time.Unix(0, conn.lastReceivedUnixNano.Load())
}

// Close - tears the connection down.
//
// Closing the underlying connection makes the read loop fail, which is what
// reports the disconnect and lets the attempt reconnect.
func (conn *WebsocketConnection) Close() error {
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	if conn.ws == nil {
		return nil
	}

	// Best effort: tell the server why we are going away, then drop the socket
	// regardless of whether the courtesy frame made it out.
	if err := conn.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err == nil {
		if err := conn.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Debug().Err(err).Msg("Unable to send websocket close frame")
		}
	}

	return conn.ws.Close()
}

// write - sends a single frame, serialised against other writers.
//
// The write error is returned rather than just logged: a failed write is the
// earliest evidence that a connection has gone half-open.
func (conn *WebsocketConnection) write(messageType int, data []byte) error {
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	if conn.ws == nil {
		return errors.New("websocket connection is not established")
	}

	if err := conn.ws.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	return conn.ws.WriteMessage(messageType, data)
}

// RegisterMessageHandler - registers handler which will be called whenever new message is received
func (conn *WebsocketConnection) RegisterMessageHandler(handler WebsocketMessageHandler) {
	conn.msgHandlersMu.Lock()
	conn.msgHandlers = append(conn.msgHandlers, handler)
	conn.msgHandlersMu.Unlock()
}

// SendMessage - low-level helper for sending raw message
// Note: Use SendRequest() for requests
func (conn *WebsocketConnection) SendMessage(m *Message) error {
	var msg *zerolog.Event

	if *m.Type == Message_KEEPALIVE {
		msg = log.Trace()
	} else {
		msg = log.Debug()
	}

	msg.Stringer("data", m).Msg("Sending message")

	bytes, err := getMessageBytes(m)
	if err != nil {
		return fmt.Errorf("failed to marshal websocket message: %w", err)
	}
	log.Trace().Bytes("rawdata", bytes).Msg("Sending data")

	if err := conn.write(websocket.BinaryMessage, bytes); err != nil {
		return fmt.Errorf("failed to write websocket message: %w", err)
	}

	return nil
}

// SendRequest - sends request to the cam and returns await function. Await function waits for the response and returns it
func (conn *WebsocketConnection) SendRequest(reqType RequestType, requestData *Request) func(time.Duration) (*Response, error) {
	// Build request
	id := atomic.AddInt32(&conn.lastRequestID, 1)

	requestData.Id = utils.ConstRefInt32(id)
	requestData.Type = RequestType(reqType).Enum()

	m := &Message{
		Type:    Message_Type(Message_REQUEST).Enum(),
		Request: requestData,
	}

	// Response handling
	resC := make(chan *Response, 1)

	conn.resHandlersMu.Lock()
	conn.resHandlers[id] = unhandledRequest{
		Request: m.Request,
		HandleResponse: func(res *Response) {
			select {
			case <-resC:
				return // Channel already closed (ie. timeout)
			default:
				resC <- res
			}
		},
	}
	conn.resHandlersMu.Unlock()

	// Send request
	if err := conn.SendMessage(m); err != nil {
		log.Error().Err(err).Msg("Failed to send websocket message")
		// Return an awaiter that immediately returns the error
		return func(timeout time.Duration) (*Response, error) {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
	}

	// Return awaiter
	return func(timeout time.Duration) (*Response, error) {
		timer := time.NewTimer(timeout)

		select {
		case <-timer.C:
			// Drop the pending handler, otherwise every request that times out
			// leaves an entry behind for the lifetime of the connection.
			conn.resHandlersMu.Lock()
			delete(conn.resHandlers, id)
			conn.resHandlersMu.Unlock()

			close(resC)
			return nil, errors.New("Request timeout")
		case res := <-resC:
			close(resC)
			timer.Stop()

			if res.StatusCode == nil {
				return res, errors.New("No status code received")
			} else if *res.StatusCode != 200 {
				if res.GetStatusMessage() != "" {
					return res, errors.New(res.GetStatusMessage())
				}

				return res, fmt.Errorf("Unexpected status code %v", *res.StatusCode)
			}

			return res, nil
		}
	}
}

type unhandledRequest struct {
	Request        *Request
	HandleResponse func(response *Response)
}

func (conn *WebsocketConnection) handleResponse(r *Response) {
	requestID := *r.RequestId
	requestType := *r.RequestType

	conn.resHandlersMu.RLock()
	unhandledReqCandidate, ok := conn.resHandlers[requestID]
	conn.resHandlersMu.RUnlock()

	if ok && requestType == *unhandledReqCandidate.Request.Type {
		conn.resHandlersMu.Lock()
		delete(conn.resHandlers, requestID)
		conn.resHandlersMu.Unlock()

		unhandledReqCandidate.HandleResponse(r)
	}
}

func (conn *WebsocketConnection) handleMessage(m *Message) {
	conn.MarkReceived()

	if *m.Type == Message_RESPONSE && m.Response != nil {
		conn.handleResponse(m.Response)
	}

	conn.msgHandlersMu.RLock()
	subscribedHandlers := make([]WebsocketMessageHandler, len(conn.msgHandlers))
	copy(subscribedHandlers, conn.msgHandlers)
	conn.msgHandlersMu.RUnlock()

	for _, handler := range subscribedHandlers {
		handler(m, conn)
	}
}

func getMessageBytes(data *Message) ([]byte, error) {
	out, err := proto.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal data")
		return nil, err
	}

	return out, nil
}
