// Package domain is the question bank: the Question aggregate with its Options
// and Blanks, its Type and AudioPolicy value objects, the Input command that
// validates itself, and QuestionManager for the prompt's own rules.
package domain

import (
	"encoding/json"
	"time"
)

// Question is a bank row with its children. Media is the id and kind pair the
// composite FK needs; the handler resolves the asset itself.
type Question struct {
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	ID                 string
	Type               Type
	Level              *Level
	Skill              *Skill
	Prompt             string
	MediaAssetID       *string
	MediaAssetKind     *string
	MediaAlt           *string
	Audio              *AudioPolicy
	Transcript         *string
	Options            []Option
	Blanks             []Blank
	Points             string
	Explanation        *string
	SampleAnswer       *string
	// SelectCount is a multiple choice's "choose exactly N", MaxWords a short
	// answer's word limit and MatchAllowReuse whether a matching question's
	// choices may answer more than one item; each is nil on every other type.
	SelectCount     *int
	MaxWords        *int
	MatchAllowReuse *bool
	MatchItems      []MatchItem
	Rubric          []RubricCriterion
	Tags            []string
	// Draft outlines referencing this question.
	UsedInTests int
	UsedIn      []TestRef
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Option struct {
	Content   json.RawMessage
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

// MatchItem is one of a matching question's items, keyed to one of its
// options. CorrectOption is that option's index in Options.
type MatchItem struct {
	ID              string
	Ordinal         int
	Text            string
	CorrectOptionID string
	CorrectOption   int
}

// RubricCriterion is one of an essay's rubric criteria.
type RubricCriterion struct {
	ID      string
	Ordinal int
	Label   string
}

type Blank struct {
	GapID           *string
	ID              string
	Ordinal         int
	AcceptedAnswers []string
	CaseSensitive   bool
}

type Type string

const (
	SingleChoice   Type = "single_choice"
	MultipleChoice Type = "multiple_choice"
	TrueFalse      Type = "true_false"
	FillBlank      Type = "fill_blank"
	ShortAnswer    Type = "short_answer"
	TFNG           Type = "tfng"
	Matching       Type = "matching"
	Essay          Type = "essay"
)

// TFNGTexts are a tfng question's three option texts, in their fixed order.
// The server writes them whatever a client sent; a reader labels the options
// by position in its own language.
var TFNGTexts = [3]string{"True", "False", "Not given"}

// Limits the bank holds a question's parts to (T-R6.3). A choice type's cap of
// eight options is ValidateAuthoring's.
const (
	MaxMatchChoices    = 12
	MinMatchItems      = 2
	MaxMatchItems      = 20
	MaxMatchItemLength = 500
	MaxRubricCriteria  = 6
	MaxRubricLabel     = 80
	MaxSelectCount     = 7
	MaxWordsLimit      = 100
)

// AudioPolicy is present if and only if the attached asset is audio.
type AudioPolicy struct {
	// MaxPlays nil means unlimited.
	MaxPlays                  *int
	AllowSeek                 bool
	ShowTranscriptAfterSubmit bool
	// AllowPause nil means the writer did not say: a create stores true and an
	// update keeps the stored value. A policy read from storage always has it.
	AllowPause *bool
}

// PauseAllowed reports whether a student may pause the recording. A policy
// that does not say allows it, as every recording did before the control.
func (p AudioPolicy) PauseAllowed() bool { return p.AllowPause == nil || *p.AllowPause }

// TypeFacets is how many bank questions each type holds for one search.
type TypeFacets struct {
	All     int
	ByType  map[Type]int
	ByLevel map[Level]int
	BySkill map[Skill]int
}

// isChoice reports whether the answer is a set of options.
func (t Type) IsChoice() bool {
	return t == SingleChoice || t == MultipleChoice || t == TrueFalse
}

// HasOptions reports whether the question stores options: the choice types,
// whose answer is a set of them, and matching, whose options are the choices
// its items are keyed to.
func (t Type) HasOptions() bool {
	return t.IsChoice() || t == TFNG || t == Matching
}

func (t Type) valid() bool {
	switch t {
	case SingleChoice, MultipleChoice, TrueFalse, FillBlank, ShortAnswer, TFNG, Matching, Essay:
		return true
	}
	return false
}
