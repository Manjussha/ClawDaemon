package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/Manjussha/clawdaemon/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12
const sessionCookieName = "clawdaemon_session"

// HashPassword hashes a plain-text password using bcrypt cost 12.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth.HashPassword: %w", err)
	}
	return string(b), nil
}

// CheckPassword compares plain text against a bcrypt hash.
func CheckPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Login validates credentials, creates a session, and returns the session token.
// Returns ErrInvalidCredentials or ErrUserNotFound on failure.
func Login(ctx context.Context, database *db.DB, username, password string, expiryHours int) (string, int, error) {
	var user db.User
	err := database.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username=?`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err == sql.ErrNoRows {
		return "", 0, fmt.Errorf("auth.Login: user not found")
	}
	if err != nil {
		return "", 0, fmt.Errorf("auth.Login: query user: %w", err)
	}
	if !CheckPassword(password, user.PasswordHash) {
		return "", 0, fmt.Errorf("auth.Login: invalid credentials")
	}

	token, err := generateToken()
	if err != nil {
		return "", 0, fmt.Errorf("auth.Login: generate token: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour)
	_, err = database.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token, twofa_verified, expires_at) VALUES (?,?,0,?)`,
		user.ID, token, expiresAt,
	)
	if err != nil {
		return "", 0, fmt.Errorf("auth.Login: create session: %w", err)
	}
	return token, user.ID, nil
}

// MarkSessionVerified marks a session's twofa_verified flag as true.
// Called when Telegram is not configured so 2FA is skipped.
func MarkSessionVerified(ctx context.Context, database *db.DB, token string) error {
	_, err := database.ExecContext(ctx, `UPDATE sessions SET twofa_verified=1 WHERE token=?`, token)
	if err != nil {
		return fmt.Errorf("auth.MarkSessionVerified: %w", err)
	}
	return nil
}

// Logout deletes a session by token.
func Logout(ctx context.Context, database *db.DB, token string) error {
	_, err := database.ExecContext(ctx, `DELETE FROM sessions WHERE token=?`, token)
	if err != nil {
		return fmt.Errorf("auth.Logout: %w", err)
	}
	return nil
}

// ValidateSession checks the session token and returns the associated User.
// Returns an error if expired, not found, or 2FA not verified.
func ValidateSession(ctx context.Context, database *db.DB, token string) (*db.User, bool, error) {
	var s db.Session
	var u db.User
	err := database.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.token, s.twofa_verified, s.expires_at,
		       u.id, u.username, u.password_hash
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token=?`, token,
	).Scan(&s.ID, &s.UserID, &s.Token, &s.TwoFAVerified, &s.ExpiresAt,
		&u.ID, &u.Username, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, false, fmt.Errorf("auth.ValidateSession: session not found")
	}
	if err != nil {
		return nil, false, fmt.Errorf("auth.ValidateSession: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, false, fmt.Errorf("auth.ValidateSession: session expired")
	}
	return &u, s.TwoFAVerified, nil
}

// SeedAdmin creates the default admin user if no users exist.
func SeedAdmin(ctx context.Context, database *db.DB, username, password string) error {
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("auth.SeedAdmin: count: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = database.ExecContext(ctx, `INSERT INTO users (username, password_hash) VALUES (?,?)`, username, hash)
	if err != nil {
		return fmt.Errorf("auth.SeedAdmin: insert: %w", err)
	}
	return nil
}

// RequireAuth is middleware that checks for a valid session cookie.
// If 2FA is required but not verified, it redirects to /2fa.
func RequireAuth(database *db.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		user, twoFAVerified, err := ValidateSession(r.Context(), database, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if !twoFAVerified {
			http.Redirect(w, r, "/2fa", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAPIKey is middleware that validates a Bearer token from Authorization header.
func RequireAPIKey(database *db.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token = auth[7:]
		}
		if token == "" {
			// Also try cookie
			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				token = cookie.Value
			}
		}
		if token == "" {
			http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, twoFAVerified, err := ValidateSession(r.Context(), database, token)
		if err != nil || !twoFAVerified {
			http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetSessionCookie writes the session cookie to the response.
// Also sets a non-HttpOnly csrf_token cookie so JS can read it for X-CSRF-Token headers.
func SetSessionCookie(w http.ResponseWriter, token string, expiryHours int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   expiryHours * 3600,
		SameSite: http.SameSiteStrictMode,
	})
	// CSRF double-submit cookie — readable by JS, same value used as X-CSRF-Token header.
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token[:16], // first 16 hex chars — enough for CSRF, not the full session token
		Path:     "/",
		HttpOnly: false,
		MaxAge:   expiryHours * 3600,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the session and CSRF cookies.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "csrf_token", Value: "", Path: "/", MaxAge: -1})
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(contextKeyUser).(*db.User)
	return u
}

// SessionTokenFromRequest extracts the session token from the cookie.
func SessionTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

type contextKey int

const contextKeyUser contextKey = iota

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}