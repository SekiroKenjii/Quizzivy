package domain

import "encoding/json"

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
	Groups       []GroupBundle
	Units        []DraftUnit
}

// DraftQuestion is one bank question resolved for the snapshot, in the position the
// outline gives it.
type DraftQuestion struct {
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	SourceID           string
	Ordinal            int
	Type               string
	Prompt             string
	MediaAssetID       *string
	MediaAssetKind     *string
	MaxPlays           *int
	AllowSeek          *bool
	ShowTranscript     *bool
	Transcript         *string
	Points             string
	Explanation        *string
	SampleAnswer       *string
	Options            []DraftOption
	Blanks             []DraftBlank
}

// DraftOption is a choice option as the draft holds it.
type DraftOption struct {
	Content   json.RawMessage
	Ordinal   int
	Text      string
	IsCorrect bool
}

// DraftBlank is a fill_blank slot with its accepted answers.
type DraftBlank struct {
	GapID           *string
	Ordinal         int
	CaseSensitive   bool
	AcceptedAnswers []string
}

// PreviewQuestion is one question as a student receives it.
type PreviewQuestion struct {
	PromptContent json.RawMessage
	ID            string
	SectionID     string
	Type          string
	Prompt        string
	Points        string
	MediaAssetID  *string
	MaxPlays      *int
	AllowSeek     *bool
	ShowScript    *bool
	Options       []PreviewOption
	Blanks        []PreviewBlank
}

type PreviewOption struct {
	Content json.RawMessage
	ID      string
	Text    string
}

type PreviewBlank struct {
	GapID         *string
	ID            string
	Ordinal       int
	CaseSensitive bool
}

// DraftUnit is one ordered standalone question or independent group within a section.
type DraftUnit struct {
	QuestionID string
	GroupID    string
}
