package baby

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each subscriber sees every change, in the order it happened. Changes used
// to be handed over from a goroutine each, in any order.
func TestSubscribersSeeUpdatesInOrder(t *testing.T) {
	manager := NewStateManager()

	var mu sync.Mutex
	var seen []int32
	unsubscribe := manager.Subscribe(func(_ string, state State) {
		mu.Lock()
		defer mu.Unlock()
		if state.TemperatureMilli != nil {
			seen = append(seen, *state.TemperatureMilli)
		}
	})
	defer unsubscribe()

	want := make([]int32, 100)
	for i := range want {
		want[i] = int32(20000 + i)
		manager.Update("baby1", *NewState().SetTemperatureMilli(want[i]))
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == len(want)
	}, 2*time.Second, 5*time.Millisecond)
	assert.Equal(t, want, seen)
}

// A new subscriber starts from the current state
func TestSubscribeStartsWithTheCurrentState(t *testing.T) {
	manager := NewStateManager()
	manager.Update("baby1", *NewState().SetNightLight(true))

	got := make(chan State, 1)
	unsubscribe := manager.Subscribe(func(_ string, state State) { got <- state })
	defer unsubscribe()

	select {
	case state := <-got:
		assert.True(t, state.GetNightLight())
	case <-time.After(time.Second):
		t.Fatal("no initial state")
	}
}

// Unsubscribing twice, or while updates are arriving, is safe
func TestUnsubscribeWhileUpdating(t *testing.T) {
	manager := NewStateManager()
	unsubscribe := manager.Subscribe(func(string, State) {})

	done := make(chan struct{})
	go func() {
		for i := range 200 {
			manager.Update("baby1", *NewState().SetTemperatureMilli(int32(i)))
		}
		close(done)
	}()
	unsubscribe()
	unsubscribe()
	<-done
}

// Motion and sound events used to skip Update, so the state (and history)
// never had them
func TestPolledEventsUpdateTheState(t *testing.T) {
	manager := NewStateManager()
	var recorded []State
	manager.SetHistoryCallback(func(_ string, state State) { recorded = append(recorded, state) })

	at := time.Unix(1_790_000_000, 0)
	manager.NotifyMotionSubscribers("baby1", at)
	manager.NotifySoundSubscribers("baby1", at.Add(time.Second))

	state := manager.GetBabyState("baby1")
	require.NotNil(t, state.MotionTimestamp)
	assert.Equal(t, int32(at.Unix()), *state.MotionTimestamp)
	require.NotNil(t, state.SoundTimestamp)
	assert.Len(t, recorded, 2)
}
