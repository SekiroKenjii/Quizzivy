//go:build integration

package application_test

import (
	"context"
	"maps"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/shared/access"
)

type staffMember struct {
	id    string
	email string
}

func staffInClass(t *testing.T, pool *pgxpool.Pool, w world, target string) staffMember {
	t.Helper()
	member := staffMember{id: uuid.NewString(), email: "staff-" + nonce(t) + "@example.com"}
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM app.attempts WHERE student_id = $1::uuid`,
			`DELETE FROM app.assignment_students WHERE user_id = $1::uuid`,
			`DELETE FROM app.class_members WHERE user_id = $1::uuid`,
			`DELETE FROM app.users WHERE id = $1::uuid`,
		} {
			if _, err := pool.Exec(context.Background(), sql, member.id); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
	})
	execAll(t, pool, `
		INSERT INTO app.users (id, email, full_name, role_id)
		VALUES ($1::uuid, $2, 'Cô Thương', (SELECT r.id FROM app.roles r WHERE r.builtin_key = 'teacher'))`, member.id, member.email)
	if target == "class" {
		execAll(t, pool, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, w.class, member.id, w.admin)
		return member
	}
	execAll(t, pool, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1::uuid, $2::uuid)`, w.assignment, member.id)
	return member
}

func TestTheMonitorAndTheExportListTheSameAccountsWhenStaffOnlyJoinedTheClass(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	enrol(t, pool, w, 2)
	joined := staffInClass(t, pool, w, "class")
	named := staffInClass(t, pool, w, "assignment")
	service := newService(t, pool)

	listed := func(scope access.Scope) (monitor, export map[string]string) {
		t.Helper()
		shown, err := service.Queries.Monitor.Handle(context.Background(), query.Monitor{Scope: scope, AssignmentID: w.assignment})
		if err != nil {
			t.Fatal(err)
		}
		monitor = map[string]string{}
		for _, row := range shown.Rows {
			monitor[emailOf(t, pool, row.StudentID)] = row.State
		}
		file, err := exportOf(t, pool, scope, w.assignment)
		if err != nil {
			t.Fatal(err)
		}
		export = map[string]string{}
		for _, row := range file.Rows {
			export[row.Email] = row.State
		}
		return monitor, export
	}

	for name, scope := range map[string]access.Scope{
		"the teacher who owns the class":  {UserID: w.admin},
		"a caller who reaches everything": everyone,
	} {
		monitor, export := listed(scope)
		if len(monitor) != 3 {
			t.Errorf("%s: the monitor lists %d accounts %v, want the world's student and the two enrolled", name, len(monitor), monitor)
		}
		for _, absent := range []staffMember{joined, named} {
			if _, listed := monitor[absent.email]; listed {
				t.Errorf("%s: the monitor lists %s, who is not a student and has not sat the paper", name, absent.email)
			}
		}
		if !maps.Equal(monitor, export) {
			t.Errorf("%s: the monitor lists %v where the export lists %v", name, monitor, export)
		}
	}

	handIn(t, pool, w, joined.id, 1, "in_progress")
	handIn(t, pool, w, named.id, 1, "voided")
	for name, scope := range map[string]access.Scope{
		"the teacher who owns the class":  {UserID: w.admin},
		"a caller who reaches everything": everyone,
	} {
		monitor, export := listed(scope)
		if len(monitor) != 5 || monitor[joined.email] != "in_progress" || monitor[named.email] != "voided" {
			t.Errorf("%s: after they sat, the monitor lists %v, want both with the state of their paper beside the three students", name, monitor)
		}
		if !maps.Equal(monitor, export) {
			t.Errorf("%s: after they sat, the monitor lists %v where the export lists %v", name, monitor, export)
		}
	}
}
