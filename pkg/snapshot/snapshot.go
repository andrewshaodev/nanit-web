// Package snapshot - a still picture of each camera, as a JPEG, taken from
// its live stream: the next keyframe, decoded by ffmpeg. For Home
// Assistant's "Still image URL" and anything else that wants a picture
// without opening the stream.
package snapshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
)

const (
	// maxAge - how long a picture is reused: Home Assistant asks every 10 s
	// or so while a dashboard shows the camera, and several dashboards
	// asking at once cost one decode
	maxAge = 5 * time.Second
	// keyframeWait - how long to wait for a keyframe. The camera sends one
	// every couple of seconds.
	keyframeWait = 10 * time.Second
	// decodeTimeout - how long ffmpeg has to turn it into a JPEG
	decodeTimeout = 10 * time.Second
)

// ErrNoStream - the camera isn't streaming to the bridge, so there's no
// picture to take
var ErrNoStream = errors.New("the camera isn't streaming")

// Picture - a JPEG, and when its frame arrived
type Picture struct {
	JPEG  []byte
	Taken time.Time
}

// Service - takes and caches the cameras' pictures
type Service struct {
	Hub *relay.Hub

	ffmpegBin string // "ffmpeg"; tests swap in a stand-in
	now       func() time.Time

	mu      sync.Mutex
	cameras map[string]*camera
}

// camera - one camera's latest picture. Its lock is held while a picture
// is taken, so requests arriving meanwhile wait for that one.
type camera struct {
	mu     sync.Mutex
	latest *Picture
}

// New - a service taking pictures from hub's streams
func New(hub *relay.Hub) *Service {
	return &Service{Hub: hub, ffmpegBin: "ffmpeg", now: time.Now, cameras: map[string]*camera{}}
}

// Get - the camera's picture: the latest if it's recent, otherwise a new
// one from the next keyframe
func (s *Service) Get(ctx context.Context, babyUID string) (Picture, error) {
	s.mu.Lock()
	c := s.cameras[babyUID]
	if c == nil {
		c = &camera{}
		s.cameras[babyUID] = c
	}
	s.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latest != nil && s.now().Sub(c.latest.Taken) < maxAge {
		return *c.latest, nil
	}
	pic, err := s.take(ctx, babyUID)
	if err != nil {
		return Picture{}, err
	}
	c.latest = &pic
	return pic, nil
}

// take - decodes the stream's next keyframe
func (s *Service) take(ctx context.Context, babyUID string) (Picture, error) {
	if s.Hub == nil {
		return Picture{}, ErrNoStream
	}
	stream, sub, ok := s.Hub.Subscribe(babyUID)
	if !ok || stream.H264 == nil {
		if ok {
			sub.Close()
		}
		return Picture{}, ErrNoStream
	}

	frame, err := nextKeyframe(ctx, sub)
	sub.Close()
	if err != nil {
		return Picture{}, err
	}
	taken := s.now()

	// Annex B: the parameter sets, then the keyframe's NAL units
	var annexB bytes.Buffer
	for _, nalu := range append([][]byte{stream.H264.SPS, stream.H264.PPS}, frame.Video...) {
		if len(nalu) == 0 {
			continue
		}
		annexB.Write([]byte{0, 0, 0, 1})
		annexB.Write(nalu)
	}

	jpeg, err := s.decode(ctx, annexB.Bytes())
	if err != nil {
		return Picture{}, err
	}
	return Picture{JPEG: jpeg, Taken: taken}, nil
}

func nextKeyframe(ctx context.Context, sub *relay.Subscription) (relay.Frame, error) {
	timeout := time.NewTimer(keyframeWait)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return relay.Frame{}, ctx.Err()
		case <-timeout.C:
			return relay.Frame{}, fmt.Errorf("no keyframe from the camera in %v", keyframeWait)
		case f, open := <-sub.Frames:
			if !open {
				return relay.Frame{}, ErrNoStream
			}
			if f.Video != nil && f.Keyframe {
				return f, nil
			}
		}
	}
}

// decode - one H.264 keyframe to a JPEG
func (s *Service) decode(ctx context.Context, annexB []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, decodeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpegBin,
		"-hide_banner", "-loglevel", "error",
		"-f", "h264", "-i", "pipe:0",
		"-frames:v", "1", "-q:v", "3",
		"-f", "image2", "-c:v", "mjpeg", "pipe:1",
	)
	cmd.Stdin = bytes.NewReader(annexB)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	if out.Len() == 0 {
		return nil, errors.New("ffmpeg gave no picture")
	}
	return out.Bytes(), nil
}
