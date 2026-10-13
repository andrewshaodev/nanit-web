package rtmpserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With the stream password on, players need ?key=, and the camera doesn't
func TestStreamPassword(t *testing.T) {
	auth := streamauth.Open(filepath.Join(t.TempDir(), "stream_auth.json"))
	settings, err := auth.SetEnabled(true)
	require.NoError(t, err)
	addr, states := startTestServerWithAuth(t, auth)

	fakeCam(t, addr)
	waitForStreamState(t, states, baby.StreamState_Alive)

	_, err = startPlayer(t, addr)
	assert.Error(t, err, "no key")
	_, err = startPlayerAt(t, addr, "/local/"+testBabyUID+"?key=wrong")
	assert.Error(t, err, "wrong key")

	r, err := startPlayerAt(t, addr, "/local/"+testBabyUID+"?key="+settings.Key)
	require.NoError(t, err)
	require.NoError(t, r.Read(), "the right key plays")

	// A new key disconnects the player
	_, err = auth.Regenerate()
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return r.Read() != nil }, 5*time.Second, 10*time.Millisecond)
}
