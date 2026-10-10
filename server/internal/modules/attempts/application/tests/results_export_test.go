//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

func exportOf(t *testing.T, pool *pgxpool.Pool, scope access.Scope, assignments ...string) (domain.ResultsExport, error) {
	t.Helper()
	return newService(t, pool).Queries.ResultsExport.Handle(context.Background(), query.ResultsExport{AssignmentIDs: assignments, Scope: scope})
}

func execAll(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%q: %v", sql, err)
	}
}

func scalar(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()
	var out string
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&out); err != nil {
		t.Fatalf("%q: %v", sql, err)
	}
	return out
}

func TestTheExportHasARowForEveryStudentAndTheAttemptTheMonitorShows(t *testing.T) {
	pool := newPool(t)
	o := openAssignment()
	o.maxAttempts = 3
	w := seedWorld(t, pool, o)
	people := enrol(t, pool, w, 6)
	submitted, graded, waiting, sitting, overdue, voided := people[0], people[1], people[2], people[3], people[4], people[5]

	first := handIn(t, pool, w, submitted, 1, "submitted")
	execAll(t, pool, `UPDATE app.attempts SET focus_loss_count = 2, flagged = true WHERE id = $1::uuid`, first)
	handIn(t, pool, w, submitted, 2, "voided")

	done := handIn(t, pool, w, graded, 1, "submitted")
	execAll(t, pool, `UPDATE app.attempts SET status = 'graded', graded_at = now() WHERE id = $1::uuid`, done)

	pending := handIn(t, pool, w, waiting, 1, "submitted")
	saved(t, pool, pending, w.essay, essay("Chưa chấm."), nil, nil, true)

	handIn(t, pool, w, sitting, 1, "in_progress")
	late := handIn(t, pool, w, overdue, 1, "in_progress")
	execAll(t, pool, `UPDATE app.attempts SET deadline_at = now() - interval '1 minute' WHERE id = $1::uuid`, late)
	handIn(t, pool, w, voided, 1, "voided")

	export, err := exportOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.Rows) != 7 {
		t.Fatalf("%d rows, want one for each of the seven students: %+v", len(export.Rows), export.Rows)
	}
	rows := map[string]domain.ResultRow{}
	for _, r := range export.Rows {
		rows[r.StudentName+" "+r.State] = r
	}
	for name, want := range map[string]string{
		"Học viên Aa": "submitted", "Học viên Ba": "graded", "Học viên Ca": "submitted", "Học viên Da": "in_progress",
		"Học viên Ea": "timed_out", "Học viên Fa": "voided", "Người dùng": "not_started",
	} {
		if _, ok := rows[name+" "+want]; !ok {
			t.Errorf("no row for %s in state %s: %+v", name, want, export.Rows)
		}
	}

	scored := rows["Học viên Aa submitted"]
	if scored.Earned == nil || *scored.Earned != 6.5 || scored.Total == nil || *scored.Total != 10 || scored.SubmittedAt == nil {
		t.Errorf("a handed-in paper reads %+v, want 6.5 of 10 and its time", scored)
	}
	if scored.FocusLoss == nil || *scored.FocusLoss != 2 || !scored.Flagged {
		t.Errorf("the paper's integrity figures read %+v, want 2 and flagged", scored)
	}
	if scored.AssignmentTitle == "" || scored.Version != 1 || scored.Email == "" || scored.Classes == "" {
		t.Errorf("the row names %+v, want the test's title, its version, the student's email and their class", scored)
	}
	if r := rows["Học viên Ca submitted"]; r.Earned != nil || r.Total != nil {
		t.Errorf("a paper with a manual mark outstanding reads %v of %v, want no score yet", deref(r.Earned), deref(r.Total))
	}
	if r := rows["Học viên Fa voided"]; r.Earned != nil || r.Total != nil {
		t.Errorf("a voided paper reads a score: %+v", r)
	}
	if r := rows["Người dùng not_started"]; r.Earned != nil || r.SubmittedAt != nil || r.FocusLoss != nil || r.Flagged {
		t.Errorf("a student who has not started reads %+v, want blanks", r)
	}
	if r := rows["Học viên Ea timed_out"]; r.SubmittedAt == nil {
		t.Errorf("the overdue attempt was closed with no time: %+v", r)
	}
}

