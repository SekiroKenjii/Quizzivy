//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"quizzivy/internal/core/wiring"
	aiports "quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/platform/config"
)

func withPrices(c *config.Config) {
	c.AI.Fast.Prices = config.TierPrices{Input: 1_000_000, Output: 5_000_000, CacheRead: 100_000}
	c.AI.Capable.Prices = config.TierPrices{Input: 3_000_000, Output: 15_000_000, CacheRead: 300_000}
}

func bootAssisted(t *testing.T, completer aiports.Completer) *world {
	t.Helper()
	return assemble(t, slog.New(slog.DiscardHandler), []wiring.Option{wiring.WithCompleter(completer)}, withPrices)
}

type assistant struct {
	mu       sync.Mutex
	requests []aiports.Request
	usage    aiports.Usage
	failure  error
}

func newAssistant() *assistant {
	return &assistant{usage: aiports.Usage{InputTokens: 1200, OutputTokens: 300}}
}

func (a *assistant) Complete(_ context.Context, req aiports.Request) (aiports.Response, error) {
	a.mu.Lock()
	a.requests = append(a.requests, req)
	failure := a.failure
	a.mu.Unlock()
	if failure != nil {
		return aiports.Response{}, failure
	}
	return aiports.Response{Object: answerTo(req), Usage: a.usage, RequestID: "req_e2e"}, nil
}

func (a *assistant) sent() []aiports.Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]aiports.Request(nil), a.requests...)
}

func (a *assistant) failWith(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failure = err
}

func textOf(req aiports.Request) string {
	var all strings.Builder
	all.WriteString(req.System)
	for _, block := range req.Blocks {
		all.WriteString("\n")
		all.WriteString(block.Text)
	}
	return all.String()
}

var answers = map[string]string{
	"questions":   `{"questions":[{"type":"short_answer","prompt":"Which word names a person who teaches students?","options":[],"answers":["teacher"],"explanation":"A teacher teaches students."}]}`,
	"findings":    `{"findings":[{"question":1,"severity":"worth_a_look","issue":"The prompt could be clearer.","fix":{"kind":"none","description":"Reword it.","label":"Reword","option":"","blank":0,"text":""}}]}`,
	"changes":     `{"changes":[{"question":1,"kind":"reword","prompt":"Which of these words is a noun?","order":[],"option":"","text":""}]}`,
	"explanation": `{"explanation":"A noun names a person, a place or a thing."}`,
	"distractors": `{"distractors":[{"text":"slowly","why":"An adverb."},{"text":"happy","why":"An adjective."}]}`,
	"score":       `{"score":0.5,"confidence":"medium","reasons":["Names the idea but gives no example."],"comment":"Add an example."}`,
	"insights":    `{"insights":[{"kind":"problem","title":"A question many missed","body":"Most papers missed the same point.","action":"review","questions":[]}]}`,
	"duplicates":  `{"duplicates":[],"tags":[]}`,
}

func answerTo(req aiports.Request) json.RawMessage {
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(req.Schema, &schema); err != nil || len(schema.Required) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(answers[schema.Required[0]])
}
