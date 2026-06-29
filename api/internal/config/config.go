// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultJWTSecret is the development-only fallback signing secret. Using it
// outside development is rejected (production) or warned about (callers).
const DefaultJWTSecret = "dev-secret-change-me"

// Config holds the settings the API needs to run.
type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	AIServiceURL       string
	AIInternalToken    string
	JWTSecret          string
	JWTTTL             time.Duration
	CORSAllowedOrigins []string
	Env                string

	// Billing (Stripe). When StripeSecretKey is empty the API runs in the
	// $0 dev mode: upgrades go through a local dev-confirm endpoint instead
	// of hosted checkout.
	StripeSecretKey     string
	StripePriceProID    string
	StripeWebhookSecret string
	WebBaseURL          string
}

// Load reads configuration from environment variables, applying
// development-friendly defaults so the service runs with zero setup.
func Load() (Config, error) {
	cfg := Config{
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://ragdesk:ragdesk@localhost:5432/ragdesk?sslmode=disable"),
		RedisURL:           getenv("REDIS_URL", "redis://localhost:6379/0"),
		AIServiceURL:       getenv("AI_SERVICE_URL", "http://localhost:8000"),
		AIInternalToken:    getenv("AI_INTERNAL_TOKEN", ""),
		JWTSecret:          getenv("JWT_SECRET", DefaultJWTSecret),
		JWTTTL:             24 * time.Hour,
		CORSAllowedOrigins: splitAndTrim(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		Env:                getenv("RAGDESK_ENV", "development"),

		StripeSecretKey:     getenv("STRIPE_SECRET_KEY", ""),
		StripePriceProID:    getenv("STRIPE_PRICE_PRO", ""),
		StripeWebhookSecret: getenv("STRIPE_WEBHOOK_SECRET", ""),
		WebBaseURL:          getenv("WEB_BASE_URL", "http://localhost:3000"),
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
	if cfg.JWTSecret == DefaultJWTSecret && cfg.Env == "production" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set when RAGDESK_ENV=production")
	}
	// If Stripe is enabled, refuse to start without a webhook secret: unverified
	// webhooks would let anyone grant themselves a paid plan.
	if cfg.StripeSecretKey != "" && cfg.StripeWebhookSecret == "" {
		return Config{}, fmt.Errorf("STRIPE_WEBHOOK_SECRET is required when STRIPE_SECRET_KEY is set")
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
