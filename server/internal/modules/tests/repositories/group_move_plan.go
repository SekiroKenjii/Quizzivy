package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

type moveKind int

const (
	moveIntoGroup moveKind = iota
	moveOutOfGroup
	moveBetweenGroups
)

type movePlan struct {
	kind          moveKind
	questionID    string
	sourceSection string
	sourceGroup   string
	targetSection string
	targetGroup   string
	index         int
	touched       []string
	expected      map[string]int64
}

func (p movePlan) lockOrder() []string {
	ids := slices.Clone(p.touched)
	slices.Sort(ids)
	return ids
}

func resolveMovePlan(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput) (movePlan, error) {
	plan := movePlan{questionID: strings.ToLower(in.QuestionID), targetSection: strings.ToLower(in.To.SectionID), index: max(in.To.Index, 0)}
	if in.To.GroupID != nil {
		plan.targetGroup = strings.ToLower(*in.To.GroupID)
	}
	if err := resolveMoveSource(ctx, tx, in.TestID, &plan); err != nil {
		return movePlan{}, err
	}
	if plan.sourceGroup == "" && plan.targetGroup == "" {
		return movePlan{}, &domain.GroupError{Rule: domain.RuleMoveOutlineOnly, QuestionID: plan.questionID}
	}
	if err := resolveMoveTarget(ctx, tx, in.TestID, plan); err != nil {
		return movePlan{}, err
	}
	switch {
	case plan.sourceGroup == "":
		plan.kind = moveIntoGroup
	case plan.targetGroup == "":
		plan.kind = moveOutOfGroup
	default:
		plan.kind = moveBetweenGroups
	}
	plan.touched = touchedGroups(plan)
	expected, err := resolveExpectedGroups(ctx, tx, in, plan)
	if err != nil {
		return movePlan{}, err
	}
	plan.expected = expected
	return plan, nil
}

func resolveMoveSource(ctx context.Context, tx pgx.Tx, testID string, plan *movePlan) error {
	err := tx.QueryRow(ctx, `SELECT g.id::text, g.owner_section_id::text FROM app.questions q
		JOIN app.question_groups g ON g.id=q.context_group_id
		JOIN app.test_sections s ON s.id=g.owner_section_id
		WHERE q.id=$1 AND s.test_id=$2`, plan.questionID, testID).Scan(&plan.sourceGroup, &plan.sourceSection)
	if err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT sq.test_section_id::text FROM app.test_section_questions sq
		JOIN app.test_sections s ON s.id=sq.test_section_id
		WHERE sq.question_id=$1 AND s.test_id=$2 ORDER BY sq.test_section_id`, plan.questionID, testID)
	if err != nil {
		return err
	}
	sections, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	switch len(sections) {
	case 0:
		return domain.ErrNotFound
	case 1:
		plan.sourceSection = sections[0]
		return nil
	default:
		return &domain.GroupError{Rule: domain.RuleMoveSourceAmbiguous, QuestionID: plan.questionID}
	}
}

func resolveMoveTarget(ctx context.Context, tx pgx.Tx, testID string, plan movePlan) error {
	var found bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.test_sections WHERE id=$1 AND test_id=$2)`,
		plan.targetSection, testID).Scan(&found); err != nil {
		return err
	}
	if !found {
		return domain.ErrNotFound
	}
	if plan.targetGroup == "" {
		return nil
	}
	var owner string
	err := tx.QueryRow(ctx, `SELECT g.owner_section_id::text FROM app.question_groups g
		JOIN app.test_sections s ON s.id=g.owner_section_id WHERE g.id=$1 AND s.test_id=$2`,
		plan.targetGroup, testID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != plan.targetSection {
		return &domain.GroupError{Rule: domain.RuleMoveGroupSection}
	}
	return nil
}

func touchedGroups(plan movePlan) []string {
	var ids []string
	if plan.sourceGroup != "" {
		ids = append(ids, plan.sourceGroup)
	}
	if plan.targetGroup != "" && plan.targetGroup != plan.sourceGroup {
		ids = append(ids, plan.targetGroup)
	}
	return ids
}

func resolveExpectedGroups(ctx context.Context, tx pgx.Tx, in domain.MoveQuestionInput, plan movePlan) (map[string]int64, error) {
	expected := make(map[string]int64, len(in.ExpectedGroups))
	ids := make([]string, 0, len(in.ExpectedGroups))
	repeated := false
	for _, entry := range in.ExpectedGroups {
		id := strings.ToLower(entry.GroupID)
		if _, seen := expected[id]; seen {
			repeated = true
			continue
		}
		expected[id] = entry.Revision
		ids = append(ids, id)
	}
	var reachable int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.question_groups g
		JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1 AND g.id=ANY($2::uuid[])`,
		in.TestID, ids).Scan(&reachable); err != nil {
		return nil, err
	}
	if reachable != len(ids) {
		return nil, domain.ErrNotFound
	}
	if repeated || slices.ContainsFunc(ids, func(id string) bool { return !slices.Contains(plan.touched, id) }) {
		return nil, &domain.GroupError{Rule: domain.RuleMoveRevisions}
	}
	if slices.ContainsFunc(plan.touched, func(id string) bool { _, listed := expected[id]; return !listed }) {
		return nil, domain.ErrStaleWrite
	}
	return expected, nil
}