func TestTheExportLeavesOutADisabledStudentAndNamesEveryClassTheReaderReaches(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	people := enrol(t, pool, w, 3)
	inTwo, disabled, named := people[0], people[1], people[2]
	ctx := context.Background()

	second := uuid.NewString()
	t.Cleanup(func() {
		for _, c := range []struct {
			sql string
			arg string
		}{
			{`DELETE FROM app.assignment_students WHERE assignment_id = $1::uuid`, w.assignment},
			{`DELETE FROM app.assignment_classes WHERE class_id = $1::uuid`, second},
			{`DELETE FROM app.class_members WHERE class_id = $1::uuid`, second},
			{`DELETE FROM app.classes WHERE id = $1::uuid`, second},
		} {
			if _, err := pool.Exec(ctx, c.sql, c.arg); err != nil {
				t.Errorf("cleanup %q: %v", c.sql, err)
			}
		}
	})
	secondName := "Khối chọn " + second
	execAll(t, pool, `INSERT INTO app.classes (id, name, teacher_id) VALUES ($1::uuid, $2, $3::uuid)`, second, secondName, w.admin)
	execAll(t, pool, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1::uuid, $2::uuid)`, w.assignment, second)
	execAll(t, pool, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, second, inTwo, w.admin)
	execAll(t, pool, `DELETE FROM app.class_members WHERE user_id = $1::uuid`, named)
	execAll(t, pool, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1::uuid, $2::uuid)`, w.assignment, named)
	execAll(t, pool, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, disabled)

	export, err := exportOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, r := range export.Rows {
		classes[r.Email] = r.Classes
	}
	if len(classes) != 3 {
		t.Fatalf("%d rows, want the world's student, the one in two classes and the one named alone: %+v", len(export.Rows), export.Rows)
	}
	firstName := scalar(t, pool, `SELECT name FROM app.classes WHERE id = $1::uuid`, w.class)
	if got, want := classes[emailOf(t, pool, inTwo)], secondName+"; "+firstName; got != want {
		t.Errorf("a student in two target classes reads %q, want %q", got, want)
	}
	if got, present := classes[emailOf(t, pool, named)]; !present || got != "" {
		t.Errorf("a student named without a class reads %q (row present %v), want a row with no class", got, present)
	}
	if _, present := classes[emailOf(t, pool, disabled)]; present {
		t.Error("a disabled student has a row")
	}
}

func emailOf(t *testing.T, pool *pgxpool.Pool, user string) string {
	t.Helper()
	return scalar(t, pool, `SELECT email FROM app.users WHERE id = $1::uuid`, user)
}

func TestTheExportKeepsTheOrderOfTheIdsAndRefusesTheWholeRequestForOneItDoesNotReach(t *testing.T) {
	pool := newPool(t)
	one := seedWorld(t, pool, openAssignment())
	other := seedWorld(t, pool, openAssignment())
	enrol(t, pool, one, 1)
	enrol(t, pool, other, 1)

	export, err := exportOf(t, pool, everyone, other.assignment, one.assignment)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.Rows) != 4 {
		t.Fatalf("%d rows, want two students on each assignment", len(export.Rows))
	}
	wanted := scalar(t, pool, `SELECT title FROM app.tests WHERE id = $1::uuid`, other.testID)
	if export.Rows[0].AssignmentTitle != wanted || export.Rows[1].AssignmentTitle != wanted ||
		export.Rows[2].AssignmentTitle == wanted || export.Rows[3].AssignmentTitle == wanted {
		t.Errorf("the rows are not grouped in the order the ids were named: %+v", export.Rows)
	}

	stranger, missing := access.Scope{UserID: uuid.NewString()}, uuid.NewString()
	for _, c := range []struct {
		name  string
		scope access.Scope
		ids   []string
	}{
		{"an assignment the caller does not reach", stranger, []string{one.assignment}},
		{"one reached and one missing", everyone, []string{one.assignment, missing}},
		{"only a missing one", everyone, []string{missing}},
		{"a repeated id and a missing one", everyone, []string{one.assignment, one.assignment, missing}},
	} {
		if _, err := exportOf(t, pool, c.scope, c.ids...); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: got %v, want ErrNotFound for the whole request", c.name, err)
		}
	}
	if _, err := exportOf(t, pool, access.Scope{UserID: one.admin}, one.assignment, one.assignment); err != nil {
		t.Errorf("an id named twice by a teacher who reaches it: %v", err)
	}
}

func TestTheExportStopsAtTwentyThousandRows(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	ctx := context.Background()
	tag := uuid.NewString()
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM app.class_members WHERE user_id IN (SELECT id FROM app.users WHERE email LIKE $1)`,
			`DELETE FROM app.users WHERE email LIKE $1`,
		} {
			if _, err := pool.Exec(ctx, sql, "cap-"+tag+"-%"); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
	})
	fill := func(from, to int) {
		t.Helper()
		execAll(t, pool, `
			WITH added AS (
			  INSERT INTO app.users (email, full_name, role_id)
			  SELECT 'cap-' || $1::text || '-' || g || '@example.com', 'Học viên ' || g,
			         (SELECT id FROM app.roles WHERE builtin_key = 'student')
			    FROM generate_series($2::int, $3::int) g
			  RETURNING id
			)
			INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			SELECT $4::uuid, id, 'admin', $5::uuid FROM added`, tag, from, to, w.class, w.admin)
	}

	fill(1, domain.MaxExportRows-1)
	export, err := exportOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatalf("exactly %d rows: %v", domain.MaxExportRows, err)
	}
	if len(export.Rows) != domain.MaxExportRows {
		t.Fatalf("%d rows, want %d", len(export.Rows), domain.MaxExportRows)
	}

	fill(domain.MaxExportRows, domain.MaxExportRows)
	if _, err := exportOf(t, pool, everyone, w.assignment); !errors.Is(err, domain.ErrExportTooLarge) {
		t.Errorf("%d rows: got %v, want ErrExportTooLarge", domain.MaxExportRows+1, err)
	}
}
