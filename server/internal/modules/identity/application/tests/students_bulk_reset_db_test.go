//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type accountState struct {
	Hash         string
	MustChange   bool
	Epoch        int
	LiveFamilies int
	ResetAudits  int
}

type committedForgets struct {
	pool  *pgxpool.Pool
	mu    sync.Mutex
	calls map[string][]accountState
}

func (c *committedForgets) Resolve(_ context.Context, id string) (access.Principal, error) {
	return access.Principal{UserID: id}, nil
}

func (c *committedForgets) Forget(id string) {
	state, err := readAccount(context.Background(), c.pool, id)
	if err != nil {
		state = accountState{Epoch: -1}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[id] = append(c.calls[id], state)
}

func (c *committedForgets) of(id string) []accountState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls[id])
}

func (c *committedForgets) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, calls := range c.calls {
		n += len(calls)
	}
	return n
}

func readAccount(ctx context.Context, pool *pgxpool.Pool, id string) (accountState, error) {
	var s accountState
	err := pool.QueryRow(ctx, `
		SELECT coalesce(u.password_hash, ''), u.must_change_password, u.session_epoch,
		       (SELECT count(*) FROM app.refresh_tokens r WHERE r.user_id = u.id AND r.revoked_at IS NULL),
		       (SELECT count(*) FROM app.audit_log a WHERE a.entity_id = u.id AND a.action = 'student.password_reset')
		  FROM app.users u WHERE u.id = $1::uuid`, id).Scan(&s.Hash, &s.MustChange, &s.Epoch, &s.LiveFamilies, &s.ResetAudits)
	return s, err
}

type bulkWorld struct {
	pool      *pgxpool.Pool
	app       *application.Application
	forgets   *committedForgets
	logs      *bytes.Buffer
	marker    string
	a, b      string
	classA    string
	classA2   string
	classB    string
	mine      [3]string
	extra     string
	shared    string
	theirs    string
	disabled  string
	grader    string
	requestA  domain.WriteRequest
	requestAB domain.WriteRequest
}

