package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
)

var (
	listedRefs  = regexp.MustCompile(`(?m)^q(\d+) \[`)
	offeredRefs = regexp.MustCompile(`(?m)^q(\d+) q(\d+)$`)
)

type scripted struct {
	mu       sync.Mutex
	requests []ports.Request
	respond  func(ctx context.Context, req ports.Request) (ports.Response, error)
}

func (c *scripted) Complete(ctx context.Context, req ports.Request) (ports.Response, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	return c.respond(ctx, req)
}

func (c *scripted) seen() []ports.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.requests)
}

func isDuplicatesCall(req ports.Request) bool { return strings.Contains(req.System, "near-duplicate") }

func listed(req ports.Request) []int {
	var refs []int
	for _, m := range listedRefs.FindAllStringSubmatch(req.Blocks[0].Text, -1) {
		n, _ := strconv.Atoi(m[1])
		refs = append(refs, n)
	}
	return refs
}

func firstRef(req ports.Request) int {
	if refs := listed(req); len(refs) > 0 {
		return refs[0]
	}
	return 0
}

func answerFor(req ports.Request) json.RawMessage {
	if isDuplicatesCall(req) {
		var rows []string
		for _, m := range offeredRefs.FindAllStringSubmatch(req.Blocks[0].Text, -1) {
			rows = append(rows, fmt.Sprintf(`{"a":"q%s","b":"q%s","why":"The same question in nearly the same words."}`, m[1], m[2]))
		}
		return json.RawMessage(`{"duplicates":[` + strings.Join(rows, ",") + `]}`)
	}
	var rows []string
	for _, n := range listed(req) {
		rows = append(rows, fmt.Sprintf(`{"question":"q%d","tags":["nouns"],"level":"b1"}`, n))
	}
	return json.RawMessage(`{"tags":[` + strings.Join(rows, ",") + `]}`)
}

func requestID(req ports.Request) string {
	if isDuplicatesCall(req) {
		return "req_dup"
	}
	return fmt.Sprintf("req_q%d", firstRef(req))
}

var perCall = ports.Usage{InputTokens: 900, OutputTokens: 400}

func answered(req ports.Request) (ports.Response, error) {
	return ports.Response{Object: answerFor(req), Usage: perCall, RequestID: requestID(req)}, nil
}

type gate struct {
	need    int32
	arrived atomic.Int32
	open    chan struct{}
}

func newGate(need int) *gate { return &gate{need: int32(need), open: make(chan struct{})} }

func (g *gate) arrive() error {
	if g.arrived.Add(1) == g.need {
		close(g.open)
	}
	select {
	case <-g.open:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("the calls did not overlap")
	}
}

func cleanupOps(t *testing.T, completer ports.Completer, budget time.Duration, bank []domain.BankQuestion) ops {
	t.Helper()
	o := newOps(t, false)
	o.reads.bank = domain.Bank{Questions: bank}
	o.app = application.New(o.repo, application.Options{
		Prices: testPrices, Zone: saigon, Logger: slog.New(slog.NewTextHandler(o.logs, nil)), CleanupBudget: budget,
		Completer: completer, Tests: o.reads, Questions: o.reads, Grading: o.reads, Analysis: o.reads, People: o.reads,
	})
	o.app.SetClock(func() time.Time { return september })
	return o
}

func bothChecks() query.Cleanup {
	return query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupDuplicates, prompts.CleanupTags}}
}

func bankOf(n int) []domain.BankQuestion {
	bank := make([]domain.BankQuestion, n)
	for i := range bank {
		bank[i] = domain.BankQuestion{ID: fmt.Sprintf("untagged-%d", i), Type: "short_answer", Prompt: fmt.Sprintf("apple%d banana%d cherry%d", i, i, i)}
	}
	return bank
}

func tagged(bank []domain.BankQuestion) []domain.BankQuestion {
	for i := range bank {
		bank[i].Tags = []string{"grammar"}
	}
	return bank
}

func totalTokens(calls int) domain.Tokens {
	return domain.Tokens{Input: perCall.InputTokens * int64(calls), Output: perCall.OutputTokens * int64(calls)}
}

func onlyEnded(t *testing.T, o ops, wantSettled int) {
	t.Helper()
	if len(o.repo.reserved) != 1 || len(o.repo.settled)+len(o.repo.released) != 1 || len(o.repo.settled) != wantSettled {
		t.Fatalf("reserved %d, settled %d, released %d; want one reservation ended once, settled %d", len(o.repo.reserved), len(o.repo.settled), len(o.repo.released), wantSettled)
	}
}

