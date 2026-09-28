//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/core/maintenance"
	assignmentsdomain "quizzivy/internal/modules/assignments/domain"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	classesapp "quizzivy/internal/modules/classes/application"
	classescmd "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type ladder struct {
	*rosterWorld
	classes     *classesapp.Application
	assignments *assignmentsrepo.Postgres
	root        string
	spare       string
	versionA    string
}

var everything = access.NewSet(access.All()...)

func newLadder(t *testing.T) *ladder {
	t.Helper()
	w := newRosterWorld(t)
	dbx := db.NewContext(w.tx)
	l := &ladder{rosterWorld: w, classes: classesapp.New(classesrepo.NewPostgres(dbx), attemptsrepo.NewStudentStats(dbx)), assignments: assignmentsrepo.NewPostgres(dbx)}
	l.root = w.user(t, "admin", "Quản trị gốc", nil)
	l.spare = w.class(t, w.a)
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề leo thang', 'published', 1, $1, $1) RETURNING id::text`, w.a)
	l.versionA = w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, test, w.a)
	return l
}

func (l *ladder) role(t *testing.T, keys ...string) string {
	t.Helper()
	role := l.id(t, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Leo thang "+uuid.NewString()[:8])
	l.grant(t, role, keys...)
	return role
}

func (l *ladder) grant(t *testing.T, role string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		l.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, role, k)
	}
}

func (l *ladder) holder(t *testing.T, role string) string {
	t.Helper()
	return l.id(t, `INSERT INTO app.users (email, full_name, role_id, created_by) VALUES ($1, $2, $3::uuid, $4) RETURNING id::text`,
		uuid.NewString()+"@example.test", "Vai tuỳ chỉnh "+l.marker, role, l.a)
}

func (l *ladder) joinByCode(t *testing.T, class, teacher string, users ...string) {
	t.Helper()
	ctx := context.Background()
	rotated, err := l.classes.Commands.Rotate.Handle(ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: class, ActorUserID: teacher}})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		joined, err := l.classes.Commands.EnrolExisting.Handle(ctx, classescmd.EnrolExisting{UserID: u, Code: rotated.Code})
		if err != nil || joined.Outcome != classesdomain.PreviewOK {
			t.Fatalf("joining by code: %+v (%v)", joined, err)
		}
	}
}

func (l *ladder) assign(ctx context.Context, req assignmentsdomain.Request, student string) error {
	now := time.Now()
	_, err := l.assignments.Create(ctx, req, assignmentsdomain.WriteInput{TestVersionID: l.versionA, StudentIDs: []string{student},
		OpensAt: now.Add(-time.Hour), ClosesAt: now.Add(time.Hour), DurationMin: 45, MaxAttempts: 1,
		Integrity: assignmentsdomain.Integrity{OnLimitExceeded: "flag", MinAwayMs: 3000}, Now: now})
	return err
}

func (l *ladder) refused(t *testing.T, label, target string) {
	t.Helper()
	ctx := context.Background()
	a := domain.WriteRequest{ActorID: l.a}
	root := domain.WriteRequest{ActorID: l.root, All: true, Grants: everything}
	before := l.snapshot(t, target)
	name := "Đổi tên"
	yes := true
	for op, err := range map[string]error{
		"A's get":             second(l.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: target, Scope: a.Scope()})),
		"scope.all's get":     second(l.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: target, Scope: root.Scope()})),
		"A's update":          second(l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: a, Input: domain.StudentPatch{ID: target, FullName: &name}})),
		"scope.all's disable": second(l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: root, Input: domain.StudentPatch{ID: target, Disabled: &yes}})),
		"A's reset":           second(l.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: a, ID: target})),
		"scope.all's reset":   second(l.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: root, ID: target})),
	} {
		if !errors.Is(err, domain.ErrStudentNotFound) {
			t.Errorf("%s: %s answered %v, want ErrStudentNotFound", label, op, err)
		}
	}
	if _, listed := l.list(t, a.Scope(), "")[target]; listed {
		t.Errorf("%s: A's student list shows them", label)
	}
	for who, member := range map[string]actor.Actor{"A": {ID: l.a, Scope: a.Scope()}, "scope.all": {ID: l.root, Scope: root.Scope()}} {
		if _, err := l.classes.Commands.AddMember.Handle(ctx, classescmd.AddMember{ClassID: l.spare, UserID: target, Actor: member}); !errors.Is(err, classesdomain.ErrNotAStudent) {
			t.Errorf("%s: %s adding them to a class answered %v, want ErrNotAStudent", label, who, err)
		}
	}
	for who, req := range map[string]assignmentsdomain.Request{"A": {ActorID: l.a}, "scope.all": {ActorID: l.root, All: true}} {
		var invalid *assignmentsdomain.ValidationError
		if err := l.assign(ctx, req, target); !errors.As(err, &invalid) {
			t.Errorf("%s: %s assigning them individually answered %v, want the missing-student answer", label, who, err)
		}
	}
	if _, err := maintenance.AnonymizeStudent(ctx, l.tx, target, true); err == nil || err.Error() != "only student accounts may be anonymized" {
		t.Errorf("%s: anonymizing them answered %v", label, err)
	}
	if after := l.snapshot(t, target); after != before {
		t.Errorf("%s: refused writes changed them from %s to %s", label, before, after)
	}
	l.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, target)
	if _, err := l.app.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: root, ID: target}); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Errorf("%s: scope.all deleting them answered %v, want ErrStudentNotFound", label, err)
	}
}

