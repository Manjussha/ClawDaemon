// Package config loads daemon configuration from environment variables.
package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/Manjussha/clawdaemon/internal/platform"
)

// Config holds all runtime configuration for ClawDaemon.
type Config struct {
	Port           string
	WorkDir        string
	DBPath         string
	CharacterDir   string
	ScreenshotsDir string

	AdminUsername string
	AdminPassword string

	TelegramToken  string
	TelegramChatID int64

	DuckDNSToken  string
	DuckDNSDomain string

	JWTSecret string

	SessionExpiryHours      int
	BruteForceMaxAttempts   int
	BruteForceBlockMinutes  int
}

// Load reads environment variables and returns a Config.
// Uses sensible defaults for optional fields.
// Panics if required fields are empty.
func Load() *Config {
	workDir := getEnv("WORK_DIR", platform.DefaultWorkDir())

	dbPath := getEnv("DB_PATH", filepath.Join(workDir, "clawdaemon.db"))
	if dbPath == "" {
		panic("config: DB_PATH is required")
	}

	chatID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)

	return &Config{
		Port:           getEnv("PORT", "8080"),
		WorkDir:        workDir,
		DBPath:         dbPath,
		CharacterDir:   getEnv("CHARACTER_DIR", filepath.Join(workDir, "character")),
		ScreenshotsDir: getEnv("SCREENSHOTS_DIR", filepath.Join(workDir, "screenshots")),

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "changeme"),

		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID: chatID,

		DuckDNSToken:  os.Getenv("DUCKDNS_TOKEN"),
		DuckDNSDomain: os.Getenv("DUCKDNS_DOMAIN"),

		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),

		SessionExpiryHours:     getEnvInt("SESSION_EXPIRY_HOURS", 24),
		BruteForceMaxAttempts:  getEnvInt("BRUTE_FORCE_MAX_ATTEMPTS", 5),
		BruteForceBlockMinutes: getEnvInt("BRUTE_FORCE_BLOCK_MINUTES", 15),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
