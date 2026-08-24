package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeToken - builds a JWT-shaped token carrying the given claims. Only the
// payload segment is ever read, so the other two are filler.
func makeToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	return fmt.Sprintf("header.%s.signature", base64.RawURLEncoding.EncodeToString(payload))
}

// TestTokenExpiryReadsRealClaim - shaped after an observed token: a three hour
// lifetime, which is three times the hour this package used to assume.
func TestTokenExpiryReadsRealClaim(t *testing.T) {
	issued := time.Now()
	expires := issued.Add(3 * time.Hour)

	token := makeToken(t, map[string]interface{}{
		"payload": map[string]interface{}{"c": true, "i": 1554304, "v": 1},
		"iat":     issued.Unix(),
		"exp":     expires.Unix(),
	})

	expiry, ok := tokenExpiry(token)
	require.True(t, ok)
	assert.Equal(t, expires.Unix(), expiry.Unix())
}

func TestTokenExpiryRejectsNonTokens(t *testing.T) {
	for name, token := range map[string]string{
		"empty":            "",
		"opaque":           "not-a-jwt",
		"too few segments": "header.payload",
		"bad base64":       "header.!!!not-base64!!!.signature",
		"payload not json": makeTokenRaw("bm90IGpzb24"),
		"no exp claim":     `header.eyJpYXQiOjE3ODc1NTA1Njl9.signature`,
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := tokenExpiry(token)
			assert.False(t, ok)
		})
	}
}

func makeTokenRaw(payload string) string {
	return fmt.Sprintf("header.%s.signature", payload)
}

// TestAuthTokenExpiryPrefersTheClaim - the stated expiry must win over the
// assumed lifetime, otherwise reading the claim buys nothing.
func TestAuthTokenExpiryPrefersTheClaim(t *testing.T) {
	issued := time.Now()
	stated := issued.Add(3 * time.Hour)

	token := makeToken(t, map[string]interface{}{"exp": stated.Unix()})

	assert.Equal(t, stated.Unix(), authTokenExpiry(token, issued).Unix())
}

// TestAuthTokenExpiryFallsBack - a token that states nothing degrades to the
// old assumption rather than being treated as immortal.
func TestAuthTokenExpiryFallsBack(t *testing.T) {
	issued := time.Now()

	expiry := authTokenExpiry("opaque-token", issued)

	assert.Equal(t, issued.Add(AuthTokenTimelife).Unix(), expiry.Unix())
}

// TestExpiredTokenIsAlreadyStale - a token past its expiry must land inside the
// renewal margin, which is what makes MaybeAuthorize refresh it.
func TestExpiredTokenIsAlreadyStale(t *testing.T) {
	issued := time.Now().Add(-4 * time.Hour)
	token := makeToken(t, map[string]interface{}{"exp": issued.Add(3 * time.Hour).Unix()})

	assert.True(t, time.Until(authTokenExpiry(token, issued)) < AuthTokenRenewMargin)
}