func TestACleanupOfATwoThousandQuestionBankIsFourOverlappingCallsUnderOneReservation(t *testing.T) {
	g := newGate(prompts.MaxTagChunks)
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		if err := g.arrive(); err != nil {
			return ports.Response{}, err
		}
		return answered(req)
	}}
	o := cleanupOps(t, completer, 0, bankOf(2000))
	start := time.Now()
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), bothChecks())
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("the pass took %v", time.Since(start))
	}
	if got.Scanned != 2000 || !got.Truncated || len(got.Findings.Tags) != prompts.MaxTagsAsked || len(got.Findings.Duplicates) != 0 {
		t.Fatalf("scanned %d, truncated %v, %d tags, %d duplicates; want 2000 read, 200 tagged, the rest left for another run", got.Scanned, got.Truncated, len(got.Findings.Tags), len(got.Findings.Duplicates))
	}
	requests := completer.seen()
	if len(requests) != prompts.MaxTagChunks {
		t.Fatalf("%d provider calls, want %d", len(requests), prompts.MaxTagChunks)
	}
	var refs []int
	for _, req := range requests {
		in := listed(req)
		if len(in) == 0 || len(in) > prompts.TagChunkSize || req.Tier != ports.TierFast || req.MaxTokens != 3000 || !json.Valid(req.Schema) {
			t.Errorf("a call asks about %d questions on the %s tier with %d tokens", len(in), req.Tier, req.MaxTokens)
		}
		refs = append(refs, in...)
	}
	slices.Sort(refs)
	for i, n := range refs {
		if n != i+1 {
			t.Fatalf("the calls name q%d at place %d, want q1..q%d once each", n, i, prompts.MaxTagsAsked)
		}
	}
	if len(refs) != prompts.MaxTagsAsked {
		t.Errorf("%d questions asked about, want %d", len(refs), prompts.MaxTagsAsked)
	}
	for _, id := range []string{"untagged-0", "untagged-1999"} {
		if strings.Contains(textOfAll(requests), id) {
			t.Errorf("a prompt carries the id %s", id)
		}
	}

	onlyEnded(t, o, 1)
	if r := o.repo.reserved[0]; r.Credits != domain.FeatureCleanup.Ceiling() || r.Feature != domain.FeatureCleanup || r.UserID != teacherID {
		t.Errorf("reserved %+v, want the clean-up's ceiling of %d once, before any call", r, domain.FeatureCleanup.Ceiling())
	}
	settled := o.repo.settled[0]
	if settled.Tokens != totalTokens(4) || settled.Tier != domain.TierFast || settled.RequestID != "req_q1,req_q101,req_q151,req_q51" {
		t.Errorf("settled %+v, want the four calls' tokens summed, the fast tier and the four request ids", settled)
	}
	if got.Usage != (domain.Usage{Credits: 2, Left: left}) {
		t.Errorf("usage %+v, want the 2 credits the sum cost and what is left", got.Usage)
	}
}

func textOfAll(requests []ports.Request) string {
	var all strings.Builder
	for _, req := range requests {
		all.WriteString(textOf(req))
	}
	return all.String()
}

func nearlyAlike(n int, year string) string {
	return fmt.Sprintf("Choose the correct form of the verb to finish the sentence about the museum in Hanoi since %s, number %d.", year, n)
}

