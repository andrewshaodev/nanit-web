// Package oidcauth - signing in to the dashboard through an OpenID Connect
// provider, such as Pocket ID. The provider decides who may sign in (Pocket
// ID's allowed user groups on the client, say); anyone it signs in gets a
// dashboard session.
//
// It's set up from the dashboard's settings and kept in a file in the data
// folder. New settings are a draft until a test sign-in with them works, and
// only then turn SSO on, so a typo can't lock everyone out.
//
// The flow is the authorization code flow with PKCE. Each sign-in's state,
// nonce and PKCE verifier are kept here for a few minutes, and the state is
// also put in a cookie, so the callback must come back to the browser that
// started it.
package oidcauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/andrewshaodev/nanit-web/pkg/utils"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// CallbackPath - where the provider sends the browser back to. It's the
// redirect URI to register with the provider, after the dashboard's address.
const CallbackPath = "/api/webauth/oidc/callback"

// loginTimeout - how long a sign-in can take at the provider
const loginTimeout = 10 * time.Minute

// Config - a provider, and the dashboard's client at it
type Config struct {
	// Issuer - the provider's issuer URL, e.g. https://id.example.com
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// Validate tidies the fields and checks them
func (c *Config) Validate() error {
	c.Issuer = strings.TrimRight(strings.TrimSpace(c.Issuer), "/")
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	u, err := url.Parse(c.Issuer)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("the issuer is a URL such as https://id.example.com")
	}
	if c.ClientID == "" {
		return errors.New("the client ID is missing")
	}
	return nil
}

// Identity - who signed in
type Identity struct {
	Subject string
	Email   string
	Name    string
}

// file - what's kept on disk
type file struct {
	// Active - what signs people in; nil while SSO is off
	Active *Config `json:"active"`
	// Draft - settings saved from the dashboard, not yet proven by a test
	// sign-in
	Draft Config `json:"draft"`
}

// pending - a sign-in waiting for its callback
type pending struct {
	config      Config
	test        bool
	nonce       string
	verifier    string
	redirectURL string
	returnTo    string
	expires     time.Time
}

// SSO - the sign-in settings, and sign-ins in progress
type SSO struct {
	path string

	mu       sync.Mutex
	settings file
	// Each issuer's discovery document, fetched on first use: the provider
	// may be down when the bridge starts
	providers map[string]*oidc.Provider
	pending   map[string]pending
}

// Open loads the settings from path. A missing file means SSO is off.
func Open(path string) *SSO {
	s := &SSO{path: path, providers: map[string]*oidc.Provider{}, pending: map[string]pending{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Error().Err(err).Str("path", path).Msg("Failed to read the SSO settings")
		}
		return s
	}
	if err := json.Unmarshal(data, &s.settings); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read the SSO settings")
		s.settings = file{}
	}
	return s
}

// Enabled - whether SSO signs people in
func (s *SSO) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Active != nil
}

// Settings - what's active (nil while off) and the draft, with the client
// secrets left out
func (s *SSO) Settings() (active *Config, draft Config, draftSecretSet bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings.Active != nil {
		a := *s.settings.Active
		a.ClientSecret = ""
		active = &a
	}
	draft = s.settings.Draft
	draftSecretSet = draft.ClientSecret != ""
	draft.ClientSecret = ""
	return active, draft, draftSecretSet
}

// SaveDraft checks next and saves it as the draft, after checking the
// provider answers at its issuer. A blank secret keeps the saved one.
func (s *SSO) SaveDraft(ctx context.Context, next Config) error {
	if err := next.Validate(); err != nil {
		return err
	}
	if _, err := s.provider(ctx, next.Issuer); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if next.ClientSecret == "" {
		next.ClientSecret = s.settings.Draft.ClientSecret
	}
	settings := s.settings
	settings.Draft = next
	return s.saveLocked(settings)
}

// Disable turns SSO off. The draft is kept, to turn it back on.
func (s *SSO) Disable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings := s.settings
	settings.Active = nil
	return s.saveLocked(settings)
}

