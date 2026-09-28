package webauth

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every dashboard request checks its session from its own goroutine, while
// others sign in and out. The map used to be unguarded, which can crash the
// process with a concurrent map write. Run with -race.
func TestSessionsAreSafeForConcurrentUse(t *testing.T) {
	wa := NewWebAuth(filepath.Join(t.TempDir(), "password.json"))

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				id, err := wa.CreateSession()
				require.NoError(t, err)
				assert.True(t, wa.ValidateSession(id))
				wa.InvalidateSession(id)
				assert.False(t, wa.ValidateSession(id))
			}
		})
	}
	wg.Wait()
}

func TestExpiredSessionsArePruned(t *testing.T) {
	wa := NewWebAuth(filepath.Join(t.TempDir(), "password.json"))
	old, err := wa.CreateSession()
	require.NoError(t, err)
	wa.sessions[old] = SessionData{SessionID: old, ExpiresAt: time.Now().Add(-time.Minute)}

	_, err = wa.CreateSession()
	require.NoError(t, err)
	assert.NotContains(t, wa.sessions, old)
	assert.Len(t, wa.sessions, 1)
}
