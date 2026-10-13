// Package relay - each camera's live stream, fanned out to every viewer.
//
// One publisher feeds a camera's stream in: the camera itself over RTMP on
// the stream server, or a pull from the server on a viewer instance. The
// outputs (RTMP, RTSP, the dashboard's live view, HLS) are all subscribers.
// Each subscriber has its own queue, so a slow or stalled one is dropped
// instead of holding up the publisher and everyone else.
package relay

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/rs/zerolog/log"
)

// subscriberQueueSize - frames a subscriber may fall behind by before it is
// dropped. The cam sends roughly 30 video and 45 audio frames a second, so
// this is a few seconds of slack.
const subscriberQueueSize = 256

// H264 - the video track's parameter sets
type H264 struct {
	SPS []byte
	PPS []byte
}

// Stream - what a publisher sends: H.264 video and AAC audio, either of
// which may be missing
type Stream struct {
	H264 *H264
	AAC  *mpeg4audio.AudioSpecificConfig
}

// Frame - one access unit. Exactly one of Video and Audio is set, and the
// slices are shared between subscribers, so nobody may modify them.
type Frame struct {
	PTS      time.Duration
	DTS      time.Duration
	Video    [][]byte // H.264 access unit
	Audio    []byte   // AAC access unit
	Keyframe bool
}

// Subscription - one viewer's queue of frames. Frames is closed when the
// publisher goes, is replaced, or the viewer falls behind.
type Subscription struct {
	Frames <-chan Frame

	frames     chan Frame
	publisher  *Publisher
	fellBehind atomic.Bool
	// started - whether the subscriber has had its first keyframe. Frames
	// before it cannot be decoded, so none are sent until then.
	started bool
}

// FellBehind - whether the subscription was closed for not keeping up
func (s *Subscription) FellBehind() bool {
	return s.fellBehind.Load()
}

// Close - stops the subscription
func (s *Subscription) Close() {
	s.publisher.mu.Lock()
	defer s.publisher.mu.Unlock()
	s.publisher.remove(s)
}

// Hub - the publisher of each camera's stream, by baby UID
type Hub struct {
	states *baby.StateManager

	mu         sync.RWMutex
	publishers map[string]*Publisher
}

// NewHub - a hub that records each stream's state in states
func NewHub(states *baby.StateManager) *Hub {
	return &Hub{states: states, publishers: map[string]*Publisher{}}
}

// Publish makes a new publisher the camera's stream. One already there is
// replaced: the camera opening a second publisher connection replaces the
// first, and its subscribers are closed so they reconnect to the new one.
func (h *Hub) Publish(babyUID string, stream Stream) *Publisher {
	p := &Publisher{
		hub:         h,
		babyUID:     babyUID,
		stream:      stream,
		hasVideo:    stream.H264 != nil,
		subscribers: map[*Subscription]struct{}{},
	}

	h.mu.Lock()
	existing, hadExisting := h.publishers[babyUID]
	h.publishers[babyUID] = p
	h.mu.Unlock()

	if hadExisting {
		log.Warn().Str("baby_uid", babyUID).Msg("Baby already has active publisher, closing existing subscribers")
		existing.closeSubscribers()
	}
	h.states.Update(babyUID, *baby.NewState().SetStreamState(baby.StreamState_Alive).SetStreamRequestState(baby.StreamRequestState_NotRequested))
	return p
}

// Subscribe - a new subscription to the camera's stream, and what the
// stream carries. ok is false while nothing is publishing.
func (h *Hub) Subscribe(babyUID string) (stream Stream, sub *Subscription, ok bool) {
	h.mu.RLock()
	p := h.publishers[babyUID]
	h.mu.RUnlock()
	if p == nil {
		return Stream{}, nil, false
	}
	sub = p.subscribe()
	if sub == nil {
		return Stream{}, nil, false
	}
	return p.stream, sub, true
}

// Publishing - whether the camera's stream has a publisher
func (h *Hub) Publishing(babyUID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.publishers[babyUID] != nil
}

// Publisher - one source of a camera's stream
type Publisher struct {
	hub      *Hub
	babyUID  string
	stream   Stream
	hasVideo bool

	mu          sync.Mutex
	subscribers map[*Subscription]struct{}
	closed      bool

	// The packet time is in whole seconds, so once a second is as fresh
	// as it gets
	lastPacketUnix int64
}

// Stream - what the publisher sends
func (p *Publisher) Stream() Stream {
	return p.stream
}

func (p *Publisher) subscribe() *Subscription {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	frames := make(chan Frame, subscriberQueueSize)
	sub := &Subscription{
		Frames:    frames,
		frames:    frames,
		publisher: p,
		// With no video there is no keyframe to wait for
		started: !p.hasVideo,
	}
	p.subscribers[sub] = struct{}{}
	return sub
}

// remove - drops a subscriber and closes its queue. The caller holds p.mu,
// which is also held for every send, so a queue is never sent to after it
// is closed.
func (p *Publisher) remove(sub *Subscription) {
	if _, ok := p.subscribers[sub]; ok {
		delete(p.subscribers, sub)
		close(sub.frames)
	}
}

// Broadcast - sends a frame to every subscriber
func (p *Publisher) Broadcast(f Frame) {
	if now := time.Now().Unix(); now != p.lastPacketUnix {
		p.lastPacketUnix = now
		p.hub.states.Update(p.babyUID, *baby.NewState().SetLastVideoPacketTime(now))
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for sub := range p.subscribers {
		if !sub.started {
			if f.Video == nil || !f.Keyframe {
				continue
			}
			sub.started = true
		}

		select {
		case sub.frames <- f:
		default:
			sub.fellBehind.Store(true)
			p.remove(sub)
		}
	}
}

// Close - the publisher has gone. Its subscribers are closed, and unless
// it was replaced, the stream is marked unhealthy.
func (p *Publisher) Close() {
	p.hub.mu.Lock()
	current := p.hub.publishers[p.babyUID] == p
	if current {
		delete(p.hub.publishers, p.babyUID)
	}
	p.hub.mu.Unlock()

	// A publisher that was replaced leaves the stream state to the one
	// that replaced it
	if current {
		p.hub.states.Update(p.babyUID, *baby.NewState().SetStreamState(baby.StreamState_Unhealthy).SetLastVideoPacketTime(0))
	}
	p.closeSubscribers()
}

func (p *Publisher) closeSubscribers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	for sub := range p.subscribers {
		p.remove(sub)
	}
}
