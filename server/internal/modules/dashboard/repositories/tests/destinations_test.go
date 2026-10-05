//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"strings"
	"testing"
	"time"
)

func destinationWorld(t *testing.T) *homeWorld {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	pool := newPool(t)
	var database, role string
	var version int
	if err = pool.QueryRow(context.Background(), `SELECT current_database(),current_user,current_setting('server_version_num')::integer`).Scan(&database, &role, &version); err != nil {
		t.Fatal(err)
	}
	if database != cfg.ConnConfig.Database || role != cfg.ConnConfig.User || version < 180000 {
		t.Fatalf("database=%s role=%s version=%d", database, role, version)
	}
	t.Logf("purpose connection database=%s role=%s version=%d", database, role, version)
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
	})
	return buildHome(t, tx)
}
func destinationSummary(t *testing.T, w *homeWorld, scope access.Scope, allowed bool) domain.Summary {
	t.Helper()
	out, err := repositories.NewPostgres(db.NewContext(w.tx)).Summary(context.Background(), domain.SummaryQuery{Scope: scope, CanReviewFlagged: allowed})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func requireFlagged(t *testing.T, out domain.Summary, count int, assignment, attempt string) {
	t.Helper()
	want := &domain.FlaggedAttempt{AssignmentID: assignment, AttemptID: attempt}
	if out.FlaggedAttempts != count || !reflect.DeepEqual(out.NewestFlaggedAttempt, want) {
		t.Fatalf("flagged count=%d pair=%+v want count=%d pair=%+v", out.FlaggedAttempts, out.NewestFlaggedAttempt, count, want)
	}
}
func requireTaking(t *testing.T, out domain.TakingNow, students, assignments int, id *string) {
	t.Helper()
	if !reflect.DeepEqual(out, domain.TakingNow{Students: students, Assignments: assignments, AssignmentID: id}) {
		t.Fatalf("taking=%+v want students=%d assignments=%d id=%v", out, students, assignments, id)
	}
}

func TestFlaggedDestinationUsesStartedAtAndIDRatherThanSubmissionOrNewerUnflagged(t *testing.T) {
	w := destinationWorld(t)
	scope := access.Scope{UserID: w.a}
	now := fixedNow()
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour',submitted_at=$3 WHERE id=$1`, w.p2, now.Add(-3*time.Hour), now.Add(24*time.Hour))
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour',submitted_at=$3 WHERE id=$1`, w.p3, now.Add(-2*time.Hour), now.Add(-24*time.Hour))
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour',submitted_at=$3 WHERE id=$1`, w.p4, now.Add(-4*time.Hour), now)
	requireFlagged(t, destinationSummary(t, w, scope, true), 3, w.aA, w.p3)
	execHome(t, w, `UPDATE app.attempts SET flagged=false,started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=$1`, w.p4, now)
	requireFlagged(t, destinationSummary(t, w, scope, true), 2, w.aA, w.p3)
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=ANY($1::uuid[])`, []string{w.p2, w.p3}, now.Add(-time.Hour))
	winner := w.p2
	if w.p3 > winner {
		winner = w.p3
	}
	requireFlagged(t, destinationSummary(t, w, scope, true), 2, w.aA, winner)
}

func TestFlaggedDestinationKeepsAuthoredSharedFormerHistoricalAndVoidedReach(t *testing.T) {
	w := destinationWorld(t)
	scope := access.Scope{UserID: w.b}
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=$1`, w.p1, fixedNow().Add(-time.Hour))
	execHome(t, w, `UPDATE app.attempts SET status='voided',void_reason='verification',started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=$1`, w.p7, fixedNow())
	requireFlagged(t, destinationSummary(t, w, scope, true), 2, w.aOldB, w.p7)
	execHome(t, w, `UPDATE app.attempts SET flagged=true,started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=$1`, w.p5, fixedNow().Add(time.Hour))
	requireFlagged(t, destinationSummary(t, w, scope, true), 3, w.aM, w.p5)
	execHome(t, w, `UPDATE app.attempts SET status='graded',graded_at=$2,started_at=$2,deadline_at=$2::timestamptz+interval '1 hour',flagged=true WHERE id=$1`, w.p6, fixedNow().Add(2*time.Hour))
	requireFlagged(t, destinationSummary(t, w, scope, true), 4, w.aB2, w.p6)
}