func TestADuplicatePairThatNoTwoChunksHoldTogetherIsFoundByOneCallOverTheCandidates(t *testing.T) {
	for name, places := range map[string][2]int{
		"in two tag chunks":                  {10, 160},
		"one of them past the chunks tagged": {3, 1950},
	} {
		t.Run(name, func(t *testing.T) {
			bank := bankOf(2000)
			for i := range bank {
				bank[i].ID = fmt.Sprintf("b-%d", i)
			}
			bank[places[0]].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2019?"
			bank[places[1]].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2020?"
			completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
			o := cleanupOps(t, completer, 0, bank)
			got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: bothChecks().Checks, Language: domain.Vietnamese})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Findings.Duplicates) != 1 || got.Findings.Duplicates[0].A.ID != fmt.Sprintf("b-%d", places[0]) || got.Findings.Duplicates[0].B.ID != fmt.Sprintf("b-%d", places[1]) {
				t.Fatalf("duplicates %+v, want the pair at %v", got.Findings.Duplicates, places)
			}
			requests := completer.seen()
			if len(requests) != 1+prompts.MaxTagChunks {
				t.Fatalf("%d provider calls, want the duplicates call and %d tagging calls", len(requests), prompts.MaxTagChunks)
			}
			both := 0
			for _, req := range requests {
				text := req.Blocks[0].Text
				if strings.Contains(text, bank[places[0]].Prompt) && strings.Contains(text, bank[places[1]].Prompt) {
					both++
					if !isDuplicatesCall(req) || len(listed(req)) != 2 || !strings.Contains(text, "in Vietnamese") {
						t.Errorf("the call that reads both is not the duplicates call over the two candidates in Vietnamese:\n%s", text)
					}
				}
			}
			if both != 1 {
				t.Errorf("%d calls read both questions, want exactly the duplicates call", both)
			}
			onlyEnded(t, o, 1)
			if o.repo.settled[0].Tokens != totalTokens(5) {
				t.Errorf("settled %+v, want the five calls summed", o.repo.settled[0].Tokens)
			}
		})
	}
}

func TestOnlyTheChecksAskedForAreMade(t *testing.T) {
	bank := bankOf(10)
	bank[2].Prompt, bank[7].Prompt = nearlyAlike(1, "2019"), nearlyAlike(1, "2020")
	for name, c := range map[string]struct {
		checks     []string
		duplicates bool
		calls      int
	}{
		"duplicates only": {[]string{prompts.CleanupDuplicates}, true, 1},
		"tags only":       {[]string{prompts.CleanupTags}, false, 1},
	} {
		t.Run(name, func(t *testing.T) {
			completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
			o := cleanupOps(t, completer, 0, slices.Clone(bank))
			got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: c.checks})
			if err != nil {
				t.Fatal(err)
			}
			requests := completer.seen()
			if len(requests) != c.calls || isDuplicatesCall(requests[0]) != c.duplicates {
				t.Fatalf("%d calls, duplicates call %v; want %d call, duplicates %v", len(requests), isDuplicatesCall(requests[0]), c.calls, c.duplicates)
			}
			if c.duplicates && (len(got.Findings.Duplicates) != 1 || len(got.Findings.Tags) != 0) || !c.duplicates && (len(got.Findings.Tags) != 10 || len(got.Findings.Duplicates) != 0) {
				t.Errorf("findings %+v", got.Findings)
			}
			if got.Truncated {
				t.Error("a clean-up that did everything it was asked is truncated")
			}
		})
	}
}

func TestAMidPassFailureSettlesWhatWasBilledAndReturnsWhatItHas(t *testing.T) {
	for name, c := range map[string]struct {
		failure error
		usage   ports.Usage
		calls   int
	}{
		"the provider is down for one chunk":      {fmt.Errorf("%w: 529", ports.ErrUnavailable), ports.Usage{}, 2},
		"one chunk is refused, which was billed":  {fmt.Errorf("%w: stop", ports.ErrRefused), ports.Usage{InputTokens: 300, OutputTokens: 20}, 3},
		"one chunk is cut off, which was billed":  {fmt.Errorf("%w: cut", ports.ErrInvalidResponse), ports.Usage{InputTokens: 300, OutputTokens: 3000}, 3},
		"one request is rejected, which is a 4xx": {fmt.Errorf("%w: 400", ports.ErrRejected), ports.Usage{}, 2},
	} {
		t.Run(name, func(t *testing.T) {
			completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
				if firstRef(req) == 51 {
					return ports.Response{Usage: c.usage, RequestID: "req_failed"}, c.failure
				}
				return answered(req)
			}}
			o := cleanupOps(t, completer, 0, bankOf(150))
			got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
			if err != nil {
				t.Fatalf("err = %v, want what the other chunks found and no error", err)
			}
			if !got.Truncated || len(got.Findings.Tags) != 100 {
				t.Errorf("truncated %v with %d tags, want the 100 of the chunks that answered, marked truncated", got.Truncated, len(got.Findings.Tags))
			}
			onlyEnded(t, o, 1)
			want := totalTokens(2)
			want.Input += c.usage.InputTokens
			want.Output += c.usage.OutputTokens
			if o.repo.settled[0].Tokens != want || strings.Contains(o.repo.settled[0].RequestID, "req_failed") != (c.calls == 3) {
				t.Errorf("settled %+v, want the %d calls the provider billed", o.repo.settled[0], c.calls)
			}
			if got.Usage.Credits == 0 {
				t.Error("the answer carries no charge")
			}
		})
	}
}

