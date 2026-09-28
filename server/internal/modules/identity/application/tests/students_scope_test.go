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

	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type rosterWorld struct {
	tx                        pgx.Tx
	app                       *application.Application
	a, b, admin               string
	classA, classB            string
	member, created, targeted string
	shared, loose             string
	marker                    string
	version                   string
}

func (w *rosterWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *rosterWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *rosterWorld) user(t *testing.T, builtin, name string, creator *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id, created_by) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = $3), $4) RETURNING id::text`,
		uuid.NewString()+"@example.test", name+" "+w.marker, builtin, creator)
}

func (w *rosterWorld) class(t *testing.T, teacher string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp "+uuid.NewString()[:8], teacher)
}

func (w *rosterWorld) enrol(t *testing.T, class, student string) {
	t.Helper()
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2)`, class, student)
}

func (w *rosterWorld) assignment(t *testing.T, author string, class *string, student *string) string {
	t.Helper()
	id := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		SELECT v.test_id, v.id, now() - interval '30 days', now() + interval '2 hours', 45, $2, now() FROM app.test_versions v WHERE v.id = $1 RETURNING id::text`,
		w.version, author)
	if class != nil {
		w.exec(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2)`, id, *class)
	}
	if student != nil {
		w.exec(t, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2)`, id, *student)
	}
	return id
}

func (w *rosterWorld) graded(t *testing.T, assignment, student string, startedAgo time.Duration) {
	t.Helper()
	w.exec(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		        started_at, deadline_at, submitted_at, graded_at, score_earned, score_total, flagged)
		VALUES ($1, $2, $3, 1, 'graded', gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - $4 * interval '1 second', now() - $4 * interval '1 second' + interval '45 minutes',
		        now() - $4 * interval '1 second' + interval '30 minutes', now(), 5, 10, true)`,
		assignment, w.version, student, startedAgo.Seconds())
}

func newRosterWorld(t *testing.T) *rosterWorld {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	dbx := db.NewContext(tx)
	w := &rosterWorld{tx: tx, app: application.New(nil, nil, 0, repositories.NewStudents(dbx), attemptsrepo.NewStudentStats(dbx)), marker: "Pham Vi " + uuid.NewString()[:8]}
	w.a, w.b, w.admin = w.user(t, "teacher", "Giáo viên A", nil), w.user(t, "teacher", "Giáo viên B", nil), w.user(t, "admin", "Quản trị", nil)
	w.classA, w.classB = w.class(t, w.a), w.class(t, w.b)
	w.member = w.user(t, "student", "Thành viên", nil)
	w.enrol(t, w.classA, w.member)
	w.created = w.user(t, "student", "Được tạo", &w.a)
	w.targeted = w.user(t, "student", "Được giao", nil)
	w.shared = w.user(t, "student", "Dùng chung", nil)
	w.enrol(t, w.classA, w.shared)
	w.enrol(t, w.classB, w.shared)
	w.loose = w.user(t, "student", "Tự do", nil)
	testID := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề phạm vi', 'published', 1, $1, $1) RETURNING id::text`, w.admin)
	w.version = w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, testID, w.admin)
	w.assignment(t, w.a, nil, &w.targeted)
	onA := w.assignment(t, w.a, &w.classA, nil)
	onB := w.assignment(t, w.b, &w.classB, nil)
	byAdminOnB := w.assignment(t, w.admin, &w.classB, nil)
	w.graded(t, onA, w.shared, time.Hour)
	w.graded(t, onB, w.shared, 20*24*time.Hour)
	w.graded(t, byAdminOnB, w.shared, 20*24*time.Hour)
	w.graded(t, w.assignment(t, w.b, nil, &w.shared), w.shared, 20*24*time.Hour)
	return w
}

func (w *rosterWorld) list(t *testing.T, scope access.Scope, classID string) map[string]domain.Student {
	t.Helper()
	found, err := w.app.Queries.ListStudents.Handle(context.Background(), query.ListStudents{Query: domain.StudentQuery{Query: w.marker, ClassID: classID, Limit: 100, Scope: scope}})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]domain.Student{}
	for _, s := range found.Items {
		out[s.ID] = s
	}
	if found.Page.Total != len(out) {
		t.Errorf("the list totals %d for %d rows", found.Page.Total, len(out))
	}
	return out
}

