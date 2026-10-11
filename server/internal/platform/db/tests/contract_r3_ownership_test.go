//go:build integration

package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

const validateOwnershipSuffix = "_validate_ownership.sql"

type ownedTable struct {
	table, column, constraint, fill string
	owned, bare                     string
}

var ownedTables = []ownedTable{
	{
		table: "tests", column: "owner_id", constraint: "tests_owner_id_not_null", fill: "tests_fill_owner",
		owned: `INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Đề ' || gen_random_uuid()::text, $1, $1)`,
		bare:  `INSERT INTO app.tests (title, created_by) VALUES ('Đề ' || gen_random_uuid()::text, $1)`,
	},
	{
		table: "questions", column: "owner_id", constraint: "questions_owner_id_not_null", fill: "questions_fill_owner",
		owned: `INSERT INTO app.questions (type, prompt, points, created_by, owner_id) VALUES ('short_answer', 'Câu hỏi', 1, $1, $1)`,
		bare:  `INSERT INTO app.questions (type, prompt, points, created_by) VALUES ('short_answer', 'Câu hỏi', 1, $1)`,
	},
	{
		table: "question_groups", column: "owner_id", constraint: "question_groups_owner_id_not_null", fill: "question_groups_fill_owner",
		owned: `INSERT INTO app.question_groups (title, created_by, owner_id) VALUES ('Bài đọc', $1, $1)`,
		bare:  `INSERT INTO app.question_groups (title, created_by) VALUES ('Bài đọc', $1)`,
	},
	{
		table: "media_assets", column: "owner_id", constraint: "media_assets_owner_id_not_null", fill: "media_assets_fill_owner",
		owned: `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			VALUES ('image', 'media/' || gen_random_uuid()::text || '.png', 'image/png', 10, 'hinh.png', sha256(gen_random_uuid()::text::bytea), $1, $1)`,
		bare: `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by)
			VALUES ('image', 'media/' || gen_random_uuid()::text || '.png', 'image/png', 10, 'hinh.png', sha256(gen_random_uuid()::text::bytea), $1)`,
	},
	{
		table: "classes", column: "teacher_id", constraint: "classes_teacher_id_not_null", fill: "classes_fill_teacher",
		owned: `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp ' || gen_random_uuid()::text, $1)`,
		bare:  `INSERT INTO app.classes (name) VALUES ('Lớp ' || gen_random_uuid()::text || $1::text)`,
	},
}

const fillObjectsQuery = `
	SELECT 'function ' || p.oid::regprocedure::text || E'\n' || pg_get_functiondef(p.oid)
	       || E'\nowner ' || pg_get_userbyid(p.proowner) || ' acl ' || coalesce(p.proacl::text, '<default>')
	  FROM pg_proc p
	 WHERE p.pronamespace = 'app'::regnamespace AND p.proname ~ 'fill'
	UNION ALL
	SELECT 'trigger ' || t.tgrelid::regclass::text || '.' || t.tgname || E'\n' || pg_get_triggerdef(t.oid) || ' enabled ' || t.tgenabled::text
	  FROM pg_trigger t
	 WHERE NOT t.tgisinternal AND t.tgname ~ 'fill'
	 ORDER BY 1`

func ownershipDatabase(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 not set; skipping the ownership contract test, which creates and drops a scratch database")
	}
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	name := "qv_ownership_" + hex.EncodeToString(nonce)
	admin, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	t.Cleanup(func() {
		var err error
		for attempt := 0; attempt < 20; attempt++ {
			if _, err = admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err == nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Errorf("dropping %s: %v", name, err)
	})
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	dsn := swapDatabase(t, db.TestDSN(t), name)
	conn := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	return conn, dsn, db.MigrationsDir(t)
}

func ownershipHolds(t *testing.T, conn *sql.DB, query string, args ...any) bool {
	t.Helper()
	var yes bool
	if err := conn.QueryRow(`SELECT EXISTS (`+query+`)`, args...).Scan(&yes); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return yes
}

func ownershipExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("%.100s: %v", query, err)
	}
}

func ownershipPgError(t *testing.T, err error) *pgconn.PgError {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a database error, got %v", err)
	}
	return pgErr
}

func ownershipAdmin(t *testing.T, conn *sql.DB) string {
	t.Helper()
	var id string
	if err := conn.QueryRow(`INSERT INTO app.users (email, full_name, role_id)
		VALUES ('teacher@example.com', 'Teacher', (SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`).Scan(&id); err != nil {
		t.Fatalf("seeding an Admin: %v", err)
	}
	return id
}

