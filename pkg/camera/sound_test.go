package camera

import (
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
