//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	assignmentrepo "quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type homeWorld struct {
	tx                                  pgx.Tx
	a, b, admin                         string
	classA, classB, classB2             string
	s1, s2, s3, s4, s5, s6, s7, s8, t1  string
	version, question                   string
	aA, aB, aB2, aM, aOldB              string
	p1, p2, p3, p4, p5, p6, p7, p8, p10 string
}

func (w *homeWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *homeWorld) user(t *testing.T, builtin string, creator *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id, created_by) VALUES ($1, 'Trang chủ', (SELECT id FROM app.roles WHERE builtin_key = $2), $3) RETURNING id::text`,
		"home-"+nonce(t)+"@example.test", builtin, creator)
}

func (w *homeWorld) class(t *testing.T, teacher string, members ...string) string {
	t.Helper()
	class := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp', $1) RETURNING id::text`, teacher)
	for _, m := range members {
		w.id(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3) RETURNING user_id::text`, class, m, teacher)
	}
	return class
}

func (w *homeWorld) assignment(t *testing.T, author, opens, closes string, classes []string, students []string) string {
	t.Helper()
	id := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts, created_by, published_at)
		SELECT v.test_id, v.id, now() + $2::interval, now() + $3::interval, 45, 3, $4, now() - interval '4 days' FROM app.test_versions v WHERE v.id = $1 RETURNING id::text`,
		w.version, opens, closes, author)
	for _, c := range classes {
		w.id(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2) RETURNING class_id::text`, id, c)
	}
	for _, s := range students {
		w.id(t, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2) RETURNING user_id::text`, id, s)
	}
	return id
}

func (w *homeWorld) paper(t *testing.T, assignment, student, started string, pending, flagged bool) string {
	t.Helper()
	status, graded := "submitted", "NULL"
	if !pending {
		status, graded = "graded", "now()"
	}
	id := w.id(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		        started_at, deadline_at, submitted_at, graded_at, flagged)
		VALUES ($1, $2, $3, 1, $4::app.attempt_status, gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - $5::interval, now() - $5::interval + interval '45 minutes', now() - $5::interval + interval '20 minutes', `+graded+`, $6) RETURNING id::text`,
		assignment, w.version, student, status, started, flagged)
	if pending {
		w.id(t, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual) VALUES ($1, $2, '{"text":"x"}'::jsonb, true) RETURNING attempt_id::text`, id, w.question)
	}
	return id
}

func newHomeWorld(t *testing.T) *homeWorld {
	t.Helper()
	return buildHome(t, isolated(t, newPool(t)))
}

func buildHome(t *testing.T, tx pgx.Tx) *homeWorld {
	t.Helper()
	w := &homeWorld{tx: tx}
	w.a, w.b, w.admin = w.user(t, "teacher", nil), w.user(t, "teacher", nil), w.user(t, "admin", nil)
	w.s1, w.s2, w.s3, w.s4 = w.user(t, "student", nil), w.user(t, "student", nil), w.user(t, "student", nil), w.user(t, "student", nil)
	w.s5, w.s6, w.s7, w.s8 = w.user(t, "student", &w.b), w.user(t, "student", nil), w.user(t, "student", nil), w.user(t, "student", nil)
	w.t1 = w.user(t, "teacher", nil)
	w.id(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1 RETURNING id::text`, w.s6)
	w.classA = w.class(t, w.a, w.s2, w.s3)
	w.classB = w.class(t, w.b, w.s1, w.s3, w.s6, w.t1, w.s8)
	w.classB2 = w.class(t, w.b, w.s7)
	w.id(t, `UPDATE app.classes SET archived_at = now() WHERE id = $1 RETURNING id::text`, w.classB2)
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề trang chủ', 'published', 1, $1, $1) RETURNING id::text`, w.admin)
	w.version = w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 5, $2) RETURNING id::text`, test, w.admin)
	section := w.id(t, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, w.version)
	w.question = w.id(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points) VALUES ($1, 0, 'short_answer', 'Viết', 5) RETURNING id::text`, section)

	w.aA = w.assignment(t, w.a, "-2 hours", "30 minutes", []string{w.classA}, nil)
	w.aB = w.assignment(t, w.b, "-2 hours", "5 hours", []string{w.classB}, nil)
	w.aB2 = w.assignment(t, w.b, "-2 hours", "48 hours", nil, []string{w.s4})
	w.aM = w.assignment(t, w.admin, "-2 hours", "1 hour", []string{w.classA, w.classB}, []string{w.s4})
	w.aOldB = w.assignment(t, w.b, "-3 days", "-1 day", []string{w.classB}, nil)

	w.p1 = w.paper(t, w.aB, w.s1, "50 minutes", true, true)
	w.p2 = w.paper(t, w.aA, w.s2, "45 minutes", true, true)
	w.p3 = w.paper(t, w.aA, w.s3, "40 minutes", true, true)
	w.p4 = w.paper(t, w.aM, w.s2, "35 minutes", true, true)
	w.p5 = w.paper(t, w.aM, w.s1, "30 minutes", true, false)
	w.p6 = w.paper(t, w.aB2, w.s4, "25 minutes", true, false)
	w.p7 = w.paper(t, w.aOldB, w.s8, "2 days", true, true)
	w.p8 = w.paper(t, w.aB, w.t1, "20 minutes", false, false)
	w.p10 = w.paper(t, w.aB, w.s3, "10 days", false, false)
	w.id(t, `DELETE FROM app.class_members WHERE class_id = $1 AND user_id = $2 RETURNING user_id::text`, w.classB, w.s8)
	return w
}