func versionOfOwnershipMigration(t *testing.T, dir string) int64 {
	t.Helper()
	migrations, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	for _, m := range migrations {
		if strings.HasSuffix(filepath.Base(m.Source), validateOwnershipSuffix) {
			return m.Version
		}
	}
	t.Fatalf("no migration ends in %s", validateOwnershipSuffix)
	return 0
}

func ownershipVersion(t *testing.T, conn *sql.DB) int64 {
	t.Helper()
	version, err := goose.GetDBVersion(conn)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func constraintValidated(t *testing.T, conn *sql.DB, table, constraint string) bool {
	t.Helper()
	var validated bool
	if err := conn.QueryRow(`SELECT convalidated FROM pg_constraint WHERE conname = $1 AND conrelid = ('app.' || $2)::regclass`,
		constraint, table).Scan(&validated); err != nil {
		t.Fatalf("%s: %v", constraint, err)
	}
	return validated
}

func fillTriggerExists(t *testing.T, conn *sql.DB, fill string) bool {
	t.Helper()
	return ownershipHolds(t, conn, `SELECT 1 FROM pg_trigger WHERE tgname = $1 AND NOT tgisinternal`, fill)
}

func fillObjects(t *testing.T, conn *sql.DB) string {
	t.Helper()
	rows, err := conn.Query(fillObjectsQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var text string
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			t.Fatal(err)
		}
		text += object + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return text
}

func TestValidatingOwnershipLeavesNoFillTriggerAndNoUnvalidatedOwner(t *testing.T) {
	conn, _, dir := ownershipDatabase(t)
	if err := goose.Up(conn, dir); err != nil {
		t.Fatalf("up: %v", err)
	}

	for _, o := range ownedTables {
		if !constraintValidated(t, conn, o.table, o.constraint) {
			t.Errorf("%s is not validated", o.constraint)
		}
		if fillTriggerExists(t, conn, o.fill) {
			t.Errorf("the trigger %s survived the migration", o.fill)
		}
	}
	for _, function := range []string{"fill_owner_from_created_by", "fill_owner_from_uploaded_by", "classes_fill_teacher"} {
		if ownershipHolds(t, conn, `SELECT 1 FROM pg_proc WHERE proname = $1 AND pronamespace = 'app'::regnamespace`, function) {
			t.Errorf("the function %s survived the migration", function)
		}
	}
	for _, kept := range []string{"tests_owner_id_fkey", "questions_owner_id_fkey", "question_groups_owner_id_fkey", "media_assets_owner_id_fkey", "classes_teacher_id_fkey"} {
		if !ownershipHolds(t, conn, `SELECT 1 FROM pg_constraint WHERE conname = $1 AND contype = 'f' AND convalidated AND confdeltype = 'r'`, kept) {
			t.Errorf("the foreign key %s is missing, not validated or not RESTRICT", kept)
		}
	}
	if ownershipHolds(t, conn, `SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgenabled <> 'O'`) {
		t.Error("a trigger was left disabled")
	}
}

func orphan(t *testing.T, conn *sql.DB, o ownedTable, admin string) {
	t.Helper()
	ownershipExec(t, conn, `ALTER TABLE app.`+o.table+` DROP CONSTRAINT `+o.constraint)
	ownershipExec(t, conn, `ALTER TABLE app.`+o.table+` DISABLE TRIGGER `+o.fill)
	ownershipExec(t, conn, o.bare, admin)
	ownershipExec(t, conn, `ALTER TABLE app.`+o.table+` ENABLE TRIGGER `+o.fill)
	ownershipExec(t, conn, `ALTER TABLE app.`+o.table+` ADD CONSTRAINT `+o.constraint+` NOT NULL `+o.column+` NOT VALID`)
}

func TestTheOwnershipMigrationStopsAtARowWithoutAnOwnerAndNamesItsTable(t *testing.T) {
	conn, _, dir := ownershipDatabase(t)
	before := versionBefore(t, dir, validateOwnershipSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	admin := ownershipAdmin(t, conn)

	for _, o := range ownedTables {
		orphan(t, conn, o, admin)

		err := goose.UpByOne(conn, dir)
		if err == nil {
			t.Fatalf("%s: the migration ran over a row without an owner", o.table)
		}
		pgErr := ownershipPgError(t, err)
		if pgErr.Code != "23502" || pgErr.TableName != o.table || pgErr.ColumnName != o.column {
			t.Errorf("%s: stopped by %s on %s.%s, want 23502 on %s.%s", o.table, pgErr.Code, pgErr.TableName, pgErr.ColumnName, o.table, o.column)
		}
		if want := "app." + o.table + " has 1 row(s) without " + o.column; pgErr.Message != want {
			t.Errorf("%s: stopped with %q, want the check's own message %q", o.table, pgErr.Message, want)
		}
		for _, other := range ownedTables {
			if constraintValidated(t, conn, other.table, other.constraint) {
				t.Errorf("%s: %s was validated although the migration failed", o.table, other.constraint)
			}
			if !fillTriggerExists(t, conn, other.fill) {
				t.Errorf("%s: %s went although the migration failed", o.table, other.fill)
			}
		}
		if version := ownershipVersion(t, conn); version != before {
			t.Errorf("%s: version = %d, want it left at %d", o.table, version, before)
		}
		ownershipExec(t, conn, `DELETE FROM app.`+o.table+` WHERE `+o.column+` IS NULL`)
	}

	if err := goose.UpByOne(conn, dir); err != nil {
		t.Fatalf("up once the rows are gone: %v", err)
	}
}

func TestTheOwnershipMigrationGivesUpWhenItsLockDoesNotArrive(t *testing.T) {
	conn, _, dir := ownershipDatabase(t)
	before := versionBefore(t, dir, validateOwnershipSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	admin := ownershipAdmin(t, conn)

	writer, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if err := writer.Rollback(); err != nil {
			t.Errorf("releasing the writer: %v", err)
		}
	}
	t.Cleanup(release)
	if _, err := writer.Exec(ownedTables[0].owned, admin); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- goose.UpByOne(conn, dir) }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("the migration ran while a transaction was writing app.tests")
		}
		if pgErr := ownershipPgError(t, err); pgErr.Code != "55P03" {
			t.Errorf("stopped by %s, want 55P03 (lock_not_available)", pgErr.Code)
		}
	case <-time.After(30 * time.Second):
		release()
		<-result
		t.Fatal("the migration kept waiting for its lock past its lock_timeout")
	}
	release()

	for _, o := range ownedTables {
		if constraintValidated(t, conn, o.table, o.constraint) {
			t.Errorf("%s was validated although the migration gave up", o.constraint)
		}
		if !fillTriggerExists(t, conn, o.fill) {
			t.Errorf("%s went although the migration gave up", o.fill)
		}
	}
	if version := ownershipVersion(t, conn); version != before {
		t.Errorf("version = %d, want it left at %d", version, before)
	}
	if err := goose.UpByOne(conn, dir); err != nil {
		t.Fatalf("up once the writer is gone: %v", err)
	}
}

