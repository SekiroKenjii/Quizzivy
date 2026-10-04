package domain_test

import (
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/tests/domain"
)

func missingMediaDraft(firstAsset string) domain.DraftContent {
	return domain.DraftContent{
		TestID: groupID(20),
		Sections: []domain.DraftSection{{
			ID: groupID(21), Ordinal: 0, Title: "Phần 1",
			Questions: []domain.DraftQuestion{
				{
					SourceID: groupID(22), Ordinal: 0, Type: "short_answer", Prompt: "Nghe tệp A", Points: "1.00",
					MediaAssetID: groupPointer(firstAsset), MediaAssetKind: groupPointer("audio"),
				},
				{
					SourceID: groupID(23), Ordinal: 1, Type: "short_answer", Prompt: "Xem hình B", Points: "1.00",
					MediaAssetID: groupPointer(groupID(26)), MediaAssetKind: groupPointer("image"),
				},
				{
					SourceID: groupID(24), Ordinal: 2, Type: "short_answer", Prompt: "Nghe tệp C", Points: "1.00",
					MediaAssetID: groupPointer(groupID(27)), MediaAssetKind: groupPointer("audio"),
				},
			},
			Groups: []domain.GroupBundle{groupFixture()},
		}},
	}
}

func TestMissingMediaNamesEachQuestionAndGroupThatUsesAGoneAsset(t *testing.T) {
	draft := missingMediaDraft(groupID(25))

	err := domain.Publishing.MissingMedia(draft, []string{groupID(25), groupID(26), groupID(7)})

	var invalid *domain.PublishValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("MissingMedia returned %v, want a PublishValidationError", err)
	}
	want := []domain.Violation{
		{Rule: domain.AudioQuestionHasAsset, SectionID: groupID(21), QuestionID: groupID(22)},
		{Rule: domain.QuestionValid, SectionID: groupID(21), QuestionID: groupID(23)},
		{Rule: domain.GroupValid, SectionID: groupID(21), GroupID: groupID(1)},
	}
	if len(invalid.Violations) != len(want) {
		t.Fatalf("got %d violations, want %d: %+v", len(invalid.Violations), len(want), invalid.Violations)
	}
	for i, got := range invalid.Violations {
		if got.Message == "" {
			t.Errorf("violation %d carries no message", i)
		}
		got.Message = ""
		if got != want[i] {
			t.Errorf("violation %d is %+v, want %+v", i, got, want[i])
		}
	}
}

func TestMissingMediaIsNilWhenTheDraftNamesNoneOfThem(t *testing.T) {
	draft := missingMediaDraft(groupID(25))

	for _, missing := range [][]string{{groupID(99)}, nil} {
		if err := domain.Publishing.MissingMedia(draft, missing); err != nil {
			t.Errorf("MissingMedia(%v) returned %v, want nil", missing, err)
		}
	}
}

func TestMissingMediaComparesIdsWithoutCase(t *testing.T) {
	draft := missingMediaDraft(strings.ToUpper(groupID(25)))

	err := domain.Publishing.MissingMedia(draft, []string{groupID(25)})

	var invalid *domain.PublishValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("MissingMedia returned %v, want a PublishValidationError", err)
	}
	if len(invalid.Violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(invalid.Violations), invalid.Violations)
	}
	if got := invalid.Violations[0]; got.Rule != domain.AudioQuestionHasAsset || got.QuestionID != groupID(22) {
		t.Errorf("violation is %+v, want %s on %s", got, domain.AudioQuestionHasAsset, groupID(22))
	}
}
