// Package soundschedule - playing the camera's sounds (white noise...)
// between set times of day, such as 7:30 PM to 6:30 AM, set from the
// dashboard. The bridge runs them, so they work with no dashboard open.
//
// A schedule starts its sound once per run: when its start time comes, or,
// if the bridge or camera was down then, as soon as it can during the run.
// Stopping the sound by hand during a run is respected; it isn't started
// again until the next run. At the end time it's stopped, if it's still the
// schedule's sound playing. The camera is also given the run's length, so
// it stops by itself even if the bridge is down by then.
package soundschedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// TimeZone - the bridge's time zone, by name where it's known (TZ), and
// its offset from UTC now in minutes
func TimeZone(now time.Time) (string, int) {
	abbr, offset := now.Zone()
	if tz := os.Getenv("TZ"); tz != "" {
		return tz, offset / 60
	}
	return abbr, offset / 60
}

// maxSchedules - per camera, more than anyone needs
const maxSchedules = 20

// clock - "HH:MM" as hours and minutes
func clock(s string) (h, m int, err error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, fmt.Errorf("%q isn't a time like 19:30", s)
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil || h > 23 || m > 59 || h < 0 || m < 0 {
		return 0, 0, fmt.Errorf("%q isn't a time like 19:30", s)
	}
	return h, m, nil
}

// Validate checks the schedule, and tidies its days
func (s *Schedule) Validate() error {
	s.Track = strings.TrimSpace(s.Track)
	if s.Track == "" {
		return errors.New("a schedule needs a sound")
	}
	if _, _, err := clock(s.Start); err != nil {
		return err
	}
	if _, _, err := clock(s.End); err != nil {
		return err
	}
	if s.Start == s.End {
		return errors.New("a schedule's start and end are the same time")
	}
	days := []int{}
	for _, d := range s.Days {
		if d < 0 || d > 6 {
			return fmt.Errorf("%d isn't a day of the week (0-6)", d)
		}
		if !slices.Contains(days, d) {
			days = append(days, d)
		}
	}
	slices.Sort(days)
	if len(days) == 7 {
		days = []int{}
	}
	s.Days = days
	return nil
}

func (s Schedule) startsOn(d time.Weekday) bool {
	return len(s.Days) == 0 || slices.Contains(s.Days, int(d))
}

// Window - the run of the schedule that's on at now, if one is: it began
// on today's or yesterday's start time
func (s Schedule) Window(now time.Time) (start, end time.Time, ok bool) {
	sh, sm, err1 := clock(s.Start)
	eh, em, err2 := clock(s.End)
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, false
	}
	for _, back := range []int{0, -1} {
		d := now.AddDate(0, 0, back)
		start = time.Date(d.Year(), d.Month(), d.Day(), sh, sm, 0, 0, now.Location())
		end = time.Date(d.Year(), d.Month(), d.Day(), eh, em, 0, 0, now.Location())
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
		if !start.After(now) && now.Before(end) && s.startsOn(start.Weekday()) {
			return start, end, true
		}
	}
	return time.Time{}, time.Time{}, false
}

// run - a schedule's latest run that the bridge started
type run struct {
	Track string `json:"track"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	// Stopped - the end has been dealt with
	Stopped bool `json:"stopped"`
}

// file - what's kept on disk
type file struct {
	// Babies - each camera's schedules, by baby UID
	Babies map[string][]Schedule `json:"babies"`
	// Runs - each schedule's latest run, by schedule ID, so a restart
	// neither starts a run twice nor forgets to end one
	Runs map[string]run `json:"runs"`
}

// Store - the schedules, kept in a file
type Store struct {
	path string

	mu   sync.Mutex
	data file
}

// Open loads the schedules from path; a missing file means none
func Open(path string) *Store {
	s := &Store{path: path, data: file{Babies: map[string][]Schedule{}, Runs: map[string]run{}}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Error().Err(err).Str("path", path).Msg("Failed to read the sound schedules")
		}
		return s
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read the sound schedules")
	}
	if s.data.Babies == nil {
		s.data.Babies = map[string][]Schedule{}
	}
	if s.data.Runs == nil {
		s.data.Runs = map[string]run{}
	}
	return s
}

// Get - a camera's schedules
func (s *Store) Get(babyUID string) []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.data.Babies[babyUID])
}

// Set replaces a camera's schedules, after checking them. New ones get an
// ID. Returns them as saved.
func (s *Store) Set(babyUID string, schedules []Schedule) ([]Schedule, error) {
	if len(schedules) > maxSchedules {
		return nil, fmt.Errorf("a camera can have at most %d schedules", maxSchedules)
	}
	schedules = clone(schedules)
	for i := range schedules {
		if err := schedules[i].Validate(); err != nil {
			return nil, err
		}
		if schedules[i].ID == "" {
			schedules[i].ID = newID()
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	next := file{Babies: map[string][]Schedule{}, Runs: s.data.Runs}
	for uid, list := range s.data.Babies {
		next.Babies[uid] = list
	}
	if len(schedules) == 0 {
		delete(next.Babies, babyUID)
	} else {
		next.Babies[babyUID] = schedules
	}
	if err := s.saveLocked(next); err != nil {
		return nil, err
	}
	return clone(schedules), nil
}

func (s *Store) all() map[string][]Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := map[string][]Schedule{}
	for uid, list := range s.data.Babies {
		all[uid] = clone(list)
	}
	return all
}

func (s *Store) run(id string) (run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.Runs[id]
	return r, ok
}

// setRun records a schedule's run, and drops those of schedules that are
// gone
func (s *Store) setRun(id string, r run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := file{Babies: s.data.Babies, Runs: map[string]run{}}
	ids := map[string]bool{}
	for _, list := range s.data.Babies {
		for _, sched := range list {
			ids[sched.ID] = true
		}
	}
	for k, v := range s.data.Runs {
		if ids[k] {
			next.Runs[k] = v
		}
	}
	next.Runs[id] = r
	if err := s.saveLocked(next); err != nil {
		log.Error().Err(err).Msg("Failed to save the sound schedules")
	}
}

func (s *Store) saveLocked(next file) error {
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := utils.WriteFileAtomic(s.path, raw); err != nil {
		return err
	}
	s.data = next
	return nil
}

func clone(list []Schedule) []Schedule {
	out := make([]Schedule, len(list))
	for i, sched := range list {
		sched.Days = slices.Clone(sched.Days)
		if sched.Days == nil {
			sched.Days = []int{}
		}
		out[i] = sched
	}
	return out
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
