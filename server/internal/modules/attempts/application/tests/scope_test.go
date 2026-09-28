//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	dashboarddomain "quizzivy/internal/modules/dashboard/domain"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type paperWorld struct {
	tx                        pgx.Tx
	store                     *repositories.Postgres
	reviews                   *repositories.Reviews
	timelines                 *repositories.Timelines
	a, b, admin               string
	studentA, studentB, loose string
	classA                    string
	version, essay            string
	ofA, byAdmin              string
	paperA, live, overdue     string
	paperB, paperLoose        string
}

func (w *paperWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *paperWorld) user(t *testing.T, builtin string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Phạm vi '||$1, (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin)
}

func (w *paperWorld) class(t *testing.T, teacher string, members ...string) string {
	t.Helper()
	class := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp', $1) RETURNING id::text`, teacher)
	for _, m := range members {
		w.id(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3) RETURNING user_id::text`, class, m, teacher)
	}
	return class
}

func (w *paperWorld) assignment(t *testing.T, author string, class string, student *string) string {
	t.Helper()
	id := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts, created_by, published_at)
		SELECT v.test_id, v.id, now() - interval '2 hours', now() + interval '2 hours', 45, 5, $2, now() FROM app.test_versions v WHERE v.id = $1 RETURNING id::text`,
		w.version, author)
	w.id(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2) RETURNING class_id::text`, id, class)
	if student != nil {
		w.id(t, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2) RETURNING user_id::text`, id, *student)
	}
	return id
}

func (w *paperWorld) attempt(t *testing.T, assignment, student string, no int, status, deadline string) string {
	t.Helper()
	submitted := "now()"
	if status == "in_progress" {
		submitted = "NULL"
	}
	id := w.id(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		        started_at, deadline_at, submitted_at)
		VALUES ($1, $2, $3, $4, $5::app.attempt_status, gen_random_uuid(), 1, sha256('b'::bytea), now() - interval '40 minutes', `+deadline+`, `+submitted+`) RETURNING id::text`,
		assignment, w.version, student, no, status)
	if status == "submitted" {
		w.id(t, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual) VALUES ($1, $2, '{"type":"text","value":"x"}'::jsonb, true) RETURNING attempt_id::text`, id, w.essay)
	}
	return id
}

func newPaperWorld(t *testing.T) *paperWorld {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	dbx := db.NewContext(tx)
	w := &paperWorld{tx: tx, store: repositories.NewPostgres(dbx, adapters.AttemptStartGuard{}), reviews: repositories.NewReviews(dbx), timelines: repositories.NewTimelines(dbx)}
	w.a, w.b, w.admin = w.user(t, "teacher"), w.user(t, "teacher"), w.user(t, "admin")
	w.studentA, w.studentB, w.loose = w.user(t, "student"), w.user(t, "student"), w.user(t, "student")
	w.classA = w.class(t, w.a, w.studentA)
	classB := w.class(t, w.b, w.studentB)
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, w.admin)
	w.version = w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 5, $2) RETURNING id::text`, test, w.admin)
	section := w.id(t, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, w.version)
	w.essay = w.id(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, sample_answer) VALUES ($1, 0, 'short_answer', 'Viết', 5, 'x') RETURNING id::text`, section)
	w.ofA = w.assignment(t, w.a, w.classA, nil)
	w.byAdmin = w.assignment(t, w.admin, classB, &w.loose)
	w.paperA = w.attempt(t, w.ofA, w.studentA, 1, "submitted", "now() + interval '5 minutes'")
	w.live = w.attempt(t, w.ofA, w.studentA, 2, "in_progress", "now() + interval '5 minutes'")
	w.overdue = w.attempt(t, w.ofA, w.user(t, "student"), 1, "in_progress", "now() - interval '1 minute'")
	w.paperB = w.attempt(t, w.byAdmin, w.studentB, 1, "submitted", "now() + interval '5 minutes'")
	w.paperLoose = w.attempt(t, w.byAdmin, w.loose, 1, "submitted", "now() + interval '5 minutes'")
	return w
}

func (w *paperWorld) snapshot(t *testing.T, attempt string) string {
	t.Helper()
	return w.id(t, `SELECT concat_ws('|', status, deadline_at, flagged, coalesce(teacher_note, ''), coalesce(void_reason, ''),
		(SELECT string_agg(coalesce(manual_score::text, '-'), ',') FROM app.attempt_answers WHERE attempt_id = a.id),
		(SELECT count(*) FROM app.audit_log WHERE entity_id = a.id)) FROM app.attempts a WHERE a.id = $1`, attempt)
}

