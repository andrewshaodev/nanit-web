package rtmpserver

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// handshakeTimeout - how long a new connection has to get through the
	// RTMP handshake and say what it wants
	handshakeTimeout = 10 * time.Second

	// publisherReadTimeout - how long the cam may go silent before its
	// stream is treated as dead. It sends many frames a second while up.
	publisherReadTimeout = 10 * time.Second

	// subscriberWriteTimeout - upper bound on writing one frame to a
	// subscriber, so a stalled one fails instead of blocking its connection
	subscriberWriteTimeout = 10 * time.Second
)

type rtmpHandler struct {
	babyStateManager  *baby.StateManager
	broadcastersMu    sync.RWMutex
	broadcastersByUID map[string]*broadcaster
}

// StartRTMPServer - Blocking server
func StartRTMPServer(addr string, babyStateManager *baby.StateManager) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error().Str("addr", addr).Err(err).Msg("Unable to start RTMP server")
		return fmt.Errorf("failed to start RTMP server on %s: %w", addr, err)
	}

	log.Info().Str("addr", addr).Msg("RTMP server started")

	return newRtmpHandler(babyStateManager).serve(lis)
}

func newRtmpHandler(babyStateManager *baby.StateManager) *rtmpHandler {
	return &rtmpHandler{
		broadcastersByUID: make(map[string]*broadcaster),
		babyStateManager:  babyStateManager,
	}
}

// serve - accepts connections until the listener is closed
func (s *rtmpHandler) serve(lis net.Listener) error {
	for {
		nc, err := lis.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			log.Error().Err(err).Msg("Failed to accept RTMP connection")
			time.Sleep(time.Second)
			continue
		}
		go s.handleConnection(nc)
	}
}

var rtmpURLRX = regexp.MustCompile(`^/local/([a-z0-9_-]+)$`)

func (s *rtmpHandler) handleConnection(nc net.Conn) {
	defer nc.Close()

	sublog := log.With().Stringer("client_addr", nc.RemoteAddr()).Logger()

	// Anything on the network can connect, and a malformed stream must not
	// take the whole app down with it
	defer func() {
		if r := recover(); r != nil {
			sublog.Error().Interface("panic", r).Msg("RTMP connection handler crashed, closing the connection")
		}
	}()

	nc.SetDeadline(time.Now().Add(handshakeTimeout))
	sc := &gortmplib.ServerConn{RW: nc}
	if err := sc.Initialize(); err != nil {
		sublog.Debug().Err(err).Msg("RTMP handshake failed")
		return
	}
	if err := sc.AcceptConn(); err != nil {
		sublog.Debug().Err(err).Msg("RTMP connection was not accepted")
		return
	}

	submatch := rtmpURLRX.FindStringSubmatch(sc.URL.Path)
	if len(submatch) != 2 {
		sublog.Warn().Str("path", sc.URL.Path).Msg("Invalid RTMP stream requested")
		return
	}

	babyUID := submatch[1]
	sublog = sublog.With().Str("baby_uid", babyUID).Logger()

	if sc.Publish {
		s.handlePublisher(babyUID, nc, sc, sublog)
	} else {
		s.handleSubscriber(babyUID, nc, sc, sublog)
	}
}

