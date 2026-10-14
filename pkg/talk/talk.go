// Package talk - talking through a camera's speaker. The dashboard sends the
// microphone's audio as PCM, ffmpeg encodes it to AAC, and the camera, asked
// with PUT_AUDIO_STREAMING, plays it from the RTMP server at
// rtmp://<public addr>/2wa/<session ID>, as it does from the Nanit app.
package talk

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/rs/zerolog/log"
)

// App - the RTMP app the camera plays talk-back from
const App = "2wa"

// sampleRate - the camera's AAC: 44.1 kHz mono, as the app sends it
const sampleRate = 44100

// stopTimeout - how long ffmpeg has to finish once its input ends
const stopTimeout = 3 * time.Second

// SIDRX - a session ID: 16 hex digits, as the app's
var SIDRX = regexp.MustCompile(`^[0-9a-f]{16}$`)

// URL - where the camera plays session sid from, on the RTMP server it
// reaches at publicAddr
func URL(publicAddr, sid string) string {
	return "rtmp://" + publicAddr + "/" + App + "/" + sid
}

// Service - the talk-back sessions, published by session ID for the RTMP
// server to play to the camera
type Service struct {
	Hub *relay.Hub

	ffmpegBin string // "ffmpeg"; tests swap in a stand-in
}

// New - a service with no sessions
func New() *Service {
	return &Service{Hub: relay.NewHub(nil), ffmpegBin: "ffmpeg"}
}

// Session - one stretch of talking
type Session struct {
	SID string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *bytes.Buffer
	pub    *relay.Publisher
	// encoded - closed once ffmpeg's output has all been read
	encoded   chan struct{}
	closeOnce sync.Once
}

// Start - a session taking PCM at inputRate (16-bit little-endian mono),
// whose AAC is published under its session ID
func (s *Service) Start(inputRate int) (*Session, error) {
	if inputRate < 8000 || inputRate > 192000 {
		return nil, fmt.Errorf("unsupported sample rate %d", inputRate)
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	sess := &Session{SID: hex.EncodeToString(b), stderr: &bytes.Buffer{}, encoded: make(chan struct{})}

	sess.cmd = exec.Command(s.ffmpegBin,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-f", "s16le", "-ar", strconv.Itoa(inputRate), "-ac", "1", "-i", "pipe:0",
		"-c:a", "aac", "-b:a", "64k", "-ar", strconv.Itoa(sampleRate), "-ac", "1",
		"-flush_packets", "1", "-f", "adts", "pipe:1",
	)
	var err error
	if sess.stdin, err = sess.cmd.StdinPipe(); err != nil {
		return nil, err
	}
	stdout, err := sess.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	sess.cmd.Stderr = sess.stderr
	if err := sess.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	sess.pub = s.Hub.Publish(sess.SID, relay.Stream{AAC: &mpeg4audio.AudioSpecificConfig{
		Type:          mpeg4audio.ObjectTypeAACLC,
		SampleRate:    sampleRate,
		ChannelConfig: 1,
		ChannelCount:  1,
	}})
	go func() {
		defer close(sess.encoded)
		if err := publishADTS(stdout, sess.pub); err != nil && !errors.Is(err, io.EOF) {
			log.Warn().Err(err).Str("sid", sess.SID).Msg("Talk-back encoder output unreadable")
		}
	}()
	return sess, nil
}

// Write - PCM from the microphone
func (sess *Session) Write(pcm []byte) error {
	_, err := sess.stdin.Write(pcm)
	return err
}

// Close - ends the session: ffmpeg finishes, and the camera's stream ends
func (sess *Session) Close() {
	sess.closeOnce.Do(func() {
		_ = sess.stdin.Close()
		select {
		case <-sess.encoded:
		case <-time.After(stopTimeout):
			_ = sess.cmd.Process.Kill()
			<-sess.encoded
		}
		if err := sess.cmd.Wait(); err != nil {
			log.Debug().Err(err).Str("sid", sess.SID).Str("stderr", sess.stderr.String()).Msg("Talk-back encoder exited")
		}
		sess.pub.Close()
	})
}

// publishADTS - each AAC frame of the ADTS stream r, timed by its place in
// the stream (1024 samples each)
func publishADTS(r io.Reader, pub *relay.Publisher) error {
	br := bufio.NewReader(r)
	header := make([]byte, 7)
	for n := 0; ; n++ {
		if _, err := io.ReadFull(br, header); err != nil {
			return err
		}
		if header[0] != 0xff || header[1]&0xf0 != 0xf0 {
			return errors.New("lost the ADTS sync word")
		}
		headerLen := 7
		if header[1]&0x01 == 0 { // a CRC follows
			headerLen = 9
		}
		frameLen := int(header[3]&0x03)<<11 | int(header[4])<<3 | int(header[5])>>5
		if frameLen < headerLen {
			return fmt.Errorf("bad ADTS frame length %d", frameLen)
		}
		frame := make([]byte, frameLen-7)
		if _, err := io.ReadFull(br, frame); err != nil {
			return err
		}
		pts := time.Duration(n) * 1024 * time.Second / sampleRate
		pub.Broadcast(relay.Frame{PTS: pts, DTS: pts, Audio: frame[headerLen-7:]})
	}
}
