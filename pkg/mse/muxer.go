// Package mse - a camera's stream as fragmented MP4, the format a browser's
// Media Source Extensions (MSE) play. The dashboard's live view receives it
// over a WebSocket: an init segment, then a fragment per video frame, which
// keeps it well under a second behind the camera. The H.264 and AAC are
// repackaged as they are, with no transcoding.
package mse

import (
	"errors"
	"fmt"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
)

const (
	videoTrackID   = 1
	audioTrackID   = 2
	videoTimeScale = 90000
	// AAC frames are 1024 samples each
	aacFrameSamples = 1024
)

// Muxer turns a subscription's frames into fMP4 fragments. A video frame's
// duration is only known once the next one arrives, so each fragment
// carries the previous video frame, with the audio received since.
type Muxer struct {
	stream relay.Stream
	seq    uint32

	started bool
	start   time.Duration // the first video frame's DTS: time zero

	prev      *relay.Frame // the video frame waiting for its duration
	prevDTS   uint64       // in videoTimeScale
	audio     [][]byte     // AAC frames received since the last fragment
	audioBase uint64       // the first of them, in the sample rate
}

// New - a muxer for a stream, which needs H.264 video; AAC audio is optional
func New(s relay.Stream) (*Muxer, error) {
	if s.H264 == nil || len(s.H264.SPS) < 4 {
		return nil, errors.New("the stream has no H.264 video")
	}
	return &Muxer{stream: s}, nil
}

// MIME - the type to give MediaSource.addSourceBuffer, with the codecs:
// avc1 with the SPS's profile, constraints and level, and AAC's object type
func (m *Muxer) MIME() string {
	sps := m.stream.H264.SPS
	codecs := fmt.Sprintf("avc1.%02x%02x%02x", sps[1], sps[2], sps[3])
	if m.hasAudio() {
		codecs += fmt.Sprintf(",mp4a.40.%d", m.stream.AAC.Type)
	}
	return fmt.Sprintf(`video/mp4; codecs="%s"`, codecs)
}

// Init - the initialization segment, which the player gets first
func (m *Muxer) Init() ([]byte, error) {
	init := fmp4.Init{Tracks: []*fmp4.InitTrack{{
		ID:        videoTrackID,
		TimeScale: videoTimeScale,
		Codec:     &codecs.H264{SPS: m.stream.H264.SPS, PPS: m.stream.H264.PPS},
	}}}
	if m.hasAudio() {
		init.Tracks = append(init.Tracks, &fmp4.InitTrack{
			ID:        audioTrackID,
			TimeScale: uint32(m.stream.AAC.SampleRate),
			Codec:     &codecs.MPEG4Audio{Config: *m.stream.AAC},
		})
	}
	var buf seekablebuffer.Buffer
	if err := init.Marshal(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write adds a frame, and returns a fragment when one is complete (on each
// video frame after the first), or nil
func (m *Muxer) Write(f relay.Frame) ([]byte, error) {
	if f.Video == nil {
		m.writeAudio(f)
		return nil, nil
	}

	if !m.started {
		// The hub starts subscribers at a keyframe; anything else can't be
		// decoded on its own
		if !f.Keyframe {
			return nil, nil
		}
		m.started = true
		m.start = f.DTS
	}
	if f.DTS < m.start {
		return nil, nil
	}
	dts := m.ticks(f.DTS, videoTimeScale)
	if m.prev == nil {
		m.prev, m.prevDTS = &f, dts
		return nil, nil
	}
	if dts <= m.prevDTS {
		// Out of order, or a repeat: there's no duration to give the
		// previous frame
		return nil, nil
	}

	frag, err := m.fragment(uint32(dts - m.prevDTS))
	m.prev, m.prevDTS = &f, dts
	return frag, err
}

func (m *Muxer) writeAudio(f relay.Frame) {
	// Audio before the first keyframe, or from before it, has no place on
	// the timeline
	if !m.hasAudio() || !m.started || f.PTS < m.start {
		return
	}
	if len(m.audio) == 0 {
		m.audioBase = m.ticks(f.PTS, uint32(m.stream.AAC.SampleRate))
	}
	m.audio = append(m.audio, f.Audio)
}

// fragment - the waiting video frame, with duration, and the audio since
func (m *Muxer) fragment(duration uint32) ([]byte, error) {
	ptsOffset := int32(m.ticks(m.prev.PTS, videoTimeScale)) - int32(m.prevDTS)
	sample, err := fmp4.NewSampleH264(ptsOffset, m.prev.Video)
	if err != nil {
		return nil, err
	}
	sample.Duration = duration
	sample.IsNonSyncSample = !m.prev.Keyframe

	m.seq++
	part := fmp4.Part{
		SequenceNumber: m.seq,
		Tracks: []*fmp4.PartTrack{{
			ID:       videoTrackID,
			BaseTime: m.prevDTS,
			Samples:  []*fmp4.Sample{sample},
		}},
	}
	if len(m.audio) > 0 {
		samples := make([]*fmp4.Sample, len(m.audio))
		for i, au := range m.audio {
			samples[i] = &fmp4.Sample{Duration: aacFrameSamples, Payload: au}
		}
		part.Tracks = append(part.Tracks, &fmp4.PartTrack{
			ID:       audioTrackID,
			BaseTime: m.audioBase,
			Samples:  samples,
		})
		m.audio = nil
	}

	var buf seekablebuffer.Buffer
	if err := part.Marshal(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *Muxer) hasAudio() bool {
	return m.stream.AAC != nil && m.stream.AAC.SampleRate > 0
}

// ticks - t since the first frame, in a track's time scale
func (m *Muxer) ticks(t time.Duration, scale uint32) uint64 {
	d := uint64(t - m.start)
	// In two parts, so a long session doesn't overflow
	return d/uint64(time.Second)*uint64(scale) + d%uint64(time.Second)*uint64(scale)/uint64(time.Second)
}
