package mse

import (
	"bytes"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
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
	AAC: &mpeg4audio.AudioSpecificConfig{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 44100, ChannelConfig: 1, ChannelCount: 1}, //nolint:staticcheck
}

func videoFrame(i int, start time.Duration) relay.Frame {
	t := start + time.Duration(i)*100*time.Millisecond
	if i%10 == 0 {
		return relay.Frame{PTS: t, DTS: t, Video: [][]byte{{0x65, 0x88, 0x84, 0x00, 0x33}}, Keyframe: true}
	}
	return relay.Frame{PTS: t, DTS: t, Video: [][]byte{{0x41, 0x9a, 0x02, 0x03}}}
}

func parsePart(t *testing.T, b []byte) *fmp4.Part {
	t.Helper()
	var parts fmp4.Parts
	require.NoError(t, parts.Unmarshal(b))
	require.Len(t, parts, 1)
	return parts[0]
}

func TestMIME(t *testing.T) {
	m, err := New(testStream)
	require.NoError(t, err)
	assert.Equal(t, `video/mp4; codecs="avc1.42c028,mp4a.40.2"`, m.MIME())

	m, err = New(relay.Stream{H264: testStream.H264})
	require.NoError(t, err)
	assert.Equal(t, `video/mp4; codecs="avc1.42c028"`, m.MIME(), "no audio codec without audio")
}

func TestNeedsVideo(t *testing.T) {
	_, err := New(relay.Stream{AAC: testStream.AAC})
	assert.Error(t, err)
}

// The init segment parses back with both tracks and their codecs
func TestInit(t *testing.T) {
	m, err := New(testStream)
	require.NoError(t, err)
	b, err := m.Init()
	require.NoError(t, err)

	var init fmp4.Init
	require.NoError(t, init.Unmarshal(bytes.NewReader(b)))
	require.Len(t, init.Tracks, 2)
	assert.Equal(t, uint32(90000), init.Tracks[0].TimeScale)
	h264, ok := init.Tracks[0].Codec.(*codecs.H264)
	require.True(t, ok)
	assert.Equal(t, testStream.H264.SPS, h264.SPS)
	assert.Equal(t, uint32(44100), init.Tracks[1].TimeScale)
	_, ok = init.Tracks[1].Codec.(*codecs.MPEG4Audio)
	assert.True(t, ok)
}

// Each fragment holds the previous video frame with its duration, and the
// audio since. Time starts at zero from the first keyframe, and the base
// times run on without gaps.
func TestFragments(t *testing.T) {
	m, err := New(testStream)
	require.NoError(t, err)

	// The camera's clock doesn't start at zero
	start := 37 * time.Second
	// Frames before the first keyframe are dropped
	frag, err := m.Write(videoFrame(5, start-time.Second))
	require.NoError(t, err)
	assert.Nil(t, frag)
	frag, err = m.Write(relay.Frame{PTS: start - 50*time.Millisecond, DTS: start - 50*time.Millisecond, Audio: []byte{1}})
	require.NoError(t, err)
	assert.Nil(t, frag)

	var parts []*fmp4.Part
	audioPerFrame := 4
	for i := 0; i < 21; i++ {
		frag, err := m.Write(videoFrame(i, start))
		require.NoError(t, err)
		if i == 0 {
			assert.Nil(t, frag, "the first frame waits for its duration")
		} else {
			require.NotNil(t, frag)
			parts = append(parts, parsePart(t, frag))
		}
		for a := 0; a < audioPerFrame; a++ {
			at := start + time.Duration(i)*100*time.Millisecond + time.Duration(a)*time.Second*aacFrameSamples/44100
			_, err := m.Write(relay.Frame{PTS: at, DTS: at, Audio: []byte{0x21, byte(a)}})
			require.NoError(t, err)
		}
	}

	require.Len(t, parts, 20)
	var wantVideo uint64
	for i, p := range parts {
		assert.Equal(t, uint32(i+1), p.SequenceNumber)
		require.Len(t, p.Tracks, 2)

		video := p.Tracks[0]
		assert.Equal(t, videoTrackID, video.ID)
		assert.Equal(t, wantVideo, video.BaseTime)
		require.Len(t, video.Samples, 1)
		assert.Equal(t, uint32(9000), video.Samples[0].Duration)
		assert.Equal(t, i%10 != 0, video.Samples[0].IsNonSyncSample, "keyframes are sync samples")
		wantVideo += uint64(video.Samples[0].Duration)

		audio := p.Tracks[1]
		assert.Equal(t, audioTrackID, audio.ID)
		assert.Len(t, audio.Samples, audioPerFrame)
		// The audio received during frame i, timed from the first keyframe
		assert.InDelta(t, float64(i)*4410, float64(audio.BaseTime), 1)
	}
}

// A repeated or backwards timestamp doesn't make a fragment, and doesn't
// break the ones after
func TestBackwardsVideoIsDropped(t *testing.T) {
	m, err := New(relay.Stream{H264: testStream.H264})
	require.NoError(t, err)
	_, _ = m.Write(videoFrame(0, 0))
	frag, _ := m.Write(videoFrame(1, 0))
	require.NotNil(t, frag)

	frag, err = m.Write(videoFrame(1, 0))
	require.NoError(t, err)
	assert.Nil(t, frag)

	frag, err = m.Write(videoFrame(2, 0))
	require.NoError(t, err)
	require.NotNil(t, frag)
	p := parsePart(t, frag)
	require.Len(t, p.Tracks, 1, "no audio track without audio")
	assert.Equal(t, uint64(9000), p.Tracks[0].BaseTime)
	assert.Equal(t, uint32(9000), p.Tracks[0].Samples[0].Duration)
}

// Time stays exact on a long-running stream
func TestLongStream(t *testing.T) {
	m, err := New(relay.Stream{H264: testStream.H264})
	require.NoError(t, err)
	_, _ = m.Write(videoFrame(0, 0))
	week := 7 * 24 * time.Hour
	_, _ = m.Write(relay.Frame{PTS: week, DTS: week, Video: [][]byte{{0x41}}})
	frag, err := m.Write(relay.Frame{PTS: week + 100*time.Millisecond, DTS: week + 100*time.Millisecond, Video: [][]byte{{0x41}}})
	require.NoError(t, err)
	p := parsePart(t, frag)
	assert.Equal(t, uint64(week/time.Second)*90000, p.Tracks[0].BaseTime)
}
