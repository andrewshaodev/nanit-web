package mqtt

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// CommandHandlers - what a camera does with the switch commands sent to it
type CommandHandlers struct {
	NightLight func(on bool)
	Standby    func(on bool)
}

// Connection - MQTT context
type Connection struct {
	Opts         Opts
	StateManager *baby.StateManager
	client       MQTT.Client

	// Each camera's handlers, by baby UID. A camera registers when its
	// websocket connects and unregisters when it drops.
	mu       sync.Mutex
	handlers map[string]registration
	nextID   int
}

type registration struct {
	id int
	CommandHandlers
}

// NewConnection - constructor
func NewConnection(opts Opts) *Connection {
	return &Connection{
		Opts:     opts,
		handlers: map[string]registration{},
	}
}

// Run - runs the mqtt connection handler
func (conn *Connection) Run(manager *baby.StateManager, ctx utils.GracefulContext) {
	conn.StateManager = manager

	opts := MQTT.NewClientOptions()
	opts.AddBroker(conn.Opts.BrokerURL)
	opts.SetClientID(conn.Opts.ClientID)
	opts.SetUsername(conn.Opts.Username)
	opts.SetPassword(conn.Opts.Password)
	opts.SetCleanSession(false)

	conn.client = MQTT.NewClient(opts)

	utils.RunWithPerseverance(func(attempt utils.AttemptContext) {
		runMqtt(conn, attempt)
	}, ctx, utils.PerseverenceOpts{
		RunnerID:       "mqtt",
		ResetThreshold: 2 * time.Second,
		Cooldown: []time.Duration{
			2 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		},
	})
}

// RegisterBaby routes the switch commands for babyUID to h, until the
// returned function is called. There used to be one handler for all
// cameras, so a command went to whichever camera connected last.
func (conn *Connection) RegisterBaby(babyUID string, h CommandHandlers) (unregister func()) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.nextID++
	id := conn.nextID
	conn.handlers[babyUID] = registration{id: id, CommandHandlers: h}

	return func() {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		// A reconnect may have registered a newer connection meanwhile
		if conn.handlers[babyUID].id == id {
			delete(conn.handlers, babyUID)
		}
	}
}

// commandTopic splits <prefix>/babies/<uid>/<control>/switch. The prefix
// may itself contain slashes.
func (conn *Connection) commandTopic(topic string) (babyUID, control string, ok bool) {
	rest, found := strings.CutPrefix(topic, conn.Opts.TopicPrefix+"/babies/")
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[2] != "switch" || baby.EnsureValidBabyUID(parts[0]) != nil {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// handleCommand runs a switch command on the camera its topic names
func (conn *Connection) handleCommand(topic string, payload []byte) {
	babyUID, control, ok := conn.commandTopic(topic)
	if !ok {
		log.Warn().Str("topic", topic).Msg("Ignoring MQTT message on an unexpected topic")
		return
	}

	conn.mu.Lock()
	h, registered := conn.handlers[babyUID]
	conn.mu.Unlock()

	var handler func(bool)
	switch control {
	case "night_light":
		handler = h.NightLight
	case "standby":
		handler = h.Standby
	default:
		log.Warn().Str("topic", topic).Msg("Unknown MQTT command")
		return
	}

	enabled := string(payload) == "true"
	sublog := log.With().Str("baby_uid", babyUID).Str("control", control).Bool("enabled", enabled).Logger()
	// Commands can arrive before the camera connects, or after it drops:
	// the broker keeps them for us (CleanSession is off)
	if !registered || handler == nil {
		sublog.Warn().Msg("Dropping MQTT command: camera not connected")
		return
	}
	sublog.Debug().Msg("Received MQTT command")
	handler(enabled)
}

func (conn *Connection) subscribeToCommands() {
	for _, control := range []string{"night_light", "standby"} {
		topic := fmt.Sprintf("%v/babies/+/%v/switch", conn.Opts.TopicPrefix, control)
		log.Debug().Str("topic", topic).Msg("Subscribing to command topic")

		token := conn.client.Subscribe(topic, 0, func(_ MQTT.Client, msg MQTT.Message) {
			conn.handleCommand(msg.Topic(), msg.Payload())
		})
		if token.Wait() && token.Error() != nil {
			log.Error().Err(token.Error()).Str("topic", topic).Msg("Failed to subscribe to command topic")
		}
	}
}

// redactURL - the broker URL for logging. It may carry credentials
// (mqtt://user:pass@host), and they used to land in the log on every connect.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		// Not parseable, so don't risk echoing a password
		return "(unparseable broker URL)"
	}
	return u.Redacted()
}

func runMqtt(conn *Connection, attempt utils.AttemptContext) {

	if token := conn.client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Str("broker_url", redactURL(conn.Opts.BrokerURL)).Err(token.Error()).Msg("Unable to connect to MQTT broker")
		attempt.Fail(token.Error())
		return
	}

	log.Info().Str("broker_url", redactURL(conn.Opts.BrokerURL)).Msg("Successfully connected to MQTT broker")

	unsubscribe := conn.StateManager.Subscribe(func(babyUID string, state baby.State) {
		publish := func(key string, value interface{}) {
			topic := fmt.Sprintf("%v/babies/%v/%v", conn.Opts.TopicPrefix, babyUID, key)
			log.Trace().Str("topic", topic).Interface("value", value).Msg("MQTT publish")

			token := conn.client.Publish(topic, 0, false, fmt.Sprintf("%v", value))
			if token.Wait(); token.Error() != nil {
				log.Error().Err(token.Error()).Msgf("Unable to publish %v update", key)
			}
		}

		for key, value := range state.AsMap(false) {
			publish(key, value)
		}

		if state.StreamState != nil && *state.StreamState != baby.StreamState_Unknown {
			publish("is_stream_alive", *state.StreamState == baby.StreamState_Alive)
		}
	})

	conn.subscribeToCommands()

	// Wait until interrupt signal is received
	<-attempt.Done()

	log.Debug().Msg("Closing MQTT connection on interrupt")
	unsubscribe()
	conn.client.Disconnect(250)
}
