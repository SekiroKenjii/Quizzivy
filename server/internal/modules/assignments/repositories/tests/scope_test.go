//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type reachWorld struct {
	tx                   pgx.Tx
	store                *repositories.Postgres
	a, b, admin          string
	classA, classB       string
	studentA, studentB   string
	versionA, versionB   string
	mineA, mineB, shared string
}

func (w *reachWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *reachWorld) user(t *testing.T, builtin string, creator *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id, created_by) VALUES ($1, 'Phạm vi', (SELECT id FROM app.roles WHERE builtin_key = $2), $3) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin, creator)
}

func (w *reachWorld) version(t *testing.T, owner string) string {
	t.Helper()
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, owner)
	return w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, test, owner)
}

func input(version string, classes, students []string) domain.WriteInput {
	now := time.Now()
	return domain.WriteInput{
		TestVersionID: version, ClassIDs: classes, StudentIDs: students,
		OpensAt: now.Add(-time.Hour), ClosesAt: now.Add(time.Hour), DurationMin: 45, MaxAttempts: 1,
		Integrity: domain.Integrity{OnLimitExceeded: "flag", MinAwayMs: 3000}, Now: now,
	}
}

func as(actor string, all bool) domain.Request {
	return domain.Request{ActorID: actor, All: all}
}

func (w *reachWorld) create(t *testing.T, req domain.Request, in domain.WriteInput) string {
	t.Helper()
	created, err := w.store.Create(context.Background(), req, in)
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	return created.ID
}

