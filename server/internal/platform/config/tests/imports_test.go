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
