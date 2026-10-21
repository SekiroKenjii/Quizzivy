//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

// guardWorld writes inside one transaction that is rolled back, so a limit it
// sets on the centre's one settings row is never seen by the packages that
// share the database.
type guardWorld struct {
	t        *testing.T
	tx       pgx.Tx
	marker   string
	actor    string
	student  role
	teacher  role
	admins   *repositories.AdminUsers
	students *repositories.Students
	imports  *repositories.UserImports
}

type role struct {
	id       string
	revision int64
}

func newGuardWorld(t *testing.T) *guardWorld {
	t.Helper()
	tx, err := newPool(t).BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(context.Background(), `SET LOCAL lock_timeout = '5s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SELECT 1 FROM app.organization_settings FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	dbx := db.NewContext(tx)
	w := &guardWorld{t: t, tx: tx, marker: "lim" + nonce(t) + ".example.test",
		admins: repositories.NewAdminUsers(dbx, nil), students: repositories.NewStudents(dbx), imports: repositories.NewUserImports(dbx)}
	w.student, w.teacher = w.role("student"), w.role("teacher")
	w.actor = w.account("admin", w.role("admin").id, false)
	w.account("gv-co-san", w.teacher.id, false)
	return w
}

func (w *guardWorld) role(key string) role {
	w.t.Helper()
	var r role
	if err := w.tx.QueryRow(context.Background(), `SELECT id::text, revision FROM app.roles WHERE builtin_key = $1`, key).Scan(&r.id, &r.revision); err != nil {
		w.t.Fatal(err)
	}
	return r
}

func (w *guardWorld) account(name, roleID string, disabled bool) string {
	w.t.Helper()
	var id string
	err := w.tx.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, disabled_at)
		VALUES ($1, $2, $3::uuid, CASE WHEN $4 THEN now() END) RETURNING id::text`,
		name+nonce(w.t)+"@"+w.marker, name, roleID, disabled).Scan(&id)
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *guardWorld) used() limits.Accounts {
	w.t.Helper()
	used, err := limits.CountAccounts(context.Background(), w.tx)
	if err != nil {
		w.t.Fatal(err)
	}
	return used
}

func (w *guardWorld) limit(name limits.Name, value int) {
	w.t.Helper()
	if _, err := w.tx.Exec(context.Background(), `UPDATE app.organization_settings SET `+limits.Column(name)+` = $1`, value); err != nil {
		w.t.Fatal(err)
	}
}

func (w *guardWorld) req() domain.WriteRequest {
	return domain.WriteRequest{ActorID: w.actor, All: true}
}

func (w *guardWorld) createUser(r role) error {
	_, err := w.admins.Create(context.Background(), w.req(), domain.NewUser{
		Email: "moi" + nonce(w.t) + "@" + w.marker, FullName: "Tài khoản mới", RoleID: r.id, RoleRevision: r.revision,
		Delivery: domain.DeliveryTemporaryPassword, Hash: "$test$hash", Now: time.Now(),
	}, nil)
	return err
}

func (w *guardWorld) settle(r role) (domain.ImportProgress, error) {
	w.t.Helper()
	ctx := context.Background()
	email := "nhap" + nonce(w.t) + "@" + w.marker
	var job string
	err := w.tx.QueryRow(ctx, `
		INSERT INTO app.user_import_jobs (created_by, request_id, filename, format, default_role_id, delivery, status, total_rows, attempts, started_at)
		VALUES ($1::uuid, gen_random_uuid(), 'nguoi-dung.csv', 'csv', $2::uuid, 'sheet', 'running', 1, 1, now()) RETURNING id::text`,
		w.actor, r.id).Scan(&job)
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.tx.Exec(ctx, `
		INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id) VALUES ($1::uuid, 0, 'Người nhập', $2, $3::uuid)`,
		job, email, r.id); err != nil {
		w.t.Fatal(err)
	}
	row := domain.ImportCreate{RoleRevision: r.revision, Hash: "$test$hash"}
	row.Ordinal, row.Email, row.FullName, row.RoleID = 0, email, "Người nhập", r.id
	return w.imports.Settle(ctx, domain.ImportBatch{JobID: job, Attempt: 1, By: w.actor, Delivery: domain.ImportSheet, Now: time.Now(), Rows: []domain.ImportCreate{row}}, nil)
}

