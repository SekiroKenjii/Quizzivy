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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transferWorld struct {
	pool  *pgxpool.Pool
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
	w := &transferWorld{pool: pool, dbx: db.NewContext(pool)}
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
			`DELETE FROM app.classes WHERE teacher_id = ANY($1::uuid[])`,
			`DELETE FROM app.users WHERE id = ANY($1::uuid[])`,
		} {
			if _, err := pool.Exec(context.Background(), sql, w.users); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
	})
	return w
}

func (w *transferWorld) user(t *testing.T, builtin string, disabled bool) (id, roleID string) {
	t.Helper()
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, disabled_at)
		VALUES ($1, 'Transfer ' || $2::text, (SELECT id FROM app.roles WHERE builtin_key = $2), CASE WHEN $3 THEN now() END)
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
		if _, err := w.pool.Exec(context.Background(), sql, owner); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
}

func (w *transferWorld) owned(t *testing.T, id string) [5]int {
	t.Helper()
	var n [5]int
	err := w.pool.QueryRow(context.Background(), `
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
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*)::int FROM app.audit_log WHERE action = 'user.content_transferred' AND entity_id = $1::uuid`, id).Scan(&n); err != nil {
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
	want := identitydomain.MovedContent{Classes: 1, Tests: 1, Questions: 2, QuestionGroups: 1, Media: 0, Assignments: 0}
	if got.Moved != want {
		t.Errorf("moved %+v, want only the live rows counted: %+v", got.Moved, want)
	}
	if n := w.owned(t, from); n != [5]int{} {
		t.Errorf("the source still owns %v", n)
	}
	if n := w.owned(t, to); n != [5]int{1, 2, 3, 2, 1} {
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
	for _, part := range []string{`"to": "` + to + `"`, `"tests": 1`, `"questions": 2`, `"questionGroups": 1`, `"importsLeft": 0`} {
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
	_, adminRole := w.user(t, "admin", false)
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
	if n := w.owned(t, from); n != [5]int{1, 2, 3, 2, 1} {
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
	if n := w.owned(t, from); n != [5]int{1, 2, 3, 2, 1} {
		t.Errorf("a failed transfer moved rows: the source owns %v", n)
	}
	if n := w.audits(t, from); n != 0 {
		t.Errorf("a failed transfer wrote %d audit rows", n)
	}
}
