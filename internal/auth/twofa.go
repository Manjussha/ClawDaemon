package auth

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/yourusername/clawdaemon/internal/db"
)

// TelegramSender can send a text message via Telegram.
type TelegramSender interface {
	Send(msg string) error
}

// GenerateOTP creates a 6-digit OTP for the user, stores it in DB with 5-min expiry.
func GenerateOTP(ctx context.Context, database *db.DB, userID int) (string, error) {
	// Invalidate any existing unused OTPs for this user.
	_, err := database.ExecContext(ctx, `UPDATE twofa_otp SET used=1 WHERE user_id=? AND used=0`, userID)
	if err != nil {
		return "", fmt.Errorf("auth.GenerateOTP: invalidate old: %w", err)
	}

	otp := fmt.Sprintf("%06d", rand.Intn(1000000))
	expiresAt := time.Now().Add(5 * time.Minute)

	_, err = database.ExecContext(ctx,
		`INSERT INTO twofa_otp (user_id, otp, used, expires_at) VALUES (?,?,0,?)`,
		userID, otp, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("auth.GenerateOTP: insert: %w", err)
	}
	return otp, nil
}

// SendOTP sends the OTP to the admin Telegram chat.
func SendOTP(sender TelegramSender, otp string) error {
	return sender.Send(fmt.Sprintf("ClawDaemon 2FA code: *%s*\n\nExpires in 5 minutes.", otp))
}

// VerifyOTP validates the OTP and marks the session as 2FA-verified.
func VerifyOTP(ctx context.Context, database *db.DB, userID int, otp, sessionToken string) error {
	var id int
	var expiresAt time.Time
	err := database.QueryRowContext(ctx,
		`SELECT id, expires_at FROM twofa_otp WHERE user_id=? AND otp=? AND used=0 LIMIT 1`,
		userID, otp,
	).Scan(&id, &expiresAt)
	if err != nil {
		return fmt.Errorf("auth.VerifyOTP: invalid or not found")
	}
	if time.Now().After(expiresAt) {
		return fmt.Errorf("auth.VerifyOTP: OTP expired")
	}

	// Mark OTP as used.
	if _, err := database.ExecContext(ctx, `UPDATE twofa_otp SET used=1 WHERE id=?`, id); err != nil {
		return fmt.Errorf("auth.VerifyOTP: mark used: %w", err)
	}
	// Mark session as 2FA verified.
	if _, err := database.ExecContext(ctx,
		`UPDATE sessions SET twofa_verified=1 WHERE token=?`, sessionToken); err != nil {
		return fmt.Errorf("auth.VerifyOTP: update session: %w", err)
	}
	return nil
}