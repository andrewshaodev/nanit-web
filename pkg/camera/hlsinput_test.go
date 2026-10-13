package camera

import (
	"testing"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
)

// The HLS converter reads the RTMP output over loopback with the internal
// key, not through the address the camera uses
func TestHLSInputURL(t *testing.T) {
	cam := New(baby.Baby{UID: "baby1"}, Options{RTMP: &RTMPOptions{
		PublicAddr:  "monitor.homelab.house:1935",
		ListenAddr:  ":1935",
		InternalKey: "KEY",
	}}, Deps{})
	assert.Equal(t, "rtmp://127.0.0.1:1935/local/baby1?key=KEY", cam.hlsInputURL())
	assert.Equal(t, "rtmp://monitor.homelab.house:1935/local/baby1", cam.LocalStreamURL())

	cam = New(baby.Baby{UID: "baby1"}, Options{}, Deps{})
	assert.Equal(t, "", cam.hlsInputURL())
}
