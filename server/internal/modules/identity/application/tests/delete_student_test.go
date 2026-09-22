//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"testing"
)

func TestStudentDeletionPreservesAuditActors(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "10.00")
	svc := application.New(nil, nil, 0, repositories.NewStudents(db.NewContext(pool)), nil)
	ctx := context.Background()
	req := domain.WriteRequest{ActorID: w.admin}
	if _, err := svc.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: req, ID: w.student}); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("active deletion = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.student); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.audit_log (actor_user_id, action, entity) VALUES ($1, 'login', 'user')`, w.student); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: req, ID: w.student}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("audit actor deletion = %v", err)
	}
	var retained int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1`, w.student).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if retained == 0 {
		t.Fatal("audit actor was rewritten or removed")
	}
}

func TestUnusedDisabledStudentCanBePermanentlyDeleted(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "10.00")
	svc := application.New(nil, nil, 0, repositories.NewStudents(db.NewContext(pool)), nil)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.student); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: domain.WriteRequest{ActorID: w.admin}, ID: w.student}); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.users WHERE id = $1)`, w.student).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("deleted student still exists")
	}
}
