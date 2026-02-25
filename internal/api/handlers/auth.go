package handlers

import (
	"net/http"

	"github.com/Manjussha/clawdaemon/internal/auth"
)

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	ip := r.RemoteAddr

	// Brute force check.
	blocked, err := auth.IsBlocked(r.Context(), h.db, ip,
		h.config.BruteForceMaxAttempts, h.config.BruteForceBlockMinutes)
	if err != nil || blocked {
		fail(w, http.StatusTooManyRequests, "IP blocked due to too many failed attempts")
		return
	}

	token, userID, err := auth.Login(r.Context(), h.db, req.Username, req.Password, h.config.SessionExpiryHours)
	if err != nil {
		_ = auth.TrackAttempt(r.Context(), h.db, ip, false)
		fail(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	_ = auth.TrackAttempt(r.Context(), h.db, ip, true)

	auth.SetSessionCookie(w, token, h.config.SessionExpiryHours)

	// Generate and send 2FA OTP if Telegram is configured.
	// If Telegram is not configured, auto-verify the session so the user can proceed.
	if h.notify != nil && h.config.TelegramToken != "" {
		otp, err := auth.GenerateOTP(r.Context(), h.db, userID)
		if err == nil {
			h.notify.SendTelegram("🔐 ClawDaemon 2FA code: *" + otp + "*\n\nExpires in 5 minutes.")
		}
	} else {
		_ = auth.MarkSessionVerified(r.Context(), h.db, token)
	}

	requires2FA := h.config.TelegramToken != ""
	ok(w, map[string]interface{}{"user_id": userID, "requires_2fa": requires2FA})
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := auth.SessionTokenFromRequest(r)
	if token != "" {
		_ = auth.Logout(r.Context(), h.db, token)
	}
	auth.ClearSessionCookie(w)
	ok(w, map[string]string{"message": "logged out"})
}

// Verify2FA handles POST /api/v1/auth/2fa/verify.
func (h *Handler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OTP string `json:"otp"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	token := auth.SessionTokenFromRequest(r)
	if token == "" {
		fail(w, http.StatusUnauthorized, "no session")
		return
	}

	user, _, err := auth.ValidateSession(r.Context(), h.db, token)
	if err != nil {
		fail(w, http.StatusUnauthorized, "invalid session")
		return
	}

	if err := auth.VerifyOTP(r.Context(), h.db, user.ID, req.OTP, token); err != nil {
		fail(w, http.StatusUnauthorized, "invalid or expired OTP")
		return
	}

	ok(w, map[string]string{"message": "2FA verified"})
}

// Me handles GET /api/v1/auth/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		fail(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	ok(w, map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
	})
}
