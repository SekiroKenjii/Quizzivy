//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"
)

type queueFixture struct {
	t         *testing.T
	ctx       context.Context
	cancel    context.CancelFunc
	pool      *pgxpool.Pool
	tx        pgx.Tx
	repo      *repositories.Reviews
	teacher   string
	foreign   string
	students  []string
	owned     map[string][]string
	committed bool
}
type queuePaper struct {
	test, version, section, assignment string
	questions                          []string
}

func newQueueFixture(t *testing.T) *queueFixture {
	return queueFixtureWithURL(t, os.Getenv("TEST_DATABASE_URL"))
}

func queueFixtureWithURL(t *testing.T, dsn string) *queueFixture {
	t.Helper()
	if dsn == "" {
		t.Fatal("the explicitly leased queue database URL is required for real verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cancel()
		if os.Getenv("QUEUE_LIFECYCLE_CHILD") != "" {
			fmt.Println("QUEUE_POOLS_CLOSED")
		}
	})
	var database, role string
	var version int
	var super bool
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::integer,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user)`).Scan(&database, &role, &version, &super); err != nil {
		t.Fatal(err)
	}
	if version < 180000 || super {
		t.Fatalf("requires nonsuper PG18: role=%s version=%d super=%t", role, version, super)
	}
	t.Logf("queue purpose=%s role=%s PG=%d", database, role, version)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	f := &queueFixture{t: t, ctx: ctx, cancel: cancel, pool: pool, tx: tx, owned: map[string][]string{}}
	f.repo = repositories.NewReviews(db.NewContext(tx))
	t.Cleanup(func() { f.rollback() })
	f.teacher = f.user("teacher", "Giáo viên", "")
	f.foreign = f.user("teacher", "Giáo viên khác", "")
	f.students = []string{f.user("student", "Trùng tên", f.teacher), f.user("student", "Trùng tên", f.teacher), f.user("student", "Học viên cuối", f.teacher)}
	return f
}

func (f *queueFixture) rollback() {
	f.t.Helper()
	f.cancel()
	if f.committed {
		raw, err := json.Marshal(f.owned)
		if err != nil {
			f.t.Errorf("committed ownership encoding: %v", err)
			return
		}
		f.t.Logf("QUEUE_COMMITTED_PRESERVED %s", raw)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := f.tx.Rollback(ctx); err != nil {
		f.t.Errorf("queue fixture rollback: %v", err)
		return
	}
	clean := true
	for table, ids := range f.owned {
		var n int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.`+table+` WHERE id=ANY($1::uuid[])`, ids).Scan(&n); err != nil {
			f.t.Errorf("queue fixture absence %s: %v", table, err)
			clean = false
		} else if n != 0 {
			f.t.Errorf("queue fixture leaked %s identities=%v count=%d", table, ids, n)
			clean = false
		}
	}
	var audit int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id=ANY($1::uuid[])`, f.owned["users"]).Scan(&audit); err != nil {
		f.t.Errorf("queue audit rollback verification: %v", err)
		clean = false
	} else if audit != 0 {
		f.t.Errorf("queue fixture audit leaked: %d", audit)
		clean = false
	}
	if clean {
		raw, err := json.Marshal(f.owned)
		if err != nil {
			f.t.Errorf("queue ownership encoding: %v", err)
			return
		}
		f.t.Logf("QUEUE_ROLLBACK_VERIFIED %s", raw)
	}
}

func (f *queueFixture) id(table string) string {
	id := uuid.NewString()
	f.owned[table] = append(f.owned[table], id)
	return id
}
func (f *queueFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.tx.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("queue fixture statement: %v", err)
	}
}
func (f *queueFixture) user(role, name, creator string) string {
	f.t.Helper()
	id := f.id("users")
	f.exec(`INSERT INTO app.users(id,email,full_name,role_id,created_by) VALUES($1,$2,$3,(SELECT id FROM app.roles WHERE builtin_key=$4),nullif($5,'')::uuid)`, id, "queue-"+id+"@example.com", name, role, creator)
	return id
}
func (f *queueFixture) paper(owner, title string, kinds ...string) queuePaper {
	f.t.Helper()
	p := queuePaper{test: f.id("tests"), version: f.id("test_versions"), section: f.id("test_version_sections")}
	f.exec(`INSERT INTO app.tests(id,title,created_by,owner_id) VALUES($1,$2,$3,$3)`, p.test, title, owner)
	f.exec(`INSERT INTO app.test_versions(id,test_id,version,total_points,published_by) VALUES($1,$2,1,$3,$4)`, p.version, p.test, len(kinds), owner)
	f.exec(`INSERT INTO app.test_version_sections(id,test_version_id,ordinal,title) VALUES($1,$2,0,'Phần 1')`, p.section, p.version)
	for ordinal, kind := range kinds {
		id := f.id("test_version_questions")
		p.questions = append(p.questions, id)
		f.exec(`INSERT INTO app.test_version_questions(id,test_version_section_id,ordinal,type,prompt,points,sample_answer) VALUES($1,$2,$3,$4::app.question_type,$5,1,CASE WHEN $4='short_answer' THEN 'Frozen teacher sample' END)`, id, p.section, ordinal, kind, fmt.Sprintf("Frozen question %d", ordinal+1))
	}
	p.assignment = f.assignment(owner, p)
	return p
}
func (f *queueFixture) assignment(owner string, p queuePaper) string {
	f.t.Helper()
	id := f.id("assignments")
	f.exec(`INSERT INTO app.assignments(id,test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by) VALUES($1,$2,$3,now()-interval '1 day',now()+interval '1 day',60,$4)`, id, p.test, p.version, owner)
	return id
}
func (f *queueFixture) attempt(p queuePaper, student, status string, submitted *time.Time, number int) string {
	f.t.Helper()
	id := f.id("attempts")
	f.exec(`INSERT INTO app.attempts(id,assignment_id,test_version_id,student_id,attempt_no,status,session_id,shuffle_seed,beacon_token_hash,started_at,deadline_at,submitted_at,graded_at,void_reason)
 VALUES($1,$2,$3,$4,$5,$6::app.attempt_status,$7,1,sha256('queue'::bytea),now()-interval '1 hour',now()+interval '1 hour',$8,CASE WHEN $6='graded' THEN now() END,CASE WHEN $6='voided' THEN 'Fixture void' END)`, id, p.assignment, p.version, student, number, status, uuid.NewString(), submitted)
	return id
}
func (f *queueFixture) answer(attempt, question string, manual bool, score *float64) {
	f.t.Helper()
	f.exec(`INSERT INTO app.attempt_answers(attempt_id,question_id,payload,requires_manual,manual_score,graded_at) VALUES($1,$2,'{"type":"text","value":"Saved answer"}',$3,$4,CASE WHEN $4::numeric IS NOT NULL THEN now() END)`, attempt, question, manual, score)
}
func (f *queueFixture) read(q domain.GradingQueueQuery) domain.GradingQueue {
	f.t.Helper()
	out, err := f.repo.GradingQueue(f.ctx, q)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}
func (f *queueFixture) own() access.Scope { return access.Scope{UserID: f.teacher} }
func queueTime(hour int) *time.Time       { v := time.Date(2026, 10, 1, hour, 0, 0, 0, time.UTC); return &v }
func queueItemIDs(q domain.GradingQueue) []string {
	out := make([]string, len(q.Items))
	for i, item := range q.Items {
		out[i] = item.AttemptID + ":" + item.Question.ID
	}
	return out
}

type queueOracleRow struct {
	attempt, assignment, question, student, title, name string
	submitted                                           *time.Time
	number                                              int
}

func (f *queueFixture) assertAll(mode string) domain.GradingQueue {
	f.t.Helper()
	rows, err := f.tx.Query(f.ctx, `SELECT at.id::text,at.assignment_id::text,q.id::text,at.student_id::text,t.title,u.full_name,at.submitted_at,
 1+(SELECT count(*) FROM app.test_version_sections ps JOIN app.test_version_questions pq ON pq.test_version_section_id=ps.id
 WHERE ps.test_version_id=at.test_version_id AND (ps.ordinal<s.ordinal OR (ps.ordinal=s.ordinal AND (pq.ordinal<q.ordinal OR (pq.ordinal=q.ordinal AND pq.id<q.id)))))
 FROM app.attempt_answers aa JOIN app.attempts at ON at.id=aa.attempt_id
 JOIN app.assignments a ON a.id=at.assignment_id JOIN app.tests t ON t.id=a.test_id JOIN app.users u ON u.id=at.student_id
 JOIN app.test_version_questions q ON q.id=aa.question_id JOIN app.test_version_sections s ON s.id=q.test_version_section_id
 WHERE at.status IN ('submitted','timed_out') AND aa.requires_manual AND aa.manual_score IS NULL AND s.test_version_id=at.test_version_id`)
	if err != nil {
		f.t.Fatal(err)
	}
	var eligible []queueOracleRow
	for rows.Next() {
		var row queueOracleRow
		if err := rows.Scan(&row.attempt, &row.assignment, &row.question, &row.student, &row.title, &row.name, &row.submitted, &row.number); err != nil {
			rows.Close()
			f.t.Fatal(err)
		}
		eligible = append(eligible, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		f.t.Fatal(err)
	}
	key := func(row queueOracleRow) string {
		if mode == "question" {
			return row.assignment + ":" + row.question
		}
		return row.student
	}
	earliest := map[string]*time.Time{}
	remaining := map[string]int{}
	students := map[string]bool{}
	for _, row := range eligible {
		k := key(row)
		remaining[k]++
		students[row.student] = true
		if old := earliest[k]; row.submitted != nil && (old == nil || row.submitted.Before(*old)) {
			earliest[k] = row.submitted
		}
	}
	timeLess := func(a, b *time.Time) bool {
		if a == nil {
			return false
		}
		return b == nil || a.Before(*b)
	}
	timeSame := func(a, b *time.Time) bool { return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b)) }
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		ka, kb := key(a), key(b)
		if !timeSame(earliest[ka], earliest[kb]) {
			return timeLess(earliest[ka], earliest[kb])
		}
		if ka != kb {
			return ka < kb
		}
		if !timeSame(a.submitted, b.submitted) {
			return timeLess(a.submitted, b.submitted)
		}
		if a.attempt != b.attempt {
			return a.attempt < b.attempt
		}
		if a.number != b.number {
			return a.number < b.number
		}
		return a.question < b.question
	})
	out := f.read(domain.GradingQueueQuery{Scope: access.Scope{All: true}, Mode: mode})
	if out.AnswersRemaining != len(eligible) || out.StudentsWaiting != len(students) {
		f.t.Fatalf("independent All totals got=%d/%d want=%d/%d", out.AnswersRemaining, out.StudentsWaiting, len(eligible), len(students))
	}
	prefix := eligible
	if len(prefix) > 200 {
		prefix = prefix[:200]
	}
	expected := make([]string, len(prefix))
	groups := []domain.GradingQueueGroup{}
	seen := map[string]bool{}
	for i, row := range prefix {
		expected[i] = row.attempt + ":" + row.question
		k := key(row)
		if !seen[k] {
			seen[k] = true
			label := row.name
			kind := "student"
			if mode == "question" {
				label = strconv.Itoa(row.number)
				kind = "question"
			}
			groups = append(groups, domain.GradingQueueGroup{Key: k, Kind: kind, Label: label, Sub: row.title, Remaining: remaining[k]})
		}
	}
	if !reflect.DeepEqual(queueItemIDs(out), expected) || !reflect.DeepEqual(out.Groups, groups) {
		f.t.Fatalf("independent All prefix/groups got IDs=%v groups=%+v want IDs=%v groups=%+v", queueItemIDs(out), out.Groups, expected, groups)
	}
	return out
}

func newCommittedQueueFixture(t *testing.T) *queueFixture {
	t.Helper()
	ordinaryURL, committedURL := os.Getenv("TEST_DATABASE_URL"), os.Getenv("QUEUE_COMMITTED_DATABASE_URL")
	if ordinaryURL == "" || committedURL == "" {
		t.Fatal("both explicitly leased ordinary and committed queue database URLs are required")
	}
	if ordinaryURL == committedURL {
		t.Fatal("ordinary and committed queue database URLs must differ")
	}
	ordinary := queueDatabaseName(t, ordinaryURL)
	committed := queueDatabaseName(t, committedURL)
	if ordinary == committed {
		t.Fatal("ordinary and committed queue actual database names must differ before fixture insertion")
	}
	return queueFixtureWithURL(t, committedURL)
}

func queueDatabaseName(t *testing.T, dsn string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var name, role string
	var version int
	var super bool
	if err := pool.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::integer,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user)`).Scan(&name, &role, &version, &super); err != nil {
		t.Fatal(err)
	}
	if version < 180000 || super {
		t.Fatalf("requires nonsuper PG18 before fixture insertion: role=%s version=%d super=%t", role, version, super)
	}
	t.Logf("QUEUE_BEFORE_INSERT purpose=%s role=%s PG=%d", name, role, version)
	return name
}

func (f *queueFixture) commitFixture() {
	f.t.Helper()
	raw, err := json.Marshal(f.owned)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Logf("QUEUE_COMMIT_OWNERSHIP %s", raw)
	if err := f.tx.Commit(f.ctx); err != nil {
		f.t.Fatal(err)
	}
	f.committed = true
	f.repo = repositories.NewReviews(db.NewContext(f.pool))
}
