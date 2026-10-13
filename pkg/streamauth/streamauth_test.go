package streamauth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOffUntilEnabled(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "stream_auth.json"))
	assert.False(t, s.Get().Enabled)
	assert.True(t, s.Allows(""), "open while off")
	var none *Store
	assert.True(t, none.Allows(""), "a nil store is open")
}

// Turning it on makes a key, which is kept through a restart, and only the
// key is let in
func TestEnableMakesAKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stream_auth.json")
	s := Open(path)
	settings, err := s.SetEnabled(true)
	require.NoError(t, err)
	require.Len(t, settings.Key, 26)
	assert.True(t, s.Allows(settings.Key))
	assert.False(t, s.Allows(""))
	assert.False(t, s.Allows("wrong"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Equal(t, settings, Open(path).Get())

	// Off and on again keeps the same key
	_, err = s.SetEnabled(false)
	require.NoError(t, err)
	assert.True(t, s.Allows(""))
	again, err := s.SetEnabled(true)
	require.NoError(t, err)
	assert.Equal(t, settings.Key, again.Key)
}

func TestRegenerate(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "stream_auth.json"))
	first, err := s.SetEnabled(true)
	require.NoError(t, err)
	changed := s.Changed()
	second, err := s.Regenerate()
	require.NoError(t, err)
	assert.NotEqual(t, first.Key, second.Key)
	assert.False(t, s.Allows(first.Key))
	assert.True(t, s.Allows(second.Key))
	select {
	case <-changed:
	default:
		t.Fatal("a change is announced")
	}
}

// A file that says enabled but has no key would lock everyone out
func TestEnabledWithoutKeyIsOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stream_auth.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"enabled":true}`), 0o600))
	assert.False(t, Open(path).Get().Enabled)
}