func (l *ladder) accepted(t *testing.T, label, target string) {
	t.Helper()
	ctx := context.Background()
	a := domain.WriteRequest{ActorID: l.a}
	name := "Đổi tên"
	if _, err := l.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: target, Scope: a.Scope()}); err != nil {
		t.Errorf("%s: A's get: %v", label, err)
	}
	if _, listed := l.list(t, a.Scope(), "")[target]; !listed {
		t.Errorf("%s: A's student list leaves them out", label)
	}
	if _, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: a, Input: domain.StudentPatch{ID: target, FullName: &name}}); err != nil {
		t.Errorf("%s: A's update: %v", label, err)
	}
	if temporary, err := l.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: a, ID: target}); err != nil || temporary == "" {
		t.Errorf("%s: A's reset: %q (%v)", label, temporary, err)
	}
	if _, err := maintenance.AnonymizeStudent(ctx, l.tx, target, false); err != nil {
		t.Errorf("%s: anonymizing them (dry run): %v", label, err)
	}
	if _, err := l.classes.Commands.AddMember.Handle(ctx, classescmd.AddMember{ClassID: l.spare, UserID: target, Actor: actor.Actor{ID: l.a, Scope: a.Scope()}}); err != nil {
		t.Errorf("%s: A adding them to a class: %v", label, err)
	}
	if err := l.assign(ctx, assignmentsdomain.Request{ActorID: l.a}, target); err != nil {
		t.Errorf("%s: A assigning them individually: %v", label, err)
	}
}

