package application_test

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"testing"
)

type countingRepo struct {
	domain.Repository
	test    domain.Test
	papers  domain.DiffPapers
	diffErr error
	diffs   []domain.DiffRequest
}

func (r *countingRepo) Get(context.Context, access.Scope, string) (domain.Test, error) {
	return r.test, nil
}

func (r *countingRepo) DiffPapers(_ context.Context, req domain.DiffRequest) (domain.DiffPapers, error) {
	r.diffs = append(r.diffs, req)
	return r.papers, r.diffErr
}

func paperWith(prompts ...string) domain.DiffPaper {
	section := domain.DraftSection{Title: "Phần 1"}
	for i, prompt := range prompts {
		section.Questions = append(section.Questions, domain.DraftQuestion{
			SourceID: fmt.Sprintf("q%d", i), Type: "short_answer", Prompt: prompt, Points: "1.00",
		})
	}
	return domain.DiffPaper{Content: domain.DraftContent{Sections: []domain.DraftSection{section}}}
}

func TestGetCountsTheChangesBetweenTheLatestVersionAndTheDraft(t *testing.T) {
	from, to := paperWith("Một", "Hai"), paperWith("Một khác", "Hai", "Ba")
	repo := &countingRepo{test: domain.Test{ID: "t1", CurrentVersion: 2}, papers: domain.DiffPapers{From: &from, To: to}}
	got, err := application.New(repo).Queries.Get.Handle(context.Background(), query.Get{ID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.UnpublishedChanges == nil || *got.UnpublishedChanges != 3 {
		t.Fatalf("unpublishedChanges = %v, want 3: one added, one changed, the total", got.UnpublishedChanges)
	}
	if len(repo.diffs) != 1 || repo.diffs[0].Version != 0 || repo.diffs[0].Against.Kind != domain.AgainstDraft {
		t.Errorf("diff requests = %+v, want one: the latest version against the draft", repo.diffs)
	}
}

func TestGetLeavesTheCountNullWhereItCannotBeCounted(t *testing.T) {
	cases := []struct {
		name    string
		version int
		err     error
		ran     bool
	}{
		{"a test never published", 0, nil, false},
		{"no version found", 1, domain.ErrNotPublished, true},
		{"a draft whose groups are refused", 1, fmt.Errorf("%w: %w", domain.ErrDraftUnreadable, &domain.GroupError{Rule: "group_membership"}), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &countingRepo{test: domain.Test{ID: "t1", CurrentVersion: c.version}, diffErr: c.err}
			got, err := application.New(repo).Queries.Get.Handle(context.Background(), query.Get{ID: "t1"})
			if err != nil {
				t.Fatalf("Get failed for a count that cannot be made: %v", err)
			}
			if got.UnpublishedChanges != nil {
				t.Errorf("unpublishedChanges = %d, want null", *got.UnpublishedChanges)
			}
			if ran := len(repo.diffs) > 0; ran != c.ran {
				t.Errorf("the diff ran = %v, want %v", ran, c.ran)
			}
		})
	}
}

func TestGetFailsWhenTheCountFailsForAnyOtherReason(t *testing.T) {
	broken := errors.New("connection reset")
	repo := &countingRepo{test: domain.Test{ID: "t1", CurrentVersion: 1}, diffErr: broken}
	_, err := application.New(repo).Queries.Get.Handle(context.Background(), query.Get{ID: "t1"})
	if !errors.Is(err, broken) {
		t.Fatalf("Get = %v, want the database failure, not a null count", err)
	}
}

func TestADiffNamesTheTwoSidesAndCountsFromNothingForAFirstVersion(t *testing.T) {
	to := paperWith("Một", "Hai")
	to.Side = domain.DiffSide{Version: 1}
	repo := &countingRepo{papers: domain.DiffPapers{To: to}}
	result, err := application.New(repo).Queries.Diff.Handle(context.Background(), query.Diff{TestID: "t1", Version: 1, Against: domain.Against{Kind: domain.AgainstPrevious}})
	if err != nil {
		t.Fatal(err)
	}
	if result.From != nil || result.To.Version != 1 {
		t.Errorf("sides = %+v and %+v, want nothing before version 1", result.From, result.To)
	}
	if len(result.Changes) != 2 || result.Changes[0].Kind != domain.ChangeAdded {
		t.Errorf("changes = %+v, want two added questions and no total", result.Changes)
	}
}
