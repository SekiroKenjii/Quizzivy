package publish_test

import (
	"errors"
	"testing"

	"quizzivy/internal/tests/publish"
)

// Validate is a pure function over a resolved Draft, so the rules are tested as
// one. Two of them -- non-positive points, and an audio policy with no asset --
// are unreachable through the bank, whose CHECK constraints forbid both. That
// is the point of re-validating at publish: it is the guard for a code path
// that skipped those constraints, and a test that has to disable triggers to
// reach it is testing the database rather than the rule.
func draftWith(q publish.Question) publish.Draft {
	return publish.Draft{
		TestID: "00000000-0000-7000-8000-000000000001",
		Sections: []publish.Section{{
			ID: "00000000-0000-7000-8000-000000000002", Ordinal: 0, Title: "Phần 1",
			Questions: []publish.Question{q},
		}},
	}
}

func base() publish.Question {
	return publish.Question{
		SourceID: "00000000-0000-7000-8000-000000000003",
		Ordinal:  0, Type: "short_answer", Prompt: "Câu hỏi", Points: "1.00",
	}
}

func onlyViolation(t *testing.T, d publish.Draft) publish.Violation {
	t.Helper()
	err := publish.Validate(d)
	var invalid *publish.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate returned %v, want a ValidationError", err)
	}
	if len(invalid.Violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(invalid.Violations), invalid.Violations)
	}
	return invalid.Violations[0]
}

func TestRulePointsMustBePositive(t *testing.T) {
	for _, points := range []string{"0.00", "-1.00", "not a number"} {
		q := base()
		q.Points = points
		v := onlyViolation(t, draftWith(q))
		if v.Rule != publish.PointsPositive {
			t.Errorf("points %q gave rule %s, want %s", points, v.Rule, publish.PointsPositive)
		}
		if v.QuestionID != q.SourceID {
			t.Errorf("violation names %q, want %q", v.QuestionID, q.SourceID)
		}
	}
}

func TestRuleAudioPolicyNeedsAnAudioAsset(t *testing.T) {
	allow, show := false, true

	t.Run("policy with no asset at all", func(t *testing.T) {
		q := base()
		q.AllowSeek, q.ShowTranscript = &allow, &show
		v := onlyViolation(t, draftWith(q))
		if v.Rule != publish.AudioQuestionHasAsset {
			t.Errorf("got rule %s, want %s", v.Rule, publish.AudioQuestionHasAsset)
		}
		if v.QuestionID != q.SourceID {
			t.Errorf("violation names %q, want %q", v.QuestionID, q.SourceID)
		}
	})

	t.Run("policy on an image asset", func(t *testing.T) {
		id, kind := "00000000-0000-7000-8000-0000000000aa", "image"
		q := base()
		q.AllowSeek, q.ShowTranscript = &allow, &show
		q.MediaAssetID, q.MediaAssetKind = &id, &kind
		if v := onlyViolation(t, draftWith(q)); v.Rule != publish.AudioQuestionHasAsset {
			t.Errorf("got rule %s, want %s", v.Rule, publish.AudioQuestionHasAsset)
		}
	})

	t.Run("policy on an audio asset passes", func(t *testing.T) {
		id, kind := "00000000-0000-7000-8000-0000000000aa", "audio"
		q := base()
		q.AllowSeek, q.ShowTranscript = &allow, &show
		q.MediaAssetID, q.MediaAssetKind = &id, &kind
		if err := publish.Validate(draftWith(q)); err != nil {
			t.Errorf("a valid audio question was rejected: %v", err)
		}
	})
}

func TestRuleChoiceNeedsACorrectOption(t *testing.T) {
	for _, questionType := range []string{"single_choice", "multiple_choice", "true_false"} {
		q := base()
		q.Type = questionType
		q.Options = []publish.Option{{Ordinal: 0, Text: "A"}, {Ordinal: 1, Text: "B"}}
		if v := onlyViolation(t, draftWith(q)); v.Rule != publish.ChoiceHasCorrectOption {
			t.Errorf("%s gave rule %s", questionType, v.Rule)
		}

		q.Options[1].IsCorrect = true
		if err := publish.Validate(draftWith(q)); err != nil {
			t.Errorf("%s with a correct option was rejected: %v", questionType, err)
		}
	}
}

