package client

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// tokenExpiry - reads the expiry a token states about itself.
//
// Nanit's access token is a JWT carrying an `exp` claim, which beats assuming a
// lifetime: the hour this package assumed for years is a third of the three
// hours a real token actually states. The claims are only read for scheduling,
// never trusted for authorization - the server decides that - so the signature
// is deliberately not verified.
func tokenExpiry(authToken string) (time.Time, bool) {
	parts := strings.Split(authToken, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	// JWT payloads are base64url without padding.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}

	return time.Unix(claims.Exp, 0), true
}

// authTokenExpiry - when the held token stops being usable.
//
// Falls back to the assumed lifetime for anything that does not state one, so a
// change in token format degrades to the old behaviour rather than to a token
// treated as immortal.
func authTokenExpiry(authToken string, authTime time.Time) time.Time {
	if expiry, ok := tokenExpiry(authToken); ok {
		return expiry
	}

	return authTime.Add(AuthTokenTimelife)
}
