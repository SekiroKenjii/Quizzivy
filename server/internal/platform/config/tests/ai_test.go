package config_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/platform/config"
)

const fixtureKey = "fixture-provider-key-9f3a77c1"

var aiVariables = []string{
	"ANTHROPIC_API_KEY", "AI_TIER_FAST_MODEL", "AI_TIER_CAPABLE_MODEL",
	"AI_TIER_FAST_PRICE_INPUT", "AI_TIER_FAST_PRICE_OUTPUT", "AI_TIER_FAST_PRICE_CACHE_READ",
	"AI_TIER_CAPABLE_PRICE_INPUT", "AI_TIER_CAPABLE_PRICE_OUTPUT", "AI_TIER_CAPABLE_PRICE_CACHE_READ",
}

func fullAI() map[string]string {
	return map[string]string{
		"ANTHROPIC_API_KEY":                fixtureKey,
		"AI_TIER_FAST_MODEL":               "fast-model-fixture",
		"AI_TIER_CAPABLE_MODEL":            "capable-model-fixture",
		"AI_TIER_FAST_PRICE_INPUT":         "0.10",
		"AI_TIER_FAST_PRICE_OUTPUT":        "0.5",
		"AI_TIER_FAST_PRICE_CACHE_READ":    "0.01",
		"AI_TIER_CAPABLE_PRICE_INPUT":      "2",
		"AI_TIER_CAPABLE_PRICE_OUTPUT":     "10.000001",
		"AI_TIER_CAPABLE_PRICE_CACHE_READ": "0.2",
	}
}

func without(env map[string]string, name string) map[string]string {
	out := merge(env, nil)
	delete(out, name)
	return out
}

func TestWithTheKeyUnsetTheAIBlockIsOffAndNothingElseIsRead(t *testing.T) {
	garbage := map[string]string{}
	for _, name := range aiVariables {
		garbage[name] = "not read"
	}
	garbage["ANTHROPIC_API_KEY"] = ""
	for _, env := range []map[string]string{nil, garbage, {"ANTHROPIC_API_KEY": "  \t "}} {
		cfg, err := loadWith(t, env)
		if err != nil {
			t.Fatalf("Load with no key refused: %v", err)
		}
		if cfg.AIEnabled() || cfg.AI.Enabled() || cfg.AI != (config.AI{}) {
			t.Fatalf("the AI block is not the zero value with no key: %+v", cfg.AI)
		}
	}
}

func TestWithTheKeySetBothTiersAndSixPricesAreRead(t *testing.T) {
	cfg, err := loadWith(t, fullAI())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AIEnabled() || cfg.AI.APIKey.Reveal() != fixtureKey {
		t.Fatalf("the AI block is off or lost the key: enabled=%v", cfg.AIEnabled())
	}
	want := config.AI{
		APIKey:  cfg.AI.APIKey,
		Fast:    config.AITier{Model: "fast-model-fixture", Prices: config.TierPrices{Input: 100_000, Output: 500_000, CacheRead: 10_000}},
		Capable: config.AITier{Model: "capable-model-fixture", Prices: config.TierPrices{Input: 2_000_000, Output: 10_000_001, CacheRead: 200_000}},
	}
	if cfg.AI != want {
		t.Fatalf("AI = %+v, want %+v", cfg.AI, want)
	}
}

