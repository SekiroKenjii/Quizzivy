// Package config reads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	Env            string
	DatabaseURL    string
	AllowedOrigins []string

	// ClientIPHeader names the ONE header that carries the real client address,
	// or "" to use the socket. Only headers the infrastructure overwrites are
	// safe here -- CF-Connecting-IP behind Cloudflare, Fly-Client-IP on Fly.
	// Never X-Forwarded-For: proxies append to it, so a client can prepend its
	// own value and choose its own rate-limit bucket.
	ClientIPHeader string

	JWTSigningKey       []byte
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	RefreshCookieSecure bool
}

// Load reads the environment and fails loudly on anything missing.
//
// No silent defaults for security-relevant values. A DATABASE_URL that falls
// back to something plausible, or an empty CORS allowlist treated as "allow
// all", are the kind of defaults that work in development and are wrong in
// production without anyone noticing.
func Load() (Config, error) {
	var err error
	cfg := Config{
		Port:           getenv("API_PORT", "8080"),
		Env:            getenv("APP_ENV", "development"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		ClientIPHeader: os.Getenv("CLIENT_IP_HEADER"),
	}

	// Fail rather than silently accept a header that cannot be trusted.
	if strings.EqualFold(cfg.ClientIPHeader, "X-Forwarded-For") {
		return cfg, fmt.Errorf(
			"CLIENT_IP_HEADER must not be X-Forwarded-For: proxies append to it, so a " +
				"client can prepend a value and choose its own rate-limit bucket (§6.5)")
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}

	// A short HMAC key is brute-forceable offline, and a forged token grants
	// whatever role the attacker writes into it. Refuse rather than warn.
	cfg.JWTSigningKey = []byte(os.Getenv("JWT_SIGNING_KEY"))
	if len(cfg.JWTSigningKey) < 32 {
		return cfg, fmt.Errorf("JWT_SIGNING_KEY must be at least 32 bytes (got %d); generate one with: openssl rand -base64 48",
			len(cfg.JWTSigningKey))
	}

	if cfg.AccessTokenTTL, err = parseDuration("ACCESS_TOKEN_TTL", "15m"); err != nil {
		return cfg, err
	}
	if cfg.RefreshTokenTTL, err = parseDuration("REFRESH_TOKEN_TTL", "720h"); err != nil {
		return cfg, err
	}

	// Defaults to TRUE. The one environment where it is false is plain-http
	// localhost; defaulting the other way would ship a session cookie in the
	// clear the first time someone forgot to set it.
	cfg.RefreshCookieSecure = getenv("REFRESH_COOKIE_SECURE", "true") != "false"

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

func parseDuration(key, fallback string) (time.Duration, error) {
	d, err := time.ParseDuration(getenv(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration such as 15m or 720h: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
