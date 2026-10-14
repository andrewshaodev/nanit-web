package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/andrewshaodev/nanit-web/pkg/oidcauth"
	"github.com/rs/zerolog/log"
)

// Signing in to the dashboard through an OpenID Connect provider (SSO). The
// browser goes to /api/webauth/oidc/login, on to the provider, and back to
// the callback, which signs it in as the password does. It's set up under
// Settings, where "Test and turn on" signs in with the new settings first
// and only turns them on if that works.

// ssoStateCookie - the sign-in's state, so the callback only completes in
// the browser that started it
const ssoStateCookie = "nanit_sso_state"

func (s *Server) ssoEnabled() bool {
	return s.SSO != nil && s.SSO.Enabled()
}

// origin - the dashboard's address as the browser reached it
func origin(r *http.Request) string {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) handleSSOLogin(w http.ResponseWriter, r *http.Request) {
	if !s.ssoEnabled() {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't set up")
		return
	}
	s.startSSO(w, r, localPath(r.URL.Query().Get("return_to")), false)
}

// handleSSOTest - a sign-in with the draft settings, which turns them on if
// it works. Behind the dashboard's own sign-in, as it changes who can get in.
func (s *Server) handleSSOTest(w http.ResponseWriter, r *http.Request) {
	if s.SSO == nil {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't available")
		return
	}
	s.startSSO(w, r, "/settings", true)
}

func (s *Server) startSSO(w http.ResponseWriter, r *http.Request, returnTo string, test bool) {
	authURL, state, err := s.SSO.Start(r.Context(), oidcauth.RedirectURL(origin(r)), returnTo, test)
	if err != nil {
		log.Error().Err(err).Msg("Couldn't start signing in with SSO")
		http.Redirect(w, r, ssoErrorURL(returnTo, "unavailable"), http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     ssoStateCookie,
		Value:    state,
		Path:     oidcauth.CallbackPath,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		// Lax still sends it on the provider's redirect back, a top-level GET
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	if s.SSO == nil {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't available")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: ssoStateCookie, Value: "", Path: oidcauth.CallbackPath, HttpOnly: true, MaxAge: -1})

	q := r.URL.Query()
	cookie, err := r.Cookie(ssoStateCookie)
	state := q.Get("state")
	if err != nil || state == "" || cookie.Value != state {
		log.Warn().Msg("SSO sign-in came back to a browser that didn't start it")
		http.Redirect(w, r, ssoErrorURL("/", "failed"), http.StatusFound)
		return
	}
	// The provider refused: the user cancelled, or isn't allowed this
	// client. Finish still clears the sign-in, and says where it began.
	if e := q.Get("error"); e != "" {
		res, _ := s.SSO.Finish(r.Context(), state, "")
		log.Warn().Str("error", e).Str("description", q.Get("error_description")).Msg("SSO sign-in refused by the provider")
		http.Redirect(w, r, ssoErrorURL(res.ReturnTo, "denied"), http.StatusFound)
		return
	}
	res, err := s.SSO.Finish(r.Context(), state, q.Get("code"))
	if err != nil {
		log.Error().Err(err).Bool("test", res.Test).Msg("SSO sign-in failed")
		http.Redirect(w, r, ssoErrorURL(res.ReturnTo, "failed"), http.StatusFound)
		return
	}
	if err := s.startSession(w, r); err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		http.Redirect(w, r, ssoErrorURL(res.ReturnTo, "failed"), http.StatusFound)
		return
	}
	who := res.Identity
	if res.Test {
		log.Info().Str("subject", who.Subject).Str("email", who.Email).Msg("SSO test sign-in worked; sign-in with SSO is on")
		http.Redirect(w, r, "/settings?sso=on", http.StatusFound)
		return
	}
	log.Info().Str("subject", who.Subject).Str("email", who.Email).Str("name", who.Name).Msg("Signed in with SSO")
	http.Redirect(w, r, res.ReturnTo, http.StatusFound)
}

// ssoErrorURL - page, with why the sign-in failed for it to show
func ssoErrorURL(page, reason string) string {
	page = localPath(page)
	sep := "?"
	if strings.Contains(page, "?") {
		sep = "&"
	}
	return page + sep + "sso_error=" + reason
}

// localPath - p if it's a path on this dashboard, or "/". Anything else
// (another site, //host) would make the sign-in an open redirect.
func localPath(p string) string {
	u, err := url.Parse(p)
	if err != nil || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return p
}

func (s *Server) ssoSettings(r *http.Request) apitypes.SSOSettingsResponse {
	active, draft, secretSet := s.SSO.Settings()
	res := apitypes.SSOSettingsResponse{
		Enabled:        active != nil,
		Draft:          apitypes.SSOConfig{Issuer: draft.Issuer, ClientID: draft.ClientID},
		DraftSecretSet: secretSet,
		CallbackURL:    oidcauth.RedirectURL(origin(r)),
	}
	if active != nil {
		res.Active = &apitypes.SSOConfig{Issuer: active.Issuer, ClientID: active.ClientID}
	}
	return res
}

func (s *Server) handleSSOSettingsGet(w http.ResponseWriter, r *http.Request) {
	if s.SSO == nil {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't available")
		return
	}
	writeJSON(w, s.ssoSettings(r))
}

// handleSSOSettingsSet saves the form as the draft. It isn't used until a
// test sign-in with it works.
func (s *Server) handleSSOSettingsSet(w http.ResponseWriter, r *http.Request) {
	if s.SSO == nil {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't available")
		return
	}
	var req apitypes.SSOSettingsRequest
	if !decode(w, r, &req) {
		return
	}
	err := s.SSO.SaveDraft(r.Context(), oidcauth.Config{Issuer: req.Issuer, ClientID: req.ClientID, ClientSecret: req.ClientSecret})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_sso", err.Error())
		return
	}
	writeJSON(w, s.ssoSettings(r))
}

func (s *Server) handleSSOSettingsDisable(w http.ResponseWriter, r *http.Request) {
	if s.SSO == nil {
		writeError(w, http.StatusNotFound, "sso_off", "Sign-in with SSO isn't available")
		return
	}
	if err := s.SSO.Disable(); err != nil {
		log.Error().Err(err).Msg("Failed to turn off SSO")
		writeError(w, http.StatusInternalServerError, "sso_failed", "Couldn't save the SSO settings")
		return
	}
	log.Info().Msg("Sign-in with SSO turned off")
	writeJSON(w, s.ssoSettings(r))
}
