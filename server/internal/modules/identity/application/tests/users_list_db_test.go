//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type readWorld struct {
	tx     pgx.Tx
	app    *application.Application
	marker string
}

func newReadWorld(t *testing.T) *readWorld {
	t.Helper()
	tx, err := linkAppPool(t).Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	dbx := db.NewContext(tx)
	w := &readWorld{tx: tx, marker: "rd" + nonce(t) + ".example.test"}
	w.app = application.New(repositories.NewUsers(dbx), nil, time.Hour, nil, nil)
	w.app.SetPrincipals(txPrincipals{tx})
	w.app.SetAdminUsers(repositories.NewAdminUsers(dbx, nil), nil)
	return w
}

type txPrincipals struct{ tx pgx.Tx }

func (p txPrincipals) Forget(string) {}

func (p txPrincipals) Resolve(ctx context.Context, id string) (access.Principal, error) {
	var admin bool
	var keys []string
	err := p.tx.QueryRow(ctx, `
		SELECT r.builtin_key = 'admin',
		       COALESCE((SELECT array_agg(rp.permission_key) FROM app.role_permissions rp WHERE rp.role_id = r.id), '{}')
		  FROM app.users u JOIN app.roles r ON r.id = u.role_id
		 WHERE u.id = $1::uuid`, id).Scan(&admin, &keys)
	if errors.Is(err, pgx.ErrNoRows) {
		return access.Principal{}, accessdomain.ErrUnknownUser
	}
	if err != nil {
		return access.Principal{}, err
	}
	if admin {
		return access.Principal{UserID: id, Permissions: access.NewSet(access.All()...)}, nil
	}
	held := make([]access.Key, len(keys))
	for i, key := range keys {
		held[i] = access.Key(key)
	}
	return access.Principal{UserID: id, Permissions: access.NewSet(held...)}, nil
}

func (w *readWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", strings.Join(strings.Fields(sql), " "), err)
	}
}

func (w *readWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", strings.Join(strings.Fields(sql), " "), err)
	}
	return id
}

func (w *readWorld) email(tag string) string {
	return tag + "-" + uuid.NewString()[:8] + "@" + w.marker
}

func (w *readWorld) user(t *testing.T, tag, role string) string {
	t.Helper()
	return w.named(t, "Người dùng "+tag, w.email(tag), role)
}

func (w *readWorld) named(t *testing.T, fullName, email, role string) string {
	t.Helper()
	roleID := role
	if _, err := uuid.Parse(role); err != nil {
		roleID = w.id(t, `SELECT id::text FROM app.roles WHERE builtin_key = $1`, role)
	}
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, $3::uuid) RETURNING id::text`, email, fullName, roleID)
}

func (w *readWorld) role(t *testing.T, name string, keys ...string) string {
	t.Helper()
	id := w.id(t, `INSERT INTO app.roles (name, description, icon, color) VALUES ($1, '', 'key-round', 'gray') RETURNING id::text`, name+" "+nonce(t))
	for _, key := range keys {
		w.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, id, key)
	}
	return id
}

func (w *readWorld) list(t *testing.T, in domain.AdminUserQuery) query.ListUsersResult {
	t.Helper()
	got, err := w.app.Queries.ListUsers.Handle(context.Background(), query.ListUsers{Query: in})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (w *readWorld) mine(t *testing.T, in domain.AdminUserQuery) query.ListUsersResult {
	t.Helper()
	in.Query = w.marker
	return w.list(t, in)
}

func accountIDs(items []domain.AdminUser) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}

func sameSet(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	g, wnt := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(wnt)
	if !slices.Equal(g, wnt) {
		t.Errorf("%s = %v, want %v", what, g, wnt)
	}
}

func TestTheTabsSplitEveryRoleAndKeepTheDisabledInTheirRoleTab(t *testing.T) {
	w := newReadWorld(t)
	admin := w.user(t, "admin", "admin")
	teacher := w.user(t, "teacher", "teacher")
	assistant := w.user(t, "assistant", "assistant")
	student := w.user(t, "student", "student")
	grader := w.user(t, "grader", w.role(t, "Chấm bài", "teaching.grading"))
	onlyTakes := w.user(t, "taker", w.role(t, "Chỉ làm bài", "learning.take_tests"))
	noKeys := w.user(t, "empty", w.role(t, "Không quyền"))
	goneTeacher := w.user(t, "gone-teacher", "teacher")
	goneStudent := w.user(t, "gone-student", "student")
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id IN ($1::uuid, $2::uuid)`, goneTeacher, goneStudent)

	for tab, want := range map[domain.AdminTab][]string{
		domain.TabAll:      {admin, teacher, assistant, student, grader, onlyTakes, noKeys, goneTeacher, goneStudent},
		domain.TabTeachers: {teacher, assistant, grader, goneTeacher},
		domain.TabStudents: {student, onlyTakes, noKeys, goneStudent},
		domain.TabAdmins:   {admin},
		domain.TabDisabled: {goneTeacher, goneStudent},
	} {
		got := w.mine(t, domain.AdminUserQuery{Tab: tab})
		sameSet(t, string(tab), accountIDs(got.Items), want...)
		if got.Page.Total != len(want) {
			t.Errorf("%s: total = %d, want %d", tab, got.Page.Total, len(want))
		}
		if got.Facets != (domain.AdminUserFacets{All: 9, Teachers: 4, Students: 4, Admins: 1, Disabled: 2}) {
			t.Errorf("%s: facets = %+v, want the same counts whichever tab is shown", tab, got.Facets)
		}
	}
}

