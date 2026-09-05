package application

import (
	"context"
	"quizzivy/internal/modules/classes/domain"
)

// Preview backs the /join/:code/confirm step (§6.2), which exists so a student
// sees WHICH class they are joining before authenticating.
//
// The order of the checks is a leak decision, not an implementation detail.
// self_join_enabled is tested before the code's own state, so a closed class
// answers exactly as a nonexistent one does -- checking revocation or expiry
// first would confirm that a code, and therefore a class, exists.
func (s *Enrolment) Preview(ctx context.Context, rawCode string) (domain.PreviewResult, error) {
	normalized := domain.JoinCodes.Normalize(rawCode)
	if normalized == "" {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}

	row, err := s.repo.LookupByCodeHash(ctx, domain.JoinCodes.Hash(normalized))
	if err != nil {
		return domain.PreviewResult{}, err
	}
	if row == nil {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}
	if !domain.JoinCodes.Equal(row.CodeHash, domain.JoinCodes.Hash(normalized)) {
		return domain.PreviewResult{Outcome: domain.PreviewInvalid}, nil
	}

	if outcome := row.Usable(s.now()); outcome != domain.PreviewOK {
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
