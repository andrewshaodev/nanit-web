// Package pull - a viewer instance's video: the stream server's RTSP output,
// played and published into the local relay hub, so the viewer's own outputs
// (the dashboard, RTSP, HLS) work as they do on the server. A viewer never
// asks the camera to stream, so it can't take the stream from the server.
package pull

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
	"github.com/rs/zerolog/log"
)

const (
	retryMin = 2 * time.Second
	retryMax = 30 * time.Second
	// Without a packet for this long the server's stream is treated as gone
	readTimeout = 10 * time.Second
)

// URL - the template with the baby's UID filled in
func URL(template, babyUID string) string {
	return strings.ReplaceAll(template, "{baby_uid}", babyUID)
}

// Run pulls url into the hub as babyUID's stream until ctx ends,
// reconnecting with backoff whenever the server's stream isn't there
func Run(ctx utils.GracefulContext, url, babyUID string, hub *relay.Hub) {
	sublog := log.With().Str("baby_uid", babyUID).Str("from", url).Logger()
	backoff := retryMin
	// A camera that's off fails the same way on every try: say so once
	lastErr := ""
	for {
		started := time.Now()
		err := pullOnce(ctx, url, babyUID, hub)
		select {
		case <-ctx.Done():
			return
		default:
		}
		// A pull that ran a while was healthy: start over from the shortest
		// wait, and report the next failure again
		if time.Since(started) > time.Minute {
			backoff = retryMin
			lastErr = ""
		}
		ev := sublog.Warn()
		if msg := fmt.Sprint(err); msg == lastErr {
			ev = sublog.Debug()
		} else {
			lastErr = msg
		}
		ev.Err(err).Dur("retry_in", backoff).Msg("Not receiving the stream server's video, will retry")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, retryMax)
	}
}

// pullOnce plays the server's stream until it ends or fails
func pullOnce(ctx utils.GracefulContext, url, babyUID string, hub *relay.Hub) error {
	u, err := base.ParseURL(url)
	if err != nil {
		return fmt.Errorf("bad NANIT_STREAM_FROM %q: %w", url, err)
	}
	tcp := gortsplib.ProtocolTCP
	c := &gortsplib.Client{Scheme: u.Scheme, Host: u.Host, Protocol: &tcp, ReadTimeout: readTimeout}
	if err := c.Start(); err != nil {
		return err
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		return err
	}

	var stream relay.Stream
	var videoMedia, audioMedia *description.Media
	var videoFormat *format.H264
	var audioFormat *format.MPEG4Audio
	if videoMedia = desc.FindFormat(&videoFormat); videoMedia != nil && videoFormat.SPS != nil && videoFormat.PPS != nil {
		stream.H264 = &relay.H264{SPS: videoFormat.SPS, PPS: videoFormat.PPS}
	} else {
		videoMedia = nil
	}
	if audioMedia = desc.FindFormat(&audioFormat); audioMedia != nil {
		stream.AAC = audioFormat.Config
	}
	if videoMedia == nil {
		return errors.New("the server's stream has no H.264 video")
	}

	medias := []*description.Media{videoMedia}
	if audioMedia != nil {
		medias = append(medias, audioMedia)
	}
	if err := c.SetupAll(desc.BaseURL, medias); err != nil {
		return err
	}

	p := hub.Publish(babyUID, stream)
	defer p.Close()
	log.Info().Str("baby_uid", babyUID).Str("from", url).Msg("Receiving video from the stream server")

	videoDec, err := videoFormat.CreateDecoder()
	if err != nil {
		return err
	}
	c.OnPacketRTP(videoMedia, videoFormat, func(pkt *rtp.Packet) {
		pts, ok := c.PacketPTS(videoMedia, pkt)
		if !ok {
			return
		}
		au, err := videoDec.Decode(pkt)
		if err != nil {
			if !errors.Is(err, rtph264.ErrMorePacketsNeeded) && !errors.Is(err, rtph264.ErrNonStartingPacketAndNoPrevious) {
				log.Debug().Err(err).Str("baby_uid", babyUID).Msg("Dropping a video packet")
			}
			return
		}
		t := toDuration(pts, videoFormat.ClockRate())
		p.Broadcast(relay.Frame{PTS: t, DTS: t, Video: au, Keyframe: h264.IsRandomAccess(au)})
	})

	if audioMedia != nil {
		audioDec, err := audioFormat.CreateDecoder()
		if err != nil {
			return err
		}
		c.OnPacketRTP(audioMedia, audioFormat, func(pkt *rtp.Packet) {
			pts, ok := c.PacketPTS(audioMedia, pkt)
			if !ok {
				return
			}
			aus, err := audioDec.Decode(pkt)
			if err != nil {
				return
			}
			// AAC frames are 1024 samples each
			for i, au := range aus {
				t := toDuration(pts+int64(i)*1024, audioFormat.ClockRate())
				p.Broadcast(relay.Frame{PTS: t, DTS: t, Audio: au})
			}
		})
	}

	if _, err := c.Play(nil); err != nil {
		return err
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- c.Wait() }()
	select {
	case <-ctx.Done():
		c.Close()
		<-waitErr
		return nil
	case err := <-waitErr:
		return err
	}
}

func toDuration(ticks int64, clockRate int) time.Duration {
	return time.Duration(ticks * int64(time.Second) / int64(clockRate))
}
