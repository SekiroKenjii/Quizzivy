//go:build integration

package repositories_test

import (
	"context"
	"maps"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
)

func send(t *testing.T, app *application.Application, cmd command.Notify) {
	t.Helper()
	if _, err := app.Commands.Notify.Handle(context.Background(), cmd); err != nil {
		t.Fatalf("notify %s: %v", cmd.DedupeKey, err)
	}
}

func TestANoticeIsStoredAsTheContractsParamsAndTarget(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	assignmentID, attemptID := uuid.NewString(), uuid.NewString()
	send(t, app, command.Notify{
		UserID: userID, Kind: domain.AttemptFlagged,
		Params:    domain.Flagged{StudentName: "Lê Hoàng Nam", Title: "Đề giữa kỳ", FocusLost: 3},
		Target:    &domain.Target{Route: domain.RouteAttempt, AssignmentID: assignmentID, AttemptID: attemptID},
		DedupeKey: "flagged:" + attemptID, Merge: domain.Replace,
	})
	row := theRow(t, pool, userID)
	if row.kind != "attempt.flagged" || row.readAt != nil {
		t.Errorf("the row is a %s read at %v, want an unread attempt.flagged", row.kind, row.readAt)
	}
	if want := map[string]any{"studentName": "Lê Hoàng Nam", "title": "Đề giữa kỳ", "focusLost": float64(3)}; !maps.Equal(row.params, want) {
		t.Errorf("params are %v, want %v", row.params, want)
	}
	if want := map[string]any{"route": "attempt", "assignmentId": assignmentID, "attemptId": attemptID}; !maps.Equal(row.target, want) {
		t.Errorf("the target is %v, want %v", row.target, want)
	}
	items := page(t, app, userID, "", 20).Items
	if len(items) != 1 || items[0].ID != row.id || items[0].Kind != domain.AttemptFlagged || items[0].Target == nil ||
		*items[0].Target != (domain.Target{Route: domain.RouteAttempt, AssignmentID: assignmentID, AttemptID: attemptID}) {
		t.Errorf("the list shows %+v, want the stored row with its target", items)
	}

	send(t, app, command.Notify{
		UserID: userID, Kind: domain.JoinCodesRotated, Params: domain.CodesRotated{Count: 2, ClassNames: []string{"Lớp A", "Lớp B"}},
		DedupeKey: "join_codes.rotated", Merge: domain.Replace,
	})
	for _, n := range page(t, app, userID, "", 20).Items {
		if n.Kind == domain.JoinCodesRotated && n.Target != nil {
			t.Errorf("a notice without a target is listed with %+v", n.Target)
		}
	}
}

func TestTheSameKeyForTwoUsersIsTwoRows(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	a, b := newUser(t, pool, "teacher"), newUser(t, pool, "teacher")
	key := "closing:" + uuid.NewString()
	notify(t, app, a, key, domain.Closing{Title: "Đề của A", NotSubmitted: 4}, domain.Replace)
	notify(t, app, b, key, domain.Closing{Title: "Đề của B", NotSubmitted: 7}, domain.Replace)
	notify(t, app, a, key, domain.Closing{Title: "Đề của A", NotSubmitted: 3}, domain.Replace)

	mine, theirs := theRow(t, pool, a), theRow(t, pool, b)
	if mine.params["notSubmitted"] != float64(3) || mine.params["title"] != "Đề của A" {
		t.Errorf("the first user's row holds %v, want their own latest notice", mine.params)
	}
	if theirs.params["notSubmitted"] != float64(7) || theirs.params["title"] != "Đề của B" {
		t.Errorf("the second user's row holds %v: a notice for one user reached the other's row", theirs.params)
	}
}

