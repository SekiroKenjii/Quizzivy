//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/visibility"
)

type adminWorld struct {
	*classWorld
	svc     *application.Application
	notices *joinNotices
	joining *atomic.Bool
}

func newAdminWorld(t *testing.T) *adminWorld {
	t.Helper()
	w := newClassWorld(t)
	on := &atomic.Bool{}
	on.Store(true)
	notices := &joinNotices{}
	svc := application.New(repositories.NewPostgres(db.NewContext(w.tx)), nil, joinKeys).
		WithNotifier(notices).WithSignInPolicy(switchableJoining(on))
	return &adminWorld{classWorld: w, svc: svc, notices: notices, joining: on}
}

func (w *adminWorld) listAll(t *testing.T, in domain.AdminListInput) ([]string, query.ListAllResult) {
	t.Helper()
	in.Limit = 50
	result, err := w.svc.Queries.ListAll.Handle(context.Background(), query.ListAll{Input: in})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		ids = append(ids, item.ID)
	}
	slices.Sort(ids)
	return ids, result
}

func (w *adminWorld) audit(t *testing.T, action, entityID string) []map[string]any {
	t.Helper()
	rows, err := w.tx.Query(context.Background(), `SELECT coalesce(diff, 'null'::jsonb), actor_user_id::text FROM app.audit_log
		WHERE action = $1 AND entity_id = $2::uuid ORDER BY id`, action, entityID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		var by string
		if err := rows.Scan(&raw, &by); err != nil {
			t.Fatal(err)
		}
		entry := map[string]any{"actor": by}
		if err := json.Unmarshal(raw, &entry); err != nil && string(raw) != "null" {
			t.Fatal(err)
		}
		out = append(out, entry)
	}
	return out
}

func sortedIDs(ids ...string) []string {
	return slices.Sorted(slices.Values(ids))
}

func TestTheAdminListSearchesClassesAndTeachersAndFiltersDisabledTeachers(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	_, before := w.listAll(t, domain.AdminListInput{})
	w.exec(t, `UPDATE app.users SET display_name = 'Cô Hương ' || $2 WHERE id = $1`, w.b, w.marker)
	archived := w.class(t, w.a)
	w.exec(t, `UPDATE app.classes SET archived_at = now() WHERE id = $1`, archived)

	if got, _ := w.listAll(t, domain.AdminListInput{Query: w.marker}); !slices.Equal(got, sortedIDs(w.classA, w.classB)) {
		t.Errorf("the marker finds %v, want both live classes", got)
	}
	if got, result := w.listAll(t, domain.AdminListInput{Query: "giao vien a " + w.marker}); !slices.Equal(got, sortedIDs(w.classA)) ||
		result.Facets.Active != 1 || result.Facets.Archived != 1 {
		t.Errorf("A's name without accents finds %v with %+v, want A's live class and one archived", got, result.Facets)
	}
	if got, _ := w.listAll(t, domain.AdminListInput{Query: "co huong " + w.marker}); !slices.Equal(got, sortedIDs(w.classB)) {
		t.Errorf("B's display name finds %v, want B's class", got)
	}
	if got, _ := w.listAll(t, domain.AdminListInput{Query: w.marker, Tab: "archived"}); !slices.Equal(got, sortedIDs(archived)) {
		t.Errorf("the archived tab holds %v, want the archived class", got)
	}

	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.a)
	got, result := w.listAll(t, domain.AdminListInput{Query: w.marker, DisabledTeacher: true})
	if !slices.Equal(got, sortedIDs(w.classA)) || result.Facets.Active != 1 || result.Facets.Archived != 1 {
		t.Errorf("the disabled-teacher filter holds %v with %+v, want A's live class and one archived", got, result.Facets)
	}
	if delta := result.Facets.DisabledTeacherClasses - before.Facets.DisabledTeacherClasses; delta != 1 {
		t.Errorf("disabledTeacherClasses moved by %d, want 1: A's live class and not the archived one", delta)
	}
	for _, item := range result.Items {
		if item.Teacher.ID != w.a || !item.Teacher.Disabled || item.Teacher.Name != "Giáo viên A "+w.marker {
			t.Errorf("the row names %+v, want A, disabled", item.Teacher)
		}
	}
	if _, err := w.svc.Queries.ListAll.Handle(ctx, query.ListAll{Input: domain.AdminListInput{Query: "100%_" + w.marker}}); err != nil {
		t.Errorf("a search holding LIKE's wildcards: %v", err)
	}
}

