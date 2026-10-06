//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"

	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"

	"github.com/jackc/pgx/v5"
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
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`,
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
	pool   db.Conn
	author string
	tests  *application.Application
	qsvc   *questionsapp.Application
}

func newBuilder(t *testing.T, pool db.Conn, author string) *builder {
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
	created, err := b.tests.Commands.Create.Handle(ctx, command.Create{Request: domain.Request{ActorID: b.author, Scope: access.Scope{UserID: b.author}}, Title: title, Description: nil})
	if err != nil {
		b.t.Fatal(err)
	}
	saved, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: domain.Request{ID: created.ID, ActorID: b.author, Scope: access.Scope{UserID: b.author}}, Input: domain.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections:          []domain.SectionInput{{Title: "Phần 1", QuestionIDs: questionIDs}},
	}})
	if err != nil {
		b.t.Fatal(err)
	}
	return saved
}

func (b *builder) publish(testID string) (domain.Version, error) {
	return application.New(repositories.NewPostgres(db.NewContext(b.pool), questionsrepo.NewPostgres(db.NewContext(b.pool)), mediarepo.NewPostgres(db.NewContext(b.pool)))).Commands.Publish.Handle(context.Background(), command.Publish{Request: domain.PublishRequest{TestID: testID, ActorID: b.author, Scope: access.Scope{UserID: b.author}}})
}

type snapshotFixture struct {
	conn     pgx.Tx
	observer db.Querier
	author   string
	builder  *builder
}

type publishOwnedIDs struct {
	Author    string   `json:"author"`
	Questions []string `json:"questions"`
	Tests     []string `json:"tests"`
	Versions  []string `json:"versions"`
	Audit     []string `json:"audit"`
}

func newSnapshotFixture(t *testing.T) *snapshotFixture {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("snapshot fixture begin: %v", err)
	}
	f := &snapshotFixture{conn: tx, observer: pool}
	t.Cleanup(func() {
		ctx := context.Background()
		owned, captureErr := capturePublishOwned(ctx, tx, f.author)
		rollbackErr := tx.Rollback(ctx)
		if captureErr != nil {
			t.Errorf("snapshot fixture capture: %v", captureErr)
		}
		if rollbackErr != nil {
			t.Errorf("snapshot fixture rollback: %v", rollbackErr)
			return
		}
		if captureErr == nil && assertPublishOwnedAbsent(t, f.observer, owned) {
			t.Logf("snapshot fixture rollback verified: author=%s questions=%d tests=%d versions=%d audit=%d", owned.Author, len(owned.Questions), len(owned.Tests), len(owned.Versions), len(owned.Audit))
		}
	})
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		"snapshot-"+hex.EncodeToString(nonce)+"@example.com").Scan(&f.author); err != nil {
		t.Fatalf("snapshot fixture author: %v", err)
	}
	f.builder = newBuilder(t, tx, f.author)
	return f
}

func capturePublishOwned(ctx context.Context, conn db.Querier, author string) (publishOwnedIDs, error) {
	owned := publishOwnedIDs{Author: author}
	if author == "" {
		return owned, nil
	}
	for _, entry := range []struct {
		query string
		ids   *[]string
	}{
		{`SELECT id::text FROM app.questions WHERE created_by = $1 ORDER BY id`, &owned.Questions},
		{`SELECT id::text FROM app.tests WHERE created_by = $1 ORDER BY id`, &owned.Tests},
		{`SELECT id::text FROM app.test_versions WHERE published_by = $1 ORDER BY id`, &owned.Versions},
		{`SELECT id::text FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id`, &owned.Audit},
	} {
		rows, err := conn.Query(ctx, entry.query, author)
		if err != nil {
			return owned, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return owned, err
			}
			*entry.ids = append(*entry.ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return owned, err
		}
	}
	return owned, nil
}

func assertPublishOwnedAbsent(t *testing.T, conn db.Querier, owned publishOwnedIDs) bool {
	t.Helper()
	verified := true
	for _, entry := range []struct {
		table string
		ids   []string
	}{
		{"users", []string{owned.Author}},
		{"questions", owned.Questions},
		{"tests", owned.Tests},
		{"test_versions", owned.Versions},
		{"audit_log", owned.Audit},
	} {
		var count int
		query := "SELECT count(*) FROM " + pgx.Identifier{"app", entry.table}.Sanitize() + " WHERE id::text = ANY($1::text[])"
		if err := conn.QueryRow(context.Background(), query, entry.ids).Scan(&count); err != nil {
			t.Errorf("snapshot fixture %s absence: %v", entry.table, err)
			verified = false
			continue
		}
		if count != 0 {
			t.Errorf("snapshot fixture %s retained %d owned rows", entry.table, count)
			verified = false
		}
	}
	return verified
}
