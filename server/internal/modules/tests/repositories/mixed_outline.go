package repositories

import (
	"context"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/tests/domain"
	"strings"
)

func lockMixedOutline(ctx context.Context, tx pgx.Tx, testID string, in domain.UpdateInput) error {
	if err := in.Validate(); err != nil {
		return err
	}
	var archived bool
	if err := tx.QueryRow(ctx, `SELECT status='archived' FROM app.tests WHERE id=$1`, testID).Scan(&archived); err != nil {
		return err
	}
	if archived {
		return domain.ErrArchived
	}
	rows, err := tx.Query(ctx, `SELECT g.id::text FROM app.question_groups g
 JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1 ORDER BY g.id FOR UPDATE OF g`, testID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	return validateOwnedGroupUnits(in.Sections, ids)
}

func validateOwnedGroupUnits(sections []domain.SectionInput, ids []string) error {
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	for _, section := range sections {
		for _, unit := range section.Units {
			if unit.Kind != "group" {
				continue
			}
			id := strings.ToLower(unit.ID)
			if !present[id] {
				return outlineGroupError("Nhóm chưa thuộc đề này. Hãy sao chép cả nhóm vào đề trước.")
			}
			delete(present, id)
		}
	}
	if len(present) > 0 {
		return outlineGroupError("Cấu trúc đang thiếu nhóm ngữ liệu. Hãy xoá nhóm bằng thao tác riêng trước khi bỏ khỏi đề.")
	}
	return nil
}

func outlineGroupError(message string) error {
	return &domain.ValidationError{Fields: []domain.FieldError{{Field: "sections", Message: message}}}
}

func clearDraftUnits(ctx context.Context, tx pgx.Tx, testID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM app.test_section_units u USING app.test_sections s
 WHERE s.id=u.test_section_id AND s.test_id=$1`, testID)
	return err
}

func replaceMixedOutline(ctx context.Context, tx pgx.Tx, testID string, sections []domain.SectionInput) error {
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.test_sections_ordinal_key,
 app.test_section_questions_ordinal_key,app.test_section_units_ordinal_key DEFERRED`); err != nil {
		return err
	}
	if err := clearDraftUnits(ctx, tx, testID); err != nil {
		return err
	}
	ids := make([]string, len(sections))
	for i, section := range sections {
		id, err := upsertSection(ctx, tx, testID, i, section)
		if err != nil {
			return err
		}
		ids[i] = id
	}
	for i, section := range sections {
		if err := writeSectionQuestions(ctx, tx, ids[i], section.QuestionIDs); err != nil {
			return err
		}
		if err := writeSectionUnits(ctx, tx, ids[i], section.Units); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM app.test_sections WHERE test_id=$1 AND NOT (id=ANY($2::uuid[]))`, testID, ids)
	return err
}

func writeSectionUnits(ctx context.Context, tx pgx.Tx, sectionID string, units []domain.SectionUnit) error {
	for i, unit := range units {
		var questionID, groupID *string
		id := strings.ToLower(unit.ID)
		if unit.Kind == "group" {
			groupID = &id
			if _, err := tx.Exec(ctx, `UPDATE app.question_groups SET owner_section_id=$2,revision=revision+1
    WHERE id=$1 AND owner_section_id IS DISTINCT FROM $2::uuid`, id, sectionID); err != nil {
				return err
			}
		} else {
			questionID = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,question_id,group_id)
   VALUES ($1,$2,$3,$4)`, sectionID, i, questionID, groupID); err != nil {
			return err
		}
	}
	return nil
}
