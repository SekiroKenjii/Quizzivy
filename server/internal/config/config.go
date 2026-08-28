// Package config reads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	Env            string
	DatabaseURL    string
	AllowedOrigins []string
	TrustProxy     bool
}

// Load reads the environment and fails loudly on anything missing.
//
// No silent defaults for security-relevant values. A DATABASE_URL that falls
// back to something plausible, or an empty CORS allowlist treated as "allow
// all", are the kind of defaults that work in development and are wrong in
// production without anyone noticing.
func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("API_PORT", "8080"),
		Env:         getenv("APP_ENV", "development"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		TrustProxy:  getenv("TRUST_PROXY", "false") == "true",
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}

	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(origins) == "" {
		return cfg, fmt.Errorf("CORS_ALLOWED_ORIGINS is required (exact origins, never '*')")
	}
	for _, o := range strings.Split(origins, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed == "*" {
			// Illegal with credentials, and this API always sends them.
			return cfg, fmt.Errorf("CORS_ALLOWED_ORIGINS must not contain '*' (§4.1)")
		}
		if trimmed != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
		}
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return cfg, fmt.Errorf("API_PORT must be numeric, got %q", cfg.Port)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
