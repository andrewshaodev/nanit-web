package rtspserver

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1920x1080 baseline, from gortmplib's own tests
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

func freeAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
	return lis.Addr().String()
}

// startServer runs the RTSP server on a hub, until the test ends
func startServer(t *testing.T, hub *relay.Hub) string {
	t.Helper()
	addr := freeAddr(t)
	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		_ = Serve(ctx, addr, hub)
	})
	t.Cleanup(runner.Cancel)
	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
		}
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	return addr
}

// publish feeds the hub like the camera does, a keyframe every 10 frames,
// until the test ends
func publish(t *testing.T, hub *relay.Hub) *relay.Publisher {
	t.Helper()
	p := hub.Publish("baby1", testStream)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
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
	t.Cleanup(func() { close(stop); <-done })
	return p
}

// play connects an RTSP player over TCP, and counts the packets per track
func play(t *testing.T, addr string) (desc *description.Session, video, audio *atomic.Int64, err error) {
	t.Helper()
	u, err := base.ParseURL("rtsp://" + addr + "/local/baby1")
	require.NoError(t, err)
	tcp := gortsplib.ProtocolTCP
	c := &gortsplib.Client{Scheme: u.Scheme, Host: u.Host, Protocol: &tcp}
	require.NoError(t, c.Start())
	t.Cleanup(c.Close)

	desc, _, err = c.Describe(u)
	if err != nil {
		return nil, nil, nil, err
	}
	require.NoError(t, c.SetupAll(desc.BaseURL, desc.Medias))
	video, audio = &atomic.Int64{}, &atomic.Int64{}
	c.OnPacketRTPAny(func(m *description.Media, _ format.Format, _ *rtp.Packet) {
		if m.Type == description.MediaTypeVideo {
			video.Add(1)
		} else {
			audio.Add(1)
		}
	})
	_, err = c.Play(nil)
	require.NoError(t, err)
	return desc, video, audio, nil
}

// A player gets the camera's H.264 and AAC, as published to the hub
func TestServesTheCamerasStream(t *testing.T) {
	hub := relay.NewHub(baby.NewStateManager())
	addr := startServer(t, hub)
	publish(t, hub)

	desc, video, audio, err := play(t, addr)
	require.NoError(t, err)
	require.Len(t, desc.Medias, 2)
	h264, ok := desc.Medias[0].Formats[0].(*format.H264)
	require.True(t, ok)
	assert.Equal(t, testStream.H264.SPS, h264.SPS)
	_, ok = desc.Medias[1].Formats[0].(*format.MPEG4Audio)
	assert.True(t, ok)

	assert.Eventually(t, func() bool { return video.Load() > 5 && audio.Load() > 5 }, 5*time.Second, 20*time.Millisecond)
}

// Several players share one stream
func TestSeveralPlayers(t *testing.T) {
	hub := relay.NewHub(baby.NewStateManager())
	addr := startServer(t, hub)
	publish(t, hub)

	_, first, _, err := play(t, addr)
	require.NoError(t, err)
	_, second, _, err := play(t, addr)
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return first.Load() > 5 && second.Load() > 5 }, 5*time.Second, 20*time.Millisecond)
}

// Nothing is served while the camera isn't publishing
func TestNoStreamWithoutPublisher(t *testing.T) {
	hub := relay.NewHub(baby.NewStateManager())
	addr := startServer(t, hub)

	_, _, _, err := play(t, addr)
	assert.ErrorContains(t, err, "404")
}

// When the camera's stream ends, players are disconnected, and a new stream
// is served once it publishes again
func TestStreamFollowsThePublisher(t *testing.T) {
	hub := relay.NewHub(baby.NewStateManager())
	addr := startServer(t, hub)
	first := publish(t, hub)

	_, video, _, err := play(t, addr)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return video.Load() > 3 }, 5*time.Second, 20*time.Millisecond)

	first.Close()
	require.Eventually(t, func() bool {
		_, _, _, err := play(t, addr)
		return err != nil
	}, 5*time.Second, 50*time.Millisecond, "nothing served once the stream ends")

	publish(t, hub)
	_, video, _, err = play(t, addr)
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return video.Load() > 3 }, 5*time.Second, 20*time.Millisecond)
}
