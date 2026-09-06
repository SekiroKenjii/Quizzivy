package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

type Preview struct {
	Code string
}

type PreviewHandler struct {
	*support.Enrolment
}

func (s PreviewHandler) Handle(ctx context.Context, q Preview) (domain.PreviewResult, error) {
	normalized := domain.JoinCodes.Normalize(q.Code)
	if normalized == "" {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}

	row, err := s.Repo.LookupByCodeHash(ctx, domain.JoinCodes.Hash(normalized))
	if err != nil {
		return domain.PreviewResult{}, err
	}
	if row == nil {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}
	if !domain.JoinCodes.Equal(row.CodeHash, domain.JoinCodes.Hash(normalized)) {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}

	if outcome := row.Usable(s.Now()); outcome != domain.PreviewOK {
		return domain.PreviewResult{Outcome: outcome}, nil
	}

	if row.TeacherName == nil || *row.TeacherName == "" {
		return domain.PreviewResult{}, domain.ErrNoTeacher
	}

	return domain.PreviewResult{
		Outcome:     domain.PreviewOK,
		ClassID:     row.ClassID,
		ClassName:   row.ClassName,
		TeacherName: *row.TeacherName,
	}, nil
}
