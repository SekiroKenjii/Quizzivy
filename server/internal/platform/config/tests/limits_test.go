package config_test

import (
	"testing"

	"quizzivy/internal/shared/limits"
)

func TestAnUnsetVariableLeavesTheHostingPlanDefault(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Limits) != 0 {
		t.Fatalf("limits = %v, want none: the loader's own fallbacks are not the deploy's choice", cfg.Limits)
	}
}

func TestADeploySetVariableOverridesItsLimit(t *testing.T) {
	env := fullMedia()
	for k, v := range map[string]string{
		"IMPORT_S3_BUCKET": "private-imports", "IMPORT_WORK_DIR": "/var/lib/quizzivy/imports",
		"MEDIA_OWNER_QUOTA_MIB": "2048", "IMPORT_GLOBAL_MIB": "512", "IMPORT_ACTOR_MIB": "64", "EMAIL_DAILY_LIMIT": "40",
	} {
		env[k] = v
	}
	cfg, err := loadWith(t, env)
	if err != nil {
		t.Fatal(err)
	}
	want := limits.Env{
		limits.MediaPerTeacherBytes:  2048 * limits.MiB,
		limits.ImportBytes:           512 * limits.MiB,
		limits.ImportPerTeacherBytes: 64 * limits.MiB,
		limits.EmailsPerDay:          40,
	}
	if len(cfg.Limits) != len(want) {
		t.Fatalf("limits = %v, want %v", cfg.Limits, want)
	}
	for name, v := range want {
		if cfg.Limits[name] != v {
			t.Errorf("%s = %d, want %d", name, cfg.Limits[name], v)
		}
	}
}
