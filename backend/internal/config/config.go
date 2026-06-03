package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv       string
	Port         string
	DatabaseURL  string
	RedisURL     string
	FrontendURL  string
	CORSOrigins  []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load() Config {
	return Config{
		AppEnv:       env("APP_ENV", "development"),
		Port:         env("APP_PORT", "8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://idp:idp@localhost:5432/idp?sslmode=disable"),
		RedisURL:     env("REDIS_URL", "redis://localhost:6379"),
		FrontendURL:  env("FRONTEND_URL", "http://localhost:5173"),
		CORSOrigins:  splitCSV(env("CORS_ORIGINS", "http://localhost:5173")),
		ReadTimeout:  secondsEnv("HTTP_READ_TIMEOUT_SECONDS", 10),
		WriteTimeout: secondsEnv("HTTP_WRITE_TIMEOUT_SECONDS", 15),
		IdleTimeout:  secondsEnv("HTTP_IDLE_TIMEOUT_SECONDS", 60),
	}
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