func (w *rosterWorld) facets(t *testing.T, scope access.Scope, classID string) domain.StudentFacets {
	t.Helper()
	f, err := w.app.Queries.StudentFacets.Handle(context.Background(), query.StudentFacets{Query: domain.StudentQuery{Query: w.marker, ClassID: classID, Scope: scope}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func keys(m map[string]domain.Student) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func sortedIDs(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func classIDs(s domain.Student) []string {
	out := make([]string, 0, len(s.Classes))
	for _, c := range s.Classes {
		out = append(out, c.ID)
	}
	slices.Sort(out)
	return out
}

func TestEachTeacherListsOnlyTheStudentsTheyReach(t *testing.T) {
	w := newRosterWorld(t)
	for name, c := range map[string]struct {
		scope   access.Scope
		want    []string
		classes []string
		work    int
		active  int
	}{
		"A":              {access.Scope{UserID: w.a}, sortedIDs(w.member, w.created, w.targeted, w.shared), sortedIDs(w.classA), 1, 1},
		"B":              {access.Scope{UserID: w.b}, sortedIDs(w.shared), sortedIDs(w.classB), 3, 0},
		"scope.all":      {access.Scope{UserID: w.admin, All: true}, sortedIDs(w.member, w.created, w.targeted, w.shared, w.loose), sortedIDs(w.classA, w.classB), 4, 1},
		"the zero scope": {access.Scope{}, []string{}, nil, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			listed := w.list(t, c.scope, "")
			if got := keys(listed); !slices.Equal(got, c.want) {
				t.Errorf("lists %v, want %v", got, c.want)
			}
			if f := w.facets(t, c.scope, ""); f.Total != len(c.want) || f.ActiveLast7Days != c.active {
				t.Errorf("facets %+v, want %d students and %d active", f, len(c.want), c.active)
			}
			shared, ok := listed[w.shared]
			if !ok {
				return
			}
			if got := classIDs(shared); !slices.Equal(got, c.classes) {
				t.Errorf("the shared student is in %v, want only %v", got, c.classes)
			}
			if shared.Stats.SubmittedCount != c.work || shared.Stats.FlaggedCount != c.work {
				t.Errorf("the shared student's figures count %d submitted and %d flagged, want %d of each", shared.Stats.SubmittedCount, shared.Stats.FlaggedCount, c.work)
			}
			got, err := w.app.Queries.GetStudent.Handle(context.Background(), query.GetStudent{ID: w.shared, Scope: c.scope})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(classIDs(got), c.classes) || got.Stats.SubmittedCount != c.work {
				t.Errorf("opening the shared student shows %v and %d submitted, want %v and %d", classIDs(got), got.Stats.SubmittedCount, c.classes, c.work)
			}
		})
	}
}

func TestAnotherTeachersClassFiltersNobody(t *testing.T) {
	w := newRosterWorld(t)
	b := access.Scope{UserID: w.b}
	for label, classID := range map[string]string{"A's class": w.classA, "a missing class": uuid.NewString()} {
		if listed := w.list(t, b, classID); len(listed) != 0 {
			t.Errorf("B filtering by %s lists %v", label, keys(listed))
		}
		if f := w.facets(t, b, classID); f.Total != 0 || f.ActiveLast7Days != 0 {
			t.Errorf("B's facets for %s: %+v", label, f)
		}
	}
	if listed := w.list(t, b, w.classB); !slices.Equal(keys(listed), []string{w.shared}) {
		t.Errorf("B filtering by B's own class lists %v, want the shared student", keys(listed))
	}
}

func (w *rosterWorld) snapshot(t *testing.T, id string) string {
	t.Helper()
	return w.id(t, `SELECT concat_ws('|', email, full_name, coalesce(password_hash, ''), coalesce(disabled_at::text, ''), session_epoch::text) FROM app.users WHERE id = $1`, id)
}

func TestAnotherTeachersStudentAnswersAsAMissingOne(t *testing.T) {
	w := newRosterWorld(t)
	ctx := context.Background()
	b := domain.WriteRequest{ActorID: w.b}
	name := "Đổi tên"
	disabled := true
	for label, id := range map[string]string{"a member of A's class": w.member, "A's own": w.created, "A's target": w.targeted, "a missing": uuid.NewString()} {
		var before string
		if label != "a missing" {
			before = w.snapshot(t, id)
		}
		if _, err := w.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: id, Scope: b.Scope()}); !errors.Is(err, domain.ErrStudentNotFound) {
			t.Errorf("B opening %s student: %v", label, err)
		}
		if _, err := w.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: b, Input: domain.StudentPatch{ID: id, FullName: &name, Disabled: &disabled}}); !errors.Is(err, domain.ErrStudentNotFound) {
			t.Errorf("B updating %s student: %v", label, err)
		}
		if _, err := w.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: b, ID: id}); !errors.Is(err, domain.ErrStudentNotFound) {
			t.Errorf("B resetting %s student's password: %v", label, err)
		}
		if _, err := w.app.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: b, ID: id}); !errors.Is(err, domain.ErrStudentNotFound) {
			t.Errorf("B deleting %s student: %v", label, err)
		}
		if label != "a missing" && w.snapshot(t, id) != before {
			t.Errorf("B's refused writes changed %s student", label)
		}
	}
	if _, err := w.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: domain.WriteRequest{ActorID: w.a}, Input: domain.StudentPatch{ID: w.targeted, FullName: &name}}); err != nil {
		t.Errorf("A updating A's target: %v", err)
	}
}

