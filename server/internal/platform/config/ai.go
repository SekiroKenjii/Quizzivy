package config

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Secret is a credential that renders as "[redacted]" through fmt, slog and
// encoding verbs, so a configuration printed in a log line, an error or a panic
// cannot carry it. Reveal is the one way to read the value.
type Secret string

// Reveal returns the credential itself, for the one adapter that sends it.
func (s Secret) Reveal() string { return string(s) }

func (Secret) String() string { return "[redacted]" }

func (Secret) GoString() string { return "[redacted]" }

func (Secret) LogValue() slog.Value { return slog.StringValue("[redacted]") }

func (Secret) MarshalText() ([]byte, error) { return []byte("[redacted]"), nil }

// MicroUSD is an amount of money in millionths of a US dollar. A price per
// million tokens held this way makes a credit (0.001 US dollars, or 1,000
// MicroUSD) a whole number of tokens times an integer, with no floating point
// to round a call across a credit boundary.
type MicroUSD int64

// TierPrices are the provider's list prices for one tier, in MicroUSD per
// million tokens.
type TierPrices struct {
	Input     MicroUSD
	Output    MicroUSD
	CacheRead MicroUSD
}

// AITier is one tier of the AI assistant: the model that serves it and the
// prices its usage is metered at.
type AITier struct {
	Model  string
	Prices TierPrices
}

// AI is the AI assistant's provider configuration. The zero value is off:
// without APIKey nothing else is read and every AI operation answers 501.
type AI struct {
	APIKey  Secret
	Fast    AITier
	Capable AITier
}

// Enabled reports whether the provider is configured.
func (a AI) Enabled() bool { return a.APIKey != "" }

// AIEnabled reports whether the AI assistant is configured. It reads
// configuration only and never asks the provider.
func (c Config) AIEnabled() bool { return c.AI.Enabled() }

const (
	minPriceMicroUSD = 1_000
	maxPriceMicroUSD = 1_000_000_000
)

var priceFormat = regexp.MustCompile(`^(\d{1,4})(?:\.(\d{1,6}))?$`)

func loadAI(cfg *Config) error {
	key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if key == "" {
		return nil
	}
	if strings.IndexFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("ANTHROPIC_API_KEY must be a single token with no whitespace or control characters; re-copy it from the provider console")
	}
	cfg.AI.APIKey = Secret(key)
	var err error
	if cfg.AI.Fast, err = loadAITier("FAST"); err != nil {
		return err
	}
	cfg.AI.Capable, err = loadAITier("CAPABLE")
	return err
}

func loadAITier(tier string) (AITier, error) {
	modelName := "AI_TIER_" + tier + "_MODEL"
	model := strings.TrimSpace(os.Getenv(modelName))
	if model == "" || strings.ContainsFunc(model, unicode.IsSpace) {
		return AITier{}, fmt.Errorf("ANTHROPIC_API_KEY is set, so %s is required: the provider's model identifier for the %s tier, with no spaces", modelName, strings.ToLower(tier))
	}
	loaded := AITier{Model: model}
	for _, price := range []struct {
		name   string
		target *MicroUSD
	}{
		{"INPUT", &loaded.Prices.Input},
		{"OUTPUT", &loaded.Prices.Output},
		{"CACHE_READ", &loaded.Prices.CacheRead},
	} {
		name := "AI_TIER_" + tier + "_PRICE_" + price.name
		value, err := parsePricePerMillion(name)
		if err != nil {
			return AITier{}, err
		}
		*price.target = value
	}
	return loaded, nil
}

func parsePricePerMillion(name string) (MicroUSD, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("ANTHROPIC_API_KEY is set, so %s is required: the provider's list price in US dollars per million tokens, such as 0.10", name)
	}
	parts := priceFormat.FindStringSubmatch(raw)
	if parts == nil {
		return 0, fmt.Errorf("%s must be a plain decimal number of US dollars per million tokens with at most six decimals, such as 0.10", name)
	}
	micros, err := strconv.ParseInt(parts[1]+parts[2]+strings.Repeat("0", 6-len(parts[2])), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a plain decimal number of US dollars per million tokens with at most six decimals, such as 0.10", name)
	}
	price := MicroUSD(micros)
	if price < minPriceMicroUSD || price > maxPriceMicroUSD {
		return 0, fmt.Errorf("%s must be between 0.001 and 1000 US dollars per million tokens; a price per token or per thousand tokens would undercount the pool", name)
	}
	return price, nil
}