func newReachWorld(t *testing.T) *reachWorld {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	w := &reachWorld{tx: tx, store: repositories.NewPostgres(db.NewContext(tx))}
	w.a, w.b, w.admin = w.user(t, "teacher", nil), w.user(t, "teacher", nil), w.user(t, "admin", nil)
	w.classA = w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp A', $1) RETURNING id::text`, w.a)
	w.classB = w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp B', $1) RETURNING id::text`, w.b)
	w.studentA, w.studentB = w.user(t, "student", &w.a), w.user(t, "student", &w.b)
	for class, student := range map[string]string{w.classA: w.studentA, w.classB: w.studentB} {
		w.id(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2) RETURNING user_id::text`, class, student)
	}
	w.versionA, w.versionB = w.version(t, w.a), w.version(t, w.b)
	w.mineA = w.create(t, as(w.a, false), input(w.versionA, []string{w.classA}, nil))
	w.mineB = w.create(t, as(w.b, false), input(w.versionB, []string{w.classB}, nil))
	w.shared = w.create(t, as(w.admin, true), input(w.versionA, []string{w.classA, w.classB}, []string{w.studentB}))
	return w
}

func (w *reachWorld) listed(t *testing.T, scope access.Scope, classID *string) []string {
	t.Helper()
	found, page, err := w.store.List(context.Background(), domain.ListInput{Scope: scope, ClassID: classID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, a := range found {
		if a.ID == w.mineA || a.ID == w.mineB || a.ID == w.shared {
			ids = append(ids, a.ID)
		}
	}
	slices.Sort(ids)
	if !scope.All && page.Total != len(ids) {
		t.Errorf("the list totals %d for %d rows", page.Total, len(ids))
	}
	return ids
}

func sortedIDs(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestEachTeacherListsTheAssignmentsTheyReach(t *testing.T) {
	w := newReachWorld(t)
	for name, c := range map[string]struct {
		scope access.Scope
		want  []string
	}{
		"A":              {access.Scope{UserID: w.a}, sortedIDs(w.mineA, w.shared)},
		"B":              {access.Scope{UserID: w.b}, sortedIDs(w.mineB, w.shared)},
		"scope.all":      {access.Scope{UserID: w.admin, All: true}, sortedIDs(w.mineA, w.mineB, w.shared)},
		"the zero scope": {access.Scope{}, []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := w.listed(t, c.scope, nil); !slices.Equal(got, c.want) {
				t.Errorf("lists %v, want %v", got, c.want)
			}
			if !c.scope.All {
				facets, err := w.store.Facets(context.Background(), domain.ListInput{Scope: c.scope})
				if err != nil || facets.All != len(c.want) {
					t.Errorf("facets count %d (%v), want %d", facets.All, err, len(c.want))
				}
			}
		})
	}
	b := access.Scope{UserID: w.b}
	for label, classID := range map[string]string{"A's class": w.classA, "a missing class": uuid.NewString()} {
		if got := w.listed(t, b, &classID); len(got) != 0 {
			t.Errorf("B filtering by %s lists %v", label, got)
		}
		facets, err := w.store.Facets(context.Background(), domain.ListInput{Scope: b, ClassID: &classID})
		if err != nil || facets.All != 0 {
			t.Errorf("B's facets for %s: %d (%v)", label, facets.All, err)
		}
	}
}

func TestAnAssignmentNamesOnlyTheTargetsTheReaderReaches(t *testing.T) {
	w := newReachWorld(t)
	for name, c := range map[string]struct {
		scope    access.Scope
		classes  []string
		students []string
		total    int
	}{
		"A":         {access.Scope{UserID: w.a}, []string{w.classA}, []string{}, 1},
		"B":         {access.Scope{UserID: w.b}, []string{w.classB}, []string{w.studentB}, 1},
		"scope.all": {access.Scope{UserID: w.admin, All: true}, sortedIDs(w.classA, w.classB), []string{w.studentB}, 2},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := w.store.Get(context.Background(), c.scope, w.shared)
			if err != nil {
				t.Fatal(err)
			}
			listed, _, err := w.store.List(context.Background(), domain.ListInput{Scope: c.scope, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			i := slices.IndexFunc(listed, func(a domain.Assignment) bool { return a.ID == w.shared })
			if i < 0 {
				t.Fatal("the list omits the assignment")
			}
			for source, a := range map[string]domain.Assignment{"get": got, "list": listed[i]} {
				classes, students := []string{}, []string{}
				for _, class := range a.Classes {
					classes = append(classes, class.ID)
				}
				for _, student := range a.Students {
					students = append(students, student.ID)
				}
				slices.Sort(classes)
				if !slices.Equal(classes, c.classes) || !slices.Equal(students, c.students) || a.TargetCount != c.total {
					t.Errorf("%s names classes %v, students %v and %d targets, want %v, %v and %d", source, classes, students, a.TargetCount, c.classes, c.students, c.total)
				}
			}
		})
	}
}

func (w *reachWorld) state(t *testing.T, id string) string {
	t.Helper()
	return w.id(t, `SELECT concat_ws('|', a.test_version_id, a.closes_at, coalesce(a.closed_at::text, ''), a.duration_minutes,
		(SELECT string_agg(class_id::text, ',' ORDER BY class_id) FROM app.assignment_classes WHERE assignment_id = a.id),
		(SELECT string_agg(user_id::text, ',' ORDER BY user_id) FROM app.assignment_students WHERE assignment_id = a.id))
		  FROM app.assignments a WHERE a.id = $1`, id)
}

func TestAnotherTeachersAssignmentAnswersAsAMissingOne(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	b := as(w.b, false)
	app := application.New(w.store)
	w.id(t, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1 RETURNING id::text`, w.mineA)
	before := w.state(t, w.mineA)
	for label, id := range map[string]string{"A's": w.mineA, "a missing": uuid.NewString()} {
		b.ID = id
		if _, err := w.store.Get(ctx, b.Scope(), id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B opening %s assignment: %v", label, err)
		}
		if _, err := app.Queries.Get.Handle(ctx, query.Get{ID: id, Scope: b.Scope()}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B opening %s assignment through the application: %v", label, err)
		}
		if _, err := w.store.Update(ctx, b, input(w.versionB, []string{w.classB}, nil)); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B updating %s assignment: %v", label, err)
		}
		if _, err := w.store.Reopen(ctx, b, time.Now().Add(time.Hour), "Gia hạn", time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B reopening %s assignment: %v", label, err)
		}
		if err := w.store.Delete(ctx, b, time.Now()); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B deleting %s assignment: %v", label, err)
		}
	}
	if after := w.state(t, w.mineA); after != before {
		t.Errorf("B's refused writes changed A's assignment from %s to %s", before, after)
	}
	if _, err := app.Queries.Get.Handle(ctx, query.Get{ID: w.mineA, Scope: access.Scope{UserID: w.a}}); err != nil {
		t.Errorf("A opening A's assignment through the application: %v", err)
	}
}