func TestEachReachRuleCountsOnlyWhileItHolds(t *testing.T) {
	w := newRosterWorld(t)
	a := access.Scope{UserID: w.a}
	for name, c := range map[string]struct {
		id     string
		revoke string
	}{
		"membership of a class A teaches": {w.member, `DELETE FROM app.class_members WHERE user_id = $1`},
		"an account A created":            {w.created, `UPDATE app.users SET created_by = NULL WHERE id = $1`},
		"a target of A's assignment":      {w.targeted, `DELETE FROM app.assignment_students WHERE user_id = $1`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := w.list(t, a, "")[c.id]; !ok {
				t.Fatal("A does not reach the student before the rule is revoked")
			}
			w.exec(t, c.revoke, c.id)
			if _, ok := w.list(t, a, "")[c.id]; ok {
				t.Error("A still reaches the student once the rule no longer holds")
			}
			if _, err := w.app.Queries.GetStudent.Handle(context.Background(), query.GetStudent{ID: c.id, Scope: a}); !errors.Is(err, domain.ErrStudentNotFound) {
				t.Errorf("A still opens the student: %v", err)
			}
		})
	}
}

func TestACreatedStudentEntersOnlyClassesTheCreatorTeaches(t *testing.T) {
	w := newRosterWorld(t)
	ctx := context.Background()
	create := func(req domain.WriteRequest, classes ...string) (domain.Student, string, error) {
		email := uuid.NewString() + "@example.test"
		result, err := w.app.Commands.CreateStudent.Handle(ctx, command.CreateStudent{Request: req, Input: domain.NewStudent{Email: email, FullName: "Mới " + w.marker, ClassIDs: classes}})
		return result.Student, email, err
	}
	b := domain.WriteRequest{ActorID: w.b}
	for label, classes := range map[string][]string{
		"A's class":                {w.classA},
		"B's class beside A's":     {w.classB, w.classA},
		"a missing class":          {uuid.NewString()},
		"B's class and a missing":  {w.classB, uuid.NewString()},
		"B's class named twice, A": {w.classB, w.classB, w.classA},
	} {
		_, email, err := create(b, classes...)
		if !errors.Is(err, domain.ErrClassNotFound) {
			t.Errorf("B creating into %s: %v, want the class not-found", label, err)
		}
		var exists bool
		if err := w.tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.users WHERE email = $1)`, email).Scan(&exists); err != nil || exists {
			t.Errorf("B's refused create into %s left an account (%v)", label, err)
		}
	}
	if _, _, err := create(domain.WriteRequest{ActorID: w.admin, All: true}, uuid.NewString()); !errors.Is(err, domain.ErrClassNotFound) {
		t.Errorf("scope.all creating into a missing class: %v, want the class not-found", err)
	}
	made, _, err := create(b, w.classB, w.classB)
	if err != nil {
		t.Fatalf("B creating into B's own class: %v", err)
	}
	var creator string
	if err := w.tx.QueryRow(ctx, `SELECT created_by::text FROM app.users WHERE id = $1`, made.ID).Scan(&creator); err != nil || creator != w.b {
		t.Errorf("B's new student was created by %s (%v)", creator, err)
	}
	if !slices.Equal(classIDs(made), []string{w.classB}) {
		t.Errorf("B's new student is in %v, want B's class", classIDs(made))
	}
	if _, _, err := create(domain.WriteRequest{ActorID: w.admin, All: true}, w.classA); err != nil {
		t.Errorf("scope.all creating into A's class: %v", err)
	}
}
