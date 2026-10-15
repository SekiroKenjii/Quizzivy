//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

type importSeed struct {
	status   string
	finished *time.Time
	rows     int
}

func (w *workerWorld) seedJob(t *testing.T, s importSeed) string {
	t.Helper()
	w.dropJobsOnCleanup(t)
	w.app.SetUserImportRetention(w.queue)
	ctx := context.Background()
	role := w.role(t, "student")
	created := time.Now().Add(-60 * day)
	var started *time.Time
	if s.finished != nil {
		created = s.finished.Add(-time.Hour)
	}
	if s.status != "queued" {
		started = at(created.Add(time.Minute))
	}
	var code *string
	if s.status == "failed" {
		code = new(string)
		*code = domain.ImportFailedInterrupted
	}
	settled, outcome := 0, "pending"
	if s.status == "done" {
		settled, outcome = s.rows, "created"
	}
	var id string
	if err := w.admin.QueryRow(ctx, `
		INSERT INTO app.user_import_jobs
		  (created_by, request_id, filename, format, default_role_id, delivery, status, total_rows,
		   created_count, error_code, created_at, started_at, completed_at)
		VALUES ($1::uuid, gen_random_uuid(), 'hoc-vien.csv', 'csv', $2::uuid, 'email_link', $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text`,
		w.importer, role, s.status, s.rows, settled, code, created, started, s.finished).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.admin.Exec(ctx, `
		INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id, outcome)
		SELECT $1::uuid, g, 'Học viên ' || g, 'hv' || g || '-' || $1::text || '@' || $2::text, $3::uuid, $4::text
		  FROM generate_series(0, $5::int - 1) g`, id, w.marker, role, outcome, s.rows); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *workerWorld) stored(t *testing.T, ids ...string) (jobs, rows int) {
	t.Helper()
	jobs = w.count(t, `SELECT count(*) FROM app.user_import_jobs WHERE id::text = ANY($1::text[])`, ids)
	rows = w.count(t, `SELECT count(*) FROM app.user_import_rows WHERE job_id::text = ANY($1::text[])`, ids)
	return jobs, rows
}

