package config

import (
	"os"
	"strings"

	"quizzivy/internal/shared/limits"
)

// loadLimitEnv records the deploy variables that override a usage limit's
// hosting-plan default (D-060): only a variable the deploy sets, so a figure the
// loader fills in when it is unset leaves the hosting plan's default in place.
// The values were already checked by the loader that owns each variable.
func loadLimitEnv(cfg *Config) {
	env := limits.Env{}
	set := func(name string) bool { return strings.TrimSpace(os.Getenv(name)) != "" }
	if set("MEDIA_OWNER_QUOTA_MIB") && cfg.MediaOwnerQuotaMiB > 0 {
		env[limits.MediaPerTeacherBytes] = int64(cfg.MediaOwnerQuotaMiB) * limits.MiB
	}
	if set("IMPORT_GLOBAL_MIB") && cfg.ImportGlobalMiB > 0 {
		env[limits.ImportBytes] = int64(cfg.ImportGlobalMiB) * limits.MiB
	}
	if set("IMPORT_ACTOR_MIB") && cfg.ImportActorMiB > 0 {
		env[limits.ImportPerTeacherBytes] = int64(cfg.ImportActorMiB) * limits.MiB
	}
	if set("EMAIL_DAILY_LIMIT") && cfg.Email.DailyLimit > 0 {
		env[limits.EmailsPerDay] = int64(cfg.Email.DailyLimit)
	}
	cfg.Limits = env
}