func TestAnAdminHoldingTakeTestsIsNeverAStudentTarget(t *testing.T) {
	w := newReadWorld(t)
	w.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key)
		SELECT id, 'learning.take_tests' FROM app.roles WHERE builtin_key = 'admin'
		ON CONFLICT DO NOTHING`)
	admin := w.user(t, "admin", "admin")

	students := w.mine(t, domain.AdminUserQuery{Tab: domain.TabStudents})
	admins := w.mine(t, domain.AdminUserQuery{Tab: domain.TabAdmins})

	if len(students.Items) != 0 || students.Facets.Students != 0 {
		t.Errorf("students tab holds %v (facet %d), want none", accountIDs(students.Items), students.Facets.Students)
	}
	sameSet(t, "admins tab", accountIDs(admins.Items), admin)
}

func TestTheNewestAccountComesFirstAndAPageIsChosenBeforeItsNeighbours(t *testing.T) {
	w := newReadWorld(t)
	var created []string
	for i := range 25 {
		created = append(created, w.user(t, fmt.Sprintf("n%02d", i), "student"))
	}
	newestFirst := slices.Clone(created)
	slices.Reverse(newestFirst)

	var seen []string
	for page, want := range map[int]int{1: 10, 2: 10, 3: 5, 4: 0} {
		got := w.mine(t, domain.AdminUserQuery{Page: page, Limit: 10})
		if len(got.Items) != want || got.Page.Total != 25 || got.Page.Number != page || got.Page.Size != 10 {
			t.Errorf("page %d: %d item(s), page = %+v, want %d of 25", page, len(got.Items), got.Page, want)
		}
		if page == 1 && !slices.Equal(accountIDs(got.Items), newestFirst[:10]) {
			t.Errorf("page 1 = %v, want the ten newest first", accountIDs(got.Items))
		}
		seen = append(seen, accountIDs(got.Items)...)
	}
	sameSet(t, "the pages together", seen, created...)

	if got := w.mine(t, domain.AdminUserQuery{}); len(got.Items) != 20 || got.Page.Size != 20 {
		t.Errorf("default page = %d item(s) of size %d, want 20", len(got.Items), got.Page.Size)
	}
}

func TestAStatusNarrowsToThatStateAndNeverReachesADisabledAccount(t *testing.T) {
	w := newReadWorld(t)
	mustChange := w.user(t, "must", "student")
	w.exec(t, `UPDATE app.users SET password_hash = 'x', must_change_password = true WHERE id = $1::uuid`, mustChange)
	never := w.user(t, "never", "student")
	active := w.user(t, "active", "student")
	w.exec(t, `UPDATE app.users SET last_sign_in_at = now() WHERE id = $1::uuid`, active)
	goneMust := w.user(t, "gone", "student")
	w.exec(t, `UPDATE app.users SET password_hash = 'x', must_change_password = true, disabled_at = now() WHERE id = $1::uuid`, goneMust)

	for status, want := range map[domain.AccountStatus][]string{
		domain.AccountMustChangePassword: {mustChange},
		domain.AccountNeverSignedIn:      {never},
	} {
		got := w.mine(t, domain.AdminUserQuery{Status: status})
		sameSet(t, string(status), accountIDs(got.Items), want...)
		if got.Facets.All != len(want) || got.Facets.Disabled != 0 {
			t.Errorf("%s: facets = %+v, want the tabs counted under the status, the disabled one out", status, got.Facets)
		}
		if got.Items[0].Status != status {
			t.Errorf("%s: row status = %s", status, got.Items[0].Status)
		}
	}
	all := w.mine(t, domain.AdminUserQuery{})
	statuses := map[string]domain.AccountStatus{}
	for _, item := range all.Items {
		statuses[item.ID] = item.Status
	}
	want := map[string]domain.AccountStatus{mustChange: domain.AccountMustChangePassword, never: domain.AccountNeverSignedIn, active: domain.AccountActive, goneMust: domain.AccountDisabled}
	for id, status := range want {
		if statuses[id] != status {
			t.Errorf("account %s is %s, want %s", id, statuses[id], status)
		}
	}
}

func TestSearchIgnoresCaseAndAccentsOnTheNameAndTheEmailTogether(t *testing.T) {
	w := newReadWorld(t)
	token := nonce(t)
	huong := w.named(t, "Nguyễn Thị Hương "+token, "huong."+token+"@"+w.marker, "student")
	dang := w.named(t, "Đặng Văn Bình "+token, "binh."+token+"@"+w.marker, "teacher")
	w.named(t, "Trần Minh Khôi "+token+"x", "khoi."+token+"x@"+w.marker, "student")

	for name, tc := range map[string]struct {
		q    string
		want []string
	}{
		"without accents":                {"nguyen thi huong " + token, []string{huong}},
		"with accents":                   {"Nguyễn Thị Hương " + token, []string{huong}},
		"the accented capital Đ":         {"dang van binh " + token, []string{dang}},
		"capitals without accents":       {strings.ToUpper("nguyen thi huong ") + token, []string{huong}},
		"part of the email":              {"huong." + token, []string{huong}},
		"the email domain and a token":   {token + "@" + w.marker, []string{huong, dang}},
		"nothing":                        {token + "-no-such-person", nil},
		"a percent sign is a percent":    {token + "%", nil},
		"an underscore is an underscore": {"huong_" + token, nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := w.list(t, domain.AdminUserQuery{Query: tc.q})
			sameSet(t, "matches", accountIDs(got.Items), tc.want...)
			if got.Facets.All != len(tc.want) {
				t.Errorf("facets.all = %d, want the tabs counted under the search: %d", got.Facets.All, len(tc.want))
			}
		})
	}

	tokenOnly := w.list(t, domain.AdminUserQuery{Query: token, Tab: domain.TabStudents})
	if len(tokenOnly.Items) != 2 || tokenOnly.Facets.Students != 2 || tokenOnly.Facets.Teachers != 1 || tokenOnly.Facets.All != 3 {
		t.Errorf("search %q in the students tab: %d item(s), facets %+v, want 2 students and the other tabs' counts beside them", token, len(tokenOnly.Items), tokenOnly.Facets)
	}
}

func TestAnEmailIsMatchedWholeWhateverItsCase(t *testing.T) {
	w := newReadWorld(t)
	address := "Mai." + nonce(t) + "@" + w.marker
	mai := w.named(t, "Lê Mai", address, "student")

	for name, tc := range map[string]struct {
		email string
		want  []string
	}{
		"as stored":         {address, []string{mai}},
		"in capitals":       {strings.ToUpper(address), []string{mai}},
		"in lower case":     {strings.ToLower(address), []string{mai}},
		"its start":         {address[:6], nil},
		"without its local": {"@" + w.marker, nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := w.list(t, domain.AdminUserQuery{Email: tc.email})
			sameSet(t, "matches", accountIDs(got.Items), tc.want...)
		})
	}
}

func TestAnAccountsClassesAreTheOnesItJoinedOrTeachesAndNeverTheArchived(t *testing.T) {
	w := newReadWorld(t)
	teacher := w.user(t, "teacher", "teacher")
	student := w.user(t, "student", "student")
	loner := w.user(t, "loner", "student")
	admin := w.user(t, "admin", "admin")

	class := func(name, teacherID string, daysAgo int, archived bool) string {
		return w.id(t, `INSERT INTO app.classes (name, teacher_id, created_at, archived_at)
			VALUES ($1, $2::uuid, now() - make_interval(days => $3), CASE WHEN $4 THEN now() END) RETURNING id::text`, name, teacherID, daysAgo, archived)
	}
	join := func(classID, userID string, daysAgo int) {
		w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by, joined_at)
			VALUES ($1::uuid, $2::uuid, 'admin', $2::uuid, now() - make_interval(days => $3))`, classID, userID, daysAgo)
	}
	older := class("Lớp cũ", teacher, 30, false)
	newer := class("Lớp mới", teacher, 5, false)
	retired := class("Lớp đã lưu trữ", teacher, 60, true)
	mine := class("Lớp của quản trị", admin, 1, false)
	join(newer, student, 20)
	join(older, student, 10)
	join(retired, student, 90)
	join(older, teacher, 1)
	w.exec(t, `UPDATE app.classes SET teacher_id = $1::uuid WHERE id = $2::uuid`, student, mine)

	rows := map[string]domain.AdminUser{}
	for _, item := range w.mine(t, domain.AdminUserQuery{}).Items {
		rows[item.ID] = item
	}
	for name, tc := range map[string]struct {
		id    string
		count int
		first string
	}{
		"a teacher counts the classes they teach, the one they joined aside": {teacher, 2, "Lớp cũ"},
		"a student counts the classes they joined, the one they teach aside": {student, 2, "Lớp mới"},
		"a student in no class":       {loner, 0, ""},
		"an admin with no class left": {admin, 0, ""},
	} {
		got := rows[tc.id]
		first := ""
		if got.FirstClass != nil {
			first = *got.FirstClass
		}
		if got.ClassCount != tc.count || first != tc.first {
			t.Errorf("%s: %d class(es), first %q, want %d, %q", name, got.ClassCount, first, tc.count, tc.first)
		}
	}

	w.exec(t, `UPDATE app.classes SET teacher_id = $1::uuid WHERE id = $2::uuid`, admin, mine)
	for _, item := range w.mine(t, domain.AdminUserQuery{Tab: domain.TabAdmins}).Items {
		if item.ClassCount != 1 || item.FirstClass == nil || *item.FirstClass != "Lớp của quản trị" {
			t.Errorf("an Admin who teaches a class: %d, %v, want 1, the class", item.ClassCount, item.FirstClass)
		}
	}
}

