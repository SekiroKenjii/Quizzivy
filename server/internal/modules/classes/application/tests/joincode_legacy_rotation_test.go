//go:build integration

package application_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/modules/classes/repositories"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsquery "quizzivy/internal/modules/notifications/application/query"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	notificationsrepo "quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
)

type ownClasses struct {
	domain.Repository
	ids []string
}

func (o ownClasses) LegacyCodeClasses(ctx context.Context, now time.Time) ([]domain.LegacyCodeClass, error) {
	all, err := o.Repository.LegacyCodeClasses(ctx, now)
	if err != nil {
		return nil, err
	}
	var own []domain.LegacyCodeClass
	for _, class := range all {
		if slices.Contains(o.ids, class.ClassID) {
			own = append(own, class)
		}
	}
	return own, nil
}

func rotationOver(pool *pgxpool.Pool, notifier ports.Notifier, classIDs ...string) *application.Application {
	repo := ownClasses{Repository: repositories.NewPostgres(db.NewContext(pool)), ids: classIDs}
	return application.New(repo, nil, joinKeys).WithNotifier(notifier)
}

func rotateLegacy(t *testing.T, svc *application.Application) domain.LegacyRotation {
	t.Helper()
	run, err := svc.Commands.RotateLegacyJoinCodes.Handle(context.Background(), command.RotateLegacyJoinCodes{
		Problem: func(err error) { t.Errorf("the rotation reported a problem: %v", err) },
	})
	if err != nil {
		t.Fatalf("the rotation: %v", err)
	}
	return run
}

type rotationResult struct {
	run domain.LegacyRotation
	err error
}

func startRotation(svc *application.Application) <-chan rotationResult {
	done := make(chan rotationResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		run, err := svc.Commands.RotateLegacyJoinCodes.Handle(ctx, command.RotateLegacyJoinCodes{})
		done <- rotationResult{run: run, err: err}
	}()
	return done
}

func finished[T any](t *testing.T, what string, done <-chan T) T {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not finish", what)
		panic("unreachable")
	}
}

func forgetRotationAudit(t *testing.T, pool *pgxpool.Pool, classIDs ...string) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `
			DELETE FROM app.audit_log
			 WHERE entity = 'class_join_code'
			   AND entity_id IN (SELECT id FROM app.class_join_codes WHERE class_id = ANY($1::uuid[]))`, classIDs); err != nil {
			t.Errorf("removing the rotation's audit rows: %v", err)
		}
	})
}

func legacyClassRow(t *testing.T, pool *pgxpool.Pool) (classID, teacherID, code string) {
	t.Helper()
	classID, teacherID, _ = makeClassRow(t, pool)
	forgetRotationAudit(t, pool, classID)
	return classID, teacherID, legacyCode(t, pool, classID, teacherID)
}

func teacherWithLegacyClasses(t *testing.T, pool *pgxpool.Pool, names ...string) (teacherID string, classIDs []string) {
	t.Helper()
	ctx := context.Background()
	n := nonce(t)
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Cô Lan', (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		"legacy-teacher-"+n+"@example.com").Scan(&teacherID); err != nil {
		t.Fatalf("insert teacher: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE entity = 'class_join_code' AND entity_id IN (SELECT jc.id FROM app.class_join_codes jc JOIN app.classes c ON c.id = jc.class_id WHERE c.teacher_id = $1)`, teacherID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_join_codes WHERE class_id IN (SELECT id FROM app.classes WHERE teacher_id = $1)`, teacherID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE teacher_id = $1`, teacherID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, teacherID)
	})
	for _, name := range names {
		var classID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, name+" "+n, teacherID).Scan(&classID); err != nil {
			t.Fatalf("insert class: %v", err)
		}
		legacyCode(t, pool, classID, teacherID)
		classIDs = append(classIDs, classID)
	}
	return teacherID, classIDs
}

type codeRow struct {
	id         string
	scheme     int16
	keyID      *int16
	hash       []byte
	ciphertext []byte
	hint       string
	expiresAt  time.Time
	maxUses    *int
	usesCount  int
	revokedAt  *time.Time
	createdBy  string
	createdAt  time.Time
}

func codeRows(t *testing.T, pool *pgxpool.Pool, classID string) []codeRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id::text, lookup_scheme, key_id, code_hash, code_ciphertext, code_hint,
		       expires_at, max_uses, uses_count, revoked_at, created_by::text, created_at
		  FROM app.class_join_codes
		 WHERE class_id = $1
		 ORDER BY (revoked_at IS NULL), id`, classID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []codeRow
	for rows.Next() {
		var c codeRow
		if err := rows.Scan(&c.id, &c.scheme, &c.keyID, &c.hash, &c.ciphertext, &c.hint,
			&c.expiresAt, &c.maxUses, &c.usesCount, &c.revokedAt, &c.createdBy, &c.createdAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func rotatedPair(t *testing.T, pool *pgxpool.Pool, classID string) (previous, current codeRow) {
	t.Helper()
	rows := codeRows(t, pool, classID)
	if len(rows) != 2 || rows[0].revokedAt == nil || rows[1].revokedAt != nil {
		t.Fatalf("class %s holds %d code rows, want the revoked legacy one and one active: %+v", classID, len(rows), rows)
	}
	return rows[0], rows[1]
}

func snapshot(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func codesOf(t *testing.T, pool *pgxpool.Pool, classID string) []string {
	t.Helper()
	return snapshot(t, pool, `SELECT to_jsonb(jc)::text FROM app.class_join_codes jc WHERE jc.class_id = $1 ORDER BY jc.id`, classID)
}

func classOf(t *testing.T, pool *pgxpool.Pool, classID string) []string {
	t.Helper()
	return snapshot(t, pool, `SELECT to_jsonb(c)::text FROM app.classes c WHERE c.id = $1`, classID)
}

func rotationAudits(t *testing.T, pool *pgxpool.Pool, classIDs ...string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM app.audit_log a
		 WHERE a.entity = 'class_join_code' AND a.diff->>'reason' = 'legacy_rotation'
		   AND a.entity_id IN (SELECT id FROM app.class_join_codes WHERE class_id = ANY($1::uuid[]))`, classIDs).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func currentCode(t *testing.T, svc *application.Application, classID string) domain.ActiveJoinCode {
	t.Helper()
	got, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: everyone, ClassID: classID})
	if err != nil {
		t.Fatalf("read the active code back: %v", err)
	}
	if got.Legacy || got.Code == "" {
		t.Fatalf("the active code of %s reads back as %+v, want a sealed code its teacher can read", classID, got)
	}
	return got
}