func TestDeniedAndAdminOwnDestinationsPreserveCountsAndWideAttemptList(t *testing.T) {
	w := destinationWorld(t)
	scope := access.Scope{UserID: w.admin}
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$2::timestamptz+interval '1 hour' WHERE id=$1`, w.p1, fixedNow().Add(24*time.Hour))
	requireFlagged(t, destinationSummary(t, w, scope, true), 1, w.aM, w.p4)
	denied := destinationSummary(t, w, scope, false)
	if denied.FlaggedAttempts != 1 || denied.NewestFlaggedAttempt != nil {
		t.Fatalf("denied=%+v", denied)
	}
	empty := destinationSummary(t, w, access.Scope{}, true)
	if empty.FlaggedAttempts != 0 || empty.NewestFlaggedAttempt != nil {
		t.Fatalf("empty=%+v", empty)
	}
	execHome(t, w, `UPDATE app.attempts SET flagged=false WHERE assignment_id=$1`, w.aM)
	if out := destinationSummary(t, w, scope, true); out.FlaggedAttempts != 0 || out.NewestFlaggedAttempt != nil {
		t.Fatalf("no flags=%+v", out)
	}
	yes := true
	repo := repositories.NewPostgres(db.NewContext(w.tx))
	rows, page, err := repo.List(context.Background(), domain.ListQuery{Scope: access.Scope{UserID: w.admin, All: true}, Flagged: &yes, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || len(rows) != 4 || !has(rows, w.p1) {
		t.Fatalf("wide page=%+v rows=%v", page, ids(rows))
	}
}

func TestTakingDestinationUsesTheInjectedDeadlineAndNewestTieWithoutRangeFiltering(t *testing.T) {
	w := destinationWorld(t)
	scope := access.Scope{UserID: w.b}
	now := fixedNow()
	for _, id := range []string{w.p1, w.p6, w.p7} {
		execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, id, now.Add(-time.Hour), now.Add(time.Hour))
	}
	execHome(t, w, `UPDATE app.attempts SET student_id=$2 WHERE id=$1`, w.p6, w.s1)
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2,started_at=$3 WHERE id=$1`, w.p7, now, now.Add(-time.Minute))
	winner := w.aB
	if w.p6 > w.p1 {
		winner = w.aB2
	}
	for _, days := range []int{7, 14, 30} {
		requireTaking(t, homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", days).TakingNow, 1, 2, &winner)
	}
	execHome(t, w, `UPDATE app.attempts SET started_at=$2,deadline_at=$3 WHERE id=$1`, w.p1, now.Add(-time.Minute), now.Add(time.Hour))
	requireTaking(t, homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 1, 2, &w.aB)
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2 WHERE id=$1`, w.p1, now.Add(-time.Second))
	requireTaking(t, homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 1, 1, &w.aB2)
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2 WHERE id=$1`, w.p6, now)
	requireTaking(t, homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 0, 0, nil)
}

