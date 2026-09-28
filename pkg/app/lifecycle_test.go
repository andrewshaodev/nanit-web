package app

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp - the app with no Nanit session, history on, and its files in
// a temp dir, serving on port
func newTestApp(t *testing.T, port int) *App {
	t.Helper()
	dir := t.TempDir()
	app, err := NewApp(Opts{
		SessionFile:     filepath.Join(dir, "session.json"),
		DataDirectories: DataDirectories{BaseDir: dir, HistoryDir: dir},
		HTTPPort:        port,
		History:         HistoryOpts{Enabled: true},
		WebAuth:         WebAuthOpts{PasswordFile: filepath.Join(dir, "password.json")},
	})
	require.NoError(t, err)
	return app
}

// freePort - a port nothing is listening on
func freePort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
	return lis.Addr().(*net.TCPAddr).Port
}

// Stopping the app stops everything it started, and the HTTP server with it
func TestRunShutsDownCleanly(t *testing.T) {
	before := runtime.NumGoroutine()
	port := freePort(t)
	app := newTestApp(t, port)

	runner := utils.RunWithGracefulCancel(app.Run)
	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", itoa(port)))
		if err == nil {
			conn.Close()
		}
		return err == nil
	}, 5*time.Second, 20*time.Millisecond, "the dashboard should start")

	done := make(chan struct{})
	go func() {
		runner.Cancel()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown didn't finish")
	}

	_, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", itoa(port)))
	assert.Error(t, err, "the HTTP server should be closed")

	// Polled by hand: assert.Eventually runs its check on a goroutine of its own
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "goroutines left after shutdown")
}

// If the dashboard can't start, the app stops with the error instead of
// carrying on without one
func TestRunStopsWhenTheHTTPPortIsTaken(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer lis.Close()
	app := newTestApp(t, lis.Addr().(*net.TCPAddr).Port)

	runner := utils.RunWithGracefulCancel(app.Run)
	stopped := make(chan error, 1)
	go func() {
		_, err := runner.Wait()
		stopped <- err
	}()

	select {
	case err := <-stopped:
		assert.ErrorContains(t, err, "HTTP server")
	case <-time.After(5 * time.Second):
		runner.Cancel()
		t.Fatal("the app kept running without its HTTP server")
	}
}

func itoa(n int) string { return fmt.Sprint(n) }

// State changes are queued for a single history writer. When it's told to
// stop, it writes what's still queued first: Run closes the database only
// after that, where it used to close it while writes could still arrive.
func TestHistoryWriterDrainsItsQueueOnShutdown(t *testing.T) {
	app := newTestApp(t, freePort(t))
	defer app.HistoryTracker.Close()
	app.historyQueue = make(chan historyUpdate, historyQueueSize)
	for i := range 20 {
		app.historyQueue <- historyUpdate{"baby1", *baby.NewState().SetTemperatureMilli(int32(20000 + i))}
	}

	// Already cancelled: all it has left to do is drain
	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		ctx.Fail(errors.New("stop"))
		app.writeHistory(ctx)
	})
	_, _ = runner.Wait()

	readings, err := app.HistoryTracker.GetSensorReadings("baby1", 0, time.Now().Unix()+60, 100)
	require.NoError(t, err)
	assert.Len(t, readings, 20)
}
