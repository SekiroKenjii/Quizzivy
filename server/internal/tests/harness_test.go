package tests_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
	"quizzivy/internal/tests"
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
	email := "tests-author-" + hex.EncodeToString(nonce) + "@example.com"

	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE created_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.questions WHERE created_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, id)
	})
	return id
}

func newService(t *testing.T, pool *pgxpool.Pool) *tests.Service {
	t.Helper()
	return tests.NewService(tests.NewStore(pool))
}

func req(author string) tests.Request { return tests.Request{ActorID: author} }

func reqFor(id, author string) tests.Request {
	return tests.Request{ID: id, ActorID: author}
}

// newQuestion adds a bank question the outline can reference.
func newQuestion(t *testing.T, pool *pgxpool.Pool, author, prompt string) string {
	t.Helper()
	svc := questions.NewService(questions.NewStore(pool))
	q, err := svc.Create(context.Background(), questions.WriteRequest{
		Input: questions.Input{
			Type: questions.ShortAnswer, Prompt: prompt, Points: "2.00", Tags: []string{},
		},
		ActorID: author,
	})
	if err != nil {
		t.Fatalf("question %q: %v", prompt, err)
	}
	return q.ID
}
