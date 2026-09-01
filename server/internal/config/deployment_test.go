package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"quizzivy/internal/config"
)

// Every name config.Load reads. Cleared before each case so a developer's own
// shell -- which has all of these set, from the repo's .env -- cannot make a
// deployment look bootable when it is not.
var configuredBy = []string{
	"API_PORT", "APP_ENV", "CLIENT_IP_HEADER", "CORS_ALLOWED_ORIGINS", "DATABASE_URL",
	"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI", "JWT_SIGNING_KEY",
	"MAX_CONCURRENT_PASSWORD_HASHES", "REFRESH_COOKIE_SECURE", "S3_ACCESS_KEY_ID",
	"S3_BUCKET", "S3_ENDPOINT", "S3_FORCE_PATH_STYLE", "S3_REGION",
	"S3_SECRET_ACCESS_KEY", "VITE_GOOGLE_CLIENT_ID",
}

// What `fly secrets set` supplies, by name, per docs/setup/dns.md. Values are
// placeholders: this asks whether the NAMES add up to a config Load accepts,
// which is the half that a committed file can be wrong about.
var flySecrets = map[string]string{
	"DATABASE_URL":         "postgres://quizzivy_app:pw@example.neon.tech/quizzivy?sslmode=require",
	"JWT_SIGNING_KEY":      "0123456789abcdef0123456789abcdef0123456789abcdef",
	"GOOGLE_CLIENT_SECRET": "placeholder-secret",
	"S3_ENDPOINT":          "https://account.r2.cloudflarestorage.com",
	"S3_BUCKET":            "quizzivy-media",
	"S3_ACCESS_KEY_ID":     "placeholder-key-id",
	"S3_SECRET_ACCESS_KEY": "placeholder-secret-key",
}

var envLine = regexp.MustCompile(`^\s*([A-Z0-9_]+)\s*=\s*"([^"]*)"`)

// flyEnv parses the [env] table out of the committed fly.toml.
//
// Hand-parsed rather than pulling in a TOML decoder: the block is a flat list of
// quoted strings that this repo writes itself, and a dependency added for a test
// needs a better reason than fifteen lines.
func flyEnv(t *testing.T) map[string]string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller")
	}
	// server/internal/config -> repo root
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "fly.toml"))
	if err != nil {
		t.Fatalf("read fly.toml: %v", err)
	}

	out := map[string]string{}
	inEnv := false
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "[") {
			inEnv = trimmed == "[env]"
			continue
		}
		if !inEnv {
			continue
		}
		if m := envLine.FindStringSubmatch(line); m != nil {
			out[m[1]] = m[2]
		}
	}
	if len(out) == 0 {
		t.Fatal("fly.toml has no [env] entries; this test is not reading what it thinks it is")
	}
	return out
}

func apply(t *testing.T, sets ...map[string]string) {
	t.Helper()
	for _, name := range configuredBy {
		t.Setenv(name, "")
	}
	for _, set := range sets {
		for name, value := range set {
			t.Setenv(name, value)
		}
	}
}

// The production deployment, assembled from the two places its configuration
// actually lives, and asked the only question that matters: does it boot.
//
// This is the test that was missing. GOOGLE_REDIRECT_URI sat in fly.toml and
// GOOGLE_CLIENT_SECRET in `fly secrets`, but the client id was in neither --
// and loadGoogle refuses two-of-three by design. Nothing on the way to
// production compared those two sets, so it was found by Fly's smoke check
// reporting "the app appears to be crashing", with an empty log tail.
func TestTheCommittedFlyConfigBootsWithTheDocumentedSecrets(t *testing.T) {
	apply(t, flyEnv(t), flySecrets)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("fly.toml [env] plus the secrets in docs/setup/dns.md does not boot: %v", err)
	}
	if !cfg.MediaEnabled() {
		t.Error("object storage is not configured; audio upload would be dead in production")
	}
	if len(cfg.GoogleRedirectURIs) == 0 {
		t.Error("google sign-in is off; §5.3 is not optional in production")
	}
}

// Proves the test above bites. Drop the one value that was missing and Load must
// refuse, rather than starting with Google half configured.
func TestRemovingTheClientIdIsRefusedRatherThanIgnored(t *testing.T) {
	env := flyEnv(t)
	delete(env, "GOOGLE_CLIENT_ID")
	apply(t, env, flySecrets)

	if _, err := config.Load(); err == nil {
		t.Fatal("Load accepted Google configured two-of-three; that is the state that crashed production")
	}
}

// The other half of the same failure, and the quieter one: object storage set
// under the R2_* names does not fail, it disables media. docs/setup/dns.md said
// to set exactly those until 2026-09-01.
func TestObjectStorageUnderTheWrongNamesLeavesMediaOff(t *testing.T) {
	secrets := map[string]string{}
	for name, value := range flySecrets {
		if strings.HasPrefix(name, "S3_") {
			continue
		}
		secrets[name] = value
	}
	apply(t, flyEnv(t), secrets, map[string]string{
		"R2_ACCOUNT_ID": "account", "R2_ACCESS_KEY_ID": "key", "R2_SECRET_ACCESS_KEY": "secret",
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("R2_* names should be ignored, not fatal: %v", err)
	}
	if cfg.MediaEnabled() {
		t.Fatal("R2_* names now reach the S3_* config; update the docs and delete this test")
	}
}
