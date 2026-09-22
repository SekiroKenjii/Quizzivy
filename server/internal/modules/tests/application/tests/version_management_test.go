//go:build integration

package application_test

import (
	"context"
	"errors"
	questionsquery "quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func versionRequest(t *testing.T, b *builder, id string, version int) domain.VersionRequest {
	t.Helper()
	current, err := b.tests.Queries.Get.Handle(context.Background(), query.Get{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return domain.VersionRequest{Request: reqFor(id, b.author), Version: version, ExpectedUpdatedAt: current.UpdatedAt}
}

func TestDefaultVersionAndDeletionNeverReusePublishedNumbers(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	draft := b.draft("Version sequence", b.shortAnswer("Original", "2.00"))
	ctx := context.Background()
	for range 2 {
		if _, err := b.publish(draft.ID); err != nil {
			t.Fatal(err)
		}
	}
	request := versionRequest(t, b, draft.ID, 1)
	current, err := b.tests.Commands.SetCurrentVersion.Handle(ctx, command.SetCurrentVersion{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	if current.CurrentVersion != 1 {
		t.Fatalf("default = %d", current.CurrentVersion)
	}
	if _, err := b.tests.Commands.SetCurrentVersion.Handle(ctx, command.SetCurrentVersion{Request: request}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale write = %v", err)
	}
	if _, err := b.tests.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: versionRequest(t, b, draft.ID, 1)}); !errors.Is(err, domain.ErrCurrentVersion) {
		t.Fatalf("current deletion = %v", err)
	}
	if _, err := b.tests.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: versionRequest(t, b, draft.ID, 2)}); err != nil {
		t.Fatal(err)
	}
	next, err := b.publish(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next.Version != 3 {
		t.Fatalf("next version = %d, want 3", next.Version)
	}
}

func TestRestoreVersionCopiesSnapshotInsteadOfEditedBankContent(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	original := b.shortAnswer("Original snapshot", "2.00")
	draft := b.draft("Restore", original)
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.questions SET prompt = 'Later bank edit', points = 99 WHERE id = $1`, original); err != nil {
		t.Fatal(err)
	}
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(ctx, command.CreateDraftFromVersion{Request: versionRequest(t, b, draft.ID, 1)})
	if err != nil {
		t.Fatal(err)
	}
	copyID := restored.Sections[0].QuestionIDs[0]
	if copyID == original {
		t.Fatal("restored draft still references the mutable source question")
	}
	var prompt string
	var points float64
	if err := pool.QueryRow(ctx, `SELECT prompt, points FROM app.questions WHERE id = $1`, copyID).Scan(&prompt, &points); err != nil {
		t.Fatal(err)
	}
	if prompt != "Original snapshot" || points != 2 {
		t.Fatalf("restored content = %q, %v", prompt, points)
	}
	if restored.CurrentVersion != 1 {
		t.Fatal("restoring the draft changed the assignment default")
	}
}

func TestPermanentTestDeletionRequiresArchive(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	draft := b.draft("Delete unused", b.shortAnswer("Unused", "1.00"))
	ctx := context.Background()
	if _, err := b.tests.Commands.Delete.Handle(ctx, command.Delete{Request: reqFor(draft.ID, b.author)}); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("active deletion = %v", err)
	}
	archived := domain.Archived
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: reqFor(draft.ID, b.author), Input: domain.UpdateInput{ExpectedUpdatedAt: draft.UpdatedAt, Status: &archived}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.tests.Commands.Delete.Handle(ctx, command.Delete{Request: reqFor(draft.ID, b.author)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.tests.Queries.Get.Handle(ctx, query.Get{ID: draft.ID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted test still accessible: %v", err)
	}
}

func TestAssignedVersionsCannotBeDeletedWithTheirParent(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	draft := b.draft("Assigned snapshot", b.shortAnswer("Assigned", "2.00"))
	first, err := b.publish(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var assignmentID string
	if err := pool.QueryRow(ctx, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
 VALUES ($1, $2, now(), now() + interval '1 day', 30, $3) RETURNING id::text`, draft.ID, first.ID, b.author).Scan(&assignmentID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM app.assignments WHERE id = $1`, assignmentID) })
	if _, err := b.tests.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: versionRequest(t, b, draft.ID, first.Version)}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("assigned version deletion = %v", err)
	}
	request := versionRequest(t, b, draft.ID, first.Version)
	archived := domain.Archived
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: request.Request, Input: domain.UpdateInput{ExpectedUpdatedAt: request.ExpectedUpdatedAt, Status: &archived}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.tests.Commands.Delete.Handle(ctx, command.Delete{Request: request.Request}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("assigned test deletion = %v", err)
	}
	versions, err := b.tests.Queries.ListVersions.Handle(ctx, query.ListVersions{TestID: draft.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatal("failed parent deletion partially removed versions")
	}
}

func TestQuestionUsageListsCurrentOutlinesButExcludesFrozenSnapshots(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	questionID := b.shortAnswer("Shared question", "1.00")
	beta := b.draft("Beta outline", questionID)
	alpha := b.draft("Alpha outline", questionID)
	ctx := context.Background()
	question, err := b.qsvc.Queries.Get.Handle(ctx, questionsquery.Get{ID: questionID})
	if err != nil {
		t.Fatal(err)
	}
	if len(question.UsedIn) != 2 || question.UsedInTests != 2 || question.UsedIn[0].ID != alpha.ID || question.UsedIn[1].ID != beta.ID {
		t.Fatalf("usage = %+v, count=%d", question.UsedIn, question.UsedInTests)
	}
	if _, err := b.publish(alpha.ID); err != nil {
		t.Fatal(err)
	}
	current, err := b.tests.Queries.Get.Handle(ctx, query.Get{ID: alpha.ID})
	if err != nil {
		t.Fatal(err)
	}
	sections := []domain.SectionInput{}
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: reqFor(alpha.ID, b.author), Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, Sections: sections, SetSections: true}}); err != nil {
		t.Fatal(err)
	}
	question, err = b.qsvc.Queries.Get.Handle(ctx, questionsquery.Get{ID: questionID})
	if err != nil {
		t.Fatal(err)
	}
	if len(question.UsedIn) != 1 || question.UsedInTests != 1 || question.UsedIn[0].ID != beta.ID {
		t.Fatalf("frozen snapshot counted as an editable reference: %+v", question.UsedIn)
	}
}
