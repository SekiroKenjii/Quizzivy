//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestOwnedQuestionsRequireTheirGroupForReadsWritesAndCopies(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := repositories.NewPostgres(db.NewContext(tx))
	before, _, err := repo.Counts(ctx, domain.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	var group, other string
	for _, id := range []*string{&group, &other} {
		if err := tx.QueryRow(ctx, `INSERT INTO app.question_groups (title,created_by) VALUES ('Context',$1) RETURNING id`, author).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	in := domain.WriteInput{ID: id.String(), ActorID: author, Now: time.Now(), Input: domain.Input{
		Type: domain.SingleChoice, Prompt: "Chọn theo ngữ liệu", Points: "1", Tags: []string{"context-" + group},
		Options: []domain.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B"}},
	}}
	created, err := repo.CreateOwned(ctx, in, domain.GroupOwnership{GroupID: group, Ordinal: 0, OptionOrder: "fixed"})
	if err != nil || created.ID != in.ID {
		t.Fatalf("owned create: %s, %v", created.ID, err)
	}
	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("standalone read detached context: %v", err)
	}
	if _, err := repo.GetIncludingDeleted(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("including-deleted read detached context: %v", err)
	}
	if _, err := repo.Update(ctx, in); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("standalone update bypassed group revision: %v", err)
	}
	if err := repo.SoftDelete(ctx, in); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("standalone delete detached child: %v", err)
	}
	if err := repo.LockForDraftUse(ctx, tx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("lone child could enter a legacy outline: %v", err)
	}
	if changed, err := repo.AddTags(ctx, []string{created.ID}, []string{"bypass"}); err != nil || changed != 0 {
		t.Fatalf("bulk tag bypass: %d, %v", changed, err)
	}
	svc := application.New(repo, nil)
	if _, err := svc.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: domain.WriteRequest{ID: created.ID, ActorID: author}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("standalone duplicate dropped group context: %v", err)
	}
	if _, err := repo.UpdateOwned(ctx, in, domain.GroupOwnership{GroupID: other, Ordinal: 0, OptionOrder: "fixed"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another group could edit member: %v", err)
	}
	in.Input.Prompt = "Đã sửa trong nhóm"
	updated, err := repo.UpdateOwned(ctx, in, domain.GroupOwnership{GroupID: group, Ordinal: 0, OptionOrder: "fixed"})
	if err != nil || updated.Prompt != in.Input.Prompt {
		t.Fatalf("group-owned update: %q, %v", updated.Prompt, err)
	}
	members, err := repo.GroupQuestions(ctx, group)
	if err != nil || len(members) != 1 || members[0].Question.ID != created.ID || members[0].Ordinal != 0 || members[0].OptionOrder != "fixed" || len(members[0].Question.Options) != 2 {
		t.Fatalf("resolved members: %+v, %v", members, err)
	}
	filters := domain.ListInput{Tags: in.Input.Tags}
	items, page, err := repo.List(ctx, filters)
	if err != nil || len(items) != 0 || page.Total != 0 {
		t.Fatalf("bank listed context-only child: %d/%d, %v", len(items), page.Total, err)
	}
	facets, err := repo.Facets(ctx, filters)
	if err != nil || facets.All != 0 {
		t.Fatalf("bank facet included owned child: %+v, %v", facets, err)
	}
	total, filtered, err := repo.Counts(ctx, filters)
	if err != nil || total != before || filtered != 0 {
		t.Fatalf("bank counts included owned child: %d/%d, previous=%d, %v", total, filtered, before, err)
	}
	tags, err := repo.Tags(ctx, domain.ListInput{Query: in.Input.Prompt})
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range tags {
		if tag == in.Input.Tags[0] {
			t.Fatal("bank tags exposed a context-only child")
		}
	}
}
