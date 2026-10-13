package pull

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/rtspserver"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/andrewshaodev/nanit-web/pkg/streamsource"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testStream = relay.Stream{
	H264: &relay.H264{
		SPS: []byte{
			0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
			0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
			0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9, 0x20,
		},
		PPS: []byte{0x08, 0x06, 0x07, 0x08},
	},
	AAC: &mpeg4audio.AudioSpecificConfig{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 48000, ChannelConfig: 1, ChannelCount: 1}, //nolint:staticcheck
}

func run(t *testing.T, f func(utils.GracefulContext)) {
	t.Helper()
	runner := utils.RunWithGracefulCancel(f)
	t.Cleanup(runner.Cancel)
}

// startStreamServer - a server hub, with its RTSP output on a free port
func startStreamServer(t *testing.T) (*relay.Hub, string) {
	return startStreamServerWithAuth(t, nil)
}

func startStreamServerWithAuth(t *testing.T, auth *streamauth.Store) (*relay.Hub, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	lis.Close()

	hub := relay.NewHub(baby.NewStateManager())
	run(t, func(ctx utils.GracefulContext) { _ = rtspserver.Serve(ctx, addr, hub, auth) })
	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
		}
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	return hub, addr
}

// source - a viewer's stream server settings, pointing at addr
func source(t *testing.T, addr, key string) *streamsource.Store {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	p, err := strconv.Atoi(port)
	require.NoError(t, err)
	src := streamsource.Open(filepath.Join(t.TempDir(), "stream_source.json"))
	_, err = src.Set(streamsource.Source{Host: host, Port: p, Key: key})
	require.NoError(t, err)
	return src
}

// camera publishes into the server hub like the camera does, until stopped
func camera(t *testing.T, hub *relay.Hub) (stop func()) {
	t.Helper()
	p := hub.Publish("baby1", testStream)
	stopC, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stopC:
				return
			case <-time.After(10 * time.Millisecond):
			}
			pts := time.Duration(i) * 100 * time.Millisecond
			au, key := [][]byte{{0x41, 0x9a, 0x02, 0x03}}, false
			if i%10 == 0 {
				au, key = [][]byte{{0x65, 0x88, 0x84, 0x00, 0x33}}, true
			}
			p.Broadcast(relay.Frame{PTS: pts, DTS: pts, Video: au, Keyframe: key})
			p.Broadcast(relay.Frame{PTS: pts, DTS: pts, Audio: []byte{0x21, 0x10, 0x04, 0x60}})
		}
	}()
	var once bool
	stop = func() {
		if once {
			return
		}
		once = true
		close(stopC)
		<-done
		p.Close()
	}
	t.Cleanup(stop)
	return stop
}

// A viewer's hub carries the server's stream: the same parameter sets, and
// video (from a keyframe) and audio frames
func TestViewerReceivesTheServersStream(t *testing.T) {
	serverHub, addr := startStreamServer(t)
	camera(t, serverHub)

	viewerStates := baby.NewStateManager()
	viewerHub := relay.NewHub(viewerStates)
	run(t, func(ctx utils.GracefulContext) {
		Run(ctx, "baby1", source(t, addr, ""), viewerHub)
	})

	var stream relay.Stream
	var sub *relay.Subscription
	require.Eventually(t, func() bool {
		var ok bool
		stream, sub, ok = viewerHub.Subscribe("baby1")
		return ok
	}, 5*time.Second, 20*time.Millisecond, "the viewer should start publishing the server's stream")
	defer sub.Close()

	assert.Equal(t, testStream.H264.SPS, stream.H264.SPS)
	assert.Equal(t, testStream.H264.PPS, stream.H264.PPS)
	require.NotNil(t, stream.AAC)
	assert.Equal(t, 48000, stream.AAC.SampleRate)
	assert.Equal(t, baby.StreamState_Alive, viewerStates.GetBabyState("baby1").GetStreamState())

	var video, audio int
	first := true
	deadline := time.After(5 * time.Second)
	for video < 5 || audio < 5 {
		select {
		case f := <-sub.Frames:
			if f.Video != nil {
				if first {
					assert.True(t, f.Keyframe, "a subscriber starts at a keyframe")
					first = false
				}
				video++
			} else {
				audio++
			}
		case <-deadline:
			t.Fatalf("got %d video and %d audio frames", video, audio)
		}
	}
}

