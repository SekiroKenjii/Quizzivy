package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
)

var askedNumber = regexp.MustCompile(`\nQ(\d+) \[`)

func longPaper(n int) domain.Paper {
	var paper domain.Paper
	for i := 1; i <= n; i++ {
		paper.Questions = append(paper.Questions, domain.PaperQuestion{
			ID: fmt.Sprintf("0198f5f0-0000-7000-8000-%012d", i), Number: i, Type: "single_choice", Prompt: fmt.Sprintf("She ___ in Hanoi since %d.", 1990+i),
			Options: []domain.Option{
				{ID: fmt.Sprintf("0198f5f0-0000-7000-8001-%012d", i), Text: "has lived", Correct: true},
				{ID: fmt.Sprintf("0198f5f0-0000-7000-8002-%012d", i), Text: "lived"},
				{ID: fmt.Sprintf("0198f5f0-0000-7000-8003-%012d", i), Text: "lives"},
			},
		})
	}
	return paper
}

func askedNumbers(req ports.Request) []int {
	var out []int
	for _, m := range askedNumber.FindAllStringSubmatch(textOf(req), -1) {
		n, _ := strconv.Atoi(m[1])
		out = append(out, n)
	}
	return out
}

func rewordEach(req ports.Request) (ports.Response, error) {
	var changes []map[string]any
	numbers := askedNumbers(req)
	for i := len(numbers) - 1; i >= 0; i-- {
		changes = append(changes, map[string]any{"question": numbers[i], "kind": "reword", "prompt": fmt.Sprintf("Reworded %d", numbers[i]), "order": []string{}, "option": "", "text": ""})
	}
	object, _ := json.Marshal(map[string]any{"changes": changes})
	return ports.Response{Object: object, Usage: perCall, RequestID: fmt.Sprintf("req_%d", numbers[0])}, nil
}

func variantOps(t *testing.T, completer ports.Completer, budget time.Duration, n int) ops {
	t.Helper()
	o := cleanupOps(t, completer, budget, nil)
	o.reads.paper = longPaper(n)
	return o
}

func reword() query.Variant {
	return query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{prompts.VariantReword}, Difficulty: prompts.DifficultySame}
}

func TestAVariantOfALongTestAsksAboutAFewQuestionsAtATimeUnderOneReservation(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return rewordEach(req) }}
	o := variantOps(t, completer, 0, 40)

	got, err := o.app.Queries.Variant.Handle(context.Background(), reword())
	if err != nil {
		t.Fatal(err)
	}
	if len(completer.requests) != 5 {
		t.Fatalf("%d requests for 40 questions, want 5 of %d", len(completer.requests), prompts.VariantChunkSize)
	}
	seen := map[int]bool{}
	for _, req := range completer.requests {
		numbers := askedNumbers(req)
		if len(numbers) == 0 || len(numbers) > prompts.VariantChunkSize {
			t.Errorf("a request asks about questions %v, want 1 to %d of them", numbers, prompts.VariantChunkSize)
		}
		if req.MaxTokens > 4096 {
			t.Errorf("a request may write %d tokens, want an answer small enough to come back well within the pass", req.MaxTokens)
		}
		for _, n := range numbers {
			if seen[n] {
				t.Errorf("question %d is asked about twice", n)
			}
			seen[n] = true
		}
	}
	if len(got.Changes) != 40 {
		t.Fatalf("%d changes, want one for each of the 40 questions", len(got.Changes))
	}
	for i, c := range got.Changes {
		if c.Number != i+1 || c.QuestionID != o.reads.paper.Questions[i].ID {
			t.Fatalf("change %d is for question %d (%s), want the changes in paper order", i, c.Number, c.QuestionID)
		}
	}
	onlyEnded(t, o, 1)
	if o.repo.settled[0].Tokens != totalTokens(5) {
		t.Errorf("settled %+v, want what all five requests were billed", o.repo.settled[0].Tokens)
	}
}

func TestAVariantHasAtMostAFewRequestsWithTheProviderAtOnce(t *testing.T) {
	var now, most atomic.Int32
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		n := now.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		now.Add(-1)
		return rewordEach(req)
	}}
	o := variantOps(t, completer, 0, 200)

	if _, err := o.app.Queries.Variant.Handle(context.Background(), reword()); err != nil {
		t.Fatal(err)
	}
	if most.Load() > query.VariantCallsInFlight || most.Load() < 2 {
		t.Errorf("%d requests at once, want them to overlap and never more than %d", most.Load(), query.VariantCallsInFlight)
	}
}

func TestAVariantKeepsTheChangesOfTheRequestsThatAnswered(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		switch askedNumbers(req)[0] {
		case 9:
			return ports.Response{}, fmt.Errorf("%w: overloaded", ports.ErrUnavailable)
		case 17:
			return ports.Response{Object: json.RawMessage(`{"changes":[]}`), Usage: perCall, RequestID: "req_17"}, nil
		}
		return rewordEach(req)
	}}
	o := variantOps(t, completer, 0, 20)

	got, err := o.app.Queries.Variant.Handle(context.Background(), reword())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Changes) != prompts.VariantChunkSize {
		t.Fatalf("%d changes, want the %d of the one request that answered usably", len(got.Changes), prompts.VariantChunkSize)
	}
	for _, c := range got.Changes {
		if c.Number > prompts.VariantChunkSize {
			t.Errorf("a change to question %d, which no usable answer named", c.Number)
		}
	}
	onlyEnded(t, o, 1)
	if o.repo.settled[0].Tokens != totalTokens(2) {
		t.Errorf("settled %+v, want the two billed answers and not the failed request", o.repo.settled[0].Tokens)
	}
}

func TestAVariantWithNoUsableAnswerIsUnavailable(t *testing.T) {
	completer := &scripted{respond: func(context.Context, ports.Request) (ports.Response, error) {
		return ports.Response{Object: json.RawMessage(`{"changes":[]}`), Usage: perCall, RequestID: "req_none"}, nil
	}}
	o := variantOps(t, completer, 0, 20)

	_, err := o.app.Queries.Variant.Handle(context.Background(), reword())
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	onlyEnded(t, o, 1)
}

func TestAVariantThatRunsOutOfTimeReturnsWhatItHas(t *testing.T) {
	completer := &scripted{respond: func(ctx context.Context, req ports.Request) (ports.Response, error) {
		if askedNumbers(req)[0] == 1 {
			return rewordEach(req)
		}
		<-ctx.Done()
		return ports.Response{}, fmt.Errorf("%w: %w", ports.ErrUnavailable, ctx.Err())
	}}
	o := variantOps(t, completer, 150*time.Millisecond, 24)

	start := time.Now()
	got, err := o.app.Queries.Variant.Handle(context.Background(), reword())
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %v, want the pass to end at its budget", time.Since(start))
	}
	if len(got.Changes) != prompts.VariantChunkSize {
		t.Errorf("%d changes, want the first request's %d", len(got.Changes), prompts.VariantChunkSize)
	}
	onlyEnded(t, o, 1)
}
