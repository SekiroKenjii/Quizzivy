//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

// The seat limit is set inside one transaction that is rolled back, so the
// packages sharing the database never see it. Those packages add and remove
// accounts meanwhile, so a try during which the other accounts moved proves
// nothing and is made again.
func TestAJoinCodeMakesNoAccountPastTheSeatLimit(t *testing.T) {
	for _, room := range []int{0, 1} {
		for try := 1; ; try++ {
			outcome, settled := enrolWithRoom(t, room)
			if settled {
				if outcome != "" {
					t.Error(outcome)
				}
				break
			}
			if try == 10 {
				t.Fatalf("room %d: the other accounts moved during every one of %d tries", room, try)
			}
		}
	}
}

func enrolWithRoom(t *testing.T, room int) (string, bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := newPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
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
	before, err := limits.CountAccounts(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET limit_seats = $1`, before.Seats+room); err != nil {
		t.Fatal(err)
	}
	m := newMember(t)
	result, enrolErr := svc.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code, Meta: domain.Meta{}})
	var made int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE email = $1`, m.Email).Scan(&made); err != nil {
		t.Fatal(err)
	}
	after, err := limits.CountAccounts(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seats-made != before.Seats {
		return "", false
	}
	switch {
	case room == 0 && (!errors.Is(enrolErr, limits.ErrSeatLimit) || made != 0):
		return fmt.Sprintf("no room: %v and %d account(s), want ErrSeatLimit and none", enrolErr, made), true
	case room > 0 && (enrolErr != nil || result.Outcome != domain.PreviewOK || made != 1):
		return fmt.Sprintf("room for one: %v %v and %d account(s), want the account enrolled", enrolErr, result.Outcome, made), true
	}
	return "", true
}
