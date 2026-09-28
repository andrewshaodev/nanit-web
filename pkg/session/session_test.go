package session

import (
	"sync"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/stretchr/testify/assert"
)

func TestStoreCredentialsRoundTrip(t *testing.T) {
	store := NewSessionStore()

	authTime := time.Now().Truncate(time.Second)
	store.StoreCredentials("access", "refresh", authTime)

	token, storedTime := store.Credentials()
	assert.Equal(t, "access", token)
	assert.True(t, authTime.Equal(storedTime))
	assert.Equal(t, "access", store.AuthToken())
	assert.Equal(t, "refresh", store.RefreshToken())
}

func TestResetClearsCredentials(t *testing.T) {
	store := NewSessionStore()
	store.StoreCredentials("access", "refresh", time.Now())
	store.SetBabies([]baby.Baby{{UID: "baby-1"}})

	store.Reset()

	assert.Empty(t, store.AuthToken())
	assert.Empty(t, store.RefreshToken())
	assert.True(t, store.AuthTime().IsZero())
	assert.Empty(t, store.Babies())
}

// TestBabiesIsACopy - the baby list is handed out to callers that iterate it
// while the REST client may be replacing it, so it must not alias the session.
func TestBabiesIsACopy(t *testing.T) {
	store := NewSessionStore()
	store.SetBabies([]baby.Baby{{UID: "baby-1"}})

	babies := store.Babies()
	babies[0].UID = "mutated"

	assert.Equal(t, "baby-1", store.Babies()[0].UID)
}

// TestConcurrentAccess - the websocket dialer reads the token on one goroutine
// while authorization writes it on another. Run under -race, this is the guard
// against that pairing going back to unsynchronised field access.
func TestConcurrentAccess(t *testing.T) {
	store := NewSessionStore()
	store.StoreCredentials("initial", "refresh", time.Now())

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				store.StoreCredentials("rotated", "refresh", time.Now())
				store.SetBabies([]baby.Baby{{UID: "baby-1"}})
				store.SetLastSeenMessageTime(time.Now())
			}
		}
	}()

	for i := 0; i < 2000; i++ {
		token, _ := store.Credentials()
		assert.NotEmpty(t, token)
		store.Babies()
		store.RefreshToken()
		store.LastSeenMessageTime()
	}

	close(stop)
	wg.Wait()
}