type refusal struct {
	preview  domain.PreviewResult
	existing domain.EnrolResult
	google   domain.EnrolResult
}

func refusalOf(t *testing.T, pool *pgxpool.Pool, svc *application.Application, code string) refusal {
	t.Helper()
	ctx := context.Background()
	_, _, student := makeClassRow(t, pool)
	existing, err := svc.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: student, Code: code})
	if err != nil {
		t.Fatalf("joining as a signed-in student: %v", err)
	}
	m := newMember(t)
	dropUser(t, pool, m.Email)
	google, err := svc.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code})
	if err != nil {
		t.Fatalf("the Google join: %v", err)
	}
	var accounts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE email = $1`, m.Email).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != 0 {
		t.Errorf("a refused Google join left %d account(s) behind", accounts)
	}
	return refusal{preview: previewOf(t, svc, code), existing: existing, google: google}
}

func refusedAs(outcome domain.PreviewOutcome) refusal {
	return refusal{
		preview:  domain.PreviewResult{Outcome: outcome},
		existing: domain.EnrolResult{Outcome: outcome},
		google:   domain.EnrolResult{Outcome: outcome},
	}
}

func ownConnection(t *testing.T, settings ...string) (*pgxpool.Pool, int) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	cfg.MaxConns = 1
	for i := 0; i+1 < len(settings); i += 2 {
		cfg.ConnConfig.RuntimeParams[settings[i]] = settings[i+1]
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	var pid int
	if err := pool.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read the connection's backend: %v", err)
	}
	return pool, pid
}

func holding(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) (release func(), pid int) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the holder: %v", err)
	}
	released := false
	release = func() {
		if released {
			return
		}
		released = true
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("release the holder: %v", err)
		}
	}
	t.Cleanup(release)
	if _, err := tx.Exec(ctx, statement, args...); err != nil {
		t.Fatalf("take the lock to hold: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read the holder's backend: %v", err)
	}
	return release, pid
}

func waitUntilBlocked(t *testing.T, pool *pgxpool.Pool, waiter, blocker int, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(context.Background(),
			`SELECT $2::int = ANY(pg_blocking_pids($1::int))`, waiter, blocker).Scan(&blocked); err != nil {
			t.Fatalf("look for %s: %v", what, err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never waited", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func onlySealedCode(t *testing.T, pool *pgxpool.Pool, classID string) codeRow {
	t.Helper()
	var active []codeRow
	for _, row := range codeRows(t, pool, classID) {
		if row.revokedAt == nil {
			active = append(active, row)
		}
	}
	if len(active) != 1 {
		t.Fatalf("class %s holds %d active codes, want exactly one", classID, len(active))
	}
	if active[0].scheme != int16(domain.LookupKeyed) {
		t.Fatalf("the active code of class %s is still stored under scheme %d", classID, active[0].scheme)
	}
	return active[0]
}

func joinedThrough(t *testing.T, pool *pgxpool.Pool, classID, userID string) string {
	t.Helper()
	var codeID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT join_code_id::text FROM app.class_members WHERE class_id = $1 AND user_id = $2`, classID, userID).Scan(&codeID); err != nil {
		t.Fatalf("the student is not a member: %v", err)
	}
	if codeID == nil {
		t.Fatal("the membership names no code")
	}
	return *codeID
}

func TestALegacyCodeIsReplacedByASealedOneThatKeepsItsLimits(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	open, openTeacher, _ := makeClassRow(t, pool)
	closed, closedTeacher, _ := makeClassRow(t, pool)
	forgetRotationAudit(t, pool, open, closed)
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, uses_count, created_by, created_at)
		VALUES ($1, sha256($2::bytea), 'AAAA', now() + interval '12 days', 40, 3, $3, now() - interval '18 days')`,
		open, "limits-"+open, openTeacher); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, sha256($2::bytea), 'BBBB', now() + interval '5 days', NULL, $3, now() - interval '25 days')`,
		closed, "limits-"+closed, closedTeacher); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, closed); err != nil {
		t.Fatal(err)
	}
	before := map[string]codeRow{open: codeRows(t, pool, open)[0], closed: codeRows(t, pool, closed)[0]}
	classesBefore := map[string][]string{open: classOf(t, pool, open), closed: classOf(t, pool, closed)}

	at := time.Now().UTC().Truncate(time.Microsecond)
	svc := rotationOver(pool, nil, open, closed)
	svc.SetClock(func() time.Time { return at })
	if run := rotateLegacy(t, svc); run != (domain.LegacyRotation{Found: 2, Rotated: 2}) {
		t.Fatalf("the run answered %+v, want both classes found and rotated", run)
	}

	for class, creator := range map[string]string{open: openTeacher, closed: closedTeacher} {
		was := before[class]
		previous, current := rotatedPair(t, pool, class)
		if previous.id != was.id || previous.revokedAt == nil || !previous.revokedAt.Equal(at) {
			t.Errorf("the legacy row is %+v, want it revoked at %s", previous, at)
		}
		if previous.scheme != int16(domain.LookupLegacy) || previous.usesCount != was.usesCount || !bytes.Equal(previous.hash, was.hash) {
			t.Errorf("the legacy row changed beyond its revocation: %+v", previous)
		}
		if current.scheme != int16(domain.LookupKeyed) || current.keyID == nil || *current.keyID != joinKeys.CurrentID() {
			t.Errorf("the new row is scheme %d under key %v, want 2 under %d", current.scheme, current.keyID, joinKeys.CurrentID())
		}
		if !current.expiresAt.Equal(was.expiresAt) {
			t.Errorf("expires_at = %s, want the legacy code's %s", current.expiresAt, was.expiresAt)
		}
		if !reflect.DeepEqual(current.maxUses, was.maxUses) {
			t.Errorf("max_uses = %v, want the legacy code's %v", current.maxUses, was.maxUses)
		}
		if current.usesCount != 0 {
			t.Errorf("uses_count = %d, want 0", current.usesCount)
		}
		if current.createdBy != creator || current.createdBy != was.createdBy {
			t.Errorf("created_by = %s, want the legacy code's %s", current.createdBy, was.createdBy)
		}
		if !current.createdAt.Equal(at) {
			t.Errorf("created_at = %s, want the rotation's instant %s", current.createdAt, at)
		}
		code, err := joinKeys.Open(class, current.id, *current.keyID, current.ciphertext)
		if err != nil {
			t.Fatalf("the new row does not open under its class and code ids: %v", err)
		}
		if current.hint != domain.JoinCodes.Hint(code) || !bytes.Equal(current.hash, joinKeys.Hash(code)) {
			t.Errorf("the new row's hint or hash is not its own code's")
		}
		if got := classOf(t, pool, class); !slices.Equal(got, classesBefore[class]) {
			t.Errorf("the class row changed:\n%v\nwas\n%v", got, classesBefore[class])
		}
	}
	if !selfJoinEnabled(t, pool, open) || selfJoinEnabled(t, pool, closed) {
		t.Error("the rotation changed a class's self-join switch")
	}
	if maxUses := codeRows(t, pool, closed)[1].maxUses; maxUses != nil {
		t.Errorf("a code without a use cap came back capped at %d", *maxUses)
	}
}

