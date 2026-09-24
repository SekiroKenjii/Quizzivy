package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"

	"github.com/google/uuid"
)

// Copy materializes a coherent observed revision with new editable identities; subsequent source changes cannot alter the result.
func (s *GroupsPostgres) Copy(ctx context.Context, in domain.CopyGroupInput) (domain.StoredGroup, error) {
	source, err := s.Get(ctx, in.SourceID)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if source.Revision != in.ExpectedSourceRevision {
		return domain.StoredGroup{}, domain.ErrStaleWrite
	}
	var identityError error
	bundle, err := source.Bundle.Copy(func() string {
		id, err := uuid.NewV7()
		if err != nil {
			identityError = err
			return ""
		}
		return id.String()
	})
	if identityError != nil {
		return domain.StoredGroup{}, identityError
	}
	if err != nil {
		return domain.StoredGroup{}, err
	}
	return s.Create(ctx, domain.CreateGroupInput{
		Bundle: bundle, OwnerSectionID: in.OwnerSectionID, ExpectedTestUpdatedAt: in.ExpectedTestUpdatedAt,
		ActorID: in.ActorID, Now: in.Now, IP: in.IP, UserAgent: in.UserAgent,
	})
}
