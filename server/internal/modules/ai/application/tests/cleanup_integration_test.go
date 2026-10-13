//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/modules/ai/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type cleaning struct {
	*world
	month   time.Time
	teacher string
	app     *application.Application
}

func (w *world) cleaning(completer ports.Completer, bank []domain.BankQuestion, pool int64) cleaning {
	w.t.Helper()
	month := w.newMonth()
	w.period(month, pool)
	return w.cleaningIn(month, completer, bank)
}

func (w *world) cleaningIn(month time.Time, completer ports.Completer, bank []domain.BankQuestion) cleaning {
	w.t.Helper()
	teacher := w.teacher()
	r := &reads{bank: domain.Bank{Questions: bank}, prefs: domain.Preferences{Language: domain.English, ReadStudentAnswers: true}}
	app := application.New(repositories.NewPostgres(db.NewContext(w.pool)), application.Options{
		Prices: testPrices, Zone: saigon, Logger: slog.New(slog.DiscardHandler), Completer: completer, Questions: r, People: r,
	})
	app.SetClock(func() time.Time { return *w.clock.Load() })
	return cleaning{world: w, month: month, teacher: teacher, app: app}
}

func (c cleaning) run(ctx context.Context) (query.Cleaned, error) {
	return c.app.Queries.Cleanup.Handle(ctx, query.Cleanup{
		Actor:  actor.Actor{ID: c.teacher, Scope: access.Scope{UserID: c.teacher}},
		Checks: []string{prompts.CleanupDuplicates, prompts.CleanupTags},
	})
}

type ledgerRow struct {
	feature, tier, request string
	input, output, credits int64
}

func (c cleaning) ledger() []ledgerRow {
	c.t.Helper()
	rows, err := c.pool.Query(context.Background(), `
		SELECT feature, tier, input_tokens, output_tokens, credits, coalesce(request_id, '')
		  FROM app.ai_usage_ledger WHERE month = $1::date AND user_id = $2::uuid ORDER BY created_at`, c.month, c.teacher)
	if err != nil {
		c.t.Fatal(err)
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.feature, &r.tier, &r.input, &r.output, &r.credits, &r.request); err != nil {
			c.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		c.t.Fatal(err)
	}
	return out
}

func (c cleaning) nothingHeld() {
	c.t.Helper()
	if p := c.periodRow(c.month); p.reserved != 0 || c.reservations(c.month) != 0 {
		c.t.Errorf("pool %+v, %d reservations; want nothing still held", p, c.reservations(c.month))
	}
	if c.hasTeacherRow(c.month, c.teacher) {
		if tr := c.teacherRow(c.month, c.teacher); tr.reserved != 0 {
			c.t.Errorf("teacher %+v; want nothing still held", tr)
		}
	}
}

func (c cleaning) heldNow() int64 { return c.teacherRow(c.month, c.teacher).reserved }

func dissimilarWithAPair(n int) []domain.BankQuestion {
	bank := bankOf(n)
	bank[4].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2019?"
	bank[110].Prompt = "Which tense does the sentence use: she has lived in Hanoi since 2020?"
	return bank
}

func TestAChunkedCleanupHoldsItsCeilingOnceAndSettlesTheSumInOneLedgerRow(t *testing.T) {
	w := newWorld(t)
	var mu sync.Mutex
	var heldAtCall []int64
	var c cleaning
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		held := c.heldNow()
		mu.Lock()
		heldAtCall = append(heldAtCall, held)
		mu.Unlock()
		return answered(req)
	}}
	c = w.cleaning(completer, dissimilarWithAPair(120), 100_000)
	got, err := c.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings.Duplicates) != 1 || len(got.Findings.Tags) != 120 || got.Truncated {
		t.Fatalf("%d duplicates, %d tags, truncated %v; want the pair, all 120 tagged and nothing left", len(got.Findings.Duplicates), len(got.Findings.Tags), got.Truncated)
	}
	if len(heldAtCall) != 4 {
		t.Fatalf("%d provider calls, want the duplicates call and three tagging calls", len(heldAtCall))
	}
	for _, held := range heldAtCall {
		if held != domain.FeatureCleanup.Ceiling() {
			t.Errorf("%d credits were held while a call was in flight, want the ceiling of %d held once for all of them", held, domain.FeatureCleanup.Ceiling())
		}
	}
	rows := c.ledger()
	if len(rows) != 1 {
		t.Fatalf("%d ledger rows, want one for the whole pass", len(rows))
	}
	row := rows[0]
	if row.feature != "cleanup" || row.tier != "fast" || row.input != 3600 || row.output != 1600 || row.request != "req_dup,req_q1,req_q101,req_q51" {
		t.Errorf("ledger %+v, want the four calls summed on the fast tier, with their request ids", row)
	}
	if tr, p := c.teacherRow(c.month, c.teacher), c.periodRow(c.month); tr.used != row.credits || p.used != row.credits || row.credits != got.Usage.Credits || row.credits < 1 {
		t.Errorf("teacher %+v, pool %+v, ledger %d, answer %d; want all four to agree", tr, p, row.credits, got.Usage.Credits)
	}
	c.nothingHeld()
}

