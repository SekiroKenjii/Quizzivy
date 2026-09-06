//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/platform/db"
	"testing"

	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"

	"github.com/jackc/pgx/v5/pgxpool"

	questionsapp "quizzivy/internal/modules/questions/application"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
)

func pubMakeAuthor(t *testing.T, pool *pgxpool.Pool) string {
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
	tests  *application.Application
	qsvc   *questionsapp.Application
}

func newBuilder(t *testing.T, pool *pgxpool.Pool, author string) *builder {
	return &builder{
		t: t, pool: pool, author: author,
		tests: application.New(repositories.NewPostgres(db.NewContext(pool), questionsrepo.NewPostgres(db.NewContext(pool)), mediarepo.NewPostgres(db.NewContext(pool)))),
		qsvc:  questionsapp.New(questionsrepo.NewPostgres(db.NewContext(pool)), mediaKinds{pool}),
	}
}

func (b *builder) question(in questionsdomain.Input) string {
	b.t.Helper()
	if in.Tags == nil {
		in.Tags = []string{}
	}
	q, err := b.qsvc.Commands.Create.Handle(context.Background(), questionscommand.Create{Request: questionsdomain.WriteRequest{Input: in, ActorID: b.author}})
	if err != nil {
		b.t.Fatalf("question %q: %v", in.Prompt, err)
	}
	return q.ID
}

func (b *builder) shortAnswer(prompt, points string) string {
	return b.question(questionsdomain.Input{
		Type: questionsdomain.ShortAnswer, Prompt: prompt, Points: points,
	})
}

// draft creates a test whose single section holds the given questions.
func (b *builder) draft(title string, questionIDs ...string) domain.Test {
	b.t.Helper()
	ctx := context.Background()
	created, err := b.tests.Commands.Create.Handle(ctx, command.Create{Request: domain.Request{ActorID: b.author}, Title: title, Description: nil})
	if err != nil {
		b.t.Fatal(err)
	}
	saved, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: domain.Request{ID: created.ID, ActorID: b.author}, Input: domain.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections:          []domain.SectionInput{{Title: "Phần 1", QuestionIDs: questionIDs}},
	}})
	if err != nil {
		b.t.Fatal(err)
	}
	return saved
}

func (b *builder) publish(testID string) (domain.PublishedVersion, error) {
	return application.New(repositories.NewPostgres(db.NewContext(b.pool), questionsrepo.NewPostgres(db.NewContext(b.pool)), mediarepo.NewPostgres(db.NewContext(b.pool)))).Commands.Publish.Handle(context.Background(), command.Publish{Request: domain.PublishRequest{TestID: testID, ActorID: b.author}})
}
