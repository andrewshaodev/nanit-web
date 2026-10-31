package soundschedule

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ny, _ = time.LoadLocation("America/New_York")

// Wednesday 14 October 2026, at h:m
func at(day, h, m int) time.Time {
	return time.Date(2026, 10, day, h, m, 0, 0, ny)
}

func TestValidate(t *testing.T) {
	for _, s := range []Schedule{
		{Start: "19:30", End: "06:30"},
		{Track: "W", Start: "7:30", End: "06:30"},
		{Track: "W", Start: "24:00", End: "06:30"},
		{Track: "W", Start: "19:60", End: "06:30"},
		{Track: "W", Start: "19:30", End: "19:30"},
		{Track: "W", Start: "19:30", End: "06:30", Days: []int{7}},
	} {
		assert.Error(t, s.Validate(), "%+v", s)
	}
	s := Schedule{Track: " White Noise.wav ", Start: "19:30", End: "06:30", Days: []int{5, 1, 1}}
	require.NoError(t, s.Validate())
	assert.Equal(t, "White Noise.wav", s.Track)
	assert.Equal(t, []int{1, 5}, s.Days)
	all := Schedule{Track: "W", Start: "19:30", End: "06:30", Days: []int{0, 1, 2, 3, 4, 5, 6}}
	require.NoError(t, all.Validate())
	assert.Empty(t, all.Days, "every day is no days")
}

func TestWindow(t *testing.T) {
	night := Schedule{Start: "19:30", End: "06:30"}
	_, _, on := night.Window(at(14, 19, 29))
	assert.False(t, on)
	start, end, on := night.Window(at(14, 19, 30))
	require.True(t, on)
	assert.Equal(t, at(14, 19, 30), start)
	assert.Equal(t, at(15, 6, 30), end)
	start, _, on = night.Window(at(15, 3, 0))
	require.True(t, on, "past midnight")
	assert.Equal(t, at(14, 19, 30), start)
	_, _, on = night.Window(at(15, 6, 30))
	assert.False(t, on, "the end isn't in the run")

	nap := Schedule{Start: "13:00", End: "15:00"}
	_, _, on = nap.Window(at(14, 14, 0))
	assert.True(t, on)
	_, _, on = nap.Window(at(14, 16, 0))
	assert.False(t, on)

	// Started on Wednesday night only (3): on early Thursday, not Friday
	wed := Schedule{Start: "19:30", End: "06:30", Days: []int{3}}
	_, _, on = wed.Window(at(15, 3, 0))
	assert.True(t, on)
	_, _, on = wed.Window(at(16, 3, 0))
	assert.False(t, on)
	_, _, on = wed.Window(at(15, 20, 0))
	assert.False(t, on)
}

func TestStoreKeepsSchedules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	s := Open(path)
	saved, err := s.Set("b", []Schedule{{Enabled: true, Track: "W", Start: "19:30", End: "06:30"}})
	require.NoError(t, err)
	require.Len(t, saved, 1)
	assert.NotEmpty(t, saved[0].ID)
	assert.Equal(t, saved, Open(path).Get("b"))

	_, err = s.Set("b", []Schedule{{Track: "W", Start: "x", End: "06:30"}})
	assert.Error(t, err)
	assert.Equal(t, saved, s.Get("b"), "a bad schedule saves nothing")

	_, err = s.Set("b", nil)
	require.NoError(t, err)
	assert.Empty(t, Open(path).Get("b"))
}

type fakePlayer struct {
	mu       sync.Mutex
	playing  string
	plays    []int32
	stops    int
	playErrs []error
}

func (f *fakePlayer) PlaySound(track string, seconds int32) (camera.SoundStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plays = append(f.plays, seconds)
	if len(f.playErrs) > 0 {
		err := f.playErrs[0]
		f.playErrs = f.playErrs[1:]
		if err != nil {
			return camera.SoundStatus{}, err
		}
	}
	f.playing = track
	return camera.SoundStatus{}, nil
}

func (f *fakePlayer) StopSound() (camera.SoundStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.playing = ""
	return camera.SoundStatus{}, nil
}

func (f *fakePlayer) SoundStatus() (camera.SoundStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return camera.SoundStatus{Playback: &camera.SoundPlayback{Playing: f.playing != "", Track: f.playing}}, nil
}

func newRunner(t *testing.T, sched Schedule) (*Runner, *fakePlayer, *time.Time) {
	t.Helper()
	store := Open(filepath.Join(t.TempDir(), "s.json"))
	_, err := store.Set("b", []Schedule{sched})
	require.NoError(t, err)
	cam := &fakePlayer{}
	now := at(14, 12, 0)
	r := &Runner{Store: store, Camera: func(string) (Player, bool) { return cam, true }, now: func() time.Time { return now }}
	return r, cam, &now
}

func TestRunStartsOnceAndStopsAtTheEnd(t *testing.T) {
	r, cam, now := newRunner(t, Schedule{Enabled: true, Track: "W", Start: "19:30", End: "06:30"})

	r.Tick()
	assert.Empty(t, cam.plays, "not yet")

	*now = at(14, 19, 30)
	r.Tick()
	r.Tick()
	assert.Equal(t, []int32{11 * 3600}, cam.plays, "once, with the run's length")
	assert.Equal(t, "W", cam.playing)

	// Stopped by hand: left stopped for the rest of the run
	_, _ = cam.StopSound()
	*now = at(14, 22, 0)
	r.Tick()
	assert.Len(t, cam.plays, 1)

	// Played again by hand, then the end stops it
	_, _ = cam.PlaySound("W", -1)
	*now = at(15, 6, 30)
	r.Tick()
	assert.Equal(t, "", cam.playing)
	assert.Equal(t, 2, cam.stops)
}

func TestEndLeavesAnotherSoundPlaying(t *testing.T) {
	r, cam, now := newRunner(t, Schedule{Enabled: true, Track: "W", Start: "13:00", End: "15:00"})
	*now = at(14, 13, 0)
	r.Tick()
	_, _ = cam.PlaySound("Birds", -1)
	*now = at(14, 15, 0)
	r.Tick()
	assert.Equal(t, "Birds", cam.playing)
	assert.Equal(t, 0, cam.stops)
}

func TestLateStartAndRetry(t *testing.T) {
	r, cam, now := newRunner(t, Schedule{Enabled: true, Track: "W", Start: "13:00", End: "15:00"})
	cam.playErrs = []error{camera.ErrNotConnected}
	*now = at(14, 14, 0)
	r.Tick()
	assert.Empty(t, cam.playing, "the camera was offline")
	*now = at(14, 14, 30)
	r.Tick()
	assert.Equal(t, "W", cam.playing, "started late, once it's back")
	assert.Equal(t, int32(1800), cam.plays[len(cam.plays)-1])
}

func TestLongTimerRefusedPlaysUntilStopped(t *testing.T) {
	r, cam, now := newRunner(t, Schedule{Enabled: true, Track: "W", Start: "13:00", End: "15:00"})
	cam.playErrs = []error{errors.New("camera answered 400")}
	*now = at(14, 13, 0)
	r.Tick()
	assert.Equal(t, []int32{7200, 0}, cam.plays)
	assert.Equal(t, "W", cam.playing)
}

func TestDisabledScheduleDoesNothing(t *testing.T) {
	r, cam, now := newRunner(t, Schedule{Enabled: false, Track: "W", Start: "13:00", End: "15:00"})
	*now = at(14, 13, 0)
	r.Tick()
	assert.Empty(t, cam.plays)
}
