package baby

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// subscriberQueueSize - how many updates a subscriber can fall behind by.
// Updates come a few a second at most (the RTMP server marks each stream
// alive once a second), so a full queue means the subscriber is stuck.
const subscriberQueueSize = 256

type stateUpdate struct {
	babyUID string
	state   State
}

type subscriber struct {
	updates  chan stateUpdate
	callback func(babyUID string, state State)
}

// StateManager - state manager context
type StateManager struct {
	babiesByUID map[string]State
	// Guarded by stateMutex too, which Update holds while it hands each
	// change to the subscribers, so each sees every change in order
	subscribers      map[*subscriber]struct{}
	stateMutex       sync.RWMutex
	subscribersMutex sync.Mutex
	historyCallback  func(babyUID string, state State) // Callback for historical tracking
}

// NewStateManager - state manager constructor
func NewStateManager() *StateManager {
	return &StateManager{
		babiesByUID: make(map[string]State),
		subscribers: make(map[*subscriber]struct{}),
	}
}

// Update - updates baby info in thread safe manner
func (manager *StateManager) Update(babyUID string, stateUpdate State) {
	var updatedState *State

	manager.stateMutex.Lock()
	defer manager.stateMutex.Unlock()

	if babyState, ok := manager.babiesByUID[babyUID]; ok {
		updatedState = babyState.Merge(&stateUpdate)
		if updatedState == &babyState {
			return
		}
	} else {
		updatedState = NewState().Merge(&stateUpdate)
	}

	manager.babiesByUID[babyUID] = *updatedState
	stateUpdate.EnhanceLogEvent(log.Debug().Str("baby_uid", babyUID)).Msg("Baby state updated")

	// Record historical data if callback is set. It is called here, in
	// order, so it must not block (the app's only queues the change).
	if manager.historyCallback != nil {
		manager.historyCallback(babyUID, stateUpdate)
	}

	// Each change used to go to each subscriber from a goroutine of its
	// own, so they could arrive out of order
	manager.notifySubscribers(babyUID, stateUpdate)
}

// Subscribe - registers function to be called on every update, starting with
// the current state of every baby. Calls are made one at a time, in order.
// Returns unsubscribe function
func (manager *StateManager) Subscribe(callback func(babyUID string, state State)) func() {
	sub := &subscriber{
		updates:  make(chan stateUpdate, subscriberQueueSize),
		callback: callback,
	}
	go func() {
		for update := range sub.updates {
			sub.callback(update.babyUID, update.state)
		}
	}()

	// Holding the state lock, no update can land between the snapshot and
	// the registration
	manager.stateMutex.RLock()
	manager.subscribersMutex.Lock()
	manager.subscribers[sub] = struct{}{}
	for babyUID, babyState := range manager.babiesByUID {
		sub.send(stateUpdate{babyUID, babyState})
	}
	manager.subscribersMutex.Unlock()
	manager.stateMutex.RUnlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			manager.subscribersMutex.Lock()
			delete(manager.subscribers, sub)
			// Only sent to under subscribersMutex, so nothing is mid-send
			close(sub.updates)
			manager.subscribersMutex.Unlock()
		})
	}
}

// send queues an update without blocking the state manager
func (sub *subscriber) send(update stateUpdate) {
	select {
	case sub.updates <- update:
	default:
		log.Warn().Str("baby_uid", update.babyUID).Msg("State subscriber is falling behind, dropping an update")
	}
}

// GetBabyState - returns current state of a baby
func (manager *StateManager) GetBabyState(babyUID string) *State {
	manager.stateMutex.RLock()
	babyState := manager.babiesByUID[babyUID]
	manager.stateMutex.RUnlock()

	return &babyState
}

// NotifyMotionSubscribers - records a motion event polled from Nanit. It
// used to go to the subscribers only, skipping Update, so the state and the
// history never had it.
func (manager *StateManager) NotifyMotionSubscribers(babyUID string, time time.Time) {
	timestamp := int32(time.Unix())
	manager.Update(babyUID, State{MotionTimestamp: &timestamp})
}

// NotifySoundSubscribers - records a sound event polled from Nanit
func (manager *StateManager) NotifySoundSubscribers(babyUID string, time time.Time) {
	timestamp := int32(time.Unix())
	manager.Update(babyUID, State{SoundTimestamp: &timestamp})
}

// notifySubscribers - stateMutex must be held
func (manager *StateManager) notifySubscribers(babyUID string, state State) {
	manager.subscribersMutex.Lock()
	defer manager.subscribersMutex.Unlock()

	for sub := range manager.subscribers {
		sub.send(stateUpdate{babyUID, state})
	}
}

// SetHistoryCallback sets a callback function for historical data tracking.
// It is called in order with each change, and must not block.
func (manager *StateManager) SetHistoryCallback(callback func(babyUID string, state State)) {
	manager.stateMutex.Lock()
	defer manager.stateMutex.Unlock()
	manager.historyCallback = callback
}
