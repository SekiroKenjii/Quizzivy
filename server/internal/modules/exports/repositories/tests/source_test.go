//go:build integration

package repositories_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"quizzivy/internal/modules/exports/domain"
)

type dump map[string][][]string

func (w *world) read(includeAnswers bool, during func(domain.Snapshot)) dump {
	w.t.Helper()
	out := dump{}
	err := w.repo.Read(context.Background(), func(s domain.Snapshot) error {
		for _, file := range domain.Files(includeAnswers) {
			err := s.Rows(context.Background(), file, includeAnswers, func(cells []string) error {
				if len(cells) != len(file.Columns) {
					w.t.Errorf("%s: a row of %d cells for %d columns", file.Name, len(cells), len(file.Columns))
				}
				out[file.Name] = append(out[file.Name], append([]string(nil), cells...))
				return nil
			})
			if err != nil {
				return err
			}
			if during != nil && file.Name == domain.UsersFile {
				during(s)
			}
		}
		return nil
	})
	if err != nil {
		w.t.Fatalf("read the snapshot: %v", err)
	}
	return out
}

func (d dump) rowWith(file string, column int, value string) []string {
	for _, row := range d[file] {
		if row[column] == value {
			return row
		}
	}
	return nil
}

func column(t *testing.T, file, name string) int {
	t.Helper()
	for _, f := range domain.Files(true) {
		if f.Name != file {
			continue
		}
		for i, c := range f.Columns {
			if c.Name == name {
				return i
			}
		}
	}
	t.Fatalf("%s has no column %s", file, name)
	return -1
}

var isoUTC = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

func TestEveryQueryReturnsTheColumnsOfItsFile(t *testing.T) {
	w := newWorld(t)
	got := w.read(true, nil)
	if len(got[domain.RolesFile]) < 4 {
		t.Errorf("roles.csv holds %d rows, but the four built-in roles always exist", len(got[domain.RolesFile]))
	}
}

func TestAQueryThatDoesNotReturnItsFilesColumnsFails(t *testing.T) {
	w := newWorld(t)
	users := domain.Files(false)[0]
	swapped := domain.File{Name: users.Name, VI: users.VI, EN: users.EN, Columns: append([]domain.Column(nil), users.Columns...)}
	swapped.Columns[0], swapped.Columns[1] = swapped.Columns[1], swapped.Columns[0]
	extra := domain.File{Name: users.Name, VI: users.VI, EN: users.EN, Columns: append(append([]domain.Column(nil), users.Columns...), domain.Column{Name: "password_hash"})}
	unknown := domain.File{Name: "payments.csv", Columns: users.Columns}
	for name, file := range map[string]domain.File{"swapped": swapped, "extra": extra, "unknown": unknown} {
		err := w.repo.Read(context.Background(), func(s domain.Snapshot) error {
			return s.Rows(context.Background(), file, false, func([]string) error { return nil })
		})
		if err == nil {
			t.Errorf("%s: a file the query does not match was streamed", name)
		}
	}
	err := w.repo.Read(context.Background(), func(s domain.Snapshot) error {
		return s.Rows(context.Background(), domain.Files(true)[len(domain.Files(true))-1], false, func([]string) error { return nil })
	})
	if err == nil {
		t.Error("answers were streamed for an export that did not ask for them")
	}
}

