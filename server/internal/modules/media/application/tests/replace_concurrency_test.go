//go:build integration

package application_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	testsdomain "quizzivy/internal/modules/tests/domain"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type replacementQuery struct {
	sql string
	pid uint32
}
type replacementTrace struct {
	events              chan replacementQuery
	cancelCommit        atomic.Bool
	failRollback        atomic.Bool
	rollbackPID         atomic.Uint32
	rollbackCloseFailed atomic.Bool
	cancelledPID        atomic.Uint32
}

func (tr *replacementTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	select {
	case tr.events <- replacementQuery{sql: data.SQL, pid: conn.PgConn().PID()}:
	default:
	}
	if strings.EqualFold(strings.TrimSpace(data.SQL), "rollback") && tr.failRollback.CompareAndSwap(true, false) {
		tr.rollbackPID.Store(conn.PgConn().PID())
		if err := conn.Close(ctx); err != nil {
			tr.rollbackCloseFailed.Store(true)
		}
	}
	if strings.EqualFold(strings.TrimSpace(data.SQL), "commit") && tr.cancelCommit.CompareAndSwap(true, false) {
		tr.cancelledPID.Store(conn.PgConn().PID())
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		return cancelled
	}
	return ctx
}
func (*replacementTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func newReplacementTrace() *replacementTrace {
	return &replacementTrace{events: make(chan replacementQuery, 256)}
}
func (w *replacementWorld) blocked(t *testing.T, tr *replacementTrace, sqlContains string, blocker uint32, excluded ...uint32) uint32 {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	var pid uint32
	for pid == 0 {
		select {
		case event := <-tr.events:
			if strings.Contains(event.sql, sqlContains) && event.pid != blocker && !slices.Contains(excluded, event.pid) {
				pid = event.pid
			}
		case <-deadline.C:
			t.Fatal("waiter never reached expected SQL")
		}
	}
	for {
		var waiting bool
		err := w.pool.QueryRow(context.Background(), `WITH RECURSIVE blockers(pid) AS (SELECT unnest(pg_blocking_pids($1)) UNION SELECT unnest(pg_blocking_pids(pid)) FROM blockers) SELECT EXISTS(SELECT 1 FROM blockers WHERE pid=$2)`, pid, blocker).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			return pid
		}
		select {
		case <-deadline.C:
			t.Fatalf("pid %d did not block on %d", pid, blocker)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func (w *replacementWorld) blocker(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Error(err)
		}
	})
	return tx
}
func replacementExec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

type replacementAnswer struct {
	result domain.ReplaceResult
	err    error
}

func (w *replacementWorld) startReplacement(in domain.ReplaceInput) <-chan replacementAnswer {
	answer := make(chan replacementAnswer, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := w.repo.Replace(ctx, in)
		answer <- replacementAnswer{result: out, err: err}
		close(answer)
	}()
	w.pending = append(w.pending, answer)
	return answer
}
func replacementWait(t *testing.T, answer <-chan replacementAnswer) replacementAnswer {
	t.Helper()
	select {
	case out := <-answer:
		return out
	case <-time.After(12 * time.Second):
		t.Fatal("replacement exceeded bound")
		return replacementAnswer{}
	}
}

