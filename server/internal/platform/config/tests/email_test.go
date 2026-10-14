package config_test

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/config"
)

var emailVariables = []string{
	"RESEND_API_KEY", "EMAIL_FROM", "APP_URL", "EMAIL_REPLY_TO", "EMAIL_DAILY_LIMIT", "EMAIL_SEND_RATE",
	"EMAIL_SINK_DIR", "SECRETS_KEY", "APP_TIMEZONE",
}

var secretsKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

func fullEmail() map[string]string {
	return map[string]string{
		"RESEND_API_KEY": "re_secret_value", "EMAIL_FROM": "Quizzivy <no-reply@quizzivy.com>",
		"APP_URL": "https://app.quizzivy.com", "SECRETS_KEY": secretsKey,
	}
}

func TestWithoutTheEmailGroupOutsideProductionTheSinkTakesTheMail(t *testing.T) {
	cfg, err := loadWith(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := cfg.Email
	if e.Enabled() || e.SinkDir != "tmp/email-sink" || e.DailyLimit != 100 || e.SendRate != 2 || e.SendInterval() != 500*time.Millisecond {
		t.Fatalf("defaults: %+v", e)
	}
	if cfg.SecretsKey != nil || cfg.AppLocation == nil || cfg.AppLocation.String() != "Asia/Ho_Chi_Minh" {
		t.Fatalf("secrets key %v, location %v", cfg.SecretsKey, cfg.AppLocation)
	}
}

func TestTheEmailGroupLoadsAndKeepsTheKeyOutOfEveryPrintedForm(t *testing.T) {
	cfg, err := loadWith(t, merge(fullEmail(), map[string]string{
		"EMAIL_REPLY_TO": "frontdesk@quizzivy.com", "EMAIL_DAILY_LIMIT": "500", "EMAIL_SEND_RATE": "5",
		"APP_URL": "https://app.quizzivy.com/", "APP_TIMEZONE": "Asia/Bangkok",
	}))
	if err != nil {
		t.Fatal(err)
	}
	e := cfg.Email
	if !e.Enabled() || e.From != "Quizzivy <no-reply@quizzivy.com>" || e.AppURL != "https://app.quizzivy.com" || e.ReplyTo != "frontdesk@quizzivy.com" ||
		e.DailyLimit != 500 || e.SendRate != 5 || e.SendInterval() != 200*time.Millisecond || len(cfg.SecretsKey) != 32 || cfg.AppLocation.String() != "Asia/Bangkok" {
		t.Fatalf("loaded: %+v", e)
	}
	if e.APIKey.Reveal() != "re_secret_value" {
		t.Fatal("the key is lost")
	}
	if printed := fmt.Sprintf("%v %+v %#v", e.APIKey, e, e); strings.Contains(printed, "re_secret_value") {
		t.Errorf("the key is printed: %s", printed)
	}
}

func TestAPartOfTheEmailGroupIsRefusedByName(t *testing.T) {
	for _, drop := range []string{"RESEND_API_KEY", "EMAIL_FROM", "APP_URL"} {
		env := fullEmail()
		env[drop] = ""
		_, err := loadWith(t, env)
		if err == nil || !strings.Contains(err.Error(), "RESEND_API_KEY, EMAIL_FROM and APP_URL together") {
			t.Errorf("without %s: %v", drop, err)
		}
	}
}

func TestProductionRefusesTheSinkAndAPartialOrMissingGroup(t *testing.T) {
	production := map[string]string{"APP_ENV": "production", "SECRETS_KEY": secretsKey}
	if _, err := loadWith(t, production); err == nil || !strings.Contains(err.Error(), "required in production") {
		t.Errorf("production without the group: %v", err)
	}
	if _, err := loadWith(t, merge(merge(fullEmail(), production), map[string]string{"EMAIL_SINK_DIR": "/tmp/sink"})); err == nil || !strings.Contains(err.Error(), "EMAIL_SINK_DIR") {
		t.Errorf("production with the sink: %v", err)
	}
	if _, err := loadWith(t, merge(merge(fullEmail(), production), map[string]string{"APP_URL": "http://app.quizzivy.com", "S3_ENDPOINT": ""})); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("production over http: %v", err)
	}
	noKey := merge(fullEmail(), production)
	noKey["SECRETS_KEY"] = ""
	if _, err := loadWith(t, noKey); err == nil || !strings.Contains(err.Error(), "SECRETS_KEY is required in production") {
		t.Errorf("production without SECRETS_KEY: %v", err)
	}
	if _, err := loadWith(t, merge(fullEmail(), production)); err != nil {
		t.Errorf("production with everything: %v", err)
	}
}

func TestBadEmailValuesAreRefusedWithoutRepeatingTheKey(t *testing.T) {
	for name, override := range map[string]map[string]string{
		"an app url with a path":     {"APP_URL": "https://app.quizzivy.com/app"},
		"an app url that is no url":  {"APP_URL": "app.quizzivy.com"},
		"a key with whitespace":      {"RESEND_API_KEY": "re_secret value"},
		"a zero daily limit":         {"EMAIL_DAILY_LIMIT": "0"},
		"a daily limit that is text": {"EMAIL_DAILY_LIMIT": "many"},
		"a zero send rate":           {"EMAIL_SEND_RATE": "0"},
		"a send rate over 100":       {"EMAIL_SEND_RATE": "101"},
		"a reply-to that is no mail": {"EMAIL_REPLY_TO": "front desk"},
		"a secrets key not base64":   {"SECRETS_KEY": "not-base64!"},
		"a short secrets key":        {"SECRETS_KEY": base64.StdEncoding.EncodeToString([]byte("short"))},
		"an unknown timezone":        {"APP_TIMEZONE": "Mars/Olympus"},
	} {
		_, err := loadWith(t, merge(fullEmail(), override))
		if err == nil {
			t.Errorf("%s was accepted", name)
			continue
		}
		for _, secret := range []string{"re_secret", secretsKey, override["SECRETS_KEY"]} {
			if secret != "" && strings.Contains(err.Error(), secret) {
				t.Errorf("%s: the error repeats a secret: %v", name, err)
			}
		}
	}
}

func TestTheImportWorkerLoadsTheSameEmailGroup(t *testing.T) {
	workerEnvironment(t, merge(fullEmail(), map[string]string{"APP_ENV": "production", "APP_TIMEZONE": "Asia/Bangkok"}))
	cfg, err := config.LoadImportWorker()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Base.Email.Enabled() || len(cfg.Base.SecretsKey) != 32 || cfg.Base.AppLocation.String() != "Asia/Bangkok" {
		t.Fatalf("worker: %+v", cfg.Base.Email)
	}
	workerEnvironment(t, map[string]string{"APP_ENV": "production"})
	if _, err := config.LoadImportWorker(); err == nil {
		t.Error("a production worker without the email group started")
	}
}