func TestSurroundingWhitespaceIsTrimmedFromTheKeyAndTheModels(t *testing.T) {
	env := merge(fullAI(), map[string]string{
		"ANTHROPIC_API_KEY": fixtureKey + "\n", "AI_TIER_FAST_MODEL": " fast-model-fixture ", "AI_TIER_FAST_PRICE_INPUT": " 0.10 ",
	})
	cfg, err := loadWith(t, env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AI.APIKey.Reveal() != fixtureKey || cfg.AI.Fast.Model != "fast-model-fixture" || cfg.AI.Fast.Prices.Input != 100_000 {
		t.Fatalf("whitespace was kept: %+v", cfg.AI.Fast)
	}
}

func TestAMissingTierOrPriceRefusesStartNamingTheVariable(t *testing.T) {
	for _, name := range aiVariables[1:] {
		_, err := loadWith(t, without(fullAI(), name))
		if err == nil {
			t.Fatalf("Load accepted the key without %s", name)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal for a missing %s does not name it: %v", name, err)
		}
		if strings.Contains(err.Error(), fixtureKey) {
			t.Errorf("the refusal for a missing %s echoes the key: %v", name, err)
		}
	}
}

func TestAMalformedModelOrPriceIsRefusedWithoutTheKey(t *testing.T) {
	cases := []struct{ name, value string }{
		{"AI_TIER_FAST_MODEL", "two words"},
		{"AI_TIER_CAPABLE_MODEL", "tab\tseparated"},
		{"AI_TIER_FAST_PRICE_INPUT", "cheap"},
		{"AI_TIER_FAST_PRICE_OUTPUT", "-1"},
		{"AI_TIER_FAST_PRICE_CACHE_READ", "1e3"},
		{"AI_TIER_CAPABLE_PRICE_INPUT", "$2"},
		{"AI_TIER_CAPABLE_PRICE_OUTPUT", "0.1234567"},
		{"AI_TIER_CAPABLE_PRICE_CACHE_READ", "2,5"},
		{"AI_TIER_FAST_PRICE_INPUT", "0"},
		{"AI_TIER_FAST_PRICE_INPUT", "0.0005"},
		{"AI_TIER_CAPABLE_PRICE_OUTPUT", "1000.01"},
		{"AI_TIER_CAPABLE_PRICE_OUTPUT", "12345"},
	}
	for _, c := range cases {
		_, err := loadWith(t, merge(fullAI(), map[string]string{c.name: c.value}))
		if err == nil {
			t.Errorf("Load accepted %s=%q", c.name, c.value)
			continue
		}
		if !strings.Contains(err.Error(), c.name) {
			t.Errorf("the refusal for %s=%q does not name the variable: %v", c.name, c.value, err)
		}
		if strings.Contains(err.Error(), fixtureKey) {
			t.Errorf("the refusal for %s echoes the key: %v", c.name, err)
		}
	}
}

func TestTheBoundsOfAPriceAreAccepted(t *testing.T) {
	for value, want := range map[string]config.MicroUSD{"0.001": 1_000, "1000": 1_000_000_000, "0.100000": 100_000, "07.5": 7_500_000} {
		cfg, err := loadWith(t, merge(fullAI(), map[string]string{"AI_TIER_FAST_PRICE_INPUT": value}))
		if err != nil || cfg.AI.Fast.Prices.Input != want {
			t.Errorf("price %q = %d (err %v), want %d", value, cfg.AI.Fast.Prices.Input, err, want)
		}
	}
	for _, value := range []string{"7.", ".5", "1 000", "0x10", "+2"} {
		if _, err := loadWith(t, merge(fullAI(), map[string]string{"AI_TIER_FAST_PRICE_INPUT": value})); err == nil {
			t.Errorf("Load accepted the price %q", value)
		}
	}
}

func TestAKeyWithWhitespaceInsideIsRefusedWithoutBeingEchoed(t *testing.T) {
	for _, key := range []string{"fixture key 9f3a", "fixture\nkey-9f3a", "fixture\x01key-9f3a"} {
		_, err := loadWith(t, merge(fullAI(), map[string]string{"ANTHROPIC_API_KEY": key}))
		if err == nil {
			t.Fatalf("Load accepted a key with whitespace or a control character: %q", key)
		}
		if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") || strings.Contains(err.Error(), "fixture") {
			t.Errorf("the refusal must name the variable and never echo the value: %v", err)
		}
	}
}

func TestTheKeyRendersRedactedWhereverTheConfigIsPrinted(t *testing.T) {
	cfg, err := loadWith(t, fullAI())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("config", "ai", cfg.AI, "key", cfg.AI.APIKey)
	marshalled, err := json.Marshal(cfg.AI)
	if err != nil {
		t.Fatal(err)
	}
	rendered := []string{
		fmt.Sprintf("%v", cfg.AI), fmt.Sprintf("%+v", cfg.AI), fmt.Sprintf("%#v", cfg.AI),
		fmt.Sprint(cfg.AI.APIKey), fmt.Sprintf("%q", cfg.AI.APIKey), fmt.Sprint(cfg), fmt.Sprintf("%+v", cfg),
		logged.String(), string(marshalled),
	}
	for _, text := range rendered {
		if strings.Contains(text, fixtureKey) || strings.Contains(text, "9f3a77c1") {
			t.Errorf("the key reached a rendering of the configuration: %s", text)
		}
	}
	if !strings.Contains(rendered[0], "[redacted]") {
		t.Errorf("a printed AI block does not show where the key was: %s", rendered[0])
	}
}

func TestEnvExampleListsEveryAIVariableWithNoValue(t *testing.T) {
	lines := strings.Split(repoFile(t, ".env.example"), "\n")
	for _, name := range aiVariables {
		if !slices.Contains(lines, name+"=") {
			t.Errorf(".env.example does not list %s with an empty value", name)
		}
	}
}
