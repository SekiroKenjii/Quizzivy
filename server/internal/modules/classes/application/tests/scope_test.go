//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type classWorld struct {
	tx             pgx.Tx
	svc            *application.Application
	a, b, admin    string
	classA, classB string
	studentA       string
	marker         string
}

func (w *classWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *classWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *classWorld) user(t *testing.T, builtin, name string, creator *string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id, created_by) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = $3), $4) RETURNING id::text`,
		uuid.NewString()+"@example.test", name+" "+w.marker, builtin, creator)
}

func (w *classWorld) class(t *testing.T, teacher string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp "+w.marker+" "+uuid.NewString()[:8], teacher)
}

func newClassWorld(t *testing.T) *classWorld {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	dbx := db.NewContext(tx)
	w := &classWorld{tx: tx, svc: application.New(repositories.NewPostgres(dbx), attemptsrepo.NewStudentStats(dbx), joinKeys), marker: "pham-vi-" + uuid.NewString()[:8]}
	w.a, w.b, w.admin = w.user(t, "teacher", "Giáo viên A", nil), w.user(t, "teacher", "Giáo viên B", nil), w.user(t, "admin", "Quản trị", nil)
	w.classA, w.classB = w.class(t, w.a), w.class(t, w.b)
	w.studentA = w.user(t, "student", "Học viên A", &w.a)
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, w.classA, w.studentA, w.a)
	return w
}

func (w *classWorld) who(id string) actor.Actor {
	return actor.Actor{ID: id, Scope: access.Scope{UserID: id, All: id == w.admin}}
}

func (w *classWorld) state(t *testing.T, classID string) string {
	t.Helper()
	return w.id(t, `SELECT concat_ws('|', name, coalesce(description, ''), self_join_enabled::text, coalesce(archived_at::text, ''),
		(SELECT count(*) FROM app.class_members m WHERE m.class_id = c.id)::text,
		(SELECT count(*) FROM app.class_join_codes j WHERE j.class_id = c.id)::text) FROM app.classes c WHERE c.id = $1`, classID)
}

func TestEachTeacherListsAndCountsOnlyTheClassesTheyTeach(t *testing.T) {
	w := newClassWorld(t)
	for name, c := range map[string]struct {
		scope    access.Scope
		want     []string
		students int
	}{
		"A":              {access.Scope{UserID: w.a}, []string{w.classA}, 1},
		"B":              {access.Scope{UserID: w.b}, []string{w.classB}, 0},
		"scope.all":      {access.Scope{UserID: w.admin, All: true}, []string{w.classA, w.classB}, 1},
		"the zero scope": {access.Scope{}, []string{}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			listed, err := w.svc.Queries.List.Handle(context.Background(), query.List{Input: domain.ListInput{Query: w.marker, Limit: 100, Scope: c.scope}})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(listed.Items))
			for _, class := range listed.Items {
				got = append(got, class.ID)
			}
			slices.Sort(got)
			want := slices.Sorted(slices.Values(c.want))
			if !slices.Equal(got, want) || listed.Page.Total != len(want) {
				t.Errorf("lists %v with total %d, want %v", got, listed.Page.Total, want)
			}
			facets, err := w.svc.Queries.Facets.Handle(context.Background(), query.Facets{Query: w.marker, Scope: c.scope})
			if err != nil || facets.All != len(want) || facets.Students != c.students {
				t.Errorf("facets %+v (%v), want %d classes and %d students", facets, err, len(want), c.students)
			}
		})
	}
}

func TestAnotherTeachersClassAnswersAsAMissingOne(t *testing.T) {
	w := newClassWorld(t)
	ctx := context.Background()
	b := w.who(w.b)
	studentB := w.user(t, "student", "Học viên B", &w.b)
	name := "Đổi tên"
	w.exec(t, `UPDATE app.classes SET archived_at = now() WHERE id = $1`, w.classA)
	before := w.state(t, w.classA)
	for label, classID := range map[string]string{"A's class": w.classA, "a missing class": uuid.NewString()} {
		if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: classID, Scope: b.Scope}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B opening %s: %v", label, err)
		}
		for input, update := range map[string]domain.UpdateInput{"a rename": {Name: &name}, "nothing": {}} {
			if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{ClassID: classID, Input: update, Scope: b.Scope}); !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("B updating %s with %s: %v", label, input, err)
			}
		}
		if _, err := w.svc.Commands.Archive.Handle(ctx, command.Archive{ClassID: classID, Archived: false, Actor: b}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B restoring %s: %v", label, err)
		}
		if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{ClassID: classID, Actor: b}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B deleting %s: %v", label, err)
		}
		members, err := w.svc.Queries.Members.Handle(ctx, query.Members{ClassID: classID, Scope: b.Scope})
		if err != nil || len(members.Items) != 0 || members.Page.Total != 0 {
			t.Errorf("B listing %s's members: %d rows, total %d (%v)", label, len(members.Items), members.Page.Total, err)
		}
		if _, err := w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: classID, UserID: studentB, Actor: b}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B adding B's student to %s: %v", label, err)
		}
		if _, err := w.svc.Commands.RemoveMember.Handle(ctx, command.RemoveMember{ClassID: classID, UserID: w.studentA, Actor: b}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B removing a member of %s: %v", label, err)
		}
		if _, err := w.svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: w.b}}); !errors.Is(err, domain.ErrClassNotFound) {
			t.Errorf("B issuing a code for %s: %v", label, err)
		}
		if _, err := w.svc.Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{ClassID: classID, ActorUserID: w.b}}); !errors.Is(err, domain.ErrClassNotFound) {
			t.Errorf("B revoking %s's code: %v", label, err)
		}
	}
	if after := w.state(t, w.classA); after != before {
		t.Errorf("B's refused writes changed A's class from %s to %s", before, after)
	}
	var audited int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1`, w.b).Scan(&audited); err != nil || audited != 0 {
		t.Errorf("B's refused writes left %d audit rows (%v)", audited, err)
	}
	if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{ClassID: w.classB, Actor: b}); !errors.Is(err, domain.ErrNotArchived) {
		t.Errorf("B deleting B's own active class: %v, want the not-archived refusal", err)
	}
	store := repositories.NewPostgres(db.NewContext(w.tx))
	if err := store.RemoveMember(ctx, domain.RemoveMemberInput{ClassID: w.classA, UserID: w.studentA, ActorUserID: w.b}); err != nil {
		t.Fatal(err)
	}
	if after := w.state(t, w.classA); after != before {
		t.Errorf("the store removed a member of A's class for B: %s, was %s", after, before)
	}
}

