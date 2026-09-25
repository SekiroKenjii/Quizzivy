package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

func lockDraftContent(ctx context.Context, tx pgx.Tx, testID string, forCopy bool) error {
	rows, err := tx.Query(ctx, `SELECT g.id FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id
		WHERE s.test_id=$1 ORDER BY g.id FOR SHARE OF g`, testID)
	if err != nil {
		return err
	}
	if _, err := pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return err
	}
	lock := "FOR SHARE OF q"
	if forCopy {
		lock = "FOR UPDATE OF q"
	}
	rows, err = tx.Query(ctx, `WITH members AS (
		SELECT sq.question_id FROM app.test_section_questions sq JOIN app.test_sections s ON s.id=sq.test_section_id WHERE s.test_id=$1
		UNION SELECT u.question_id FROM app.test_section_units u JOIN app.test_sections s ON s.id=u.test_section_id WHERE s.test_id=$1 AND u.question_id IS NOT NULL
		UNION SELECT q.id FROM app.questions q JOIN app.question_groups g ON g.id=q.context_group_id JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1
	) SELECT q.id::text FROM members m JOIN app.questions q ON q.id=m.question_id ORDER BY q.id `+lock, testID)
	if err != nil {
		return err
	}
	_, err = pgx.CollectRows(rows, pgx.RowTo[string])
	return err
}

func (s *Postgres) loadDraftUnits(ctx context.Context, tx pgx.Tx, section *domain.DraftSection) error {
	rows, err := tx.Query(ctx, `SELECT coalesce(question_id::text,''),coalesce(group_id::text,'') FROM app.test_section_units
		WHERE test_section_id=$1 ORDER BY ordinal`, section.ID)
	if err != nil {
		return err
	}
	section.Units, err = pgx.CollectRows(rows, pgx.RowToStructByPos[domain.DraftUnit])
	if err != nil || len(section.Units) == 0 {
		return err
	}
	standalone := make(map[string]domain.DraftQuestion, len(section.Questions))
	for _, question := range section.Questions {
		standalone[question.SourceID] = question
	}
	section.Questions = nil
	groups := NewGroupsPostgres(db.NewContext(tx), s.groupQuestions, s.media)
	for _, unit := range section.Units {
		if unit.GroupID == "" {
			question, exists := standalone[unit.QuestionID]
			if !exists {
				return domain.ErrUnknownQuestion
			}
			question.Ordinal = len(section.Questions)
			section.Questions = append(section.Questions, question)
			continue
		}
		if err := appendDraftGroup(ctx, groups, section, unit.GroupID); err != nil {
			return err
		}
	}
	return nil
}

func draftGroupQuestion(q domain.GroupQuestion, ordinal int) domain.DraftQuestion {
	in := q.Input
	out := domain.DraftQuestion{SourceID: q.ID, Ordinal: ordinal, Type: string(in.Type), Prompt: in.Prompt,
		PromptContent: in.PromptContent, ExplanationContent: in.ExplanationContent, Points: in.Points,
		MediaAssetID: in.MediaAssetID, MediaAssetKind: q.MediaAssetKind, Transcript: in.Transcript, Explanation: in.Explanation, SampleAnswer: in.SampleAnswer}
	if in.Audio != nil {
		out.MaxPlays, out.AllowSeek, out.ShowTranscript = in.Audio.MaxPlays, &in.Audio.AllowSeek, &in.Audio.ShowTranscriptAfterSubmit
	}
	for i, option := range in.Options {
		out.Options = append(out.Options, domain.DraftOption{Ordinal: i, Text: option.Text, Content: option.Content, IsCorrect: option.IsCorrect})
	}
	for _, blank := range in.Blanks {
		out.Blanks = append(out.Blanks, domain.DraftBlank{Ordinal: blank.Ordinal, GapID: blank.GapID, AcceptedAnswers: blank.AcceptedAnswers, CaseSensitive: blank.CaseSensitive})
	}
	return out
}

func appendDraftGroup(ctx context.Context, groups *GroupsPostgres, section *domain.DraftSection, id string) error {
	stored, err := groups.Get(ctx, id)
	if err != nil {
		return err
	}
	if stored.OwnerSectionID == nil || *stored.OwnerSectionID != section.ID {
		return fmt.Errorf("publish: group belongs to another section")
	}
	section.Groups = append(section.Groups, stored.Bundle)
	for _, question := range stored.Bundle.Questions {
		section.Questions = append(section.Questions, draftGroupQuestion(question, len(section.Questions)))
	}
	return nil
}
