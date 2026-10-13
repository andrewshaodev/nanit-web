package relay

import (
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUID = "baby1"

var (
	videoAndAudio = Stream{H264: &H264{SPS: []byte{0x67}, PPS: []byte{0x68}}}
	audioOnly     = Stream{}
)

func videoFrame(keyframe bool) Frame {
	return Frame{Video: [][]byte{{0x65}}, Keyframe: keyframe}
}

func audioFrame() Frame {
	return Frame{Audio: []byte{0x01}}
}

func drain(sub *Subscription) []Frame {
	var frames []Frame
	for {
		select {
		case f, open := <-sub.Frames:
			if !open {
				return frames
			}
			frames = append(frames, f)
		default:
			return frames
		}
	}
}

func subscribe(t *testing.T, h *Hub) *Subscription {
	t.Helper()
	_, sub, ok := h.Subscribe(testUID)
	require.True(t, ok)
	return sub
}

// Frames before the first keyframe cannot be decoded, so a subscriber joining
// mid-stream starts at the next one, audio included.
func TestSubscriberStartsAtKeyframe(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	p := h.Publish(testUID, videoAndAudio)
	sub := subscribe(t, h)

	p.Broadcast(videoFrame(false))
	p.Broadcast(audioFrame())
	p.Broadcast(videoFrame(true))
	p.Broadcast(audioFrame())
	p.Broadcast(videoFrame(false))

	got := drain(sub)
	require.Len(t, got, 3)
	assert.True(t, got[0].Keyframe)
	assert.NotNil(t, got[1].Audio)
	assert.False(t, got[2].Keyframe)
}

func TestAudioOnlySubscriberStartsImmediately(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	p := h.Publish(testUID, audioOnly)
	sub := subscribe(t, h)

	p.Broadcast(audioFrame())

	assert.Len(t, drain(sub), 1)
}

// A subscriber that stops reading used to block the publisher, and with it
// every other subscriber. It is now dropped once its queue is full.
func TestStalledSubscriberDoesNotBlockOthers(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	p := h.Publish(testUID, videoAndAudio)
	stalled := subscribe(t, h)
	healthy := subscribe(t, h)

	// The healthy subscriber keeps up exactly, taking each frame as it is
	// sent; the stalled one takes none
	received := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i <= subscriberQueueSize*4; i++ {
			p.Broadcast(videoFrame(i == 0))
			if _, open := <-healthy.Frames; open {
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
	assert.False(t, healthy.FellBehind())
	assert.True(t, stalled.FellBehind())
	assert.Len(t, drain(stalled), subscriberQueueSize, "the stalled queue is closed after what it held")
}

// Closing used to race sends to the same queues and could panic
func TestCloseWhileBroadcasting(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	p := h.Publish(testUID, videoAndAudio)
	for i := 0; i < 8; i++ {
		subscribe(t, h)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10000; i++ {
			p.Broadcast(videoFrame(i%30 == 0))
		}
	}()
	p.Close()
	<-done

	_, _, ok := h.Subscribe(testUID)
	assert.False(t, ok, "a closed publisher takes no new subscribers")
}

func TestUnsubscribeClosesQueueOnce(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	p := h.Publish(testUID, videoAndAudio)
	sub := subscribe(t, h)

	sub.Close()
	sub.Close()
	p.Close()

	_, open := <-sub.Frames
	assert.False(t, open)
}

// The camera opening a second publisher replaces the first: the first's
// subscribers are closed so they reconnect, and its closing later doesn't
// mark the new stream unhealthy
func TestSecondPublisherReplacesFirst(t *testing.T) {
	states := baby.NewStateManager()
	h := NewHub(states)
	first := h.Publish(testUID, videoAndAudio)
	old := subscribe(t, h)

	second := h.Publish(testUID, videoAndAudio)
	_, open := <-old.Frames
	assert.False(t, open, "the first publisher's subscribers are closed")

	first.Close()
	assert.True(t, h.Publishing(testUID))
	assert.Equal(t, baby.StreamState_Alive, states.GetBabyState(testUID).GetStreamState())

	second.Close()
	assert.False(t, h.Publishing(testUID))
	assert.Equal(t, baby.StreamState_Unhealthy, states.GetBabyState(testUID).GetStreamState())
}

func TestSubscribeWithoutPublisher(t *testing.T) {
	h := NewHub(baby.NewStateManager())
	_, _, ok := h.Subscribe(testUID)
	assert.False(t, ok)
}

// Frames mark the stream as having video, once a second at most
func TestBroadcastMarksPacketTime(t *testing.T) {
	states := baby.NewStateManager()
	h := NewHub(states)
	p := h.Publish(testUID, videoAndAudio)
	p.Broadcast(videoFrame(true))

	got := states.GetBabyState(testUID).GetLastVideoPacketTime()
	require.NotNil(t, got)
	assert.InDelta(t, time.Now().Unix(), *got, 2)
}
