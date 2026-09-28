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

// fakeTranscoder records what the stream loop does with HLS
type fakeTranscoder struct {
	mu      sync.Mutex
	running bool
	starts  int
	stops   int
}

func (f *fakeTranscoder) StartTranscoding(string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = true
	f.starts++
	return nil
}

func (f *fakeTranscoder) StopTranscoding(string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	f.stops++
}

func (f *fakeTranscoder) IsTranscoding(string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *fakeTranscoder) counts() (starts, stops int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.stops
}

type streamTest struct {
	cam   *Camera
	conn  *fakeConn
	hls   *fakeTranscoder
	state *baby.StateManager
	stop  func()
}

// startStreamLoop runs a camera's stream loop, with retries in milliseconds
func startStreamLoop(t *testing.T, autoStart bool) *streamTest {
	t.Helper()
	st := &streamTest{
		conn:  &fakeConn{responses: map[client.RequestType]*client.Response{}, errs: map[client.RequestType]error{}},
		hls:   &fakeTranscoder{},
		state: baby.NewStateManager(),
	}
	st.cam = New(baby.Baby{UID: "baby1"}, Options{RTMP: &RTMPOptions{PublicAddr: "192.0.2.1:1935", AutoStart: autoStart}},
		Deps{State: st.state, HLS: st.hls})
	st.cam.streamRetryMin = 20 * time.Millisecond
	st.cam.streamRetryMax = 40 * time.Millisecond

	runner := utils.RunWithGracefulCancel(st.cam.runStream)
	st.stop = runner.Cancel
	t.Cleanup(runner.Cancel)
	return st
}

// streamRequests - the PUT_STREAMING requests sent so far, by status
func (st *streamTest) streamRequests() (started, stopped int) {
	for _, req := range st.conn.sent() {
		if req.GetType() != client.RequestType_PUT_STREAMING {
			continue
		}
		switch req.GetStreaming().GetStatus() {
		case client.Streaming_STARTED:
			started++
		case client.Streaming_STOPPED:
			stopped++
		}
	}
	return
}

func (st *streamTest) publish(alive bool) {
	streamState := baby.StreamState_Unhealthy
	if alive {
		streamState = baby.StreamState_Alive
	}
	st.state.Update("baby1", *baby.NewState().SetStreamState(streamState))
}

func (st *streamTest) connect() {
	st.state.Update("baby1", *baby.NewState().SetWebsocketAlive(true))
	st.cam.sendStreamEvent(connectionChanged{st.conn})
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 2*time.Second, 5*time.Millisecond, what)
}

// never - cond stays false for a while
func never(t *testing.T, what string, cond func() bool) {
	t.Helper()
	assert.Never(t, cond, 150*time.Millisecond, 10*time.Millisecond, what)
}

// The camera is asked once when it connects, HLS starts when it actually
// publishes (not a few seconds after asking, hoping), and a stream that
// stops is asked for again
func TestStreamIsRequestedAndTranscodedOnPublish(t *testing.T) {
	st := startStreamLoop(t, true)
	st.connect()
	eventually(t, "a stream request on connect", func() bool { n, _ := st.streamRequests(); return n == 1 })
	never(t, "HLS before the camera publishes", func() bool { s, _ := st.hls.counts(); return s > 0 })

	st.publish(true)
	eventually(t, "HLS once publishing", func() bool { s, _ := st.hls.counts(); return s == 1 })

	st.publish(false)
	eventually(t, "HLS stopped with the stream", func() bool { _, s := st.hls.counts(); return s == 1 })
	eventually(t, "the stream asked for again", func() bool { n, _ := st.streamRequests(); return n == 2 })
}

// Asking for a stream the camera already publishes makes it open a second
// publisher, which cuts off everyone watching the first
func TestPublishingCameraIsNotAskedAgain(t *testing.T) {
	st := startStreamLoop(t, true)
	st.publish(true)
	eventually(t, "HLS for the running stream", func() bool { s, _ := st.hls.counts(); return s == 1 })

	st.connect()
	never(t, "a request while already publishing", func() bool { n, _ := st.streamRequests(); return n > 0 })
}

// A refused request is retried, backing off, until the camera publishes
func TestRefusedRequestIsRetried(t *testing.T) {
	st := startStreamLoop(t, true)
	st.conn.mu.Lock()
	st.conn.errs[client.RequestType_PUT_STREAMING] = errors.New("Forbidden: Number of Mobile App connections above limit, declining connection")
	st.conn.mu.Unlock()

	st.connect()
	eventually(t, "retries after the refusal", func() bool { n, _ := st.streamRequests(); return n >= 3 })
	assert.Equal(t, baby.StreamRequestState_RequestFailed, st.state.GetBabyState("baby1").GetStreamRequestState())

	st.conn.mu.Lock()
	delete(st.conn.errs, client.RequestType_PUT_STREAMING)
	st.conn.mu.Unlock()
	st.publish(true)
	time.Sleep(50 * time.Millisecond) // any retry already under way finishes
	before, _ := st.streamRequests()
	never(t, "retries once publishing", func() bool { n, _ := st.streamRequests(); return n > before })
}

// With NANIT_RTMP_AUTO_START off, the camera is only asked when the API asks
func TestWithoutAutoStartOnlyTheAPIAsks(t *testing.T) {
	st := startStreamLoop(t, false)
	assert.ErrorIs(t, st.cam.RequestStream(), ErrNotConnected)

	st.connect()
	never(t, "a request on connect", func() bool { n, _ := st.streamRequests(); return n > 0 })

	require.NoError(t, st.cam.RequestStream())
	eventually(t, "the API's request", func() bool { n, _ := st.streamRequests(); return n == 1 })
	st.publish(true)
	eventually(t, "HLS once publishing", func() bool { s, _ := st.hls.counts(); return s == 1 })

	// And a stream that stops isn't asked for again
	st.publish(false)
	never(t, "a request after the stream stops", func() bool { n, _ := st.streamRequests(); return n > 1 })
}

// On shutdown the camera is told to stop, which ends its publishing. That
// mustn't be answered with a new request.
func TestStreamStoppedAfterDisconnectIsNotRequestedAgain(t *testing.T) {
	st := startStreamLoop(t, true)
	st.publish(true)
	st.connect()
	eventually(t, "HLS running", func() bool { s, _ := st.hls.counts(); return s == 1 })

	st.cam.sendStreamEvent(connectionChanged{nil})
	st.publish(false)
	never(t, "a request after disconnecting", func() bool { n, _ := st.streamRequests(); return n > 0 })
}
