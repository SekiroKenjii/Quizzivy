package domain

// DraftContent is the whole outline, resolved against the bank, ready to validate and
// freeze.
type DraftContent struct {
	TestID   string
	Sections []DraftSection
}

// DraftSection is one part of the outline with its questions in order.
type DraftSection struct {
	ID           string
	Ordinal      int
	Title        string
	Instructions *string
	Questions    []DraftQuestion
}

// DraftQuestion is one bank question resolved for the snapshot, in the position the
// outline gives it.
type DraftQuestion struct {
	SourceID       string
	Ordinal        int
	Type           string
	Prompt         string
	MediaAssetID   *string
	MediaAssetKind *string
	MaxPlays       *int
	AllowSeek      *bool
	ShowTranscript *bool
	Transcript     *string
	Points         string
	Explanation    *string
	SampleAnswer   *string
	Options        []DraftOption
	Blanks         []DraftBlank
}

// DraftOption is a choice option as the draft holds it.
type DraftOption struct {
	Ordinal   int
	Text      string
	IsCorrect bool
}

// DraftBlank is a fill_blank slot with its accepted answers.
type DraftBlank struct {
	Ordinal         int
	CaseSensitive   bool
	AcceptedAnswers []string
}

// isChoice reports whether the type's answer is a set of options.
func isChoice(questionType string) bool {
	switch questionType {
	case "single_choice", "multiple_choice", "true_false":
		return true
	}
	return false
}

// PreviewQuestion is one question as a student receives it.
//
// It carries no is_correct, no accepted answer, no sample answer and no
// transcript. That is not a projection applied on the way out -- those columns
// are never selected, so nothing downstream can leak one by forgetting to strip
// it (§14 E2E 9).
type PreviewQuestion struct {
	ID           string
	Type         string
	Prompt       string
	Points       string
	MediaAssetID *string
	MaxPlays     *int
	AllowSeek    *bool
	ShowScript   *bool
	Options      []PreviewOption
	Blanks       []PreviewBlank
}

type PreviewOption struct {
	ID   string
	Text string
}

type PreviewBlank struct {
	ID      string
	Ordinal int
}
