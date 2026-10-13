package history

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A database from before spells kept their camera gets the column, and the
// next message about a spell fills it in
func TestCriesCameraAddedToOldDatabase(t *testing.T) {
	dir := t.TempDir()
	old, err := sql.Open("sqlite3", filepath.Join(dir, "history.db"))
	require.NoError(t, err)
	_, err = old.Exec(`
		CREATE TABLE cries (review_id TEXT PRIMARY KEY, baby_uid TEXT NOT NULL, start_time REAL NOT NULL, end_time REAL, created_at INTEGER);
		INSERT INTO cries (review_id, baby_uid, start_time) VALUES ('a', 'baby1', 10);
	`)
	require.NoError(t, err)
	require.NoError(t, old.Close())

	tracker, err := NewTracker(dir, true)
	require.NoError(t, err)
	defer tracker.Close()

	c, ok, err := tracker.GetCry("a")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Empty(t, c.Camera)

	require.NoError(t, tracker.RecordCry(Cry{ReviewID: "a", BabyUID: "baby1", Camera: "nursery", Start: 10}))
	c, _, err = tracker.GetCry("a")
	require.NoError(t, err)
	assert.Equal(t, "nursery", c.Camera)

	// Opening it again leaves it be
	require.NoError(t, tracker.Close())
	tracker, err = NewTracker(dir, true)
	require.NoError(t, err)
	defer tracker.Close()
}
