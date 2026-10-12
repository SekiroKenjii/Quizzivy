//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

type profileCleanupReport struct {
	ChildMode       string
	ChildStage      string
	ChildFault      string
	CommitAttempted bool
	CommitCanceled  bool
	CommitError     string
	ID              string
	Inserted        bool
	Events          []string
	Errors          []string
	Audits          []string
	Joined          bool
	WriterObserved  bool
	Released        bool
	Deleted         int64
	Absent          bool
	AuditRetained   bool
	PoolClosed      bool
}

type profileRace struct {
	t               *testing.T
	pool            *pgxpool.Pool
	ctx             context.Context
	cancel          context.CancelFunc
	id              string
	holder          pgx.Tx
	conn            *pgxpool.Conn
	done            chan struct{}
	writerErr       error
	observed        bool
	report          profileCleanupReport
	fault           string
	writerDelay     time.Duration
	observerFailure bool
}

func newProfileRace(t *testing.T) *profileRace {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	r := &profileRace{t: t, ctx: ctx, cancel: cancel}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	r.pool = pool
	t.Cleanup(r.cleanup)
	if err := pool.QueryRow(ctx, `SELECT uuidv7()::text`).Scan(&r.id); err != nil {
		t.Fatal(err)
	}
	r.report.ID = r.id
	t.Logf("owned race user=%s cleanup registered before INSERT", r.id)
	return r
}