func TestScopeAllReachesEveryClass(t *testing.T) {
	w := newClassWorld(t)
	ctx := context.Background()
	admin := w.who(w.admin)
	name := "Quản trị đổi tên"
	if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: w.classA, Scope: admin.Scope}); err != nil {
		t.Errorf("scope.all opening A's class: %v", err)
	}
	if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{ClassID: w.classA, Input: domain.UpdateInput{Name: &name}, Scope: admin.Scope}); err != nil {
		t.Errorf("scope.all renaming A's class: %v", err)
	}
	if _, err := w.svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: w.classA, ActorUserID: w.admin, All: true}}); err != nil {
		t.Errorf("scope.all issuing a code for A's class: %v", err)
	}
	if _, err := w.svc.Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{ClassID: w.classA, ActorUserID: w.admin, All: true}}); err != nil {
		t.Errorf("scope.all revoking A's code: %v", err)
	}
	loose := w.user(t, "student", "Tự do", nil)
	if _, err := w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: w.classA, UserID: loose, Actor: admin}); err != nil {
		t.Errorf("scope.all adding a student to A's class: %v", err)
	}
	if _, err := w.svc.Commands.RemoveMember.Handle(ctx, command.RemoveMember{ClassID: w.classA, UserID: loose, Actor: admin}); err != nil {
		t.Errorf("scope.all removing that student: %v", err)
	}
	if left := w.id(t, `SELECT count(*)::text FROM app.class_members WHERE class_id = $1 AND user_id = $2`, w.classA, loose); left != "0" {
		t.Error("scope.all's removal left the student in A's class")
	}
	if _, err := w.svc.Commands.Archive.Handle(ctx, command.Archive{ClassID: w.classA, Archived: true, Actor: admin}); err != nil {
		t.Errorf("scope.all archiving A's class: %v", err)
	}
	if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{ClassID: w.classA, Actor: admin}); err != nil {
		t.Errorf("scope.all deleting A's archived class: %v", err)
	}
}

