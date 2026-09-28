package rtmpserver

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1920x1080 baseline, from gortmplib's own tests
var (
	testSPS = []byte{
		0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
		0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
		0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9, 0x20,
	}
	testPPS       = []byte{0x08, 0x06, 0x07, 0x08}
	testAACConfig = &mpeg4audio.AudioSpecificConfig{
		Type:          mpeg4audio.ObjectTypeAACLC,
		SampleRate:    48000,
		ChannelConfig: 1,
		ChannelCount:  1, //nolint:staticcheck
	}

	testIDR    = [][]byte{{0x65, 0x88, 0x84, 0x00, 0x33}} // NAL type 5
	testNonIDR = [][]byte{{0x41, 0x9a, 0x02, 0x03}}       // NAL type 1
	testAAC    = []byte{0x21, 0x10, 0x04, 0x60}
)

const testBabyUID = "baby1"

func startTestServer(t *testing.T) (addr string, states *baby.StateManager) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { lis.Close() })

	states = baby.NewStateManager()
	go newRtmpHandler(states).serve(lis)
	return lis.Addr().String(), states
}

func tryDial(t *testing.T, addr, path string, publish bool) (*gortmplib.Client, error) {
	u, err := url.Parse("rtmp://" + addr + path)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := &gortmplib.Client{URL: u, Publish: publish}
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	t.Cleanup(c.Close)
	return c, nil
}

func dial(t *testing.T, addr, path string, publish bool) *gortmplib.Client {
	c, err := tryDial(t, addr, path, publish)
	require.NoError(t, err)
	return c
}

// startPlayer - connects a player and reads the stream description. The
// returned reader has a callback for every track, as gortmplib requires.
func startPlayer(t *testing.T, addr string) (*gortmplib.Reader, error) {
	player, err := tryDial(t, addr, "/local/"+testBabyUID, false)
	if err != nil {
		return nil, err
	}
	player.NetConn().SetDeadline(time.Now().Add(5 * time.Second))

	r := &gortmplib.Reader{Conn: player}
	if err := r.Initialize(); err != nil {
		return nil, err
	}
	for _, track := range r.Tracks() {
		switch track.Codec.(type) {
		case *codecs.H264:
			r.OnDataH264(track, func(time.Duration, time.Duration, [][]byte) {})
		case *codecs.MPEG4Audio:
			r.OnDataMPEG4Audio(track, func(time.Duration, []byte) {})
		}
	}
	return r, nil
}

// fakeCam publishes H264 and AAC like the cam does, a keyframe every 10
// frames, until stopped or the connection fails
func fakeCam(t *testing.T, addr string) (stop func()) {
	c := dial(t, addr, "/local/"+testBabyUID, true)

	video := &gortmplib.Track{Codec: &codecs.H264{SPS: testSPS, PPS: testPPS}}
	audio := &gortmplib.Track{Codec: &codecs.MPEG4Audio{Config: testAACConfig}}
	w := &gortmplib.Writer{Conn: c, Tracks: []*gortmplib.Track{video, audio}}
	require.NoError(t, w.Initialize())

	stopC := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stopC:
				return
			case <-time.After(10 * time.Millisecond):
			}

			pts := time.Duration(i) * 33 * time.Millisecond
			au := testNonIDR
			if i%10 == 0 {
				au = testIDR
			}
			if w.WriteH264(video, pts, pts, au) != nil {
				return
			}
			if w.WriteMPEG4Audio(audio, pts, testAAC) != nil {
				return
			}
		}
	}()

	return func() {
		close(stopC)
		<-done
		c.Close()
	}
}

func waitForStreamState(t *testing.T, states *baby.StateManager, want baby.StreamState) {
	t.Helper()
	require.Eventually(t, func() bool {
		return states.GetBabyState(testBabyUID).GetStreamState() == want
	}, 5*time.Second, 10*time.Millisecond)
}

