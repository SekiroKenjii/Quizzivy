package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

func loadImports(cfg *Config) error {
	cfg.ImportBucket = os.Getenv("IMPORT_S3_BUCKET")
	cfg.ImportWorkDir = os.Getenv("IMPORT_WORK_DIR")
	processing, err := getenvBool("IMPORT_PROCESSING_ENABLED", false)
	if err != nil {
		return err
	}
	if cfg.ImportBucket == "" && cfg.ImportWorkDir == "" {
		if processing {
			return fmt.Errorf("IMPORT_PROCESSING_ENABLED requires IMPORT_S3_BUCKET and IMPORT_WORK_DIR: a worker has nothing to process without import storage")
		}
		return nil
	}
	cfg.ImportProcessing = processing
	if processing {
		wake, err := url.Parse(os.Getenv("IMPORT_WORKER_WAKE_URL"))
		if err != nil || (wake.Scheme != "http" && wake.Scheme != "https") || wake.Host == "" || wake.Path != "/wake" {
			return fmt.Errorf("IMPORT_PROCESSING_ENABLED requires IMPORT_WORKER_WAKE_URL, the worker's http(s) .../wake endpoint")
		}
		cfg.ImportWorkerWakeURL = wake.String()
	}
	if !cfg.MediaEnabled() || cfg.ImportBucket == "" || cfg.ImportWorkDir == "" {
		return fmt.Errorf("imports require existing S3 configuration, IMPORT_S3_BUCKET and IMPORT_WORK_DIR together")
	}
	if cfg.ImportBucket == cfg.S3Bucket {
		return fmt.Errorf("IMPORT_S3_BUCKET must be separate from the media bucket")
	}
	if !filepath.IsAbs(cfg.ImportWorkDir) {
		return fmt.Errorf("IMPORT_WORK_DIR must be an absolute path on disk, not tmpfs")
	}
	switch os.Getenv("IMPORT_LEGACY_DOC") {
	case "", "false":
	case "true":
		cfg.ImportLegacyDoc = true
	default:
		return fmt.Errorf("IMPORT_LEGACY_DOC must be true or false")
	}
	return loadImportQuotas(cfg)
}

func loadImportQuotas(cfg *Config) error {
	for _, v := range []struct {
		name          string
		value         *int
		fallback, max int
	}{
		{"IMPORT_ACTOR_COUNT", &cfg.ImportActorCount, 10, 10000},
		{"IMPORT_GLOBAL_COUNT", &cfg.ImportGlobalCount, 100, 100000},
		{"IMPORT_SOURCES_PER_ITEM", &cfg.ImportSourcesPerItem, 32, 1000},
		{"IMPORT_ACTOR_MIB", &cfg.ImportActorMiB, 256, 1048576},
		{"IMPORT_GLOBAL_MIB", &cfg.ImportGlobalMiB, 1024, 104857600},
	} {
		n, err := getenvInt(v.name, v.fallback)
		if err != nil {
			return err
		}
		if n < 1 || n > v.max {
			return fmt.Errorf("%s must be between 1 and %d", v.name, v.max)
		}
		*v.value = n
	}
	return nil
}
