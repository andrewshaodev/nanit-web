package talk

import (
	"bytes"
	"encoding/binary"
	"math"
	"os/exec"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURL(t *testing.T) {
	assert.Equal(t, "rtmp://monitor.example.com:1935/2wa/0123456789abcdef", URL("monitor.example.com:1935", "0123456789abcdef"))
}

// Each ADTS frame becomes one AAC frame, timed by its place in the stream
func TestPublishADTS(t *testing.T) {
	aus := [][]byte{{1, 2, 3}, {4, 5}, {6}}
	var adts []byte
	for _, au := range aus {
		b, err := mpeg4audio.ADTSPackets{{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 44100, ChannelConfig: 1, AU: au}}.Marshal()
		require.NoError(t, err)
		adts = append(adts, b...)
	}

	hub := relay.NewHub(nil)
	pub := hub.Publish("s", relay.Stream{})
	_, sub, ok := hub.Subscribe("s")
	require.True(t, ok)

	assert.ErrorContains(t, publishADTS(bytes.NewReader(adts), pub), "EOF")
	for i, au := range aus {
		f := <-sub.Frames
		assert.Equal(t, au, f.Audio)
		assert.Equal(t, time.Duration(i)*1024*time.Second/44100, f.PTS)
	}
}

func TestGarbageIsRefused(t *testing.T) {
	hub := relay.NewHub(nil)
	err := publishADTS(bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8}), hub.Publish("s", relay.Stream{}))
	assert.ErrorContains(t, err, "sync")
}

func TestSampleRateIsChecked(t *testing.T) {
	_, err := New().Start(100)
	assert.Error(t, err)
}

// With ffmpeg installed: a second of a tone comes out as about a second of
// AAC, published under the session ID
func TestSessionEncodes(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg isn't installed")
	}
	s := New()
	sess, err := s.Start(48000)
	require.NoError(t, err)
	assert.Regexp(t, SIDRX, sess.SID)

	stream, sub, ok := s.Hub.Subscribe(sess.SID)
	require.True(t, ok)
	assert.Equal(t, 44100, stream.AAC.SampleRate)

	pcm := make([]byte, 48000*2)
	for i := 0; i < 48000; i++ {
		v := int16(8000 * math.Sin(2*math.Pi*440*float64(i)/48000))
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	require.NoError(t, sess.Write(pcm))
	sess.Close()

	frames := 0
	for range sub.Frames {
		frames++
	}
	// 44100 / 1024 = 43 a second
	assert.InDelta(t, 43, frames, 3)
	_, _, ok = s.Hub.Subscribe(sess.SID)
	assert.False(t, ok, "a closed session is no longer published")
}