func TestAWriteNamesOnlyWhatTheWriterReaches(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	b := as(w.b, false)
	staffOfB := w.user(t, "teacher", &w.b)
	disabledOfB := w.user(t, "student", &w.b)
	w.id(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1 RETURNING id::text`, disabledOfB)
	for label, c := range map[string]struct {
		version string
		classes []string
		student []string
	}{
		"A's version":            {w.versionA, []string{w.classB}, nil},
		"a missing version":      {uuid.NewString(), []string{w.classB}, nil},
		"A's class":              {w.versionB, []string{w.classA}, nil},
		"a missing class":        {w.versionB, []string{uuid.NewString()}, nil},
		"A's student":            {w.versionB, []string{w.classB}, []string{w.studentA}},
		"a missing student":      {w.versionB, []string{w.classB}, []string{uuid.NewString()}},
		"a staff account B made": {w.versionB, []string{w.classB}, []string{staffOfB}},
		"B's disabled student":   {w.versionB, []string{w.classB}, []string{disabledOfB}},
	} {
		_, err := w.store.Create(ctx, b, input(c.version, c.classes, c.student))
		var invalid *domain.ValidationError
		if !errors.Is(err, domain.ErrTestNotPublished) && !errors.As(err, &invalid) {
			t.Errorf("B assigning %s: %v, want the answer a missing id gets", label, err)
		}
	}
	for pair, ids := range map[string][2]string{"version": {w.versionA, uuid.NewString()}, "class": {w.classA, uuid.NewString()}, "student": {w.studentA, uuid.NewString()}} {
		answers := [2]string{}
		for i, id := range ids {
			in := input(w.versionB, []string{w.classB}, nil)
			switch pair {
			case "version":
				in.TestVersionID = id
			case "class":
				in.ClassIDs = []string{id}
			case "student":
				in.StudentIDs = []string{id}
			}
			_, err := w.store.Create(ctx, b, in)
			answers[i] = shape(err, id)
		}
		if answers[0] != answers[1] {
			t.Errorf("B naming A's %s answered %q, a missing one %q", pair, answers[0], answers[1])
		}
	}
	repoint := as(w.b, false)
	repoint.ID = w.mineB
	before := w.state(t, w.mineB)
	for label, version := range map[string]string{"A's version": w.versionA, "a missing version": uuid.NewString()} {
		if _, err := w.store.Update(ctx, repoint, input(version, []string{w.classB}, nil)); !errors.Is(err, domain.ErrTestNotPublished) {
			t.Errorf("B re-pointing B's assignment to %s: %v, want the answer a missing version gets", label, err)
		}
	}
	if after := w.state(t, w.mineB); after != before {
		t.Errorf("B's refused re-pointing changed the assignment from %s to %s", before, after)
	}
	if _, err := w.store.Create(ctx, b, input(w.versionB, []string{w.classB}, []string{w.studentB})); err != nil {
		t.Errorf("B assigning B's own version, class and student: %v", err)
	}
	if _, err := w.store.Create(ctx, as(w.admin, true), input(w.versionA, []string{w.classA}, []string{w.studentB})); err != nil {
		t.Errorf("scope.all assigning across teachers: %v", err)
	}
}

func shape(err error, id string) string {
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) {
		if err == nil {
			return "nil"
		}
		return err.Error()
	}
	out := ""
	for _, f := range invalid.Fields {
		out += f.Field + ":" + strings.ReplaceAll(f.Message, id, "<id>") + ";"
	}
	return out
}

func TestAClassArmTeacherCanCloseAnAdminsAssignmentAndKeepsHiddenTargets(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	b := as(w.b, false)
	b.ID = w.shared
	w.id(t, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2) RETURNING user_id::text`, w.shared, w.studentA)
	in := input(w.versionA, []string{w.classB}, []string{w.studentB})
	in.CloseNow = true
	saved, err := w.store.Update(ctx, b, in)
	if err != nil {
		t.Fatalf("B closing the Admin's assignment on B's class: %v", err)
	}
	if saved.ClosedAt == nil {
		t.Error("the assignment is not closed")
	}
	rows, err := w.tx.Query(ctx, `SELECT class_id::text FROM app.assignment_classes WHERE assignment_id = $1 ORDER BY class_id`, w.shared)
	if err != nil {
		t.Fatal(err)
	}
	classes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(classes, sortedIDs(w.classA, w.classB)) {
		t.Errorf("B's save left classes %v, want A's class kept beside B's", classes)
	}
	rows, err = w.tx.Query(ctx, `SELECT user_id::text FROM app.assignment_students WHERE assignment_id = $1 ORDER BY user_id`, w.shared)
	if err != nil {
		t.Fatal(err)
	}
	students, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(students, sortedIDs(w.studentA, w.studentB)) {
		t.Errorf("B's save left students %v, want A's student kept beside B's", students)
	}
	in = input(w.versionA, []string{w.classA, w.classB}, nil)
	var invalid *domain.ValidationError
	if _, err := w.store.Update(ctx, b, in); !errors.As(err, &invalid) {
		t.Errorf("B adding A's class: %v, want the missing-class answer", err)
	}
}

func TestScopeAllWritesAnotherTeachersAssignment(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	admin := as(w.admin, true)
	w.id(t, `UPDATE app.assignments SET published_at = now() - interval '2 hours', closed_at = now() - interval '1 minute' WHERE id = $1 RETURNING id::text`, w.mineA)
	admin.ID = w.mineA
	reopened, err := w.store.Reopen(ctx, admin, time.Now().Add(time.Hour), "Gia hạn", time.Now())
	if err != nil {
		t.Fatalf("scope.all reopening A's assignment: %v", err)
	}
	if reopened.ClosedAt != nil {
		t.Error("the reopened assignment is still closed")
	}
	w.id(t, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1 RETURNING id::text`, w.mineB)
	admin.ID = w.mineB
	if err := w.store.Delete(ctx, admin, time.Now()); err != nil {
		t.Fatalf("scope.all deleting B's closed assignment: %v", err)
	}
	if _, err := w.store.Get(ctx, access.Scope{All: true}, w.mineB); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B's assignment is still there: %v", err)
	}
}
