package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"quizzivy/internal/platform/config"
)

// Every name config.Load reads. Cleared before each case so a developer's own
// shell -- which has all of these set, from the repo's .env -- cannot make a
// deployment look bootable when it is not.
var configuredBy = []string{
	"API_PORT", "APP_ENV", "CLIENT_IP_HEADER", "CORS_ALLOWED_ORIGINS", "DATABASE_URL",
	"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI", "JWT_SIGNING_KEY",
	"MAX_CONCURRENT_PASSWORD_HASHES", "REFRESH_COOKIE_SECURE", "S3_ACCESS_KEY_ID",
	"S3_BUCKET", "S3_ENDPOINT", "S3_FORCE_PATH_STYLE", "S3_REGION",
	"S3_SECRET_ACCESS_KEY", "VITE_GOOGLE_CLIENT_ID", "DOCS_PUBLIC",
	"IMPORT_S3_BUCKET", "IMPORT_WORK_DIR", "IMPORT_LEGACY_DOC", "IMPORT_PROCESSING_ENABLED",
	"IMPORT_ACTOR_COUNT", "IMPORT_GLOBAL_COUNT", "IMPORT_SOURCES_PER_ITEM", "IMPORT_ACTOR_MIB",
	"IMPORT_GLOBAL_MIB",
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

var tableLine = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s#"']+))\s*(?:#.*)?$`)

// flyEnv parses the [env] table out of the committed fly.toml.
//
// Hand-parsed rather than pulling in a TOML decoder: the block is a flat list of
// scalars that this repo writes itself, and a dependency added for a test
// needs a better reason than a few lines.
func flyEnv(t *testing.T) map[string]string {
	t.Helper()
	out := flyTable(t, "env")
	if len(out) == 0 {
		t.Fatal("fly.toml has no [env] entries; this test is not reading what it thinks it is")
	}
	return out
}

func flyTable(t *testing.T, name string) map[string]string {
	t.Helper()
	out := map[string]string{}
	inTable := false
	for _, line := range strings.Split(repoFile(t, "fly.toml"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inTable = trimmed == "["+name+"]"
			continue
		}
		if !inTable || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := tableLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("fly.toml [%s] line %q is not a key = value this test can read", name, trimmed)
		}
		out[m[1]] = m[2] + m[3] + m[4]
	}
	return out
}

func repoFile(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
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
	if cfg.DocsPublic {
		t.Error("the API reference is open to anyone; production must keep the docs gate on")
	}
}

func dockerfileShipsTheWorker(t *testing.T) bool {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(repoFile(t, "Dockerfile"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, trimmed)
		}
	}
	runtime := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			runtime = i
		}
	}
	built, copied := false, false
	for i, line := range lines {
		if i < runtime && strings.Contains(line, "./cmd/import-worker") {
			built = true
		}
		if i > runtime && strings.HasPrefix(strings.ToUpper(line), "COPY ") && strings.Contains(line, "/app/import-worker") {
			copied = true
		}
	}
	return built && copied
}

func TestProductionProcessesWordImportsOnlyBesideADeployedWorker(t *testing.T) {
	apply(t, flyEnv(t), flySecrets)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	worker := false
	for _, command := range flyTable(t, "processes") {
		worker = worker || strings.HasPrefix(command, "/app/import-worker")
	}
	if cfg.ImportProcessing != worker {
		t.Fatalf("fly.toml: IMPORT_PROCESSING_ENABLED=%v, import worker process declared=%v; the two go together", cfg.ImportProcessing, worker)
	}
	if worker && !dockerfileShipsTheWorker(t) {
		t.Fatal("fly.toml runs /app/import-worker but the Dockerfile does not build it and copy it there")
	}
	if cfg.ImportLegacyDoc {
		t.Fatal("production accepts .doc uploads, but the converter .doc needs is a Docker container, and the production image has no Docker daemon")
	}
}

func TestPublicDocsAreRefusedInProduction(t *testing.T) {
	apply(t, flyEnv(t), flySecrets, map[string]string{"DOCS_PUBLIC": "true"})

	if _, err := config.Load(); err == nil {
		t.Fatal("Load accepted DOCS_PUBLIC=true with APP_ENV=production")
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
