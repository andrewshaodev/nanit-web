package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// apiBaseURL - Nanit's REST API. Tests point it at a stand-in server.
var apiBaseURL = "https://api.nanit.com"

// Nanit's /login answers 201 once signed in, and 482 when it has sent a code
// that has to come back with a second request
const (
	loginStatusSignedIn    = http.StatusCreated
	loginStatusMFARequired = 482
)

// LoginChallenge - where Nanit sent the sign-in code. Pass MFAToken and
// Channel back to FinishLogin with the code.
type LoginChallenge struct {
	MFAToken    string
	Channel     string // "sms" or "email"
	PhoneSuffix string
}

// LoginError - Nanit turned the sign-in down (a wrong password or code, say)
type LoginError struct {
	Status  int
	Message string
}

func (e *LoginError) Error() string {
	return e.Message
}

type loginResponse struct {
	authResponsePayload
	MFAToken    string `json:"mfa_token"`
	Channel     string `json:"channel"`
	PhoneSuffix string `json:"phone_suffix"`
	Error       string `json:"error"`
}

// StartLogin - signs in with an email and password. Nanit normally sends a
// code by text or email and returns where it went. An account without 2FA
// is signed in straight away, and then the challenge is nil.
func (c *NanitClient) StartLogin(email, password string) (*LoginChallenge, error) {
	status, res, err := postLogin(map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return nil, err
	}

	switch status {
	case loginStatusMFARequired:
		log.Info().Str("channel", res.Channel).Str("phone_suffix", res.PhoneSuffix).Msg("Nanit sent a sign-in code")
		return &LoginChallenge{MFAToken: res.MFAToken, Channel: res.Channel, PhoneSuffix: res.PhoneSuffix}, nil
	case loginStatusSignedIn:
		return nil, c.storeLogin(res)
	default:
		return nil, loginError(status, res, "Login failed")
	}
}

// FinishLogin - completes a sign-in with the code Nanit sent, and saves the
// session
func (c *NanitClient) FinishLogin(email, password, mfaToken, code, channel string) error {
	if channel == "" {
		channel = "email" // clients from before the channel was passed through
	}
	status, res, err := postLogin(map[string]string{
		"email":     email,
		"password":  password,
		"mfa_token": mfaToken,
		// A string, so a code like "0123" keeps its leading zero
		"mfa_code": code,
		"channel":  channel,
	})
	if err != nil {
		return err
	}
	if status != loginStatusSignedIn {
		return loginError(status, res, "Verification failed")
	}
	return c.storeLogin(res)
}

// storeLogin keeps the tokens from a completed sign-in. The dashboard's
// sign-in used to write the session file itself, with the MFA token as the
// access token, and it only worked because a forced refresh replaced it.
func (c *NanitClient) storeLogin(res loginResponse) error {
	if res.AccessToken == "" || res.RefreshToken == "" {
		return fmt.Errorf("nanit signed in but sent no tokens")
	}
	c.SessionStore.StoreCredentials(res.AccessToken, res.RefreshToken, time.Now())
	if err := c.SessionStore.Save(); err != nil {
		return fmt.Errorf("failed to save the Nanit session: %w", err)
	}
	log.Info().Msg("Signed in to Nanit")
	return nil
}

func loginError(status int, res loginResponse, fallback string) error {
	msg := res.Error
	if msg == "" {
		msg = fallback
	}
	log.Error().Int("status_code", status).Str("error", msg).Msg("Nanit rejected the sign-in")
	return &LoginError{Status: status, Message: msg}
}

// postLogin sends body to /login. The body carries the password (and the
// code), so it is never logged, and nor is the response, which carries the
// tokens.
func postLogin(body map[string]string) (int, loginResponse, error) {
	var res loginResponse

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, res, fmt.Errorf("failed to encode the sign-in request: %w", err)
	}
	req, err := http.NewRequest("POST", apiBaseURL+"/login", bytes.NewReader(payload))
	if err != nil {
		return 0, res, fmt.Errorf("failed to create the sign-in request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Without it, an account with 2FA is refused outright
	req.Header.Set("nanit-api-version", "1")

	r, err := myClient.Do(req)
	if err != nil {
		return 0, res, fmt.Errorf("failed to reach Nanit: %w", err)
	}
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		return r.StatusCode, res, fmt.Errorf("unexpected sign-in response from Nanit (status %d): %w", r.StatusCode, err)
	}
	return r.StatusCode, res, nil
}