func TestTheAdminListOpensEveryTeachersCode(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	rotated, err := w.svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: w.classB, ActorUserID: w.b}})
	if err != nil {
		t.Fatal(err)
	}
	_, result := w.listAll(t, domain.AdminListInput{Query: w.marker})
	for _, item := range result.Items {
		switch item.ID {
		case w.classB:
			if item.JoinCode == nil || domain.JoinCodes.Format(item.Code) != rotated.Code || !item.JoinCode.ExpiresAt.Equal(rotated.ExpiresAt) {
				t.Errorf("B's class reads %q (%+v), want %s", item.Code, item.JoinCode, rotated.Code)
			}
		case w.classA:
			if item.JoinCode != nil || item.Code != "" {
				t.Errorf("A's class has no code and reads %q", item.Code)
			}
		}
	}
}

func TestAnAdminCreatesAClassForATeacherWithACode(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	label, room := "  Thứ 3, 5 · 18:00 ", " "
	class, err := w.svc.Commands.CreateForTeacher.Handle(ctx, command.CreateForTeacher{
		Name: "Lớp mới " + w.marker, TeacherID: w.a, ScheduleLabel: &label, Room: &room, Actor: w.who(w.admin),
	})
	if err != nil {
		t.Fatal(err)
	}
	if class.ScheduleLabel == nil || *class.ScheduleLabel != "Thứ 3, 5 · 18:00" || class.Room != nil || !class.SelfJoinEnabled {
		t.Errorf("created %+v, want the trimmed label, no room and self-join on", class)
	}
	if class.JoinCode == nil || class.JoinCode.Legacy || class.JoinCode.MaxUses == nil || *class.JoinCode.MaxUses != domain.DefaultMaxUses {
		t.Errorf("the class's code is %+v, want a sealed one with the default cap", class.JoinCode)
	}
	if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: class.ID, Scope: access.Scope{UserID: w.a}}); err != nil {
		t.Errorf("A cannot read the class made for them: %v", err)
	}
	if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: class.ID, Scope: access.Scope{UserID: w.admin}}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the Admin without scope.all reads it: %v", err)
	}
	code, err := w.svc.Queries.ActiveCode.Handle(ctx, query.ActiveCode{ClassID: class.ID, Scope: access.Scope{UserID: w.a}})
	if err != nil || code.Code == "" {
		t.Errorf("A reads the code as %+v (%v)", code, err)
	}
	created := w.audit(t, "class.created", class.ID)
	if len(created) != 1 || created[0]["onBehalfOf"] != w.a || created[0]["actor"] != w.admin {
		t.Errorf("class.created rows %v, want one by the Admin on behalf of A", created)
	}
	var issued int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log l JOIN app.class_join_codes c ON c.id = l.entity_id
		WHERE l.action = 'class.join_code_issued' AND c.class_id = $1 AND l.actor_user_id = $2`, class.ID, w.admin).Scan(&issued); err != nil || issued != 1 {
		t.Errorf("%d class.join_code_issued rows (%v), want 1", issued, err)
	}
}

func TestAClassCreatedWhileJoiningIsOffHasNoCode(t *testing.T) {
	w := newAdminWorld(t)
	w.joining.Store(false)
	class, err := w.svc.Commands.CreateForTeacher.Handle(context.Background(), command.CreateForTeacher{
		Name: "Lớp " + w.marker, TeacherID: w.b, Actor: w.who(w.admin),
	})
	if err != nil {
		t.Fatal(err)
	}
	if class.JoinCode != nil || !class.SelfJoinEnabled {
		t.Errorf("created %+v, want self-join on and no code", class)
	}
}

func TestAClassGoesOnlyToAnActiveTeacher(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	disabled := w.user(t, "teacher", "Đã tắt", nil)
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, disabled)
	noWorkspace := w.user(t, "teacher", "Không quyền", nil)
	role := w.id(t, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Chỉ đọc "+w.marker)
	w.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'system.audit.read')`, role)
	w.exec(t, `UPDATE app.users SET role_id = $2 WHERE id = $1`, noWorkspace, role)
	var classes int
	count := func() int {
		if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.classes WHERE name = $1`, "Lớp "+w.marker).Scan(&classes); err != nil {
			t.Fatal(err)
		}
		return classes
	}
	before := count()
	for label, teacher := range map[string]string{
		"a student":           w.studentA,
		"a disabled teacher":  disabled,
		"no teacher at all":   uuid.NewString(),
		"a role without keys": noWorkspace,
	} {
		_, err := w.svc.Commands.CreateForTeacher.Handle(ctx, command.CreateForTeacher{Name: "Lớp " + w.marker, TeacherID: teacher, Actor: w.who(w.admin)})
		if !errors.Is(err, domain.ErrInvalidTeacher) {
			t.Errorf("creating for %s: %v, want ErrInvalidTeacher", label, err)
		}
		_, err = w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classA, TeacherID: teacher, Actor: w.who(w.admin)})
		if !errors.Is(err, domain.ErrInvalidTeacher) {
			t.Errorf("giving A's class to %s: %v, want ErrInvalidTeacher", label, err)
		}
	}
	if count() != before {
		t.Error("a refused create wrote a class")
	}
	_, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: uuid.NewString(), TeacherID: disabled, Actor: w.who(w.admin)})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a missing class with a disabled teacher: %v, want ErrNotFound first", err)
	}
	if got := w.audit(t, "class.teacher_changed", w.classA); len(got) != 0 {
		t.Errorf("refused changes wrote %v", got)
	}
}

func TestAChangeOfTeacherMovesTheClassAndItsAssignments(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	testID := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề', 'published', 1, $1, $1) RETURNING id::text`, w.a)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, testID, w.a)
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		VALUES ($1, $2, now(), now() + interval '1 day', 45, $3) RETURNING id::text`, testID, version, w.a)
	w.exec(t, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2)`, assignment, w.classA)
	reaches := func(teacher string) bool {
		var ok bool
		if err := w.tx.QueryRow(ctx, `SELECT $2::uuid IN `+visibility.AssignmentIDs(1), teacher, assignment).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if reaches(w.b) {
		t.Fatal("B reaches A's assignment before the change")
	}

	class, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classA, TeacherID: w.b, Notify: true, Actor: w.who(w.admin)})
	if err != nil || class.ID != w.classA || class.StudentCount != 1 {
		t.Fatalf("change: %+v (%v)", class, err)
	}
	if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: w.classA, Scope: access.Scope{UserID: w.b}}); err != nil {
		t.Errorf("B cannot read the class: %v", err)
	}
	if _, err := w.svc.Queries.Get.Handle(ctx, query.Get{ClassID: w.classA, Scope: access.Scope{UserID: w.a}}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("A still reads the class: %v", err)
	}
	if !reaches(w.b) {
		t.Error("B does not reach the assignment on the class")
	}
	changed := w.audit(t, "class.teacher_changed", w.classA)
	teacher, _ := changed[0]["teacher"].(map[string]any)
	if len(changed) != 1 || changed[0]["actor"] != w.admin || teacher["old"] != w.a || teacher["new"] != w.b {
		t.Errorf("class.teacher_changed rows %v, want one by the Admin from A to B", changed)
	}

	w.notices.mu.Lock()
	sent := slices.Clone(w.notices.sent)
	w.notices.mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("%d notifications, want one each for B and A", len(sent))
	}
	for _, n := range sent {
		params, ok := n.Params.(notificationsdomain.TeacherChanged)
		if n.Kind != notificationsdomain.ClassTeacherChanged || !ok || params.ClassName != class.Name ||
			n.DedupeKey != "teacher_changed:"+w.classA || n.Target == nil || n.Target.Route != notificationsdomain.RouteClasses {
			t.Errorf("notification %+v", n)
		}
		switch n.UserID {
		case w.b:
			if params.TeacherName != nil {
				t.Errorf("B's item names %q, want no teacher", *params.TeacherName)
			}
		case w.a:
			if params.TeacherName == nil || *params.TeacherName != "Giáo viên B "+w.marker {
				t.Errorf("A's item names %v, want B", params.TeacherName)
			}
		default:
			t.Errorf("a notification for %s", n.UserID)
		}
	}

	if _, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classA, TeacherID: w.b, Notify: true, Actor: w.who(w.admin)}); err != nil {
		t.Fatal(err)
	}
	if got := len(w.audit(t, "class.teacher_changed", w.classA)); got != 1 || w.notices.count() != 2 {
		t.Errorf("naming the current teacher wrote %d rows and %d notifications in all, want 1 and 2", got, w.notices.count())
	}
}

