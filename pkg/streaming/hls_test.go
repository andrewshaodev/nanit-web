package streaming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A retry used to rebuild its own argument list, which still re-encoded with
// libx264, so the first FFmpeg failure silently brought the CPU load back.
func TestFFmpegArgsRemuxOnly(t *testing.T) {
	h := &HLSTranscoder{rtmpURL: "rtmp://127.0.0.1/local/baby", hlsDir: t.TempDir()}
	args := h.ffmpegArgs()

	assert.NotContains(t, args, "libx264")
	assert.Subset(t, args, []string{"-c:v", "copy", "-c:a", "copy"})
}
