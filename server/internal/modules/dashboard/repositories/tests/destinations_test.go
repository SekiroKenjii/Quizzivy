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

func destinationWorld(t *testing.T, before ...func(pgx.Tx)) *homeWorld {
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
	var priorUsers map[string]bool
	t.Cleanup(func() {
		var owned []string
		if priorUsers != nil {
			users, err := destinationUserIDs(tx)
			if err != nil {
				t.Errorf("capture owned users before rollback: %v", err)
			} else {
				for _, id := range users {
					if !priorUsers[id] {
						owned = append(owned, id)
					}
				}
				t.Logf("owned transaction users=%v", owned)
			}
		}
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
		if priorUsers != nil {
			var remaining int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE id=ANY($1::uuid[])`, owned).Scan(&remaining); err != nil {
				t.Errorf("check owned users after rollback: %v", err)
			} else if remaining != 0 {
				t.Errorf("owned users remain after rollback: %d", remaining)
			} else {
				t.Logf("checked rollback owned users=%d remaining=0", len(owned))
			}
		}
	})
	if len(before) != 0 {
		users, err := destinationUserIDs(tx)
		if err != nil {
			t.Fatal(err)
		}
		priorUsers = map[string]bool{}
		for _, id := range users {
			priorUsers[id] = true
		}
	}
	for _, prepare := range before {
		prepare(tx)
	}
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
		t.Fatalf("taking students=%d assignments=%d id=%s want students=%d assignments=%d id=%s", out.Students, out.Assignments, destinationID(out.AssignmentID), students, assignments, destinationID(id))
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
	for _, test := range []struct {
		name  string
		count int
	}{{"without_background", 0}, {"foreign_background", 1}, {"foreign_second_page", 101}} {
		t.Run(test.name, func(t *testing.T) {
			w, baseline := destinationOracleWorld(t, test.count)
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
			requireCompleteFlaggedList(t, w, baseline)
		})
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
	for _, test := range []struct {
		name  string
		count int
	}{{"without_background", 0}, {"foreign_background", 1}} {
		t.Run(test.name, func(t *testing.T) {
			w, baseline := destinationOracleWorld(t, test.count)
			now := fixedNow()
			for _, id := range []string{w.p1, w.p4, w.p5} {
				execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, id, now.Add(-time.Hour), now.Add(time.Hour))
			}
			execHome(t, w, `UPDATE app.attempts SET started_at=$2 WHERE id=$1`, w.p1, now.Add(-time.Minute))
			requireTaking(t, homeAt(t, w, access.Scope{UserID: w.a}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 1, 1, &w.aM)
			requireTaking(t, homeAt(t, w, access.Scope{UserID: w.admin}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 2, 1, &w.aM)
			taking := append([]takingTuple(nil), baseline.taking...)
			taking = append(taking,
				takingTuple{id: w.p1, student: w.s1, assignment: w.aB, started: now.Add(-time.Minute)},
				takingTuple{id: w.p4, student: w.s2, assignment: w.aM, started: now.Add(-time.Hour)},
				takingTuple{id: w.p5, student: w.s1, assignment: w.aM, started: now.Add(-time.Hour)},
			)
			want := expectedTaking(taking)
			requireTaking(t, homeAt(t, w, access.Scope{UserID: w.admin, All: true}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, want.Students, want.Assignments, want.AssignmentID)
			requireTaking(t, homeAt(t, w, access.Scope{}, now, "Asia/Ho_Chi_Minh", 14).TakingNow, 0, 0, nil)
		})
	}
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

func destinationID(id *string) string {
	if id == nil {
		return "<nil>"
	}
	return *id
}

func destinationUserIDs(tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(context.Background(), `SELECT id::text FROM app.users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

type takingTuple struct {
	id, student, assignment string
	started                 time.Time
}

type destinationBaseline struct {
	taking  []takingTuple
	flagged map[string]bool
	foreign []takingTuple
}

func destinationBackground(t *testing.T, tx pgx.Tx, count int) []takingTuple {
	t.Helper()
	if count == 0 {
		return nil
	}
	w := &homeWorld{tx: tx}
	teacher, student := w.user(t, "teacher", nil), w.user(t, "student", nil)
	testID := w.id(t, `INSERT INTO app.tests(title,status,current_version,created_by,owner_id) VALUES ('Foreign destination control','published',1,$1,$1) RETURNING id::text`, teacher)
	w.version = w.id(t, `INSERT INTO app.test_versions(test_id,version,total_points,published_by) VALUES ($1,1,1,$2) RETURNING id::text`, testID, teacher)
	out := make([]takingTuple, 0, count)
	for i := 0; i < count; i++ {
		assignment := w.assignment(t, teacher, "-2 hours", "2 hours", nil, []string{student})
		started := fixedNow().Add(time.Minute + time.Duration(i)*time.Millisecond)
		id := w.id(t, `INSERT INTO app.attempts(assignment_id,test_version_id,student_id,attempt_no,status,session_id,shuffle_seed,beacon_token_hash,started_at,deadline_at,flagged) VALUES ($1,$2,$3,1,'in_progress',gen_random_uuid(),1,sha256('b'::bytea),$4,$5,true) RETURNING id::text`, assignment, w.version, student, started, fixedNow().Add(time.Hour))
		out = append(out, takingTuple{id: id, student: student, assignment: assignment, started: started})
		t.Logf("owned foreign control teacher=%s student=%s test=%s version=%s assignment=%s attempt=%s started=%s deadline=%s", teacher, student, testID, w.version, assignment, id, started.Format(time.RFC3339Nano), fixedNow().Add(time.Hour).Format(time.RFC3339Nano))
	}
	return out
}

func captureDestinationBaseline(t *testing.T, tx pgx.Tx) destinationBaseline {
	t.Helper()
	out := destinationBaseline{flagged: map[string]bool{}}
	rows, err := tx.Query(context.Background(), `SELECT id::text,student_id::text,assignment_id::text,started_at FROM app.attempts WHERE status='in_progress' AND deadline_at>$1`, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var row takingTuple
		if err := rows.Scan(&row.id, &row.student, &row.assignment, &row.started); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		out.taking = append(out.taking, row)
		t.Logf("initial taking tuple attempt=%s student=%s assignment=%s started=%s", row.id, row.student, row.assignment, row.started.Format(time.RFC3339Nano))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows, err = tx.Query(context.Background(), `SELECT id::text FROM app.attempts WHERE flagged`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out.flagged[id] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func destinationOracleWorld(t *testing.T, count int) (*homeWorld, destinationBaseline) {
	t.Helper()
	var baseline destinationBaseline
	w := destinationWorld(t, func(tx pgx.Tx) {
		foreign := destinationBackground(t, tx, count)
		baseline = captureDestinationBaseline(t, tx)
		baseline.foreign = foreign
		for _, expected := range foreign {
			found := false
			for _, row := range baseline.taking {
				if row.id == expected.id && row.student == expected.student && row.assignment == expected.assignment && row.started.Equal(expected.started) {
					found = true
				}
			}
			if !found || !baseline.flagged[expected.id] {
				t.Fatalf("foreign control missing before buildHome: %+v", expected)
			}
		}
	})
	var testID, sectionID string
	if err := w.tx.QueryRow(context.Background(), `SELECT v.test_id::text,s.id::text FROM app.test_versions v JOIN app.test_version_sections s ON s.test_version_id=v.id WHERE v.id=$1`, w.version).Scan(&testID, &sectionID); err != nil {
		t.Fatal(err)
	}
	t.Logf("owned target IDs users=%v classes=%v test=%s version=%s section=%s question=%s assignments=%v attempts=%v",
		[]string{w.a, w.b, w.admin, w.s1, w.s2, w.s3, w.s4, w.s5, w.s6, w.s7, w.s8, w.s9, w.t1},
		[]string{w.classA, w.classB, w.classB2}, testID, w.version, sectionID, w.question,
		[]string{w.aA, w.aB, w.aB2, w.aM, w.aOldB}, []string{w.p1, w.p2, w.p3, w.p4, w.p5, w.p6, w.p7, w.p8, w.p10})
	for _, row := range baseline.taking {
		if row.student == w.s1 || row.student == w.s2 || row.assignment == w.aB || row.assignment == w.aM || row.id == w.p1 || row.id == w.p4 || row.id == w.p5 {
			t.Fatalf("target identity overlaps baseline: %+v", row)
		}
	}
	for _, id := range []string{w.p1, w.p2, w.p3, w.p7} {
		if baseline.flagged[id] {
			t.Fatalf("target flagged identity overlaps baseline: %s", id)
		}
	}
	return w, baseline
}

func expectedTaking(rows []takingTuple) domain.TakingNow {
	students, assignments := map[string]bool{}, map[string]bool{}
	var newest *takingTuple
	for i := range rows {
		row := &rows[i]
		students[row.student], assignments[row.assignment] = true, true
		if newest == nil || row.started.After(newest.started) || row.started.Equal(newest.started) && row.id > newest.id {
			newest = row
		}
	}
	out := domain.TakingNow{Students: len(students), Assignments: len(assignments)}
	if newest != nil {
		assignment := newest.assignment
		out.AssignmentID = &assignment
	}
	return out
}

func requireCompleteFlaggedList(t *testing.T, w *homeWorld, baseline destinationBaseline) {
	t.Helper()
	expected := map[string]bool{}
	for id := range baseline.flagged {
		expected[id] = true
	}
	for _, id := range []string{w.p1, w.p2, w.p3, w.p7} {
		expected[id] = true
	}
	yes := true
	repo := repositories.NewPostgres(db.NewContext(w.tx))
	seen := map[string]bool{}
	pages := (len(expected) + 99) / 100
	for number := 1; number <= pages; number++ {
		rows, page, err := repo.List(context.Background(), domain.ListQuery{Scope: access.Scope{UserID: w.admin, All: true}, Flagged: &yes, Limit: 100, Page: number})
		if err != nil {
			t.Fatal(err)
		}
		length := len(expected) - (number-1)*100
		if length > 100 {
			length = 100
		}
		if page.Total != len(expected) || len(rows) != length {
			t.Fatalf("wide page=%+v rows=%v expected total=%d length=%d", page, ids(rows), len(expected), length)
		}
		for _, row := range rows {
			if !expected[row.ID] || seen[row.ID] {
				t.Fatalf("extra/duplicate wide attempt=%s", row.ID)
			}
			seen[row.ID] = true
		}
		t.Logf("complete public flagged page=%d length=%d total=%d", number, len(rows), page.Total)
	}
	if !reflect.DeepEqual(seen, expected) {
		t.Fatalf("missing wide identities actual=%v expected=%v", seen, expected)
	}
}
