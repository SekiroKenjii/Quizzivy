package questions_test

import (
	"errors"
	"testing"

	"quizzivy/internal/questions"
)

// The cross-field rules QuestionInput's description calls out as the ones "a
// single schema cannot express". Each exists because the alternative is
// discovering it mid-test, as a student.

func audio() *string        { k := "audio"; return &k }
func image() *string        { k := "image"; return &k }
func id() *string           { s := "01a04900-0000-7000-8000-000000000001"; return &s }
func text(s string) *string { return &s }

// fields returns the field names a validation failure named, so a test can
// assert WHICH rule fired rather than merely that something did.
func fields(t *testing.T, err error) []string {
	t.Helper()
	var invalid *questions.ValidationError
	if !errors.As(err, &invalid) {
		return nil
	}
	out := make([]string, len(invalid.Fields))
	for i, f := range invalid.Fields {
		out[i] = f.Field
	}
	return out
}

func hasField(t *testing.T, err error, want string) bool {
	t.Helper()
	for _, f := range fields(t, err) {
		if f == want {
			return true
		}
	}
	return false
}

func choice(opts ...questions.OptionInput) questions.Input {
	return questions.Input{
		Type: questions.SingleChoice, Prompt: "Chọn đáp án đúng", Points: "1.00", Options: opts,
	}
}

func TestAChoiceQuestionNeedsACorrectOption(t *testing.T) {
	// The rule that makes a question gradeable at all. Without it the bank
	// accepts a question no answer can score on.
	err := choice(
		questions.OptionInput{Text: "A", IsCorrect: false},
		questions.OptionInput{Text: "B", IsCorrect: false},
	).Validate(nil)
	if !hasField(t, err, "options") {
		t.Errorf("a choice question with no correct option was accepted: %v", err)
	}
}

func TestSingleChoiceRejectsTwoCorrectOptions(t *testing.T) {
	err := choice(
		questions.OptionInput{Text: "A", IsCorrect: true},
		questions.OptionInput{Text: "B", IsCorrect: true},
	).Validate(nil)
	if !hasField(t, err, "options") {
		t.Errorf("single_choice accepted two correct options: %v", err)
	}
}

func TestAValidChoiceQuestionPasses(t *testing.T) {
	err := choice(
		questions.OptionInput{Text: "A", IsCorrect: true},
		questions.OptionInput{Text: "B", IsCorrect: false},
	).Validate(nil)
	if err != nil {
		t.Errorf("a valid question was rejected: %v", err)
	}
}

// §7's placeholders and the blank ordinals must be the same SET. A blank with
// no placeholder is unreachable; a placeholder with no blank renders as literal
// `{{2}}` to the student and can never be answered.
func TestFillBlankPlaceholdersMustMatchTheBlanks(t *testing.T) {
	cases := []struct {
		name    string
		prompt  string
		blanks  []int
		wantBad bool
	}{
		{"matching", "Điền {{1}} và {{2}}", []int{1, 2}, false},
		{"blank with no placeholder", "Điền {{1}}", []int{1, 2}, true},
		{"placeholder with no blank", "Điền {{1}} và {{2}}", []int{1}, true},
		{"out of order but matching", "Điền {{2}} rồi {{1}}", []int{1, 2}, false},
		{"repeated placeholder counts once", "Điền {{1}} và lại {{1}}", []int{1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := questions.Input{
				Type: questions.FillBlank, Prompt: tc.prompt, Points: "1.00",
			}
			for _, n := range tc.blanks {
				in.Blanks = append(in.Blanks, questions.BlankInput{
					Ordinal: n, AcceptedAnswers: []string{"x"},
				})
			}
			err := in.Validate(nil)
			if tc.wantBad && err == nil {
				t.Error("accepted a prompt and blanks that do not correspond")
			}
			if !tc.wantBad && err != nil {
				t.Errorf("rejected a valid fill_blank: %v", err)
			}
		})
	}
}

func TestPromptPlaceholdersIgnoresWhatIsNotOne(t *testing.T) {
	// `{{0}}` is not a placeholder this system defines -- blanks are 1-indexed
	// -- so it is prose that happens to look like one, not an error.
	got := questions.PromptPlaceholders("a {{1}} b {{0}} c {{x}} d {{12}}")
	want := []int{1, 12}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

// [D-04] the biconditional, in application terms. The database enforces it too;
// this is what produces the Vietnamese message instead of a 500.
func TestAudioPolicyRequiresAnAudioAsset(t *testing.T) {
	policy := &questions.AudioPolicy{AllowSeek: false, ShowTranscriptAfterSubmit: true}

	t.Run("policy on an image is rejected", func(t *testing.T) {
		in := questions.Input{
			Type: questions.ShortAnswer, Prompt: "Xem ảnh", Points: "1.00",
			MediaAssetID: id(), Audio: policy,
		}
		if err := in.Validate(image()); !hasField(t, err, "audio") {
			t.Errorf("an image with an audio policy was accepted: %v", err)
		}
	})

	t.Run("audio without a policy is rejected", func(t *testing.T) {
		in := questions.Input{
			Type: questions.ShortAnswer, Prompt: "Nghe", Points: "1.00", MediaAssetID: id(),
		}
		if err := in.Validate(audio()); !hasField(t, err, "audio") {
			t.Errorf("an audio asset with no policy was accepted: %v", err)
		}
	})

	t.Run("audio with a policy passes", func(t *testing.T) {
		in := questions.Input{
			Type: questions.ShortAnswer, Prompt: "Nghe", Points: "1.00",
			MediaAssetID: id(), Audio: policy,
		}
		if err := in.Validate(audio()); err != nil {
			t.Errorf("a valid audio question was rejected: %v", err)
		}
	})

	t.Run("no media and no policy passes", func(t *testing.T) {
		in := questions.Input{Type: questions.ShortAnswer, Prompt: "Viết", Points: "1.00"}
		if err := in.Validate(nil); err != nil {
			t.Errorf("a plain question was rejected: %v", err)
		}
	})
}

func TestTranscriptRequiresAudio(t *testing.T) {
	in := questions.Input{
		Type: questions.ShortAnswer, Prompt: "Viết", Points: "1.00",
		Transcript: text("Hello there."),
	}
	if err := in.Validate(nil); !hasField(t, err, "transcript") {
		t.Errorf("a transcript with no audio was accepted: %v", err)
	}
}

func TestSampleAnswerIsShortAnswerOnly(t *testing.T) {
	// §7 marks it short_answer only, and admin-only at read time.
	in := choice(
		questions.OptionInput{Text: "A", IsCorrect: true},
		questions.OptionInput{Text: "B", IsCorrect: false},
	)
	in.SampleAnswer = text("Đáp án mẫu")
	if err := in.Validate(nil); !hasField(t, err, "sampleAnswer") {
		t.Errorf("sampleAnswer on single_choice was accepted: %v", err)
	}
}

// Every failure at once, not the first. Fixing a form should not be a series of
// round trips.
func TestValidationReportsEveryFieldAtOnce(t *testing.T) {
	in := questions.Input{
		Type: questions.SingleChoice, Prompt: "", Points: "1.00",
		SampleAnswer: text("nope"),
		Options: []questions.OptionInput{
			{Text: "A", IsCorrect: false},
			{Text: "B", IsCorrect: false},
		},
	}
	got := fields(t, in.Validate(nil))
	for _, want := range []string{"prompt", "options", "sampleAnswer"} {
		found := false
		for _, f := range got {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("field %q missing from %v; the client would fix one problem per round trip", want, got)
		}
	}
}
