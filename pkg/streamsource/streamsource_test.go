package streamsource

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURL(t *testing.T) {
	s := Source{Host: "monitor.homelab.house", Port: 8554}
	assert.Equal(t, "rtsp://monitor.homelab.house:8554/local/bb36bfe3", s.URL("bb36bfe3"))
	s.Key = "KEY"
	assert.Equal(t, "rtsp://nanit:KEY@monitor.homelab.house:8554/local/bb36bfe3", s.URL("bb36bfe3"))
	assert.Equal(t, "rtsp://nanit:xxxxx@monitor.homelab.house:8554/local/{baby_uid}", s.Redacted("{baby_uid}"))
	assert.Equal(t, "rtsp://[fd00::1]:8554/local/x", Source{Host: "fd00::1"}.URL("x"), "IPv6, default port")
}

func TestValidate(t *testing.T) {
	for _, host := range []string{"rtsp://host", "host:8554", "host/path", "a b"} {
		s := Source{Host: host}
		assert.Error(t, s.Validate(), host)
	}
	s := Source{Host: "  monitor.homelab.house ", Key: " k "}
	require.NoError(t, s.Validate())
	assert.Equal(t, Source{Host: "monitor.homelab.house", Port: DefaultPort, Key: "k"}, s)
	assert.Error(t, (&Source{Host: "h", Port: 70000}).Validate())
	empty := Source{Port: 1, Key: "k"}
	require.NoError(t, empty.Validate())
	assert.Equal(t, Source{}, empty, "no host clears the rest")
}

func TestSaveAndStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stream_source.json")
	s := Open(path)
	assert.False(t, s.Get().Configured())

	s.SetStatus("b", false, errors.New("bad status code: 401 (Unauthorized)"))
	changed := s.Changed()
	saved, err := s.Set(Source{Host: "h", Key: "k"})
	require.NoError(t, err)
	assert.Equal(t, Source{Host: "h", Port: DefaultPort, Key: "k"}, saved)
	assert.Equal(t, saved, Open(path).Get(), "kept through a restart")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	select {
	case <-changed:
	default:
		t.Fatal("a change is announced")
	}
	_, ok := s.Status("b")
	assert.False(t, ok, "statuses are for the old server")

	s.SetStatus("b", false, errors.New("bad status code: 401 (Unauthorized)"))
	st, _ := s.Status("b")
	assert.Contains(t, st.Error, "key")
}
