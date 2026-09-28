package camera

import (
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
)

func streamState(websocketAlive bool, streamState baby.StreamState) *baby.State {
	return baby.NewState().SetWebsocketAlive(websocketAlive).SetStreamState(streamState)
}

// TestShouldRequestStream - re-requesting a stream the cam is already
// publishing makes it open a second publisher connection, and registering that
// one closes every subscriber of the first. That is what interrupted anything
// consuming the RTMP stream once per reconnect.
func TestShouldRequestStream(t *testing.T) {
	assert.False(t, shouldRequestStream(streamState(true, baby.StreamState_Alive)),
		"a stream the cam is already publishing must not be requested again")

	assert.True(t, shouldRequestStream(streamState(true, baby.StreamState_Unhealthy)))
	assert.True(t, shouldRequestStream(streamState(true, baby.StreamState_Unknown)))
}

func TestShouldReleaseStreamResources(t *testing.T) {
	// A reconnect: the websocket is gone but the cam publishes straight
	// through it, so the stream and its subscribers must be left alone.
	assert.False(t, shouldReleaseStreamResources(streamState(false, baby.StreamState_Alive)),
		"a reconnect must not tear down a stream the cam is still publishing")

	// Deliberate shutdown, socket still usable.
	assert.True(t, shouldReleaseStreamResources(streamState(true, baby.StreamState_Alive)))

	// The cam stopped publishing on its own, so there is nothing left to keep.
	assert.True(t, shouldReleaseStreamResources(streamState(false, baby.StreamState_Unhealthy)))
	assert.True(t, shouldReleaseStreamResources(streamState(false, baby.StreamState_Unknown)))
}
