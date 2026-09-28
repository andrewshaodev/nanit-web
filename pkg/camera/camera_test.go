package camera

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn stands in for a camera's websocket: it records each request and
// answers with 200, or with the canned response for its type
type fakeConn struct {
	mu        sync.Mutex
	requests  []*client.Request
	responses map[client.RequestType]*client.Response
	errs      map[client.RequestType]error
}

func (f *fakeConn) SendRequest(reqType client.RequestType, req *client.Request) func(time.Duration) (*client.Response, error) {
	f.mu.Lock()
	req.Type = reqType.Enum()
	f.requests = append(f.requests, req)
	res, ok := f.responses[reqType]
	err := f.errs[reqType]
	f.mu.Unlock()
	if !ok {
		res = &client.Response{StatusCode: utils.ConstRefInt32(200)}
	}
	return func(time.Duration) (*client.Response, error) { return res, err }
}

func (f *fakeConn) sent() []*client.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*client.Request(nil), f.requests...)
}

func newTestCamera(t *testing.T) (*Camera, *fakeConn) {
	t.Helper()
	cam := New(baby.Baby{UID: "baby1"}, Options{}, Deps{State: baby.NewStateManager()})
	conn := &fakeConn{responses: map[client.RequestType]*client.Response{}, errs: map[client.RequestType]error{}}
	cam.setConnection(conn)
	return cam, conn
}

func TestCommandsNeedAConnection(t *testing.T) {
	cam := New(baby.Baby{UID: "baby1"}, Options{}, Deps{State: baby.NewStateManager()})

	assert.ErrorIs(t, cam.SetNightLight(true), ErrNotConnected)
	assert.ErrorIs(t, cam.SetStandby(true), ErrNotConnected)
	_, err := cam.SoundStatus()
	assert.ErrorIs(t, err, ErrNotConnected)
	_, err = cam.PlaySound("White Noise.wav", 0)
	assert.ErrorIs(t, err, ErrNotConnected)
	_, err = cam.SetVolume(40)
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestToggleFlipsTheKnownState(t *testing.T) {
	cam, conn := newTestCamera(t)
	cam.deps.State.Update("baby1", *baby.NewState().SetNightLight(true).SetStandby(false))

	on, err := cam.ToggleNightLight()
	require.NoError(t, err)
	assert.False(t, on)
	on, err = cam.ToggleStandby()
	require.NoError(t, err)
	assert.True(t, on)

	sent := conn.sent()
	require.Len(t, sent, 2)
	assert.Equal(t, client.RequestType_PUT_CONTROL, sent[0].GetType())
	assert.Equal(t, client.Control_LIGHT_OFF, sent[0].GetControl().GetNightLight())
	assert.Equal(t, client.RequestType_PUT_SETTINGS, sent[1].GetType())
	assert.True(t, sent[1].GetSettings().GetSleepMode())
}

// What a sound command puts on the wire, checked against the fake: nothing
// here reaches a camera
func TestSoundCommands(t *testing.T) {
	cam, conn := newTestCamera(t)
	conn.responses[client.RequestType_GET_SOUNDTRACKS] = &client.Response{
		StatusCode:  utils.ConstRefInt32(200),
		Soundtracks: []*client.Soundtrack{{Name: utils.ConstRefStr("White Noise.wav")}, {Name: utils.ConstRefStr("Birds.wav")}},
	}
	conn.responses[client.RequestType_GET_PLAYBACK] = &client.Response{
		StatusCode: utils.ConstRefInt32(200),
		Playback: &client.Playback{
			Status:             client.Playback_STARTED.Enum(),
			SelectedSoundtrack: &client.Soundtrack{Name: utils.ConstRefStr("Birds.wav")},
		},
	}
	conn.responses[client.RequestType_GET_SETTINGS] = &client.Response{
		StatusCode: utils.ConstRefInt32(200),
		Settings:   &client.Settings{Volume: utils.ConstRefInt32(35)},
	}

	status, err := cam.PlaySound("Birds.wav", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"White Noise.wav", "Birds.wav"}, status.Tracks)
	assert.Equal(t, &SoundPlayback{Playing: true, Track: "Birds.wav"}, status.Playback)
	assert.Equal(t, int32(35), *status.Volume)
	assert.Empty(t, status.Errors)

	play := conn.sent()[0]
	assert.Equal(t, client.RequestType_PUT_PLAYBACK, play.GetType())
	assert.Equal(t, client.Playback_STARTED, play.GetPlayback().GetStatus())
	assert.Equal(t, int32(-1), play.GetPlayback().GetDuration(), "no duration keeps playing")
	assert.Equal(t, "Birds.wav", play.GetPlayback().GetSoundtrack().GetName())
	assert.Equal(t, "Birds.wav", play.GetPlayback().GetSelectedSoundtrack().GetName())

	_, err = cam.SetVolume(250)
	require.NoError(t, err)
	var volume *client.Request
	for _, req := range conn.sent() {
		if req.GetType() == client.RequestType_PUT_SETTINGS {
			volume = req
		}
	}
	require.NotNil(t, volume)
	assert.Equal(t, int32(100), volume.GetSettings().GetVolume(), "clamped to 100")

	_, err = cam.StopSound()
	require.NoError(t, err)
	var stop *client.Request
	for _, req := range conn.sent() {
		if req.GetType() == client.RequestType_PUT_PLAYBACK {
			stop = req
		}
	}
	assert.Equal(t, client.Playback_STOPPED, stop.GetPlayback().GetStatus())
}

// A failed read leaves the rest of the status intact, and says what failed
func TestSoundStatusIsPartialOnError(t *testing.T) {
	cam, conn := newTestCamera(t)
	conn.responses[client.RequestType_GET_PLAYBACK] = &client.Response{
		StatusCode: utils.ConstRefInt32(500), StatusMessage: utils.ConstRefStr("busy"),
	}

	status, err := cam.SoundStatus()
	require.NoError(t, err)
	assert.Nil(t, status.Playback)
	require.Len(t, status.Errors, 1)
	assert.Contains(t, status.Errors[0], "playback")
}

// Syncing the same babies again changes nothing, and a baby that's gone
// has its camera stopped
func TestRegistrySync(t *testing.T) {
	registry := NewRegistry(Options{}, Deps{State: baby.NewStateManager()})
	babies := []baby.Baby{{UID: "baby1"}, {UID: "baby2"}}

	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		registry.Sync(ctx, babies)
		first, _ := registry.Get("baby1")
		registry.Sync(ctx, babies)
		again, _ := registry.Get("baby1")
		assert.Same(t, first, again, "a second sync must not start another camera")

		registry.Sync(ctx, babies[1:])
		_, running := registry.Get("baby1")
		assert.False(t, running)
		_, running = registry.Get("baby2")
		assert.True(t, running)

		registry.StopAll()
		_, running = registry.Get("baby2")
		assert.False(t, running)
		ctx.Fail(errors.New("done"))
	})

	done := make(chan struct{})
	go func() { _, _ = runner.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cameras didn't stop")
	}
}
