package query

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
)

type Limits struct{}

// LimitsResult states what an upload may be before a file is chosen.
type LimitsResult struct {
	MaxBytes int64
	Formats  []string
}

type LimitsHandler struct{ Legacy bool }

func (h LimitsHandler) Handle(context.Context, Limits) (LimitsResult, error) {
	formats := []string{"docx", "pdf"}
	if h.Legacy {
		formats = append(formats, "doc")
	}
	return LimitsResult{MaxBytes: domain.MaxSourceBytes, Formats: formats}, nil
}
