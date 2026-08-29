package publish_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
	"quizzivy/internal/tests"
	"quizzivy/internal/tests/publish"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func makeAuthor(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role)
		 VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		"publish-"+hex.EncodeToString(nonce)+"@example.com").Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE published_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE created_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.questions WHERE created_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, id)
	})
	return id
}

// builder assembles a draft test with one section, so each test can vary only
// what it is about.
type builder struct {
	t      *testing.T
	pool   *pgxpool.Pool
	author string
	tests  *tests.Service
	qsvc   *questions.Service
}

func newBuilder(t *testing.T, pool *pgxpool.Pool, author string) *builder {
	return &builder{
		t: t, pool: pool, author: author,
		tests: tests.NewService(tests.NewStore(pool)),
		qsvc:  questions.NewService(questions.NewStore(pool)),
	}
}

func (b *builder) question(in questions.Input) string {
	b.t.Helper()
	if in.Tags == nil {
		in.Tags = []string{}
	}
	q, err := b.qsvc.Create(context.Background(), questions.WriteRequest{Input: in, ActorID: b.author})
	if err != nil {
		b.t.Fatalf("question %q: %v", in.Prompt, err)
	}
	return q.ID
}

func (b *builder) shortAnswer(prompt, points string) string {
	return b.question(questions.Input{
		Type: questions.ShortAnswer, Prompt: prompt, Points: points,
	})
}

// draft creates a test whose single section holds the given questions.
func (b *builder) draft(title string, questionIDs ...string) tests.Test {
	b.t.Helper()
	ctx := context.Background()
	created, err := b.tests.Create(ctx, tests.Request{ActorID: b.author}, title, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	saved, err := b.tests.Update(ctx, tests.Request{ID: created.ID, ActorID: b.author},
		tests.UpdateInput{
			ExpectedUpdatedAt: created.UpdatedAt,
			SetSections:       true,
			Sections:          []tests.SectionInput{{Title: "Phần 1", QuestionIDs: questionIDs}},
		})
	if err != nil {
		b.t.Fatal(err)
	}
	return saved
}

func (b *builder) publish(testID string) (publish.Version, error) {
	return publish.NewPublisher(b.pool).Publish(context.Background(),
		publish.Request{TestID: testID, ActorID: b.author})
}
