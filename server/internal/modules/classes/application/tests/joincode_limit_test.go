//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

// The seat limit is set inside one transaction that is rolled back, so the
// packages sharing the database never see it, and that transaction holds the
// accounts in SHARE mode, so the seats it counts cannot move under it.
func TestAJoinCodeMakesNoAccountPastTheSeatLimit(t *testing.T) {
	for _, room := range []int{0, 1} {
		ctx := context.Background()
		tx, err := newPool(t).Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(ctx) })
		if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `LOCK TABLE app.users IN SHARE MODE`); err != nil {
			t.Fatal(err)
		}
		svc := application.New(repositories.NewPostgres(db.NewContext(tx)), nil, joinKeys)
		n := nonce(t)
		var teacherID, classID string
		if err := tx.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
			VALUES ($1, 'Giáo viên', (SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`,
			"teacher-"+n+"@example.com").Scan(&teacherID); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`,
			"Lớp "+n, teacherID).Scan(&classID); err != nil {
			t.Fatal(err)
		}
		code := issueCode(t, svc, classID, teacherID)
		used, err := limits.CountAccounts(ctx, tx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET limit_seats = $1`, used.Seats+room); err != nil {
			t.Fatal(err)
		}
		m := newMember(t)
		result, err := svc.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code, Meta: domain.Meta{}})
		var made int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE email = $1`, m.Email).Scan(&made); err != nil {
			t.Fatal(err)
		}
		switch room {
		case 0:
			if !errors.Is(err, limits.ErrSeatLimit) || made != 0 {
				t.Errorf("no room: %v and %d account(s), want ErrSeatLimit and none", err, made)
			}
		default:
			if err != nil || result.Outcome != domain.PreviewOK || made != 1 {
				t.Errorf("room for one: %v %v and %d account(s), want the account enrolled", err, result.Outcome, made)
			}
		}
		_ = tx.Rollback(ctx)
	}
}
