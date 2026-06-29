package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv             string
	Port               string
	DatabaseURL        string
	RedisURL           string
	FrontendURL        string
	CORSOrigins        []string
	JWTSecret          string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	RefreshCookieName  string
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	MinIOPublicURL     string
	MinIOUseSSL        bool
	SeedAdminEmail     string
	SeedAdminPassword  string
	SeedAdminFirstName string
	SeedAdminLastName  string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	SMTPHost           string
	SMTPPort           int
	SMTPUsername       string
	SMTPPassword       string
	SMTPFromEmail      string
	SMTPFromName       string
	EmailQueueKey      string
	ReminderTimezone   string
}

func Load() Config {
	return Config{
		AppEnv:             env("APP_ENV", "development"),
		Port:               env("APP_PORT", "8080"),
		DatabaseURL:        env("DATABASE_URL", "postgres://idp:idp@localhost:5432/idp?sslmode=disable"),
		RedisURL:           env("REDIS_URL", "redis://localhost:6379"),
		FrontendURL:        env("FRONTEND_URL", "http://localhost:5173"),
		CORSOrigins:        splitCSV(env("CORS_ORIGINS", "http://localhost:5173")),
		JWTSecret:          env("JWT_SECRET", "local-development-secret-change-me"),
		JWTAccessTTL:       durationEnv("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:      durationEnv("JWT_REFRESH_TTL", 30*24*time.Hour),
		RefreshCookieName:  env("REFRESH_COOKIE_NAME", "idp_refresh_token"),
		MinIOEndpoint:      env("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:     env("MINIO_ACCESS_KEY", "minio"),
		MinIOSecretKey:     env("MINIO_SECRET_KEY", "minio12345"),
		MinIOBucket:        env("MINIO_BUCKET", "idp-platform"),
		MinIOPublicURL:     env("MINIO_PUBLIC_URL", "http://localhost:9000"),
		MinIOUseSSL:        boolEnv("MINIO_USE_SSL", false),
		SeedAdminEmail:     env("SEED_ADMIN_EMAIL", "admin@idp.local"),
		SeedAdminPassword:  env("SEED_ADMIN_PASSWORD", "Admin12345"),
		SeedAdminFirstName: env("SEED_ADMIN_FIRST_NAME", "HR"),
		SeedAdminLastName:  env("SEED_ADMIN_LAST_NAME", "Admin"),
		ReadTimeout:        secondsEnv("HTTP_READ_TIMEOUT_SECONDS", 10),
		WriteTimeout:       secondsEnv("HTTP_WRITE_TIMEOUT_SECONDS", 15),
		IdleTimeout:        secondsEnv("HTTP_IDLE_TIMEOUT_SECONDS", 60),
		SMTPHost:           env("SMTP_HOST", "mailpit"),
		SMTPPort:           intEnv("SMTP_PORT", 1025),
		SMTPUsername:       env("SMTP_USERNAME", ""),
		SMTPPassword:       env("SMTP_PASSWORD", ""),
		SMTPFromEmail:      env("SMTP_FROM_EMAIL", "noreply@idp.local"),
		SMTPFromName:       env("SMTP_FROM_NAME", "IDP Platform"),
		EmailQueueKey:      env("EMAIL_QUEUE_KEY", "idp:email:queue"),
		ReminderTimezone:   env("EMAIL_REMINDER_TIMEZONE", "Europe/Moscow"),
	}
}

func boolEnv(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes"
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func secondsEnv(key string, fallback int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return time.Duration(fallback) * time.Second
	}

	return time.Duration(value) * time.Second
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
