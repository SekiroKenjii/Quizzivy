//go:build aismoke

package adapters_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/platform/config"
)

const livePassage = "Many people who work in London prefer to live outside of it, and to go to their offices, factories or schools every day by train, car or bus, even though this means they have to get up earlier in the morning and come home later in the evening.\n\n" +
	"One advantage of living outside of London is that houses are cheaper. Even a small flat in London without a garden costs quite a lot to rent. With the same amount of money, one can get a little house in the country with a garden of one's own.\n\n" +
	"Then, in the country, one can get to know one's neighbours, while in London, people often do not even know the names of the people living next door."

func liveCompleter(t *testing.T) ports.Completer {
	t.Helper()
	key := os.Getenv("QUIZZIVY_AI_TEST_KEY")
	if key == "" {
		t.Fatal("QUIZZIVY_AI_TEST_KEY is not set; this smoke calls the real provider and needs a key")
	}
	model := func(name, fallback string) string {
		if v := os.Getenv(name); v != "" {
			return v
		}
		return fallback
	}
	completer, err := adapters.NewAI(config.AI{
		APIKey:  config.Secret(key),
		Fast:    config.AITier{Model: model("QUIZZIVY_AI_TEST_FAST_MODEL", "claude-haiku-5-5")},
		Capable: config.AITier{Model: model("QUIZZIVY_AI_TEST_CAPABLE_MODEL", "claude-sonnet-5-5")},
	}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return completer
}

func ask(t *testing.T, c ports.Completer, p prompts.Prompt) ports.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	response, err := c.Complete(ctx, ports.Request{Tier: ports.Tier(p.Tier), System: p.System, Blocks: []ports.Block{ports.TextBlock(p.User)}, Schema: p.Schema, MaxTokens: p.MaxTokens})
	if err != nil {
		t.Fatalf("%s: %v", p.Feature, err)
	}
	t.Logf("%s: %d input and %d output tokens, request %s", p.Feature, response.Usage.InputTokens, response.Usage.OutputTokens, response.RequestID)
	return response
}

func TestLiveEachAIFeatureGetsAUsableAnswerFromTheProvider(t *testing.T) {
	c := liveCompleter(t)
	t.Run("generate", func(t *testing.T) {
		in := prompts.GenerateInput{Source: livePassage, Types: []string{prompts.SingleChoice, prompts.TrueFalse, prompts.ShortAnswer}, Count: 5, Level: "b2", Explanations: true, Language: domain.English}
		got, err := prompts.DecodeGenerate(ask(t, c, prompts.Generate(in)).Object, in)
		if err != nil || len(got) != in.Count {
			t.Fatalf("kept %d of %d questions: %v", len(got), in.Count, err)
		}
	})
	t.Run("generate standalone for a section", func(t *testing.T) {
		section := domain.Section{Title: "Part 2: Past simple and present perfect", Instructions: "Choose the correct form of the verb.", Questions: []domain.SectionQuestion{
			{Type: prompts.SingleChoice, Prompt: "I ___ to Da Nang last summer."},
			{Type: prompts.FillBlank, Prompt: "She has ___ (live) in Hanoi since 2019."},
		}}
		in := prompts.GenerateInput{Source: "irregular verbs", Types: []string{prompts.SingleChoice, prompts.FillBlank}, Count: 8, Level: "b1", Explanations: true, Language: domain.Vietnamese, Standalone: true, Section: &section}
		got, err := prompts.DecodeGenerate(ask(t, c, prompts.Generate(in)).Object, in)
		if err != nil || len(got) < in.Count-1 {
			t.Fatalf("kept %d of %d questions: %v", len(got), in.Count, err)
		}
	})
	question := domain.Question{Type: prompts.SingleChoice, Prompt: "Why do many London workers live outside the city?", Options: []domain.Option{{ID: "a", Text: "Houses are cheaper", Correct: true}, {ID: "b", Text: "The trains are free"}, {ID: "c", Text: "Offices are outside London"}}}
	t.Run("explanation", func(t *testing.T) {
		p, err := prompts.Explanation(prompts.ExplanationInput{Question: question, Language: domain.Vietnamese})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := prompts.DecodeExplanation(ask(t, c, p).Object); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("distractors", func(t *testing.T) {
		in := prompts.DistractorsInput{Question: question, Language: domain.English}
		p, err := prompts.Distractors(in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := prompts.DecodeDistractors(ask(t, c, p).Object, in); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("score", func(t *testing.T) {
		in := prompts.ScoreInput{Answer: domain.GradedAnswer{Type: prompts.ShortAnswer, Prompt: "What is cheaper outside London?", Points: 1, Accepted: []string{"houses"}, Answer: "the houses"}, Language: domain.English}
		if _, err := prompts.DecodeScore(ask(t, c, prompts.Score(in)).Object, in); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tagging on the fast tier", func(t *testing.T) {
		in := prompts.TaggingInput{Bank: []domain.BankQuestion{{ID: "b1", Type: prompts.ShortAnswer, Prompt: "What is cheaper outside London?"}, {ID: "b2", Type: prompts.SingleChoice, Prompt: "Choose the form: She ___ in Hanoi since 2019."}}, Asked: []int{0, 1}}
		p, err := prompts.Tagging(in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := prompts.DecodeTagging(ask(t, c, p).Object, in); err != nil {
			t.Fatal(err)
		}
	})
}
