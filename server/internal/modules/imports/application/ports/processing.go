package ports

import (
	"context"
	"io"
	"quizzivy/internal/modules/imports/domain"
)

// DocumentBody is disk-backed, seekable input whose size and checksum have already been verified.
type DocumentBody interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

// DocumentInput separates original-source ownership from the immutable native/normalized identity used by extraction coordinates.
type DocumentInput struct {
	OriginalID, Identity, Role, Format string
	Body                               DocumentBody
	Bytes                              int64
}

// StageOutput owns transient files until Close; the plan and fresh readers must describe the same immutable bytes.
type StageOutput struct {
	Plan  domain.ArtifactPlan
	Open  func(string) (io.ReadSeekCloser, error)
	Close func() error
}

// ProcessingEngine normalizes and extracts privately outside database transactions; versions include all configuration affecting stage output.
// Without a converter, native DOCX skips normalization and legacy DOC cannot be processed.
type ProcessingEngine interface {
	Converts() bool
	NormalizationVersion() string
	ExtractionVersion() string
	Normalize(context.Context, DocumentInput) (StageOutput, error)
	Extract(context.Context, DocumentInput) (StageOutput, error)
}

type ProcessingSources interface {
	Sources(context.Context, string, int64) ([]domain.Source, error)
}
