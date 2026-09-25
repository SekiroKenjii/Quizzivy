package config_test

import "testing"

func TestImportIntakeRequiresSeparatePrivateStorageAndDiskDirectory(t *testing.T) {
	for _, extra := range []map[string]string{
		{"IMPORT_S3_BUCKET": "quizzivy-media", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports"},
		{"IMPORT_S3_BUCKET": "private-imports"},
		{"IMPORT_WORK_DIR": "/var/lib/quizzivy/imports"},
		{"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "relative"},
		{"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports", "IMPORT_ACTOR_MIB": "-1"},
	} {
		if _, err := loadWith(t, merge(fullMedia(), extra)); err == nil {
			t.Fatalf("unsafe import configuration accepted: %v", extra)
		}
	}
	cfg, err := loadWith(t, merge(fullMedia(), map[string]string{"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports"}))
	if err != nil || cfg.ImportActorMiB != 256 || cfg.ImportGlobalCount != 100 {
		t.Fatalf("valid intake: %+v %v", cfg, err)
	}
}

func TestImportProcessingIsOffUntilSwitchedOnBesideImportStorage(t *testing.T) {
	storage := merge(fullMedia(), map[string]string{"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports"})
	cfg, err := loadWith(t, storage)
	if err != nil || cfg.ImportProcessing {
		t.Fatalf("processing must default to off: %+v %v", cfg.ImportProcessing, err)
	}
	worker := merge(storage, map[string]string{"IMPORT_PROCESSING_ENABLED": "true", "IMPORT_WORKER_WAKE_URL": "http://localhost:8091/wake"})
	cfg, err = loadWith(t, worker)
	if err != nil || !cfg.ImportProcessing || cfg.ImportWorkerWakeURL != "http://localhost:8091/wake" {
		t.Fatalf("processing switched on beside storage: %+v %q %v", cfg.ImportProcessing, cfg.ImportWorkerWakeURL, err)
	}
	for _, wake := range []string{"", "localhost:8091", "ftp://worker/wake", "http://"} {
		if _, err := loadWith(t, merge(worker, map[string]string{"IMPORT_WORKER_WAKE_URL": wake})); err == nil {
			t.Fatalf("processing accepted with wake URL %q", wake)
		}
	}
	if _, err := loadWith(t, merge(fullMedia(), map[string]string{"IMPORT_PROCESSING_ENABLED": "true"})); err == nil {
		t.Fatal("processing accepted without import storage")
	}
	if _, err := loadWith(t, merge(worker, map[string]string{"IMPORT_PROCESSING_ENABLED": "yes please"})); err == nil {
		t.Fatal("a malformed processing switch was accepted")
	}
}