func TestACleanupWhoseEveryCallFailsIsUnavailableAndPaysOnlyForWhatWasBilled(t *testing.T) {
	for name, c := range map[string]struct {
		failure error
		usage   ports.Usage
		settles bool
	}{
		"the provider is down": {fmt.Errorf("%w: 529", ports.ErrUnavailable), ports.Usage{}, false},
		"every call refused":   {fmt.Errorf("%w: stop", ports.ErrRefused), ports.Usage{InputTokens: 300, OutputTokens: 20}, true},
		"a bad key":            {fmt.Errorf("%w: 401", ports.ErrRejected), ports.Usage{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			completer := &scripted{respond: func(context.Context, ports.Request) (ports.Response, error) {
				return ports.Response{Usage: c.usage, RequestID: "req_x"}, c.failure
			}}
			o := cleanupOps(t, completer, 0, bankOf(100))
			_, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
			if !errors.Is(err, domain.ErrUnavailable) {
				t.Fatalf("err = %v, want ErrUnavailable", err)
			}
			if c.settles {
				onlyEnded(t, o, 1)
				if got := o.repo.settled[0].Tokens; got.Input != 600 || got.Output != 40 {
					t.Errorf("settled %+v, want both refusals' tokens", got)
				}
				return
			}
			onlyEnded(t, o, 0)
		})
	}
}

func TestAnAnswerWithNothingUsableInOneChunkIsSettledAndTheOthersAreKept(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		if firstRef(req) == 51 {
			return ports.Response{Object: json.RawMessage(`{"unexpected":1}`), Usage: perCall, RequestID: "req_odd"}, nil
		}
		return answered(req)
	}}
	o := cleanupOps(t, completer, 0, bankOf(150))
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
	if err != nil || !got.Truncated || len(got.Findings.Tags) != 100 {
		t.Fatalf("%d tags, truncated %v, %v; want the other two chunks' 100, marked truncated", len(got.Findings.Tags), got.Truncated, err)
	}
	onlyEnded(t, o, 1)
	if o.repo.settled[0].Tokens != totalTokens(3) {
		t.Errorf("settled %+v, want the provider paid for all three answers", o.repo.settled[0].Tokens)
	}
}

func TestACleanupThatRunsOutOfTimeReturnsWhatItHasAndPaysForIt(t *testing.T) {
	var cut atomic.Int32
	untilTheBudgetEnds := func(ctx context.Context, _ ports.Request) (ports.Response, error) {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cut.Add(1)
		}
		return ports.Response{}, ctx.Err()
	}
	completer := &scripted{respond: func(ctx context.Context, req ports.Request) (ports.Response, error) {
		if firstRef(req) == 1 {
			return answered(req)
		}
		return untilTheBudgetEnds(ctx, req)
	}}
	o := cleanupOps(t, completer, 150*time.Millisecond, bankOf(200))
	start := time.Now()
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second || cut.Load() != 3 {
		t.Errorf("took %v and %d calls saw the budget end, want the pass to end at its budget with three calls cut", time.Since(start), cut.Load())
	}
	if !got.Truncated || len(got.Findings.Tags) != prompts.TagChunkSize {
		t.Errorf("truncated %v with %d tags, want the one chunk that answered", got.Truncated, len(got.Findings.Tags))
	}
	onlyEnded(t, o, 1)
	if o.repo.settled[0].Tokens != totalTokens(1) {
		t.Errorf("settled %+v, want only the call that answered", o.repo.settled[0].Tokens)
	}

	nothing := &scripted{respond: untilTheBudgetEnds}
	o = cleanupOps(t, nothing, 100*time.Millisecond, bankOf(100))
	if _, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable when nothing finished in time", err)
	}
	onlyEnded(t, o, 0)
}

