package cqrs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/shared/cqrs"
)

type said struct{ text string }

type key struct{}

func quiet() (*slog.Logger, *bytes.Buffer) {
	var out bytes.Buffer
	return slog.New(slog.NewTextHandler(&out, nil)), &out
}

func listing(sent *[]string, failOn ...string) cqrs.CommandHandler[said, cqrs.Nothing] {
	return cqrs.HandlerFunc[said, cqrs.Nothing](func(_ context.Context, c said) (cqrs.Nothing, error) {
		for _, bad := range failOn {
			if c.text == bad {
				return cqrs.Nothing{}, errors.New("refused " + c.text)
			}
		}
		*sent = append(*sent, c.text)
		return cqrs.Nothing{}, nil
	})
}

func commands(texts ...string) func(context.Context) ([]said, error) {
	return func(context.Context) ([]said, error) {
		out := make([]said, len(texts))
		for i, text := range texts {
			out[i] = said{text}
		}
		return out, nil
	}
}

func TestAnnounceDeliversEveryCommandInOrder(t *testing.T) {
	var sent []string
	logger, log := quiet()
	cqrs.Announce(context.Background(), listing(&sent), logger, "test", commands("a", "b", "c"))
	if strings.Join(sent, "") != "abc" {
		t.Errorf("delivered %v, want a b c in order", sent)
	}
	if log.Len() != 0 {
		t.Errorf("a clean delivery logged %q", log.String())
	}
}

func TestAnnounceWithoutAHandlerReadsNothing(t *testing.T) {
	called := false
	cqrs.Announce[said](context.Background(), nil, nil, "test", func(context.Context) ([]said, error) {
		called = true
		return nil, nil
	})
	if called {
		t.Error("build ran although nobody could be told")
	}
}

func TestAnnounceOutlivesTheCancellationOfItsCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "kept"))
	cancel()
	var sawCancelled bool
	var sawValue any
	var deadline time.Time
	handler := cqrs.HandlerFunc[said, cqrs.Nothing](func(ctx context.Context, _ said) (cqrs.Nothing, error) {
		sawCancelled = ctx.Err() != nil
		sawValue = ctx.Value(key{})
		deadline, _ = ctx.Deadline()
		return cqrs.Nothing{}, nil
	})
	before := time.Now()
	cqrs.Announce(ctx, handler, nil, "test", commands("a"))
	if sawCancelled {
		t.Error("a request that ended after its commit cancelled the announcement")
	}
	if sawValue != "kept" {
		t.Errorf("the announcement lost the request's values: %v", sawValue)
	}
	if got := deadline.Sub(before); got <= 0 || got > cqrs.AnnounceBudget+time.Second {
		t.Errorf("the announcement's budget is %v, want at most %v", got, cqrs.AnnounceBudget)
	}
}

func TestAnnounceLogsOneFailureAndDeliversTheRest(t *testing.T) {
	var sent []string
	logger, log := quiet()
	cqrs.Announce(context.Background(), listing(&sent, "b", "d"), logger, "attempt", commands("a", "b", "c", "d", "e"))
	if strings.Join(sent, "") != "ace" {
		t.Errorf("delivered %v, want a c e", sent)
	}
	logged := log.String()
	if strings.Count(logged, "announcement not delivered") != 1 {
		t.Errorf("the failures were logged %d times, want once: %q", strings.Count(logged, "announcement not delivered"), logged)
	}
	for _, want := range []string{"what=attempt", "commands=5", "undelivered=2", "refused b"} {
		if !strings.Contains(logged, want) {
			t.Errorf("the log %q does not say %q", logged, want)
		}
	}
}

func TestAnnounceLogsAFailedPreparationAndDeliversNothing(t *testing.T) {
	var sent []string
	logger, log := quiet()
	cqrs.Announce(context.Background(), listing(&sent), logger, "class joined", func(context.Context) ([]said, error) {
		return []said{{"a"}}, errors.New("the briefing could not be read")
	})
	if len(sent) != 0 {
		t.Errorf("delivered %v although the preparation failed", sent)
	}
	if !strings.Contains(log.String(), "announcement not prepared") || !strings.Contains(log.String(), "the briefing could not be read") {
		t.Errorf("the log %q does not say what failed", log.String())
	}
}

func TestAnnounceStopsOnceTheBudgetIsSpentAndCountsWhatWasLeft(t *testing.T) {
	logger, log := quiet()
	var reached []string
	waits := cqrs.HandlerFunc[said, cqrs.Nothing](func(ctx context.Context, c said) (cqrs.Nothing, error) {
		reached = append(reached, c.text)
		if c.text == "a" {
			return cqrs.Nothing{}, nil
		}
		<-ctx.Done()
		return cqrs.Nothing{}, ctx.Err()
	})
	cqrs.AnnounceWithin(context.Background(), 30*time.Millisecond, waits, logger, "test", commands("a", "b", "c", "d"))
	if strings.Join(reached, "") != "ab" {
		t.Errorf("reached %v, want a and then b, which spent the budget", reached)
	}
	for _, want := range []string{"commands=4", "undelivered=3", "context deadline exceeded"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("the log %q does not say %q", log.String(), want)
		}
	}
}
