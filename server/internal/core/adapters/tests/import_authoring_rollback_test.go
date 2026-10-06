//go:build integration

package adapters_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/core/adapters"
	importsdomain "quizzivy/internal/modules/imports/domain"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionscmd "quizzivy/internal/modules/questions/application/command"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
	"testing"
)

func TestImportAuthoringLateCapRollsBackRealPoolTransactionAndAudits(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	owner := uuid.NewString()
	t.Cleanup(func() {
		c := context.Background()
		var persisted int
		err := pool.QueryRow(c, `SELECT (SELECT count(*) FROM app.questions WHERE created_by=$1)+(SELECT count(*) FROM app.tests WHERE created_by=$1)+(SELECT count(*) FROM app.question_groups WHERE created_by=$1)+(SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1)`, owner).Scan(&persisted)
		if err != nil || persisted != 0 {
			t.Errorf("failed import outside-TX owned absence=%d %v", persisted, err)
			return
		}
		tag, err := pool.Exec(c, `DELETE FROM app.users WHERE id=$1`, owner)
		if err != nil || tag.RowsAffected() != 1 {
			t.Errorf("exact teacher cleanup rows=%d err=%v", tag.RowsAffected(), err)
			return
		}
		var remains bool
		if err := pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM app.users WHERE id=$1)`, owner).Scan(&remains); err != nil || remains {
			t.Errorf("teacher absence remains=%v err=%v", remains, err)
		}
		t.Logf("import rollback and exact teacher cleanup checked owner=%s", owner)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Import teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, owner, owner+"@example.test"); err != nil {
		t.Fatal(err)
	}
	var written []string
	var auditIDs []string
	var createCalls int
	committer := adapters.ImportCommitter{DB: db.NewContext(pool), Tests: func(scoped db.Context) *testsapp.Application {
		return testsapp.New(testsrepo.NewPostgres(scoped, questionsrepo.NewPostgres(scoped), nil))
	}, Questions: func(scoped db.Context) *questionsapp.Application {
		app := questionsapp.New(questionsrepo.NewPostgres(scoped), nil)
		create := app.Commands.Create
		app.Commands.Create = cqrs.HandlerFunc[questionscmd.Create, questionsdomain.Question](func(ctx context.Context, cmd questionscmd.Create) (questionsdomain.Question, error) {
			createCalls++
			q, err := create.Handle(ctx, cmd)
			if err == nil {
				written = append(written, q.ID)
				rows, e := scoped.Query(ctx, `SELECT id::text FROM app.audit_log WHERE actor_user_id=$1 ORDER BY id`, owner)
				if e != nil {
					t.Fatal(e)
				}
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err != nil {
						rows.Close()
						t.Fatal(err)
					}
					auditIDs = append(auditIDs, id)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			return q, err
		})
		return app
	}}
	valid := questionsdomain.Input{Type: questionsdomain.SingleChoice, Prompt: "Valid first", Points: "1", Tags: []string{}, Options: []questionsdomain.OptionInput{{Text: "yes", IsCorrect: true}, {Text: "no"}}}
	invalid := valid
	invalid.Prompt = "Oversized later"
	invalid.Options = make([]questionsdomain.OptionInput, 9)
	for i := range invalid.Options {
		invalid.Options[i] = questionsdomain.OptionInput{Text: "choice", IsCorrect: i == 0}
	}
	plan := importsdomain.CommitPlan{Title: "Rollback", Sections: []importsdomain.PlanSection{{Title: "Part", Units: []importsdomain.PlanUnit{{Question: &valid}, {Question: &invalid}}}}}
	recorded := false
	_, err = committer.Materialize(ctx, plan, owner, actor.Actor{ID: owner, Scope: access.Scope{UserID: owner}}, func(context.Context, importsdomain.CommitStore, string) error { recorded = true; return nil })
	var invalidInput *questionsdomain.ValidationError
	if !errors.As(err, &invalidInput) || len(invalidInput.Fields) != 1 || invalidInput.Fields[0].Field != "options" || createCalls != 2 || len(written) != 1 || len(auditIDs) != 2 || recorded {
		t.Fatalf("late refusal not reached err=%v creates=%d written=%v audits=%v recorded=%v", err, createCalls, written, auditIDs, recorded)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.questions WHERE id=ANY($1::uuid[]))+(SELECT count(*) FROM app.audit_log WHERE id=ANY($2::uuid[]))+(SELECT count(*) FROM app.tests WHERE created_by=$3)`, written, auditIDs, owner).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("real rollback left captured rows=%d err=%v", remaining, err)
	}
	t.Logf("real pool rollback reached owner=%s questions=%v audit=%v", owner, written, auditIDs)
}
