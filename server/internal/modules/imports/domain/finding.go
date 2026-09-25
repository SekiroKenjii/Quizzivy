package domain

// Severity orders what a teacher must do: fix a blocker, decide a review item, or merely read information.
type Severity string

const (
	Blocking       Severity = "blocking"
	ReviewRequired Severity = "review_required"
	Informational  Severity = "informational"
)

const (
	CodeMissingAnswer          = "MISSING_ANSWER"
	CodeConflictingKeys        = "CONFLICTING_ANSWER_KEYS"
	CodeUnusableKey            = "UNUSABLE_ANSWER_KEY"
	CodeUnmatchedKey           = "UNMATCHED_ANSWER_KEY"
	CodeKeyPaperAmbiguous      = "AMBIGUOUS_KEY_PAPER"
	CodeUnsupportedInteraction = "UNSUPPORTED_INTERACTION"
	CodeInvalidQuestion        = "INVALID_QUESTION"
	CodeInvalidGroup           = "INVALID_GROUP"
	CodeUnassignedText         = "UNASSIGNED_SOURCE_TEXT"
	CodeSourceObject           = "UNSUPPORTED_DOCUMENT_OBJECT"
	CodeMultiplePapers         = "MULTIPLE_PAPERS_IN_SOURCE"
	CodeMissingUnderline       = "UNDERLINE_MARKS_MISSING"
	CodeIrregularOptions       = "IRREGULAR_OPTION_COUNT"
	CodeScoringDefaulted       = "SCORING_DEFAULTED"
	CodeFormattingSimplified   = "FORMATTING_SIMPLIFIED"
	CodeEmptySection           = "EMPTY_SECTION"
	CodeNumberingIrregular     = "NUMBERING_IRREGULAR"
	CodeColoredText            = "COLORED_TEXT_IN_CONTENT"
	CodeOptionReference        = "OPTION_LABEL_REFERENCE"
	CodeNoQuestions            = "NO_QUESTIONS"
)

// Finding is one actionable observation; Count aggregates repeats so one cause yields one finding.
type Finding struct {
	ID           string      `json:"id"`
	Code         string      `json:"code"`
	Severity     Severity    `json:"severity"`
	Target       string      `json:"target,omitempty"`
	Field        string      `json:"field,omitempty"`
	Count        int         `json:"count"`
	Acknowledged bool        `json:"acknowledged,omitempty"`
	Evidence     []SourceRef `json:"evidence"`
}