func TestTheRowCarriesTheRoleAndHowTheAccountSignsIn(t *testing.T) {
	w := newReadWorld(t)
	both := w.user(t, "both", "teacher")
	w.exec(t, `UPDATE app.users SET password_hash = 'x', last_active_at = now() WHERE id = $1::uuid`, both)
	w.exec(t, `INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link) VALUES ($1::uuid, 'google', $2, $3)`, both, "sub-"+nonce(t), w.email("google"))
	bare := w.user(t, "bare", "student")

	rows := map[string]domain.AdminUser{}
	for _, item := range w.mine(t, domain.AdminUserQuery{}).Items {
		rows[item.ID] = item
	}
	if r := rows[both]; !r.HasPassword || !r.HasGoogle || r.Role.BuiltinKey != "teacher" || r.Role.Name == "" || r.Role.Icon == "" || r.Role.Color == "" || r.LastActiveAt == nil {
		t.Errorf("both = %+v", r)
	}
	if r := rows[bare]; r.HasPassword || r.HasGoogle || r.LastActiveAt != nil || r.Role.BuiltinKey != "student" {
		t.Errorf("bare = %+v", r)
	}
}

func TestTheSheetNamesTheCreatorTheClassesAndWhatTheAccountOwns(t *testing.T) {
	w := newReadWorld(t)
	creator := w.user(t, "creator", "teacher")
	owner := w.user(t, "owner", "teacher")
	w.exec(t, `UPDATE app.users SET created_by = $1::uuid WHERE id = $2::uuid`, creator, owner)

	for i := range 22 {
		w.id(t, `INSERT INTO app.classes (name, teacher_id, created_at) VALUES ($1, $2::uuid, now() - make_interval(mins => $3)) RETURNING id::text`, fmt.Sprintf("Lớp %02d", i), owner, 100-i)
	}
	w.exec(t, `INSERT INTO app.classes (name, teacher_id, archived_at) VALUES ('Lớp lưu trữ', $1::uuid, now())`, owner)
	live := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề sống', 'published', 1, $1::uuid, $1::uuid) RETURNING id::text`, owner)
	w.exec(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id, deleted_at) VALUES ('Đề đã xoá', 'draft', 0, $1::uuid, $1::uuid, now())`, owner)
	w.exec(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer) VALUES ('short_answer', 'Câu hỏi', 1, $1::uuid, $1::uuid, 'Đáp án')`, owner)
	w.exec(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer, deleted_at) VALUES ('short_answer', 'Câu đã xoá', 1, $1::uuid, $1::uuid, 'Đáp án', now())`, owner)
	media := func(deleted bool) {
		w.exec(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id, deleted_at)
			VALUES ('audio', $1, 'audio/mpeg', 128, 1000, 'a.mp3', sha256(convert_to($1::text, 'UTF8')), $2::uuid, $2::uuid, CASE WHEN $3 THEN now() END)`, "rd/"+uuid.NewString(), owner, deleted)
	}
	media(false)
	media(true)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, 1, $2::uuid) RETURNING id::text`, live, owner)
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		VALUES ($1::uuid, $2::uuid, now() - interval '1 hour', now() + interval '1 hour', 30, $3::uuid, now()) RETURNING id::text`, live, version, owner)

	got, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: domain.WriteRequest{ActorID: creator, All: true, Grants: access.NewSet(access.All()...)}, ID: owner})
	if err != nil {
		t.Fatal(err)
	}
	d := got.Detail
	if d.CreatedBy == nil || d.CreatedBy.ID != creator || d.CreatedBy.FullName != "Người dùng creator" {
		t.Errorf("createdBy = %+v, want the creator", d.CreatedBy)
	}
	if d.User.ClassCount != 22 || len(d.Classes) != domain.MaxUserClasses || d.Classes[0].Name != "Lớp 00" {
		t.Errorf("classes = %d listed of %d, first %v, want %d of 22 in the order firstName is drawn from", len(d.Classes), d.User.ClassCount, d.Classes[0], domain.MaxUserClasses)
	}
	if d.Owned != (domain.OwnedContent{Classes: 23, Tests: 1, Questions: 1, Media: 1, Assignments: 1}) {
		t.Errorf("owned = %+v, want 23 classes (the archived one too), and one each of what was not deleted", d.Owned)
	}
	_ = assignment

	none, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: domain.WriteRequest{ActorID: owner, All: true, Grants: access.NewSet(access.All()...)}, ID: creator})
	if err != nil {
		t.Fatal(err)
	}
	if none.Detail.CreatedBy != nil || none.Detail.Owned != (domain.OwnedContent{Classes: 0}) || len(none.Detail.Classes) != 0 || len(none.Detail.Events) != 0 {
		t.Errorf("an account that owns nothing: %+v", none.Detail)
	}
}