func TestADisabledPreviousTeacherIsNotToldAndNotifyOffTellsNobody(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	if _, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classB, TeacherID: w.a, Actor: w.who(w.admin)}); err != nil {
		t.Fatal(err)
	}
	if w.notices.count() != 0 {
		t.Errorf("notify off sent %d notifications", w.notices.count())
	}
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.a)
	if _, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classB, TeacherID: w.b, Notify: true, Actor: w.who(w.admin)}); err != nil {
		t.Fatal(err)
	}
	w.notices.mu.Lock()
	defer w.notices.mu.Unlock()
	if len(w.notices.sent) != 1 || w.notices.sent[0].UserID != w.b {
		t.Errorf("sent %+v, want B's item alone", w.notices.sent)
	}
}

func TestAnActiveAdminCanBeGivenAClass(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	teacherOf := func(classID string) string {
		return w.id(t, `SELECT teacher_id::text FROM app.classes WHERE id = $1`, classID)
	}
	created, err := w.svc.Commands.CreateForTeacher.Handle(ctx, command.CreateForTeacher{Name: "Lớp " + w.marker, TeacherID: w.admin, Actor: w.who(w.admin)})
	if err != nil {
		t.Fatalf("creating a class for an Admin: %v, want it made: the built-in Admin holds every key", err)
	}
	if got := teacherOf(created.ID); got != w.admin {
		t.Errorf("the class made for the Admin is taught by %s", got)
	}
	if _, err := w.svc.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{ClassID: w.classA, TeacherID: w.admin, Actor: w.who(w.admin)}); err != nil {
		t.Fatalf("giving A's class to an Admin: %v, want it moved", err)
	}
	if got := teacherOf(w.classA); got != w.admin {
		t.Errorf("A's class is taught by %s after the change, want the Admin", got)
	}
}

