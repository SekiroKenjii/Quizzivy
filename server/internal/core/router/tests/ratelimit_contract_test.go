package router_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/core/router"
	"quizzivy/internal/platform/ratelimit"
)

func rulesOf(t *testing.T, pattern string, raw any) map[string][]ratelimit.Rule {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var blocks map[string]map[string]int
	if err := json.Unmarshal(encoded, &blocks); err != nil {
		t.Fatalf("%s: x-rate-limit is not a map of buckets to windows: %v", pattern, err)
	}
	out := map[string][]ratelimit.Rule{}
	for name, windows := range blocks {
		for window, n := range windows {
			switch window {
			case "perMinute":
				out[name] = append(out[name], ratelimit.Rule{Burst: n, Window: time.Minute})
			case "perHour":
				out[name] = append(out[name], ratelimit.Rule{Burst: n, Window: time.Hour})
			default:
				t.Errorf("%s: bucket %s names an unknown window %q", pattern, name, window)
			}
		}
	}
	return out
}

func sortedRules(rules []ratelimit.Rule) []ratelimit.Rule {
	out := slices.Clone(rules)
	slices.SortFunc(out, func(a, b ratelimit.Rule) int { return int(a.Window - b.Window) })
	return out
}

func TestEveryRateLimitBlockMatchesTheRegistry(t *testing.T) {
	reg := router.RateLimits()
	checked := 0
	eachOperation(freshSpec(t), func(pattern string, op *openapi3.Operation) {
		raw, ok := op.Extensions["x-rate-limit"]
		if !ok {
			return
		}
		checked++
		route, ok := reg.Lookup(pattern)
		if !ok {
			t.Errorf("%s declares x-rate-limit but has no registry entry", pattern)
			return
		}
		want := rulesOf(t, pattern, raw)
		got := map[string][]ratelimit.Rule{"perIp": route.PerIP.Rules()}
		for _, bucket := range route.Keyed {
			got[bucket.Name] = bucket.Limiter.Rules()
		}
		if len(got) != len(want) {
			t.Errorf("%s: the registry has buckets %v, the contract %v", pattern, got, want)
		}
		for name, rules := range want {
			if !slices.Equal(sortedRules(got[name]), sortedRules(rules)) {
				t.Errorf("%s: bucket %s is %v in the registry, %v in the contract", pattern, name, got[name], rules)
			}
		}
	})
	if checked != 7 {
		t.Errorf("%d operations declare x-rate-limit, want 7: the join preview and join, login, Google, refresh, logout and the status", checked)
	}
}