func (r *profileRace) insert(name string) {
	r.t.Helper()
	tag, err := r.pool.Exec(r.ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES ($1::uuid,'r407-owned-'||$1::text||'@example.com',$2,(SELECT id FROM app.roles WHERE builtin_key='student'))`, r.id, name)
	if err != nil {
		r.t.Fatal(err)
	}
	if tag.RowsAffected() != 1 {
		r.t.Fatalf("owned insert affected=%d", tag.RowsAffected())
	}
	r.report.Inserted = true
}

func (r *profileRace) begin() pgx.Tx {
	r.t.Helper()
	holder, err := r.pool.Begin(r.ctx)
	if err != nil {
		r.t.Fatal(err)
	}
	r.holder = holder
	return holder
}

func (r *profileRace) startWriter(write func(context.Context, *repositories.Users) error) {
	r.t.Helper()
	conn, err := r.pool.Acquire(r.ctx)
	if err != nil {
		r.t.Fatal(err)
	}
	r.conn = conn
	r.done = make(chan struct{})
	go func() {
		r.writerErr = write(r.ctx, repositories.NewUsers(db.NewContext(conn)))
		if r.writerDelay > 0 {
			time.Sleep(r.writerDelay)
		}
		close(r.done)
	}()
	waitProfileLock(r.t, r.ctx, profileWorld{pool: r.pool}, int(conn.Conn().PgConn().PID()))
	if r.observerFailure {
		if _, err := r.pool.Exec(r.ctx, `SELECT missing_r407_observer_column FROM app.users WHERE id=$1::uuid`, r.id); err != nil {
			r.t.Fatalf("unrelated writer observer query failed: %v", err)
		}
		r.t.Fatal("unrelated invalid observer query unexpectedly succeeded")
	}
}

func (r *profileRace) result() error {
	r.t.Helper()
	<-r.done
	r.report.Joined = true
	r.observed = true
	r.report.WriterObserved = true
	return r.writerErr
}

func (r *profileRace) failure(stage string, err error) {
	if err == nil {
		return
	}
	message := fmt.Sprintf("%s: %v", stage, err)
	r.report.Errors = append(r.report.Errors, message)
	r.t.Errorf("owned fixture cleanup %s", message)
}

func (r *profileRace) cleanup() {
	r.cancel()
	r.report.Events = append(r.report.Events, "cancel")
	r.rollbackHolder()
	r.joinWriter()
	if r.conn != nil {
		if !r.report.Joined {
			r.failure("release", errors.New("writer not joined"))
			return
		}
		r.conn.Release()
		r.report.Released = true
	}
	r.report.Events = append(r.report.Events, "release")
	r.captureAudits()
	r.removeUser()
	r.verifyAuditRetention()
	r.pool.Close()
	r.report.PoolClosed = true
	r.report.Events = append(r.report.Events, "pool-close")
	r.t.Logf("owned cleanup report=%s", profileReportJSON(r.report))
}

func (r *profileRace) rollbackHolder() {
	r.report.Events = append(r.report.Events, "rollback")
	if r.holder == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if r.fault == "rollback" {
		cancel()
	}
	err := r.holder.Rollback(ctx)
	if !errors.Is(err, pgx.ErrTxClosed) {
		r.failure("rollback", err)
	}
}

func (r *profileRace) joinWriter() {
	r.report.Events = append(r.report.Events, "join")
	if r.done == nil {
		return
	}
	timeout := 5 * time.Second
	if r.fault == "join" {
		timeout = time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case <-r.done:
	case <-ctx.Done():
		r.failure("join", ctx.Err())
		drain, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		select {
		case <-r.done:
		case <-drain.Done():
			r.failure("join completion", drain.Err())
			return
		}
	}
	r.report.Joined = true
	if !r.observed && r.writerErr != nil && !errors.Is(r.writerErr, context.Canceled) && !errors.Is(r.writerErr, context.DeadlineExceeded) {
		r.failure("writer", r.writerErr)
	}
}

func (r *profileRace) captureAudits() {
	if r.id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	statement := `SELECT id::text FROM app.audit_log WHERE entity='user' AND entity_id=$1::uuid ORDER BY id`
	if r.fault == "audit-query" {
		statement = `SELECT missing_fixture_column FROM app.audit_log WHERE entity_id=$1::uuid`
	}
	rows, err := r.pool.Query(ctx, statement, r.id)
	if err != nil {
		r.failure("audit query", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.failure("audit scan", err)
			return
		}
		r.report.Audits = append(r.report.Audits, id)
	}
	r.failure("audit rows", rows.Err())
}

func (r *profileRace) removeUser() {
	r.report.Events = append(r.report.Events, "delete")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if r.report.Inserted {
		tag, err := r.pool.Exec(ctx, `DELETE FROM app.users WHERE id=$1::uuid`, r.id)
		r.failure("delete", err)
		if err == nil {
			r.report.Deleted = tag.RowsAffected()
			if r.fault == "delete-query" {
				_, err := r.pool.Exec(ctx, `DELETE FROM app.users WHERE id=$1::uuid AND missing_fixture_column IS NULL`, r.id)
				r.failure("delete query", err)
			}
			if r.fault == "delete-rows" {
				repeated, err := r.pool.Exec(ctx, `DELETE FROM app.users WHERE id=$1::uuid`, r.id)
				r.failure("repeat delete", err)
				if err == nil {
					r.report.Deleted = repeated.RowsAffected()
				}
			}
			if r.report.Deleted != 1 {
				r.failure("delete rows", fmt.Errorf("affected=%d, want1", r.report.Deleted))
			}
		}
	}
	if r.id == "" {
		return
	}
	statement := `SELECT count(*) FROM app.users WHERE id=$1::uuid`
	if r.fault == "absence-query" {
		statement = `SELECT missing_fixture_column FROM app.users WHERE id=$1::uuid`
	}
	var count int
	err := r.pool.QueryRow(ctx, statement, r.id).Scan(&count)
	r.failure("absence query", err)
	if err == nil {
		r.report.Absent = count == 0
		if count != 0 {
			r.failure("absence", fmt.Errorf("remaining=%d", count))
		}
	}
}

func (r *profileRace) verifyAuditRetention() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range r.report.Audits {
		var preserved bool
		err := r.pool.QueryRow(ctx, `SELECT entity='user' AND entity_id=$2::uuid AND actor_user_id IS NULL FROM app.audit_log WHERE id=$1::bigint`, id, r.id).Scan(&preserved)
		r.failure("audit retention query", err)
		if err == nil && !preserved {
			r.failure("audit retention", fmt.Errorf("audit %s changed identity/actor", id))
		}
	}
	r.report.AuditRetained = len(r.report.Errors) == 0
}

func profileReportJSON(report profileCleanupReport) string {
	data, err := json.Marshal(report)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestProfileFixtureLifecycleChild(t *testing.T) {
	mode := os.Getenv("R407_FIXTURE_CHILD")
	if mode == "" {
		return
	}
	var r *profileRace
	t.Cleanup(func() {
		if r != nil {
			if err := os.WriteFile(os.Getenv("R407_FIXTURE_REPORT"), []byte(profileReportJSON(r.report)), 0600); err != nil {
				t.Errorf("report write: %v", err)
			}
		}
	})
	r = newProfileRace(t)
	r.observerFailure = os.Getenv("R407_UNRELATED_FAILURE") == "1"
	r.report.ChildMode = mode
	if mode == "before-insert" {
		r.report.ChildStage = "allocated"
		t.Fatal(profileChildMarker(mode, r.report.ChildStage))
	}
	r.insert("Owned lifecycle")
	if mode == "after-insert" {
		r.report.ChildStage = "inserted"
		t.Fatal(profileChildMarker(mode, r.report.ChildStage))
	}
	holder := r.begin()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if _, err := repositories.NewUsers(db.NewContext(holder)).UpdatePreferences(r.ctx, domain.PreferencesRecord{UserID: r.id, Patch: json.RawMessage(`{"theme":"dark"}`), Now: now}); err != nil {
		t.Fatal(err)
	}
	if mode == "join" {
		r.writerDelay = 100 * time.Millisecond
	}
	r.startWriter(func(ctx context.Context, users *repositories.Users) error {
		_, err := users.UpdatePreferences(ctx, domain.PreferencesRecord{UserID: r.id, Patch: json.RawMessage(`{"compactTables":true}`), Now: now})
		return err
	})
	switch mode {
	case "fatal":
		r.report.ChildStage = "writer-blocked"
		t.Fatal(profileChildMarker(mode, r.report.ChildStage))
	case "rollback", "join":
		r.fault = mode
		r.report.ChildFault = mode
		r.report.ChildStage = "writer-blocked"
		t.Fatal(profileChildMarker(mode, r.report.ChildStage))
	}
	if mode == "commit-failure" {
		r.cancel()
		r.report.CommitAttempted = true
		if err := holder.Commit(r.ctx); err != nil {
			r.report.CommitError = err.Error()
			r.report.CommitCanceled = errors.Is(err, context.Canceled)
			r.report.ChildStage = "canceled-commit-failed"
			t.Fatalf("%s: %v", profileChildMarker(mode, r.report.ChildStage), err)
		}
		t.Fatal("canceled commit unexpectedly succeeded")
	}
	if err := holder.Commit(r.ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.result(); err != nil {
		t.Fatal(err)
	}
	r.fault = mode
	if mode != "normal" {
		r.report.ChildFault = mode
	}
	r.report.ChildStage = "writer-observed"
	t.Log(profileChildMarker(mode, r.report.ChildStage))
}

func TestProfileFixtureLifecycleSuccessAndFatalPathsOwnExactRemoval(t *testing.T) {
	for _, mode := range []string{"normal", "fatal", "before-insert", "after-insert", "commit-failure", "rollback", "join", "audit-query", "absence-query", "delete-query", "delete-rows"} {
		t.Run(mode, func(t *testing.T) {
			report, output, childErr := runProfileLifecycleChild(t, mode, false)
			if err := verifyProfileChildFailure(mode, childErr, report, output); err != nil {
				t.Fatal(err)
			}
			expected := []string{"cancel", "rollback", "join", "release", "delete", "pool-close"}
			if strings.Join(report.Events, ",") != strings.Join(expected, ",") || !report.PoolClosed {
				t.Fatalf("cleanup order=%+v", report)
			}
			if mode != "before-insert" && mode != "after-insert" && (!report.Joined || !report.Released || !report.Inserted || (mode != "delete-rows" && report.Deleted != 1 || mode == "delete-rows" && report.Deleted != 0)) {
				t.Fatalf("writer/user ownership=%+v", report)
			}
			if mode == "after-insert" && (!report.Inserted || report.Deleted != 1 || report.Joined || report.Released) {
				t.Fatalf("partial committed setup cleanup=%+v", report)
			}
			if mode == "before-insert" && (report.Inserted || report.Deleted != 0) {
				t.Fatalf("pre-insert cleanup=%+v", report)
			}
			fault := mode == "rollback" || mode == "join" || mode == "audit-query" || mode == "absence-query" || mode == "delete-query" || mode == "delete-rows"
			if fault != (len(report.Errors) > 0) {
				t.Fatalf("unhandled/masked cleanup errors=%+v", report)
			}
			if fault && (len(report.Errors) != 1 || !strings.HasPrefix(report.Errors[0], map[string]string{"rollback": "rollback:", "join": "join:", "audit-query": "audit query:", "absence-query": "absence query:", "delete-query": "delete query:", "delete-rows": "delete rows:"}[mode])) {
				t.Fatalf("unexpected additional cleanup errors=%+v", report)
			}
			if (mode == "fatal" || mode == "rollback" || mode == "join" || mode == "commit-failure") && report.WriterObserved {
				t.Fatal("failure did not precede ordinary result receive")
			}
			if mode == "normal" && !report.WriterObserved {
				t.Fatal("normal result was never observed")
			}
			if mode == "absence-query" {
				if report.Absent {
					t.Fatal("failed absence query was masked")
				}
			} else if !report.Absent {
				t.Fatalf("owned user remained=%+v", report)
			}
			if (mode == "normal" || mode == "absence-query" || mode == "delete-query" || mode == "delete-rows") && len(report.Audits) != 2 {
				t.Fatalf("expected retained audits=%+v", report)
			}
			if (mode == "fatal" || mode == "before-insert" || mode == "after-insert" || mode == "commit-failure") && len(report.Audits) != 0 {
				t.Fatalf("rolled-back fixture retained unexpected audit=%+v", report)
			}
			if (mode == "normal" || mode == "fatal" || mode == "before-insert" || mode == "after-insert" || mode == "commit-failure") && !report.AuditRetained {
				t.Fatalf("retention check failed=%+v", report)
			}
			pool, poolErr := pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_URL"))
			if poolErr != nil {
				t.Fatal(poolErr)
			}
			defer pool.Close()
			var remaining int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE id=$1::uuid`, report.ID).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("independent exact absence=%d %v", remaining, err)
			}
		})
	}
}