func (w *homeWorld) submitted(t *testing.T, attempt string) *time.Time {
	t.Helper()
	var at time.Time
	if err := w.tx.QueryRow(context.Background(), `SELECT submitted_at FROM app.attempts WHERE id = $1`, attempt).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return &at
}

func (w *homeWorld) summary(t *testing.T, scope access.Scope) domain.Summary {
	t.Helper()
	app := application.New(repositories.NewPostgres(db.NewContext(w.tx)))
	out, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type figures struct {
	open, awaiting, active, flagged, closingSoon, waiting, total int
	oldest                                                       *time.Time
	next                                                         string
	submittedCount, targetCount                                  int
	recent                                                       []string
}

func figuresOf(s domain.Summary) figures {
	f := figures{open: s.OpenAssignments, awaiting: s.AwaitingGrading, active: s.ActiveStudents, flagged: s.FlaggedAttempts,
		closingSoon: s.ClosingSoon, waiting: s.WaitingStudents, total: s.TotalStudents, oldest: s.OldestWaitingAt, recent: ids(s.Recent)}
	if s.NextClosing != nil {
		f.next, f.submittedCount, f.targetCount = s.NextClosing.ID, s.NextClosing.SubmittedCount, s.NextClosing.TargetCount
	}
	return f
}

func sameFigures(a, b figures) bool {
	sameOldest := (a.oldest == nil) == (b.oldest == nil) && (a.oldest == nil || a.oldest.Equal(*b.oldest))
	return sameOldest && a.open == b.open && a.awaiting == b.awaiting && a.active == b.active && a.flagged == b.flagged &&
		a.closingSoon == b.closingSoon && a.waiting == b.waiting && a.total == b.total && a.next == b.next &&
		a.submittedCount == b.submittedCount && a.targetCount == b.targetCount && slices.Equal(a.recent, b.recent)
}

func TestEachReaderSeesOnlyWhatTheyReachOnTheirHome(t *testing.T) {
	w := newHomeWorld(t)
	for name, c := range map[string]struct {
		scope access.Scope
		want  figures
	}{
		"B": {access.Scope{UserID: w.b}, figures{open: 3, awaiting: 4, active: 1, flagged: 2, closingSoon: 2, waiting: 3, total: 3,
			oldest: w.submitted(t, w.p7), next: w.aM, submittedCount: 2, targetCount: 4, recent: []string{w.p8, w.p6, w.p5, w.p1, w.p7, w.p10}}},
		"A": {access.Scope{UserID: w.a}, figures{open: 2, awaiting: 3, active: 2, flagged: 3, closingSoon: 2, waiting: 2, total: 2,
			oldest: w.submitted(t, w.p2), next: w.aA, submittedCount: 2, targetCount: 2, recent: []string{w.p4, w.p3, w.p2}}},
		"the Admin without scope.all": {access.Scope{UserID: w.admin}, figures{open: 1, awaiting: 2, active: 0, flagged: 1, closingSoon: 1, waiting: 2, total: 0,
			oldest: w.submitted(t, w.p4), next: w.aM, submittedCount: 2, targetCount: 1, recent: []string{w.p5, w.p4}}},
		"the zero scope": {access.Scope{}, figures{recent: []string{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := figuresOf(w.summary(t, c.scope)); !sameFigures(got, c.want) {
				t.Errorf("reads %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestEveryFigureAgreesWithThePageItOpens(t *testing.T) {
	w := newHomeWorld(t)
	ctx := context.Background()
	store := repositories.NewPostgres(db.NewContext(w.tx))
	assignments := assignmentrepo.NewPostgres(db.NewContext(w.tx))
	yes := true
	for name, scope := range map[string]access.Scope{"A": {UserID: w.a}, "B": {UserID: w.b}, "the Admin without scope.all": {UserID: w.admin}} {
		t.Run(name, func(t *testing.T) {
			home := w.summary(t, scope)
			flagged, flaggedPage, err := store.List(ctx, domain.ListQuery{Scope: scope, Flagged: &yes, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			if home.FlaggedAttempts != flaggedPage.Total || home.FlaggedAttempts != len(flagged) {
				t.Errorf("flagged %d, the flagged queue holds %d", home.FlaggedAttempts, flaggedPage.Total)
			}
			queue, _, err := store.List(ctx, domain.ListQuery{Scope: scope, PendingGrading: &yes, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			pending, students := 0, map[string]bool{}
			var oldest *time.Time
			for _, r := range queue {
				pending += r.PendingManual
				students[r.StudentID] = true
				if r.SubmittedAt != nil && (oldest == nil || r.SubmittedAt.Before(*oldest)) {
					oldest = r.SubmittedAt
				}
			}
			if home.AwaitingGrading != pending || home.WaitingStudents != len(students) || oldest == nil || !home.OldestWaitingAt.Equal(*oldest) {
				t.Errorf("awaiting %d from %d students since %v, the queue holds %d from %d since %v",
					home.AwaitingGrading, home.WaitingStudents, home.OldestWaitingAt, pending, len(students), oldest)
			}
			all, _, err := store.List(ctx, domain.ListQuery{Scope: scope, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range ids(home.Recent) {
				if !has(all, id) {
					t.Errorf("recent attempt %s is not in the reader's attempt list", id)
				}
			}
			if home.ClosingSoon == 0 || home.NextClosing == nil {
				t.Fatalf("closing soon %d with next %v", home.ClosingSoon, home.NextClosing)
			}
			embed, err := assignments.Get(ctx, scope, home.NextClosing.ID)
			if err != nil {
				t.Fatalf("the next closing assignment does not open for its reader: %v", err)
			}
			if embed.TargetCount != home.NextClosing.TargetCount || embed.SubmittedCount != home.NextClosing.SubmittedCount {
				t.Errorf("next closing reads %d/%d, the assignment %d/%d",
					home.NextClosing.SubmittedCount, home.NextClosing.TargetCount, embed.SubmittedCount, embed.TargetCount)
			}
		})
	}
}

func TestScopeAllReadsExactlyWhatTheUnscopedHomeRead(t *testing.T) {
	pool := newPool(t)
	tx := isolated(t, pool)
	before := legacySummary(t, tx)
	w := buildHome(t, tx)
	after := legacySummary(t, tx)
	if after.OpenAssignments-before.OpenAssignments != 4 || after.TotalStudents-before.TotalStudents != 7 {
		t.Fatalf("the fixture moved open by %d and students by %d, want 4 and 7",
			after.OpenAssignments-before.OpenAssignments, after.TotalStudents-before.TotalStudents)
	}
	for name, scope := range map[string]access.Scope{"everyone": everyone, "a scope.all reader reaching nothing": {UserID: w.b, All: true}} {
		if got, want := figuresOf(w.summary(t, scope)), figuresOf(after); !sameFigures(got, want) {
			t.Errorf("%s reads %+v, the unscoped home %+v", name, got, want)
		}
	}
}

func legacySummary(t *testing.T, q db.Querier) domain.Summary {
	t.Helper()
	ctx := context.Background()
	var out domain.Summary
	if err := q.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM app.assignments a
		    WHERE a.published_at IS NOT NULL
		      AND a.closed_at IS NULL
		      AND now() >= a.opens_at AND now() < a.closes_at),
		  (SELECT count(*)
		     FROM app.attempt_answers ans
		     JOIN app.attempts at ON at.id = ans.attempt_id
		    WHERE ans.requires_manual AND ans.manual_score IS NULL
		      AND at.status IN ('submitted', 'timed_out')),
		  (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		    WHERE at.started_at >= now() - $1::interval),
		  (SELECT count(*) FROM app.attempts at WHERE at.flagged),
          (SELECT count(*) FROM app.assignments a WHERE a.published_at IS NOT NULL
             AND a.closed_at IS NULL AND a.opens_at <= now() AND a.closes_at > now()
             AND a.closes_at <= now() + interval '24 hours'),
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.attempt_answers ans ON ans.attempt_id = at.id
             WHERE at.status IN ('submitted','timed_out') AND ans.requires_manual AND ans.manual_score IS NULL),
          (SELECT min(at.submitted_at) FROM app.attempts at
             WHERE at.status IN ('submitted','timed_out') AND EXISTS (
               SELECT 1 FROM app.attempt_answers ans WHERE ans.attempt_id = at.id
                 AND ans.requires_manual AND ans.manual_score IS NULL)),
          (SELECT count(*) FROM app.users WHERE role = 'student' AND disabled_at IS NULL)
	`, domain.ActiveWindow).Scan(
		&out.OpenAssignments, &out.AwaitingGrading, &out.ActiveStudents, &out.FlaggedAttempts,
		&out.ClosingSoon, &out.WaitingStudents, &out.OldestWaitingAt, &out.TotalStudents); err != nil {
		t.Fatal(err)
	}
	var next domain.ClosingAssignment
	err := q.QueryRow(ctx, `
   SELECT a.id::text, t.title, a.closes_at,
          (SELECT count(DISTINCT at.student_id) FROM app.attempts at
             JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
            WHERE at.assignment_id = a.id AND at.status IN ('submitted','timed_out','graded')),
          (SELECT count(*) FROM (
             SELECT m.user_id FROM app.assignment_classes ac
               JOIN app.class_members m ON m.class_id = ac.class_id WHERE ac.assignment_id = a.id
             UNION SELECT ast.user_id FROM app.assignment_students ast WHERE ast.assignment_id = a.id
           ) roster JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL)
     FROM app.assignments a JOIN app.tests t ON t.id = a.test_id
    WHERE a.published_at IS NOT NULL AND a.closed_at IS NULL
      AND a.opens_at <= now() AND a.closes_at > now() AND a.closes_at <= now() + interval '24 hours'
    ORDER BY a.closes_at, a.id LIMIT 1
 `).Scan(&next.ID, &next.Title, &next.ClosesAt, &next.SubmittedCount, &next.TargetCount)
	switch {
	case err == nil:
		out.NextClosing = &next
	case !errors.Is(err, pgx.ErrNoRows):
		t.Fatal(err)
	}
	rows, err := q.Query(ctx, `
		SELECT at.id::text, at.student_id::text, u.full_name,
		       at.assignment_id::text, t.title, at.status::text, at.submitted_at,
		       (SELECT count(*) FROM app.attempt_answers ans
		         WHERE ans.attempt_id = at.id
		           AND ans.requires_manual AND ans.manual_score IS NULL),
		       at.flagged
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  JOIN app.assignments a ON a.id = at.assignment_id
		  JOIN app.tests t ON t.id = a.test_id
		 ORDER BY at.started_at DESC
		 LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.Recent
		if err := rows.Scan(&r.ID, &r.StudentID, &r.StudentName, &r.AssignmentID, &r.TestTitle, &r.Status, &r.SubmittedAt, &r.PendingManual, &r.Flagged); err != nil {
			t.Fatal(err)
		}
		out.Recent = append(out.Recent, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
