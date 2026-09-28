package utils_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// events records what the goroutines under test did, in order. The order
// comes from the sleeps; the mutex only keeps the race detector quiet.
type events struct {
	mu  sync.Mutex
	out string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.out += " " + s
}

func (e *events) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.out
}

func TestGracefulRunner(t *testing.T) {
	out := &events{}

	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			<-childCtx.Done()
			time.Sleep(500 * time.Millisecond)
			out.add("sub_finished")
		})

		<-ctx.Done()
		time.Sleep(200 * time.Millisecond)
		out.add("main_finished")
	})

	time.Sleep(100 * time.Millisecond)
	runner.Cancel()
	out.add("after_cancel")

	assert.Equal(t, " main_finished sub_finished after_cancel", out.String())
}

func TestGracefulRunnerFinished(t *testing.T) {
	out := &events{}
	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			time.Sleep(300 * time.Millisecond)
			out.add("sub_finished")
		})

		time.Sleep(200 * time.Millisecond)
		out.add("main_finished")
	})

	_, err := runner.Wait()
	assert.NoError(t, err)
	assert.Equal(t, " main_finished sub_finished", out.String())
}

func TestGracefulRunnerFail(t *testing.T) {
	out := &events{}
	runner := utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			<-childCtx.Done()
			time.Sleep(200 * time.Millisecond)
			out.add("sub_finished")
		})

		for {
			select {
			case <-time.After(200 * time.Millisecond):
				ctx.Fail(errors.New("simulated failure"))
			case <-ctx.Done():
				time.Sleep(100 * time.Millisecond)
				out.add("main_finished")
				return
			}
		}
	})

	_, err := runner.Wait()
	assert.EqualError(t, err, "simulated failure")
	assert.Equal(t, " main_finished sub_finished", out.String())
}