func TestReplaceLeavesTheSecondParams(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	key := "submitted:" + uuid.NewString() + ":1958400"
	notify(t, app, userID, key, domain.Submitted{Title: "Đề A", Count: 2, ToGrade: 1}, domain.Replace)
	first := theRow(t, pool, userID)
	time.Sleep(20 * time.Millisecond)
	notify(t, app, userID, key, domain.Submitted{Title: "Đề B", Count: 3, ToGrade: 0}, domain.Replace)
	second := theRow(t, pool, userID)
	if want := map[string]any{"title": "Đề B", "count": float64(3), "toGrade": float64(0)}; !maps.Equal(second.params, want) {
		t.Errorf("after two replacing notices the params are %v, want the second's %v", second.params, want)
	}
	if second.id != first.id || !second.createdAt.Equal(first.createdAt) {
		t.Errorf("the merge moved the row from %s at %v to %s at %v", first.id, first.createdAt, second.id, second.createdAt)
	}
	if !second.updatedAt.After(first.updatedAt) {
		t.Errorf("the merge left updated_at at %v, was %v", second.updatedAt, first.updatedAt)
	}
}

func TestAddSumsTheCountsAndTakesEverythingElseFromTheNewNotice(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	key := "submitted:" + uuid.NewString() + ":1958400"
	first, second := uuid.NewString(), uuid.NewString()
	send(t, app, command.Notify{
		UserID: userID, Kind: domain.AttemptSubmitted, Params: domain.Submitted{Title: "Đề A", Count: 2, ToGrade: 1},
		Target: &domain.Target{Route: domain.RouteAssignment, AssignmentID: first}, DedupeKey: key, Merge: domain.Add,
	})
	if got := theRow(t, pool, userID).params; got["count"] != float64(2) || got["toGrade"] != float64(1) {
		t.Errorf("the first notice stored %v, want its own counts", got)
	}
	mark(t, app, userID, theRow(t, pool, userID).id)
	if got := unread(t, app, userID); got != 0 {
		t.Fatalf("%d unread after marking the only row", got)
	}

	send(t, app, command.Notify{
		UserID: userID, Kind: domain.AttemptSubmitted, Params: domain.Submitted{Title: "Đề A (đã sửa)", Count: 3, ToGrade: 4},
		Target: &domain.Target{Route: domain.RouteGrading, AssignmentID: second}, DedupeKey: key, Merge: domain.Add,
	})
	row := theRow(t, pool, userID)
	if want := map[string]any{"title": "Đề A (đã sửa)", "count": float64(5), "toGrade": float64(5)}; !maps.Equal(row.params, want) {
		t.Errorf("after 2 then 3 papers and 1 then 4 to grade the params are %v, want %v", row.params, want)
	}
	if want := map[string]any{"route": "grading", "assignmentId": second}; !maps.Equal(row.target, want) {
		t.Errorf("the target is %v, want the new notice's %v", row.target, want)
	}
	if row.readAt != nil || unread(t, app, userID) != 1 {
		t.Errorf("a merged row stayed read at %v: a count that grew is news again", row.readAt)
	}
}