func TestEveryWayToTakeASeatOrATeacherPlaceStopsAtTheLimit(t *testing.T) {
	cases := []struct {
		name  string
		limit limits.Name
		want  error
		take  func(w *guardWorld) error
	}{
		{"an Admin creates a student", limits.Seats, limits.ErrSeatLimit, func(w *guardWorld) error { return w.createUser(w.student) }},
		{"an Admin creates a teacher", limits.Teachers, limits.ErrTeacherLimit, func(w *guardWorld) error { return w.createUser(w.teacher) }},
		{"a teacher adds a student", limits.Seats, limits.ErrSeatLimit, func(w *guardWorld) error {
			_, err := w.students.Create(context.Background(), w.req(), domain.NewStudent{
				Email: "hv" + nonce(w.t) + "@" + w.marker, FullName: "Học viên mới", Hash: "$test$hash", Now: time.Now(),
			})
			return err
		}},
		{"an Admin enables an account", limits.Seats, limits.ErrSeatLimit, func(w *guardWorld) error {
			id := w.account("da-tat", w.student.id, true)
			return w.admins.Enable(context.Background(), w.req(), domain.AccountWrite{UserID: id, RoleID: w.student.id, Now: time.Now()})
		}},
		{"an Admin enables a teacher", limits.Teachers, limits.ErrTeacherLimit, func(w *guardWorld) error {
			id := w.account("gv-da-tat", w.teacher.id, true)
			return w.admins.Enable(context.Background(), w.req(), domain.AccountWrite{UserID: id, RoleID: w.teacher.id, Now: time.Now()})
		}},
		{"a student is made a teacher", limits.Teachers, limits.ErrTeacherLimit, func(w *guardWorld) error {
			id := w.account("len-gv", w.student.id, false)
			return w.admins.ChangeRole(context.Background(), w.req(), domain.RoleChange{
				AccountWrite: domain.AccountWrite{UserID: id, RoleID: w.student.id, Now: time.Now()},
				ToRoleID:     w.teacher.id, ToRoleRevision: w.teacher.revision, ToOpensTeacher: true,
			})
		}},
		{"an import row", limits.Seats, nil, func(w *guardWorld) error {
			progress, err := w.settle(w.student)
			if err == nil && progress.OverLimit == 1 && progress.Created == 0 && progress.Done {
				return limits.ErrSeatLimit
			}
			if err == nil && progress.Created == 1 {
				return nil
			}
			return errors.Join(err, errors.New("unexpected progress"))
		}},
		{"an import row of a teacher", limits.Teachers, nil, func(w *guardWorld) error {
			progress, err := w.settle(w.teacher)
			if err == nil && progress.OverLimit == 1 && progress.Created == 0 {
				return limits.ErrTeacherLimit
			}
			if err == nil && progress.Created == 1 {
				return nil
			}
			return errors.Join(err, errors.New("unexpected progress"))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full := newGuardWorld(t)
			used := full.used()
			full.limit(c.limit, map[limits.Name]int{limits.Seats: used.Seats, limits.Teachers: used.Teachers}[c.limit])
			before := full.used()
			err := c.take(full)
			want := c.want
			if want == nil {
				want = map[limits.Name]error{limits.Seats: limits.ErrSeatLimit, limits.Teachers: limits.ErrTeacherLimit}[c.limit]
			}
			if !errors.Is(err, want) {
				t.Errorf("at the limit: %v, want %v", err, want)
			}
			if after := full.used(); after.Teachers != before.Teachers || (c.limit == limits.Seats && after.Seats != before.Seats) {
				t.Errorf("at the limit the accounts moved from %+v to %+v", before, after)
			}

			if err := full.tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			room := newGuardWorld(t)
			now := room.used()
			room.limit(c.limit, map[limits.Name]int{limits.Seats: now.Seats, limits.Teachers: now.Teachers}[c.limit]+1)
			if err := c.take(room); err != nil {
				t.Errorf("with room for one: %v, want it taken", err)
			}
		})
	}
}

func TestAWriteThatTakesNoPlaceIsNeverRefused(t *testing.T) {
	w := newGuardWorld(t)
	used := w.used()
	w.limit(limits.Seats, used.Seats)
	w.limit(limits.Teachers, used.Teachers)
	disabled := w.account("da-tat", w.student.id, true)
	err := w.admins.ChangeRole(context.Background(), w.req(), domain.RoleChange{
		AccountWrite: domain.AccountWrite{UserID: disabled, RoleID: w.student.id, Now: time.Now()},
		ToRoleID:     w.teacher.id, ToRoleRevision: w.teacher.revision, ToOpensTeacher: true,
	})
	if err != nil {
		t.Errorf("a disabled account given a teacher role takes no place: %v", err)
	}
	teacher := w.account("gv", w.teacher.id, false)
	if _, err := w.tx.Exec(context.Background(), `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, teacher); err != nil {
		t.Fatal(err)
	}
	w.limit(limits.Seats, w.used().Seats)
	if err := w.admins.Disable(context.Background(), w.req(), domain.AccountWrite{UserID: w.account("con", w.student.id, false), RoleID: w.student.id, Now: time.Now()}); err != nil {
		t.Errorf("disabling frees a seat and is never refused: %v", err)
	}
}

func TestAnImportPreviewLeavesTheRoomTheLimitsAllow(t *testing.T) {
	w := newGuardWorld(t)
	room, err := w.imports.Room(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if room.Seats != nil || room.Teachers != nil {
		t.Errorf("room = %+v, want no limit by default", room)
	}
	if _, ok := room.TeacherRoles[w.teacher.id]; !ok {
		t.Error("the built-in teacher role must take a teacher place")
	}
	if _, ok := room.TeacherRoles[w.student.id]; ok {
		t.Error("the built-in student role must not take a teacher place")
	}
	used := w.used()
	w.limit(limits.Seats, used.Seats+3)
	w.limit(limits.Teachers, used.Teachers)
	if room, err = w.imports.Room(context.Background()); err != nil {
		t.Fatal(err)
	}
	if room.Seats == nil || *room.Seats != 3 || room.Teachers == nil || *room.Teachers != 0 {
		t.Errorf("room = seats %v teachers %v, want 3 and 0", room.Seats, room.Teachers)
	}
}