func TestACancelledCleanupSettlesWhatWasBilledOnceAndReleasesTheRest(t *testing.T) {
	for name, c := range map[string]struct {
		settles int
	}{
		"after one chunk answered": {1},
		"before any answered":      {0},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var started atomic.Int32
			answeredOne := make(chan struct{})
			var once sync.Once
			completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
				started.Add(1)
				if c.settles == 1 && firstRef(req) == 1 {
					defer once.Do(func() { close(answeredOne) })
					return answered(req)
				}
				<-ctx.Done()
				return ports.Response{}, ctx.Err()
			}}
			o := cleanupOps(t, completer, 0, bankOf(150))
			go func() {
				for started.Load() < 3 {
					time.Sleep(time.Millisecond)
				}
				if c.settles == 1 {
					<-answeredOne
				}
				cancel()
			}()
			_, err := o.app.Queries.Cleanup.Handle(ctx, query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want the cancellation", err)
			}
			onlyEnded(t, o, c.settles)
			if c.settles == 1 && o.repo.settled[0].Tokens != totalTokens(1) {
				t.Errorf("settled %+v, want the one call that answered", o.repo.settled[0].Tokens)
			}
		})
	}
}

func TestAPanicInOneCleanupCallStillEndsTheReservationOnceWithWhatWasBilled(t *testing.T) {
	for name, c := range map[string]struct {
		panics  func(ports.Request) bool
		settles int
	}{
		"one of three calls": {func(req ports.Request) bool { return firstRef(req) == 51 }, 1},
		"every one of them":  {func(ports.Request) bool { return true }, 0},
	} {
		t.Run(name, func(t *testing.T) {
			completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
				if c.panics(req) {
					panic("the provider client crashed")
				}
				return answered(req)
			}}
			o := cleanupOps(t, completer, 0, bankOf(150))
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				_, _ = o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
			}()
			if recovered == nil || !strings.Contains(fmt.Sprint(recovered), "the provider client crashed") {
				t.Fatalf("recovered %v, want the provider's panic to reach the caller", recovered)
			}
			onlyEnded(t, o, c.settles)
			if c.settles == 1 {
				calls := len(completer.seen()) - 1
				if got := o.repo.settled[0].Tokens; got != totalTokens(calls) {
					t.Errorf("settled %+v, want the %d calls that returned", got, calls)
				}
			}
		})
	}
}

func TestACleanupThatCannotFitItsReservationDropsTheLastCallsAndSaysSo(t *testing.T) {
	bank := bankOf(200)
	chunks, _ := prompts.TagChunks(bank)
	full, err := prompts.Tagging(prompts.TaggingInput{Bank: bank, Asked: chunks[0]})
	if err != nil {
		t.Fatal(err)
	}
	tokens := int64(len(full.System)+len(full.User)+len(full.Schema)) + 1_000 + int64(full.MaxTokens)
	pricey := func(creditsPerCall int64) domain.Prices {
		perToken := creditsPerCall * 1_000 * 1_000_000 / tokens
		return domain.Prices{Fast: domain.Price{Input: perToken, Output: perToken, CacheRead: perToken}, Capable: testPrices.Capable}
	}
	ceiling := domain.FeatureCleanup.Ceiling()

	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	o := cleanupOps(t, completer, 0, slices.Clone(bank))
	o.app = application.New(o.repo, application.Options{
		Prices: pricey(ceiling * 45 / 100), Zone: saigon, Logger: slog.New(slog.DiscardHandler),
		Completer: completer, Tests: o.reads, Questions: o.reads, Grading: o.reads, Analysis: o.reads, People: o.reads,
	})
	o.app.SetClock(func() time.Time { return september })
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(completer.seen()); n != 2 || !got.Truncated || len(got.Findings.Tags) != 2*prompts.TagChunkSize {
		t.Errorf("%d calls, truncated %v, %d tags; want the two calls that fit the reservation, and the rest left", n, got.Truncated, len(got.Findings.Tags))
	}
	onlyEnded(t, o, 1)
	if o.repo.reserved[0].Credits != ceiling {
		t.Errorf("reserved %d, want the ceiling %d", o.repo.reserved[0].Credits, ceiling)
	}

	completer = &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	o = cleanupOps(t, completer, 0, slices.Clone(bank))
	o.app = application.New(o.repo, application.Options{
		Prices: pricey(ceiling * 2), Zone: saigon, Logger: slog.New(slog.NewTextHandler(o.logs, nil)),
		Completer: completer, Tests: o.reads, Questions: o.reads, Grading: o.reads, Analysis: o.reads, People: o.reads,
	})
	if _, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupTags}}); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable when not even one call fits", err)
	}
	if len(completer.seen()) != 0 || len(o.repo.reserved) != 0 || !strings.Contains(o.logs.String(), "level=ERROR") {
		t.Errorf("called %d times, reserved %d, logs %q; want nothing held or sent, and an error for the operator", len(completer.seen()), len(o.repo.reserved), o.logs.String())
	}
}

