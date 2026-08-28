package config

import (
	"strings"
	"testing"
	"time"
)

// loadWith runs Load with a minimal valid environment plus the overrides given.
// t.Setenv restores everything afterwards and forbids parallel tests, which is
// what makes touching the process environment safe here.
func loadWith(t *testing.T, env map[string]string) (Config, error) {
	t.Helper()
	base := map[string]string{
		"DATABASE_URL":          "postgres://u:p@localhost:5432/db?sslmode=disable",
		"JWT_SIGNING_KEY":       strings.Repeat("k", 64),
		"CORS_ALLOWED_ORIGINS":  "http://localhost:5173",
		"CLIENT_IP_HEADER":      "CF-Connecting-IP",
		"S3_ENDPOINT":           "",
		"S3_BUCKET":             "",
		"S3_ACCESS_KEY_ID":      "",
		"S3_SECRET_ACCESS_KEY":  "",
		"S3_FORCE_PATH_STYLE":   "",
		"SIGNED_URL_TTL":        "",
		"GOOGLE_CLIENT_ID":      "",
		"GOOGLE_CLIENT_SECRET":  "",
		"GOOGLE_REDIRECT_URI":   "",
		"VITE_GOOGLE_CLIENT_ID": "",
	}
	for k, v := range env {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, v)
	}
	return Load()
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
	// The old form was `getenv(key, "true") != "false"`, under which every one
	// of these quietly meant path-style.
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
	// Every other input here fails loudly; this one used to substitute a
	// default silently.
	if _, err := loadWith(t, merge(fullMedia(),
		map[string]string{"S3_FORCE_PATH_STYLE": "yes please"})); err == nil {
		t.Error("an unparseable S3_FORCE_PATH_STYLE was accepted")
	}
}

// TestMediaIsAllOrNothing mirrors loadGoogle's rule: half-configured is worse
// than off, because the endpoint exists and fails in a way that looks like the
// provider's fault.
func TestMediaIsAllOrNothing(t *testing.T) {
	// The specific gap this closes: bucket and credentials but no endpoint used
	// to report media as ENABLED, and the SDK then resolved against real AWS S3
	// rather than R2 or MinIO.
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
