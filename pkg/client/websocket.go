package client

import (
	"errors"
	"fmt"
	sync "sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sacOO7/gowebsocket"
	"github.com/indiefan/home_assistant_nanit/pkg/baby"
	"github.com/indiefan/home_assistant_nanit/pkg/utils"
	"google.golang.org/protobuf/proto"
)

// errTokenRenewal - reason recorded when a healthy connection is retired on
// purpose so it can be reopened with a fresh token
var errTokenRenewal = errors.New("closing connection to renew the auth token")

// errConnectionStale - reason recorded when the camera stopped answering
var errConnectionStale = errors.New("camera stopped responding")

type readyState struct {
	Context    utils.GracefulContext
	Connection *WebsocketConnection
}

// WebsocketConnectionHandler - handler of ready connection
type WebsocketConnectionHandler func(*WebsocketConnection, utils.GracefulContext)

// WebsocketConnectionManager - connection manager
type WebsocketConnectionManager struct {
	BabyUID          string
	CameraUID        string
	API              *NanitClient
	BabyStateManager *baby.StateManager

	mu               sync.RWMutex
	readyState       *readyState
	readySubscribers []WebsocketConnectionHandler
}

// NewWebsocketConnectionManager - constructor
func NewWebsocketConnectionManager(babyUID string, cameraUID string, api *NanitClient, babyStateManager *baby.StateManager) *WebsocketConnectionManager {
	manager := &WebsocketConnectionManager{
		BabyUID:          babyUID,
		CameraUID:        cameraUID,
		API:              api,
		BabyStateManager: babyStateManager,
	}

	manager.WithReadyConnection(manager.watchConnectionHealth)

	return manager
}

// watchConnectionHealth - keeps the connection warm and tears it down once it
// stops carrying traffic.
//
// Keepalives are one-way and the underlying library has no read deadline, so a
// connection whose token expired server-side stays open and silent forever
// while every command sent over it times out. Silence is therefore treated as
// suspicious: the camera is asked a question it always answers, and failing to
// answer ends the connection so the attempt loop can reconnect.
func (manager *WebsocketConnectionManager) watchConnectionHealth(conn *WebsocketConnection, ctx utils.GracefulContext) {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	drop := func(reason error) {
		log.Error().Err(reason).Str("camera_uid", manager.CameraUID).Msg("Dropping websocket connection")
		if err := conn.Close(); err != nil {
			log.Debug().Err(err).Msg("Error while closing stale websocket connection")
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.SendMessage(&Message{
				Type: Message_Type(Message_KEEPALIVE).Enum(),
			}); err != nil {
				// A write that fails is the connection telling us it is gone.
				drop(fmt.Errorf("keepalive write failed: %w", err))
				return
			}

			if time.Since(conn.LastReceived()) < livenessProbeAfter {
				continue
			}

			if !manager.probeConnection(conn, ctx) {
				drop(errConnectionStale)
				return
			}
		}
	}
}

// probeConnection - asks the camera for its status and reports whether it
// answered before the probe timeout. Cancellation counts as success: the
// connection is being torn down for other reasons and does not need dropping.
func (manager *WebsocketConnectionManager) probeConnection(conn *WebsocketConnection, ctx utils.GracefulContext) bool {
	log.Debug().
		Str("camera_uid", manager.CameraUID).
		Dur("silent_for", time.Since(conn.LastReceived())).
		Msg("Connection has been quiet, probing the camera")

	awaitResponse := conn.SendRequest(RequestType_GET_STATUS, &Request{
		GetStatus_: &GetStatus{
			All: utils.ConstRefBool(true),
		},
	})

	resultC := make(chan error, 1)
	go func() {
		_, err := awaitResponse(livenessProbeTimeout)
		resultC <- err
	}()

	select {
	case <-ctx.Done():
		return true
	case err := <-resultC:
		if err != nil {
			log.Warn().Err(err).Str("camera_uid", manager.CameraUID).Msg("Camera did not answer the liveness probe")
			return false
		}

		return true
	}
}

// WithReadyConnection - registers handler which will be called as a go routine upon ready connection
func (manager *WebsocketConnectionManager) WithReadyConnection(handler WebsocketConnectionHandler) {
	manager.mu.Lock()
	readyState := manager.readyState
	manager.readySubscribers = append(manager.readySubscribers, handler)
	manager.mu.Unlock()

	if readyState != nil {
		log.Debug().Msg("Immediately notifying ready handler")
		notifyReadyHandler(handler, *readyState)
	}
}

// RunWithinContext - starts websocket connection attempt loop
func (manager *WebsocketConnectionManager) RunWithinContext(ctx utils.GracefulContext) {
	utils.RunWithPerseverance(manager.run, ctx, utils.PerseverenceOpts{
		RunnerID:       fmt.Sprintf("websocket-%v", manager.CameraUID),
		ResetThreshold: 2 * time.Second,
		Cooldown: []time.Duration{
			// 2 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
		},
	})
}