func rowShape(t *testing.T, pool *pgxpool.Pool, codeID string) []string {
	t.Helper()
	return snapshot(t, pool, `
		SELECT e.key || ': ' || jsonb_typeof(e.value)
		       || CASE WHEN e.key IN ('code_hash', 'code_ciphertext', 'code_hint') THEN ' of ' || length(e.value #>> '{}') ELSE '' END
		       || CASE WHEN e.key IN ('lookup_scheme', 'key_id', 'uses_count', 'max_uses') THEN ' = ' || coalesce(e.value #>> '{}', 'null') ELSE '' END
		  FROM app.class_join_codes jc, jsonb_each(to_jsonb(jc)) e
		 WHERE jc.id = $1
		 ORDER BY e.key`, codeID)
}

func TestTheNewRowHasExactlyTheColumnsATeachersRotateWrites(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, _, _ := legacyClassRow(t, pool)
	teachers, teacher, _ := makeClassRow(t, pool)
	issueCode(t, svc, teachers, teacher)

	if run := rotateLegacy(t, rotationOver(pool, nil, classID)); run.Rotated != 1 {
		t.Fatalf("the run answered %+v", run)
	}
	_, current := rotatedPair(t, pool, classID)
	got, want := rowShape(t, pool, current.id), rowShape(t, pool, activeRow(t, pool, teachers).id)
	if len(want) < 12 {
		t.Fatalf("a row a teacher's Rotate wrote reads as %v", want)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the rotation's row is\n%s\nand a teacher's Rotate writes\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestARotatedLegacyCodeAnswersAsACodeItsTeacherReplaced(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()
	classID, _, old := legacyClassRow(t, pool)
	_, _, early := makeClassRow(t, pool)
	if joined, err := svc.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: early, Code: old}); err != nil || joined.Outcome != domain.PreviewOK {
		t.Fatalf("the legacy code did not redeem before the rotation: %+v (%v)", joined, err)
	}

	teachers, teacher, _ := makeClassRow(t, pool)
	replaced := issueCode(t, svc, teachers, teacher)
	issueCode(t, svc, teachers, teacher)

	if run := rotateLegacy(t, rotationOver(pool, nil, classID)); run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
		t.Fatalf("the run answered %+v", run)
	}

	rotated, byTeacher := refusalOf(t, pool, svc, old), refusalOf(t, pool, svc, replaced)
	if rotated != byTeacher {
		t.Errorf("the rotated legacy code answers %+v and a code its teacher replaced %+v; they must be identical", rotated, byTeacher)
	}
	if rotated != refusedAs(domain.PreviewRevoked) {
		t.Errorf("the rotated legacy code answers %+v, want PreviewRevoked and no class data on every path", rotated)
	}
	if code := classeshttp.JoinCodeError(ctx, rotated.preview.Outcome).Error.Code; code != "JOIN_CODE_REVOKED" {
		t.Errorf("the transport answers %s, want JOIN_CODE_REVOKED", code)
	}
	if n := memberCount(t, pool, classID); n != 2 {
		t.Errorf("the class has %d members, want the two it had before the rotation", n)
	}

	current := currentCode(t, svc, classID)
	if current.UsesCount != 0 {
		t.Errorf("the new code starts at %d uses", current.UsesCount)
	}
	joinsAll(t, pool, svc, classID, domain.JoinCodes.Format(current.Code))
	if n := memberCount(t, pool, classID); n != 4 {
		t.Errorf("the class has %d members after two joined through the new code, want 4", n)
	}
}

func TestAClosedOrArchivedClassAnswersInvalidBeforeAndAfterItsRotation(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()
	closed, _, closedCode := legacyClassRow(t, pool)
	archived, _, archivedCode := legacyClassRow(t, pool)
	if _, err := pool.Exec(ctx, `UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, closed); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.classes SET archived_at = now() WHERE id = $1`, archived); err != nil {
		t.Fatal(err)
	}
	classesBefore := map[string][]string{closed: classOf(t, pool, closed), archived: classOf(t, pool, archived)}
	unknown := refusalOf(t, pool, svc, "ZZZZ-ZZZZ")
	if unknown != refusedAs(domain.PreviewInvalid) {
		t.Fatalf("an unknown code answers %+v", unknown)
	}
	for class, code := range map[string]string{closed: closedCode, archived: archivedCode} {
		if got := refusalOf(t, pool, svc, code); got != unknown {
			t.Fatalf("before the rotation the legacy code of %s answers %+v, want what an unknown code answers", class, got)
		}
	}

	if run := rotateLegacy(t, rotationOver(pool, nil, closed, archived)); run != (domain.LegacyRotation{Found: 2, Rotated: 2}) {
		t.Fatalf("the run answered %+v, want both classes rotated", run)
	}

	for class, code := range map[string]string{closed: closedCode, archived: archivedCode} {
		onlySealedCode(t, pool, class)
		if got := refusalOf(t, pool, svc, code); got != unknown {
			t.Errorf("after the rotation the old code of %s answers %+v, want what an unknown code answers", class, got)
		}
		fresh := domain.JoinCodes.Format(currentCode(t, svc, class).Code)
		if got := refusalOf(t, pool, svc, fresh); got != unknown {
			t.Errorf("the new code of %s answers %+v, want what an unknown code answers until its teacher opens the class", class, got)
		}
		if got := classOf(t, pool, class); !slices.Equal(got, classesBefore[class]) {
			t.Errorf("the class row changed:\n%v\nwas\n%v", got, classesBefore[class])
		}
	}
	if selfJoinEnabled(t, pool, closed) {
		t.Error("the rotation opened a class whose teacher had closed self-join")
	}
}

