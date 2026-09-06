package query

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

// ActiveCode returns the live code's metadata, or nil.
type ActiveCode struct {
	ClassID string
}

type ActiveCodeHandler struct {
	*support.Enrolment
}

func (s ActiveCodeHandler) Handle(ctx context.Context, q ActiveCode) (*domain.IssuedCode, error) {
	c, err := s.Repo.ActiveCode(ctx, q.ClassID)
	if err != nil {
		return nil, fmt.Errorf("active code for class %s: %w", q.ClassID, err)
	}
	return c, nil
}