func (w *bulkWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *bulkWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *bulkWorld) user(t *testing.T, builtin, name string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = $3)) RETURNING id::text`,
		uuid.NewString()+"@example.test", name+" "+w.marker, builtin)
}

func (w *bulkWorld) student(t *testing.T, name string, classes ...string) string {
	t.Helper()
	id := w.id(t, `INSERT INTO app.users (email, full_name, role_id, password_hash) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'student'), $3) RETURNING id::text`,
		uuid.NewString()+"@example.test", name+" "+w.marker, "old-hash-"+name)
	for _, class := range classes {
		w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, class, id, w.a)
	}
	for i := range 2 {
		w.exec(t, `INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1::uuid, gen_random_uuid(), sha256(($2 || $1)::bytea), now() + interval '30 days')`, id, string(rune('a'+i)))
	}
	return id
}

func (w *bulkWorld) class(t *testing.T, teacher string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp "+uuid.NewString()[:8]+" "+w.marker, teacher)
}

func newBulkWorld(t *testing.T) *bulkWorld {
	t.Helper()
	pool := newPool(t)
	w := &bulkWorld{pool: pool, logs: &bytes.Buffer{}, marker: "Reset " + nonce(t)}
	t.Cleanup(func() {
		ctx := context.Background()
		owned := `SELECT id FROM app.users WHERE full_name LIKE '%' || $1`
		for _, stmt := range []string{
			`DELETE FROM app.refresh_tokens WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.audit_log WHERE entity_id IN (` + owned + `) OR actor_user_id IN (` + owned + `)`,
			`DELETE FROM app.class_members WHERE class_id IN (SELECT id FROM app.classes WHERE name LIKE '%' || $1)`,
			`DELETE FROM app.classes WHERE name LIKE '%' || $1`,
			`DELETE FROM app.users WHERE full_name LIKE '%' || $1`,
			`DELETE FROM app.role_permissions WHERE role_id IN (SELECT id FROM app.roles WHERE name LIKE '%' || $1)`,
			`DELETE FROM app.roles WHERE name LIKE '%' || $1`,
		} {
			if _, err := pool.Exec(ctx, stmt, w.marker); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})

	w.a, w.b = w.user(t, "teacher", "Giáo viên A"), w.user(t, "teacher", "Giáo viên B")
	w.classA, w.classA2, w.classB = w.class(t, w.a), w.class(t, w.a), w.class(t, w.b)
	w.mine = [3]string{w.student(t, "Một", w.classA), w.student(t, "Hai", w.classA), w.student(t, "Ba", w.classA, w.classA2)}
	w.extra = w.student(t, "Thêm", w.classA2)
	w.shared = w.student(t, "Chung", w.classA, w.classB)
	w.theirs = w.student(t, "Của B", w.classB)
	w.disabled = w.student(t, "Đã khoá", w.classA)
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.disabled)

	role := w.id(t, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Chấm bài "+w.marker)
	w.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, 'learning.take_tests'), ($1::uuid, 'teaching.grading')`, role)
	w.grader = w.id(t, `INSERT INTO app.users (email, full_name, role_id, password_hash) VALUES ($1, $2, $3::uuid, 'old-hash-grader') RETURNING id::text`,
		uuid.NewString()+"@example.test", "Người chấm "+w.marker, role)
	w.exec(t, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, w.classA, w.grader, w.a)

	dbx := db.NewContext(pool)
	w.app = application.New(nil, nil, 0, repositories.NewStudents(dbx), attemptsrepo.NewStudentStats(dbx))
	w.forgets = &committedForgets{pool: pool, calls: map[string][]accountState{}}
	w.app.SetPrincipals(w.forgets)
	w.app.SetLogger(slog.New(slog.NewTextHandler(w.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	w.requestA = domain.WriteRequest{ActorID: w.a, Grants: access.NewSet(access.PeopleStudentsResetPassword), IP: "203.0.113.9", UserAgent: "bulk-reset-test"}
	w.requestAB = domain.WriteRequest{ActorID: w.a, All: true, Grants: access.NewSet(access.PeopleStudentsResetPassword, access.PeopleUsersManage)}
	return w
}

func (w *bulkWorld) resetAs(t *testing.T, req domain.WriteRequest, ids ...string) domain.BulkReset {
	t.Helper()
	result, err := w.app.Commands.ResetStudentsPasswords.Handle(context.Background(), command.ResetStudentsPasswords{Request: req, IDs: ids})
	if err != nil {
		t.Fatalf("a bulk reset answered an error: %v", err)
	}
	return result
}

func (w *bulkWorld) state(t *testing.T, id string) accountState {
	t.Helper()
	s, err := readAccount(context.Background(), w.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (w *bulkWorld) listAs(t *testing.T, scope access.Scope, mustChange *bool, classes ...string) (map[string]domain.Student, int) {
	t.Helper()
	found, err := w.app.Queries.ListStudents.Handle(context.Background(), query.ListStudents{Query: domain.StudentQuery{Query: w.marker, ClassIDs: classes, MustChange: mustChange, Limit: 100, Scope: scope}})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]domain.Student{}
	for _, s := range found.Items {
		out[s.ID] = s
	}
	return out, found.Page.Total
}

func TestABulkResetChangesTheCallersStudentsAndLeavesEveryoneElseAsTheyWere(t *testing.T) {
	w := newBulkWorld(t)
	ctx := context.Background()
	missing := uuid.NewString()
	ids := []string{w.mine[0], w.theirs, w.mine[1], missing, w.b, w.grader, w.disabled, w.mine[2]}
	before := map[string]accountState{}
	for _, id := range ids {
		if id != missing {
			before[id] = w.state(t, id)
		}
	}

	result := w.resetAs(t, w.requestA, ids...)

	if got, want := resetIDs(result), []string{w.mine[0], w.mine[1], w.mine[2]}; !slices.Equal(got, want) {
		t.Fatalf("reset %v, want %v", got, want)
	}
	if got, want := failureOf(result), []string{w.theirs, missing, w.b, w.grader, w.disabled}; !slices.Equal(got, want) {
		t.Errorf("failed %v, want %v in the order asked", got, want)
	}
	for _, id := range failureOf(result) {
		if got := reasonFor(result, id); !errors.Is(got, domain.ErrStudentNotFound) {
			t.Errorf("%s failed with %v, want the not-found a missing student gets: another teacher's student, a teacher, a grader and a disabled account are all missing ones", id, got)
		}
		if id == missing {
			continue
		}
		if after := w.state(t, id); after != before[id] {
			t.Errorf("a refused reset changed %s: %+v became %+v", id, before[id], after)
		}
		if calls := w.forgets.of(id); len(calls) != 0 {
			t.Errorf("a refused reset forgot %s %d times", id, len(calls))
		}
	}
	for _, r := range result.Reset {
		after := w.state(t, r.StudentID)
		if after.Hash == before[r.StudentID].Hash || !strings.HasPrefix(after.Hash, "$argon2id$") {
			t.Errorf("%s holds the hash %q", r.StudentID, after.Hash)
		}
		if ok, err := domain.Passwords.Verify(ctx, r.TemporaryPassword, after.Hash); err != nil || !ok {
			t.Errorf("the temporary password for %s does not match the stored hash (%v)", r.StudentID, err)
		}
		if !after.MustChange || after.Epoch != before[r.StudentID].Epoch+1 || after.LiveFamilies != 0 || after.ResetAudits != 1 {
			t.Errorf("%s after the reset: %+v, want a forced change, the epoch moved by one, no live family and one audit row", r.StudentID, after)
		}
		calls := w.forgets.of(r.StudentID)
		if len(calls) != 1 || calls[0].Epoch != after.Epoch || calls[0].LiveFamilies != 0 || calls[0].ResetAudits != 1 {
			t.Errorf("%s was forgotten as %+v, want once, after the commit", r.StudentID, calls)
		}
		var actor, ip, agent string
		if err := w.pool.QueryRow(ctx, `SELECT actor_user_id::text, host(ip), user_agent FROM app.audit_log WHERE entity_id = $1::uuid AND action = 'student.password_reset'`, r.StudentID).Scan(&actor, &ip, &agent); err != nil {
			t.Fatal(err)
		}
		if actor != w.a || ip != "203.0.113.9" || agent != "bulk-reset-test" {
			t.Errorf("the audit row for %s names %s from %s as %q", r.StudentID, actor, ip, agent)
		}
		var row string
		if err := w.pool.QueryRow(ctx, `SELECT string_agg(to_jsonb(a)::text, ' ') FROM app.audit_log a WHERE a.entity_id = $1::uuid`, r.StudentID).Scan(&row); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(row, r.TemporaryPassword) || strings.Contains(row, after.Hash) {
			t.Errorf("the audit rows for %s hold the password or its hash: %s", r.StudentID, row)
		}
		if strings.Contains(w.logs.String(), r.TemporaryPassword) {
			t.Errorf("the log holds %s's temporary password", r.StudentID)
		}
	}
	if total := w.forgets.total(); total != 3 {
		t.Errorf("%d accounts were forgotten, want the 3 that were reset", total)
	}
}

func TestASharedStudentIsRefusedToATeacherAndResetForAManager(t *testing.T) {
	w := newBulkWorld(t)
	before := w.state(t, w.shared)

	result := w.resetAs(t, w.requestA, w.shared, w.mine[0])

	if got := resetIDs(result); !slices.Equal(got, []string{w.mine[0]}) {
		t.Errorf("reset %v, want only A's own student", got)
	}
	if got := reasonFor(result, w.shared); !errors.Is(got, domain.ErrStudentShared) {
		t.Errorf("the shared student failed with %v, want the shared-student refusal", got)
	}
	if _, err := w.app.Commands.ResetStudentPassword.Handle(context.Background(), command.ResetStudentPassword{Request: w.requestA, ID: w.shared}); !errors.Is(err, domain.ErrStudentShared) {
		t.Errorf("the single reset of the shared student answered %v", err)
	}
	if after := w.state(t, w.shared); after != before {
		t.Errorf("the refusals changed the shared student: %+v became %+v", before, after)
	}

	managed := w.resetAs(t, w.requestAB, w.shared, w.theirs)
	if got := resetIDs(managed); !slices.Equal(got, []string{w.shared, w.theirs}) {
		t.Fatalf("a manager reset %v (failed %v), want both", got, failureOf(managed))
	}
	if after := w.state(t, w.shared); after.Epoch != before.Epoch+1 || !after.MustChange || after.LiveFamilies != 0 {
		t.Errorf("the shared student after a manager's reset: %+v", after)
	}
}

func TestAFullBatchOfFortyIsCommittedStudentByStudentWithTwoHashesAtOnce(t *testing.T) {
	w := newBulkWorld(t)
	ids := make([]string, domain.MaxBulkReset)
	for i := range ids {
		ids[i] = w.student(t, fmt.Sprintf("Lô %02d", i), w.classA)
	}
	var calls, inFlight, peak atomic.Int64
	w.app.SetTemporaryPasswords(func(context.Context) (string, string, error) {
		n := calls.Add(1)
		now := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			seen := peak.Load()
			if now <= seen || peak.CompareAndSwap(seen, now) {
				break
			}
		}
		return fmt.Sprintf("pw-%d", n), fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA%d", n), nil
	})

	result := w.resetAs(t, w.requestA, ids...)

	if !slices.Equal(resetIDs(result), ids) || len(result.Failed) != 0 {
		t.Fatalf("reset %d of %d, failed %v", len(result.Reset), len(ids), failureOf(result))
	}
	for _, id := range ids {
		if s := w.state(t, id); s.Epoch != 1 || s.LiveFamilies != 0 || s.ResetAudits != 1 || !s.MustChange {
			t.Errorf("%s after the batch: %+v", id, s)
		}
		if calls := w.forgets.of(id); len(calls) != 1 || calls[0].Epoch != 1 || calls[0].ResetAudits != 1 {
			t.Errorf("%s was forgotten as %+v", id, calls)
		}
	}
	if peak.Load() > 2 {
		t.Errorf("%d passwords were made at once, want at most 2", peak.Load())
	}
}

func TestListingFiltersByAnyOfSeveralClassesAndByTheTemporaryPasswordFlag(t *testing.T) {
	w := newBulkWorld(t)
	a := access.Scope{UserID: w.a}
	all := access.Scope{UserID: w.a, All: true}
	inA := []string{w.mine[0], w.mine[1], w.mine[2], w.shared}
	inA2 := []string{w.mine[2], w.extra}

	for name, c := range map[string]struct {
		scope   access.Scope
		classes []string
		want    []string
	}{
		"one class":                          {a, []string{w.classA}, inA},
		"another of the teacher's classes":   {a, []string{w.classA2}, inA2},
		"either of two classes":              {a, []string{w.classA, w.classA2}, append(slices.Clone(inA), w.extra)},
		"a class and another teacher's":      {a, []string{w.classA2, w.classB}, inA2},
		"another teacher's class alone":      {a, []string{w.classB}, nil},
		"a missing class and a real one":     {a, []string{uuid.NewString(), w.classA2}, inA2},
		"the same class twice":               {a, []string{w.classA2, w.classA2}, inA2},
		"scope.all, a class of each teacher": {all, []string{w.classA2, w.classB}, []string{w.mine[2], w.extra, w.shared, w.theirs}},
	} {
		t.Run(name, func(t *testing.T) {
			listed, total := w.listAs(t, c.scope, nil, c.classes...)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := keys(listed); !slices.Equal(got, want) {
				t.Errorf("lists %v, want %v", got, want)
			}
			if total != len(want) {
				t.Errorf("the list totals %d for %d students: a student in two of the classes must count once", total, len(want))
			}
			facets, err := w.app.Queries.StudentFacets.Handle(context.Background(), query.StudentFacets{Query: domain.StudentQuery{Query: w.marker, ClassIDs: c.classes, Scope: c.scope}})
			if err != nil {
				t.Fatal(err)
			}
			if facets.Total != len(want) {
				t.Errorf("the facets total %d, want %d", facets.Total, len(want))
			}
		})
	}

	reset := w.resetAs(t, w.requestA, w.mine[0], w.mine[1])
	if len(reset.Reset) != 2 {
		t.Fatalf("reset %v", resetIDs(reset))
	}
	yes, no := true, false
	for name, c := range map[string]struct {
		flag    *bool
		classes []string
		want    []string
	}{
		"waiting to change, in any class": {&yes, nil, []string{w.mine[0], w.mine[1]}},
		"waiting to change, in a class":   {&yes, []string{w.classA, w.classA2}, []string{w.mine[0], w.mine[1]}},
		"waiting to change, other class":  {&yes, []string{w.classA2}, nil},
		"not waiting, in two classes":     {&no, []string{w.classA, w.classA2}, []string{w.mine[2], w.shared, w.extra}},
		"not asked":                       {nil, []string{w.classA2}, inA2},
	} {
		t.Run(name, func(t *testing.T) {
			listed, total := w.listAs(t, a, c.flag, c.classes...)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := keys(listed); !slices.Equal(got, want) {
				t.Errorf("lists %v, want %v", got, want)
			}
			if total != len(want) {
				t.Errorf("the list totals %d for %d students", total, len(want))
			}
			facets, err := w.app.Queries.StudentFacets.Handle(context.Background(), query.StudentFacets{Query: domain.StudentQuery{Query: w.marker, ClassIDs: c.classes, MustChange: c.flag, Scope: a}})
			if err != nil {
				t.Fatal(err)
			}
			if facets.Total != len(want) {
				t.Errorf("the facets total %d, want %d: they must count the same students the list shows", facets.Total, len(want))
			}
		})
	}
}