func TestReplacementPoolWaitsForGroupBeforeAssetAndReloadsCurrentGraph(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	group := w.group(t, w.a, nil)
	w.stimulus(t, group, old)
	groups := testsrepo.NewGroupsPostgres(db.NewContext(w.pool), adapters.GroupQuestions{}, w.repo)
	stored, err := groups.Get(context.Background(), access.Scope{UserID: w.a}, group)
	if err != nil {
		t.Fatal(err)
	}
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	replacementExec(t, tx, `SELECT id FROM app.question_groups WHERE id=$1 FOR UPDATE`, group)
	in := w.input(old, w.a, domain.KindImage, 30, 30)
	answer := w.startReplacement(in)
	w.blocked(t, tr, "ORDER BY g.id FOR UPDATE OF g", pid)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := repositories.LockForVersionUse(ctx, tx, old); err != nil {
		t.Fatalf("asset locked before group: %v", err)
	}
	replacementExec(t, tx, `UPDATE app.media_assets SET display_name='Tên sau chờ' WHERE id=$1`, old)
	stored.Bundle.Group.Stimuli[0].Content = []byte(strings.Replace(string(stored.Bundle.Group.Stimuli[0].Content), "Nội dung", "Sau khi lưu", 1))
	saver := testsrepo.NewGroupsPostgres(db.NewContext(tx), adapters.GroupQuestions{}, w.repo)
	if _, err := saver.Update(ctx, testsdomain.UpdateGroupInput{Bundle: stored.Bundle, GroupMutation: testsdomain.GroupMutation{ID: group, ExpectedRevision: stored.Revision, ActorID: w.a, Now: time.Now(), Scope: access.Scope{UserID: w.a}, Grants: access.NewSet(access.ContentQuestionsWrite)}}); err != nil {
		t.Fatalf("public group save=%v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := replacementWait(t, answer)
	if out.err != nil {
		t.Fatal(out.err)
	}
	if out.result.Asset.DisplayName != "Tên sau chờ" {
		t.Fatalf("stale name=%q", out.result.Asset.DisplayName)
	}
	raw := w.text(t, `SELECT content::text FROM app.group_stimuli WHERE group_id=$1`, group)
	if !strings.Contains(raw, "Sau khi lưu") || !strings.Contains(raw, out.result.Asset.ID) || strings.Contains(raw, old) {
		t.Fatalf("stale content=%s", raw)
	}
	if revision := w.scalar(t, `SELECT revision FROM app.question_groups WHERE id=$1`, group); revision != 3 {
		t.Fatalf("revision=%d", revision)
	}
}
func TestReplacementPoolSameOldContendersCommitOnlyOnce(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	if err := repositories.LockForVersionUse(context.Background(), tx, old); err != nil {
		t.Fatal(err)
	}
	a, b := w.input(old, w.a, domain.KindImage, 30, 30), w.input(old, w.a, domain.KindImage, 30, 30)
	first, second := w.startReplacement(a), w.startReplacement(b)
	p1 := w.blocked(t, tr, "FOR UPDATE", pid)
	p2 := w.blocked(t, tr, "FOR UPDATE", pid, p1)
	if p1 == p2 {
		t.Fatal("not two distinct contenders")
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	x, y := replacementWait(t, first), replacementWait(t, second)
	success, refused := x, y
	if x.err != nil {
		success, refused = y, x
	}
	if success.err != nil || !errors.Is(refused.err, domain.ErrNotFound) {
		t.Fatalf("outcomes=%v/%v", x.err, y.err)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=ANY($1::uuid[])`, []string{a.Asset.ID, b.Asset.ID}); got != 1 {
		t.Fatalf("committed replacements=%d", got)
	}
	if link := w.text(t, `SELECT replaced_by::text FROM app.media_assets WHERE id=$1`, old); link != success.result.Asset.ID {
		t.Fatalf("link=%s", link)
	}
}
func TestReplacementPoolDeletionWinsBeforeLockedRecheck(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	if err := repositories.LockForVersionUse(context.Background(), tx, old); err != nil {
		t.Fatal(err)
	}
	in := w.input(old, w.a, domain.KindImage, 30, 30)
	answer := w.startReplacement(in)
	w.blocked(t, tr, "FOR UPDATE", pid)
	if err := repositories.NewPostgres(db.NewContext(tx)).SoftDelete(context.Background(), domain.DeleteInput{ID: old, ActorID: w.a, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := replacementWait(t, answer)
	if !errors.Is(out.err, domain.ErrNotFound) {
		t.Fatalf("delete race=%v", out.err)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1`, in.Asset.ID); got != 0 {
		t.Fatalf("new rows=%d", got)
	}
}
func TestReplacementPoolUsesOwnerQuotaLockAndRereadsAfterWait(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	replacementExec(t, tx, `SELECT pg_advisory_xact_lock(73820,hashtext($1::uuid::text))`, w.a)
	in := w.input(old, w.a, domain.KindImage, 30, 40)
	answer := w.startReplacement(in)
	w.blocked(t, tr, "pg_advisory_xact_lock", pid)
	extra := w.input(old, w.a, domain.KindImage, 11, 40).Asset
	extra.OwnerID = w.a
	if _, err := repositories.Insert(context.Background(), tx, extra); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := replacementWait(t, answer)
	if !errors.Is(out.err, domain.ErrQuotaExceeded) {
		t.Fatalf("quota race=%v", out.err)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1`, in.Asset.ID); got != 0 {
		t.Fatalf("new rows=%d", got)
	}
}
func TestReplacementPoolOwnerChangeDuringAssetWaitAborts(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	if err := repositories.LockForVersionUse(context.Background(), tx, old); err != nil {
		t.Fatal(err)
	}
	in := w.input(old, w.a, domain.KindImage, 30, 30)
	answer := w.startReplacement(in)
	w.blocked(t, tr, "FOR UPDATE", pid)
	replacementExec(t, tx, `UPDATE app.media_assets SET owner_id=$2 WHERE id=$1`, old, w.b)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := replacementWait(t, answer)
	if !errors.Is(out.err, domain.ErrNotFound) {
		t.Fatalf("owner recheck=%v", out.err)
	}
}
func TestReplacementPoolNoSendCommitCancellationDisposesAndCannotLaterCommit(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	old := w.asset(t, w.a, domain.KindImage, 20)
	in := w.input(old, w.a, domain.KindImage, 30, 30)
	tr.cancelCommit.Store(true)
	_, err := w.repo.Replace(context.Background(), in)
	var failure *domain.ReplacementError
	if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementNotCommitted || tr.cancelledPID.Load() == 0 {
		t.Fatalf("no-send classification=%v pid=%d", err, tr.cancelledPID.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
	defer cancel()
	for {
		var exists bool
		if err := w.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1)`, tr.cancelledPID.Load()).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("canceled transaction session not reclaimed")
		case <-time.After(10 * time.Millisecond):
		}
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if rows := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1`, in.Asset.ID); rows != 0 {
		t.Fatalf("future borrower committed abandoned row=%d", rows)
	}
	if links := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1 AND replaced_by IS NOT NULL`, old); links != 0 {
		t.Fatalf("abandoned link=%d", links)
	}
	if logs := w.scalar(t, `SELECT count(*) FROM app.audit_log WHERE entity_id=ANY($1::uuid[])`, []string{old, in.Asset.ID}); logs != 0 {
		t.Fatalf("abandoned audit=%d", logs)
	}
	out, err := w.repo.Replace(ctx, w.input(old, w.a, domain.KindImage, 30, 30))
	if err != nil || out.Asset.ID == "" {
		t.Fatalf("pool unusable result=%+v err=%v", out, err)
	}
}
