package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	copiedMemberOptionOrder = "shuffle"
	entityQuestionGroup     = "question_group"
	ruleOwnerArchived       = "group_owner_archived"
)

type moveOutcome struct {
	questionID string
	copied     bool
}

type moveAudit struct {
	QuestionID  string `json:"questionId"`
	Copied      bool   `json:"copied"`
	FromSection string `json:"fromSectionId"`
	FromGroup   string `json:"fromGroupId,omitempty"`
	ToSection   string `json:"toSectionId"`
	ToGroup     string `json:"toGroupId,omitempty"`
	Index       int    `json:"index"`
}

// MoveQuestion moves one question of a test's draft across a group's boundary, between two groups, or within one,
// under the test revision and the revision of every group it touches. A standalone question dragged into a group
// is copied, leaving the bank question as it was; a member keeps its id when it leaves a group or changes groups.
// A refused move writes nothing.
func (s *GroupsPostgres) MoveQuestion(ctx context.Context, in domain.MoveQuestionInput) (domain.MoveQuestionResult, error) {
	result, err := s.moveQuestion(ctx, in)
	return result, groupWriteError(err)
}

func (s *GroupsPostgres) moveQuestion(ctx context.Context, in domain.MoveQuestionInput) (domain.MoveQuestionResult, error) {
	if s.questions == nil {
		return domain.MoveQuestionResult{}, fmt.Errorf("groups: question store unavailable")
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	owner, err := lockMovedTest(ctx, tx, in)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	plan, err := resolveMovePlan(ctx, tx, in)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	if err := lockMovedGroups(ctx, tx, in, plan); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	order, err := lockMovedMembers(ctx, tx, plan)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	outcome, err := s.applyMove(ctx, tx, in, plan, owner, order)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	return s.finishMove(ctx, tx, in, plan, outcome)
}

func lockMovedTest(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput) (string, error) {
	owner, err := checkVersion(ctx, tx, in.TestID, in.ExpectedTestUpdatedAt, in.Scope)
	if err != nil {
		return "", err
	}
	var archived bool
	if err := tx.QueryRow(ctx, `SELECT status='archived' FROM app.tests WHERE id=$1`, in.TestID).Scan(&archived); err != nil {
		return "", err
	}
	if archived {
		return "", &domain.GroupError{Rule: ruleOwnerArchived}
	}
	return owner, domain.RequireGroupWrite(in.Grants, false)
}

func lockMovedGroups(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan) error {
	for _, id := range plan.lockOrder() {
		stored, err := readLockedGroup(ctx, tx, id, groupExclusive)
		if err != nil {
			return err
		}
		if !inScope(in.Scope, stored.OwnerID) {
			return domain.ErrNotFound
		}
		if stored.Revision != plan.expected[id] {
			return domain.ErrStaleWrite
		}
	}
	return nil
}

func lockMovedMembers(ctx context.Context, tx pgx.Tx, plan movePlan) (map[string][]string, error) {
	order := make(map[string][]string, len(plan.touched))
	for _, id := range plan.lockOrder() {
		if _, err := lockGroupMembers(ctx, tx, id); err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, `SELECT id::text FROM app.questions WHERE context_group_id=$1 ORDER BY context_ordinal`, id)
		if err != nil {
			return nil, err
		}
		if order[id], err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func (s *GroupsPostgres) applyMove(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan, owner string, order map[string][]string) (moveOutcome, error) {
	switch plan.kind {
	case moveIntoGroup:
		return s.copyIntoGroup(ctx, tx, in, plan, order)
	case moveOutOfGroup:
		return s.moveOutOfGroup(ctx, tx, plan, owner, order)
	default:
		return reparentMember(ctx, tx, plan, order)
	}
}

func (s *GroupsPostgres) copyIntoGroup(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan, order map[string][]string) (moveOutcome, error) {
	members := order[plan.targetGroup]
	if len(members) >= domain.MaxGroupMembers {
		return moveOutcome{}, &domain.GroupError{Rule: domain.RuleGroupLimits}
	}
	if err := s.lockBankQuestion(ctx, tx, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	source, err := s.questions.BankQuestion(ctx, tx, in.Scope, plan.questionID)
	if err != nil {
		return moveOutcome{}, bankReadError(err)
	}
	member, err := detachedMember(source)
	if err != nil {
		return moveOutcome{}, err
	}
	if err := s.lockGroupAssets(ctx, tx, in.Scope, domain.GroupBundle{Questions: []domain.GroupQuestion{member}}); err != nil {
		return moveOutcome{}, err
	}
	if err := s.ensureSectionUnits(ctx, tx, plan.sourceSection); err != nil {
		return moveOutcome{}, err
	}
	if err := deferMoveOrdinals(ctx, tx); err != nil {
		return moveOutcome{}, err
	}
	if err := removeStandaloneQuestion(ctx, tx, plan.sourceSection, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	write := questions.WriteInput{ID: member.ID, Input: member.Input, MediaAssetKind: member.MediaAssetKind,
		ActorID: in.ActorID, Now: in.Now, IP: in.IP, UserAgent: in.UserAgent}
	owned := questions.GroupOwnership{GroupID: plan.targetGroup, Ordinal: len(members), OptionOrder: copiedMemberOptionOrder}
	if _, err := s.questions.CreateGroupMember(ctx, tx, write, owned); err != nil {
		return moveOutcome{}, err
	}
	if err := setMemberOrder(ctx, tx, placed(members, member.ID, plan.index)); err != nil {
		return moveOutcome{}, err
	}
	return moveOutcome{questionID: member.ID, copied: true}, nil
}

func (s *GroupsPostgres) moveOutOfGroup(ctx context.Context, tx pgx.Tx, plan movePlan, owner string, order map[string][]string) (moveOutcome, error) {
	members := order[plan.sourceGroup]
	if !slices.Contains(members, plan.questionID) {
		return moveOutcome{}, domain.ErrNotFound
	}
	if err := requireUnbound(ctx, tx, plan.sourceGroup, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	if err := s.ensureSectionUnits(ctx, tx, plan.targetSection); err != nil {
		return moveOutcome{}, err
	}
	if err := deferMoveOrdinals(ctx, tx); err != nil {
		return moveOutcome{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE app.questions SET context_group_id=NULL, context_ordinal=NULL, context_option_order=NULL
		WHERE id=$1 AND context_group_id=$2`, plan.questionID, plan.sourceGroup)
	if err != nil {
		return moveOutcome{}, err
	}
	if tag.RowsAffected() != 1 {
		return moveOutcome{}, domain.ErrNotFound
	}
	if err := setMemberOrder(ctx, tx, without(members, plan.questionID)); err != nil {
		return moveOutcome{}, err
	}
	if err := s.lockBankQuestion(ctx, tx, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	foreign, err := s.questions.NotOwnedBy(ctx, tx, owner, []string{plan.questionID})
	if err != nil {
		return moveOutcome{}, err
	}
	if len(foreign) > 0 {
		return moveOutcome{}, domain.ErrNotFound
	}
	if err := insertStandaloneQuestion(ctx, tx, plan.targetSection, plan.questionID, plan.index); err != nil {
		return moveOutcome{}, err
	}
	return moveOutcome{questionID: plan.questionID}, nil
}

func reparentMember(ctx context.Context, tx pgx.Tx, plan movePlan, order map[string][]string) (moveOutcome, error) {
	source, target := order[plan.sourceGroup], order[plan.targetGroup]
	if !slices.Contains(source, plan.questionID) {
		return moveOutcome{}, domain.ErrNotFound
	}
	if err := deferMoveOrdinals(ctx, tx); err != nil {
		return moveOutcome{}, err
	}
	if plan.sourceGroup == plan.targetGroup {
		return moveOutcome{questionID: plan.questionID}, setMemberOrder(ctx, tx, placed(target, plan.questionID, plan.index))
	}
	if len(target) >= domain.MaxGroupMembers {
		return moveOutcome{}, &domain.GroupError{Rule: domain.RuleGroupLimits}
	}
	if err := requireUnbound(ctx, tx, plan.sourceGroup, plan.questionID); err != nil {
		return moveOutcome{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE app.questions SET context_group_id=$2, context_ordinal=$3
		WHERE id=$1 AND context_group_id=$4`, plan.questionID, plan.targetGroup, len(target), plan.sourceGroup)
	if err != nil {
		return moveOutcome{}, err
	}
	if tag.RowsAffected() != 1 {
		return moveOutcome{}, domain.ErrNotFound
	}
	if err := setMemberOrder(ctx, tx, without(source, plan.questionID)); err != nil {
		return moveOutcome{}, err
	}
	return moveOutcome{questionID: plan.questionID}, setMemberOrder(ctx, tx, placed(target, plan.questionID, plan.index))
}

func (s *GroupsPostgres) finishMove(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan, outcome moveOutcome) (domain.MoveQuestionResult, error) {
	if _, err := tx.Exec(ctx, `UPDATE app.question_groups SET revision=revision+1 WHERE id=ANY($1::uuid[])`, plan.touched); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET updated_at=$2 WHERE id=$1`, in.TestID, in.Now); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	if err := recordMove(ctx, tx, in, plan, outcome); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	if err := settleMoveConstraints(ctx, tx); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	groups := make([]domain.StoredGroup, 0, len(plan.touched))
	for _, id := range plan.touched {
		group, err := readGroup(ctx, tx, id)
		if err != nil {
			return domain.MoveQuestionResult{}, err
		}
		if err := s.readGraph(ctx, tx, &group); err != nil {
			return domain.MoveQuestionResult{}, err
		}
		groups = append(groups, group)
	}
	drafts := NewPostgres(db.NewContext(tx), s.questions, s.media).WithGroupQuestions(s.questions)
	test, err := drafts.get(ctx, tx, in.TestID)
	if err != nil {
		return domain.MoveQuestionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MoveQuestionResult{}, err
	}
	return domain.MoveQuestionResult{Test: test, QuestionID: outcome.questionID, Copied: outcome.copied, Groups: groups}, nil
}

func recordMove(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan, outcome moveOutcome) error {
	diff, err := json.Marshal(moveAudit{QuestionID: outcome.questionID, Copied: outcome.copied,
		FromSection: plan.sourceSection, FromGroup: plan.sourceGroup, ToSection: plan.targetSection, ToGroup: plan.targetGroup, Index: plan.index})
	if err != nil {
		return err
	}
	entry := audit.Entry{ActorUserID: &in.ActorID, Action: "test.question_moved", Entity: entityTest, EntityID: &in.TestID,
		OccurredAt: in.Now, IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent), Diff: diff}
	if err := audit.Write(ctx, tx, entry); err != nil {
		return err
	}
	for _, id := range plan.touched {
		entry := audit.Entry{ActorUserID: &in.ActorID, Action: "question_group.updated", Entity: entityQuestionGroup, EntityID: &id,
			OccurredAt: in.Now, IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent)}
		if err := audit.Write(ctx, tx, entry); err != nil {
			return err
		}
	}
	return nil
}

func detachedMember(source questions.Question) (domain.GroupQuestion, error) {
	var failure error
	member := domain.GroupQuestion{ID: source.ID, Input: groupQuestionInput(source), MediaAssetKind: source.MediaAssetKind}
	copied, err := member.Copy(func() string {
		id, err := uuid.NewV7()
		if err != nil {
			failure = err
			return ""
		}
		return id.String()
	})
	if failure != nil {
		return domain.GroupQuestion{}, failure
	}
	return copied, err
}

func bankReadError(err error) error {
	if errors.Is(err, questions.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}