// When the server's stream ends the viewer's does too, and it comes back
// when the server's does
func TestViewerFollowsTheServer(t *testing.T) {
	serverHub, addr := startStreamServer(t)
	stop := camera(t, serverHub)

	viewerStates := baby.NewStateManager()
	viewerHub := relay.NewHub(viewerStates)
	run(t, func(ctx utils.GracefulContext) {
		Run(ctx, "baby1", source(t, addr, ""), viewerHub)
	})
	require.Eventually(t, func() bool { return viewerHub.Publishing("baby1") }, 5*time.Second, 20*time.Millisecond)

	stop()
	require.Eventually(t, func() bool { return !viewerHub.Publishing("baby1") }, 15*time.Second, 50*time.Millisecond)
	assert.Equal(t, baby.StreamState_Unhealthy, viewerStates.GetBabyState("baby1").GetStreamState())

	camera(t, serverHub)
	assert.Eventually(t, func() bool { return viewerHub.Publishing("baby1") }, 15*time.Second, 50*time.Millisecond)
}

// A viewer of a server with the stream password gives it in its address
func TestViewerSignsIn(t *testing.T) {
	auth := streamauth.Open(filepath.Join(t.TempDir(), "stream_auth.json"))
	settings, err := auth.SetEnabled(true)
	require.NoError(t, err)
	serverHub, addr := startStreamServerWithAuth(t, auth)
	camera(t, serverHub)

	viewerHub := relay.NewHub(baby.NewStateManager())
	run(t, func(ctx utils.GracefulContext) {
		Run(ctx, "baby1", source(t, addr, settings.Key), viewerHub)
	})
	assert.Eventually(t, func() bool { return viewerHub.Publishing("baby1") }, 5*time.Second, 20*time.Millisecond)
}

// A wrong key is reported in words, and saving the right one reconnects
// without waiting out the backoff
func TestStatusAndReconnectOnChange(t *testing.T) {
	auth := streamauth.Open(filepath.Join(t.TempDir(), "stream_auth.json"))
	settings, err := auth.SetEnabled(true)
	require.NoError(t, err)
	serverHub, addr := startStreamServerWithAuth(t, auth)
	camera(t, serverHub)

	src := source(t, addr, "wrong")
	viewerHub := relay.NewHub(baby.NewStateManager())
	run(t, func(ctx utils.GracefulContext) { Run(ctx, "baby1", src, viewerHub) })

	require.Eventually(t, func() bool {
		st, ok := src.Status("baby1")
		return ok && !st.Receiving && strings.Contains(st.Error, "key")
	}, 5*time.Second, 20*time.Millisecond, "a wrong key is explained")

	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	_, err = src.Set(streamsource.Source{Host: host, Port: p, Key: settings.Key})
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		st, ok := src.Status("baby1")
		return ok && st.Receiving && viewerHub.Publishing("baby1")
	}, 3*time.Second, 20*time.Millisecond, "the new key is used straight away")
}

// With no stream server set, a viewer waits, and starts once one is
func TestWaitsForASource(t *testing.T) {
	serverHub, addr := startStreamServer(t)
	camera(t, serverHub)

	src := streamsource.Open(filepath.Join(t.TempDir(), "stream_source.json"))
	viewerHub := relay.NewHub(baby.NewStateManager())
	run(t, func(ctx utils.GracefulContext) { Run(ctx, "baby1", src, viewerHub) })
	time.Sleep(200 * time.Millisecond)
	assert.False(t, viewerHub.Publishing("baby1"))

	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	_, err := src.Set(streamsource.Source{Host: host, Port: p})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return viewerHub.Publishing("baby1") }, 3*time.Second, 20*time.Millisecond)
}