// A player (ffmpeg for HLS, VLC, Home Assistant) gets the cam's tracks and
// then its frames, starting at a keyframe
func TestRelaysStreamToSubscriber(t *testing.T) {
	addr, states := startTestServer(t)
	stop := fakeCam(t, addr)
	defer stop()
	waitForStreamState(t, states, baby.StreamState_Alive)

	r, err := startPlayer(t, addr)
	require.NoError(t, err)

	var video *gortmplib.Track
	var gotAudio bool
	for _, track := range r.Tracks() {
		switch codec := track.Codec.(type) {
		case *codecs.H264:
			video = track
			assert.Equal(t, testSPS, codec.SPS)
			assert.Equal(t, testPPS, codec.PPS)
		case *codecs.MPEG4Audio:
			r.OnDataMPEG4Audio(track, func(time.Duration, []byte) { gotAudio = true })
		}
	}
	require.NotNil(t, video, "the H264 track is relayed")

	// gortmplib reports the stream's SPS and PPS as a frame of their own; only
	// frames carrying picture data count here
	var aus [][][]byte
	r.OnDataH264(video, func(_ time.Duration, _ time.Duration, au [][]byte) {
		for _, nalu := range au {
			if typ := h264.NALUType(nalu[0] & 0x1f); typ != h264.NALUTypeSPS && typ != h264.NALUTypePPS {
				aus = append(aus, au)
				return
			}
		}
	})
	for len(aus) < 12 || !gotAudio {
		require.NoError(t, r.Read())
	}

	assert.True(t, h264.IsRandomAccess(aus[0]), "the first frame is a keyframe")
	assert.True(t, containsNALU(aus, testIDR[0]))
	assert.True(t, containsNALU(aus, testNonIDR[0]))
	assert.NotNil(t, states.GetBabyState(testBabyUID).GetLastVideoPacketTime())
}

// The server closes these during the RTMP exchange, so either the connect or
// reading the stream description fails
func TestSubscriberWithoutPublisherIsClosed(t *testing.T) {
	addr, _ := startTestServer(t)

	_, err := startPlayer(t, addr)
	assert.Error(t, err)
}

func TestInvalidPathIsClosed(t *testing.T) {
	addr, _ := startTestServer(t)

	player, err := tryDial(t, addr, "/elsewhere/"+testBabyUID, false)
	if err == nil {
		player.NetConn().SetDeadline(time.Now().Add(5 * time.Second))
		err = (&gortmplib.Reader{Conn: player}).Initialize()
	}
	assert.Error(t, err)
}

// When the cam stops publishing, its players are closed and the stream is
// reported unhealthy so the app asks the cam to publish again
func TestPublisherDisconnect(t *testing.T) {
	addr, states := startTestServer(t)
	stop := fakeCam(t, addr)
	waitForStreamState(t, states, baby.StreamState_Alive)

	r, err := startPlayer(t, addr)
	require.NoError(t, err)

	stop()

	waitForStreamState(t, states, baby.StreamState_Unhealthy)
	for {
		if err := r.Read(); err != nil {
			break
		}
	}
}

// The cam opening a second publisher connection replaces the first, whose
// players are closed so they reconnect to the new stream. The stream stays
// alive throughout.
func TestSecondPublisherReplacesFirst(t *testing.T) {
	addr, states := startTestServer(t)
	stopFirst := fakeCam(t, addr)
	waitForStreamState(t, states, baby.StreamState_Alive)

	r, err := startPlayer(t, addr)
	require.NoError(t, err)

	stopSecond := fakeCam(t, addr)
	defer stopSecond()

	for {
		if err := r.Read(); err != nil {
			break
		}
	}

	stopFirst()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, baby.StreamState_Alive, states.GetBabyState(testBabyUID).GetStreamState(),
		"the replaced publisher going away leaves the new stream alive")
}

func containsNALU(aus [][][]byte, want []byte) bool {
	for _, au := range aus {
		for _, nalu := range au {
			if string(nalu) == string(want) {
				return true
			}
		}
	}
	return false
}