func (w *workerWorld) pruneImports(t *testing.T, limit int) int {
	t.Helper()
	n, err := w.app.Commands.PruneUserImports.Handle(context.Background(), command.PruneUserImports{Limit: limit})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAnImportIsPrunedThirtyDaysAfterItFinishedWithItsRows(t *testing.T) {
	w := newWorkerWorld(t)
	now := time.Now()
	finishedAgo := func(age time.Duration) *time.Time { return at(now.Add(-age)) }

	cases := []struct {
		name string
		seed importSeed
		gone bool
	}{
		{"done 40 days ago", importSeed{"done", finishedAgo(40 * day), 3}, true},
		{"failed 31 days ago", importSeed{"failed", finishedAgo(31 * day), 2}, true},
		{"done just over 30 days ago", importSeed{"done", finishedAgo(30*day + time.Hour), 1}, true},
		{"failed just over 30 days ago", importSeed{"failed", finishedAgo(30*day + time.Hour), 1}, true},
		{"done just under 30 days ago", importSeed{"done", finishedAgo(29*day + 12*time.Hour), 2}, false},
		{"failed just under 30 days ago", importSeed{"failed", finishedAgo(29*day + 12*time.Hour), 2}, false},
		{"done a minute ago", importSeed{"done", finishedAgo(time.Minute), 2}, false},
		{"queued since 60 days ago", importSeed{"queued", nil, 2}, false},
		{"running since 60 days ago", importSeed{"running", nil, 2}, false},
	}
	ids := make([]string, len(cases))
	gone, goneRows := 0, 0
	for i, c := range cases {
		ids[i] = w.seedJob(t, c.seed)
		if c.gone {
			gone++
			goneRows += c.seed.rows
		}
	}
	account := w.insertUser(t, "hoc-vien", "student")
	if _, err := w.admin.Exec(context.Background(), `UPDATE app.user_import_rows SET user_id = $2::uuid WHERE job_id = $1::uuid AND ordinal = 0`, ids[0], account); err != nil {
		t.Fatal(err)
	}
	jobsBefore, rowsBefore := w.stored(t, ids...)

	removed := w.pruneImports(t, 10000)

	jobsAfter, rowsAfter := w.stored(t, ids...)
	for i, c := range cases {
		if jobs, rows := w.stored(t, ids[i]); (jobs == 0) != c.gone || (rows == 0) != c.gone {
			t.Errorf("%s: %d job(s) and %d row(s) left, want it %s", c.name, jobs, rows, map[bool]string{true: "gone", false: "kept"}[c.gone])
		}
	}
	if jobsBefore-jobsAfter != gone || rowsBefore-rowsAfter != goneRows || removed < gone {
		t.Errorf("%d job(s) and %d row(s) went and the prune reported %d, want %d job(s) and %d row(s)", jobsBefore-jobsAfter, rowsBefore-rowsAfter, removed, gone, goneRows)
	}
	if n := w.count(t, `SELECT count(*) FROM app.users WHERE id = $1::uuid`, account); n != 1 {
		t.Error("the account an import made went with the import")
	}

	w.pruneImports(t, 10000)
	if again, _ := w.stored(t, ids...); again != jobsAfter {
		t.Errorf("a second run left %d job(s), want %d", again, jobsAfter)
	}
}

func TestImportsArePrunedTheEarliestFirstAndAtMostLimitAtATime(t *testing.T) {
	w := newWorkerWorld(t)
	ids := make([]string, 5)
	for i := len(ids) - 1; i >= 0; i-- {
		ids[i] = w.seedJob(t, importSeed{"done", at(time.Now().Add(-3650*day - time.Duration(len(ids)-i)*day)), 2})
	}

	for step, want := range []int{2, 2, 1} {
		if n := w.pruneImports(t, 2); n != want {
			t.Fatalf("run %d pruned %d, want %d", step+1, n, want)
		}
		left := ids[min(len(ids), (step+1)*2):]
		if jobs, rows := w.stored(t, ids...); jobs != len(left) || rows != 2*len(left) {
			t.Fatalf("after run %d there are %d job(s) and %d row(s), want the %d newest and their rows", step+1, jobs, rows, len(left))
		}
		if len(left) > 0 {
			if jobs, _ := w.stored(t, left...); jobs != len(left) {
				t.Errorf("after run %d the jobs left are not the newest", step+1)
			}
		}
	}
}

func TestAtTheEdgeOnlyAnImportThatFinishedEarlierIsPruned(t *testing.T) {
	w := newWorkerWorld(t)
	edge := time.Now().Add(-domain.UserImportsKept).Truncate(time.Microsecond)

	type seeded struct {
		name string
		id   string
		gone bool
	}
	var jobs []seeded
	for _, offset := range []time.Duration{-time.Microsecond, 0, time.Microsecond} {
		for _, status := range []string{"done", "failed"} {
			jobs = append(jobs, seeded{status + " at " + offset.String(), w.seedJob(t, importSeed{status, at(edge.Add(offset)), 1}), offset < 0})
		}
	}

	if _, err := w.queue.Prune(context.Background(), edge, 10000); err != nil {
		t.Fatal(err)
	}

	for _, j := range jobs {
		if left, _ := w.stored(t, j.id); (left == 0) != j.gone {
			t.Errorf("an import %s: %d left, want it %s", j.name, left, map[bool]string{true: "gone", false: "kept"}[j.gone])
		}
	}
}

func TestAPruneSkipsAnImportAnotherTransactionHoldsAndTakesItNextTime(t *testing.T) {
	ctx := context.Background()
	w := newWorkerWorld(t)
	held := w.seedJob(t, importSeed{"done", at(time.Now().Add(-3650*day - 2*day)), 2})
	free := w.seedJob(t, importSeed{"done", at(time.Now().Add(-3650*day - day)), 2})

	tx, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err := tx.Exec(ctx, `SELECT 1 FROM app.user_import_jobs WHERE id = $1::uuid FOR UPDATE`, held); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct {
		n   int
		err error
	}, 1)
	go func() {
		n, err := w.app.Commands.PruneUserImports.Handle(ctx, command.PruneUserImports{Limit: 10000})
		done <- struct {
			n   int
			err error
		}{n, err}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
	case <-time.After(10 * time.Second):
		if err := tx.Rollback(ctx); err != nil {
			t.Error(err)
		}
		<-done
		t.Fatal("the prune waited for an import another transaction holds")
	}
	if jobs, rows := w.stored(t, held, free); jobs != 1 || rows != 2 {
		t.Fatalf("after the prune %d job(s) and %d row(s) are left, want only the held one with its 2 rows", jobs, rows)
	}
	if jobs, _ := w.stored(t, held); jobs != 1 {
		t.Fatal("the job that was left is not the held one")
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	w.pruneImports(t, 10000)
	if jobs, rows := w.stored(t, held, free); jobs != 0 || rows != 0 {
		t.Errorf("the next run left %d job(s) and %d row(s)", jobs, rows)
	}
}
