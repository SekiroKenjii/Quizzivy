package publish

// Option is a choice option as the draft holds it.
type Option struct {
	Ordinal   int
	Text      string
	IsCorrect bool
}

// Blank is a fill_blank slot with its accepted answers.
type Blank struct {
	Ordinal         int
	CaseSensitive   bool
	AcceptedAnswers []string
}

// Question is one bank question resolved for the snapshot, in the position the
// outline gives it.
type Question struct {
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
	Options        []Option
	Blanks         []Blank
}

// Section is one part of the outline with its questions in order.
type Section struct {
	ID           string
	Ordinal      int
	Title        string
	Instructions *string
	Questions    []Question
}

// Draft is the whole outline, resolved against the bank, ready to validate and
// freeze.
type Draft struct {
	TestID   string
	Sections []Section
}

// isChoice reports whether the type's answer is a set of options.
func isChoice(questionType string) bool {
	switch questionType {
	case "single_choice", "multiple_choice", "true_false":
		return true
	}
	return false
}
