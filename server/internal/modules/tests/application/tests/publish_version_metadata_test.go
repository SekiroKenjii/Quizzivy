//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/repositories"

	questionsrepo "quizzivy/internal/modules/questions/repositories"
)

func (b *builder) publishWithNote(testID string, note *string) (domain.Version, error) {
	app := application.New(repositories.NewPostgres(db.NewContext(b.pool), questionsrepo.NewPostgres(db.NewContext(b.pool)), mediarepo.NewPostgres(db.NewContext(b.pool))))
	return app.Commands.Publish.Handle(context.Background(), command.Publish{Request: domain.PublishRequest{
		TestID: testID, ActorID: b.author, ChangeNote: note, Scope: access.Scope{UserID: b.author},
	}})
}

func textPtr(s string) *string { return &s }

func TestPublishKeepsTheTrimmedChangeNoteAndTheHistoryReadsItBack(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	draft := b.draft("Ghi chú phiên bản", b.shortAnswer("Câu một", "1.00"))

	noted, err := b.publishWithNote(draft.ID, textPtr("  Sửa lại câu một \n"))
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	if noted.ChangeNote == nil || *noted.ChangeNote != "Sửa lại câu một" {
		t.Fatalf("v1 note = %v, want the trimmed text", noted.ChangeNote)
	}
	blank, err := b.publishWithNote(draft.ID, textPtr(" \t\n"))
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if blank.ChangeNote != nil {
		t.Fatalf("v2 note = %q, want none for a blank note", *blank.ChangeNote)
	}
	absent, err := b.publishWithNote(draft.ID, nil)
	if err != nil {
		t.Fatalf("publish v3: %v", err)
	}
	if absent.ChangeNote != nil {
		t.Fatalf("v3 note = %q, want none", *absent.ChangeNote)
	}

	versions, err := b.tests.Queries.ListVersions.Handle(context.Background(), query.ListVersions{TestID: draft.ID, Scope: everyone})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("want 3 versions, got %d", len(versions))
	}
	if versions[2].ChangeNote == nil || *versions[2].ChangeNote != "Sửa lại câu một" {
		t.Errorf("history v1 note = %v", versions[2].ChangeNote)
	}
	if versions[1].ChangeNote != nil || versions[0].ChangeNote != nil {
		t.Errorf("history notes of v2 and v3 = %v and %v, want none", versions[1].ChangeNote, versions[0].ChangeNote)
	}
	if versions[0].TestUpdatedAt != nil {
		t.Errorf("the history carries a test update time %v; only the publish answer does", versions[0].TestUpdatedAt)
	}
}

func TestPublishAnswersTheTestUpdateTimeSoTheNextSaveIsNotStale(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	draft := b.draft("Lưu sau khi xuất bản", b.shortAnswer("Câu hỏi", "1.00"))

	published, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.TestUpdatedAt == nil {
		t.Fatal("the publish answer has no test update time")
	}
	if !published.TestUpdatedAt.After(draft.UpdatedAt) {
		t.Fatalf("test update time %v is not after the draft's %v: the publish did not move it", *published.TestUpdatedAt, draft.UpdatedAt)
	}

	ctx := context.Background()
	var stored time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, draft.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.Equal(*published.TestUpdatedAt) {
		t.Fatalf("the answer says %v, the table holds %v", *published.TestUpdatedAt, stored)
	}

	title := "Đổi tên sau khi xuất bản"
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: reqFor(draft.ID, b.author), Input: domain.UpdateInput{
		ExpectedUpdatedAt: draft.UpdatedAt, Title: &title,
	}}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("a save with the pre-publish time = %v, want STALE_WRITE", err)
	}
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: reqFor(draft.ID, b.author), Input: domain.UpdateInput{
		ExpectedUpdatedAt: *published.TestUpdatedAt, Title: &title,
	}}); err != nil {
		t.Fatalf("a save with the publish answer's time: %v", err)
	}
}

type assignmentWindow struct {
	published bool
	opens     time.Duration
	closes    time.Duration
	closedAt  *time.Duration
}

func hoursFromNow(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

func insertAssignment(t *testing.T, pool *pgxpool.Pool, testID, versionID, creator string, w assignmentWindow) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	var publishedAt, closedAt *time.Time
	if w.published {
		at := now.Add(-48 * time.Hour)
		publishedAt = &at
	}
	if w.closedAt != nil {
		at := now.Add(*w.closedAt)
		closedAt = &at
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.assignments
		(test_id, test_version_id, opens_at, closes_at, closed_at, published_at, duration_minutes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 30, $7)`,
		testID, versionID, now.Add(w.opens), now.Add(w.closes), closedAt, publishedAt, creator); err != nil {
		t.Fatalf("insert assignment: %v", err)
	}
}

func removeAssignmentsOnCleanup(t *testing.T, pool *pgxpool.Pool, testID string) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.assignments WHERE test_id = $1`, testID); err != nil {
			t.Errorf("cleanup assignments of test %s: %v", testID, err)
		}
	})
}