func TestTheRotationLeavesEveryOtherCodeAlone(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()
	repo := repositories.NewPostgres(db.NewContext(pool))

	revoked, revokedTeacher, _ := makeClassRow(t, pool)
	expired, expiredTeacher, _ := makeClassRow(t, pool)
	sealed, sealedTeacher, _ := makeClassRow(t, pool)
	none, _, _ := makeClassRow(t, pool)
	live, liveTeacher, _ := legacyClassRow(t, pool)
	forgetRotationAudit(t, pool, revoked, expired, sealed, none)
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, revoked_at, created_by)
		VALUES ($1, sha256($2::bytea), 'CCCC', now() + interval '30 days', now(), $3)`,
		revoked, "alone-"+revoked, revokedTeacher); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, created_at)
		VALUES ($1, sha256($2::bytea), 'DDDD', now() - interval '1 day', $3, now() - interval '31 days')`,
		expired, "alone-"+expired, expiredTeacher); err != nil {
		t.Fatal(err)
	}
	issueCode(t, svc, sealed, sealedTeacher)

	untouched := []string{revoked, expired, sealed, none}
	before := map[string][]string{}
	for _, class := range untouched {
		before[class] = codesOf(t, pool, class)
	}

	listed, err := repo.LegacyCodeClasses(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var found *domain.LegacyCodeClass
	for _, class := range listed {
		if slices.Contains(untouched, class.ClassID) {
			t.Errorf("class %s is listed although it holds no usable legacy code", class.ClassID)
		}
		if class.ClassID == live {
			found = &class
		}
	}
	var liveName string
	if err := pool.QueryRow(ctx, `SELECT name FROM app.classes WHERE id = $1`, live).Scan(&liveName); err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ClassName != liveName || found.TeacherID == nil || *found.TeacherID != liveTeacher {
		t.Errorf("the class with a usable legacy code is listed as %+v, want its name and its teacher", found)
	}

	for _, class := range append(untouched, "0195c000-0000-7000-8000-00000000ffff") {
		wrote, err := repo.RotateLegacyCode(ctx, sealedInput(t, class))
		if err != nil || wrote {
			t.Errorf("rotating class %s answered %v (%v), want nothing written and no error", class, wrote, err)
		}
	}
	for _, class := range untouched {
		if got := codesOf(t, pool, class); !slices.Equal(got, before[class]) {
			t.Errorf("the codes of class %s changed:\n%v\nwere\n%v", class, got, before[class])
		}
	}
	if n := rotationAudits(t, pool, untouched...); n != 0 {
		t.Errorf("%d audit row(s) for codes the rotation must leave alone", n)
	}
	if run := rotateLegacy(t, rotationOver(pool, nil, untouched...)); run != (domain.LegacyRotation{}) {
		t.Errorf("a run over the four classes answered %+v, want nothing found", run)
	}
	if row := activeRow(t, pool, expired); row.scheme != int16(domain.LookupLegacy) {
		t.Errorf("the expired legacy code is no longer the class's active row: %+v", row)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, revoked_at, created_by, created_at)
		VALUES ($1, sha256($2::bytea), 'EEEE', now() + interval '3 days', now() - interval '7 days', $3, now() - interval '27 days')`,
		live, "alone-earlier-"+live, liveTeacher); err != nil {
		t.Fatal(err)
	}
	const earlier = `SELECT to_jsonb(jc)::text FROM app.class_join_codes jc WHERE jc.class_id = $1 AND jc.code_hint = 'EEEE'`
	revokedEarlier := snapshot(t, pool, earlier, live)
	if run := rotateLegacy(t, rotationOver(pool, nil, live)); run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
		t.Fatalf("the run over the class with a usable legacy code answered %+v", run)
	}
	onlySealedCode(t, pool, live)
	if got := snapshot(t, pool, earlier, live); len(got) != 1 || !slices.Equal(got, revokedEarlier) {
		t.Errorf("the rotation touched a code of the class revoked long before:\n%v\nwas\n%v", got, revokedEarlier)
	}
}

func sealedInput(t *testing.T, classID string) domain.LegacyRotationInput {
	t.Helper()
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		t.Fatal(err)
	}
	codeID := uuid.Must(uuid.NewV7()).String()
	sealedCode, err := joinKeys.Seal(classID, codeID, code)
	if err != nil {
		t.Fatal(err)
	}
	return domain.LegacyRotationInput{
		ClassID: classID, CodeID: codeID, CodeHash: joinKeys.Hash(code), Ciphertext: sealedCode,
		KeyID: joinKeys.CurrentID(), Hint: domain.JoinCodes.Hint(code), Now: time.Now(),
	}
}

func TestARotationThatFailsLeavesTheLegacyCodeInPlaceAndIsNotRetryable(t *testing.T) {
	pool := newPool(t)
	classID, _, _ := legacyClassRow(t, pool)
	before := codesOf(t, pool, classID)

	in := sealedInput(t, classID)
	in.CodeHash = activeRow(t, pool, classID).hash
	wrote, err := repositories.NewPostgres(db.NewContext(pool)).RotateLegacyCode(context.Background(), in)
	if err == nil || wrote {
		t.Fatalf("a new row under a hash another row holds answered %v (%v), want the unique violation", wrote, err)
	}
	if errors.Is(err, domain.ErrRotationContended) {
		t.Errorf("a unique violation answers as a contended rotation, which the run would retry: %v", err)
	}
	if got := codesOf(t, pool, classID); !slices.Equal(got, before) {
		t.Errorf("the failed rotation left its revocation behind:\n%v\nwas\n%v", got, before)
	}
	if n := rotationAudits(t, pool, classID); n != 0 {
		t.Errorf("the failed rotation left %d audit row(s) behind", n)
	}
}

func TestASerializationFailureAnswersAsContendedAndWritesNothing(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	classID, _, _ := legacyClassRow(t, pool)
	strictPool, strictPid := ownConnection(t, "default_transaction_isolation", "repeatable read")
	repo := repositories.NewPostgres(db.NewContext(strictPool))
	release, gate := holding(t, pool, `SELECT pg_advisory_xact_lock(73819, 41)`)

	type outcome struct {
		wrote bool
		err   error
	}
	done := make(chan outcome, 1)
	in := sealedInput(t, classID)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		wrote, err := repo.RotateLegacyCode(ctx, in)
		done <- outcome{wrote, err}
	}()
	waitUntilBlocked(t, pool, strictPid, gate, "the rotation, its snapshot taken, on advisory key 41")
	if _, err := pool.Exec(ctx,
		`UPDATE app.class_join_codes SET uses_count = uses_count + 1 WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
		t.Fatal(err)
	}
	before := codesOf(t, pool, classID)
	release()

	got := finished(t, "the rotation", done)
	if !errors.Is(got.err, domain.ErrRotationContended) || got.wrote {
		t.Fatalf("a rotation whose code row changed under its snapshot answered %v (%v), want ErrRotationContended", got.wrote, got.err)
	}
	var pg *pgconn.PgError
	if !errors.As(got.err, &pg) || pg.Code != pgerrcode.SerializationFailure {
		t.Errorf("the refusal is %v, want it to carry the database's 40001", got.err)
	}
	if after := codesOf(t, pool, classID); !slices.Equal(after, before) {
		t.Errorf("the refused rotation wrote:\n%v\nwas\n%v", after, before)
	}

	wrote, err := repo.RotateLegacyCode(ctx, sealedInput(t, classID))
	if err != nil || !wrote {
		t.Fatalf("the same rotation tried again answered %v (%v), want the code replaced", wrote, err)
	}
	onlySealedCode(t, pool, classID)
}

