package repositories

import (
	"context"
	"errors"
	"fmt"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

const groupMembershipRule = "group_membership"

// GroupQuestionStore persists owned interactions inside the aggregate transaction, without exposing them as standalone bank rows.
type GroupQuestionStore interface {
	QuestionLocks
	CreateGroupMember(context.Context, pgx.Tx, questions.WriteInput, questions.GroupOwnership) (questions.Question, error)
	UpdateGroupMember(context.Context, pgx.Tx, questions.WriteInput, questions.GroupOwnership) (questions.Question, error)
	GroupMembers(context.Context, pgx.Tx, string) ([]questions.OwnedQuestion, error)
}

// GroupsPostgres owns complete draft context graphs; callers expose it only through group-aware authoring commands.
type GroupsPostgres struct {
	db.Repository
	questions GroupQuestionStore
	media     MediaLocks
}

func NewGroupsPostgres(dbx db.Context, questions GroupQuestionStore, media MediaLocks) *GroupsPostgres {
	return &GroupsPostgres{Repository: db.NewRepository(dbx), questions: questions, media: media}
}

// Get reads a complete group while holding its shared aggregate lock, including archived bank groups.
func (s *GroupsPostgres) Get(ctx context.Context, id string) (domain.StoredGroup, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	group, err := readGroup(ctx, tx, id)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if err := s.readGraph(ctx, tx, &group); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.StoredGroup{}, err
	}
	return group, nil
}

func readGroup(ctx context.Context, tx pgx.Tx, id string) (domain.StoredGroup, error) {
	return readLockedGroup(ctx, tx, id, "FOR SHARE")
}

func readLockedGroup(ctx context.Context, tx pgx.Tx, id, lock string) (domain.StoredGroup, error) {
	var stored domain.StoredGroup
	err := tx.QueryRow(ctx, `SELECT id::text, title, instructions, owner_section_id::text,
		revision, archived_at, created_at, updated_at FROM app.question_groups WHERE id=$1 `+lock, id).
		Scan(&stored.Bundle.Group.ID, &stored.Bundle.Group.Title, &stored.Bundle.Group.Instructions,
			&stored.OwnerSectionID, &stored.Revision, &stored.ArchivedAt, &stored.CreatedAt, &stored.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoredGroup{}, domain.ErrNotFound
	}
	return stored, err
}

func (s *GroupsPostgres) readGraph(ctx context.Context, tx pgx.Tx, stored *domain.StoredGroup) error {
	if s.questions == nil {
		return fmt.Errorf("groups: question store unavailable")
	}
	members, err := s.questions.GroupMembers(ctx, tx, stored.Bundle.Group.ID)
	if err != nil {
		return err
	}
	stored.Bundle.Group.Members = make([]domain.GroupMember, len(members))
	stored.Bundle.Questions = make([]domain.GroupQuestion, len(members))
	for i, member := range members {
		if member.Ordinal != i {
			return &domain.GroupError{Rule: groupMembershipRule, QuestionID: member.Question.ID}
		}
		stored.Bundle.Group.Members[i] = domain.GroupMember{QuestionID: member.Question.ID, OptionOrder: member.OptionOrder}
		stored.Bundle.Questions[i] = domain.GroupQuestion{ID: member.Question.ID, Input: groupQuestionInput(member.Question), MediaAssetKind: member.Question.MediaAssetKind}
	}
	if err := readGroupMaterials(ctx, tx, &stored.Bundle.Group, draftGraphTables); err != nil {
		return err
	}
	if err := readGroupRecordings(ctx, tx, &stored.Bundle.Group, draftGraphTables); err != nil {
		return err
	}
	if err := checkGroupAssetBindings(ctx, tx, stored.Bundle.Group, draftGraphTables); err != nil {
		return err
	}
	return stored.Bundle.Validate()
}

func groupQuestionInput(q questions.Question) questions.Input {
	in := questions.Input{
		Type: q.Type, Prompt: q.Prompt, PromptContent: q.PromptContent, Points: q.Points,
		Explanation: q.Explanation, ExplanationContent: q.ExplanationContent, SampleAnswer: q.SampleAnswer,
		MediaAssetID: q.MediaAssetID, Audio: q.Audio, Transcript: q.Transcript, Tags: q.Tags,
	}
	for _, option := range q.Options {
		in.Options = append(in.Options, questions.OptionInput{ID: &option.ID, Content: option.Content, Text: option.Text, IsCorrect: option.IsCorrect})
	}
	for _, blank := range q.Blanks {
		in.Blanks = append(in.Blanks, questions.BlankInput{ID: &blank.ID, GapID: blank.GapID, Ordinal: blank.Ordinal, CaseSensitive: blank.CaseSensitive, AcceptedAnswers: blank.AcceptedAnswers})
	}
	return in
}
