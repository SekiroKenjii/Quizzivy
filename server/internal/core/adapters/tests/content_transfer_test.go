//go:build integration

package adapters_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	assignmentscmd "quizzivy/internal/modules/assignments/application/command"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	classescmd "quizzivy/internal/modules/classes/application/command"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	mediacmd "quizzivy/internal/modules/media/application/command"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscmd "quizzivy/internal/modules/questions/application/command"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testscmd "quizzivy/internal/modules/tests/application/command"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transferWorld struct {
	pool  *pgxpool.Pool
	q     db.Querier
	dbx   db.Context
	users []string
}

func newTransferWorld(t *testing.T) *transferWorld {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	w := &transferWorld{pool: pool, q: pool, dbx: db.NewContext(pool)}
	t.Cleanup(func() {
		if len(w.users) == 0 {
			return
		}
		for _, sql := range []string{
			`DELETE FROM app.audit_log WHERE entity_id = ANY($1::uuid[]) OR actor_user_id = ANY($1::uuid[])`,
			`DELETE FROM app.media_assets WHERE uploaded_by = ANY($1::uuid[])`,
			`DELETE FROM app.question_groups WHERE created_by = ANY($1::uuid[])`,
			`DELETE FROM app.questions WHERE created_by = ANY($1::uuid[])`,
			`DELETE FROM app.tests WHERE created_by = ANY($1::uuid[])`,
			`DELETE FROM app.class_members WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM app.classes WHERE teacher_id = ANY($1::uuid[])`,
			`DELETE FROM app.users WHERE created_by = ANY($1::uuid[])`,
			`DELETE FROM app.users WHERE id = ANY($1::uuid[])`,
		} {
			if _, err := pool.Exec(context.Background(), sql, w.users); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
	})
	return w
}

func (w *transferWorld) rolledBack(t *testing.T) {
	t.Helper()
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	w.q, w.dbx = tx, db.NewContext(tx)
}

func (w *transferWorld) user(t *testing.T, builtin string, disabled bool) (id, roleID string) {
	t.Helper()
	err := w.q.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, disabled_at)
		VALUES ($1, 'Transfer ' || $2::text || ' ' || substr($1, 1, 8), (SELECT id FROM app.roles WHERE builtin_key = $2), CASE WHEN $3 THEN now() END)
		RETURNING id::text, role_id::text`, uuid.NewString()+"@example.test", builtin, disabled).Scan(&id, &roleID)
	if err != nil {
		t.Fatal(err)
	}
	w.users = append(w.users, id)
	return id, roleID
}

func (w *transferWorld) content(t *testing.T, owner string) {
	t.Helper()
	for _, sql := range []string{
		`INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp chuyển', $1::uuid)`,
		`INSERT INTO app.classes (name, teacher_id, archived_at) VALUES ('Lớp đã lưu trữ', $1::uuid, now())`,
		`INSERT INTO app.tests (title, owner_id, created_by) VALUES ('Đề còn', $1::uuid, $1::uuid)`,
		`INSERT INTO app.tests (title, owner_id, created_by, deleted_at) VALUES ('Đề đã xoá', $1::uuid, $1::uuid, now())`,
		`INSERT INTO app.questions (type, prompt, points, owner_id, created_by) VALUES ('short_answer', 'Còn?', 1, $1::uuid, $1::uuid)`,
		`INSERT INTO app.questions (type, prompt, points, owner_id, created_by) VALUES ('short_answer', 'Còn nữa?', 1, $1::uuid, $1::uuid)`,
		`INSERT INTO app.questions (type, prompt, points, owner_id, created_by, deleted_at) VALUES ('short_answer', 'Đã xoá?', 1, $1::uuid, $1::uuid, now())`,
		`INSERT INTO app.question_groups (title, owner_id, created_by) VALUES ('Nhóm', $1::uuid, $1::uuid)`,
		`INSERT INTO app.question_groups (title, owner_id, created_by, archived_at) VALUES ('Nhóm cũ', $1::uuid, $1::uuid, now())`,
		`INSERT INTO app.media_assets (kind, mime_type, bytes, checksum_sha256, original_filename, storage_key, owner_id, uploaded_by, deleted_at)
		 VALUES ('image', 'image/png', 1, sha256(gen_random_uuid()::text::bytea), 'a.png', gen_random_uuid()::text, $1::uuid, $1::uuid, now())`,
	} {
		if _, err := w.q.Exec(context.Background(), sql, owner); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
}