func TestRuleBlankNeedsAnAcceptedAnswer(t *testing.T) {
	q := base()
	q.Type = "fill_blank"
	q.Prompt = "Điền {{1}}"
	q.Blanks = []publish.Blank{{Ordinal: 1}}

	if v := onlyViolation(t, draftWith(q)); v.Rule != publish.BlankHasAcceptedAnswer {
		t.Errorf("got rule %s, want %s", v.Rule, publish.BlankHasAcceptedAnswer)
	}

	q.Blanks[0].AcceptedAnswers = []string{"đi"}
	if err := publish.Validate(draftWith(q)); err != nil {
		t.Errorf("a blank with an answer was rejected: %v", err)
	}
}

func TestRulePlaceholdersMustMatchTheBlanks(t *testing.T) {
	cases := []struct {
		name    string
		prompt  string
		blanks  []int
		wantBad bool
	}{
		{"matching", "Điền {{1}} và {{2}}", []int{1, 2}, false},
		{"placeholder with no blank", "Điền {{1}} và {{2}}", []int{1}, true},
		{"blank with no placeholder", "Điền {{1}}", []int{1, 2}, true},
		{"repeated placeholder counts once", "Điền {{1}} rồi {{1}}", []int{1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := base()
			q.Type = "fill_blank"
			q.Prompt = tc.prompt
			for _, n := range tc.blanks {
				q.Blanks = append(q.Blanks, publish.Blank{Ordinal: n, AcceptedAnswers: []string{"x"}})
			}

			err := publish.Validate(draftWith(q))
			if !tc.wantBad {
				if err != nil {
					t.Errorf("rejected a valid fill_blank: %v", err)
				}
				return
			}
			var invalid *publish.ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("got %v, want a ValidationError", err)
			}
			found := false
			for _, v := range invalid.Violations {
				if v.Rule == publish.BlankPlaceholdersMatch && v.QuestionID == q.SourceID {
					found = true
				}
			}
			if !found {
				t.Errorf("no anchored %s violation in %+v", publish.BlankPlaceholdersMatch, invalid.Violations)
			}
		})
	}
}

func TestRuleSectionMustNotBeEmpty(t *testing.T) {
	d := publish.Draft{
		TestID: "00000000-0000-7000-8000-000000000001",
		Sections: []publish.Section{
			{ID: "00000000-0000-7000-8000-000000000002", Title: "Phần rỗng"},
		},
	}
	v := onlyViolation(t, d)
	if v.Rule != publish.SectionNotEmpty {
		t.Errorf("got rule %s, want %s", v.Rule, publish.SectionNotEmpty)
	}
	if v.SectionID != d.Sections[0].ID {
		t.Errorf("violation names section %q, want %q", v.SectionID, d.Sections[0].ID)
	}
	if v.QuestionID != "" {
		t.Errorf("an empty section named question %q; there is none", v.QuestionID)
	}
}

// An empty section reports once, rather than also reporting every rule for the
// questions it does not have.
func TestAnEmptySectionDoesNotCascade(t *testing.T) {
	d := publish.Draft{
		TestID: "00000000-0000-7000-8000-000000000001",
		Sections: []publish.Section{
			{ID: "00000000-0000-7000-8000-000000000002", Title: "Rỗng"},
			{ID: "00000000-0000-7000-8000-000000000004", Title: "Cũng rỗng"},
		},
	}
	err := publish.Validate(d)
	var invalid *publish.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatal(err)
	}
	if len(invalid.Violations) != 2 {
		t.Errorf("got %d violations for two empty sections, want 2: %+v",
			len(invalid.Violations), invalid.Violations)
	}
}

func TestAValidDraftPasses(t *testing.T) {
	q := base()
	q.Type = "single_choice"
	q.Options = []publish.Option{{Ordinal: 0, Text: "A", IsCorrect: true}, {Ordinal: 1, Text: "B"}}
	if err := publish.Validate(draftWith(q)); err != nil {
		t.Errorf("a valid draft was rejected: %v", err)
	}
}
