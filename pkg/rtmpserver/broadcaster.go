package rtmpserver

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortmplib"
)

// subscriberQueueSize - frames a subscriber may fall behind by before it is
// dropped. The cam sends roughly 30 video and 45 audio frames a second, so
// this is a few seconds of slack.
const subscriberQueueSize = 256

// frame - one access unit from the publisher, ready to be written to any
// subscriber. Exactly one of video and audio is set, and the slices are
// shared between subscribers, so nobody may modify them.
type frame struct {
	track    *gortmplib.Track
	pts      time.Duration
	dts      time.Duration
	video    [][]byte // H264 access unit
	audio    []byte   // AAC access unit
	keyframe bool
}

func (f frame) writeTo(w *gortmplib.Writer) error {
	if f.video != nil {
		return w.WriteH264(f.track, f.pts, f.dts, f.video)
	}
	return w.WriteMPEG4Audio(f.track, f.pts, f.audio)
}

type subscriber struct {
	frames chan frame

	// started - whether the subscriber has had its first keyframe. Frames
	// before it cannot be decoded, so none are sent until then.
	started bool

	// fellBehind - set when the subscriber was dropped for not keeping up
	fellBehind atomic.Bool
}

// broadcaster fans one publisher's frames out to its subscribers. Each
// subscriber has its own queue, drained by its own connection, so a slow or
// stalled one is dropped instead of holding up the publisher and everyone
// else.
type broadcaster struct {
	tracks   []*gortmplib.Track
	hasVideo bool

	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	closed      bool
}

func newBroadcaster(tracks []*gortmplib.Track) *broadcaster {
	b := &broadcaster{
		tracks:      tracks,
		subscribers: make(map[*subscriber]struct{}),
	}
	for _, track := range tracks {
		if track.Codec.IsVideo() {
			b.hasVideo = true
		}
	}
	return b
}

// subscribe - adds a subscriber, or returns nil once the publisher is gone
func (b *broadcaster) subscribe() *subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	sub := &subscriber{
		frames: make(chan frame, subscriberQueueSize),
		// With no video there is no keyframe to wait for
		started: !b.hasVideo,
	}
	b.subscribers[sub] = struct{}{}
	return sub
}

func (b *broadcaster) unsubscribe(sub *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.remove(sub)
}

// remove - drops a subscriber and closes its queue. The caller holds b.mu,
// which is also held for every send, so a queue is never sent to after it is
// closed.
func (b *broadcaster) remove(sub *subscriber) {
	if _, ok := b.subscribers[sub]; ok {
		delete(b.subscribers, sub)
		close(sub.frames)
	}
}

func (b *broadcaster) broadcast(f frame) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sub := range b.subscribers {
		if !sub.started {
			if !f.keyframe {
				continue
			}
			sub.started = true
		}

		select {
		case sub.frames <- f:
		default:
			sub.fellBehind.Store(true)
			b.remove(sub)
		}
	}
}

// close - ends every subscription, once the publisher is gone or replaced
func (b *broadcaster) close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	for sub := range b.subscribers {
		b.remove(sub)
	}
}