func TestAnAdminWhoTakesTestsIsNeverAStudentTarget(t *testing.T) {
	l := newLadder(t)
	l.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key)
		SELECT r.id, 'learning.take_tests' FROM app.roles r WHERE r.builtin_key = 'admin' ON CONFLICT (role_id, permission_key) DO NOTHING`)
	taker := l.user(t, "admin", "Quản trị làm bài", nil)
	pupil := l.user(t, "student", "Học viên", nil)
	l.joinByCode(t, l.classA, l.a, taker, pupil)
	if _, err := l.app.Queries.GetStudent.Handle(context.Background(), query.GetStudent{ID: pupil, Scope: access.Scope{UserID: l.a}}); err != nil {
		t.Fatalf("a student who joined by the same code is not A's: %v", err)
	}
	if account, err := l.app.Queries.StudentAccount.Handle(context.Background(), query.StudentAccount{ID: taker}); err != nil || account.ID != taker {
		t.Errorf("the account of an attempt's sitter who is an Admin: %+v (%v)", account, err)
	}
	l.refused(t, "the Admin who takes tests", taker)
}

func TestACustomRoleIsAStudentTargetOnlyWhileItHoldsNothingButTakeTests(t *testing.T) {
	l := newLadder(t)
	rows, err := l.tx.Query(context.Background(), `SELECT key FROM app.permissions WHERE in_matrix AND key <> 'learning.take_tests' ORDER BY group_key, ordinal`)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil || len(keys) == 0 {
		t.Fatalf("matrix keys %v (%v)", keys, err)
	}
	for _, k := range keys {
		role := l.role(t, string(access.LearningTakeTests))
		holder := l.holder(t, role)
		l.grant(t, role, k)
		l.refused(t, "a role granted "+k+" after it was assigned", holder)
	}

	emptied := l.role(t, string(access.TeachingGrading))
	holder := l.holder(t, emptied)
	l.exec(t, `DELETE FROM app.role_permissions WHERE role_id = $1::uuid`, emptied)
	l.accepted(t, "a role emptied after it was assigned", holder)
	l.accepted(t, "a role holding only Take tests", l.holder(t, l.role(t, string(access.LearningTakeTests))))
}

func TestOnlyAStudentNoOneElseReachesIsResetOrReaddressedByATeacher(t *testing.T) {
	l := newLadder(t)
	ctx := context.Background()
	a, b := domain.WriteRequest{ActorID: l.a}, domain.WriteRequest{ActorID: l.b}
	admin := domain.WriteRequest{ActorID: l.admin, All: true, Grants: everything}
	onlyB := l.user(t, "student", "Chỉ lớp B", nil)
	l.enrol(t, l.classB, onlyB)
	createdByA := l.user(t, "student", "A tạo, học lớp B", &l.a)
	l.enrol(t, l.classB, createdByA)
	targetedByA := l.user(t, "student", "A giao riêng, học lớp B", nil)
	l.enrol(t, l.classB, targetedByA)
	l.assignment(t, l.a, nil, &targetedByA)

	for label, c := range map[string]struct {
		req     domain.WriteRequest
		student string
		want    error
	}{
		"B, a student also in A's class":               {b, l.shared, domain.ErrStudentShared},
		"B, a student A created":                       {b, createdByA, domain.ErrStudentShared},
		"B, a student A targets individually":          {b, targetedByA, domain.ErrStudentShared},
		"B, a student only in B's class":               {b, onlyB, nil},
		"B, a student in no class whom A created":      {b, l.created, domain.ErrStudentNotFound},
		"A, a student in no class whom A created":      {a, l.created, nil},
		"A, a student only in A's class":               {a, l.member, nil},
		"the Admin, a student two teachers share":      {admin, l.shared, nil},
		"A, a student in no class whom A only targets": {a, l.targeted, domain.ErrStudentShared},
	} {
		before := l.snapshot(t, c.student)
		_, err := l.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: c.req, ID: c.student})
		if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("reset by %s answered %v, want %v", label, err, c.want)
		}
		if c.want != nil && l.snapshot(t, c.student) != before {
			t.Errorf("a refused reset by %s changed the student", label)
		}
	}

	for label, c := range map[string]struct {
		req     domain.WriteRequest
		student string
		want    error
	}{
		"B, a student also in A's class":          {b, l.shared, domain.ErrStudentShared},
		"B, a student only in B's class":          {b, onlyB, nil},
		"the Admin, a student two teachers share": {admin, l.shared, nil},
	} {
		before := l.snapshot(t, c.student)
		email := uuid.NewString() + "@example.test"
		_, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: c.req, Input: domain.StudentPatch{ID: c.student, Email: &email}})
		if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("an email change by %s answered %v, want %v", label, err, c.want)
		}
		if c.want != nil && l.snapshot(t, c.student) != before {
			t.Errorf("a refused email change by %s changed the student", label)
		}
	}
	gone := l.user(t, "student", "Đã khoá, dùng chung", nil)
	l.enrol(t, l.classA, gone)
	l.enrol(t, l.classB, gone)
	l.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, gone)
	if _, err := l.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: b, ID: gone}); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Errorf("B resetting a disabled shared student answered %v, want ErrStudentNotFound", err)
	}
	name := "Chỉ đổi tên"
	if _, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: b, Input: domain.StudentPatch{ID: l.shared, FullName: &name}}); err != nil {
		t.Errorf("B renaming a shared student: %v", err)
	}

	yes := true
	before := l.snapshot(t, l.member)
	if _, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: a, Input: domain.StudentPatch{ID: l.member, Disabled: &yes}}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("a Teacher disabling their own student answered %v, want ErrForbidden", err)
	}
	if l.snapshot(t, l.member) != before {
		t.Error("a refused disable changed the student")
	}
	if _, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: b, Input: domain.StudentPatch{ID: l.member, Disabled: &yes}}); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Errorf("a Teacher disabling another teacher's student answered %v, want ErrStudentNotFound", err)
	}
	if _, err := l.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: admin, Input: domain.StudentPatch{ID: l.member, Disabled: &yes}}); err != nil {
		t.Errorf("the Admin disabling a student: %v", err)
	}
}

func second[T any](_ T, err error) error { return err }
