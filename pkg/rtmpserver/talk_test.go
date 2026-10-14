package rtmpserver

import (
	"net"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/relay"
	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSID = "0123456789abcdef"

func startTalkServer(t *testing.T) (addr string, talkHub *relay.Hub) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { lis.Close() })

	talkHub = relay.NewHub(nil)
	handler := newRtmpHandler(relay.NewHub(baby.NewStateManager()), nil)
	handler.talk = talkHub
	go handler.serve(lis)
	return lis.Addr().String(), talkHub
}

// The camera plays a talk-back session's audio from /2wa/<session ID>
func TestCameraPlaysTalkBack(t *testing.T) {
	addr, talkHub := startTalkServer(t)
	pub := talkHub.Publish(testSID, relay.Stream{AAC: &mpeg4audio.AudioSpecificConfig{
		Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 44100, ChannelConfig: 1, ChannelCount: 1,
	}})
	defer pub.Close()
	// Sent all along: the player reads the first frames to learn the tracks
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				pts := time.Duration(i) * 23 * time.Millisecond
				pub.Broadcast(relay.Frame{PTS: pts, DTS: pts, Audio: []byte{0x21, 0x10, 0x04}})
			}
		}
	}()

	player, err := tryDial(t, addr, "/2wa/"+testSID, false)
	require.NoError(t, err)
	player.NetConn().SetDeadline(time.Now().Add(5 * time.Second))
	r := &gortmplib.Reader{Conn: player}
	require.NoError(t, r.Initialize())
	require.Len(t, r.Tracks(), 1)
	_, isAAC := r.Tracks()[0].Codec.(*codecs.MPEG4Audio)
	require.True(t, isAAC)

	got := make(chan []byte, 16)
	r.OnDataMPEG4Audio(r.Tracks()[0], func(_ time.Duration, au []byte) { got <- au })

	go func() {
		for r.Read() == nil {
		}
	}()
	select {
	case au := <-got:
		assert.Equal(t, []byte{0x21, 0x10, 0x04}, au)
	case <-time.After(5 * time.Second):
		t.Fatal("the player had no audio")
	}
}

// A session that doesn't exist, or a publisher on the talk-back app, is
// turned away
func TestUnknownTalkBackIsClosed(t *testing.T) {
	addr, _ := startTalkServer(t)

	_, err := startPlayerAt(t, addr, "/2wa/"+testSID)
	assert.Error(t, err)

	pub, err := tryDial(t, addr, "/2wa/"+testSID, true)
	if err == nil {
		pub.NetConn().SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1)
		_, err = pub.NetConn().Read(buf)
	}
	assert.Error(t, err)
}
