// Package rtspserver - each camera's stream over RTSP, the format camera
// software (Frigate, go2rtc, NVRs, Home Assistant) expects. The stream comes
// from the relay hub as is: H.264 and AAC, with no transcoding. With the
// stream password on, players sign in as streamauth.User with the key.
package rtspserver

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/streamauth"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph264"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpmpeg4audio"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/rs/zerolog/log"
)

// pathRX - rtsp://host:port/local/{baby_uid}, as for RTMP. Setup requests
// add a track suffix after the UID.
var pathRX = regexp.MustCompile(`^/?local/([a-z0-9_-]+)(/.*)?$`)

// server - one RTSP stream per camera that is publishing to the hub,
// created when a player first asks for it
type server struct {
	hub  *relay.Hub
	auth *streamauth.Store // nil: no password
	srv  *gortsplib.Server

	mu      sync.Mutex
	streams map[string]*cameraStream
}

type cameraStream struct {
	stream *gortsplib.ServerStream
	sub    *relay.Subscription
}

// Serve runs the RTSP server on addr until ctx ends. It is TCP only, so a
// container needs just the one port.
func Serve(ctx utils.GracefulContext, addr string, hub *relay.Hub, auth *streamauth.Store) error {
	s := &server{hub: hub, auth: auth, streams: map[string]*cameraStream{}}
	s.srv = &gortsplib.Server{
		Handler:      s,
		RTSPAddress:  addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if err := s.srv.Start(); err != nil {
		return fmt.Errorf("failed to start RTSP server on %s: %w", addr, err)
	}
	log.Info().Str("addr", addr).Msg("RTSP server started")

	waitErr := make(chan error, 1)
	go func() { waitErr <- s.srv.Wait() }()
	if auth != nil {
		go s.closeOnPasswordChange(ctx)
	}

	select {
	case <-ctx.Done():
		s.srv.Close()
		<-waitErr
		s.closeAll()
		log.Info().Msg("RTSP server stopped")
		return nil
	case err := <-waitErr:
		s.closeAll()
		return err
	}
}

// closeOnPasswordChange ends every stream when the password is turned on or
// off or its key changes, so players already watching have to sign in with
// the new settings
func (s *server) closeOnPasswordChange(ctx utils.GracefulContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.auth.Changed():
			log.Info().Msg("Stream password changed, disconnecting RTSP players")
			s.closeAll()
		}
	}
}

// signedIn - whether a request carries the stream password, when one is set
func (s *server) signedIn(conn *gortsplib.ServerConn, req *base.Request) bool {
	key, required := s.auth.Required()
	return !required || conn.VerifyCredentials(req, streamauth.User, key)
}

// unauthorized asks the player to sign in, or closes the connection if it
// gave the wrong password. A new one each time: the library adds the
// sign-in challenge to its headers.
func unauthorized() *base.Response {
	return &base.Response{StatusCode: base.StatusUnauthorized}
}

// OnDescribe - a player asks what a camera's stream carries
func (s *server) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	if !s.signedIn(ctx.Conn, ctx.Request) {
		return unauthorized(), nil, liberrors.ErrServerAuth{}
	}
	stream := s.streamFor(ctx.Path, true)
	if stream == nil {
		return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
	}
	return &base.Response{StatusCode: base.StatusOK}, stream, nil
}

// OnSetup - a player picks the tracks to receive
func (s *server) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	if !s.signedIn(ctx.Conn, ctx.Request) {
		return unauthorized(), nil, liberrors.ErrServerAuth{}
	}
	stream := s.streamFor(ctx.Path, false)
	if stream == nil {
		return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
	}
	return &base.Response{StatusCode: base.StatusOK}, stream, nil
}

// OnPlay - a player starts receiving
func (s *server) OnPlay(_ *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	return &base.Response{StatusCode: base.StatusOK}, nil
}

