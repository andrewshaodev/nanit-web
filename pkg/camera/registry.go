package camera

import (
	"sync"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// Registry - the account's cameras, each running while it's listed
type Registry struct {
	opts Options
	deps Deps

	mu      sync.Mutex
	cameras map[string]*running
}

type running struct {
	camera *Camera
	runner utils.GracefulRunner
}

// NewRegistry - no cameras yet; Sync starts them
func NewRegistry(opts Options, deps Deps) *Registry {
	return &Registry{opts: opts, deps: deps, cameras: map[string]*running{}}
}

// Sync starts a camera, under ctx, for each baby that doesn't have one
// running, and stops those whose baby is no longer listed. Calling it again
// with the same babies changes nothing: signing in to Nanit a second time
// used to start every camera again.
func (r *Registry) Sync(ctx utils.GracefulContext, babies []baby.Baby) {
	listed := map[string]bool{}
	var stale []*running

	r.mu.Lock()
	for _, b := range babies {
		listed[b.UID] = true
		if _, ok := r.cameras[b.UID]; ok {
			continue
		}
		cam := New(b, r.opts, r.deps)
		r.cameras[b.UID] = &running{camera: cam, runner: ctx.RunAsChild(cam.Run)}
		log.Info().Str("baby_uid", b.UID).Str("name", b.Name).Msg("Started monitoring baby")
	}
	for uid, cam := range r.cameras {
		if !listed[uid] {
			stale = append(stale, cam)
			delete(r.cameras, uid)
		}
	}
	r.mu.Unlock()

	// Outside the lock: stopping waits for the camera's cleanup
	for _, cam := range stale {
		log.Info().Str("baby_uid", cam.camera.UID()).Msg("Baby no longer on the account, stopping its camera")
		cam.runner.Cancel()
	}
}

// Get - the camera for babyUID, if it's running
func (r *Registry) Get(babyUID string) (*Camera, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cam, ok := r.cameras[babyUID]
	if !ok {
		return nil, false
	}
	return cam.camera, true
}

// StopAll stops every camera, waiting for each to clean up (on signing out
// of Nanit, say)
func (r *Registry) StopAll() {
	r.mu.Lock()
	all := r.cameras
	r.cameras = map[string]*running{}
	r.mu.Unlock()

	for _, cam := range all {
		cam.runner.Cancel()
	}
}
