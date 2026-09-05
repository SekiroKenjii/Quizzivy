// Package domain is the question bank: the Question aggregate with its Options
// and Blanks, its Type and AudioPolicy value objects, the Input command that
// validates itself, and QuestionManager for the prompt's own rules.
package domain

import (
	"time"
)

// Question is a bank row with its children. Media is the id and kind pair the
// composite FK needs; the handler resolves the asset itself.
type Question struct {
	ID             string
	Type           Type
	Prompt         string
	MediaAssetID   *string
	MediaAssetKind *string
	Audio          *AudioPolicy
	Transcript     *string
	Options        []Option
	Blanks         []Blank
	Points         string
	Explanation    *string
	SampleAnswer   *string
	Tags           []string
	// Draft outlines referencing this question.
	UsedInTests int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Option struct {
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

type Blank struct {
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
)

// AudioPolicy is present if and only if the attached asset is audio.
type AudioPolicy struct {
	// MaxPlays nil means unlimited.
	MaxPlays                  *int
	AllowSeek                 bool
	ShowTranscriptAfterSubmit bool
}

// TypeFacets is how many bank questions each type holds for one search.
type TypeFacets struct {
	All    int
	ByType map[Type]int
}

// isChoice reports whether the answer is a set of options.
func (t Type) IsChoice() bool {
	return t == SingleChoice || t == MultipleChoice || t == TrueFalse
}

func (t Type) valid() bool {
	switch t {
	case SingleChoice, MultipleChoice, TrueFalse, FillBlank, ShortAnswer:
		return true
	}
	return false
}