func (w *transferWorld) owned(t *testing.T, id string) [5]int {
	t.Helper()
	var n [5]int
	err := w.q.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.classes WHERE teacher_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.tests WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.questions WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.question_groups WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.media_assets WHERE owner_id = $1::uuid)::int`, id).Scan(&n[0], &n[1], &n[2], &n[3], &n[4])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *transferWorld) audits(t *testing.T, id string) int {
	t.Helper()
	var n int
	if err := w.q.QueryRow(context.Background(), `SELECT count(*)::int FROM app.audit_log WHERE action = 'user.content_transferred' AND entity_id = $1::uuid`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func movers(scoped db.Context) adapters.ContentMovers {
	return adapters.ContentMovers{
		Ledger:      identityrepo.NewTransferLedger(scoped),
		Classes:     classescmd.TransferOwnershipHandler{Repo: classesrepo.NewPostgres(scoped)},
		Tests:       testscmd.TransferOwnershipHandler{Repo: testsrepo.NewPostgres(scoped, nil, nil)},
		Questions:   questionscmd.TransferOwnershipHandler{Repo: questionsrepo.NewPostgres(scoped)},
		Media:       mediacmd.TransferOwnershipHandler{Repo: mediarepo.NewPostgres(scoped)},
		Assignments: assignmentscmd.TransferOwnershipHandler{Repo: assignmentsrepo.NewPostgres(scoped)},
	}
}

func (w *transferWorld) move(from, fromRole, to string, change func(*adapters.ContentMovers)) (identitydomain.TransferredContent, error) {
	mover := adapters.ContentTransfer{DB: w.dbx, Movers: func(scoped db.Context) adapters.ContentMovers {
		m := movers(scoped)
		if change != nil {
			change(&m)
		}
		return m
	}}
	req := identitydomain.WriteRequest{ActorID: to, IP: "203.0.113.4", UserAgent: "transfer-test"}
	return mover.Move(context.Background(), req, identitydomain.ContentTransfer{FromID: from, FromRoleID: fromRole, ToID: to, Now: time.Now()})
}

func TestATransferMovesDeletedRowsUncountedLeavesProvenanceAndAuditsTheCounts(t *testing.T) {
	w := newTransferWorld(t)
	from, fromRole := w.user(t, "teacher", true)
	to, _ := w.user(t, "teacher", false)
	w.content(t, from)

	got, err := w.move(from, fromRole, to, nil)

	if err != nil {
		t.Fatal(err)
	}
	want := identitydomain.MovedContent{Classes: 2, Tests: 1, Questions: 2, QuestionGroups: 1, Media: 0, Assignments: 0}
	if got.Moved != want {
		t.Errorf("moved %+v, want every class and only the live rows of the rest counted: %+v", got.Moved, want)
	}
	if n := w.owned(t, from); n != [5]int{} {
		t.Errorf("the source still owns %v", n)
	}
	if n := w.owned(t, to); n != [5]int{2, 2, 3, 2, 1} {
		t.Errorf("the recipient owns %v, want every row, deleted and archived ones too", n)
	}
	var madeBy int
	if err := w.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.tests WHERE created_by = $1::uuid) + (SELECT count(*) FROM app.questions WHERE created_by = $1::uuid)
		     + (SELECT count(*) FROM app.question_groups WHERE created_by = $1::uuid) + (SELECT count(*) FROM app.media_assets WHERE uploaded_by = $1::uuid)`, from).Scan(&madeBy); err != nil {
		t.Fatal(err)
	}
	if madeBy != 8 {
		t.Errorf("%d rows still name the source as their maker, want all 8", madeBy)
	}
	var diff string
	if err := w.pool.QueryRow(context.Background(), `
		SELECT diff::text FROM app.audit_log WHERE action = 'user.content_transferred' AND entity_id = $1::uuid AND actor_user_id = $2::uuid`, from, to).Scan(&diff); err != nil {
		t.Fatal(err)
	}
	var named bool
	if err := w.pool.QueryRow(context.Background(), `
		SELECT a.diff->'owner' = jsonb_build_object('old', f.full_name, 'new', r.full_name)
		  FROM app.audit_log a, app.users f, app.users r
		 WHERE a.action = 'user.content_transferred' AND a.entity_id = $1::uuid AND f.id = $1::uuid AND r.id = $2::uuid`, from, to).Scan(&named); err != nil || !named {
		t.Errorf("the audit row does not name the owner change from the source to the recipient (%v): %s", err, diff)
	}
	for _, part := range []string{
		`"to": "` + to + `"`, `"classes": 2`, `"tests": 1`, `"questions": 2`, `"questionGroups": 1`, `"importsLeft": 0`,
	} {
		if !strings.Contains(diff, part) {
			t.Errorf("audit diff %s lacks %s", diff, part)
		}
	}
}

