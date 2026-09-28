package httpapi

import (
	"errors"
	"net/http"
	"os"

	"github.com/andrewshaodev/nanit-web/pkg/client"
	"github.com/andrewshaodev/nanit-web/pkg/httpapi/apitypes"
	"github.com/rs/zerolog/log"
)

// Signing in to Nanit

func (s *Server) handleNanitLogin(w http.ResponseWriter, r *http.Request) {
	var req apitypes.LoginRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_credentials", "Email and password are required")
		return
	}

	challenge, err := s.Nanit.StartLogin(req.Email, req.Password)
	if err != nil {
		writeLoginError(w, err)
		return
	}

	// An account without 2FA is signed in already
	if challenge == nil {
		s.afterSignIn()
		writeJSON(w, apitypes.LoginResponse{Success: true, SignedIn: true, Message: "Signed in to Nanit"})
		return
	}

	// Nanit picks the channel (sms or email). The client needs it to say
	// where the code went, and to send it back with the code.
	writeJSON(w, apitypes.LoginResponse{
		Success:     true,
		MFAToken:    challenge.MFAToken,
		Channel:     challenge.Channel,
		PhoneSuffix: challenge.PhoneSuffix,
		Message:     "MFA token received. Enter the verification code Nanit sent.",
	})
}

func (s *Server) handleNanitVerify2FA(w http.ResponseWriter, r *http.Request) {
	var req apitypes.Verify2FARequest
	if !decode(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" || req.MFACode == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "Email, password and code are all required")
		return
	}

	if err := s.Nanit.FinishLogin(req.Email, req.Password, req.MFAToken, req.MFACode, req.Channel); err != nil {
		writeLoginError(w, err)
		return
	}

	s.afterSignIn()
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Authentication completed successfully"})
}

// afterSignIn fetches the account's cameras before the sign-in is answered,
// so the dashboard it opens has them: they used to be fetched in the
// background, and the first load said "No babies configured". The cameras'
// connections then start in the background.
func (s *Server) afterSignIn() {
	if _, err := s.Nanit.FetchBabies(); err != nil {
		log.Error().Err(err).Msg("Failed to fetch babies after signing in")
	}
	if s.SignedIn != nil {
		go s.SignedIn()
	}
}

// writeLoginError answers a failed Nanit sign-in: 401 with Nanit's reason
// when it said no, 502 when it couldn't be reached or made no sense
func writeLoginError(w http.ResponseWriter, err error) {
	var rejected *client.LoginError
	if errors.As(err, &rejected) {
		writeError(w, http.StatusUnauthorized, "nanit_rejected", err.Error())
		return
	}
	log.Error().Err(err).Msg("Nanit sign-in failed")
	writeError(w, http.StatusBadGateway, "nanit_unavailable", err.Error())
}

func (s *Server) handleNanitStatus(w http.ResponseWriter, r *http.Request) {
	res := apitypes.AuthStatusResponse{Message: "No authentication found"}

	if _, err := os.Stat(s.Config.SessionFile); err == nil {
		if s.Sessions == nil || s.Sessions.RefreshToken() == "" {
			res.Message = "Session file exists but invalid"
		} else {
			res.Authenticated = true
			res.Message = "Authenticated"
			if s.Nanit != nil {
				res.Email = s.Nanit.Email
			}
			if authTime := s.Sessions.AuthTime(); !authTime.IsZero() {
				unix := authTime.Unix()
				res.AuthTime = &unix
			}
			babies := s.Sessions.Babies()
			res.BabiesCount = len(babies)
			// Running: at least one camera is connected
			for _, b := range babies {
				if s.State.GetBabyState(b.UID).GetIsWebsocketAlive() {
					res.ServicesRunning = true
					break
				}
			}
		}
	}
	writeJSON(w, res)
}

// handleNanitReset signs out of Nanit: the cameras are disconnected, and the
// saved session deleted
func (s *Server) handleNanitReset(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("Stopping monitoring services for authentication reset")

	// Disconnect the cameras while the session can still tell them to stop
	// streaming. They used to keep running, reconnecting without a token.
	if s.Cameras != nil {
		s.Cameras.StopAll()
	}
	if s.HLS != nil {
		s.HLS.StopAll()
	}
	if s.Sessions != nil {
		s.Sessions.Reset()
	}
	if s.Config.SessionFile != "" {
		if err := os.Remove(s.Config.SessionFile); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Str("file", s.Config.SessionFile).Msg("Failed to remove session file")
			writeError(w, http.StatusInternalServerError, "reset_failed", "Failed to reset authentication")
			return
		}
	}
	// The store itself is left in place and merely emptied: handing the
	// client a fresh one would detach it from the store the rest of the app
	// reads, so a later login would be invisible
	if s.Nanit != nil {
		s.Nanit.RefreshToken = ""
	}

	log.Info().Msg("Authentication reset completed successfully")
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Authentication reset successfully. Please re-authenticate."})
}

// The dashboard password

func (s *Server) handleWebAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, apitypes.WebAuthStatusResponse{
		PasswordProtectionEnabled: true,
		PasswordSet:               s.WebAuth.IsPasswordSet(),
		// With no password set, everyone counts as signed in
		Authenticated: s.signedIn(r),
	})
}

func (s *Server) handleWebAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req apitypes.PasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.WebAuth.IsPasswordSet() {
		writeError(w, http.StatusBadRequest, "no_password", "No password is set")
		return
	}
	if !s.WebAuth.VerifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_password", "Invalid password")
		return
	}

	sessionID, err := s.WebAuth.CreateSession()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		writeError(w, http.StatusInternalServerError, "session_failed", "Failed to create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		// Only sent back over HTTPS, when that's how the dashboard is
		// reached. It was never set, even behind an HTTPS proxy.
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours, as the session
	})
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Login successful"})
}

// isHTTPS - whether the browser reached the dashboard over HTTPS, directly
// or through a reverse proxy
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *Server) handleWebAuthLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.WebAuth.InvalidateSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Logout successful"})
}

func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var req apitypes.PasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if s.WebAuth.IsPasswordSet() {
		writeError(w, http.StatusBadRequest, "password_set", "Password is already set. Use change-password instead.")
		return
	}
	if err := s.WebAuth.SetPassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return
	}
	log.Info().Msg("Password protection enabled")
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Password set successfully"})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req apitypes.ChangePasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.WebAuth.IsPasswordSet() {
		writeError(w, http.StatusBadRequest, "no_password", "No password is currently set")
		return
	}
	if !s.WebAuth.VerifyPassword(req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_current_password", "Current password is incorrect")
		return
	}
	if err := s.WebAuth.SetPassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return
	}
	log.Info().Msg("Password changed successfully")
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Password changed successfully"})
}

func (s *Server) handleRemovePassword(w http.ResponseWriter, r *http.Request) {
	var req apitypes.PasswordRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.WebAuth.IsPasswordSet() {
		writeError(w, http.StatusBadRequest, "no_password", "No password is currently set")
		return
	}
	if !s.WebAuth.VerifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_password", "Password is incorrect")
		return
	}
	if err := s.WebAuth.RemovePassword(); err != nil {
		log.Error().Err(err).Msg("Failed to remove password")
		writeError(w, http.StatusInternalServerError, "remove_failed", "Failed to remove password")
		return
	}
	writeJSON(w, apitypes.MessageResponse{Success: true, Message: "Password protection disabled successfully"})
}
