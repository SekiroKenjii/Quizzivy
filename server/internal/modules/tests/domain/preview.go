package domain

import (
	"errors"
)

// ErrNotPublished is returned when a test has no version to render.
var ErrNotPublished = errors.New("tests: no published version")

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
