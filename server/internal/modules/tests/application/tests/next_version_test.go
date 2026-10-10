//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"testing"
)

func nextVersionRead(t *testing.T, b *builder, id string) int {
	t.Helper()
	got, err := b.tests.Queries.Get.Handle(context.Background(), query.Get{ID: id, Scope: everyone})
	if err != nil {
		t.Fatal(err)
	}
	return got.NextVersion
}

func nextVersionListed(t *testing.T, b *builder, title string) int {
	t.Helper()
	listed, err := b.tests.Queries.List.Handle(context.Background(), query.List{Input: domain.ListInput{Query: title, Scope: access.Scope{UserID: b.author}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("listed %d tests titled %q, want 1", len(listed.Items), title)
	}
	return listed.Items[0].NextVersion
}

func TestTheNextVersionIsWhatTheNextPublishAssigns(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, pubMakeAuthor(t, pool))
	title := "Số phiên bản kế tiếp " + b.author[:8]
	draft := b.draft(title, b.shortAnswer("Câu một", "2.00"))
	ctx := context.Background()

	if draft.NextVersion != 1 {
		t.Fatalf("a test never published reports next version %d on creation, want 1", draft.NextVersion)
	}
	for want := 1; want <= 3; want++ {
		if got := nextVersionRead(t, b, draft.ID); got != want {
			t.Fatalf("before publish %d the test reports next version %d", want, got)
		}
		published, err := b.publish(draft.ID)
		if err != nil {
			t.Fatalf("publish %d: %v", want, err)
		}
		if published.Version != want {
			t.Fatalf("publish assigned %d, the test had reported %d", published.Version, want)
		}
	}

	request := versionRequest(t, b, draft.ID, 1)
	setDefault, err := b.tests.Commands.SetCurrentVersion.Handle(ctx, command.SetCurrentVersion{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	if setDefault.CurrentVersion != 1 || setDefault.NextVersion != 4 {
		t.Fatalf("after choosing version 1 as the default: current %d, next %d, want 1 and 4", setDefault.CurrentVersion, setDefault.NextVersion)
	}

	if _, err := b.tests.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: versionRequest(t, b, draft.ID, 3)}); err != nil {
		t.Fatalf("delete the newest version, which is not the default: %v", err)
	}
	if got := nextVersionRead(t, b, draft.ID); got != 4 {
		t.Fatalf("after deleting version 3 the test reports next version %d, want 4: the highest listed version + 1 is 3", got)
	}
	if got := nextVersionListed(t, b, title); got != 4 {
		t.Fatalf("the list reports next version %d, want 4", got)
	}
	published, err := b.publish(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.Version != 4 {
		t.Fatalf("publish assigned %d, the test had reported 4", published.Version)
	}

	if _, err := b.tests.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: versionRequest(t, b, draft.ID, 2)}); err != nil {
		t.Fatal(err)
	}
	if got := nextVersionRead(t, b, draft.ID); got != 5 {
		t.Fatalf("after deleting version 2 the test reports next version %d, want 5", got)
	}
}
