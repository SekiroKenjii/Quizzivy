package domain

import (
	"encoding/json"
	questions "quizzivy/internal/modules/questions/domain"
)

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
// outline gives it. SourceID is the bank question; FrozenID is set only on a
// question read back from a version, and is the id of its frozen row.
type DraftQuestion struct {
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	SourceID           string
	FrozenID           string
	Ordinal            int
	Type               string
	Level              *questions.Level
	Skill              *questions.Skill
	Prompt             string
	MediaAssetID       *string
	MediaAssetKind     *string
	MediaAlt           *string
	MaxPlays           *int
	AllowSeek          *bool
	ShowTranscript     *bool
	AllowPause         *bool
	Transcript         *string
	Points             string
	Explanation        *string
	SampleAnswer       *string
	SelectCount        *int
	MaxWords           *int
	MatchAllowReuse    *bool
	Options            []DraftOption
	Blanks             []DraftBlank
	MatchItems         []DraftMatchItem
	Rubric             []DraftCriterion
}

// PauseAllowed reports whether a student may pause the question's audio. An
// unset value allows it, as every question did before the control existed.
func (q DraftQuestion) PauseAllowed() bool { return q.AllowPause == nil || *q.AllowPause }

// DraftOption is a choice option as the draft holds it.
type DraftOption struct {
	Content   json.RawMessage
	Ordinal   int
	Text      string
	IsCorrect bool
}

// DraftMatchItem is a matching item as the draft holds it. CorrectOption is
// the index, in the question's Options in ordinal order, of the choice it is
// keyed to, or -1 when it names none of them.
type DraftMatchItem struct {
	Ordinal       int
	Text          string
	CorrectOption int
}

// DraftCriterion is one of an essay's rubric criteria as the draft holds it.
type DraftCriterion struct {
	Ordinal int
	Label   string
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
	MediaAlt      *string
	MaxPlays      *int
	AllowSeek     *bool
	ShowScript    *bool
	AllowPause    bool
	SelectCount   *int
	MaxWords      *int
	MatchReuse    *bool
	Options       []PreviewOption
	Blanks        []PreviewBlank
	MatchItems    []PreviewMatchItem
}

// PreviewOption is a choice as a student receives it. Key is a matching
// choice's key and empty on every other type.
type PreviewOption struct {
	Content json.RawMessage
	ID      string
	Key     string
	Text    string
}

// PreviewMatchItem is a matching item as a student receives it: its label
// and text, never the choice it is keyed to.
type PreviewMatchItem struct {
	ID   string
	Key  string
	Text string
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
