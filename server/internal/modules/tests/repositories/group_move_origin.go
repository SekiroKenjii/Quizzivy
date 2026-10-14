package repositories

import (
	"context"
	"errors"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"slices"

	"github.com/jackc/pgx/v5"
)

type originJudgement struct {
	id      string
	restore bool
	edited  bool
}

func copiedFromOf(ctx context.Context, tx pgx.Tx, memberID string) (string, error) {
	var origin *string
	if err := tx.QueryRow(ctx, `SELECT copied_from_id::text FROM app.questions WHERE id=$1`, memberID).Scan(&origin); err != nil {
		return "", err
	}
	if origin == nil {
		return "", nil
	}
	return *origin, nil
}

func (s *GroupsPostgres) judgeOrigin(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan, owner, originID string) (originJudgement, error) {
	if originID == "" {
		return originJudgement{}, nil
	}
	original, usable, err := s.liveOrigin(ctx, tx, in, owner, originID)
	if err != nil || !usable {
		return originJudgement{}, err
	}
	members, err := s.questions.GroupMembers(ctx, tx, plan.sourceGroup)
	if err != nil {
		return originJudgement{}, err
	}
	at := slices.IndexFunc(members, func(member questions.OwnedQuestion) bool { return member.Question.ID == plan.questionID })
	if at < 0 {
		return originJudgement{}, domain.ErrNotFound
	}
	if groupQuestionOf(members[at].Question).SameContent(groupQuestionOf(original)) {
		return originJudgement{id: originID, restore: true}, nil
	}
	return originJudgement{id: originID, edited: true}, nil
}

func (s *GroupsPostgres) liveOrigin(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, owner, originID string) (questions.Question, bool, error) {
	if err := s.questions.LockForDraftUse(ctx, tx, originID); err != nil {
		return questions.Question{}, false, ignoreMissing(err)
	}
	original, err := s.questions.BankQuestion(ctx, tx, in.Scope, originID)
	if err != nil {
		return questions.Question{}, false, ignoreMissing(err)
	}
	foreign, err := s.questions.NotOwnedBy(ctx, tx, owner, []string{originID})
	if err != nil || len(foreign) > 0 {
		return questions.Question{}, false, err
	}
	var held bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.test_section_questions q
		JOIN app.test_sections s ON s.id=q.test_section_id WHERE s.test_id=$1 AND q.question_id=$2)`, in.TestID, originID).Scan(&held); err != nil || held {
		return questions.Question{}, false, err
	}
	return original, true, nil
}

func ignoreMissing(err error) error {
	if errors.Is(err, questions.ErrNotFound) {
		return nil
	}
	return err
}

func (s *GroupsPostgres) restoreOrigin(ctx context.Context, tx pgx.Tx, plan movePlan, members []string, originID string) (moveOutcome, error) {
	if err := removeMember(ctx, tx, plan.sourceGroup, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	if err := setMemberOrder(ctx, tx, without(members, plan.questionID)); err != nil {
		return moveOutcome{}, err
	}
	if err := s.lockBankQuestion(ctx, tx, originID); err != nil {
		return moveOutcome{}, err
	}
	if err := insertStandaloneQuestion(ctx, tx, plan.targetSection, originID, plan.index); err != nil {
		return moveOutcome{}, err
	}
	return moveOutcome{questionID: originID, restored: true, removedID: plan.questionID}, nil
}
