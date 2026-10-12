package config_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/config"
)

// loadWith runs Load with a minimal valid environment plus the overrides given.
// t.Setenv restores everything afterwards and forbids parallel tests, which is
// what makes touching the process environment safe here.
func loadWith(t *testing.T, env map[string]string) (config.Config, error) {
	t.Helper()
	base := map[string]string{
		"IMPORT_S3_BUCKET": "", "IMPORT_WORK_DIR": "", "IMPORT_ACTOR_COUNT": "", "IMPORT_GLOBAL_COUNT": "", "IMPORT_SOURCES_PER_ITEM": "", "IMPORT_ACTOR_MIB": "", "IMPORT_GLOBAL_MIB": "", "IMPORT_LEGACY_DOC": "", "IMPORT_PROCESSING_ENABLED": "", "IMPORT_WORKER_WAKE_URL": "",
		"DATABASE_URL":           "postgres://u:p@localhost:5432/db?sslmode=disable",
		"JWT_SIGNING_KEY":        strings.Repeat("k", 64),
		"JOIN_CODE_KEY":          base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa1}, 32)),
		"JOIN_CODE_KEY_PREVIOUS": "",
		"CORS_ALLOWED_ORIGINS":   "http://localhost:5173",
		"CLIENT_IP_HEADER":       "CF-Connecting-IP",
		"S3_ENDPOINT":            "",
		"S3_BUCKET":              "",
		"S3_ACCESS_KEY_ID":       "",
		"S3_SECRET_ACCESS_KEY":   "",
		"S3_FORCE_PATH_STYLE":    "",
		"SIGNED_URL_TTL":         "",
		"MEDIA_OWNER_QUOTA_MIB":  "",
		"GOOGLE_CLIENT_ID":       "",
		"GOOGLE_CLIENT_SECRET":   "",
		"GOOGLE_REDIRECT_URI":    "",
		"VITE_GOOGLE_CLIENT_ID":  "",
		"DOCS_PUBLIC":            "",
		"APP_ENV":                "",
	}
	for k, v := range env {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, v)
	}
	return config.Load()
}

func fullMedia() map[string]string {
	return map[string]string{
		"S3_ENDPOINT":          "https://account.r2.cloudflarestorage.com",
		"S3_BUCKET":            "quizzivy-media",
		"S3_ACCESS_KEY_ID":     "key",
		"S3_SECRET_ACCESS_KEY": "secret",
	}
}

// TestPathStyleDefaultsToFalseForR2 is the production-safety half.
//
// It used to default to TRUE while .env.example said "MinIO needs this; R2 does
// not" -- which reads as an instruction to drop the line in production. Dropping
// it took the default, gave R2 path-style addressing, and broke every upload in
// production while dev against MinIO worked perfectly.
func TestPathStyleDefaultsToFalseForR2(t *testing.T) {
	cfg, err := loadWith(t, fullMedia())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.S3ForcePathStyle {
		t.Error("S3_FORCE_PATH_STYLE defaults to true, which breaks R2 in production")
	}
}

func TestPathStyleIsParsedNotCompared(t *testing.T) {
	for _, v := range []string{"0", "False", "FALSE", "f"} {
		cfg, err := loadWith(t, merge(fullMedia(), map[string]string{"S3_FORCE_PATH_STYLE": v}))
		if err != nil {
			t.Fatalf("S3_FORCE_PATH_STYLE=%q was rejected: %v", v, err)
		}
		if cfg.S3ForcePathStyle {
			t.Errorf("S3_FORCE_PATH_STYLE=%q was read as true", v)
		}
	}
	for _, v := range []string{"1", "True", "TRUE", "t", "true"} {
		cfg, err := loadWith(t, merge(fullMedia(), map[string]string{"S3_FORCE_PATH_STYLE": v}))
		if err != nil {
			t.Fatalf("S3_FORCE_PATH_STYLE=%q was rejected: %v", v, err)
		}
		if !cfg.S3ForcePathStyle {
			t.Errorf("S3_FORCE_PATH_STYLE=%q was read as false", v)
		}
	}
}

func TestAnUnparseableBooleanFailsLoudly(t *testing.T) {
	if _, err := loadWith(t, merge(fullMedia(),
		map[string]string{"S3_FORCE_PATH_STYLE": "yes please"})); err == nil {
		t.Error("an unparseable S3_FORCE_PATH_STYLE was accepted")
	}
}

// TestMediaIsAllOrNothing mirrors loadGoogle's rule: half-configured is worse
// than off, because the endpoint exists and fails in a way that looks like the
// provider's fault.
func TestMediaIsAllOrNothing(t *testing.T) {
	partial := fullMedia()
	partial["S3_ENDPOINT"] = ""
	if _, err := loadWith(t, partial); err == nil {
		t.Error("media with no S3_ENDPOINT was accepted; the SDK would talk to AWS")
	}

	for _, missing := range []string{"S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		env := fullMedia()
		env[missing] = ""
		if _, err := loadWith(t, env); err == nil {
			t.Errorf("media configured without %s was accepted", missing)
		}
	}
}

func TestMediaOffIsStillAValidDeployment(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil {
		t.Fatalf("a deployment with no object storage must still start: %v", err)
	}
	if cfg.MediaEnabled() {
		t.Error("MediaEnabled with nothing configured")
	}
}

func TestMediaFullyConfiguredIsEnabled(t *testing.T) {
	cfg, err := loadWith(t, fullMedia())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.MediaEnabled() {
		t.Error("all four variables set and MediaEnabled is false")
	}
}

