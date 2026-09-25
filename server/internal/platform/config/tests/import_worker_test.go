package config_test

import (
	"strings"
	"testing"

	"quizzivy/internal/platform/config"
)

func workerEnvironment(t *testing.T, overrides map[string]string) {
	t.Helper()
	env := merge(fullMedia(), map[string]string{
		"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports",
		"IMPORT_DOCKER_BINARY": "/usr/bin/docker", "IMPORT_CONVERTER_IMAGE": "sha256:" + strings.Repeat("a", 64),
		"IMPORT_WORKER_GLOBAL_LIMIT": "", "IMPORT_WORKER_ACTOR_LIMIT": "",
		"IMPORT_ARTIFACT_ACTOR_MIB": "", "IMPORT_ARTIFACT_GLOBAL_MIB": "", "IMPORT_ARTIFACT_SETS_PER_ITEM": "",
	})
	_, _ = loadWith(t, merge(env, overrides))
	t.Setenv("JWT_SIGNING_KEY", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
}

func TestImportWorkerNeedsNoAPISecretsAndDefaultsToOneLease(t *testing.T) {
	workerEnvironment(t, nil)
	cfg, err := config.LoadImportWorker()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GlobalWorkers != 1 || cfg.ActorWorkers != 1 || cfg.ActorArtifactMiB != 512 || cfg.GlobalArtifactMiB != 2048 || cfg.SetsPerImport != 200 {
		t.Fatal("unexpected worker defaults")
	}
	if len(cfg.Base.JWTSigningKey) != 0 {
		t.Fatal("worker loaded API signing credentials")
	}
}

func TestImportWorkerRejectsUnboundedOrMutableRuntime(t *testing.T) {
	for _, extra := range []map[string]string{
		{"DATABASE_URL": ""},
		{"IMPORT_S3_BUCKET": "", "IMPORT_WORK_DIR": ""},
		{"IMPORT_DOCKER_BINARY": "docker"},
		{"IMPORT_CONVERTER_IMAGE": "converter:latest"},
		{"IMPORT_WORKER_GLOBAL_LIMIT": "0"},
		{"IMPORT_WORKER_ACTOR_LIMIT": "2"},
		{"IMPORT_ARTIFACT_ACTOR_MIB": "2049"},
		{"IMPORT_ARTIFACT_GLOBAL_MIB": "unlimited"},
		{"IMPORT_ARTIFACT_SETS_PER_ITEM": "1001"},
	} {
		workerEnvironment(t, extra)
		if _, err := config.LoadImportWorker(); err == nil {
			t.Fatalf("unsafe worker configuration accepted: %v", extra)
		}
	}
}