func TestACleanupWithNothingToAskCallsNoOneButStillObeysThePauseAndTheBudgets(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	o := cleanupOps(t, completer, 0, tagged(bankOf(40)))
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), bothChecks())
	if err != nil {
		t.Fatal(err)
	}
	if got.Scanned != 40 || got.Truncated || len(got.Findings.Tags) != 0 || len(got.Findings.Duplicates) != 0 || len(completer.seen()) != 0 {
		t.Errorf("%+v after %d calls, want a tidy bank answered without a call", got, len(completer.seen()))
	}
	onlyEnded(t, o, 0)
	if got.Usage != (domain.Usage{Credits: 0, Left: left}) {
		t.Errorf("usage %+v, want nothing charged", got.Usage)
	}

	for _, refusal := range []error{domain.ErrPaused, &domain.Exhausted{Scope: domain.ScopePool}} {
		o = cleanupOps(t, completer, 0, tagged(bankOf(40)))
		o.repo.reserveErr = refusal
		if _, err := o.app.Queries.Cleanup.Handle(context.Background(), bothChecks()); err != refusal {
			t.Errorf("err = %v, want %v as it is: a free scan is still the teacher's AI", err, refusal)
		}
	}

	o = cleanupOps(t, completer, 0, nil)
	if _, err := o.app.Queries.Cleanup.Handle(context.Background(), bothChecks()); !errors.Is(err, domain.ErrNothingToRead) || len(o.repo.reserved) != 0 {
		t.Errorf("an empty bank: %v, reserved %d", err, len(o.repo.reserved))
	}
}

func TestABankLargerThanTheReadCapIsSaidToBeTruncated(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	o := cleanupOps(t, completer, 0, tagged(bankOf(30)))
	o.reads.bank.Truncated = true
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), bothChecks())
	if err != nil || !got.Truncated {
		t.Errorf("truncated %v, %v; want the bank that held more than the read marked truncated", got.Truncated, err)
	}
	if o.reads.lastLimit != query.MaxCleanupQuestions {
		t.Errorf("the clean-up asked for %d questions, want %d", o.reads.lastLimit, query.MaxCleanupQuestions)
	}
}

func TestATallySumsOnlyWhatTheProviderBilled(t *testing.T) {
	var tally command.Tally
	if got := tally.Response(); got.Usage != (ports.Usage{}) || got.RequestID != "" {
		t.Errorf("an empty tally is %+v", got)
	}
	tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 10, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4}, RequestID: "req_b"}, nil)
	tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 1, OutputTokens: 1}, RequestID: "req_a"}, fmt.Errorf("%w: stop", ports.ErrRefused))
	tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 5, OutputTokens: 5}, RequestID: "req_c"}, fmt.Errorf("%w: cut", ports.ErrInvalidResponse))
	tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 99, OutputTokens: 99}, RequestID: "req_x"}, fmt.Errorf("%w: 529", ports.ErrUnavailable))
	tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 99}, RequestID: "req_y"}, ports.ErrRejected)
	tally.Add(ports.Response{}, context.Canceled)
	wantUsage := ports.Usage{InputTokens: 16, OutputTokens: 8, CacheReadTokens: 3, CacheWriteTokens: 4}
	if got := tally.Response(); got.Usage != wantUsage || got.RequestID != "req_a,req_b,req_c" {
		t.Errorf("response %+v, want %+v and the three ids of the billed calls", got, wantUsage)
	}
}

func TestATallyKeepsTheRequestIdsThatFitTheLedgerWhole(t *testing.T) {
	var tally command.Tally
	long := strings.Repeat("x", 90)
	for _, id := range []string{"req_3_" + long, "req_1_" + long, "req_2_" + long} {
		tally.Add(ports.Response{Usage: ports.Usage{InputTokens: 1}, RequestID: id}, nil)
	}
	got := tally.Response()
	want := "req_1_" + long + ",req_2_" + long
	if got.RequestID != want || len(got.RequestID) > 200 || got.Usage.InputTokens != 3 {
		t.Errorf("request id %q (%d), tokens %d; want the two ids that fit whole and all three calls counted", got.RequestID, len(got.RequestID), got.Usage.InputTokens)
	}
}

