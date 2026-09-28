package rtmpserver

import (
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testVideoTrack = &gortmplib.Track{Codec: &codecs.H264{SPS: testSPS, PPS: testPPS}}
	testAudioTrack = &gortmplib.Track{Codec: &codecs.MPEG4Audio{Config: testAACConfig}}
)

func videoFrame(keyframe bool) frame {
	return frame{track: testVideoTrack, video: [][]byte{{0x65}}, keyframe: keyframe}
}

func audioFrame() frame {
	return frame{track: testAudioTrack, audio: []byte{0x01}}
}

func drain(sub *subscriber) []frame {
	var frames []frame
	for {
		select {
		case f, open := <-sub.frames:
			if !open {
				return frames
			}
			frames = append(frames, f)
		default:
			return frames
		}
	}
}

// Frames before the first keyframe cannot be decoded, so a subscriber joining
// mid-stream starts at the next one, audio included.
func TestSubscriberStartsAtKeyframe(t *testing.T) {
	b := newBroadcaster([]*gortmplib.Track{testVideoTrack, testAudioTrack})
	sub := b.subscribe()

	b.broadcast(videoFrame(false))
	b.broadcast(audioFrame())
	b.broadcast(videoFrame(true))
	b.broadcast(audioFrame())
	b.broadcast(videoFrame(false))

	got := drain(sub)
	require.Len(t, got, 3)
	assert.True(t, got[0].keyframe)
	assert.NotNil(t, got[1].audio)
	assert.False(t, got[2].keyframe)
}

func TestAudioOnlySubscriberStartsImmediately(t *testing.T) {
	b := newBroadcaster([]*gortmplib.Track{testAudioTrack})
	sub := b.subscribe()

	b.broadcast(audioFrame())

	assert.Len(t, drain(sub), 1)
}

// A subscriber that stops reading used to block the publisher, and with it
// every other subscriber. It is now dropped once its queue is full.
func TestStalledSubscriberDoesNotBlockOthers(t *testing.T) {
	b := newBroadcaster([]*gortmplib.Track{testVideoTrack})
	stalled := b.subscribe()
	healthy := b.subscribe()

	// The healthy subscriber keeps up exactly, taking each frame as it is
	// sent; the stalled one takes none
	received := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i <= subscriberQueueSize*4; i++ {
			b.broadcast(videoFrame(i == 0))
			if _, open := <-healthy.frames; open {
				received++
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("broadcast blocked on a stalled subscriber")
	}

	assert.Equal(t, subscriberQueueSize*4+1, received, "the healthy subscriber got every frame")
	assert.False(t, healthy.fellBehind.Load())
	assert.True(t, stalled.fellBehind.Load())
	assert.Len(t, drain(stalled), subscriberQueueSize, "the stalled queue is closed after what it held")
}

// Closing used to race sends to the same queues and could panic
func TestCloseWhileBroadcasting(t *testing.T) {
	b := newBroadcaster([]*gortmplib.Track{testVideoTrack})
	for i := 0; i < 8; i++ {
		b.subscribe()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10000; i++ {
			b.broadcast(videoFrame(i%30 == 0))
		}
	}()
	b.close()
	<-done

	assert.Nil(t, b.subscribe(), "a closed broadcaster takes no new subscribers")
}

func TestUnsubscribeClosesQueueOnce(t *testing.T) {
	b := newBroadcaster([]*gortmplib.Track{testVideoTrack})
	sub := b.subscribe()

	b.unsubscribe(sub)
	b.unsubscribe(sub)
	b.close()

	_, open := <-sub.frames
	assert.False(t, open)
}
