package mqtt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Broker URLs can carry credentials, which used to be logged on every connect
func TestRedactURLHidesPassword(t *testing.T) {
	assert.Equal(t, "mqtt://homeassistant:xxxxx@192.168.1.10:1883", redactURL("mqtt://homeassistant:s3cret@192.168.1.10:1883"))
	assert.Equal(t, "tcp://192.168.1.10:1883", redactURL("tcp://192.168.1.10:1883"))
	assert.NotContains(t, redactURL("mqtt://u:p%zz@host"), "p%zz")
}
