package soundschedule

import (
	"errors"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/camera"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

const (
	// tickInterval - how often the schedules are checked
	tickInterval = 15 * time.Second
	// minCatchUp - a run with less than this left isn't started late
	minCatchUp = time.Minute
	// stopGiveUp - how long after its end a run's stop is still tried,
	// while the camera is unreachable
	stopGiveUp = 30 * time.Minute
)

// Player - the camera commands a schedule uses
type Player interface {
	PlaySound(track string, seconds int32) (camera.SoundStatus, error)
	StopSound() (camera.SoundStatus, error)
	SoundStatus() (camera.SoundStatus, error)
}

// Runner - starts and stops the schedules' sounds
type Runner struct {
	Store *Store
	// Camera - a baby's camera, if the bridge has it
	Camera func(babyUID string) (Player, bool)

	now func() time.Time // time.Now; tests set the clock
}

// Run checks the schedules until ctx ends
func (r *Runner) Run(ctx utils.GracefulContext) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		r.Tick()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick - starts the runs that are due, and ends those that are over
func (r *Runner) Tick() {
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	for uid, list := range r.Store.all() {
		for _, sched := range list {
			r.check(uid, sched, now)
		}
	}
}

func (r *Runner) check(uid string, sched Schedule, now time.Time) {
	sublog := log.With().Str("baby_uid", uid).Str("schedule", sched.ID).Str("track", sched.Track).Logger()
	last, hadRun := r.Store.run(sched.ID)

	// The last run's end, whether or not the schedule is still on: turning
	// it off doesn't leave its sound playing past the end
	if hadRun && !last.Stopped && !now.Before(time.Unix(last.End, 0)) {
		if now.Sub(time.Unix(last.End, 0)) > stopGiveUp {
			sublog.Warn().Msg("Gave up stopping a scheduled sound: the camera was unreachable")
			last.Stopped = true
			r.Store.setRun(sched.ID, last)
		} else if cam, ok := r.Camera(uid); ok {
			status, err := cam.SoundStatus()
			if err == nil {
				// Only the schedule's own sound: something else played since
				// is left alone
				if p := status.Playback; p != nil && p.Playing && p.Track == last.Track {
					if _, err = cam.StopSound(); err == nil {
						sublog.Info().Msg("Scheduled sound stopped")
					}
				}
			}
			if err != nil {
				sublog.Warn().Err(err).Msg("Couldn't stop a scheduled sound, will retry")
			} else {
				last.Stopped = true
				r.Store.setRun(sched.ID, last)
			}
		}
	}

	if !sched.Enabled {
		return
	}
	start, end, on := sched.Window(now)
	if !on || (hadRun && last.Start == start.Unix()) || end.Sub(now) < minCatchUp {
		return
	}
	cam, ok := r.Camera(uid)
	if !ok {
		return
	}
	seconds := int32(end.Sub(now) / time.Second)
	_, err := cam.PlaySound(sched.Track, seconds)
	if err != nil && !errors.Is(err, camera.ErrNotConnected) {
		// In case the camera won't take so long a timer: played until
		// stopped, which the end does
		sublog.Warn().Err(err).Int32("seconds", seconds).Msg("Camera refused the scheduled sound's length, playing it until stopped")
		_, err = cam.PlaySound(sched.Track, 0)
	}
	if err != nil {
		sublog.Warn().Err(err).Msg("Couldn't start a scheduled sound, will retry")
		return
	}
	sublog.Info().Time("until", end).Msg("Scheduled sound started")
	r.Store.setRun(sched.ID, run{Track: sched.Track, Start: start.Unix(), End: end.Unix()})
}
