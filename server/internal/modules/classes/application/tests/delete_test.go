//go:build integration

package application_test

import (
	"context"
	"errors"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"testing"
)

func TestDeletingAnArchivedClassKeepsStudentAccounts(t *testing.T) {
	pool := newPool(t)
	id, teacher, student := makeClass(t, pool)
	svc := application.New(repositories.NewPostgres(db.NewContext(pool)), attemptsrepo.NewStudentStats(db.NewContext(pool)))
	ctx := context.Background()
	by := actor.Actor{ID: teacher}
	if _, err := svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: id, UserID: student, Actor: by}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Delete.Handle(ctx, command.Delete{ClassID: id, Actor: by}); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("active deletion = %v", err)
	}
	if _, err := svc.Commands.Archive.Handle(ctx, command.Archive{ClassID: id, Archived: true, Actor: by}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Delete.Handle(ctx, command.Delete{ClassID: id, Actor: by}); err != nil {
		t.Fatal(err)
	}
	var accountExists, membershipExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.users WHERE id = $1), EXISTS (SELECT 1 FROM app.class_members WHERE class_id = $2)`, student, id).Scan(&accountExists, &membershipExists); err != nil {
		t.Fatal(err)
	}
	if !accountExists || membershipExists {
		t.Fatalf("account=%v membership=%v", accountExists, membershipExists)
	}
}
