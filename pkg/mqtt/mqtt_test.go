package mqtt

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Broker URLs can carry credentials, which used to be logged on every connect
func TestRedactURLHidesPassword(t *testing.T) {
	assert.Equal(t, "mqtt://homeassistant:xxxxx@192.168.1.10:1883", redactURL("mqtt://homeassistant:s3cret@192.168.1.10:1883"))
	assert.Equal(t, "tcp://192.168.1.10:1883", redactURL("tcp://192.168.1.10:1883"))
	assert.NotContains(t, redactURL("mqtt://u:p%zz@host"), "p%zz")
}

// Each camera gets the commands sent to its own topic. There used to be one
// handler, so every command went to the camera that connected last, and a
// command arriving before any camera connected called a nil func.
func TestCommandsGoToTheNamedCamera(t *testing.T) {
	conn := NewConnection(Opts{TopicPrefix: "home/nanit"})
	got := map[string][]string{}
	for _, uid := range []string{"baby1", "baby2"} {
		conn.RegisterBaby(uid, CommandHandlers{
			NightLight: func(on bool) { got[uid] = append(got[uid], fmt.Sprintf("light=%v", on)) },
			Standby:    func(on bool) { got[uid] = append(got[uid], fmt.Sprintf("standby=%v", on)) },
		})
	}

	conn.handleCommand("home/nanit/babies/baby1/night_light/switch", []byte("true"))
	conn.handleCommand("home/nanit/babies/baby2/standby/switch", []byte("false"))
	conn.handleCommand("home/nanit/babies/baby3/standby/switch", []byte("true")) // not connected
	conn.handleCommand("home/nanit/babies/baby1/volume/switch", []byte("true"))  // not a control
	conn.handleCommand("other/babies/baby1/night_light/switch", []byte("true"))  // wrong prefix
	conn.handleCommand("home/nanit/babies/BAD!/night_light/switch", []byte("true"))

	assert.Equal(t, map[string][]string{
		"baby1": {"light=true"},
		"baby2": {"standby=false"},
	}, got)
}

// A camera that reconnects registers again before its old connection has
// finished closing. The old one's unregister mustn't remove the new one.
func TestStaleUnregisterKeepsTheNewConnection(t *testing.T) {
	conn := NewConnection(Opts{TopicPrefix: "nanit"})
	calls := 0
	old := conn.RegisterBaby("baby1", CommandHandlers{NightLight: func(bool) { t.Error("old connection used") }})
	conn.RegisterBaby("baby1", CommandHandlers{NightLight: func(bool) { calls++ }})
	old()

	conn.handleCommand("nanit/babies/baby1/night_light/switch", []byte("true"))
	assert.Equal(t, 1, calls)
}