func TestTakingDestinationKeepsSharedClassOwnAndForeignBoundaries(t *testing.T) {
	w := destinationWorld(t)
	now := fixedNow()
	for _, id := range []string{w.p1, w.p4, w.p5} {
		execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, id, now.Add(-time.Hour), now.Add(time.Hour))
	}
	execHome(t, w, `UPDATE app.attempts SET started_at=$2 WHERE id=$1`, w.p1, now.Add(-time.Minute))
	requireTaking(t, homeAt(t, w, access.Scope{UserID: w.a}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 1, 1, &w.aM)
	requireTaking(t, homeAt(t, w, access.Scope{UserID: w.admin}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 2, 1, &w.aM)
	requireTaking(t, homeAt(t, w, access.Scope{UserID: w.admin, All: true}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 2, 2, &w.aB)
	requireTaking(t, homeAt(t, w, access.Scope{}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 0, 0, nil)
}

type capturedStatement struct {
	sql  string
	args []any
}
type destinationConnection struct {
	pgx.Tx
	statements []capturedStatement
}

func (c *destinationConnection) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.statements = append(c.statements, capturedStatement{sql: sql, args: append([]any(nil), args...)})
	return c.Tx.QueryRow(ctx, sql, args...)
}
func statementWith(t *testing.T, c *destinationConnection, fragment string) capturedStatement {
	t.Helper()
	var found []capturedStatement
	for _, s := range c.statements {
		if strings.Contains(s.sql, fragment) {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s statement count=%d", fragment, len(found))
	}
	return found[0]
}
func explainDestination(t *testing.T, w *homeWorld, s capturedStatement) map[string]any {
	t.Helper()
	var raw []byte
	if err := w.tx.QueryRow(context.Background(), `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) `+s.sql, s.args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plans []map[string]any
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	t.Logf("ordinary plan %s", raw)
	return plans[0]["Plan"].(map[string]any)
}
func findPlan(node map[string]any, match func(map[string]any) bool) bool {
	if match(node) {
		return true
	}
	if children, ok := node["Plans"].([]any); ok {
		for _, child := range children {
			if findPlan(child.(map[string]any), match) {
				return true
			}
		}
	}
	return false
}

func TestDestinationStatementsShareCountsAndIDsAndDeniedSelectorNeverExecutes(t *testing.T) {
	w := destinationWorld(t)
	execHome(t, w, `INSERT INTO app.attempts (assignment_id,test_version_id,student_id,attempt_no,status,session_id,shuffle_seed,beacon_token_hash,started_at,deadline_at,submitted_at,graded_at,flagged)
 SELECT CASE WHEN n%3=0 THEN $1::uuid ELSE $2::uuid END,$3,$4,n+10,'graded',gen_random_uuid(),1,sha256('b'::bytea),$5::timestamptz-n*interval '1 minute',$5::timestamptz+interval '1 hour',$5,$5,n%10=0 FROM generate_series(1,3000) n`, w.aA, w.aB, w.version, w.s2, fixedNow())
	for _, allowed := range []bool{false, true} {
		c := &destinationConnection{Tx: w.tx}
		repo := repositories.NewPostgres(db.NewContext(c))
		out, err := repo.Summary(context.Background(), domain.SummaryQuery{Scope: access.Scope{UserID: w.a}, CanReviewFlagged: allowed})
		if err != nil {
			t.Fatal(err)
		}
		if out.FlaggedAttempts != 103 || (out.NewestFlaggedAttempt != nil) != allowed {
			t.Fatalf("allowed=%v count=%d pair=%+v", allowed, out.FlaggedAttempts, out.NewestFlaggedAttempt)
		}
		s := statementWith(t, c, "destination.assignment_id::text")
		if !strings.Contains(s.sql, "count(*) FROM app.attempts at WHERE at.flagged") {
			t.Fatal("count and selector did not share a statement")
		}
		plan := explainDestination(t, w, s)
		if !allowed && !findPlan(plan, func(n map[string]any) bool { return n["One-Time Filter"] == "false" && n["Actual Rows"] == float64(0) }) {
			t.Fatal("denied selector was not skipped by its false gate")
		}
	}
	for _, id := range []string{w.p1, w.p4, w.p5, w.p6} {
		execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, id, fixedNow().Add(-time.Hour), fixedNow().Add(time.Hour))
	}
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2 WHERE id=$1`, w.p6, fixedNow())
	c := &destinationConnection{Tx: w.tx}
	repo := repositories.NewPostgres(db.NewContext(c))
	out, err := repo.Home(context.Background(), domain.HomeQuery{Scope: access.Scope{UserID: w.b}, Now: fixedNow(), Zone: "Asia/Ho_Chi_Minh", Days: 14})
	if err != nil {
		t.Fatal(err)
	}
	requireTaking(t, out.TakingNow, 1, 2, &w.aM)
	s := statementWith(t, c, "WITH taking AS")
	if !strings.Contains(s.sql, "count(DISTINCT student_id)") || !strings.Contains(s.sql, "ORDER BY started_at DESC,id DESC LIMIT 1") {
		t.Fatal("taking counts and destination did not share a statement")
	}
	explainDestination(t, w, s)
	t.Log("representative attempts=3009 reachable graded bulk=1000 foreign bulk=2000 flagged bulk=300 current=3 exact-deadline=1; plans are small-data observations")
}
