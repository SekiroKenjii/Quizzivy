package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"quizzivy/internal/platform/secrets"
)

// Email is the outgoing mail's configuration. RESEND_API_KEY, EMAIL_FROM and
// APP_URL are one all-or-nothing group, required in production. Without the
// group outside production the messages go to the sink directory SinkDir
// names instead.
type Email struct {
	APIKey     Secret
	From       string
	AppURL     string
	ReplyTo    string
	DailyLimit int
	SendRate   int
	SinkDir    string
}

// Enabled reports whether the Resend group is configured.
func (e Email) Enabled() bool { return e.APIKey != "" }

// SendInterval is the pause between two sends at SendRate a second.
func (e Email) SendInterval() time.Duration {
	if e.SendRate < 1 {
		return 0
	}
	return time.Second / time.Duration(e.SendRate)
}

const (
	defaultEmailDailyLimit = 100
	maxEmailDailyLimit     = 100000
	defaultEmailSendRate   = 2
	maxEmailSendRate       = 100
	defaultEmailSinkDir    = "tmp/email-sink"
)

func loadEmail(cfg *Config) error {
	if err := loadResend(cfg); err != nil {
		return err
	}
	return loadEmailLimits(cfg)
}

func loadResend(cfg *Config) error {
	e := &cfg.Email
	key := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	from := strings.TrimSpace(os.Getenv("EMAIL_FROM"))
	appURL := strings.TrimSpace(os.Getenv("APP_URL"))
	e.ReplyTo = strings.TrimSpace(os.Getenv("EMAIL_REPLY_TO"))
	e.SinkDir = strings.TrimSpace(os.Getenv("EMAIL_SINK_DIR"))
	production := cfg.Env == productionEnv
	set := 0
	for _, v := range []string{key, from, appURL} {
		if v != "" {
			set++
		}
	}
	if err := checkEmailGroup(set, production, e.SinkDir); err != nil {
		return err
	}
	if e.SinkDir == "" {
		e.SinkDir = defaultEmailSinkDir
	}
	if e.ReplyTo != "" {
		if err := checkMailbox("EMAIL_REPLY_TO", e.ReplyTo); err != nil {
			return err
		}
	}
	if set == 0 {
		return nil
	}
	if strings.ContainsAny(key, " \t\r\n") {
		return fmt.Errorf("RESEND_API_KEY must be a single token with no whitespace; re-copy it from the provider console")
	}
	if err := checkMailbox("EMAIL_FROM", from); err != nil {
		return err
	}
	origin, err := parseAppOrigin(appURL, production)
	if err != nil {
		return err
	}
	e.APIKey, e.From, e.AppURL = Secret(key), from, origin
	return nil
}

func checkEmailGroup(set int, production bool, sinkDir string) error {
	switch {
	case set != 0 && set != 3:
		return fmt.Errorf("email needs RESEND_API_KEY, EMAIL_FROM and APP_URL together, or none of them")
	case set == 0 && production:
		return fmt.Errorf("RESEND_API_KEY, EMAIL_FROM and APP_URL are required in production: without them the sink would take every message")
	case sinkDir != "" && production:
		return fmt.Errorf("EMAIL_SINK_DIR must not be set in production: it writes every message, links included, to disk instead of sending it")
	}
	return nil
}

func checkMailbox(name, raw string) error {
	if strings.ContainsFunc(raw, unicode.IsControl) {
		return fmt.Errorf("%s must not contain a line break or other control character", name)
	}
	if _, err := mail.ParseAddress(raw); err != nil {
		return fmt.Errorf("%s must be one email address, such as Quizzivy <no-reply@quizzivy.com>", name)
	}
	return nil
}

func parseAppOrigin(raw string, production bool) (string, error) {
	if strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return "", fmt.Errorf("APP_URL must not contain whitespace or a control character")
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil || (u.Scheme != schemeHTTPS && u.Scheme != "http") || u.Host == "" || u.User != nil:
		return "", fmt.Errorf("APP_URL must be an origin such as https://app.quizzivy.com")
	case (u.Path != "" && u.Path != "/") || strings.ContainsAny(raw, "?#"):
		return "", fmt.Errorf("APP_URL must be an origin such as https://app.quizzivy.com, with no path, query or fragment")
	case production && u.Scheme != schemeHTTPS:
		return "", fmt.Errorf("APP_URL must be https in production: a set-password link travels in it")
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}

func loadEmailLimits(cfg *Config) error {
	e := &cfg.Email
	var err error
	if e.DailyLimit, err = getenvInt("EMAIL_DAILY_LIMIT", defaultEmailDailyLimit); err != nil {
		return err
	}
	if e.DailyLimit < 1 || e.DailyLimit > maxEmailDailyLimit {
		return fmt.Errorf("EMAIL_DAILY_LIMIT must be between 1 and %d", maxEmailDailyLimit)
	}
	if e.SendRate, err = getenvInt("EMAIL_SEND_RATE", defaultEmailSendRate); err != nil {
		return err
	}
	if e.SendRate < 1 || e.SendRate > maxEmailSendRate {
		return fmt.Errorf("EMAIL_SEND_RATE must be between 1 and %d a second", maxEmailSendRate)
	}
	return nil
}

func loadSecretsKey(cfg *Config) error {
	raw := strings.TrimSpace(os.Getenv("SECRETS_KEY"))
	if raw == "" {
		if cfg.Env == productionEnv {
			return fmt.Errorf("SECRETS_KEY is required in production: standard base64 of %d random bytes; generate one with: openssl rand -base64 32", secrets.KeySize)
		}
		return nil
	}
	key, err := secrets.ParseKey(raw)
	if err != nil {
		return err
	}
	cfg.SecretsKey = key
	return nil
}

func loadAppLocation(cfg *Config) error {
	var err error
	cfg.AppLocation, err = AppLocation()
	return err
}

func loadEmailGroup(cfg *Config) error {
	for _, load := range []func(*Config) error{loadSecretsKey, loadAppLocation, loadEmail} {
		if err := load(cfg); err != nil {
			return err
		}
	}
	return nil
}