func TestATransferIsRefusedForEachWrongPartyAndMovesNothing(t *testing.T) {
	w := newTransferWorld(t)
	from, fromRole := w.user(t, "teacher", true)
	active, activeRole := w.user(t, "teacher", false)
	to, _ := w.user(t, "teacher", false)
	gone, _ := w.user(t, "teacher", true)
	pupil, _ := w.user(t, "student", false)
	var adminRole string
	if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.roles WHERE builtin_key = 'admin'`).Scan(&adminRole); err != nil {
		t.Fatal(err)
	}
	w.content(t, from)

	for label, c := range map[string]struct {
		from, role, to string
		want           error
	}{
		"a missing source":          {uuid.NewString(), fromRole, to, identitydomain.ErrUserNotFound},
		"a source whose role moved": {from, adminRole, to, identitydomain.ErrRoleChanged},
		"an active source":          {active, activeRole, to, identitydomain.ErrUserNotDisabled},
		"the source itself":         {from, fromRole, from, identitydomain.ErrInvalidRecipient},
		"a missing recipient":       {from, fromRole, uuid.NewString(), identitydomain.ErrInvalidRecipient},
		"a disabled recipient":      {from, fromRole, gone, identitydomain.ErrInvalidRecipient},
		"a student":                 {from, fromRole, pupil, identitydomain.ErrInvalidRecipient},
	} {
		if _, err := w.move(c.from, c.role, c.to, nil); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", label, err, c.want)
		}
	}
	if n := w.owned(t, from); n != [5]int{2, 2, 3, 2, 1} {
		t.Errorf("a refusal moved rows: the source owns %v", n)
	}
	if n := w.audits(t, from); n != 0 {
		t.Errorf("a refusal wrote %d audit rows", n)
	}
}

func TestAFailingStepMovesNothingAndADeadlockIsAContendedWrite(t *testing.T) {
	w := newTransferWorld(t)
	from, fromRole := w.user(t, "teacher", true)
	to, _ := w.user(t, "teacher", false)
	w.content(t, from)
	deadlock := &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}

	_, err := w.move(from, fromRole, to, func(m *adapters.ContentMovers) {
		m.Assignments = cqrs.HandlerFunc[assignmentscmd.TransferOwnership, int](func(context.Context, assignmentscmd.TransferOwnership) (int, error) {
			return 0, deadlock
		})
	})

	if !errors.Is(err, identitydomain.ErrWriteContended) || !errors.Is(err, deadlock) {
		t.Errorf("a deadlock on the last step: %v, want ErrWriteContended keeping the database error", err)
	}
	if n := w.owned(t, from); n != [5]int{2, 2, 3, 2, 1} {
		t.Errorf("a failed transfer moved rows: the source owns %v", n)
	}
	if n := w.audits(t, from); n != 0 {
		t.Errorf("a failed transfer wrote %d audit rows", n)
	}
}

func TestAnActiveAdminTakesAnAccountsContentLikeATeacher(t *testing.T) {
	w := newTransferWorld(t)
	w.rolledBack(t)
	from, fromRole := w.user(t, "teacher", true)
	to, _ := w.user(t, "admin", false)
	w.content(t, from)

	got, err := w.move(from, fromRole, to, nil)

	if err != nil {
		t.Fatalf("moving to an Admin: %v, want it done: the Admin holds the teacher workspace", err)
	}
	if got.Moved.Classes != 2 || got.Moved.Questions != 2 {
		t.Errorf("moved %+v, want the content counted", got.Moved)
	}
	if n := w.owned(t, to); n != [5]int{2, 2, 3, 2, 1} {
		t.Errorf("the Admin owns %v, want every row", n)
	}
}

func (w *transferWorld) backend(t *testing.T, tx pgx.Tx) int {
	t.Helper()
	var pid int
	if err := tx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	return pid
}

func (w *transferWorld) waitBlockedBy(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := w.pool.QueryRow(context.Background(), `
			SELECT EXISTS (SELECT 1 FROM pg_stat_activity a WHERE $1::int = ANY(pg_blocking_pids(a.pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("nothing waited on the holder")
}

func (w *transferWorld) lockNowait(table, id string) error {
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(context.Background(), `SELECT 1 FROM app.`+table+` WHERE id = $1::uuid FOR NO KEY UPDATE NOWAIT`, id)
	return err
}

func TestTheLedgerHoldsTheRecipientsRoleUntilTheTransferEnds(t *testing.T) {
	w := newTransferWorld(t)
	from, fromRole := w.user(t, "teacher", true)
	to, toRole := w.user(t, "teacher", false)
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if err := identityrepo.NewTransferLedger(db.NewContext(tx)).LockTransfer(context.Background(), identitydomain.ContentTransfer{FromID: from, FromRoleID: fromRole, ToID: to}); err != nil {
		t.Fatal(err)
	}

	var pg *pgconn.PgError
	if err := w.lockNowait("roles", toRole); !errors.As(err, &pg) || pg.Code != pgerrcode.LockNotAvailable {
		t.Errorf("an edit of the recipient's role during the transfer: %v, want it to wait on the transfer's share lock", err)
	}
	for _, id := range []string{from, to} {
		if err := w.lockNowait("users", id); !errors.As(err, &pg) || pg.Code != pgerrcode.LockNotAvailable {
			t.Errorf("a write to account %s during the transfer: %v, want it to wait", id, err)
		}
	}
}

func TestTheLedgerLocksBothAccountsInIdOrderAndNotInTheOrderTheTableHoldsThem(t *testing.T) {
	w := newTransferWorld(t)
	ctx := context.Background()
	a, aRole := w.user(t, "teacher", true)
	b, bRole := w.user(t, "teacher", true)
	lower, higher, from, fromRole, to := a, b, a, aRole, b
	if b < a {
		lower, higher, from, fromRole, to = b, a, b, bRole, a
	}
	if _, err := w.pool.Exec(ctx, `UPDATE app.users SET disabled_at = NULL WHERE id = $1::uuid`, to); err != nil {
		t.Fatal(err)
	}
	for round := 0; ; round++ {
		var after bool
		if err := w.pool.QueryRow(ctx, `
			SELECT (SELECT ctid FROM app.users WHERE id = $1::uuid) > (SELECT ctid FROM app.users WHERE id = $2::uuid)`, lower, higher).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after {
			break
		}
		if round == 50 {
			t.Fatal("the lower id never moved past the higher one in the table")
		}
		if _, err := w.pool.Exec(ctx, `UPDATE app.users SET email = 'm' || email WHERE id = $1::uuid`, lower); err != nil {
			t.Fatal(err)
		}
	}
	holder, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, lower); err != nil {
		t.Fatal(err)
	}
	ledger, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ledger.Rollback(ctx) }()
	if _, err := ledger.Exec(ctx, `SET LOCAL enable_indexscan = off; SET LOCAL enable_indexonlyscan = off; SET LOCAL enable_bitmapscan = off`); err != nil {
		t.Fatal(err)
	}

	locked := make(chan error, 1)
	go func() {
		locked <- identityrepo.NewTransferLedger(db.NewContext(ledger)).LockTransfer(ctx, identitydomain.ContentTransfer{FromID: from, FromRoleID: fromRole, ToID: to})
	}()
	w.waitBlockedBy(t, w.backend(t, holder))
	probed := w.lockNowait("users", higher)
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-locked:
		if err != nil {
			t.Fatalf("the lock: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the lock never finished")
	}
	if probed != nil {
		t.Errorf("the higher id was locked while the lower one was still awaited (%v): the ledger locks in table order, not id order", probed)
	}
}

func (w *transferWorld) student(t *testing.T, createdBy string) string {
	t.Helper()
	var id string
	if err := w.q.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, created_by)
		VALUES ($1, 'Học sinh chuyển', (SELECT id FROM app.roles WHERE builtin_key = 'student'), $2::uuid)
		RETURNING id::text`, uuid.NewString()+"@example.test", createdBy).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestATransferCountsTheStudentsTheSourceCreatedThatTheRecipientStillDoesNotReach(t *testing.T) {
	w := newTransferWorld(t)
	w.rolledBack(t)
	from, fromRole := w.user(t, "teacher", true)
	to, _ := w.user(t, "teacher", false)
	w.content(t, from)
	alone, classed, own := w.student(t, from), w.student(t, from), w.student(t, to)
	for _, id := range []string{classed, own} {
		if _, err := w.q.Exec(context.Background(), `
			INSERT INTO app.class_members (class_id, user_id, joined_via) SELECT c.id, $2::uuid, 'admin' FROM app.classes c WHERE c.teacher_id = $1::uuid AND c.archived_at IS NULL`, from, id); err != nil {
			t.Fatal(err)
		}
	}

	got, err := w.move(from, fromRole, to, nil)

	if err != nil {
		t.Fatal(err)
	}
	if got.LeftBehind.Students != 1 {
		t.Errorf("students left behind = %d, want 1: only %s is in no moved class", got.LeftBehind.Students, alone)
	}
	var left int
	if err := w.q.QueryRow(context.Background(), `
		SELECT (diff->>'studentsLeft')::int FROM app.audit_log WHERE action = 'user.content_transferred' AND entity_id = $1::uuid`, from).Scan(&left); err != nil || left != 1 {
		t.Errorf("the audit row's studentsLeft = %d (%v), want 1", left, err)
	}
}