func runProfileLifecycleChild(t *testing.T, mode string, unrelated bool) (profileCleanupReport, []byte, error) {
	t.Helper()
	reportPath := filepath.Join(t.TempDir(), "cleanup.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProfileFixtureLifecycleChild$", "-test.v", "-test.timeout=25s")
	command.Env = append(os.Environ(), "R407_FIXTURE_CHILD="+mode, "R407_FIXTURE_REPORT="+reportPath, fmt.Sprintf("R407_UNRELATED_FAILURE=%d", map[bool]int{false: 0, true: 1}[unrelated]))
	output, err := command.CombinedOutput()
	t.Logf("raw child mode=%s unrelated=%t output:\n%s", mode, unrelated, output)
	if ctx.Err() != nil {
		t.Fatalf("child deadline: %v", ctx.Err())
	}
	data, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var report profileCleanupReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	return report, output, err
}

func profileChildMarker(mode, stage string) string {
	return "R407 intended child mode=" + mode + " stage=" + stage
}

func verifyProfileChildFailure(mode string, err error, report profileCleanupReport, output []byte) error {
	stage, ok := map[string]string{
		"normal": "writer-observed", "before-insert": "allocated", "after-insert": "inserted",
		"fatal": "writer-blocked", "rollback": "writer-blocked", "join": "writer-blocked",
		"commit-failure": "canceled-commit-failed", "audit-query": "writer-observed",
		"absence-query": "writer-observed", "delete-query": "writer-observed", "delete-rows": "writer-observed",
	}[mode]
	if !ok {
		return fmt.Errorf("unknown child mode %q", mode)
	}
	if mode == "normal" {
		if err != nil {
			return fmt.Errorf("normal child failed: %w", err)
		}
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("induced failure did not return the expected test exit1: %v", err)
		}
	}
	text := string(output)
	if strings.Contains(text, "panic:") || strings.Contains(text, "fatal error:") || strings.Contains(text, "unrelated writer observer query failed:") || strings.Contains(text, "canceled commit unexpectedly succeeded") {
		return errors.New("child failed outside the intended branch")
	}
	if report.ChildMode != mode || report.ChildStage != stage || !strings.Contains(text, profileChildMarker(mode, stage)) {
		return fmt.Errorf("child did not prove mode=%s stage=%s: reported mode=%s stage=%s", mode, stage, report.ChildMode, report.ChildStage)
	}
	fault := mode == "rollback" || mode == "join" || mode == "audit-query" || mode == "absence-query" || mode == "delete-query" || mode == "delete-rows"
	if fault && report.ChildFault != mode || !fault && report.ChildFault != "" {
		return errors.New("child did not prove the intended cleanup fault was armed")
	}
	if mode == "commit-failure" {
		if !report.CommitAttempted || !report.CommitCanceled || report.CommitError == "" || !strings.Contains(text, profileChildMarker(mode, stage)+": "+report.CommitError) {
			return errors.New("child did not prove an actual canceled commit error")
		}
	} else if report.CommitAttempted || report.CommitCanceled || report.CommitError != "" {
		return errors.New("child reported an unexpected commit failure")
	}
	return nil
}