func TestAddingAMemberNeedsAStudentTheTeacherReaches(t *testing.T) {
	w := newClassWorld(t)
	ctx := context.Background()
	b := w.who(w.b)
	disabled := w.user(t, "student", "Đã khoá", &w.b)
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, disabled)
	for label, userID := range map[string]string{
		"A's student":            w.studentA,
		"a missing account":      uuid.NewString(),
		"B's disabled student":   disabled,
		"a teacher":              w.a,
		"a staff account B made": w.user(t, "teacher", "Trợ giảng", &w.b),
		"a student nobody holds": w.user(t, "student", "Tự do", nil),
	} {
		if _, err := w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: w.classB, UserID: userID, Actor: b}); !errors.Is(err, domain.ErrNotAStudent) {
			t.Errorf("B adding %s to B's class: %v, want the one refusal every unreachable account gets", label, err)
		}
	}

	otherClassOfB := w.class(t, w.b)
	inOtherClass := w.user(t, "student", "Lớp khác", nil)
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, otherClassOfB, inOtherClass, w.b)
	testID := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, w.b)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, testID, w.b)
	targeted := w.user(t, "student", "Được giao", nil)
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		VALUES ($1, $2, now(), now() + interval '1 day', 45, $3) RETURNING id::text`, testID, version, w.b)
	w.exec(t, `INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2)`, assignment, targeted)
	for label, userID := range map[string]string{
		"a student B created":         w.user(t, "student", "Của B", &w.b),
		"a member of another B class": inOtherClass,
		"a target of B's assignment":  targeted,
	} {
		if _, err := w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: w.classB, UserID: userID, Actor: b}); err != nil {
			t.Errorf("B adding %s to B's class: %v", label, err)
		}
	}
}

func TestAClassNamesItsOwnTeacher(t *testing.T) {
	w := newClassWorld(t)
	ctx := context.Background()
	code, err := w.svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: w.classA, ActorUserID: w.a}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := w.svc.Queries.Preview.Handle(ctx, query.Preview{Code: code.Code})
	if err != nil || preview.TeacherName != "Giáo viên A "+w.marker {
		t.Errorf("the join preview names %q (%v), want A", preview.TeacherName, err)
	}
	mine, err := w.svc.Queries.ListMine.Handle(ctx, query.ListMine{UserID: w.studentA})
	if err != nil || len(mine) != 1 || mine[0].TeacherName == nil || *mine[0].TeacherName != "Giáo viên A "+w.marker {
		t.Errorf("the student's classes name %+v (%v), want A", mine, err)
	}
}

func (w *classWorld) sat(t *testing.T, author, class, student string) {
	t.Helper()
	testID := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, author)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, testID, author)
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		VALUES ($1, $2, now() - interval '1 day', now() + interval '1 day', 45, $3, now()) RETURNING id::text`, testID, version, author)
	w.exec(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2)`, assignment, class)
	w.exec(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		        started_at, deadline_at, submitted_at, graded_at, score_earned, score_total, flagged)
		VALUES ($1, $2, $3, 1, 'graded', gen_random_uuid(), 1, sha256('b'::bytea), now() - interval '2 hours', now() - interval '1 hour', now() - interval '90 minutes', now(), 5, 10, true)`,
		assignment, version, student)
}

func TestARosterShowsOnlyTheViewersFigures(t *testing.T) {
	w := newClassWorld(t)
	ctx := context.Background()
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, w.classB, w.studentA, w.b)
	w.sat(t, w.a, w.classA, w.studentA)
	submitted := func(scope access.Scope, classID string) int {
		t.Helper()
		members, err := w.svc.Queries.Members.Handle(ctx, query.Members{ClassID: classID, Scope: scope})
		if err != nil || len(members.Items) != 1 {
			t.Fatalf("the roster lists %d members (%v)", len(members.Items), err)
		}
		return members.Items[0].Stats.SubmittedCount + members.Items[0].Stats.FlaggedCount
	}
	if got := submitted(w.who(w.b).Scope, w.classB); got != 0 {
		t.Errorf("B's roster shows %d figures from A's assignment", got)
	}
	if got := submitted(w.who(w.a).Scope, w.classA); got != 2 {
		t.Errorf("A's roster shows %d figures, want A's own", got)
	}
	if got := submitted(w.who(w.admin).Scope, w.classB); got != 2 {
		t.Errorf("scope.all's roster shows %d figures, want every one", got)
	}

	reachedByB := w.user(t, "student", "Của B", &w.b)
	w.sat(t, w.a, w.classA, reachedByB)
	added, err := w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: w.classB, UserID: reachedByB, Actor: w.who(w.b)})
	if err != nil || added.Stats.SubmittedCount != 0 {
		t.Errorf("B's added member carries %d submitted from A's assignment (%v)", added.Stats.SubmittedCount, err)
	}
	otherOfB := w.class(t, w.b)
	added, err = w.svc.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: otherOfB, UserID: reachedByB, Actor: w.who(w.admin)})
	if err != nil || added.Stats.SubmittedCount != 1 {
		t.Errorf("scope.all's added member carries %d submitted, want 1 (%v)", added.Stats.SubmittedCount, err)
	}
}
