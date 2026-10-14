package rtmpserver

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/andrewshaodev/nanit-web/pkg/talk"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
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

// rtmpHandler - the camera publishes into the relay hub over RTMP, and RTMP
// players watch it. With the stream password on, players add ?key= to the
// address; the camera's push never needs it.
type rtmpHandler struct {
	hub  *relay.Hub
	auth *streamauth.Store // nil: no password
	// talk - talk-back sessions, which the camera plays from /2wa/<session
	// ID>; nil without talk-back
	talk *relay.Hub
}

// Serve runs the RTMP server on addr until ctx ends, then closes its
// listener. The listener used to stay open for the life of the process.
func Serve(ctx utils.GracefulContext, addr string, hub *relay.Hub, auth *streamauth.Store, talkHub *relay.Hub) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start RTMP server on %s: %w", addr, err)
	}

	log.Info().Str("addr", addr).Msg("RTMP server started")

	go func() {
		<-ctx.Done()
		lis.Close()
	}()

	handler := newRtmpHandler(hub, auth)
	handler.talk = talkHub
	err = handler.serve(lis)
	if errors.Is(err, net.ErrClosed) {
		log.Info().Msg("RTMP server stopped")
		return nil
	}
	return err
}

func newRtmpHandler(hub *relay.Hub, auth *streamauth.Store) *rtmpHandler {
	return &rtmpHandler{hub: hub, auth: auth}
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

var (
	rtmpURLRX = regexp.MustCompile(`^/local/([a-z0-9_-]+)$`)
	talkURLRX = regexp.MustCompile(`^/` + talk.App + `/([0-9a-f]{16})$`)
)

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

	// The camera playing talk-back. The session ID, random and only around
	// while someone talks, is what lets it in.
	if m := talkURLRX.FindStringSubmatch(sc.URL.Path); m != nil && !sc.Publish && s.talk != nil {
		s.handleSubscriber(s.talk, m[1], nc, sc, sublog.With().Str("talk_sid", m[1]).Logger())
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
		if !s.auth.Allows(sc.URL.Query().Get("key")) {
			sublog.Warn().Msg("RTMP player refused: missing or wrong stream password key")
			return
		}
		s.handleSubscriber(s.hub, babyUID, nc, sc, sublog)
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

	var stream relay.Stream
	for _, track := range r.Tracks() {
		switch codec := track.Codec.(type) {
		case *codecs.H264:
			stream.H264 = &relay.H264{SPS: codec.SPS, PPS: codec.PPS}
		case *codecs.MPEG4Audio:
			stream.AAC = codec.Config
		default:
			// Every track needs a data callback, or the reader crashes on its
			// first packet. The cam sends only H264 and AAC, so refuse anything
			// else outright.
			sublog.Error().Str("codec", fmt.Sprintf("%T", track.Codec)).Msg("Publisher sent an unsupported track, closing it")
			return
		}
	}

	p := s.hub.Publish(babyUID, stream)
	defer p.Close()
	sublog.Info().Msg("New stream publisher connected")

	for _, track := range r.Tracks() {
		switch track.Codec.(type) {
		case *codecs.H264:
			r.OnDataH264(track, func(pts time.Duration, dts time.Duration, au [][]byte) {
				p.Broadcast(relay.Frame{PTS: pts, DTS: dts, Video: au, Keyframe: h264.IsRandomAccess(au)})
			})
		case *codecs.MPEG4Audio:
			r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
				p.Broadcast(relay.Frame{PTS: pts, DTS: pts, Audio: au})
			})
		}
	}

	for {
		// Both directions: reading can mean writing acknowledgements back
		nc.SetDeadline(time.Now().Add(publisherReadTimeout))
		if err := r.Read(); err != nil {
			sublog.Warn().Err(err).Msg("Publisher stream closed unexpectedly")
			return
		}
	}
}

func (s *rtmpHandler) handleSubscriber(hub *relay.Hub, babyUID string, nc net.Conn, sc *gortmplib.ServerConn, sublog zerolog.Logger) {
	// A change to the stream password disconnects players, so they have to
	// connect again with the new settings
	var passwordChanged <-chan struct{}
	if s.auth != nil {
		passwordChanged = s.auth.Changed()
	}

	stream, sub, ok := hub.Subscribe(babyUID)
	if !ok {
		sublog.Warn().Msg("No stream publisher registered yet, closing subscriber stream")
		return
	}
	defer sub.Close()

	var video, audio *gortmplib.Track
	var tracks []*gortmplib.Track
	if stream.H264 != nil {
		video = &gortmplib.Track{Codec: &codecs.H264{SPS: stream.H264.SPS, PPS: stream.H264.PPS}}
		tracks = append(tracks, video)
	}
	if stream.AAC != nil {
		audio = &gortmplib.Track{Codec: &codecs.MPEG4Audio{Config: stream.AAC}}
		tracks = append(tracks, audio)
	}

	nc.SetDeadline(time.Now().Add(subscriberWriteTimeout))
	w := &gortmplib.Writer{Conn: sc, Tracks: tracks}
	if err := w.Initialize(); err != nil {
		sublog.Debug().Err(err).Msg("Unable to describe the stream to subscriber")
		return
	}
	nc.SetDeadline(time.Time{})

	sublog.Debug().Msg("New stream subscriber connected")

	// A subscriber has nothing more to say once it is playing, so the only
	// thing a read can tell us is that it has gone. The writer reads the
	// connection itself (since gortmplib 1.0.4), and Wait returns when that
	// read fails; reading it here as well raced with it.
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		_ = w.Wait()
	}()

	for {
		select {
		case f, open := <-sub.Frames:
			if !open {
				if sub.FellBehind() {
					sublog.Warn().Msg("Closing subscriber that could not keep up with the stream")
				} else {
					sublog.Debug().Msg("Closing subscriber because publisher quit")
				}
				return
			}

			nc.SetWriteDeadline(time.Now().Add(subscriberWriteTimeout))
			var err error
			if f.Video != nil {
				err = w.WriteH264(video, f.PTS, f.DTS, f.Video)
			} else {
				err = w.WriteMPEG4Audio(audio, f.PTS, f.Audio)
			}
			if err != nil {
				sublog.Debug().Err(err).Msg("Unable to write to stream subscriber")
				return
			}

		case <-gone:
			sublog.Debug().Msg("Stream subscriber disconnected")
			return

		case <-passwordChanged:
			sublog.Info().Msg("Stream password changed, disconnecting RTMP player")
			return
		}
	}
}
