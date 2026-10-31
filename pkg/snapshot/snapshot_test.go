package snapshot

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testStream = relay.Stream{H264: &relay.H264{SPS: []byte{0x67, 1}, PPS: []byte{0x68, 2}}}

// fakeFFmpeg - a stand-in that saves what it's given and answers "jpeg"
func fakeFFmpeg(t *testing.T) (bin, input string) {
	dir := t.TempDir()
	input = filepath.Join(dir, "input")
	bin = filepath.Join(dir, "ffmpeg")
	script := "#!/bin/sh\ncat > '" + input + "'\nprintf jpeg\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin, input
}

// publishKeyframes sends frames until stop is closed, a keyframe every
// third video frame
func publishKeyframes(p *relay.Publisher, stop <-chan struct{}) {
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		case <-time.After(5 * time.Millisecond):
		}
		key := i%3 == 0
		nalu := []byte{0x41, byte(i)}
		if key {
			nalu = []byte{0x65, byte(i)}
		}
		p.Broadcast(relay.Frame{Video: [][]byte{nalu}, Keyframe: key, PTS: time.Duration(i) * 100 * time.Millisecond})
		p.Broadcast(relay.Frame{Audio: []byte{1}})
	}
}

func TestTakesTheNextKeyframeAndReusesIt(t *testing.T) {
	hub := relay.NewHub(nil)
	p := hub.Publish("baby1", testStream)
	stop := make(chan struct{})
	defer close(stop)
	go publishKeyframes(p, stop)

	s := New(hub)
	bin, input := fakeFFmpeg(t)
	s.ffmpegBin = bin
	now := time.Now()
	s.now = func() time.Time { return now }

	pic, err := s.Get(context.Background(), "baby1")
	require.NoError(t, err)
	assert.Equal(t, "jpeg", string(pic.JPEG))

	// ffmpeg got the parameter sets, then a keyframe, in Annex B
	raw, err := os.ReadFile(input)
	require.NoError(t, err)
	var au h264.AnnexB
	require.NoError(t, au.Unmarshal(raw))
	require.Len(t, au, 3)
	assert.Equal(t, testStream.H264.SPS, au[0])
	assert.Equal(t, testStream.H264.PPS, au[1])
	assert.Equal(t, h264.NALUTypeIDR, h264.NALUType(au[2][0]&0x1f))

	// Within maxAge it's reused; ffmpeg isn't run again
	require.NoError(t, os.Remove(input))
	now = now.Add(maxAge - time.Second)
	again, err := s.Get(context.Background(), "baby1")
	require.NoError(t, err)
	assert.Equal(t, pic.Taken, again.Taken)
	assert.NoFileExists(t, input)

	// After it, a new one is taken
	now = now.Add(2 * time.Second)
	_, err = s.Get(context.Background(), "baby1")
	require.NoError(t, err)
	assert.FileExists(t, input)
}

func TestNoStream(t *testing.T) {
	s := New(relay.NewHub(nil))
	_, err := s.Get(context.Background(), "baby1")
	assert.ErrorIs(t, err, ErrNoStream)

	// Audio only: nothing to picture
	hub := relay.NewHub(nil)
	hub.Publish("baby1", relay.Stream{})
	_, err = New(hub).Get(context.Background(), "baby1")
	assert.ErrorIs(t, err, ErrNoStream)
}

func TestStreamEndingWhileWaiting(t *testing.T) {
	hub := relay.NewHub(nil)
	p := hub.Publish("baby1", testStream)
	go func() {
		time.Sleep(20 * time.Millisecond)
		p.Close()
	}()
	_, err := New(hub).Get(context.Background(), "baby1")
	assert.ErrorIs(t, err, ErrNoStream)
}

// With the real ffmpeg: a keyframe it encoded comes back as a JPEG
func TestRealFFmpegDecodesAKeyframe(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=1", "-frames:v", "1",
		"-c:v", "libx264", "-f", "h264", "pipe:1").Output()
	if err != nil {
		t.Skip("ffmpeg can't encode H.264 here: ", err)
	}
	var au h264.AnnexB
	require.NoError(t, au.Unmarshal(out))
	stream := relay.Stream{H264: &relay.H264{}}
	var frame [][]byte
	for _, nalu := range au {
		switch h264.NALUType(nalu[0] & 0x1f) {
		case h264.NALUTypeSPS:
			stream.H264.SPS = nalu
		case h264.NALUTypePPS:
			stream.H264.PPS = nalu
		default:
			frame = append(frame, nalu)
		}
	}

	hub := relay.NewHub(nil)
	p := hub.Publish("baby1", stream)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
				p.Broadcast(relay.Frame{Video: frame, Keyframe: true})
			}
		}
	}()

	pic, err := New(hub).Get(context.Background(), "baby1")
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(pic.JPEG, []byte{0xff, 0xd8}), "a JPEG starts with SOI")
}