func TestTheSheetCountsTheTestsAStudentIsTakingAndListsTheNewestActivityFirst(t *testing.T) {
	w := newReadWorld(t)
	teacher := w.user(t, "teacher", "teacher")
	student := w.user(t, "student", "student")
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề kiểm tra', 'published', 1, $1::uuid, $1::uuid) RETURNING id::text`, teacher)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, 1, $2::uuid) RETURNING id::text`, test, teacher)
	assignment := func() string {
		return w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
			VALUES ($1::uuid, $2::uuid, now() - interval '3 days', now() + interval '3 days', 30, $3::uuid, now()) RETURNING id::text`, test, version, teacher)
	}
	attempt := func(assignmentID, status string, submittedAgo *int) {
		w.exec(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		                                     started_at, deadline_at, submitted_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::app.attempt_status, gen_random_uuid(), 1, sha256('b'::bytea),
			        now() - interval '2 days', now() + interval '1 day', CASE WHEN $5::int IS NULL THEN NULL ELSE now() - make_interval(hours => $5::int) END)`,
			assignmentID, version, student, status, submittedAgo)
	}
	hours := 5
	attempt(assignment(), "submitted", &hours)
	attempt(assignment(), "in_progress", nil)

	classID := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp 10A', $1::uuid) RETURNING id::text`, teacher)
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by, joined_at) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid, now() - interval '10 days')`, classID, student, teacher)
	w.exec(t, `UPDATE app.users SET last_sign_in_at = now() - interval '1 hour', last_sign_in_method = 'google' WHERE id = $1::uuid`, student)
	for i := range 10 {
		w.exec(t, `INSERT INTO app.audit_log (actor_user_id, action, entity, occurred_at) VALUES ($1::uuid, 'user.avatar_set', 'user', now() - make_interval(mins => $2))`, student, 100+i)
	}

	got, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: domain.WriteRequest{ActorID: teacher, All: true, Grants: access.NewSet(access.All()...)}, ID: student})
	if err != nil {
		t.Fatal(err)
	}
	d := got.Detail
	if d.InProgressAttempts != 1 {
		t.Errorf("inProgressAttempts = %d, want 1: the submitted one is not in progress", d.InProgressAttempts)
	}
	if len(d.Events) != domain.MaxUserEvents {
		t.Fatalf("%d event(s), want the cap of %d: %+v", len(d.Events), domain.MaxUserEvents, d.Events)
	}
	for i := 1; i < len(d.Events); i++ {
		if d.Events[i].At.After(d.Events[i-1].At) {
			t.Errorf("events are not newest first at %d: %v after %v", i, d.Events[i].At, d.Events[i-1].At)
		}
	}
	first := d.Events[0]
	if first.Kind != domain.EventSignIn || first.Detail == nil || *first.Detail != "google" {
		t.Errorf("first event = %+v, want the latest sign-in with its method", first)
	}
	byKind := map[domain.UserEventKind]string{}
	for _, e := range d.Events {
		if e.Detail != nil {
			byKind[e.Kind] = *e.Detail
		}
	}
	if byKind[domain.EventSubmission] != "" && byKind[domain.EventSubmission] != "Đề kiểm tra" {
		t.Errorf("submission detail = %q", byKind[domain.EventSubmission])
	}
	if byKind[domain.EventAudit] != "user.avatar_set" {
		t.Errorf("audit detail = %q, want the action", byKind[domain.EventAudit])
	}
	if d.Classes[0].Name != "Lớp 10A" {
		t.Errorf("classes = %+v", d.Classes)
	}
}

func TestEachKindOfActivityShowsWhenItIsTheNewest(t *testing.T) {
	w := newReadWorld(t)
	teacher := w.user(t, "teacher", "teacher")
	student := w.user(t, "student", "student")
	test := w.id(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề nộp bài', 'published', 1, $1::uuid, $1::uuid) RETURNING id::text`, teacher)
	version := w.id(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, 1, $2::uuid) RETURNING id::text`, test, teacher)
	assignment := w.id(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		VALUES ($1::uuid, $2::uuid, now() - interval '3 days', now() + interval '3 days', 30, $3::uuid, now()) RETURNING id::text`, test, version, teacher)
	w.exec(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at, submitted_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'submitted', gen_random_uuid(), 1, sha256('b'::bytea), now() - interval '2 days', now() + interval '1 day', now() - interval '3 hours')`, assignment, version, student)
	classID := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp tham gia', $1::uuid) RETURNING id::text`, teacher)
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by, joined_at) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid, now() - interval '5 hours')`, classID, student, teacher)
	w.exec(t, `UPDATE app.users SET last_sign_in_at = now() - interval '4 hours', last_sign_in_method = 'password' WHERE id = $1::uuid`, student)
	w.exec(t, `INSERT INTO app.audit_log (actor_user_id, action, entity, occurred_at) VALUES ($1::uuid, 'user.avatar_set', 'user', now() - interval '6 hours')`, student)

	got, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: domain.WriteRequest{ActorID: teacher, All: true, Grants: access.NewSet(access.All()...)}, ID: student})
	if err != nil {
		t.Fatal(err)
	}
	var kinds, details []string
	for _, e := range got.Detail.Events {
		kinds = append(kinds, string(e.Kind))
		details = append(details, *e.Detail)
	}
	if !slices.Equal(kinds, []string{"submission", "sign_in", "class_joined", "audit"}) ||
		!slices.Equal(details, []string{"Đề nộp bài", "password", "Lớp tham gia", "user.avatar_set"}) {
		t.Errorf("events = %v %v, want the submission, the sign-in, the join and the audit entry, newest first, each with its subject", kinds, details)
	}
}

func TestTheLastActiveAdminIsKnownToTheDirectory(t *testing.T) {
	w := newReadWorld(t)
	first := w.user(t, "admin-1", "admin")
	w.exec(t, `UPDATE app.users SET disabled_at = now()
		WHERE role_id = (SELECT id FROM app.roles WHERE builtin_key = 'admin') AND id <> $1::uuid AND disabled_at IS NULL`, first)
	by := domain.WriteRequest{ActorID: first, All: true, Grants: access.NewSet(access.All()...)}

	lastAdmin := func(id string) bool {
		got, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: by, ID: id})
		if err != nil {
			t.Fatal(err)
		}
		return got.Detail.LastActiveAdmin
	}
	if !lastAdmin(first) {
		t.Error("the only active Admin is not reported as the last one")
	}

	second := w.user(t, "admin-2", "admin")
	if lastAdmin(first) || lastAdmin(second) {
		t.Error("two active Admins: neither is the last")
	}

	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, second)
	if !lastAdmin(first) {
		t.Error("with the other Admin disabled, the first is the last one again")
	}
	if lastAdmin(second) {
		t.Error("a disabled Admin is never the last active one")
	}

	got, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{Request: domain.WriteRequest{ActorID: second, All: true, Grants: access.NewSet(access.All()...)}, ID: first})
	if err != nil {
		t.Fatal(err)
	}
	if got.Can.Disable || got.Can.ChangeRole {
		t.Errorf("can = %+v, want the last active Admin neither disabled nor demoted", got.Can)
	}
}

func TestAnIdNoOneHoldsIsNotFound(t *testing.T) {
	w := newReadWorld(t)
	_, err := w.app.Queries.GetUser.Handle(context.Background(), query.GetUser{
		Request: domain.WriteRequest{ActorID: uuid.NewString(), All: true, Grants: access.NewSet(access.All()...)}, ID: uuid.NewString()})
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}
