package adapters

import (
	"context"
	"errors"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"

	"github.com/jackc/pgx/v5"
)

// GroupQuestions binds question persistence to the tests aggregate's existing transaction.
type GroupQuestions struct{}

func (GroupQuestions) LockForDraftUse(ctx context.Context, tx pgx.Tx, questionID string) error {
	return repositories.LockForDraftUse(ctx, tx, questionID)
}

func (GroupQuestions) NotOwnedBy(ctx context.Context, tx pgx.Tx, ownerID string, questionIDs []string) ([]string, error) {
	return repositories.NotOwnedBy(ctx, tx, ownerID, questionIDs)
}

func (GroupQuestions) CreateGroupMember(ctx context.Context, tx pgx.Tx, in domain.WriteInput, ownership domain.GroupOwnership) (domain.Question, error) {
	return repositories.NewPostgres(db.NewContext(tx)).WithMedia(QuestionMedia{}).CreateOwned(ctx, in, ownership)
}

func (GroupQuestions) GroupMembers(ctx context.Context, tx pgx.Tx, groupID string) ([]domain.OwnedQuestion, error) {
	return repositories.NewPostgres(db.NewContext(tx)).GroupQuestions(ctx, groupID)
}

func (GroupQuestions) UpdateGroupMember(ctx context.Context, tx pgx.Tx, in domain.WriteInput, ownership domain.GroupOwnership) (domain.Question, error) {
	return repositories.NewPostgres(db.NewContext(tx)).WithMedia(QuestionMedia{}).UpdateOwned(ctx, in, ownership)
}

func (GroupQuestions) BankQuestion(ctx context.Context, tx pgx.Tx, scope access.Scope, id string) (domain.Question, error) {
	return repositories.NewPostgres(db.NewContext(tx)).Get(ctx, scope, id)
}

// QuestionMedia gives the question store the media module's locks and
// readability on the store's own transaction, for the media a question's
// prose shows (DG-109).
type QuestionMedia struct{}

func (QuestionMedia) LockForVersionUse(ctx context.Context, tx pgx.Tx, assetID string) error {
	err := mediarepo.LockForVersionUse(ctx, tx, assetID)
	if errors.Is(err, mediadomain.ErrNotFound) {
		return domain.ErrMediaNotFound
	}
	return err
}

func (QuestionMedia) ReadableKinds(ctx context.Context, tx pgx.Tx, scope access.Scope, assetIDs []string) (map[string]string, error) {
	readable, err := mediarepo.Readable(ctx, tx, scope, assetIDs)
	if err != nil {
		return nil, err
	}
	kinds := make(map[string]string, len(readable))
	for id, kind := range readable {
		kinds[id] = string(kind)
	}
	return kinds, nil
}
