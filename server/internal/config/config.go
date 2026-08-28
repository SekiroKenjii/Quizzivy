// Package config reads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"quizzivy/internal/auth"
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

	// Google sign-in (§5.3). Optional as a group: a deployment without it
	// still serves password login. Partially set is a misconfiguration and
	// refuses to start.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURIs []string

	// MaxConcurrentPasswordHashes bounds Argon2id concurrency. Each in-flight
	// hash holds a 64 MiB arena, so this is a MEMORY limit wearing a
	// concurrency limit's clothes -- see auth.DefaultMaxConcurrentHashes.
	MaxConcurrentPasswordHashes int

	// Object storage (§11.2). MinIO in development, R2 in production, through
	// the same client -- they differ only in endpoint and addressing style.
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3ForcePathStyle  bool
	SignedURLTTL      time.Duration

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

	if err := loadGoogle(&cfg); err != nil {
		return cfg, err
	}

	// Deliberately NOT derived from GOMAXPROCS or the CPU count. The binding
	// constraint is RAM, and a shared-cpu-1x machine reports the host's cores,
	// not its own memory -- sizing on cores would pick a number that OOMs.
	cfg.MaxConcurrentPasswordHashes, err = getenvInt("MAX_CONCURRENT_PASSWORD_HASHES",
		auth.DefaultMaxConcurrentHashes)
	if err != nil {
		return cfg, err
	}
	if cfg.MaxConcurrentPasswordHashes < 1 {
		return cfg, fmt.Errorf("MAX_CONCURRENT_PASSWORD_HASHES must be at least 1")
	}

	if err := loadMedia(&cfg); err != nil {
		return cfg, err
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

// loadGoogle reads the §5.3 credentials.
//
// The client id is deliberately read from VITE_GOOGLE_CLIENT_ID when
// GOOGLE_CLIENT_ID is unset. It is the same public value the frontend bundles,
// and the backend needs it as the `aud` it verifies ID tokens against. Two
// names for one value is how they end up different, and the failure -- every
// Google sign-in rejected for a bad audience -- says nothing about the cause.
func loadGoogle(cfg *Config) error {
	cfg.GoogleClientID = getenv("GOOGLE_CLIENT_ID", os.Getenv("VITE_GOOGLE_CLIENT_ID"))
	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	redirects := os.Getenv("GOOGLE_REDIRECT_URI")

	set := 0
	for _, v := range []string{cfg.GoogleClientID, cfg.GoogleClientSecret, redirects} {
		if strings.TrimSpace(v) != "" {
			set++
		}
	}
	switch set {
	case 0:
		return nil // Google sign-in is simply off.
	case 3:
	default:
		// Half-configured is worse than off: the endpoint would exist and fail
		// in a way that looks like Google's fault.
		return fmt.Errorf("google sign-in needs GOOGLE_CLIENT_ID (or VITE_GOOGLE_CLIENT_ID), " +
			"GOOGLE_CLIENT_SECRET and GOOGLE_REDIRECT_URI together, or none of them")
	}

	// Comma-separated so one build can serve localhost and production. Exact
	// matching is done at exchange time; prefix matching on redirect URIs is
	// the classic OAuth mistake.
	for _, uri := range strings.Split(redirects, ",") {
		if uri = strings.TrimSpace(uri); uri != "" {
			cfg.GoogleRedirectURIs = append(cfg.GoogleRedirectURIs, uri)
		}
	}
	return nil
}

// MediaEnabled reports whether object storage is configured. Media upload is
// optional as a group, like Google sign-in: a deployment without it serves
// everything else rather than refusing to start.
//
// The ENDPOINT counts. Without it the SDK has no BaseEndpoint and resolves
// against real AWS S3 -- so a deployment holding a bucket and R2 credentials
// but no S3_ENDPOINT used to report media as enabled and then talk to the wrong
// provider entirely. loadMedia now refuses that at startup, so by the time this
// is asked the four are all set or all empty.
func (c Config) MediaEnabled() bool {
	return c.S3Endpoint != "" && c.S3Bucket != "" &&
		c.S3AccessKeyID != "" && c.S3SecretAccessKey != ""
}

// loadMedia reads the object-storage group, all-or-nothing, the same way
// loadGoogle reads Google's.
func loadMedia(cfg *Config) error {
	cfg.S3Endpoint = os.Getenv("S3_ENDPOINT")
	cfg.S3Region = getenv("S3_REGION", "auto")
	cfg.S3Bucket = os.Getenv("S3_BUCKET")
	cfg.S3AccessKeyID = os.Getenv("S3_ACCESS_KEY_ID")
	cfg.S3SecretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")

	set := 0
	for _, v := range []string{
		cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKeyID, cfg.S3SecretAccessKey,
	} {
		if strings.TrimSpace(v) != "" {
			set++
		}
	}
	switch set {
	case 0:
		// Media is simply off. Still read the remaining values so a misspelled
		// boolean is reported rather than ignored.
	case 4:
	default:
		// Half-configured is worse than off, exactly as for Google: the upload
		// endpoint would exist and fail in a way that looks like R2's fault.
		return fmt.Errorf("object storage needs S3_ENDPOINT, S3_BUCKET, " +
			"S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY together, or none of them")
	}

	// Defaults to FALSE, which is what production wants.
	//
	// It used to default to true -- path-style addressing -- while .env.example
	// said "MinIO needs this; R2 does not", which reads as an instruction to
	// drop the line for production. Dropping it took the default and broke
	// every upload against R2, which serves buckets as a subdomain, while dev
	// against MinIO worked perfectly. The default now matches the deployment
	// that cannot easily be tested by hand.
	forcePathStyle, err := getenvBool("S3_FORCE_PATH_STYLE", false)
	if err != nil {
		return err
	}
	cfg.S3ForcePathStyle = forcePathStyle

	// §11.2's ten minutes, configurable like every other TTL here rather than
	// documented in .env.example and read by nothing.
	ttl, err := parseDuration("SIGNED_URL_TTL", "10m")
	if err != nil {
		return err
	}
	cfg.SignedURLTTL = ttl
	return nil
}

// GoogleEnabled reports whether §5.3 sign-in is configured.
func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && len(c.GoogleRedirectURIs) > 0
}

// getenvInt reads an integer setting. An ABSENT value takes the fallback; a
// PRESENT but unparseable one is an error.
//
// The two are not the same thing and used to be treated as such. Every other
// input here fails loudly -- a non-numeric API_PORT, X-Forwarded-For as the
// client-IP header, a `*` in the CORS origins, a half-configured Google block --
// and this was the one that quietly substituted the default. Its only caller
// bounds Argon2id concurrency on a 512 MB machine, so "8 slots" instead of "8"
// would start the process on 4 and say nothing, and whoever raised it after
// resizing the machine would never learn it had not applied.
// getenvBool parses a boolean the way getenvInt parses an integer: anything
// unparseable is an error, not a silent fallback.
//
// The previous form was `getenv(key, "true") != "false"`, under which `0`,
// `False`, `FALSE` and `no` all quietly meant true -- the same shape that was
// fixed for getenvInt one commit earlier, and for the same reason: every other
// input here fails loudly.
func getenvBool(key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean such as true or false, got %q", key, v)
	}
	return b, nil
}

func getenvInt(key string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
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