func (manager *WebsocketConnectionManager) run(attempt utils.AttemptContext) {
	// Reauthorize if it is not a first try or we assume we don't have a valid token.
	// A failure here is fatal to the attempt: dialing with a token we already
	// know is stale only produces a connection that cannot be used.
	if err := manager.API.MaybeAuthorize(attempt.GetTry() > 1); err != nil {
		log.Error().Err(err).Msg("Unable to authorize before opening the websocket")
		attempt.Fail(err)
		return
	}

	// Read the token from the store rather than from a pointer captured when
	// the manager was built: the session can be replaced wholesale on re-login,
	// which used to pin every reconnect to the token from the original session.
	authToken, authTime := manager.API.SessionStore.Credentials()
	if authToken == "" {
		err := errors.New("no auth token available")
		log.Error().Err(err).Msg("Refusing to open a websocket without a token")
		attempt.Fail(err)
		return
	}

	// Remote
	url := fmt.Sprintf("wss://api.nanit.com/focus/cameras/%v/user_connect", manager.CameraUID)
	auth := fmt.Sprintf("Bearer %v", authToken)

	// Local
	// url := "wss://192.168.3.195:442"
	// auth := fmt.Sprintf("token %v", userCamToken)

	// -------

	var once sync.Once // Just because gowebsocket is buggy and can invoke OnDisconnect multiple times :-/

	var connMu sync.Mutex
	var conn *WebsocketConnection

	currentConnection := func() *WebsocketConnection {
		connMu.Lock()
		defer connMu.Unlock()
		return conn
	}

	socket := gowebsocket.New(url)
	socket.RequestHeader.Set("Authorization", auth)

	// Handle new connection
	socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Info().Str("url", url).Msg("Connected to websocket")

		// The connection is published before the read loop starts, so a frame
		// arriving immediately cannot find a half-built state to dereference.
		newConn := NewWebsocketConnection(&socket)
		readyState := readyState{attempt, newConn}

		connMu.Lock()
		conn = newConn
		connMu.Unlock()

		manager.mu.Lock()
		manager.readyState = &readyState
		subscribedHandlers := make([]WebsocketConnectionHandler, len(manager.readySubscribers))
		copy(subscribedHandlers, manager.readySubscribers)
		manager.mu.Unlock()

		go func() {
			manager.BabyStateManager.Update(manager.BabyUID, *baby.NewState().SetWebsocketAlive(true))

			log.Trace().Int("num_handlers", len(subscribedHandlers)).Msg("Notifying websocket ready handlers")

			for _, handler := range subscribedHandlers {
				notifyReadyHandler(handler, readyState)
			}
		}()
	}

	// Handle failed attempts for connection
	socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Error().Str("url", url).Err(err).Msg("Unable to establish websocket connection")
		attempt.Fail(err)
	}

	// Handle lost connection
	socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		once.Do(func() {
			manager.BabyStateManager.Update(manager.BabyUID, *baby.NewState().SetWebsocketAlive(false))

			if err != nil {
				log.Error().Err(err).Msg("Disconnected from server")
				attempt.Fail(err)
			} else {
				log.Warn().Msg("Disconnected from server")
				attempt.Fail(errors.New("Server closed the connection"))
			}
		})
	}

	socket.OnBinaryMessage = func(data []byte, _ gowebsocket.Socket) {
		activeConn := currentConnection()
		if activeConn == nil {
			log.Warn().Msg("Received a message before the connection was ready, dropping it")
			return
		}

		// Anything arriving on the socket proves the peer is still there, even
		// if we cannot make sense of the frame itself.
		activeConn.MarkReceived()

		m := &Message{}
		err := proto.Unmarshal(data, m)
		if err != nil {
			log.Error().Err(err).Bytes("rawdata", data).Msg("Received malformed binary message")
			return
		}

		log.Debug().Stringer("data", m).Msg("Received message")

		go activeConn.handleMessage(m)
	}

	log.Trace().Msg("Connecting to websocket")
	socket.Connect()

	// The Authorization header is fixed for the life of the connection and the
	// camera stops answering once the token behind it expires, without ever
	// closing the socket. Retire the connection while the token is still good
	// so the attempt loop reopens it with a fresh one. Landing just inside the
	// window where MaybeAuthorize considers the token stale is what makes that
	// reconnect pick up a new one rather than reuse this one.
	expiry := authTokenExpiry(authToken, authTime)
	renewIn := time.Until(expiry.Add(connectionRenewGrace - AuthTokenRenewMargin))
	if renewIn < minConnectionLifetime {
		renewIn = minConnectionLifetime
	}

	renewTimer := time.NewTimer(renewIn)
	defer renewTimer.Stop()

	log.Debug().
		Str("camera_uid", manager.CameraUID).
		Dur("renew_in", renewIn).
		Msg("Websocket connection scheduled for token renewal")

	select {
	case <-attempt.Done():
	case <-renewTimer.C:
		log.Info().
			Str("camera_uid", manager.CameraUID).
			Msg("Auth token is nearing expiry, reconnecting with a fresh one")
		attempt.Fail(errTokenRenewal)
	}

	if activeConn := currentConnection(); activeConn != nil {
		log.Debug().Msg("Closing websocket")
		if err := activeConn.Close(); err != nil {
			log.Debug().Err(err).Msg("Error while closing websocket")
		}
	}
}

func notifyReadyHandler(handler WebsocketConnectionHandler, state readyState) {
	state.Context.RunAsChild(func(childCtx utils.GracefulContext) {
		handler(state.Connection, childCtx)
	})
}
