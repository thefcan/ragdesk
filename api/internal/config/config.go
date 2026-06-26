// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds the settings the API needs to run.
type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	AIServiceURL       string
	JWTSecret          string
	JWTTTL             time.Duration
	CORSAllowedOrigins []string
	Env                string
}

// Load reads configuration from environment variables, applying
// development-friendly defaults so the service runs with zero setup.
func Load() (Config, error) {
	cfg := Config{
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://ragdesk:ragdesk@localhost:5432/ragdesk?sslmode=disable"),
		RedisURL:           getenv("REDIS_URL", "redis://localhost:6379/0"),
		AIServiceURL:       getenv("AI_SERVICE_URL", "http://localhost:8000"),
		JWTSecret:          getenv("JWT_SECRET", "dev-secret-change-me"),
		JWTTTL:             24 * time.Hour,
		CORSAllowedOrigins: splitAndTrim(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		Env:                getenv("RAGDESK_ENV", "development"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
