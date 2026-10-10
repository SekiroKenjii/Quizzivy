//go:build integration

package visibility_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/shared/visibility"
)

type world struct {
	t  *testing.T
	tx pgx.Tx

	creator, t1, t2, t3 string
	c1, c2, c3          string
	both, onlyOne       string
	onlyTwo, named      string
	madeByTwo, viaOther string
	elsewhere           string
	assignment          string
	students            map[string]string
}

func (w *world) id(sql string, args ...any) string {
	w.t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *world) user(builtin string, creator *string) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.users (email, full_name, role_id, created_by)
		VALUES ($1, 'Phạm vi', (SELECT id FROM app.roles WHERE builtin_key = $2), $3) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin, creator)
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) join(class, student string) {
	w.t.Helper()
	w.exec(`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2)`, class, student)
}

func (w *world) assign(creator string) string {
	w.t.Helper()
	test := w.id(`INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, creator)
	version := w.id(`INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, test, creator)
	return w.id(`INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		VALUES ($1, $2, now() - interval '1 hour', now() + interval '1 hour', 45, $3, now()) RETURNING id::text`, test, version, creator)
}

func newWorld(t *testing.T) *world {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("rolling the world back: %v", err)
		}
	})
	w := &world{t: t, tx: tx}

	w.creator, w.t1, w.t2, w.t3 = w.user("teacher", nil), w.user("teacher", nil), w.user("teacher", nil), w.user("teacher", nil)
	w.c1 = w.id(`INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp 1', $1) RETURNING id::text`, w.t1)
	w.c2 = w.id(`INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp 2', $1) RETURNING id::text`, w.t2)
	w.c3 = w.id(`INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp 3', $1) RETURNING id::text`, w.t3)

	w.both, w.onlyOne, w.onlyTwo = w.user("student", nil), w.user("student", nil), w.user("student", nil)
	w.named, w.madeByTwo, w.viaOther, w.elsewhere = w.user("student", nil), w.user("student", &w.t2), w.user("student", nil), w.user("student", nil)
	w.join(w.c1, w.both)
	w.join(w.c2, w.both)
	w.join(w.c1, w.onlyOne)
	w.join(w.c2, w.onlyTwo)
	w.join(w.c1, w.madeByTwo)
	w.join(w.c1, w.viaOther)
	w.join(w.c3, w.elsewhere)

	w.assignment = w.assign(w.creator)
	w.exec(`INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2), ($1, $3)`, w.assignment, w.c1, w.c2)
	w.exec(`INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2)`, w.assignment, w.named)

	other := w.assign(w.t2)
	w.exec(`INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2)`, other, w.viaOther)

	w.students = map[string]string{
		"in both target classes": w.both, "in the first only": w.onlyOne, "in the second only": w.onlyTwo,
		"named on the assignment": w.named, "created by the second teacher": w.madeByTwo,
		"named on the second teacher's other assignment": w.viaOther, "in a class the assignment does not target": w.elsewhere,
	}
	return w
}

func (w *world) readers(student string) []string {
	w.t.Helper()
	rows, err := w.tx.Query(context.Background(),
		`SELECT r.id::text FROM app.users r WHERE r.id IN `+visibility.PaperReaders("$1::uuid", "$2::uuid")+` ORDER BY r.id`,
		w.assignment, student)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			w.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return ids
}

func (w *world) reaches(teacher, student string) bool {
	w.t.Helper()
	var reached bool
	err := w.tx.QueryRow(context.Background(), `
		SELECT $1::uuid IN `+visibility.AssignmentIDs(2)+`
		   AND `+visibility.Papers(4, 2, "$1::uuid", "$3::uuid"),
		w.assignment, teacher, student, false).Scan(&reached)
	if err != nil {
		w.t.Fatal(err)
	}
	return reached
}

func TestThePapersAStudentSatIsReadByExactlyTheTeachersPapersLetReachIt(t *testing.T) {
	w := newWorld(t)
	teachers := []string{w.creator, w.t1, w.t2, w.t3}
	for name, student := range w.students {
		var forward []string
		for _, teacher := range teachers {
			if w.reaches(teacher, student) {
				forward = append(forward, teacher)
			}
		}
		slices.Sort(forward)
		if got := w.readers(student); !slices.Equal(got, forward) {
			t.Errorf("a student %s: PaperReaders gives %v, Papers lets %v read the paper", name, got, forward)
		}
	}
}

func TestATeacherOfOneTargetClassIsNotToldOfAStudentInAnother(t *testing.T) {
	w := newWorld(t)
	cases := []struct {
		name    string
		student string
		want    []string
	}{
		{"a student in the first class only", w.onlyOne, []string{w.creator, w.t1}},
		{"a student in the second class only", w.onlyTwo, []string{w.creator, w.t2}},
		{"a student in both classes", w.both, []string{w.creator, w.t1, w.t2}},
		{"a student named on the assignment alone", w.named, []string{w.creator}},
		{"a student in a class the assignment does not target", w.elsewhere, []string{w.creator}},
	}
	for _, c := range cases {
		want := slices.Clone(c.want)
		slices.Sort(want)
		if got := w.readers(c.student); !slices.Equal(got, want) {
			t.Errorf("%s: read by %v, want %v", c.name, got, want)
		}
	}
}

func TestTheRosterIsTheTargetClassesAndTheNamedStudents(t *testing.T) {
	w := newWorld(t)
	rows, err := w.tx.Query(context.Background(),
		`SELECT r.user_id::text FROM `+visibility.Roster("$1::uuid")+` r ORDER BY 1`, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{w.both, w.onlyOne, w.onlyTwo, w.named, w.madeByTwo, w.viaOther}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the roster is %v, want %v: two classes' members once each, and the named student", got, want)
	}
}