func TestProfileFixtureVerifierRejectsUnrelatedFailureAfterWriterCreation(t *testing.T) {
	for _, mode := range []string{"fatal", "commit-failure"} {
		t.Run(mode, func(t *testing.T) {
			report, output, err := runProfileLifecycleChild(t, mode, true)
			if !strings.Contains(string(output), "unrelated writer observer query failed:") {
				t.Fatal("actual unrelated observer SQL failure was not reached")
			}
			if strings.Join(report.Events, ",") != "cancel,rollback,join,release,delete,pool-close" || !report.Inserted || !report.Joined || !report.Released || report.WriterObserved || report.Deleted != 1 || !report.Absent || !report.AuditRetained || !report.PoolClosed || len(report.Errors) != 0 || len(report.Audits) != 0 {
				t.Fatalf("unrelated child cleanup was not successful: %+v", report)
			}
			pool, poolErr := pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_URL"))
			if poolErr != nil {
				t.Fatal(poolErr)
			}
			defer pool.Close()
			var remaining int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE id=$1::uuid`, report.ID).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("independent unrelated user absence=%d %v", remaining, err)
			}
			if rejection := verifyProfileChildFailure(mode, err, report, output); rejection == nil {
				t.Fatal("unrelated child incorrectly accepted as intended failure coverage")
			} else {
				t.Logf("same child verifier rejected unrelated failure: %v", rejection)
			}
		})
	}
}

func TestProfileFixtureVerifierRequiresReachedStageAndActualCommitError(t *testing.T) {
	for _, mode := range []string{"fatal", "commit-failure"} {
		t.Run(mode, func(t *testing.T) {
			report, output, childErr := runProfileLifecycleChild(t, mode, false)
			if err := verifyProfileChildFailure(mode, childErr, report, output); err != nil {
				t.Fatal(err)
			}
			if !report.Absent || !report.AuditRetained || !report.PoolClosed || report.Deleted != 1 || len(report.Errors) != 0 {
				t.Fatalf("actual proof child cleanup=%+v", report)
			}
			for _, omission := range []string{"mode", "stage", "marker", "panic", "deadline", "wrong-failure", "exit"} {
				t.Run(omission, func(t *testing.T) {
					altered, raw, result := report, output, childErr
					switch omission {
					case "mode":
						altered.ChildMode = "wrong-mode"
					case "stage":
						altered.ChildStage = ""
					case "marker":
						raw = []byte(strings.ReplaceAll(string(output), profileChildMarker(mode, report.ChildStage), "wrong intended branch"))
					case "panic":
						raw = append(append([]byte{}, output...), []byte("\npanic: unrelated failure")...)
					case "deadline":
						result = context.DeadlineExceeded
					case "wrong-failure":
						result = errors.New("unrelated child failure")
					case "exit":
						result = nil
					}
					if verifyProfileChildFailure(mode, result, altered, raw) == nil {
						t.Fatalf("accepted missing proof %s", omission)
					}
				})
			}
			if mode == "commit-failure" {
				for _, omission := range []string{"attempt", "cancellation", "error"} {
					t.Run(omission, func(t *testing.T) {
						altered := report
						switch omission {
						case "attempt":
							altered.CommitAttempted = false
						case "cancellation":
							altered.CommitCanceled = false
						case "error":
							altered.CommitError = ""
						}
						if verifyProfileChildFailure(mode, childErr, altered, output) == nil {
							t.Fatalf("accepted missing commit proof %s", omission)
						}
					})
				}
			}
		})
	}
}
