package questions_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
)

// countingTracer counts every query the pool sends, using pgx's own tracing
// hook so the count is what actually reached the wire.
type countingTracer struct {
	mu sync.Mutex
	n  int
}

func (t *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n++
	return ctx
}

func (t *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *countingTracer) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.n
}

func tracedPool(t *testing.T) (*pgxpool.Pool, *countingTracer) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	tracer := &countingTracer{}
	cfg.ConnConfig.Tracer = tracer
	// One connection, so a lazily-opened second one cannot add its setup
	// queries to the count mid-measurement.
	cfg.MinConns, cfg.MaxConns = 1, 1

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, tracer
}

// TestListCostsAFixedNumberOfQueries pins the fix for the 1+2N fetch.
//
// Listing used to run one query for the page and two more per row, so a full
// page of 100 cost 201 round trips -- paid in network latency against Neon, on
// the screen a teacher browses most. Counted rather than reasoned about,
// because the next loader added to that path would reintroduce it silently.
func TestListCostsAFixedNumberOfQueries(t *testing.T) {
	pool, tracer := tracedPool(t)
	author := makeAuthor(t, pool)
	svc := questions.NewService(questions.NewStore(pool))
	ctx := context.Background()

	const rows = 12
	for i := range rows {
		in := questions.Input{
			Type:   questions.SingleChoice,
			Prompt: "Câu hỏi đếm số " + string(rune('A'+i)),
			Points: "1.00", Tags: []string{},
			Options: []questions.OptionInput{
				{Text: "A", IsCorrect: true},
				{Text: "B", IsCorrect: false},
				{Text: "C", IsCorrect: false},
			},
		}
		if _, err := svc.Create(ctx, questions.WriteRequest{Input: in, ActorID: author}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	measure := func(limit int) int {
		before := tracer.count()
		if _, _, err := svc.List(ctx, questions.ListInput{Limit: limit}); err != nil {
			t.Fatal(err)
		}
		return tracer.count() - before
	}

	small := measure(2)
	large := measure(rows)

	if large != small {
		t.Errorf("listing 2 rows cost %d queries and %d rows cost %d; the cost must not grow "+
			"with page size, or a full page of %d is %d round trips",
			small, rows, large, questions.MaxLimit, small+2*(questions.MaxLimit-1))
	}
	if large > 3 {
		t.Errorf("a page cost %d queries; expected the page plus one per child table", large)
	}
	t.Logf("a page costs %d queries at any size", large)
}
