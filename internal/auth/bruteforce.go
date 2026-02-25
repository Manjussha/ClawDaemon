package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/clawdaemon/internal/db"
)

// TrackAttempt records a login attempt (success or failure) for an IP.
func TrackAttempt(ctx context.Context, database *db.DB, ip string, success bool) error {
	sval := 0
	if success {
		sval = 1
	}
	_, err := database.ExecContext(ctx,
		`INSERT INTO login_attempts (ip, success) VALUES (?,?)`, ip, sval)
	if err != nil {
		return fmt.Errorf("auth.TrackAttempt: %w", err)
	}
	return nil
}

// IsBlocked returns true if the IP has exceeded maxAttempts failures in the last blockMinutes.
func IsBlocked(ctx context.Context, database *db.DB, ip string, maxAttempts, blockMinutes int) (bool, error) {
	since := time.Now().Add(-time.Duration(blockMinutes) * time.Minute)
	var count int
	err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM login_attempts WHERE ip=? AND success=0 AND created_at > ?`,
		ip, since,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("auth.IsBlocked: %w", err)
	}
	return count >= maxAttempts, nil
}

// CleanOldAttempts removes login attempt records older than 24 hours.
func CleanOldAttempts(ctx context.Context, database *db.DB) error {
	cutoff := time.Now().Add(-24 * time.Hour)
	_, err := database.ExecContext(ctx, `DELETE FROM login_attempts WHERE created_at < ?`, cutoff)
	if err != nil {
		return fmt.Errorf("auth.CleanOldAttempts: %w", err)
	}
	return nil
}