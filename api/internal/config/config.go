// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
)

// Config holds the settings the API needs to run.
type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	AIServiceURL string
	Env          string
}

// Load reads configuration from environment variables, applying
// development-friendly defaults so the service runs with zero setup.
func Load() (Config, error) {
	cfg := Config{
		Port:         getenv("PORT", "8080"),
		DatabaseURL:  getenv("DATABASE_URL", "postgres://ragdesk:ragdesk@localhost:5432/ragdesk?sslmode=disable"),
		RedisURL:     getenv("REDIS_URL", "redis://localhost:6379/0"),
		AIServiceURL: getenv("AI_SERVICE_URL", "http://localhost:8000"),
		Env:          getenv("RAGDESK_ENV", "development"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
