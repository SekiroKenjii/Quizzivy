//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
)

type who struct {
	created    *time.Time
	signedIn   bool
	disabled   bool
	mustChange bool
}

func builtin(t *testing.T, tx pgx.Tx, key string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(), `SELECT id::text FROM app.roles WHERE builtin_key = $1`, key).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func customRole(t *testing.T, tx pgx.Tx, permissions ...string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Vai trò "+nonce(t)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, key := range permissions {
		if _, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, $2)`, id, key); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func person(t *testing.T, tx pgx.Tx, roleID, name string, w who) string {
	t.Helper()
	var id string
	err := tx.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, disabled_at, last_sign_in_at, created_at)
		VALUES ($1, $2, $3, CASE WHEN $4 THEN 'hash' END, $4,
		        CASE WHEN $5 THEN now() END, CASE WHEN $6 THEN now() END, coalesce($7::timestamptz, now()))
		RETURNING id::text`,
		"admin-read-"+nonce(t)+"@example.test", name, roleID, w.mustChange, w.disabled, w.signedIn, w.created).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

func at(year int, month time.Month, d int) *time.Time {
	v := time.Date(year, month, d, 12, 0, 0, 0, time.UTC)
	return &v
}

func TestAccountsClassEveryUserByRoleAndCountTheEnabledByDG54(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()
	since := time.Now().Add(-time.Hour)

	before, err := store.Accounts(ctx, since)
	if err != nil {
		t.Fatal(err)
	}

	teacher, admin, student := builtin(t, tx, "teacher"), builtin(t, tx, "admin"), builtin(t, tx, "student")
	staffRole := customRole(t, tx, "learning.take_tests", "content.tests.write")
	learnerRole := customRole(t, tx, "learning.take_tests")
	emptyRole := customRole(t, tx)

	person(t, tx, teacher, "Cô Lan", who{signedIn: true})
	person(t, tx, teacher, "Thầy Nam", who{disabled: true, created: at(2001, time.March, 1)})
	person(t, tx, staffRole, "Cô Mai", who{mustChange: true})
	person(t, tx, teacher, "Thầy Hùng", who{created: at(2002, time.March, 1)})
	person(t, tx, admin, "Quản trị", who{signedIn: true})
	person(t, tx, student, "Học viên cũ", who{created: at(2001, time.January, 1)})
	person(t, tx, learnerRole, "Học viên vai trò", who{disabled: true, created: at(2001, time.June, 1)})
	person(t, tx, student, "Học viên mới", who{signedIn: true})
	person(t, tx, emptyRole, "Học viên không quyền", who{mustChange: true})

	after, err := store.Accounts(ctx, since)
	if err != nil {
		t.Fatal(err)
	}
	delta := domain.Accounts{
		Total:              after.Total - before.Total,
		Students:           after.Students - before.Students,
		StudentsNew:        after.StudentsNew - before.StudentsNew,
		Teachers:           after.Teachers - before.Teachers,
		TeachersDisabled:   after.TeachersDisabled - before.TeachersDisabled,
		MustChangePassword: after.MustChangePassword - before.MustChangePassword,
		NeverSignedIn:      after.NeverSignedIn - before.NeverSignedIn,
	}
	want := domain.Accounts{Total: 9, Students: 4, StudentsNew: 2, Teachers: 4, TeachersDisabled: 1, MustChangePassword: 2, NeverSignedIn: 2}
	if delta != want {
		t.Errorf("accounts delta = %+v, want %+v", delta, want)
	}
	oldest := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.UTC)
	if after.OldestNeverSignedIn == nil || !after.OldestNeverSignedIn.Equal(oldest) {
		t.Errorf("oldest never signed in = %v, want %v", after.OldestNeverSignedIn, oldest)
	}

	mustChange, err := store.MustChangePassword(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mustChange != after.MustChangePassword {
		t.Errorf("MustChangePassword = %d, Accounts counted %d", mustChange, after.MustChangePassword)
	}
}

