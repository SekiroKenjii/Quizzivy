package repositories

import (
	"context"
	"errors"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"slices"

	"github.com/jackc/pgx/v5"
)

const maxSectionUnits = 32768

const deferredMoveOrdinals = `app.questions_context_ordinal_key, app.test_section_units_ordinal_key, app.test_section_questions_ordinal_key`

func deferMoveOrdinals(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SET CONSTRAINTS `+deferredMoveOrdinals+` DEFERRED`)
	return err
}

func settleMoveConstraints(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SET CONSTRAINTS `+deferredMoveOrdinals+`, app.group_gap_bindings_member IMMEDIATE`)
	return err
}

func (s *GroupsPostgres) lockBankQuestion(ctx context.Context, tx pgx.Tx, id string) error {
	err := s.questions.LockForDraftUse(ctx, tx, id)
	if errors.Is(err, questions.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func (s *GroupsPostgres) ensureSectionUnits(ctx context.Context, tx pgx.Tx, sectionID string) error {
	var populated bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.test_section_units WHERE test_section_id=$1)`, sectionID).Scan(&populated); err != nil {
		return err
	}
	if populated {
		return nil
	}
	err := s.seedSectionUnits(ctx, tx, sectionID)
	if errors.Is(err, questions.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func removeStandaloneQuestion(ctx context.Context, tx pgx.Tx, sectionID, questionID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_section_units WHERE test_section_id=$1 AND question_id=$2`, sectionID, questionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_section_questions WHERE test_section_id=$1 AND question_id=$2`, sectionID, questionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `WITH ordered AS (
		SELECT id,row_number() OVER (ORDER BY ordinal)-1 AS ordinal FROM app.test_section_units WHERE test_section_id=$1
	) UPDATE app.test_section_units u SET ordinal=o.ordinal FROM ordered o WHERE u.id=o.id AND u.ordinal<>o.ordinal`, sectionID); err != nil {
		return err
	}
	return syncSectionQuestions(ctx, tx, sectionID)
}

func insertStandaloneQuestion(ctx context.Context, tx pgx.Tx, sectionID, questionID string, index int) error {
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.test_section_units WHERE test_section_id=$1`, sectionID).Scan(&count); err != nil {
		return err
	}
	if count >= maxSectionUnits {
		return &domain.GroupError{Rule: domain.RuleGroupLimits}
	}
	at := min(max(index, 0), count)
	if _, err := tx.Exec(ctx, `UPDATE app.test_section_units SET ordinal=ordinal+1 WHERE test_section_id=$1 AND ordinal>=$2`, sectionID, at); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,question_id) VALUES ($1,$2,$3)`, sectionID, at, questionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id,ordinal,question_id)
		SELECT $1,count(*),$2 FROM app.test_section_questions WHERE test_section_id=$1`, sectionID, questionID); err != nil {
		return err
	}
	return syncSectionQuestions(ctx, tx, sectionID)
}

func syncSectionQuestions(ctx context.Context, tx pgx.Tx, sectionID string) error {
	_, err := tx.Exec(ctx, `WITH ordered AS (
		SELECT question_id,row_number() OVER (ORDER BY ordinal)-1 AS ordinal FROM app.test_section_units
		 WHERE test_section_id=$1 AND question_id IS NOT NULL
	) UPDATE app.test_section_questions q SET ordinal=o.ordinal FROM ordered o
	   WHERE q.test_section_id=$1 AND q.question_id=o.question_id AND q.ordinal<>o.ordinal`, sectionID)
	return err
}

func placed(ids []string, id string, index int) []string {
	rest := without(ids, id)
	return slices.Insert(rest, min(max(index, 0), len(rest)), id)
}

func without(ids []string, id string) []string {
	rest := make([]string, 0, len(ids))
	for _, other := range ids {
		if other != id {
			rest = append(rest, other)
		}
	}
	return rest
}

func setMemberOrder(ctx context.Context, tx pgx.Tx, ids []string) error {
	ordinals := make([]int, len(ids))
	for i := range ids {
		ordinals[i] = i
	}
	_, err := tx.Exec(ctx, `UPDATE app.questions q SET context_ordinal=o.ordinal FROM unnest($1::uuid[],$2::int[]) AS o(id,ordinal)
		WHERE q.id=o.id AND q.context_ordinal IS DISTINCT FROM o.ordinal`, ids, ordinals)
	return err
}

func requireUnbound(ctx context.Context, tx pgx.Tx, groupID, questionID string) error {
	var stimulus, gap string
	err := tx.QueryRow(ctx, `SELECT stimulus_id::text, gap_id FROM app.group_gap_bindings WHERE group_id=$1 AND question_id=$2
		ORDER BY stimulus_id, gap_id LIMIT 1`, groupID, questionID).Scan(&stimulus, &gap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &domain.GroupError{Rule: domain.RuleGroupGapBound, StimulusID: stimulus, GapID: gap, QuestionID: questionID}
}
