//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/domain"
)

func (w *dueWorld) held(user string) map[string]heldRow {
	w.t.Helper()
	rows, err := w.tx.Query(context.Background(),
		`SELECT dedupe_key, kind, params::text, read_at FROM app.notifications WHERE user_id = $1::uuid`, user)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]heldRow{}
	for rows.Next() {
		var key string
		var row heldRow
		if err := rows.Scan(&key, &row.kind, &row.params, &row.readAt); err != nil {
			w.t.Fatal(err)
		}
		out[key] = row
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

type heldRow struct {
	kind   string
	params string
	readAt *time.Time
}

func (w *dueWorld) write(user string, notices []domain.Notice) int {
	w.t.Helper()
	written, err := w.store.InsertAbsent(context.Background(), user, notices)
	if err != nil {
		w.t.Fatalf("InsertAbsent: %v", err)
	}
	return written
}

func TestAWrittenDueItemIsNeverTouchedAgain(t *testing.T) {
	w := newDueWorld(t)
	w.assignment(spec{opens: at(-26 * hour), closes: at(30 * time.Minute)})
	due := w.due(w.student)
	if len(due) != 3 {
		t.Fatalf("the student has %d items due, want the opening and the two reminders", len(due))
	}
	if written := w.write(w.student, due); written != 3 {
		t.Fatalf("wrote %d, want 3", written)
	}

	read := at(time.Minute)
	opened := domain.OpenedKey(due[0].Target.AssignmentID)
	w.exec(`UPDATE app.notifications SET read_at = $2, params = '{"title":"Đã sửa"}' WHERE user_id = $1::uuid AND dedupe_key = $3`, w.student, read, opened)
	if written := w.write(w.student, due); written != 0 {
		t.Errorf("the same items wrote %d more rows", written)
	}
	row := w.held(w.student)[opened]
	if row.readAt == nil || !row.readAt.Equal(read) || row.params != `{"title": "Đã sửa"}` {
		t.Errorf("a notification the user had read was changed: %+v", row)
	}
	if got := len(w.held(w.student)); got != 3 {
		t.Errorf("the student holds %d rows, want 3", got)
	}
}

func TestASwitchTurnedOffKeepsItsKindsOutButNothingElse(t *testing.T) {
	cases := []struct {
		name    string
		event   domain.Event
		missing []domain.Kind
	}{
		{"test due soon", domain.EventAssignmentDueSoon, []domain.Kind{domain.AssignmentOpened, domain.AssignmentDueSoon}},
		{"result ready", domain.EventResultReady, []domain.Kind{domain.ResultReady}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newDueWorld(t)
			w.assignment(spec{opens: at(-26 * hour), closes: at(30 * time.Minute)})
			ended := w.assignment(spec{release: "after_close", opens: at(-3 * day), closes: at(-hour)})
			w.attempt(ended, w.student, "graded", at(-2*hour))
			w.prefer(w.student, c.event, false)

			w.write(w.student, w.due(w.student))
			kept := map[string]bool{}
			for _, row := range w.held(w.student) {
				kept[row.kind] = true
			}
			for _, kind := range []domain.Kind{domain.AssignmentOpened, domain.AssignmentDueSoon, domain.ResultReady} {
				silenced := slices.Contains(c.missing, kind)
				if kept[string(kind)] == silenced {
					t.Errorf("%s is kept = %v with %s off, want %v", kind, kept[string(kind)], c.event, !silenced)
				}
			}
		})
	}

	w := newDueWorld(t)
	w.assignment(spec{opens: at(-2 * day), closes: at(30 * time.Minute)})
	w.prefer(w.creator, domain.EventAssignmentClosing, false)
	w.write(w.creator, w.due(w.creator))
	if held := w.held(w.creator); len(held) != 0 {
		t.Errorf("a teacher who switched off closing notices holds %+v", held)
	}
	w.prefer(w.creator, domain.EventAssignmentClosing, true)
	if written := w.write(w.creator, w.due(w.creator)); written != 1 {
		t.Errorf("with the switch back on the teacher was written %d notices, want 1", written)
	}
}

func TestAnOtherSwitchDoesNotSilenceAKind(t *testing.T) {
	w := newDueWorld(t)
	w.assignment(spec{opens: at(-time.Hour), closes: at(5 * day)})
	for _, event := range domain.Events() {
		if event != domain.EventAssignmentDueSoon {
			w.prefer(w.student, event, false)
		}
	}
	if written := w.write(w.student, w.due(w.student)); written != 1 {
		t.Errorf("wrote %d with every other switch off, want the opening", written)
	}
}

func TestAResultReadyRowHoldsTheTitleAndNothingElse(t *testing.T) {
	w := newDueWorld(t)
	assignment := w.assignment(spec{release: "after_close", opens: at(-3 * day), closes: at(-hour)})
	attempt := w.attempt(assignment, w.student, "graded", at(-2*hour))
	w.exec(`UPDATE app.attempts SET score_earned = 7.5, score_total = 10 WHERE id = $1::uuid`, attempt)
	w.write(w.student, w.due(w.student))

	row, ok := w.held(w.student)[domain.ReadyKey(attempt)]
	if !ok {
		t.Fatal("no result row was written")
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(row.params), &params); err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params["title"] != "Đề kiểm tra Unit 5" {
		t.Errorf("the row holds %v, want only the title", params)
	}
	for _, leak := range []string{"7.5", "score", "band", "earned"} {
		if strings.Contains(row.params, leak) {
			t.Errorf("the row %s says %q", row.params, leak)
		}
	}
	var target string
	if err := w.tx.QueryRow(t.Context(), `SELECT target::text FROM app.notifications WHERE user_id = $1::uuid AND dedupe_key = $2`, w.student, domain.ReadyKey(attempt)).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(target, "7.5") {
		t.Errorf("the target %s carries the score", target)
	}
}

func TestInsertAbsentWithNothingToWriteReachesNothing(t *testing.T) {
	w := newDueWorld(t)
	if written := w.write(w.student, nil); written != 0 {
		t.Errorf("wrote %d from nothing", written)
	}
}
