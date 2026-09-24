package adapters

import (
	"context"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// GroupQuestions binds question persistence to the tests aggregate's existing transaction.
type GroupQuestions struct{}

func (GroupQuestions) LockForDraftUse(ctx context.Context, tx pgx.Tx, questionID string) error {
	return repositories.LockForDraftUse(ctx, tx, questionID)
}

func (GroupQuestions) CreateGroupMember(ctx context.Context, tx pgx.Tx, in domain.WriteInput, ownership domain.GroupOwnership) (domain.Question, error) {
	return repositories.NewPostgres(db.NewContext(tx)).CreateOwned(ctx, in, ownership)
}

func (GroupQuestions) GroupMembers(ctx context.Context, tx pgx.Tx, groupID string) ([]domain.OwnedQuestion, error) {
	return repositories.NewPostgres(db.NewContext(tx)).GroupQuestions(ctx, groupID)
}