func TestAMidPassFailureLeavesTheLedgerWithWhatWasBilledAndNothingHeld(t *testing.T) {
	w := newWorld(t)
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		if firstRef(req) == 51 {
			return ports.Response{}, fmt.Errorf("%w: 529", ports.ErrUnavailable)
		}
		return answered(req)
	}}
	c := w.cleaning(completer, bankOf(150), 100_000)
	got, err := c.run(context.Background())
	if err != nil || !got.Truncated || len(got.Findings.Tags) != 100 {
		t.Fatalf("%d tags, truncated %v, %v; want the two chunks that answered, marked truncated", len(got.Findings.Tags), got.Truncated, err)
	}
	rows := c.ledger()
	if len(rows) != 1 || rows[0].input != 1800 || rows[0].output != 800 || strings.Contains(rows[0].request, "q51") {
		t.Errorf("ledger %+v, want one row for the two calls that were billed", rows)
	}
	if tr := c.teacherRow(c.month, c.teacher); tr.used != rows[0].credits {
		t.Errorf("teacher %+v, want %d used", tr, rows[0].credits)
	}
	c.nothingHeld()
}

func TestACancelledCleanupEndsItsReservationEvenThoughTheCallerIsGone(t *testing.T) {
	for name, answersFirst := range map[string]bool{"before any call answered": false, "after one answered": true} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var started sync.WaitGroup
			started.Add(3)
			answeredOne := make(chan struct{})
			var once sync.Once
			completer := &scripted{respond: func(callCtx context.Context, req ports.Request) (ports.Response, error) {
				started.Done()
				if answersFirst && firstRef(req) == 1 {
					defer once.Do(func() { close(answeredOne) })
					return answered(req)
				}
				<-callCtx.Done()
				return ports.Response{}, callCtx.Err()
			}}
			c := w.cleaning(completer, bankOf(150), 100_000)
			go func() {
				started.Wait()
				if answersFirst {
					<-answeredOne
				}
				cancel()
			}()
			if _, err := c.run(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want the cancellation", err)
			}
			rows := c.ledger()
			if answersFirst != (len(rows) == 1) {
				t.Errorf("ledger %+v, want a row exactly when a call answered", rows)
			}
			if tr := c.teacherRow(c.month, c.teacher); !answersFirst && tr.used != 0 {
				t.Errorf("teacher %+v, want nothing charged", tr)
			}
			c.nothingHeld()
		})
	}
}

func TestAPanicInACleanupCallReturnsTheHeldCreditsAndChargesTheCallsThatReturned(t *testing.T) {
	w := newWorld(t)
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		if firstRef(req) == 101 {
			panic("the provider client crashed")
		}
		return answered(req)
	}}
	c := w.cleaning(completer, bankOf(150), 100_000)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic was swallowed")
			}
		}()
		_, _ = c.run(context.Background())
	}()
	rows := c.ledger()
	if len(rows) != 1 || rows[0].input != 1800 || rows[0].output != 800 {
		t.Errorf("ledger %+v, want one row for the two calls that returned", rows)
	}
	c.nothingHeld()
}

func TestACleanupThePoolCannotCoverIsRefusedBeforeAnyCall(t *testing.T) {
	w := newWorld(t)
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) { return answered(req) }}
	c := w.cleaning(completer, bankOf(150), domain.FeatureCleanup.Ceiling()-1)
	_, err := c.run(context.Background())
	if !isScope(err, domain.ScopePool) {
		t.Fatalf("err = %v, want the pool's refusal", err)
	}
	if n := len(completer.seen()); n != 0 || len(c.ledger()) != 0 {
		t.Errorf("%d provider calls and %d ledger rows after a refusal", n, len(c.ledger()))
	}
	c.nothingHeld()
}

func TestTwoCleanupsRacingForOnePoolWithRoomForOneLeaveExactlyOne(t *testing.T) {
	w := newWorld(t)
	returned := make(chan struct{})
	var once sync.Once
	completer := &scripted{respond: func(_ context.Context, req ports.Request) (ports.Response, error) {
		select {
		case <-returned:
		case <-time.After(5 * time.Second):
		}
		return answered(req)
	}}
	first := w.cleaning(completer, bankOf(40), domain.FeatureCleanup.Ceiling()+10)
	second := w.cleaningIn(first.month, completer, bankOf(40))
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, c := range []cleaning{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer once.Do(func() { close(returned) })
			_, errs[i] = c.run(context.Background())
		}()
	}
	wg.Wait()
	refused := 0
	for _, err := range errs {
		if isScope(err, domain.ScopePool) {
			refused++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if refused != 1 {
		t.Errorf("errors %v, want exactly one of the two refused by the pool", errs)
	}
	if rows := len(first.ledger()) + len(second.ledger()); rows != 1 {
		t.Errorf("%d ledger rows, want the one cleanup that ran", rows)
	}
	first.nothingHeld()
	second.nothingHeld()
}