func TestADisabledAccountIsNeitherOnATemporaryPasswordNorNeverSignedIn(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()
	before, err := store.Accounts(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	person(t, tx, builtin(t, tx, "teacher"), "Thầy đã khóa", who{disabled: true, mustChange: true})

	after, err := store.Accounts(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if after.MustChangePassword != before.MustChangePassword || after.NeverSignedIn != before.NeverSignedIn {
		t.Errorf("a disabled account counted: before %+v after %+v", before, after)
	}
	if after.TeachersDisabled != before.TeachersDisabled+1 || after.Teachers != before.Teachers+1 {
		t.Errorf("a disabled teacher is still a teacher: before %+v after %+v", before, after)
	}
}

func TestActivityIsCountedPerLocalDayAcrossAMonthBoundary(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()

	studentRole := builtin(t, tx, "student")
	teacher, admin := builtin(t, tx, "teacher"), builtin(t, tx, "admin")
	s1 := person(t, tx, studentRole, "Học viên 1", who{})
	s2 := person(t, tx, customRole(t, tx, "learning.take_tests"), "Học viên 2", who{})
	t1 := person(t, tx, teacher, "Cô Lan", who{})
	a1 := person(t, tx, admin, "Quản trị", who{})

	beforeJan, err := store.ActiveSince(ctx, day(2001, time.January, 31))
	if err != nil {
		t.Fatal(err)
	}
	beforeFeb, err := store.ActiveSince(ctx, day(2001, time.February, 1))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range []struct {
		user string
		on   time.Time
	}{
		{s1, day(2001, time.January, 31)}, {t1, day(2001, time.January, 31)},
		{s1, day(2001, time.February, 1)}, {s2, day(2001, time.February, 1)}, {a1, day(2001, time.February, 1)},
	} {
		if _, err := tx.Exec(ctx, `INSERT INTO app.user_activity_days (user_id, day) VALUES ($1, $2)`, row.user, row.on); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.ActiveByDay(ctx, day(2001, time.January, 28), day(2001, time.February, 3))
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.ActiveDay{
		{Date: day(2001, time.January, 31), Students: 1, Staff: 1},
		{Date: day(2001, time.February, 1), Students: 2, Staff: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("days = %+v, want %+v", got, want)
	}
	for i := range want {
		if !got[i].Date.Equal(want[i].Date) || got[i].Students != want[i].Students || got[i].Staff != want[i].Staff {
			t.Errorf("day %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	afterJan, err := store.ActiveSince(ctx, day(2001, time.January, 31))
	if err != nil {
		t.Fatal(err)
	}
	afterFeb, err := store.ActiveSince(ctx, day(2001, time.February, 1))
	if err != nil {
		t.Fatal(err)
	}
	if afterJan-beforeJan != 4 {
		t.Errorf("active since the 31st: +%d, want +4 distinct accounts", afterJan-beforeJan)
	}
	if afterFeb-beforeFeb != 3 {
		t.Errorf("active since the 1st: +%d, want +3 distinct accounts", afterFeb-beforeFeb)
	}
}

func TestClassesAreCountedOpenAndArchivedAndTheNewestOfADisabledTeacherIsNamed(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()
	counts, err := store.ClassCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := store.DisabledTeacherClasses(ctx)
	if err != nil {
		t.Fatal(err)
	}

	teacher := builtin(t, tx, "teacher")
	active := person(t, tx, teacher, "Cô Lan", who{signedIn: true})
	gone := person(t, tx, teacher, "Thầy Nam "+nonce(t), who{disabled: true})
	class := func(owner, name string, created string, archived bool) {
		t.Helper()
		_, err := tx.Exec(ctx, `INSERT INTO app.classes (name, teacher_id, created_at, archived_at)
			VALUES ($1, $2, $3::timestamptz, CASE WHEN $4 THEN now() END)`, name, owner, created, archived)
		if err != nil {
			t.Fatal(err)
		}
	}
	class(active, "Lớp đang học", "2030-01-01", false)
	class(active, "Lớp đã lưu trữ", "2030-01-02", true)
	class(gone, "Lớp cũ", "2031-01-01", false)
	class(gone, "Lớp mới nhất", "2032-01-01", true)

	got, err := store.ClassCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active-counts.Active != 2 || got.Archived-counts.Archived != 2 {
		t.Errorf("class counts moved by active +%d archived +%d, want +2 and +2", got.Active-counts.Active, got.Archived-counts.Archived)
	}

	hit, err := store.DisabledTeacherClasses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hit.Count-disabled.Count != 2 {
		t.Errorf("classes of a disabled teacher moved by %d, want 2, archived ones included", hit.Count-disabled.Count)
	}
	if hit.ClassName != "Lớp mới nhất" || hit.TeacherName == "" {
		t.Errorf("newest = %q of %q, want the archived newest class with its teacher", hit.ClassName, hit.TeacherName)
	}
}

func TestHeldCountsOnlyWhatIsStillInStorage(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()
	before, err := store.Held(ctx)
	if err != nil {
		t.Fatal(err)
	}

	owner := person(t, tx, builtin(t, tx, "teacher"), "Cô Lan", who{signedIn: true})
	media := func(bytes int64, purged bool) {
		t.Helper()
		_, err := tx.Exec(ctx, `
			INSERT INTO app.media_assets (kind, display_name, original_filename, mime_type, bytes, checksum_sha256, storage_key, uploaded_by, owner_id, unreferenced_at, purged_at)
			VALUES ('image', 'Ảnh', 'a.png', 'image/png', $1, sha256($2::bytea), $2, $3, $3,
			        CASE WHEN $4 THEN now() END, CASE WHEN $4 THEN now() END)`,
			bytes, "media/"+nonce(t), owner, purged)
		if err != nil {
			t.Fatal(err)
		}
	}
	media(1000, false)
	media(250, false)
	media(5000, true)

	source := func(importID string, bytes int64) {
		t.Helper()
		_, err := tx.Exec(ctx, `
			INSERT INTO app.word_import_sources (import_id, upload_id, expected_revision, role, filename, format, bytes, checksum_sha256, storage_key, uploaded_by)
			VALUES ($1, gen_random_uuid(), 1, 'exam', 'de.docx', 'docx', $2, sha256($3::bytea), $3, $4)`,
			importID, bytes, "imports/"+nonce(t), owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	word := func(status string, removed bool) string {
		t.Helper()
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO app.word_imports (created_by, request_id, title, status, files_removed_at)
			VALUES ($1, gen_random_uuid(), 'Đề nhập', $2, CASE WHEN $3 THEN now() END) RETURNING id::text`, owner, status, removed).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	kept := word("awaiting_sources", false)
	source(kept, 3000)
	source(kept, 4000)
	source(word("failed", false), 500)
	source(word("cancelled", true), 9000)

	seed(t, tx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false)

	after, err := store.Held(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.MediaBytes - before.MediaBytes; got != 1250 {
		t.Errorf("media bytes moved by %d, want 1250: a purged file is gone", got)
	}
	if got := after.ImportBytes - before.ImportBytes; got != 7500 {
		t.Errorf("import bytes moved by %d, want 7500: an import whose files were removed holds none", got)
	}
	if got := after.ImportCount - before.ImportCount; got != 2 {
		t.Errorf("import count moved by %d, want 2 imports however many documents each holds", got)
	}
	if got := after.Attempts - before.Attempts; got != 1 {
		t.Errorf("attempts moved by %d, want 1", got)
	}
}