func multiByteTwins(pairs int) []domain.BankQuestion {
	tag := strings.Repeat("ế", 80)
	bank := make([]domain.BankQuestion, 0, 2*pairs)
	for i := range pairs {
		var twin [2][]string
		for j := range 25 {
			word := fmt.Sprintf("%s%03d%c", strings.Repeat("ế", 8), i, 'a'+j)
			twin[0], twin[1] = append(twin[0], word), append(twin[1], word)
		}
		twin[1][24] = fmt.Sprintf("%s%03dz", strings.Repeat("ế", 8), i)
		for k := range twin {
			bank = append(bank, domain.BankQuestion{
				ID: fmt.Sprintf("twin-%d-%d", i, k), Type: "multiple_choice", Level: "pre_a1", Tags: []string{tag, tag, tag, tag}, Prompt: strings.Join(twin[k], " "),
			})
		}
	}
	return bank
}

func TestADuplicatesCallTooLongOnMultiByteTextIsCutToFitAndTheAnswerIsTruncatedNotRefused(t *testing.T) {
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	bank := multiByteTwins(prompts.MaxCandidatePairs)
	o := cleanupOps(t, completer, 0, bank)
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupDuplicates}})
	if err != nil {
		t.Fatalf("err = %v, want a smaller call and not a refusal of the pass's own choice", err)
	}
	requests := completer.seen()
	if len(requests) != 1 {
		t.Fatalf("%d provider calls, want the one duplicates call", len(requests))
	}
	user := requests[0].Blocks[0].Text
	offered := len(offeredRefs.FindAllString(user, -1))
	if len(user) > prompts.MaxCleanupBytes || offered == 0 || offered >= prompts.MaxCandidatePairs || len(listed(requests[0])) != 2*offered {
		t.Errorf("the call is %d bytes with %d pairs over %d questions, want fewer than %d pairs inside the cap of %d", len(user), offered, len(listed(requests[0])), prompts.MaxCandidatePairs, prompts.MaxCleanupBytes)
	}
	if !got.Truncated || len(got.Findings.Duplicates) != prompts.MaxDuplicatesAsked || got.Scanned != len(bank) {
		t.Errorf("truncated %v, %d duplicates, scanned %d; want the pairs the call kept, at most %d, and truncated", got.Truncated, len(got.Findings.Duplicates), got.Scanned, prompts.MaxDuplicatesAsked)
	}
	onlyEnded(t, o, 1)

	ascii := cleanupOps(t, completer, 0, multiByteTwinsOfLetter("e", prompts.MaxCandidatePairs))
	if got, err := ascii.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupDuplicates}}); err != nil || got.Truncated {
		t.Errorf("truncated %v, %v: the same bank in ASCII fits and is not cut", got.Truncated, err)
	}
}

func multiByteTwinsOfLetter(letter string, pairs int) []domain.BankQuestion {
	bank := multiByteTwins(pairs)
	for i := range bank {
		bank[i].Prompt = strings.ReplaceAll(bank[i].Prompt, "ế", letter)
		for j := range bank[i].Tags {
			bank[i].Tags[j] = strings.ReplaceAll(bank[i].Tags[j], "ế", letter)
		}
	}
	return bank
}

func TestASearchForLookAlikesThatStopsAtItsWorkLimitIsSaidToBeTruncated(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	bank := make([]domain.BankQuestion, 1000)
	for i := range bank {
		words := make([]string, 0, 300)
		for _, w := range r.Perm(5000)[:300] {
			words = append(words, fmt.Sprintf("v%d", w))
		}
		bank[i] = domain.BankQuestion{ID: fmt.Sprintf("long-%d", i), Type: "short_answer", Prompt: strings.Join(words, " "), Tags: []string{"grammar"}}
	}
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	o := cleanupOps(t, completer, 0, bank)
	got, err := o.app.Queries.Cleanup.Handle(context.Background(), query.Cleanup{Actor: teacher, Checks: []string{prompts.CleanupDuplicates}})
	if err != nil || !got.Truncated || len(completer.seen()) != 0 {
		t.Errorf("truncated %v, %d calls, %v; want a pass that found no pair before its limit, said so, and asked nobody", got.Truncated, len(completer.seen()), err)
	}
	onlyEnded(t, o, 0)
}
