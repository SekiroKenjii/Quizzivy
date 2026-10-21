//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/audit/application/command"
	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/shared/actor"
)

func (w *world) start(c command.StartExport) (domain.ExportPlan, error) {
	w.t.Helper()
	return w.app.Commands.StartExport.Handle(context.Background(), c)
}

func (w *world) drain(plan domain.ExportPlan) []domain.Entry {
	w.t.Helper()
	var all []domain.Entry
	var after *domain.Cursor
	for {
		batch, err := w.app.Queries.ExportBatch.Handle(context.Background(), query.ExportBatch{Filter: plan.Filter, After: after})
		if err != nil {
			w.t.Fatal(err)
		}
		all = append(all, batch.Entries...)
		if batch.Next == nil {
			return all
		}
		after = batch.Next
	}
}

func (w *world) exports(actorID string) []string {
	w.t.Helper()
	rows, err := w.pool.Query(context.Background(),
		`SELECT diff::text FROM app.audit_log WHERE actor_user_id = $1::uuid AND action = 'audit.exported' ORDER BY id`, actorID)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var diff string
		if err := rows.Scan(&diff); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, diff)
	}
	return out
}

func (w *world) exporter(name string) actor.Actor {
	if name == "" {
		name = "Người xuất " + nonce(w.t)
	}
	id := w.user("teacher", name, "")
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`, id); err != nil {
			w.t.Errorf("removing the export entries: %v", err)
		}
	})
	return actor.Actor{ID: id, IP: "198.51.100.4", UserAgent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0"}
}

func TestAnExportReadsEveryEntryOnceInTheListsOrderAcrossBatches(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	var want []int64
	for i := range 2500 {
		want = append(want, w.rows(by, march(10+i%5, 9, 0)))
	}
	slices.Reverse(want)
	caller := w.exporter("")

	plan, err := w.start(command.StartExport{Actor: caller, Period: domain.Month, Format: "csv", Query: w.nonce})
	if err != nil {
		t.Fatal(err)
	}
	got := w.drain(plan)

	if plan.Rows != 2500 || len(got) != 2500 {
		t.Fatalf("plan counts %d, the batches hold %d, want 2500", plan.Rows, len(got))
	}
	list := w.list(query.List{Range: domain.Month, Query: w.nonce, Limit: 100})
	if !slices.Equal(ids(got[:100]), ids(list.Entries)) {
		t.Errorf("the export starts %v, the list %v", ids(got[:5]), ids(list.Entries[:5]))
	}
	seen := map[int64]bool{}
	for i, e := range got {
		if seen[e.ID] {
			t.Fatalf("entry %d is exported twice", e.ID)
		}
		seen[e.ID] = true
		if i > 0 && (e.OccurredAt.After(got[i-1].OccurredAt) || (e.OccurredAt.Equal(got[i-1].OccurredAt) && e.ID > got[i-1].ID)) {
			t.Fatalf("entry %d at %v follows %d at %v: not newest first", e.ID, e.OccurredAt, got[i-1].ID, got[i-1].OccurredAt)
		}
	}
}

func (w *world) rows(actorID string, at time.Time) int64 {
	return w.write(entry{actor: actorID, action: "x.y", at: at})
}

func TestAnExportIsAuditedBeforeItsFirstEntryAndIsNotInItself(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	w.rows(by, march(12, 9, 0))
	caller := w.exporter("Người xuất " + w.nonce)

	plan, err := w.start(command.StartExport{Actor: caller, Period: domain.Week, Format: "json", Category: domain.Classes, Query: "người xuất " + w.nonce})
	if err != nil {
		t.Fatal(err)
	}

	recorded := w.exports(caller.ID)
	if len(recorded) != 1 {
		t.Fatalf("%d export entries, want 1", len(recorded))
	}
	var diff map[string]any
	if err := json.Unmarshal([]byte(recorded[0]), &diff); err != nil {
		t.Fatal(err)
	}
	if diff["period"] != "7d" || diff["format"] != "json" || diff["rows"] != float64(0) || diff["category"] != "classes" || diff["q"] != "người xuất "+w.nonce {
		t.Errorf("diff %v", diff)
	}
	var action, entity, ip, userAgent string
	var at time.Time
	if err := w.pool.QueryRow(context.Background(),
		`SELECT action, entity, host(ip), user_agent, occurred_at FROM app.audit_log WHERE actor_user_id = $1::uuid AND action = 'audit.exported'`, caller.ID).
		Scan(&action, &entity, &ip, &userAgent, &at); err != nil {
		t.Fatal(err)
	}
	if entity != "audit_log" || ip != "198.51.100.4" || userAgent != caller.UserAgent || !at.Equal(march(15, 12, 0)) {
		t.Errorf("entry %s about %s from %s %q at %v", action, entity, ip, userAgent, at)
	}

	everything, err := w.start(command.StartExport{Actor: caller, Period: domain.Week, Format: "csv", Query: "người xuất " + w.nonce})
	if err != nil {
		t.Fatal(err)
	}
	if everything.Rows != 1 {
		t.Fatalf("the second export counts %d, want only the first export's entry", everything.Rows)
	}
	if got := w.drain(everything); len(got) != 1 || got[0].Action != "audit.exported" {
		t.Errorf("entries %+v", got)
	}
	if len(w.drain(plan)) != 0 || plan.Rows != 0 {
		t.Errorf("the first export holds %d rows, its own audit entry among them", plan.Rows)
	}
}

func TestAnExportReadsTheLogAsItStoodWhenItBegan(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	for i := range 5 {
		w.rows(by, march(12, 9, i))
	}
	plan, err := w.start(command.StartExport{Actor: w.exporter(""), Period: domain.Week, Format: "csv", Query: w.nonce})
	if err != nil {
		t.Fatal(err)
	}

	late := w.rows(by, march(14, 9, 0))
	got := w.drain(plan)

	if plan.Rows != 5 || len(got) != 5 {
		t.Errorf("counted %d and read %d, want 5 of each", plan.Rows, len(got))
	}
	if slices.Contains(ids(got), late) {
		t.Error("an entry written after the export began is in it")
	}
}

func TestAnExportOfNothingIsAnEmptyFileAndStillAudited(t *testing.T) {
	w := newWorld(t)
	caller := w.exporter("")
	plan, err := w.start(command.StartExport{Actor: caller, Period: domain.Month, Format: "csv", Query: "không có gì " + w.nonce})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rows != 0 || len(w.drain(plan)) != 0 || len(w.exports(caller.ID)) != 1 {
		t.Errorf("rows %d, exports %v", plan.Rows, w.exports(caller.ID))
	}
}

func TestOnlyTheThreePeriodsAndTheSixCategoriesAreExportable(t *testing.T) {
	w := newWorld(t)
	for _, period := range []domain.Range{domain.Today, domain.Forever, domain.Custom, "decade"} {
		if _, err := w.start(command.StartExport{Actor: w.exporter(""), Period: period, Format: "csv"}); !errors.Is(err, domain.ErrUnknownRange) {
			t.Errorf("period %q: %v", period, err)
		}
	}
	if _, err := w.start(command.StartExport{Actor: w.exporter(""), Period: domain.Week, Category: "organization"}); !errors.Is(err, domain.ErrUnknownCategory) {
		t.Errorf("category: %v", err)
	}
}

func TestTheYearIsCountedFromTheFirstOfJanuaryInTheViewersZone(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	w.rows(by, time.Date(2000, time.December, 31, 23, 59, 0, 0, vietnam))
	inside := w.rows(by, time.Date(2001, time.January, 1, 0, 0, 0, 0, vietnam))
	plan, err := w.start(command.StartExport{Actor: w.exporter(""), Period: domain.Year, Format: "csv", Query: w.nonce})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.drain(plan); !slices.Equal(ids(got), []int64{inside}) {
		t.Errorf("the year reads %v, want only %d", ids(got), inside)
	}
}

func TestMoreThanTheCapIsRefusedAndNothingIsAudited(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	w.exec(`
		INSERT INTO app.audit_log (actor_user_id, action, entity, occurred_at)
		SELECT $1::uuid, 'cap.test', 'class', $2::timestamptz + make_interval(secs => g) FROM generate_series(1, $3::int) g`,
		by, march(10, 0, 0), domain.MaxExportRows)
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`, by); err != nil {
			t.Errorf("removing the cap rows: %v", err)
		}
	})
	caller := w.exporter("")

	plan, err := w.start(command.StartExport{Actor: caller, Period: domain.Month, Format: "csv", Query: w.nonce})
	if err != nil {
		t.Fatalf("exactly %d entries should be allowed: %v", domain.MaxExportRows, err)
	}
	if plan.Rows != domain.MaxExportRows {
		t.Fatalf("plan counts %d", plan.Rows)
	}
	read := 0
	var after *domain.Cursor
	for {
		batch, err := w.app.Queries.ExportBatch.Handle(context.Background(), query.ExportBatch{Filter: plan.Filter, After: after})
		if err != nil {
			t.Fatal(err)
		}
		read += len(batch.Entries)
		if batch.Next == nil {
			break
		}
		after = batch.Next
	}
	if read != domain.MaxExportRows {
		t.Errorf("read %d entries, want %d", read, domain.MaxExportRows)
	}
	if len(w.exports(caller.ID)) != 1 {
		t.Errorf("%d exports recorded", len(w.exports(caller.ID)))
	}

	w.rows(by, march(11, 0, 0))
	if _, err := w.start(command.StartExport{Actor: caller, Period: domain.Month, Format: "csv", Query: w.nonce}); !errors.Is(err, domain.ErrExportTooLarge) {
		t.Fatalf("one over the cap: %v, want ErrExportTooLarge", err)
	}
	if len(w.exports(caller.ID)) != 1 {
		t.Errorf("a refused export was audited: %d recorded", len(w.exports(caller.ID)))
	}
}

func TestACountStopsAtOneOverTheCap(t *testing.T) {
	w := newWorld(t)
	by := w.user("teacher", "Người làm "+w.nonce, "")
	w.exec(`
		INSERT INTO app.audit_log (actor_user_id, action, entity, occurred_at)
		SELECT $1::uuid, 'cap.test', 'class', $2::timestamptz + make_interval(secs => g) FROM generate_series(1, $3::int) g`,
		by, march(10, 0, 0), domain.MaxExportRows+500)
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`, by); err != nil {
			t.Errorf("removing the cap rows: %v", err)
		}
	})
	f := w.march()
	f.Query = w.nonce

	counted, err := w.repo.CountExport(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if counted.Rows != domain.MaxExportRows+1 {
		t.Errorf("counted %d, want %d: it reads one past the cap and stops", counted.Rows, domain.MaxExportRows+1)
	}
}