func (s *rtmpHandler) handlePublisher(babyUID string, nc net.Conn, sc *gortmplib.ServerConn, sublog zerolog.Logger) {
	// Reading the stream's metadata and codec configuration is still part of
	// setting up, so the handshake deadline still applies
	r := &gortmplib.Reader{Conn: sc}
	if err := r.Initialize(); err != nil {
		sublog.Warn().Err(err).Msg("Publisher did not describe its stream")
		return
	}

	b := newBroadcaster(r.Tracks())

	// The value is in whole seconds, so once a second is as fresh as it gets
	var lastPacketUnix int64
	markPacket := func() {
		if now := time.Now().Unix(); now != lastPacketUnix {
			lastPacketUnix = now
			s.babyStateManager.Update(babyUID, *baby.NewState().SetLastVideoPacketTime(now))
		}
	}

	for _, track := range r.Tracks() {
		switch track.Codec.(type) {
		case *codecs.H264:
			r.OnDataH264(track, func(pts time.Duration, dts time.Duration, au [][]byte) {
				markPacket()
				b.broadcast(frame{track: track, pts: pts, dts: dts, video: au, keyframe: h264.IsRandomAccess(au)})
			})

		case *codecs.MPEG4Audio:
			r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
				markPacket()
				b.broadcast(frame{track: track, pts: pts, audio: au})
			})

		default:
			// Every track needs a data callback, or the reader crashes on its
			// first packet. The cam sends only H264 and AAC, so refuse anything
			// else outright.
			sublog.Error().Str("codec", fmt.Sprintf("%T", track.Codec)).Msg("Publisher sent an unsupported track, closing it")
			return
		}
	}

	s.setPublisher(babyUID, b)
	sublog.Info().Msg("New stream publisher connected")
	s.babyStateManager.Update(babyUID, *baby.NewState().SetStreamState(baby.StreamState_Alive).SetStreamRequestState(baby.StreamRequestState_NotRequested))

	for {
		// Both directions: reading can mean writing acknowledgements back
		nc.SetDeadline(time.Now().Add(publisherReadTimeout))
		if err := r.Read(); err != nil {
			sublog.Warn().Err(err).Msg("Publisher stream closed unexpectedly")
			// A publisher that was replaced leaves the stream state to the one
			// that replaced it
			if s.clearPublisher(babyUID, b) {
				s.babyStateManager.Update(babyUID, *baby.NewState().SetStreamState(baby.StreamState_Unhealthy).SetLastVideoPacketTime(0))
			}
			b.close()
			return
		}
	}
}

func (s *rtmpHandler) handleSubscriber(babyUID string, nc net.Conn, sc *gortmplib.ServerConn, sublog zerolog.Logger) {
	b := s.getPublisher(babyUID)
	if b == nil {
		sublog.Warn().Msg("No stream publisher registered yet, closing subscriber stream")
		return
	}

	nc.SetDeadline(time.Now().Add(subscriberWriteTimeout))
	w := &gortmplib.Writer{Conn: sc, Tracks: b.tracks}
	if err := w.Initialize(); err != nil {
		sublog.Debug().Err(err).Msg("Unable to describe the stream to subscriber")
		return
	}
	nc.SetDeadline(time.Time{})

	sub := b.subscribe()
	if sub == nil {
		sublog.Debug().Msg("Publisher quit before the subscriber could join")
		return
	}
	defer b.unsubscribe(sub)

	sublog.Debug().Msg("New stream subscriber connected")

	// A subscriber has nothing more to say once it is playing, so the only
	// thing a read can tell us is that it has gone
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			if _, err := sc.Read(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case f, open := <-sub.frames:
			if !open {
				if sub.fellBehind.Load() {
					sublog.Warn().Msg("Closing subscriber that could not keep up with the stream")
				} else {
					sublog.Debug().Msg("Closing subscriber because publisher quit")
				}
				return
			}

			nc.SetWriteDeadline(time.Now().Add(subscriberWriteTimeout))
			if err := f.writeTo(w); err != nil {
				sublog.Debug().Err(err).Msg("Unable to write to stream subscriber")
				return
			}

		case <-gone:
			sublog.Debug().Msg("Stream subscriber disconnected")
			return
		}
	}
}

// setPublisher - makes b the baby's publisher. The cam opening a second
// publisher connection replaces the first, whose subscribers are closed so
// they reconnect to the new stream.
func (s *rtmpHandler) setPublisher(babyUID string, b *broadcaster) {
	s.broadcastersMu.Lock()
	existing, hadExisting := s.broadcastersByUID[babyUID]
	s.broadcastersByUID[babyUID] = b
	s.broadcastersMu.Unlock()

	if hadExisting {
		log.Warn().Str("baby_uid", babyUID).Msg("Baby already has active publisher, closing existing subscribers")
		existing.close()
	}
}

func (s *rtmpHandler) getPublisher(babyUID string) *broadcaster {
	s.broadcastersMu.RLock()
	defer s.broadcastersMu.RUnlock()
	return s.broadcastersByUID[babyUID]
}

// clearPublisher - removes b as the baby's publisher, reporting whether it
// still was
func (s *rtmpHandler) clearPublisher(babyUID string, b *broadcaster) bool {
	s.broadcastersMu.Lock()
	defer s.broadcastersMu.Unlock()

	if s.broadcastersByUID[babyUID] != b {
		return false
	}
	delete(s.broadcastersByUID, babyUID)
	return true
}