// streamFor - the RTSP stream for the camera a path names. With create, one
// is set up from the hub if the camera is publishing.
func (s *server) streamFor(path string, create bool) *gortsplib.ServerStream {
	m := pathRX.FindStringSubmatch(path)
	if m == nil {
		return nil
	}
	uid := m[1]

	s.mu.Lock()
	defer s.mu.Unlock()
	if cs, ok := s.streams[uid]; ok {
		return cs.stream
	}
	if !create {
		return nil
	}

	relayStream, sub, ok := s.hub.Subscribe(uid)
	if !ok {
		return nil
	}

	var video, audio *description.Media
	var videoFormat *format.H264
	var audioFormat *format.MPEG4Audio
	desc := &description.Session{}
	if relayStream.H264 != nil {
		videoFormat = &format.H264{PayloadTyp: 96, PacketizationMode: 1, SPS: relayStream.H264.SPS, PPS: relayStream.H264.PPS}
		video = &description.Media{Type: description.MediaTypeVideo, Formats: []format.Format{videoFormat}}
		desc.Medias = append(desc.Medias, video)
	}
	if relayStream.AAC != nil {
		audioFormat = &format.MPEG4Audio{PayloadTyp: 97, Config: relayStream.AAC, SizeLength: 13, IndexLength: 3, IndexDeltaLength: 3}
		audio = &description.Media{Type: description.MediaTypeAudio, Formats: []format.Format{audioFormat}}
		desc.Medias = append(desc.Medias, audio)
	}

	stream := &gortsplib.ServerStream{Server: s.srv, Desc: desc}
	if err := stream.Initialize(); err != nil {
		log.Error().Err(err).Str("baby_uid", uid).Msg("Failed to set up the RTSP stream")
		sub.Close()
		return nil
	}

	cs := &cameraStream{stream: stream, sub: sub}
	s.streams[uid] = cs
	go s.pump(uid, cs, video, videoFormat, audio, audioFormat)
	log.Info().Str("baby_uid", uid).Msg("RTSP stream started")
	return stream
}

// pump - the camera's frames into its RTSP stream, until the camera's stream
// ends or is replaced. Players are then disconnected, and reconnect to the
// new one, as RTMP players do.
func (s *server) pump(uid string, cs *cameraStream, video *description.Media, videoFormat *format.H264, audio *description.Media, audioFormat *format.MPEG4Audio) {
	defer func() {
		s.mu.Lock()
		if s.streams[uid] == cs {
			delete(s.streams, uid)
		}
		s.mu.Unlock()
		cs.sub.Close()
		cs.stream.Close()
		log.Info().Str("baby_uid", uid).Msg("RTSP stream ended")
	}()

	sublog := log.With().Str("baby_uid", uid).Logger()
	videoEnc, audioEnc := newVideoEncoder(videoFormat), newAudioEncoder(audioFormat)
	videoStart, audioStart := randomUint32(), randomUint32()

	for f := range cs.sub.Frames {
		if f.Video != nil {
			if videoEnc == nil {
				continue
			}
			packets, err := videoEnc.Encode(f.Video)
			if err != nil {
				sublog.Debug().Err(err).Msg("Failed to packetize a video frame")
				continue
			}
			ts := videoStart + uint32(int64(f.PTS)*90000/int64(time.Second))
			for _, pkt := range packets {
				pkt.Timestamp = ts
				_ = cs.stream.WritePacketRTPWithNTP(video, pkt, time.Now())
			}
			continue
		}

		if audioEnc == nil {
			continue
		}
		packets, err := audioEnc.Encode([][]byte{f.Audio})
		if err != nil {
			sublog.Debug().Err(err).Msg("Failed to packetize an audio frame")
			continue
		}
		ts := audioStart + uint32(int64(f.PTS)*int64(audioFormat.ClockRate())/int64(time.Second))
		for _, pkt := range packets {
			pkt.Timestamp = ts
			_ = cs.stream.WritePacketRTPWithNTP(audio, pkt, time.Now())
		}
	}
}

// newVideoEncoder - packetizes H.264 for RTP, or nil without video
func newVideoEncoder(f *format.H264) *rtph264.Encoder {
	if f == nil {
		return nil
	}
	enc, err := f.CreateEncoder()
	if err != nil {
		log.Error().Err(err).Msg("Failed to set up the RTSP video encoder")
		return nil
	}
	return enc
}

// newAudioEncoder - packetizes AAC for RTP, or nil without audio
func newAudioEncoder(f *format.MPEG4Audio) *rtpmpeg4audio.Encoder {
	if f == nil {
		return nil
	}
	enc, err := f.CreateEncoder()
	if err != nil {
		log.Error().Err(err).Msg("Failed to set up the RTSP audio encoder")
		return nil
	}
	return enc
}

func (s *server) closeAll() {
	s.mu.Lock()
	all := s.streams
	s.streams = map[string]*cameraStream{}
	s.mu.Unlock()
	for _, cs := range all {
		// The pump ends once its subscription is closed
		cs.sub.Close()
	}
}

func randomUint32() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint32(b[:])
}