func TestAnotherTeachersPaperAnswersAsAMissingOne(t *testing.T) {
	w := newPaperWorld(t)
	ctx := context.Background()
	b := access.Scope{UserID: w.b}
	req := domain.Request{ActorID: w.b}
	note := "ghi chú của B"
	for label, ids := range map[string][2]string{"A's": {w.paperA, w.live}, "a missing": {uuid.NewString(), uuid.NewString()}} {
		paper, live := ids[0], ids[1]
		var before, liveBefore string
		if label == "A's" {
			before, liveBefore = w.snapshot(t, paper), w.snapshot(t, live)
		}
		if _, err := w.reviews.Get(ctx, b, paper); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Errorf("B opening %s paper: %v", label, err)
		}
		if _, err := w.timelines.Timeline(ctx, b, paper); !errors.Is(err, domain.ErrTimelineNotFound) {
			t.Errorf("B reading %s timeline: %v", label, err)
		}
		if err := w.reviews.SetNote(ctx, b, paper, &note); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Errorf("B noting %s paper: %v", label, err)
		}
		if _, err := w.reviews.Grade(ctx, b, paper, w.b, []domain.GradeItem{{QuestionID: w.essay, Points: 5}}); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Errorf("B grading %s paper: %v", label, err)
		}
		if _, err := w.reviews.Finish(ctx, b, paper); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Errorf("B finishing %s paper: %v", label, err)
		}
		if _, err := w.store.Extend(ctx, req, live, 10, "thêm giờ", time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B extending %s attempt: %v", label, err)
		}
		if _, err := w.store.Flag(ctx, req, paper, true, "", time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B flagging %s attempt: %v", label, err)
		}
		if _, err := w.store.Void(ctx, req, paper, "huỷ", time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B voiding %s attempt: %v", label, err)
		}
		if _, err := w.store.Reset(ctx, req, live, "làm lại", time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B resetting %s attempt: %v", label, err)
		}
		if label == "A's" && (w.snapshot(t, paper) != before || w.snapshot(t, live) != liveBefore) {
			t.Errorf("B's refused writes changed A's attempts")
		}
	}
	for label, assignment := range map[string]string{"A's": w.ofA, "a missing": uuid.NewString()} {
		if _, err := w.store.Monitor(ctx, b, assignment, time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B monitoring %s assignment: %v", label, err)
		}
		if _, err := w.reviews.AnswersForQuestion(ctx, b, assignment, w.essay); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Errorf("B reading %s answers by question: %v", label, err)
		}
		if due, err := w.store.DueAttempts(ctx, b, assignment, time.Now()); err != nil || len(due) != 0 {
			t.Errorf("B's overdue sweep of %s assignment finds %v (%v)", label, due, err)
		}
	}
	if due, err := w.store.DueAttempts(ctx, access.Scope{UserID: w.a}, w.ofA, time.Now()); err != nil || !slices.Equal(due, []string{w.overdue}) {
		t.Errorf("A's overdue sweep finds %v (%v), want A's overdue attempt", due, err)
	}
}

func TestTheOwnerAndScopeAllReachEveryPaper(t *testing.T) {
	w := newPaperWorld(t)
	ctx := context.Background()
	for name, who := range map[string]access.Scope{"the creator": {UserID: w.a}, "scope.all": {UserID: w.admin, All: true}} {
		t.Run(name, func(t *testing.T) {
			req := domain.Request{ActorID: who.UserID, All: who.All}
			if _, err := w.reviews.Get(ctx, who, w.paperA); err != nil {
				t.Errorf("opening: %v", err)
			}
			if _, err := w.timelines.Timeline(ctx, who, w.paperA); err != nil {
				t.Errorf("reading the timeline: %v", err)
			}
			note := "ghi chú " + name
			if err := w.reviews.SetNote(ctx, who, w.paperA, &note); err != nil {
				t.Errorf("noting: %v", err)
			}
			if _, err := w.reviews.Grade(ctx, who, w.paperA, who.UserID, []domain.GradeItem{{QuestionID: w.essay, Points: 4}}); err != nil {
				t.Errorf("grading: %v", err)
			}
			if _, err := w.reviews.Finish(ctx, who, w.paperA); err != nil {
				t.Errorf("finishing: %v", err)
			}
			if _, err := w.store.Flag(ctx, req, w.paperA, name == "scope.all", "", time.Now()); err != nil {
				t.Errorf("flagging: %v", err)
			}
			if _, err := w.store.Extend(ctx, req, w.live, 5, "thêm giờ", time.Now()); err != nil {
				t.Errorf("extending: %v", err)
			}
			if _, err := w.store.Monitor(ctx, who, w.ofA, time.Now()); err != nil {
				t.Errorf("monitoring: %v", err)
			}
		})
	}
	req := domain.Request{ActorID: w.admin, All: true}
	if _, err := w.store.Void(ctx, req, w.paperA, "huỷ", time.Now()); err != nil {
		t.Errorf("scope.all voiding: %v", err)
	}
	if _, err := w.store.Reset(ctx, domain.Request{ActorID: w.a}, w.live, "làm lại", time.Now()); err != nil {
		t.Errorf("the creator resetting: %v", err)
	}
}

