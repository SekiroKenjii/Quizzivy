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

func TestAccountsClassEveryUserByStudentTargetAndCountTheEnabledByDG54(t *testing.T) {
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
	person(t, tx, admin, "Quản trị phụ", who{signedIn: true})
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
		Enabled:            after.Enabled - before.Enabled,
		Students:           after.Students - before.Students,
		StudentsNew:        after.StudentsNew - before.StudentsNew,
		Teachers:           after.Teachers - before.Teachers,
		TeachersDisabled:   after.TeachersDisabled - before.TeachersDisabled,
		MustChangePassword: after.MustChangePassword - before.MustChangePassword,
		NeverSignedIn:      after.NeverSignedIn - before.NeverSignedIn,
	}
	want := domain.Accounts{Total: 10, Enabled: 8, Students: 4, StudentsNew: 2, Teachers: 5, TeachersDisabled: 1, MustChangePassword: 2, NeverSignedIn: 2}
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

func TestADisabledAccountIsNeitherATeacherNorASeatNorOnATemporaryPasswordNorNeverSignedIn(t *testing.T) {
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
	if after.TeachersDisabled != before.TeachersDisabled+1 || after.Teachers != before.Teachers || after.Total != before.Total+1 || after.Enabled != before.Enabled {
		t.Errorf("a disabled teacher is counted once, as disabled, and holds no seat: before %+v after %+v", before, after)
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
	s3 := person(t, tx, studentRole, "Học viên 3", who{})
	t1 := person(t, tx, teacher, "Cô Lan", who{})
	a1 := person(t, tx, admin, "Quản trị", who{})
	gone := person(t, tx, teacher, "Thầy đã khóa", who{disabled: true})

	week := func(from, to time.Time) int {
		t.Helper()
		n, err := store.ActiveBetween(ctx, from, to)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	jan31, feb1, feb3, feb5 := day(2001, time.January, 31), day(2001, time.February, 1), day(2001, time.February, 3), day(2001, time.February, 5)
	beforeAll, beforeJan, beforeFeb, beforeLate := week(jan31, feb3), week(jan31, jan31), week(feb1, feb3), week(feb5, feb5)

	for _, row := range []struct {
		user string
		on   time.Time
	}{
		{s1, jan31}, {t1, jan31},
		{s1, feb1}, {s2, feb1}, {a1, feb1}, {gone, feb1},
		{s3, feb5},
	} {
		if _, err := tx.Exec(ctx, `INSERT INTO app.user_activity_days (user_id, day) VALUES ($1, $2)`, row.user, row.on); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.ActiveByDay(ctx, day(2001, time.January, 28), feb3)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.ActiveDay{
		{Date: jan31, Students: 1, Staff: 1},
		{Date: feb1, Students: 2, Staff: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("days = %+v, want %+v", got, want)
	}
	for i := range want {
		if !got[i].Date.Equal(want[i].Date) || got[i].Students != want[i].Students || got[i].Staff != want[i].Staff {
			t.Errorf("day %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	for _, c := range []struct {
		name     string
		got, was int
		want     int
	}{
		{"the 31st to the 3rd, the account disabled since included and the 5th left out", week(jan31, feb3), beforeAll, 5},
		{"the 31st alone", week(jan31, jan31), beforeJan, 2},
		{"the 1st to the 3rd", week(feb1, feb3), beforeFeb, 4},
		{"the 5th alone", week(feb5, feb5), beforeLate, 1},
	} {
		if c.got-c.was != c.want {
			t.Errorf("active %s: +%d, want +%d distinct accounts", c.name, c.got-c.was, c.want)
		}
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
	before, err := store.Held(ctx, mayTenth)
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

	after, err := store.Held(ctx, mayTenth)
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
	if got := after.LargestLibrary; got < 1250 {
		t.Errorf("largest library = %d, want at least this teacher's 1250", got)
	}
	if got := after.OpenImportBytes - before.OpenImportBytes; got != 7500 {
		t.Errorf("open import bytes moved by %d, want 7500: the open and the failed import, not the cancelled one", got)
	}
	if after.DatabaseBytes <= 0 {
		t.Errorf("database bytes = %d, want the database's size", after.DatabaseBytes)
	}
}

var mayTenth = domain.EmailWindow{
	Now:        time.Date(2031, time.May, 10, 12, 0, 0, 0, time.UTC),
	DayStart:   time.Date(2031, time.May, 10, 0, 0, 0, 0, time.UTC),
	NextDay:    time.Date(2031, time.May, 11, 0, 0, 0, 0, time.UTC),
	MonthStart: time.Date(2031, time.May, 1, 0, 0, 0, 0, time.UTC),
	NextMonth:  time.Date(2031, time.June, 1, 0, 0, 0, 0, time.UTC),
}

func TestHeldCountsTheMailOfTheDayAndMonthAndWhatACapHolds(t *testing.T) {
	tx := isolated(t, newPool(t))
	store := repositories.NewPostgres(db.NewContext(tx))
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `DELETE FROM app.email_outbox WHERE sent_at IS NULL AND failed_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	before, err := store.Held(ctx, mayTenth)
	if err != nil {
		t.Fatal(err)
	}
	mail := func(sent, failed, deliver string, attempts int, held, code string) {
		t.Helper()
		_, err := tx.Exec(ctx, `
			INSERT INTO app.email_outbox (to_address, template, locale, sent_at, failed_at, deliver_after, attempts, held_by, last_error_code)
			VALUES ($1, 'sign_in_invite', 'vi', $2::timestamptz, $3::timestamptz, coalesce($4::timestamptz, now()), $5, $6, $7)`,
			nonce(t)+"@example.com", nullable(sent), nullable(failed), nullable(deliver), attempts, nullable(held), nullable(code))
		if err != nil {
			t.Fatal(err)
		}
	}
	mail("2031-05-10T08:00:00Z", "", "", 1, "", "")
	mail("2031-05-02T08:00:00Z", "", "", 1, "", "")
	mail("2031-04-30T23:00:00Z", "", "", 1, "", "")
	mail("", "", "2031-06-01T00:00:00Z", 0, "month", "")
	mail("", "", "2031-06-01T00:00:00Z", 2, "month", "")
	mail("", "", "2031-05-10T12:05:00Z", 2, "", "")
	mail("", "", "2031-05-11T00:00:00Z", 0, "", "")
	mail("", "2031-05-10T09:00:00Z", "2031-07-01T00:00:00Z", 0, "month", "expired")
	mail("", "2031-04-30T09:00:00Z", "2031-05-01T00:00:00Z", 0, "month", "expired")
	mail("", "2031-05-09T09:00:00Z", "2031-05-09T00:00:00Z", 0, "", "expired")
	mail("", "", "2031-05-10T07:00:00Z", 0, "", "")

	after, err := store.Held(ctx, mayTenth)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.EmailsToday - before.EmailsToday; got != 1 {
		t.Errorf("sent today moved by %d, want 1", got)
	}
	if got := after.EmailsMonth - before.EmailsMonth; got != 2 {
		t.Errorf("sent this month moved by %d, want 2: April's is not May's", got)
	}
	if after.EmailsHeld != 2 {
		t.Errorf("held = %d, want 2: only a row a cap marked is held, a retry or a row enqueued for later is not, and a failed row is settled", after.EmailsHeld)
	}
	if got := after.EmailsExpired - before.EmailsExpired; got != 1 {
		t.Errorf("expired while held moved by %d, want 1: April's and one never held do not count", got)
	}
	if after.HeldUntil == nil || !after.HeldUntil.Equal(mayTenth.NextMonth) {
		t.Errorf("held until = %v, want the 1st of June", after.HeldUntil)
	}
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