func TestAnUnreadableCodeListsAsNoCodeAndLeavesTheOthersReadable(t *testing.T) {
	w := newAdminWorld(t)
	ctx := context.Background()
	create := func() string {
		t.Helper()
		class, err := w.svc.Commands.CreateForTeacher.Handle(ctx, command.CreateForTeacher{Name: "Lớp " + w.marker, TeacherID: w.a, Actor: w.who(w.admin)})
		if err != nil || class.JoinCode == nil {
			t.Fatalf("create: %+v (%v), want a class with a code", class, err)
		}
		return class.ID
	}
	broken, intact := create(), create()
	w.exec(t, `UPDATE app.class_join_codes SET code_ciphertext = set_byte(code_ciphertext, 20, get_byte(code_ciphertext, 20) # 255)
		WHERE class_id = $1 AND revoked_at IS NULL`, broken)

	_, result := w.listAll(t, domain.AdminListInput{Query: w.marker})
	seen := 0
	for _, item := range result.Items {
		switch item.ID {
		case broken:
			seen++
			if item.Code != "" {
				t.Errorf("the class whose code cannot be opened lists %q, want no code", item.Code)
			}
		case intact:
			seen++
			if item.Code == "" {
				t.Error("the class beside it lists no code")
			}
		}
	}
	if seen != 2 {
		t.Errorf("the list holds %d of the two classes, want both", seen)
	}
}
