package repositories

import (
	"context"
	"quizzivy/internal/modules/questions/domain"

	"github.com/jackc/pgx/v5"
)

// CreateOwned inserts a member through the group aggregate's transaction; the caller holds its owner lock and validates the full graph.
func (s *Postgres) CreateOwned(ctx context.Context, in domain.WriteInput, ownership domain.GroupOwnership) (domain.Question, error) {
	in.Input.Tags = append([]string{}, in.Input.Tags...)
	return s.write(ctx, in, false, &ownership)
}

// UpdateOwned replaces a member only within its expected group; the caller holds the aggregate lock and validates all dependent bindings.
func (s *Postgres) UpdateOwned(ctx context.Context, in domain.WriteInput, ownership domain.GroupOwnership) (domain.Question, error) {
	in.Input.Tags = append([]string{}, in.Input.Tags...)
	return s.write(ctx, in, true, &ownership)
}

// GroupQuestions resolves ordered members in bulk on the caller's transaction, which must hold the aggregate lock.
func (s *Postgres) GroupQuestions(ctx context.Context, groupID string) ([]domain.OwnedQuestion, error) {
	rows, err := s.Query(ctx, `SELECT`+questionColumns+`, q.context_ordinal, q.context_option_order
		FROM app.questions q WHERE q.context_group_id=$1 AND q.deleted_at IS NULL ORDER BY q.context_ordinal`, groupID)
	if err != nil {
		return nil, err
	}
	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.OwnedQuestion, error) {
		var member domain.OwnedQuestion
		question, err := scanQuestion(row, &member.Ordinal, &member.OptionOrder)
		member.Question = question
		return member, err
	})
	if err != nil {
		return nil, err
	}
	questions := make([]domain.Question, len(members))
	for i := range members {
		questions[i] = members[i].Question
	}
	if err := s.attachChildren(ctx, questions); err != nil {
		return nil, err
	}
	for i := range members {
		members[i].Question = questions[i]
	}
	return members, nil
}

func ownershipValues(ownership *domain.GroupOwnership) (*string, *int, *string) {
	if ownership == nil {
		return nil, nil, nil
	}
	return &ownership.GroupID, &ownership.Ordinal, &ownership.OptionOrder
}