func TestTheApplicationCarriesTheScopeToTheStore(t *testing.T) {
	w := newPaperWorld(t)
	ctx := context.Background()
	app := application.New(w.timelines, w.reviews, w.store)
	b, a := access.Scope{UserID: w.b}, access.Scope{UserID: w.a}
	note := "ghi chú"
	grade := []domain.GradeItem{{QuestionID: w.essay, Points: 3}}
	paperBefore, overdueBefore := w.snapshot(t, w.paperA), w.snapshot(t, w.overdue)

	if _, err := app.Queries.Review.Handle(ctx, query.Review{AttemptID: w.paperA, Scope: b}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("B's review: %v", err)
	}
	if _, err := app.Queries.Timeline.Handle(ctx, query.Timeline{AttemptID: w.paperA, Scope: b}); !errors.Is(err, domain.ErrTimelineNotFound) {
		t.Errorf("B's timeline: %v", err)
	}
	if _, err := app.Queries.Monitor.Handle(ctx, query.Monitor{AssignmentID: w.ofA, Scope: b}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B's monitor: %v", err)
	}
	if _, err := app.Queries.AnswersForQuestion.Handle(ctx, query.AnswersForQuestion{AssignmentID: w.ofA, QuestionID: w.essay, Scope: b}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("B's answers by question: %v", err)
	}
	if _, err := app.Commands.SetNote.Handle(ctx, command.SetNote{AttemptID: w.paperA, Note: &note, Scope: b}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("B's note: %v", err)
	}
	if _, err := app.Commands.Grade.Handle(ctx, command.Grade{AttemptID: w.paperA, GraderID: w.b, Items: grade, Scope: b}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("B's grade: %v", err)
	}
	if _, err := app.Commands.Finish.Handle(ctx, command.Finish{AttemptID: w.paperA, Scope: b}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("B's finish: %v", err)
	}
	if _, err := app.Commands.ExpireDue.Handle(ctx, command.ExpireDue{AssignmentID: w.ofA, Scope: b}); err != nil {
		t.Errorf("B's overdue sweep: %v", err)
	}
	if w.snapshot(t, w.paperA) != paperBefore || w.snapshot(t, w.overdue) != overdueBefore {
		t.Fatal("B's refused calls changed A's attempts")
	}

	if _, err := app.Queries.Review.Handle(ctx, query.Review{AttemptID: w.paperA, Scope: a}); err != nil {
		t.Errorf("A's review: %v", err)
	}
	if _, err := app.Queries.Timeline.Handle(ctx, query.Timeline{AttemptID: w.paperA, Scope: a}); err != nil {
		t.Errorf("A's timeline: %v", err)
	}
	if _, err := app.Queries.AnswersForQuestion.Handle(ctx, query.AnswersForQuestion{AssignmentID: w.ofA, QuestionID: w.essay, Scope: a}); err != nil {
		t.Errorf("A's answers by question: %v", err)
	}
	if _, err := app.Commands.SetNote.Handle(ctx, command.SetNote{AttemptID: w.paperA, Note: &note, Scope: a}); err != nil {
		t.Errorf("A's note: %v", err)
	}
	if _, err := app.Commands.Grade.Handle(ctx, command.Grade{AttemptID: w.paperA, GraderID: w.a, Items: grade, Scope: a}); err != nil {
		t.Errorf("A's grade: %v", err)
	}
	if _, err := app.Commands.Finish.Handle(ctx, command.Finish{AttemptID: w.paperA, Scope: a}); err != nil {
		t.Errorf("A's finish: %v", err)
	}
	if _, err := app.Commands.ExpireDue.Handle(ctx, command.ExpireDue{AssignmentID: w.ofA, Scope: a}); err != nil {
		t.Errorf("A's overdue sweep: %v", err)
	}
	if w.snapshot(t, w.overdue) == overdueBefore {
		t.Error("A's overdue sweep left A's overdue attempt open")
	}
	if _, err := app.Queries.Monitor.Handle(ctx, query.Monitor{AssignmentID: w.ofA, Scope: a}); err != nil {
		t.Errorf("A's monitor: %v", err)
	}
}