// TestSignedURLTTLIsRead closes the gap where .env.example documented the
// variable and grep found no reader: changing it did nothing, silently.
func TestSignedURLTTLIsRead(t *testing.T) {
	cfg, err := loadWith(t, merge(fullMedia(), map[string]string{"SIGNED_URL_TTL": "3m"}))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SignedURLTTL != 3*time.Minute {
		t.Errorf("SignedURLTTL = %v, want 3m -- the variable is documented but not read", cfg.SignedURLTTL)
	}
}

func TestSignedURLTTLDefaultsToTenMinutes(t *testing.T) {
	cfg, err := loadWith(t, fullMedia())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SignedURLTTL != 10*time.Minute {
		t.Errorf("SignedURLTTL = %v, want §11.2's 10m", cfg.SignedURLTTL)
	}
}

func TestTheMediaQuotaDefaultsToFiveGibibytes(t *testing.T) {
	for name, env := range map[string]map[string]string{"with object storage": fullMedia(), "without it": nil} {
		cfg, err := loadWith(t, env)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.MediaOwnerQuotaMiB != 5120 {
			t.Errorf("%s: MediaOwnerQuotaMiB = %d, want 5120", name, cfg.MediaOwnerQuotaMiB)
		}
		if got := int64(cfg.MediaOwnerQuotaMiB) << 20; got != mediadomain.DefaultOwnerQuotaBytes {
			t.Errorf("%s: the configured default is %d bytes and the media domain's is %d", name, got, mediadomain.DefaultOwnerQuotaBytes)
		}
	}
}

func TestTheMediaQuotaIsReadAndBounded(t *testing.T) {
	for value, want := range map[string]int{"1": 1, "256": 256, " 2048 ": 2048, "1048576": 1048576} {
		cfg, err := loadWith(t, merge(fullMedia(), map[string]string{"MEDIA_OWNER_QUOTA_MIB": value}))
		if err != nil || cfg.MediaOwnerQuotaMiB != want {
			t.Errorf("MEDIA_OWNER_QUOTA_MIB=%q read as %d (%v), want %d", value, cfg.MediaOwnerQuotaMiB, err, want)
		}
	}
	for _, value := range []string{"-1", "0", "1048577", "5 GB", "5120.5"} {
		if _, err := loadWith(t, merge(fullMedia(), map[string]string{"MEDIA_OWNER_QUOTA_MIB": value})); err == nil {
			t.Errorf("MEDIA_OWNER_QUOTA_MIB=%q was accepted", value)
		}
	}
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func TestTheDocsStayGatedUnlessOptedIn(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil || cfg.DocsPublic {
		t.Fatalf("default: DocsPublic=%v err %v", cfg.DocsPublic, err)
	}
	cfg, err = loadWith(t, map[string]string{"DOCS_PUBLIC": "true"})
	if err != nil || !cfg.DocsPublic {
		t.Fatalf("local opt-in: DocsPublic=%v err %v", cfg.DocsPublic, err)
	}
	if _, err := loadWith(t, map[string]string{"DOCS_PUBLIC": "true", "APP_ENV": "production"}); err == nil {
		t.Fatal("DOCS_PUBLIC=true was accepted in production")
	}
	if _, err := loadWith(t, map[string]string{"DOCS_PUBLIC": "sometimes"}); err == nil {
		t.Fatal("an unparseable DOCS_PUBLIC was accepted")
	}
}

func TestTheJoinCodeKeyIsRequiredAsThirtyTwoBase64Bytes(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 31))
	long := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 33))
	current := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa1}, 32))
	urlSafe := base64.URLEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, 32))
	for label, env := range map[string]map[string]string{
		"a missing key":                   {"JOIN_CODE_KEY": ""},
		"a short key":                     {"JOIN_CODE_KEY": short},
		"a long key":                      {"JOIN_CODE_KEY": long},
		"an undecodable key":              {"JOIN_CODE_KEY": "secret-value-not-base64!!"},
		"a URL-safe key":                  {"JOIN_CODE_KEY": urlSafe},
		"a short previous key":            {"JOIN_CODE_KEY_PREVIOUS": short},
		"an undecodable previous key":     {"JOIN_CODE_KEY_PREVIOUS": "secret-value-not-base64!!"},
		"a previous key equal to the key": {"JOIN_CODE_KEY_PREVIOUS": current},
	} {
		_, err := loadWith(t, env)
		if err == nil {
			t.Errorf("%s was accepted", label)
			continue
		}
		for _, v := range env {
			if v != "" && strings.Contains(err.Error(), v) {
				t.Errorf("%s: the error repeats the value: %v", label, err)
			}
		}
		if label == "a missing key" && !strings.Contains(err.Error(), "openssl rand -base64 32") {
			t.Errorf("the missing-key error does not say how to make one: %v", err)
		}
	}
}

func TestTheJoinCodeKeysAreDecoded(t *testing.T) {
	previous := bytes.Repeat([]byte{0xb2}, 32)
	cfg, err := loadWith(t, map[string]string{"JOIN_CODE_KEY_PREVIOUS": " " + base64.StdEncoding.EncodeToString(previous) + "\n"})
	if err != nil || !bytes.Equal(cfg.JoinCodeKeyPrevious, previous) {
		t.Fatalf("a pasted key with a trailing newline: %x (%v)", cfg.JoinCodeKeyPrevious, err)
	}
	if !bytes.Equal(cfg.JoinCodeKey, bytes.Repeat([]byte{0xa1}, 32)) || !bytes.Equal(cfg.JoinCodeKeyPrevious, previous) {
		t.Errorf("keys decoded as %x and %x", cfg.JoinCodeKey, cfg.JoinCodeKeyPrevious)
	}
	cfg, err = loadWith(t, nil)
	if err != nil || cfg.JoinCodeKeyPrevious != nil {
		t.Errorf("no previous key: %x (%v)", cfg.JoinCodeKeyPrevious, err)
	}
}