func TestAddCountsAMissingStoredValueAsZeroAndAddsOnlyWhatTheNoticeCarries(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	key := "submitted:" + uuid.NewString() + ":1958400"
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.notifications (user_id, kind, params, dedupe_key)
		VALUES ($1::uuid, 'attempt.submitted', '{"title":"Đề cũ","toGrade":2}'::jsonb, $2)`, userID, key); err != nil {
		t.Fatal(err)
	}
	notify(t, app, userID, key, domain.Submitted{Title: "Đề A", Count: 2, ToGrade: 1}, domain.Add)
	if want, got := map[string]any{"title": "Đề A", "count": float64(2), "toGrade": float64(3)}, theRow(t, pool, userID).params; !maps.Equal(got, want) {
		t.Errorf("added onto a row with no count the params are %v, want %v", got, want)
	}

	notify(t, app, userID, key, domain.Ready{Title: "Đề A"}, domain.Add)
	row := theRow(t, pool, userID)
	if want := map[string]any{"title": "Đề A"}; !maps.Equal(row.params, want) || row.kind != "result.ready" {
		t.Errorf("a notice that carries no count left %s %v, want its own params alone: %v", row.kind, row.params, want)
	}
}

func TestASwitchedOffEventWritesNothing(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID, other := newUser(t, pool, "student"), newUser(t, pool, "student")
	off := domain.WithDefaults(nil)
	off[3].InApp = false
	save(t, app, userID, off)

	closes := time.Now().Add(24 * time.Hour)
	notify(t, app, userID, "opened:1", domain.Opened{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.Replace)
	notify(t, app, userID, "due_soon:1", domain.DueSoon{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.Replace)
	notify(t, app, userID, "extended:1", domain.Extended{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.Replace)
	if rows := rowsOf(t, pool, userID); len(rows) != 0 {
		t.Errorf("with Test due soon off %d rows were written, want none: %+v", len(rows), rows)
	}
	notify(t, app, other, "due_soon:1", domain.DueSoon{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.Replace)
	if rows := rowsOf(t, pool, other); len(rows) != 1 {
		t.Errorf("one user's switch stopped another user's notice: %d rows", len(rows))
	}

	notify(t, app, userID, "result:1", domain.Ready{Title: "Đề giữa kỳ"}, domain.Replace)
	notify(t, app, userID, "joined:1", domain.Joined{StudentName: "Bảo", ClassName: "Lớp A"}, domain.Replace)
	if rows := rowsOf(t, pool, userID); len(rows) != 2 {
		t.Fatalf("a switch that is on and a kind without a switch wrote %d rows, want 2", len(rows))
	}

	everything := domain.WithDefaults(nil)
	for i := range everything {
		everything[i].InApp = false
	}
	save(t, app, userID, everything)
	notify(t, app, userID, "result:1", domain.Ready{Title: "Đề đã đổi tên"}, domain.Replace)
	notify(t, app, userID, "rotated", domain.CodesRotated{Count: 1, ClassNames: []string{"Lớp A"}}, domain.Replace)
	rows := rowsOf(t, pool, userID)
	if len(rows) != 3 {
		t.Fatalf("with every switch off the user holds %d rows, want the two from before and the one kind without a switch", len(rows))
	}
	for _, r := range rows {
		if r.kind == "result.ready" && r.params["title"] != "Đề giữa kỳ" {
			t.Errorf("a switched-off notice merged into the stored row: %v", r.params)
		}
	}

	on := domain.WithDefaults(nil)
	save(t, app, userID, on)
	notify(t, app, userID, "due_soon:1", domain.DueSoon{Title: "Đề giữa kỳ", ClosesAt: closes}, domain.Replace)
	if rows := rowsOf(t, pool, userID); len(rows) != 4 {
		t.Errorf("with the switch back on the user holds %d rows, want 4", len(rows))
	}
}

func TestConcurrentAddsLoseNoIncrement(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	const writers = 20
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = writers
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	userID := newUser(t, pool, "teacher")
	key := "submitted:" + uuid.NewString() + ":1958400"

	conns := make([]*pgxpool.Conn, writers)
	for i := range conns {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		conns[i] = conn
	}
	start := make(chan struct{})
	failures := make(chan error, writers)
	var done sync.WaitGroup
	for _, conn := range conns {
		done.Go(func() {
			defer conn.Release()
			app := application.New(repositories.NewPostgres(db.NewContext(conn)))
			<-start
			_, err := app.Commands.Notify.Handle(ctx, command.Notify{
				UserID: userID, Kind: domain.AttemptSubmitted, Params: domain.Submitted{Title: "Đề giữa kỳ", Count: 1, ToGrade: 1},
				DedupeKey: key, Merge: domain.Add,
			})
			failures <- err
		})
	}
	close(start)
	done.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Errorf("a concurrent notice failed: %v", err)
		}
	}

	rows := rowsOf(t, pool, userID)
	if len(rows) != 1 {
		t.Fatalf("%d writers left %d rows, want exactly one", writers, len(rows))
	}
	if rows[0].params["count"] != float64(writers) || rows[0].params["toGrade"] != float64(writers) {
		t.Errorf("%d writers each added one and the row holds count %v and toGrade %v, want %d each",
			writers, rows[0].params["count"], rows[0].params["toGrade"], writers)
	}
}