func (w *paperWorld) monitored(t *testing.T, scope access.Scope, assignment string) []string {
	t.Helper()
	monitor, err := w.store.Monitor(context.Background(), scope, assignment, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, row := range monitor.Rows {
		ids = append(ids, row.StudentID)
	}
	slices.Sort(ids)
	return ids
}

func (w *paperWorld) byQuestion(t *testing.T, scope access.Scope, assignment string) []string {
	t.Helper()
	byQ, err := w.reviews.AnswersForQuestion(context.Background(), scope, assignment, w.essay)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, item := range byQ.Items {
		ids = append(ids, item.AttemptID)
	}
	slices.Sort(ids)
	return ids
}

func (w *paperWorld) listed(t *testing.T, scope access.Scope) []string {
	t.Helper()
	found, _, err := dashboardrepo.NewPostgres(db.NewContext(w.tx)).List(context.Background(), dashboarddomain.ListQuery{Scope: scope, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	mine := map[string]bool{w.paperA: true, w.live: true, w.overdue: true, w.paperB: true, w.paperLoose: true}
	var ids []string
	for _, r := range found {
		if mine[r.ID] {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func sorted(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestListsOfPapersShowOnlyTheStudentsTheReaderReaches(t *testing.T) {
	w := newPaperWorld(t)
	b := access.Scope{UserID: w.b}
	admin := access.Scope{UserID: w.admin, All: true}
	if got := w.monitored(t, b, w.byAdmin); !slices.Equal(got, []string{w.studentB}) {
		t.Errorf("B's monitor of the Admin's assignment lists %v, want only B's student", got)
	}
	if got := w.monitored(t, admin, w.byAdmin); !slices.Equal(got, sorted(w.studentB, w.loose)) {
		t.Errorf("scope.all's monitor lists %v, want both", got)
	}
	if got := w.byQuestion(t, b, w.byAdmin); !slices.Equal(got, []string{w.paperB}) {
		t.Errorf("B's answers by question list %v, want only B's student's paper", got)
	}
	if got := w.byQuestion(t, admin, w.byAdmin); !slices.Equal(got, sorted(w.paperB, w.paperLoose)) {
		t.Errorf("scope.all's answers by question list %v, want both", got)
	}
	if got := w.listed(t, b); !slices.Equal(got, []string{w.paperB}) {
		t.Errorf("B's attempt list shows %v, want only B's student's paper", got)
	}
	if got := w.listed(t, access.Scope{}); len(got) != 0 {
		t.Errorf("the zero scope's attempt list shows %v", got)
	}

	w.id(t, `DELETE FROM app.class_members WHERE class_id = $1 AND user_id = $2 RETURNING user_id::text`, w.classA, w.studentA)
	a := access.Scope{UserID: w.a}
	if got := w.byQuestion(t, a, w.ofA); !slices.Equal(got, []string{w.paperA}) {
		t.Errorf("A's answers by question after the student left list %v, want the paper still there", got)
	}
	if got := w.listed(t, a); !slices.Equal(got, sorted(w.paperA, w.live, w.overdue)) {
		t.Errorf("A's attempt list after the student left shows %v, want every attempt on A's assignment", got)
	}
}

func TestAMixedAssignmentShowsOnlyWhatTheReaderReaches(t *testing.T) {
	w := newPaperWorld(t)
	b := access.Scope{UserID: w.b}
	w.id(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2) RETURNING class_id::text`, w.byAdmin, w.classA)
	if got := w.monitored(t, b, w.byAdmin); !slices.Equal(got, []string{w.studentB}) {
		t.Errorf("B's monitor lists %v, want only B's student and none of A's class", got)
	}
	if got := w.monitored(t, access.Scope{UserID: w.admin, All: true}, w.byAdmin); !slices.Equal(got, sorted(w.studentA, w.studentB, w.loose)) {
		t.Errorf("scope.all's monitor lists %v, want all three", got)
	}

	elsewhere := w.assignment(t, w.admin, w.classA, &w.studentB)
	paper := w.attempt(t, elsewhere, w.studentB, 1, "submitted", "now() + interval '5 minutes'")
	found, _, err := dashboardrepo.NewPostgres(db.NewContext(w.tx)).List(context.Background(), dashboarddomain.ListQuery{Scope: b, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(found, func(r dashboarddomain.Recent) bool { return r.ID == paper }) {
		t.Error("B's attempt list shows B's student's paper on an assignment B does not reach")
	}
	if !slices.ContainsFunc(found, func(r dashboarddomain.Recent) bool { return r.ID == w.paperB }) {
		t.Error("B's attempt list lost B's student's paper on the assignment B reaches")
	}
}
