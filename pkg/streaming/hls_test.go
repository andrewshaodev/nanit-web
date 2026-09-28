package streaming

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A retry used to rebuild its own argument list, which still re-encoded with
// libx264, so the first FFmpeg failure silently brought the CPU load back.
func TestFFmpegArgsRemuxOnly(t *testing.T) {
	h := &HLSTranscoder{rtmpURL: "rtmp://127.0.0.1/local/baby", hlsDir: t.TempDir()}
	args := h.ffmpegArgs()

	assert.NotContains(t, args, "libx264")
	assert.Subset(t, args, []string{"-c:v", "copy", "-c:a", "copy"})
}

// fakeFFmpeg writes a stand-in for ffmpeg that records each launch in
// launchLog, exits with an error on the first launch (like ffmpeg does when
// the cam is not publishing yet), and keeps running on every later one.
func fakeFFmpeg(t *testing.T) (bin, launchLog string) {
	dir := t.TempDir()
	bin = filepath.Join(dir, "ffmpeg")
	launchLog = filepath.Join(dir, "launches")
	script := fmt.Sprintf(`#!/bin/sh
echo launch >> %q
[ "$(wc -l < %q)" -eq 1 ] && exit 1
exec sleep 30
`, launchLog, launchLog)
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin, launchLog
}

func launches(launchLog string) int {
	data, err := os.ReadFile(launchLog)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "launch")
}

// The retry used to see the transcoder marked stopped by the failed run it was
// retrying, and quietly gave up, so FFmpeg was never relaunched.
func TestFailedFFmpegIsRetried(t *testing.T) {
	bin, launchLog := fakeFFmpeg(t)
	h := NewHLSTranscoder("baby", "rtmp://127.0.0.1/local/baby", t.TempDir())
	h.ffmpegBin = bin
	h.retryDelay = 50 * time.Millisecond

	require.NoError(t, h.Start())
	t.Cleanup(h.Stop)

	assert.Eventually(t, func() bool { return launches(launchLog) == 2 }, 5*time.Second, 20*time.Millisecond,
		"FFmpeg should be relaunched after its first run fails")
	assert.True(t, h.IsRunning(), "a relaunched transcoder should report running")
}

// Between the failed run and its retry the transcoder must still count as
// running, or the app's stream monitor restarts it at the same time.
func TestTranscoderRunningWhileRetryPending(t *testing.T) {
	bin, launchLog := fakeFFmpeg(t)
	h := NewHLSTranscoder("baby", "rtmp://127.0.0.1/local/baby", t.TempDir())
	h.ffmpegBin = bin
	h.retryDelay = time.Hour

	require.NoError(t, h.Start())
	t.Cleanup(h.Stop)

	require.Eventually(t, func() bool {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
		return h.retryCount == 1
	}, 5*time.Second, 20*time.Millisecond, "the failed run should schedule a retry")
	assert.True(t, h.IsRunning())
	assert.Equal(t, 1, launches(launchLog))
}

// Stopping the transcoder cancels a pending retry.
func TestStopCancelsPendingRetry(t *testing.T) {
	bin, launchLog := fakeFFmpeg(t)
	h := NewHLSTranscoder("baby", "rtmp://127.0.0.1/local/baby", t.TempDir())
	h.ffmpegBin = bin
	h.retryDelay = 200 * time.Millisecond

	require.NoError(t, h.Start())
	require.Eventually(t, func() bool {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
		return h.retryCount == 1
	}, 5*time.Second, 20*time.Millisecond)

	h.Stop()
	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, 1, launches(launchLog), "no relaunch after Stop")
	assert.False(t, h.IsRunning())
}
