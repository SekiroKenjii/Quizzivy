package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) copyDraftGraph(ctx context.Context, tx pgx.Tx, testID string, draft domain.DraftContent, in domain.DuplicateInput) error {
	protected := domain.DraftContent{}
	for _, section := range draft.Sections {
		owned := domain.DraftSection{Groups: section.Groups}
		for _, group := range section.Groups {
			for _, question := range group.Questions {
				owned.Questions = append(owned.Questions, draftGroupQuestion(question, len(owned.Questions)))
			}
		}
		protected.Sections = append(protected.Sections, owned)
	}
	if err := lockMediaAssets(ctx, tx, protected, s.media); err != nil {
		return err
	}
	for _, section := range draft.Sections {
		if err := s.copyDraftSection(ctx, tx, testID, section, in); err != nil {
			return err
		}
	}
	return nil
}

func (s *Postgres) copyDraftSection(ctx context.Context, tx pgx.Tx, testID string, section domain.DraftSection, in domain.DuplicateInput) error {
	var sectionID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title,instructions) VALUES ($1,$2,$3,$4) RETURNING id::text`, testID, section.Ordinal, section.Title, section.Instructions).Scan(&sectionID); err != nil {
		return err
	}
	ids := standaloneDraftIDs(section)
	if err := s.lockQuestions(ctx, tx, []domain.SectionInput{{QuestionIDs: ids}}); err != nil {
		return err
	}
	if err := writeSectionQuestions(ctx, tx, sectionID, ids); err != nil {
		return err
	}
	if len(section.Units) == 0 {
		return nil
	}
	questions := make(map[string]string, len(ids))
	for _, id := range ids {
		questions[id] = id
	}
	groups := NewGroupsPostgres(db.NewContext(tx), s.groupQuestions, s.media)
	copied := make(map[string]string, len(section.Groups))
	for _, group := range section.Groups {
		bundle, err := detachedGroupCopy(group)
		if err != nil {
			return err
		}
		var updated time.Time
		if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&updated); err != nil {
			return err
		}
		stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updated,
			ActorID: in.ActorID, Now: in.Now, IP: in.IP, UserAgent: in.UserAgent})
		if err != nil {
			return err
		}
		copied[group.Group.ID] = stored.Bundle.Group.ID
	}
	return replaceCopiedUnits(ctx, tx, sectionID, section.Units, questions, copied)
}

func standaloneDraftIDs(section domain.DraftSection) []string {
	ids := []string{}
	if len(section.Units) == 0 {
		for _, question := range section.Questions {
			ids = append(ids, question.SourceID)
		}
		return ids
	}
	for _, unit := range section.Units {
		if unit.QuestionID != "" {
			ids = append(ids, unit.QuestionID)
		}
	}
	return ids
}