func TestTheRotationIsAuditedAsTheSystemWithHintsOnly(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()
	classID, _, old := legacyClassRow(t, pool)

	at := time.Now().UTC().Truncate(time.Microsecond)
	rotation := rotationOver(pool, nil, classID)
	rotation.SetClock(func() time.Time { return at })
	if run := rotateLegacy(t, rotation); run.Rotated != 1 {
		t.Fatalf("the run answered %+v", run)
	}
	previous, current := rotatedPair(t, pool, classID)
	fresh := currentCode(t, svc, classID).Code

	rows, err := pool.Query(ctx, `
		SELECT actor_user_id::text, action, entity_id::text, occurred_at, host(ip), user_agent, diff::text
		  FROM app.audit_log
		 WHERE entity = 'class_join_code' AND entity_id = ANY($1::uuid[])`, []string{previous.id, current.id})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type entry struct {
		actor, ip, userAgent *string
		action, entityID     string
		occurredAt           time.Time
		diff                 string
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.actor, &e.action, &e.entityID, &e.occurredAt, &e.ip, &e.userAgent, &e.diff); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d audit rows for one rotation, want exactly 1", len(entries))
	}
	e := entries[0]
	if e.actor != nil {
		t.Errorf("the actor is %s, want none: the System rotated the code", *e.actor)
	}
	if e.action != "class.join_code_rotated" || e.entityID != current.id {
		t.Errorf("action %s on %s, want class.join_code_rotated on the new row %s", e.action, e.entityID, current.id)
	}
	if !e.occurredAt.Equal(at) || e.ip != nil || e.userAgent != nil {
		t.Errorf("occurred at %s from %v with %v, want %s and no request", e.occurredAt, e.ip, e.userAgent, at)
	}
	var diff map[string]any
	if err := json.Unmarshal([]byte(e.diff), &diff); err != nil {
		t.Fatalf("the diff is not an object: %v", err)
	}
	want := map[string]any{"reason": "legacy_rotation", "previousHint": previous.hint, "hint": current.hint}
	if !reflect.DeepEqual(diff, want) {
		t.Errorf("diff = %v, want %v", diff, want)
	}
	if previous.hint != domain.JoinCodes.Hint(domain.JoinCodes.Normalize(old)) || current.hint != domain.JoinCodes.Hint(fresh) {
		t.Errorf("the hints %s and %s are not the two codes' last four", previous.hint, current.hint)
	}
	canonical := domain.JoinCodes.Normalize(old)
	for what, needle := range map[string]string{
		"the new code":              fresh,
		"the new code, grouped":     domain.JoinCodes.Format(fresh),
		"the new code's other four": fresh[:4],
		"the old code":              canonical,
		"the old code, grouped":     old,
		"the old code's other four": canonical[:4],
		"the new hash":              hex.EncodeToString(current.hash),
		"the new hash in base64":    base64.StdEncoding.EncodeToString(current.hash),
		"the old hash":              hex.EncodeToString(previous.hash),
		"the old hash in base64":    base64.StdEncoding.EncodeToString(previous.hash),
		"the ciphertext":            hex.EncodeToString(current.ciphertext),
		"the ciphertext in base64":  base64.StdEncoding.EncodeToString(current.ciphertext),
	} {
		if needle == previous.hint || needle == current.hint {
			continue
		}
		if strings.Contains(e.diff, needle) {
			t.Errorf("the audit diff carries %s", what)
		}
	}
}

