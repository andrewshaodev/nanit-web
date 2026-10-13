// Package frigate - crying spells from Frigate's audio detection, kept in the
// history database, and their recordings for the dashboard.
//
// Frigate turns each crying spell into one review item. Its MQTT topic
// <prefix>/reviews sends the item when it opens, as it goes on and when it
// ends, and every message is saved as it comes, so a missed one is filled in
// by the next. The review API fills in anything missed while the bridge or
// the broker was down.
package frigate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/baby"
	"github.com/andrewshaodev/nanit-web/pkg/history"
	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/rs/zerolog/log"
)

// CryLabel - Frigate's audio label for crying
const CryLabel = "crying"

// ClipPadding - seconds of recording kept either side of a spell's clip, so
// it starts before the first cry
const ClipPadding = 30

// How often the review API is checked for spells MQTT missed, and how far
// back. At start the whole of Frigate's usual retention is checked.
const (
	syncInterval  = 5 * time.Minute
	syncLookback  = time.Hour
	startLookback = 14 * 24 * time.Hour
)

// Opts - where Frigate is, and which of its cameras is which baby
type Opts struct {
	// URL - Frigate's API, such as http://frigate:5000
	URL string
	// TopicPrefix - Frigate's MQTT topic prefix
	TopicPrefix string
	// Cameras - Frigate camera name to baby UID
	Cameras map[string]string
}

// ParseCameras reads "nursery=bb36bfe3,travel=380fb4f2": Frigate camera
// names and the babies they watch
func ParseCameras(s string) (map[string]string, error) {
	cameras := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, uid, ok := strings.Cut(pair, "=")
		name, uid = strings.TrimSpace(name), strings.TrimSpace(uid)
		if !ok || name == "" {
			return nil, fmt.Errorf("%q isn't camera=baby_uid", pair)
		}
		if err := baby.EnsureValidBabyUID(uid); err != nil {
			return nil, fmt.Errorf("%q: %w", pair, err)
		}
		cameras[name] = uid
	}
	if len(cameras) == 0 {
		return nil, fmt.Errorf("no cameras given")
	}
	return cameras, nil
}

// Review - a review item, as the API and MQTT send it
type Review struct {
	ID        string   `json:"id"`
	Camera    string   `json:"camera"`
	StartTime float64  `json:"start_time"`
	EndTime   *float64 `json:"end_time"`
	Severity  string   `json:"severity"`
	Data      struct {
		Audio []string `json:"audio"`
	} `json:"data"`
}

// reviewMessage - one message on <prefix>/reviews
type reviewMessage struct {
	Type  string `json:"type"` // new, update or end
	After Review `json:"after"`
}

// reviewIDPattern - the characters Frigate's ids use. The id goes into the
// API's paths and the clip cache's file names.
var reviewIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// ValidReviewID - whether id looks like one of Frigate's
func ValidReviewID(id string) bool {
	return reviewIDPattern.MatchString(id)
}

// Recorder - where spells are saved
type Recorder interface {
	RecordCry(history.Cry) error
}

// Service - Frigate's crying spells
type Service struct {
	opts   Opts
	cries  Recorder
	client *http.Client
}

// New - a service that saves spells to cries
func New(opts Opts, cries Recorder) *Service {
	opts.URL = strings.TrimRight(opts.URL, "/")
	if opts.TopicPrefix == "" {
		opts.TopicPrefix = "frigate"
	}
	return &Service{opts: opts, cries: cries, client: &http.Client{}}
}

// ReviewsTopic - the MQTT topic Frigate sends review items on
func (s *Service) ReviewsTopic() string {
	return s.opts.TopicPrefix + "/reviews"
}

// cry - the spell a review item records, if it's a crying alert on one of
// the babies' cameras
func (s *Service) cry(r Review) (history.Cry, bool) {
	uid, ok := s.opts.Cameras[r.Camera]
	if !ok || r.Severity != "alert" || !slices.Contains(r.Data.Audio, CryLabel) || !ValidReviewID(r.ID) {
		return history.Cry{}, false
	}
	return history.Cry{ReviewID: r.ID, BabyUID: uid, Start: r.StartTime, End: r.EndTime}, true
}

// HandleReview saves the spell in a message on ReviewsTopic
func (s *Service) HandleReview(payload []byte) {
	var msg reviewMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Warn().Err(err).Msg("Ignoring a Frigate review message that isn't JSON")
		return
	}
	c, ok := s.cry(msg.After)
	if !ok {
		return
	}
	if err := s.cries.RecordCry(c); err != nil {
		log.Error().Err(err).Msg("Failed to save a crying spell")
		return
	}
	log.Debug().Str("baby_uid", c.BabyUID).Str("review_id", c.ReviewID).Str("type", msg.Type).Msg("Crying spell from Frigate")
}

// Run checks the review API for spells at start, and then every few
// minutes, until ctx ends
func (s *Service) Run(ctx utils.GracefulContext) {
	reqCtx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		cancel()
	}()
	defer cancel()

	lookback := startLookback
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		if err := s.Sync(reqCtx, time.Now().Add(-lookback)); err != nil && reqCtx.Err() == nil {
			log.Warn().Err(err).Msg("Couldn't check Frigate for crying spells")
		} else {
			lookback = syncLookback
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// Sync saves the crying alerts Frigate has from after onwards
func (s *Service) Sync(ctx context.Context, after time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cameras := make([]string, 0, len(s.opts.Cameras))
	for name := range s.opts.Cameras {
		cameras = append(cameras, name)
	}
	slices.Sort(cameras)

	q := url.Values{}
	q.Set("cameras", strings.Join(cameras, ","))
	q.Set("severity", "alert")
	q.Set("after", strconv.FormatInt(after.Unix(), 10))
	q.Set("limit", "1000")
	res, err := s.get(ctx, "/api/review?"+q.Encode())
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var reviews []Review
	if err := json.NewDecoder(res.Body).Decode(&reviews); err != nil {
		return fmt.Errorf("reading Frigate's reviews: %w", err)
	}
	saved := 0
	for _, r := range reviews {
		c, ok := s.cry(r)
		if !ok {
			continue
		}
		if err := s.cries.RecordCry(c); err != nil {
			return err
		}
		saved++
	}
	log.Debug().Int("reviews", len(reviews)).Int("cries", saved).Msg("Checked Frigate for crying spells")
	return nil
}

// Clip - a spell's recording as MP4, with ClipPadding either side. The
// caller closes it.
func (s *Service) Clip(ctx context.Context, reviewID string) (io.ReadCloser, error) {
	if !ValidReviewID(reviewID) {
		return nil, fmt.Errorf("invalid review id")
	}
	res, err := s.get(ctx, fmt.Sprintf("/api/review/%s/clip.mp4?padding=%d", reviewID, ClipPadding))
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// get - a request to Frigate's API, failing on anything but a 200
func (s *Service) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.opts.URL+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("frigate answered %s", res.Status)
	}
	return res, nil
}
