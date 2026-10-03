package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

// Preview backs the /join/:code/confirm step (§6.2), which exists so a student
// sees WHICH class they are joining before authenticating.
type Preview struct {
	Code string
}

type PreviewHandler struct {
	*support.Enrolment
}

func (s PreviewHandler) Handle(ctx context.Context, q Preview) (domain.PreviewResult, error) {
	code, ok := s.Lookup(q.Code)
	if !ok {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}

	row, err := s.Repo.LookupByCode(ctx, code)
	if err != nil {
		return domain.PreviewResult{}, err
	}
	if row == nil || !code.Matches(row.Lookup) {
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