func TestTwoStartUpsAtOnceRotateEachClassOnce(t *testing.T) {
	pool := newPool(t)
	notes := notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
	names := []string{"Lớp A", "Lớp B", "Lớp C", "Lớp D", "Lớp E", "Lớp F", "Lớp G", "Lớp H", "Lớp I", "Lớp K"}
	teacherID, classIDs := teacherWithLegacyClasses(t, pool, names...)

	firstPool, firstPid := ownConnection(t)
	secondPool, secondPid := ownConnection(t)
	release, gate := holding(t, pool, `SELECT pg_advisory_xact_lock(73819, 41)`)
	first := startRotation(rotationOver(firstPool, notes.Commands.Notify, classIDs...))
	second := startRotation(rotationOver(secondPool, notes.Commands.Notify, classIDs...))
	waitUntilBlocked(t, pool, firstPid, gate, "the first start-up, on advisory key 41")
	waitUntilBlocked(t, pool, secondPid, gate, "the second start-up, on advisory key 41")
	for _, class := range classIDs {
		if rows := codeRows(t, pool, class); len(rows) != 1 || rows[0].revokedAt != nil {
			t.Fatalf("a start-up wrote to class %s before it held advisory key 41: %+v", class, rows)
		}
	}
	release()

	a, b := finished(t, "the first start-up", first), finished(t, "the second start-up", second)
	if a.err != nil || b.err != nil {
		t.Fatalf("the runs failed: %v, %v", a.err, b.err)
	}
	if a.run.Found != 10 || b.run.Found != 10 {
		t.Errorf("the runs found %d and %d classes, want both to have listed all ten before either wrote", a.run.Found, b.run.Found)
	}
	if a.run.Rotated+b.run.Rotated != 10 || a.run.Failed != 0 || b.run.Failed != 0 {
		t.Errorf("the runs answered %+v and %+v, want ten rotations between them and no failure", a.run, b.run)
	}
	for _, class := range classIDs {
		rotatedPair(t, pool, class)
		onlySealedCode(t, pool, class)
	}
	if n := rotationAudits(t, pool, classIDs...); n != 10 {
		t.Errorf("%d audit rows, want one for each of the ten classes", n)
	}

	page, err := notes.Queries.List.Handle(context.Background(), notificationsquery.List{UserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("the teacher holds %d notifications, want one whatever way the runs split the classes", len(page.Items))
	}
	var params struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(page.Items[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Count != 10 {
		t.Errorf("the notification counts %d classes, want the ten the two runs rotated between them", params.Count)
	}
	if told := a.run.Teachers + b.run.Teachers; told < 1 || told > 2 || a.run.NotifyFailed+b.run.NotifyFailed != 0 {
		t.Errorf("the runs told the teacher %d times with %d failures", told, a.run.NotifyFailed+b.run.NotifyFailed)
	}
}

func TestATeachersOwnRotateAndTheRotationLeaveOneSealedCode(t *testing.T) {
	pool := newPool(t)
	rotate := func(over *pgxpool.Pool, classID, teacherID string) <-chan error {
		done := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := withKeys(over, joinKeys).Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
			done <- err
		}()
		return done
	}

	t.Run("the teacher first", func(t *testing.T) {
		classID, teacherID, _ := legacyClassRow(t, pool)
		teacherPool, teacherPid := ownConnection(t)
		jobPool, jobPid := ownConnection(t)
		release, gate := holding(t, pool, `SELECT 1 FROM app.users WHERE id = $1 FOR UPDATE`, teacherID)

		teacher := rotate(teacherPool, classID, teacherID)
		waitUntilBlocked(t, pool, teacherPid, gate, "the teacher's Rotate, on its new row's creator")
		job := startRotation(rotationOver(jobPool, nil, classID))
		waitUntilBlocked(t, pool, jobPid, teacherPid, "the rotation, on the class row")
		release()

		if err := finished(t, "the teacher's Rotate", teacher); err != nil {
			t.Fatalf("the teacher's Rotate: %v", err)
		}
		got := finished(t, "the rotation", job)
		if got.err != nil || got.run != (domain.LegacyRotation{Found: 1}) {
			t.Errorf("the rotation answered %+v (%v), want the class found and left to its teacher's code", got.run, got.err)
		}
		onlySealedCode(t, pool, classID)
		if rows := codeRows(t, pool, classID); len(rows) != 2 {
			t.Errorf("the class holds %d code rows, want the legacy one and the teacher's", len(rows))
		}
		if n := rotationAudits(t, pool, classID); n != 0 {
			t.Errorf("%d rotation audit rows for a class its teacher rotated first", n)
		}
	})

	t.Run("the rotation first, held at its second read of the code", func(t *testing.T) {
		classID, teacherID, _ := legacyClassRow(t, pool)
		teacherPool, teacherPid := ownConnection(t)
		jobPool, jobPid := ownConnection(t)
		release, gate := holding(t, pool, `SELECT 1 FROM app.class_join_codes WHERE class_id = $1 FOR SHARE`, classID)

		job := startRotation(rotationOver(jobPool, nil, classID))
		waitUntilBlocked(t, pool, jobPid, gate, "the rotation, holding the class row, on the code row")
		teacher := rotate(teacherPool, classID, teacherID)
		waitUntilBlocked(t, pool, teacherPid, jobPid, "the teacher's Rotate, on the class row")
		release()

		got := finished(t, "the rotation", job)
		if got.err != nil || got.run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
			t.Errorf("the rotation answered %+v (%v), want the class rotated at its first attempt", got.run, got.err)
		}
		if err := finished(t, "the teacher's Rotate", teacher); err != nil {
			t.Fatalf("the teacher's Rotate, which waited for the class row and must not meet the rotation again: %v", err)
		}
		active := onlySealedCode(t, pool, classID)
		if rows := codeRows(t, pool, classID); len(rows) != 3 {
			t.Errorf("the class holds %d code rows, want the legacy one, the rotation's and the teacher's", len(rows))
		}
		var auditedID string
		if err := pool.QueryRow(context.Background(), `
			SELECT a.entity_id::text FROM app.audit_log a
			 WHERE a.diff->>'reason' = 'legacy_rotation'
			   AND a.entity_id IN (SELECT id FROM app.class_join_codes WHERE class_id = $1)`, classID).Scan(&auditedID); err != nil {
			t.Fatalf("the rotation's audit row: %v", err)
		}
		if auditedID == active.id {
			t.Error("the active code is the rotation's, although the teacher rotated after it")
		}
	})
}

func TestAStudentRedeemingTheOldCodeAndTheRotationNeverLeaveALegacyCode(t *testing.T) {
	pool := newPool(t)

	t.Run("the redemption first, while the rotation waits for advisory key 41", func(t *testing.T) {
		classID, _, old := legacyClassRow(t, pool)
		legacy := activeRow(t, pool, classID)
		_, _, student := makeClassRow(t, pool)
		jobPool, jobPid := ownConnection(t)
		release, gate := holding(t, pool, `SELECT pg_advisory_xact_lock(73819, 41)`)

		job := startRotation(rotationOver(jobPool, nil, classID))
		waitUntilBlocked(t, pool, jobPid, gate, "the rotation, on advisory key 41")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		joined, err := newSvc(t, pool).Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: student, Code: old})
		if err != nil || joined.Outcome != domain.PreviewOK {
			t.Fatalf("a redemption while the rotation waited for its advisory key: %+v (%v); the rotation must hold no row before the key", joined, err)
		}
		release()

		got := finished(t, "the rotation", job)
		if got.err != nil || got.run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
			t.Errorf("the rotation answered %+v (%v), want the class rotated", got.run, got.err)
		}
		onlySealedCode(t, pool, classID)
		if through := joinedThrough(t, pool, classID, student); through != legacy.id {
			t.Errorf("the student joined through %s, want the legacy row %s", through, legacy.id)
		}
	})

	t.Run("the rotation first, and the student is refused as revoked", func(t *testing.T) {
		classID, teacherID, old := legacyClassRow(t, pool)
		_, _, student := makeClassRow(t, pool)
		jobPool, jobPid := ownConnection(t)
		studentPool, studentPid := ownConnection(t)
		release, gate := holding(t, pool, `SELECT 1 FROM app.users WHERE id = $1 FOR UPDATE`, teacherID)

		job := startRotation(rotationOver(jobPool, nil, classID))
		waitUntilBlocked(t, pool, jobPid, gate, "the rotation, on its new row's creator")
		type enrolment struct {
			result domain.EnrolResult
			err    error
		}
		enrol := make(chan enrolment, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := withKeys(studentPool, joinKeys).Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: student, Code: old})
			enrol <- enrolment{result, err}
		}()
		waitUntilBlocked(t, pool, studentPid, jobPid, "the redemption, on the code row")
		release()

		got := finished(t, "the rotation", job)
		if got.err != nil || got.run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
			t.Errorf("the rotation answered %+v (%v), want the class rotated", got.run, got.err)
		}
		joined := finished(t, "the redemption", enrol)
		if joined.err != nil || joined.result != (domain.EnrolResult{Outcome: domain.PreviewRevoked}) {
			t.Errorf("the redemption answered %+v (%v), want PreviewRevoked", joined.result, joined.err)
		}
		onlySealedCode(t, pool, classID)
		if n := memberCount(t, pool, classID); n != 1 {
			t.Errorf("the class has %d members, want the refused student left out", n)
		}
	})

	t.Run("a deadlock the rotation loses is retried in the same run", func(t *testing.T) {
		const rounds = 3
		for round := 1; ; round++ {
			classID, _, old := legacyClassRow(t, pool)
			legacy := activeRow(t, pool, classID)
			m := newMember(t)
			dropUser(t, pool, m.Email)
			jobPool, jobPid := ownConnection(t)
			studentPool, studentPid := ownConnection(t)
			release, gate := holding(t, pool, `
				INSERT INTO app.users (email, full_name, role_id)
				VALUES ($1, 'Giữ chỗ', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, m.Email)

			type enrolment struct {
				result domain.EnrolResult
				err    error
			}
			enrol := make(chan enrolment, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				result, err := withKeys(studentPool, joinKeys).Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: old})
				enrol <- enrolment{result, err}
			}()
			waitUntilBlocked(t, pool, studentPid, gate, "the redemption, holding the code row, on its new account's email")
			job := startRotation(rotationOver(jobPool, nil, classID))
			waitUntilBlocked(t, pool, jobPid, studentPid, "the rotation, holding the class row, on the code row")
			release()

			joined := finished(t, "the redemption", enrol)
			got := finished(t, "the rotation", job)
			if got.err != nil || got.run != (domain.LegacyRotation{Found: 1, Rotated: 1}) {
				t.Fatalf("round %d: the rotation answered %+v (%v), want the class rotated and none failed", round, got.run, got.err)
			}
			onlySealedCode(t, pool, classID)

			var pg *pgconn.PgError
			if errors.As(joined.err, &pg) && pg.Code == pgerrcode.DeadlockDetected {
				if round == rounds {
					t.Fatalf("in %d rounds the database always chose the redemption as the deadlock victim, so the rotation's retry is unproven", rounds)
				}
				continue
			}
			if joined.err != nil || joined.result.Outcome != domain.PreviewOK {
				t.Fatalf("round %d: the redemption answered %+v (%v), want the student enrolled", round, joined.result, joined.err)
			}
			if through := joinedThrough(t, pool, classID, joined.result.UserID); through != legacy.id {
				t.Errorf("round %d: the student joined through %s, want the legacy row %s", round, through, legacy.id)
			}
			return
		}
	})
}

func TestASecondRunWritesNothing(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	notes := notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
	teacherID, classIDs := teacherWithLegacyClasses(t, pool, "Lớp chạy lại 1", "Lớp chạy lại 2")
	svc := rotationOver(pool, notes.Commands.Notify, classIDs...)

	if run := rotateLegacy(t, svc); run != (domain.LegacyRotation{Found: 2, Rotated: 2, Teachers: 1}) {
		t.Fatalf("the first run answered %+v", run)
	}
	if _, err := notes.Commands.MarkAllRead.Handle(ctx, notificationscommand.MarkAllRead{UserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	codes := map[string][]string{}
	for _, class := range classIDs {
		codes[class] = codesOf(t, pool, class)
	}
	notices := snapshot(t, pool, `SELECT to_jsonb(n)::text FROM app.notifications n WHERE n.user_id = $1 ORDER BY n.id`, teacherID)
	audits := rotationAudits(t, pool, classIDs...)
	if len(notices) != 1 || audits != 2 {
		t.Fatalf("after the first run the teacher holds %d notifications and the classes %d audit rows, want 1 and 2", len(notices), audits)
	}

	if run := rotateLegacy(t, svc); run != (domain.LegacyRotation{}) {
		t.Errorf("the second run answered %+v, want nothing found", run)
	}
	for _, class := range classIDs {
		if got := codesOf(t, pool, class); !slices.Equal(got, codes[class]) {
			t.Errorf("the second run changed the codes of class %s:\n%v\nwere\n%v", class, got, codes[class])
		}
	}
	if n := rotationAudits(t, pool, classIDs...); n != audits {
		t.Errorf("the second run wrote %d audit row(s)", n-audits)
	}
	if got := snapshot(t, pool, `SELECT to_jsonb(n)::text FROM app.notifications n WHERE n.user_id = $1 ORDER BY n.id`, teacherID); !slices.Equal(got, notices) {
		t.Errorf("the second run changed the teacher's notifications:\n%v\nwere\n%v", got, notices)
	}
}

type storedNotice struct {
	id, kind, dedupeKey string
	params, target      string
	readAt              *time.Time
}

func noticesOf(t *testing.T, pool *pgxpool.Pool, userID string) []storedNotice {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id::text, kind, dedupe_key, params::text, target::text, read_at
		  FROM app.notifications WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []storedNotice
	for rows.Next() {
		var n storedNotice
		if err := rows.Scan(&n.id, &n.kind, &n.dedupeKey, &n.params, &n.target, &n.readAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func rotatedNotice(t *testing.T, raw string) (int, []string) {
	t.Helper()
	var params map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 {
		t.Errorf("the params are %s, want count and classNames and nothing else", raw)
	}
	var count int
	var names []string
	if err := json.Unmarshal(params["count"], &count); err != nil {
		t.Fatalf("count in %s: %v", raw, err)
	}
	if err := json.Unmarshal(params["classNames"], &names); err != nil {
		t.Fatalf("classNames in %s: %v", raw, err)
	}
	return count, names
}

func classNames(t *testing.T, pool *pgxpool.Pool, classIDs []string) []string {
	t.Helper()
	names := make([]string, len(classIDs))
	for i, class := range classIDs {
		if err := pool.QueryRow(context.Background(), `SELECT name FROM app.classes WHERE id = $1`, class).Scan(&names[i]); err != nil {
			t.Fatal(err)
		}
	}
	return names
}

func TestEachTeacherIsToldOnceOfTheirRotatedClasses(t *testing.T) {
	pool := newPool(t)
	notes := notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
	lan, lanClasses := teacherWithLegacyClasses(t, pool, "Lớp G", "Lớp B", "Lớp E", "Lớp A", "Lớp D", "Lớp C", "Lớp F")
	mai, maiClasses := teacherWithLegacyClasses(t, pool, "Lớp 11B")

	run := rotateLegacy(t, rotationOver(pool, notes.Commands.Notify, append(slices.Clone(lanClasses), maiClasses...)...))
	if run != (domain.LegacyRotation{Found: 8, Rotated: 8, Teachers: 2}) {
		t.Fatalf("the run answered %+v", run)
	}

	inNameOrder := classNames(t, pool, []string{lanClasses[3], lanClasses[1], lanClasses[5], lanClasses[4], lanClasses[2]})
	for teacher, want := range map[string]struct {
		count int
		names []string
	}{
		lan: {7, inNameOrder},
		mai: {1, classNames(t, pool, maiClasses)},
	} {
		stored := noticesOf(t, pool, teacher)
		if len(stored) != 1 {
			t.Fatalf("teacher %s holds %d notifications, want one", teacher, len(stored))
		}
		n := stored[0]
		if n.kind != "join_codes.rotated" || n.dedupeKey != "join_codes.rotated:legacy" || n.target != `{"route": "classes"}` || n.readAt != nil {
			t.Errorf("the notification is %+v, want join_codes.rotated under join_codes.rotated:legacy, the classes route, unread", n)
		}
		if count, names := rotatedNotice(t, n.params); count != want.count || !slices.Equal(names, want.names) {
			t.Errorf("the notification counts %d classes and names %v, want %d and %v", count, names, want.count, want.names)
		}
		page, err := notes.Queries.List.Handle(context.Background(), notificationsquery.List{UserID: teacher})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Kind != notificationsdomain.JoinCodesRotated || page.Items[0].ReadAt != nil ||
			page.Items[0].Target == nil || *page.Items[0].Target != (notificationsdomain.Target{Route: notificationsdomain.RouteClasses}) {
			t.Errorf("the teacher's list reads %+v, want the one unread notice that opens Classes", page.Items)
		}
	}
}

func TestATeacherWhoseClassesWereSplitOverTwoRunsHoldsOneSummedNotification(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	notes := notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
	teacherID, classIDs := teacherWithLegacyClasses(t, pool, "Lớp A", "Lớp B", "Lớp C", "Lớp D", "Lớp E", "Lớp F")

	if run := rotateLegacy(t, rotationOver(pool, notes.Commands.Notify, classIDs[:2]...)); run != (domain.LegacyRotation{Found: 2, Rotated: 2, Teachers: 1}) {
		t.Fatalf("the first run answered %+v", run)
	}
	first := noticesOf(t, pool, teacherID)
	if len(first) != 1 {
		t.Fatalf("after the first run the teacher holds %d notifications", len(first))
	}
	if _, err := notes.Commands.MarkAllRead.Handle(ctx, notificationscommand.MarkAllRead{UserID: teacherID}); err != nil {
		t.Fatal(err)
	}

	if run := rotateLegacy(t, rotationOver(pool, notes.Commands.Notify, classIDs...)); run != (domain.LegacyRotation{Found: 4, Rotated: 4, Teachers: 1}) {
		t.Fatalf("the second run answered %+v", run)
	}
	stored := noticesOf(t, pool, teacherID)
	if len(stored) != 1 || stored[0].id != first[0].id {
		t.Fatalf("the teacher holds %+v, want the first run's one notification", stored)
	}
	if stored[0].readAt != nil {
		t.Error("the notification stayed read although the second run rotated four more classes")
	}
	if count, names := rotatedNotice(t, stored[0].params); count != 6 || !slices.Equal(names, classNames(t, pool, classIDs[2:])) {
		t.Errorf("the notification counts %d classes and names %v, want the six of both runs and the second run's four names", count, names)
	}
}