func TestAVersionCountsEveryAssignmentThatNamesIt(t *testing.T) {
	pool := newPool(t)
	owner := pubMakeAuthor(t, pool)
	colleague := pubMakeAuthor(t, pool)
	b := newBuilder(t, pool, owner)
	draft := b.draft("Số lượt giao", b.shortAnswer("Câu hỏi", "1.00"))
	removeAssignmentsOnCleanup(t, pool, draft.ID)

	first, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	second, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if first.AssignmentCount != 0 || second.AssignmentCount != 0 {
		t.Fatalf("a new version counts %d and %d assignments, want none", first.AssignmentCount, second.AssignmentCount)
	}

	insertAssignment(t, pool, draft.ID, first.ID, owner, assignmentWindow{published: false, opens: hoursFromNow(1), closes: hoursFromNow(2)})
	insertAssignment(t, pool, draft.ID, first.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-1), closes: hoursFromNow(1)})
	insertAssignment(t, pool, draft.ID, first.ID, colleague, assignmentWindow{published: true, opens: hoursFromNow(-3), closes: hoursFromNow(-1)})

	versions, err := b.tests.Queries.ListVersions.Handle(context.Background(), query.ListVersions{TestID: draft.ID, Scope: everyone})
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 || versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("versions = %+v, want [2 1]", versions)
	}
	if versions[1].AssignmentCount != 3 {
		t.Errorf("version 1 counts %d assignments, want the draft, the open one and the colleague's closed one", versions[1].AssignmentCount)
	}
	if versions[0].AssignmentCount != 0 {
		t.Errorf("version 2 counts %d assignments, want none", versions[0].AssignmentCount)
	}
}

func TestATestCountsItsAssignmentsByDerivedStatus(t *testing.T) {
	pool := newPool(t)
	owner := pubMakeAuthor(t, pool)
	colleague := pubMakeAuthor(t, pool)
	b := newBuilder(t, pool, owner)
	title := "Đếm theo trạng thái"
	draft := b.draft(title, b.shortAnswer("Câu hỏi", "1.00"))
	other := b.draft("Đề khác", b.shortAnswer("Câu khác", "1.00"))
	removeAssignmentsOnCleanup(t, pool, draft.ID)
	removeAssignmentsOnCleanup(t, pool, other.ID)

	first, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	second, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	otherVersion, err := b.publish(other.ID)
	if err != nil {
		t.Fatalf("publish other: %v", err)
	}

	early := hoursFromNow(-1)
	pending := hoursFromNow(1)
	insertAssignment(t, pool, draft.ID, first.ID, owner, assignmentWindow{published: false, opens: hoursFromNow(-1), closes: hoursFromNow(1)})
	insertAssignment(t, pool, draft.ID, first.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(1), closes: hoursFromNow(2)})
	insertAssignment(t, pool, draft.ID, first.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-1), closes: hoursFromNow(1)})
	insertAssignment(t, pool, draft.ID, second.ID, colleague, assignmentWindow{published: true, opens: hoursFromNow(-2), closes: hoursFromNow(2)})
	insertAssignment(t, pool, draft.ID, second.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-3), closes: hoursFromNow(-1)})
	insertAssignment(t, pool, draft.ID, second.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-3), closes: hoursFromNow(1), closedAt: &early})
	insertAssignment(t, pool, draft.ID, second.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-3), closes: hoursFromNow(2), closedAt: &pending})
	insertAssignment(t, pool, other.ID, otherVersion.ID, owner, assignmentWindow{published: true, opens: hoursFromNow(-1), closes: hoursFromNow(1)})

	want := domain.AssignmentCounts{Live: 3, Scheduled: 1, Closed: 2}
	ctx := context.Background()

	got, err := b.tests.Queries.Get.Handle(ctx, query.Get{ID: draft.ID, Scope: everyone})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Assignments != want {
		t.Errorf("getTest counts %+v, want %+v (drafts left out, every creator, every version)", got.Assignments, want)
	}

	listed, err := b.tests.Queries.List.Handle(ctx, query.List{Input: domain.ListInput{Query: title, Scope: access.Scope{UserID: owner}}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != draft.ID {
		t.Fatalf("list = %+v, want only %q", listed.Items, title)
	}
	if listed.Items[0].Assignments != want {
		t.Errorf("list row counts %+v, want %+v", listed.Items[0].Assignments, want)
	}

	if untouched, err := b.tests.Queries.Get.Handle(ctx, query.Get{ID: other.ID, Scope: everyone}); err != nil {
		t.Fatalf("get other: %v", err)
	} else if untouched.Assignments != (domain.AssignmentCounts{Live: 1}) {
		t.Errorf("the other test counts %+v, want one live", untouched.Assignments)
	}
}