func TestUsersAreExportedWithoutTheirSecrets(t *testing.T) {
	w := newWorld(t)
	var id string
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, display_name, phone, role_id, password_hash, locale, time_zone, must_change_password, disabled_at, last_sign_in_method)
		VALUES ($1, $2, 'An', '0901234567', (SELECT id FROM app.roles WHERE builtin_key = 'student'),
		        'argon2id$SECRET-HASH-9f3', 'vi', 'Asia/Ho_Chi_Minh', true, '2026-10-01 03:04:05+00', 'password')
		RETURNING id::text`, "bi-mat-"+nonce(t)+"@example.com", `=HYPERLINK("http://x")`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
			t.Errorf("removing the user: %v", err)
		}
	})

	got := w.read(true, nil)
	row := got.rowWith(domain.UsersFile, column(t, domain.UsersFile, "id"), id)
	if row == nil {
		t.Fatal("the user is not in users.csv")
	}
	if name := row[column(t, domain.UsersFile, "full_name")]; name != `=HYPERLINK("http://x")` {
		t.Errorf("full_name = %q; the escaping belongs to the CSV writer, not to the query", name)
	}
	want := map[string]string{
		"display_name": "An", "phone": "0901234567", "must_change_password": "true",
		"locale": "vi", "time_zone": "Asia/Ho_Chi_Minh", "disabled_at": "2026-10-01T03:04:05Z",
		"last_sign_in_method": "password", "created_by": "", "last_active_at": "",
	}
	for name, v := range want {
		if got := row[column(t, domain.UsersFile, name)]; got != v {
			t.Errorf("%s = %q, want %q", name, got, v)
		}
	}
	if row[column(t, domain.UsersFile, "role_name")] == "" || row[column(t, domain.UsersFile, "role_id")] == "" {
		t.Errorf("the role of the user is missing: %v", row)
	}
	for _, name := range []string{"created_at", "disabled_at"} {
		if v := row[column(t, domain.UsersFile, name)]; !isoUTC.MatchString(v) {
			t.Errorf("%s = %q is not an ISO 8601 UTC time", name, v)
		}
	}
	for file, rows := range got {
		for _, cells := range rows {
			for _, cell := range cells {
				if strings.Contains(cell, "SECRET-HASH") {
					t.Fatalf("%s carries a password hash", file)
				}
			}
		}
	}
}

func TestRolesCarryTheirPermissionsSeparatedBySemicolons(t *testing.T) {
	w := newWorld(t)
	got := w.read(false, nil)
	row := got.rowWith(domain.RolesFile, column(t, domain.RolesFile, "builtin_key"), "teacher")
	if row == nil {
		t.Fatal("the teacher role is not in roles.csv")
	}
	permissions := row[column(t, domain.RolesFile, "permissions")]
	if !strings.Contains(permissions, ";") || strings.Contains(permissions, " ") || strings.Contains(permissions, ",") {
		t.Errorf("permissions = %q", permissions)
	}
	if custom := got.rowWith(domain.RolesFile, column(t, domain.RolesFile, "builtin_key"), ""); custom != nil && custom[column(t, domain.RolesFile, "name")] == "" {
		t.Error("a custom role has no name")
	}
}

func TestClassesAndTheirMembersAreExportedWithoutJoinCodes(t *testing.T) {
	w := newWorld(t)
	teacher := w.user("teacher", "Cô Hoa")
	student := w.user("student", "Em Lan")
	var class string
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.classes (name, teacher_id, schedule_label, room) VALUES ($1, $2::uuid, 'Thứ 2 và 4', 'P.201')
		RETURNING id::text`, "Lớp "+nonce(t), teacher).Scan(&class)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.classes WHERE id = $1::uuid`, class); err != nil {
			t.Errorf("removing the class: %v", err)
		}
	})
	w.exec(`INSERT INTO app.class_members (class_id, user_id, joined_via) VALUES ($1::uuid, $2::uuid, 'admin')`, class, student)

	got := w.read(false, nil)
	row := got.rowWith(domain.ClassesFile, column(t, domain.ClassesFile, "id"), class)
	if row == nil {
		t.Fatal("the class is not in classes.csv")
	}
	if row[column(t, domain.ClassesFile, "teacher_id")] != teacher || row[column(t, domain.ClassesFile, "teacher_name")] != "Cô Hoa" ||
		row[column(t, domain.ClassesFile, "room")] != "P.201" || row[column(t, domain.ClassesFile, "self_join_enabled")] != "true" ||
		row[column(t, domain.ClassesFile, "archived_at")] != "" {
		t.Errorf("class = %v", row)
	}
	member := got.rowWith(domain.ClassMembersFile, column(t, domain.ClassMembersFile, "user_id"), student)
	if member == nil || member[column(t, domain.ClassMembersFile, "class_id")] != class || member[column(t, domain.ClassMembersFile, "joined_via")] != "admin" {
		t.Errorf("member = %v", member)
	}
}

func TestAnExportDescribesOneMomentEvenWhileTheCentreKeepsWorking(t *testing.T) {
	w := newWorld(t)
	before := w.user("student", "Em Trước")
	var added string
	got := w.read(false, func(domain.Snapshot) {
		added = w.user("student", "Em Sau")
	})
	if got.rowWith(domain.UsersFile, column(t, domain.UsersFile, "id"), before) == nil {
		t.Error("a user that existed before the snapshot is missing")
	}
	if added == "" {
		t.Fatal("the concurrent write did not run")
	}
	if got.rowWith(domain.UsersFile, column(t, domain.UsersFile, "id"), added) != nil {
		t.Error("a user created while the snapshot was open appears in it")
	}
	if again := w.read(false, nil); again.rowWith(domain.UsersFile, column(t, domain.UsersFile, "id"), added) == nil {
		t.Error("the next snapshot does not see the user")
	}
}
