package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
