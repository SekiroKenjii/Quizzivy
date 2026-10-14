//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/domain"
)

func TestRequestQueuesAnExportAndAuditsIt(t *testing.T) {
	w := newWorld(t)
	admin := w.admin()
	got, err := w.repo.Request(context.Background(), domain.Request{ActorID: admin, IP: "203.0.113.7", UserAgent: "Mozilla/5.0", IncludeAnswers: true})
	if err != nil {
		t.Fatal(err)
	}
	w.track(got.ID)
	if got.Status != domain.Queued || got.Attempts != 0 || !got.IncludeAnswers || got.RequestedBy != admin {
		t.Errorf("export = %+v", got)
	}
	if got.Bytes != nil || got.CompletedAt != nil || got.ExpiresAt != nil || got.DownloadedAt != nil {
		t.Errorf("a new export already has an outcome: %+v", got)
	}
	if time.Since(got.CreatedAt) > time.Minute {
		t.Errorf("created at %v", got.CreatedAt)
	}
	var (
		actor, ip, agent, diff string
	)
	err = w.pool.QueryRow(context.Background(), `
		SELECT actor_user_id::text, host(ip), user_agent, diff::text
		  FROM app.audit_log WHERE action = 'data_export.requested' AND entity = 'data_export' AND entity_id = $1::uuid`, got.ID).
		Scan(&actor, &ip, &agent, &diff)
	if err != nil {
		t.Fatalf("the request was not audited: %v", err)
	}
	if actor != admin || ip != "203.0.113.7" || agent != "Mozilla/5.0" || diff != `{"includeAnswers": true}` {
		t.Errorf("audit = actor %s ip %s agent %s diff %s", actor, ip, agent, diff)
	}
}

func TestOnlyOneExportIsQueuedOrRunning(t *testing.T) {
	w := newWorld(t)
	first, second := w.admin(), w.admin()
	ctx := context.Background()
	queued, err := w.repo.Request(ctx, domain.Request{ActorID: first})
	if err != nil {
		t.Fatal(err)
	}
	w.track(queued.ID)

	for name, who := range map[string]string{"the same admin": first, "another admin": second} {
		if _, err := w.repo.Request(ctx, domain.Request{ActorID: who}); !errors.Is(err, domain.ErrInProgress) {
			t.Errorf("%s while one is queued: %v", name, err)
		}
	}
	claimed, err := w.repo.Claim(ctx)
	if err != nil || claimed.ID != queued.ID {
		t.Fatalf("claim = %+v, %v", claimed, err)
	}
	if _, err := w.repo.Request(ctx, domain.Request{ActorID: second}); !errors.Is(err, domain.ErrInProgress) {
		t.Errorf("while one is running: %v", err)
	}
	if n := w.auditCount("data_export.requested", queued.ID); n != 1 {
		t.Errorf("the refused requests were audited: %d rows", n)
	}

	if err := w.repo.Fail(ctx, claimed.ID, claimed.Attempts, domain.CodeFailed); err != nil {
		t.Fatal(err)
	}
	next, err := w.repo.Request(ctx, domain.Request{ActorID: second})
	if err != nil {
		t.Fatalf("a failed export still blocks the next: %v", err)
	}
	w.track(next.ID)
}

func TestARequestRefusedByTheIndexLeavesNoRowBehind(t *testing.T) {
	w := newWorld(t)
	admin := w.admin()
	w.export(admin, seed{status: "queued"})
	if _, err := w.repo.Request(context.Background(), domain.Request{ActorID: admin}); !errors.Is(err, domain.ErrInProgress) {
		t.Fatalf("error = %v", err)
	}
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.data_exports WHERE requested_by = $1::uuid`, admin).Scan(&n); err != nil || n != 1 {
		t.Fatalf("exports of the admin = %d, %v", n, err)
	}
}

func TestExportsRequestedAtOnceQueueExactlyOne(t *testing.T) {
	w := newWorld(t)
	admins := []string{w.admin(), w.admin(), w.admin(), w.admin()}
	const attempts = 12
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		queued   []string
		refused  int
		failures []error
	)
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := w.repo.Request(context.Background(), domain.Request{ActorID: admins[i%len(admins)], IncludeAnswers: i%2 == 0})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				queued = append(queued, got.ID)
			case errors.Is(err, domain.ErrInProgress):
				refused++
			default:
				failures = append(failures, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	for _, id := range queued {
		w.track(id)
	}
	if len(failures) != 0 {
		t.Fatalf("unexpected errors: %v", failures)
	}
	if len(queued) != 1 || refused != attempts-1 {
		t.Fatalf("queued %d, refused %d of %d concurrent requests", len(queued), refused, attempts)
	}
}

func TestGetReadsOnlyTheRequestersExport(t *testing.T) {
	w := newWorld(t)
	owner, other := w.admin(), w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour, includeAnswers: true})

	got, err := w.repo.Get(context.Background(), id, owner)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Status != domain.Ready || !got.IncludeAnswers || got.Bytes == nil || *got.Bytes != 1234 || got.ExpiresAt == nil {
		t.Errorf("export = %+v", got)
	}
	if _, err := w.repo.Get(context.Background(), id, other); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("another admin's read: %v", err)
	}
	if _, err := w.repo.Get(context.Background(), "0198f000-0000-7000-8000-00000000dead", owner); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a missing id: %v", err)
	}
}

func TestAReadyExportPastItsTimeReadsExpired(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	lapsed := w.export(owner, seed{status: "ready", expiresIn: -time.Minute})
	got, err := w.repo.Get(context.Background(), lapsed, owner)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.Expired {
		t.Errorf("status = %s, want expired", got.Status)
	}
	if stored := w.row(lapsed); stored.status != "ready" {
		t.Errorf("a read changed the row to %s; the housekeeping owns that", stored.status)
	}
}

func TestGetReportsEachStateAsStored(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	cases := map[string]seed{
		"queued":  {status: "queued"},
		"running": {status: "running", attempts: 1, ranFor: time.Second},
		"failed":  {status: "failed", attempts: 1, errorCode: domain.CodeFailed},
	}
	for want, s := range cases {
		id := w.export(owner, s)
		got, err := w.repo.Get(context.Background(), id, owner)
		if err != nil || string(got.Status) != want {
			t.Errorf("%s: %+v, %v", want, got, err)
		}
		w.exec(`DELETE FROM app.data_exports WHERE id = $1::uuid`, id)
	}
}
