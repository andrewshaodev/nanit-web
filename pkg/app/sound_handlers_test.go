package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// No duration, or a nonsensical one, means "keep playing", as the Nanit app
// loops sounds. Only a positive number of seconds sets a timer.
func TestPlayDuration(t *testing.T) {
	assert.Equal(t, soundDurationForever, playDuration(0))
	assert.Equal(t, soundDurationForever, playDuration(-5))
	assert.Equal(t, int32(1800), playDuration(1800))
}

func TestClampVolume(t *testing.T) {
	assert.Equal(t, int32(0), clampVolume(-10))
	assert.Equal(t, int32(40), clampVolume(40))
	assert.Equal(t, int32(100), clampVolume(250))
}

func TestSoundAPIRequiresAConnectedCamera(t *testing.T) {
	app := &App{}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/sound/baby1", "", http.StatusServiceUnavailable},
		{"POST", "/api/sound/baby1/play", `{"track":"White Noise.wav"}`, http.StatusServiceUnavailable},
		{"GET", "/api/sound/", "", http.StatusNotFound},
		{"GET", "/api/sound/baby1/play/extra", "", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		handleSoundAPI(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)), app)
		assert.Equal(t, tc.want, w.Code, "%s %s", tc.method, tc.path)
	}
}
