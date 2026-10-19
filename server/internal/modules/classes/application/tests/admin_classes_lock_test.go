//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type handover struct {
	classID, teacherID, heirID string
}

func committedHandover(t *testing.T, pool *pgxpool.Pool) handover {
	t.Helper()
	ctx := context.Background()
	var h handover
	t.Cleanup(func() {
		c := context.Background()
		var users []string
		for _, id := range []string{h.teacherID, h.heirID} {
			if id != "" {
				users = append(users, id)
			}
		}
		if h.classID != "" {
			for _, sql := range []string{`DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, `DELETE FROM app.classes WHERE id = $1::uuid`} {
				if _, err := pool.Exec(c, sql, h.classID); err != nil {
					t.Errorf("clean up the handover's class: %v", err)
				}
			}
		}
		if _, err := pool.Exec(c, `DELETE FROM app.users WHERE id = ANY($1::uuid[])`, users); err != nil {
			t.Errorf("clean up the handover's accounts: %v", err)
		}
	})
	insert := func(dest *string, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("set up the handover: %v", err)
		}
	}
	n := nonce(t)
	const teacher = `INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`
	insert(&h.teacherID, teacher, "giao-"+n+"@example.com", "Giáo viên cũ")
	insert(&h.heirID, teacher, "nhan-"+n+"@example.com", "Giáo viên mới")
	insert(&h.classID, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp bàn giao "+n, h.teacherID)
	return h
}

func (h handover) changeIn(pool *pgxpool.Pool) <-chan error {
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(pool, joinKeys).Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{
			ClassID: h.classID, TeacherID: h.heirID,
			Actor: actor.Actor{ID: h.teacherID, Scope: access.Scope{UserID: h.teacherID, All: true}},
		})
		done <- err
	}()
	return done
}

func teacherOfClass(t *testing.T, pool *pgxpool.Pool, classID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `SELECT teacher_id::text FROM app.classes WHERE id = $1`, classID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAChangeOfTeacherWaitsForADemotionAndThenRefusesTheStudent(t *testing.T) {
	pool := newPool(t)
	h := committedHandover(t, pool)
	changePool, changePid := ownConnection(t)
	demotion := newHeldTx(t, pool, `UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'student') WHERE id = $1`, h.heirID)

	changed := h.changeIn(changePool)
	waitUntilBlocked(t, pool, changePid, demotion.pid, "the change of teacher, on the heir's account a demotion holds")
	if err := demotion.commit(); err != nil {
		t.Fatalf("commit the demotion: %v", err)
	}

	if err := finished(t, "the change of teacher", changed); !errors.Is(err, domain.ErrInvalidTeacher) {
		t.Errorf("the change after the heir became a student: %v, want ErrInvalidTeacher", err)
	}
	if got := teacherOfClass(t, pool, h.classID); got != h.teacherID {
		t.Errorf("the class is taught by %s, want its teacher before the change: a student must never teach it", got)
	}
}

func TestAChangeOfTeacherLocksTheAccountBeforeTheClass(t *testing.T) {
	pool := newPool(t)
	h := committedHandover(t, pool)
	changePool, changePid := ownConnection(t)
	transfer := newHeldTx(t, pool, `SELECT 1 FROM app.users WHERE id = $1 FOR UPDATE`, h.heirID)

	changed := h.changeIn(changePool)
	waitUntilBlocked(t, pool, changePid, transfer.pid, "the change of teacher, on the heir's account a transfer holds")
	ctx := context.Background()
	if _, err := transfer.tx.Exec(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := transfer.tx.Exec(ctx, `SELECT 1 FROM app.classes WHERE id = $1 FOR UPDATE`, h.classID); err != nil {
		t.Fatalf("the transfer, holding the account, could not take the class: %v; the change of teacher took the class first", err)
	}
	if err := transfer.commit(); err != nil {
		t.Fatalf("commit the transfer: %v", err)
	}

	if err := finished(t, "the change of teacher", changed); err != nil {
		t.Fatalf("the change of teacher after the transfer: %v", err)
	}
	if got := teacherOfClass(t, pool, h.classID); got != h.heirID {
		t.Errorf("the class is taught by %s, want the heir", got)
	}
}