func TestTheOwnershipMigrationRunsUnderTheWritesOfTheBinaryBeforeIt(t *testing.T) {
	conn, dsn, dir := ownershipDatabase(t)
	before := versionBefore(t, dir, validateOwnershipSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	admin := ownershipAdmin(t, conn)
	app := openAs(t, appRoleDSN(t, dsn))
	app.SetMaxOpenConns(2 * len(ownedTables))

	counts := make(map[string]*atomic.Int64, len(ownedTables))
	for _, o := range ownedTables {
		counts[o.table] = new(atomic.Int64)
	}
	failures := make(chan error, 2*len(ownedTables))
	stop := make(chan struct{})
	var workers sync.WaitGroup
	run := func(name string, step func() error, count *atomic.Int64) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := step(); err != nil {
					failures <- fmt.Errorf("%s: %w", name, err)
					return
				}
				if count != nil {
					count.Add(1)
				}
			}
		}()
	}
	for _, o := range ownedTables {
		run("insert into "+o.table, func() error { _, err := app.Exec(o.owned, admin); return err }, counts[o.table])
		run("read "+o.table, func() error { var n int; return app.QueryRow(`SELECT count(*) FROM app.` + o.table).Scan(&n) }, nil)
	}
	writesSince := func(floor map[string]int64) {
		deadline := time.Now().Add(20 * time.Second)
		for _, o := range ownedTables {
			for counts[o.table].Load() < floor[o.table]+20 && time.Now().Before(deadline) && len(failures) == 0 {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
	snapshot := func() map[string]int64 {
		floor := make(map[string]int64, len(ownedTables))
		for _, o := range ownedTables {
			floor[o.table] = counts[o.table].Load()
		}
		return floor
	}

	writesSince(snapshot())
	floor := snapshot()
	migration := goose.UpByOne(conn, dir)
	if migration == nil {
		writesSince(floor)
	}
	close(stop)
	workers.Wait()
	close(failures)

	if migration != nil {
		t.Fatalf("the migration, under writes: %v", migration)
	}
	for failure := range failures {
		t.Errorf("a statement of the binary before this release failed: %v", failure)
	}
	for _, o := range ownedTables {
		if counts[o.table].Load() == 0 {
			t.Errorf("no row reached app.%s", o.table)
		}
		if !constraintValidated(t, conn, o.table, o.constraint) {
			t.Errorf("%s is not validated", o.constraint)
		}
	}
}

func TestTheFillTriggersComeBackFromDownAndGoAgain(t *testing.T) {
	conn, _, dir := ownershipDatabase(t)
	before := versionBefore(t, dir, validateOwnershipSuffix)
	removed := versionOfOwnershipMigration(t, dir)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	original := fillObjects(t, conn)
	if original == "" {
		t.Fatal("the expand half has no fill object to compare with")
	}
	admin := ownershipAdmin(t, conn)
	for _, o := range ownedTables {
		ownershipExec(t, conn, o.owned, admin)
	}

	if err := goose.UpTo(conn, dir, removed); err != nil {
		t.Fatalf("up: %v", err)
	}
	if gone := fillObjects(t, conn); gone != "" {
		t.Fatalf("fill objects survived the migration:\n%s", gone)
	}

	if err := goose.Down(conn, dir); err != nil {
		t.Fatalf("down: %v", err)
	}
	if restored := fillObjects(t, conn); restored != original {
		t.Errorf("Down did not restore the fill objects as they were\n--- before ---\n%s\n--- restored ---\n%s", original, restored)
	}
	for _, o := range ownedTables {
		if !constraintValidated(t, conn, o.table, o.constraint) {
			t.Errorf("Down weakened %s", o.constraint)
		}
		var owner string
		if err := conn.QueryRow(o.bare+` RETURNING `+o.column+`::text`, admin).Scan(&owner); err != nil {
			t.Errorf("%s: a row without an owner after Down: %v", o.table, err)
			continue
		}
		if owner != admin {
			t.Errorf("%s: the restored trigger filled %s, want %s", o.table, owner, admin)
		}
	}
	var rows int
	for _, o := range ownedTables {
		var n int
		if err := conn.QueryRow(`SELECT count(*) FROM app.`+o.table+` WHERE `+o.column+` = $1`, admin).Scan(&n); err != nil {
			t.Fatal(err)
		}
		rows += n
	}
	if rows != 2*len(ownedTables) {
		t.Errorf("%d rows belong to the Admin after Down, want %d", rows, 2*len(ownedTables))
	}

	if err := goose.UpTo(conn, dir, removed); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if gone := fillObjects(t, conn); gone != "" {
		t.Errorf("fill objects survived the second up:\n%s", gone)
	}
}

func TestValidatingAnOwnerConstraintBlocksNeitherReadsNorWrites(t *testing.T) {
	conn, dsn, dir := ownershipDatabase(t)
	before := versionBefore(t, dir, validateOwnershipSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	admin := ownershipAdmin(t, conn)
	app := openAs(t, appRoleDSN(t, dsn))

	validating, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = validating.Rollback() })
	if _, err := validating.Exec(`ALTER TABLE app.tests VALIDATE CONSTRAINT tests_owner_id_not_null`); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := validating.QueryRow(`SELECT l.mode FROM pg_locks l WHERE l.pid = pg_backend_pid() AND l.relation = 'app.tests'::regclass
		AND l.mode <> 'AccessShareLock'`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "ShareUpdateExclusiveLock" {
		t.Fatalf("validating holds %s on app.tests, want ShareUpdateExclusiveLock", mode)
	}

	other, err := app.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Rollback() }()
	if _, err := other.Exec(`SET LOCAL lock_timeout = '2s'`); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := other.QueryRow(`INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Đang xác thực', $1, $1) RETURNING id::text`, admin).Scan(&id); err != nil {
		t.Fatalf("an insert while the constraint validates: %v", err)
	}
	if _, err := other.Exec(`UPDATE app.tests SET title = 'Đã sửa' WHERE id = $1`, id); err != nil {
		t.Fatalf("an update while the constraint validates: %v", err)
	}
	var n int
	if err := other.QueryRow(`SELECT count(*) FROM app.tests`).Scan(&n); err != nil {
		t.Fatalf("a read while the constraint validates: %v", err)
	}
}