func (s *SSO) saveLocked(settings file) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	// Holds the client secret: 0600, as the Nanit session is
	if err := utils.WriteFileAtomic(s.path, data); err != nil {
		return err
	}
	s.settings = settings
	return nil
}

// provider - the issuer's endpoints and keys
func (s *SSO) provider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	s.mu.Lock()
	p, ok := s.providers[issuer]
	s.mu.Unlock()
	if ok {
		return p, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach the sign-in provider at %s: %w", issuer, err)
	}
	s.mu.Lock()
	s.providers[issuer] = p
	s.mu.Unlock()
	return p, nil
}

func oauthConfig(c Config, p *oidc.Provider, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

// RedirectURL - the callback URL for a dashboard reached at origin (such
// as https://monitor.example.com)
func RedirectURL(origin string) string {
	return strings.TrimRight(origin, "/") + CallbackPath
}

// Start begins a sign-in: the provider's URL to send the browser to, and
// the state to keep in its cookie. returnTo is the dashboard page to go back
// to afterwards. A test sign-in uses the draft, and turns it on if it works.
func (s *SSO) Start(ctx context.Context, redirectURL, returnTo string, test bool) (authURL, state string, err error) {
	s.mu.Lock()
	var config Config
	switch {
	case test:
		config = s.settings.Draft
	case s.settings.Active != nil:
		config = *s.settings.Active
	}
	s.mu.Unlock()
	if config.ClientID == "" {
		return "", "", errors.New("SSO isn't set up")
	}

	p, err := s.provider(ctx, config.Issuer)
	if err != nil {
		return "", "", err
	}
	state, nonce := randomString(), randomString()
	verifier := oauth2.GenerateVerifier()

	s.mu.Lock()
	now := time.Now()
	for k, pend := range s.pending {
		if now.After(pend.expires) {
			delete(s.pending, k)
		}
	}
	s.pending[state] = pending{config: config, test: test, nonce: nonce, verifier: verifier, redirectURL: redirectURL, returnTo: returnTo, expires: now.Add(loginTimeout)}
	s.mu.Unlock()

	authURL = oauthConfig(config, p, redirectURL).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	return authURL, state, nil
}

// Result - how a sign-in ended
type Result struct {
	Identity Identity
	// ReturnTo - the dashboard page to go back to
	ReturnTo string
	// Test - it was a test of the draft, which is now on if it worked
	Test bool
}

// Finish completes the sign-in the provider sent back with state and code,
// checking the ID token. A test sign-in that works turns its settings on.
func (s *SSO) Finish(ctx context.Context, state, code string) (Result, error) {
	s.mu.Lock()
	pend, ok := s.pending[state]
	delete(s.pending, state)
	s.mu.Unlock()
	if !ok || time.Now().After(pend.expires) {
		return Result{}, errors.New("the sign-in expired or wasn't started here")
	}
	res := Result{ReturnTo: pend.returnTo, Test: pend.test}

	who, err := s.exchange(ctx, pend, code)
	if err != nil {
		return res, err
	}
	res.Identity = who

	if pend.test {
		s.mu.Lock()
		defer s.mu.Unlock()
		// The settings tested, even if the draft has changed since
		settings := s.settings
		active := pend.config
		settings.Active = &active
		if err := s.saveLocked(settings); err != nil {
			return res, fmt.Errorf("saving the SSO settings: %w", err)
		}
	}
	return res, nil
}

func (s *SSO) exchange(ctx context.Context, pend pending, code string) (Identity, error) {
	p, err := s.provider(ctx, pend.config.Issuer)
	if err != nil {
		return Identity{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	token, err := oauthConfig(pend.config, p, pend.redirectURL).Exchange(ctx, code, oauth2.VerifierOption(pend.verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchanging the code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("the provider sent no ID token")
	}
	idToken, err := p.Verifier(&oidc.Config{ClientID: pend.config.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("checking the ID token: %w", err)
	}
	if idToken.Nonce != pend.nonce {
		return Identity{}, errors.New("the ID token is for a different sign-in")
	}

	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	_ = idToken.Claims(&claims)
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	return Identity{Subject: idToken.Subject, Email: claims.Email, Name: name}, nil
}

func randomString() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
