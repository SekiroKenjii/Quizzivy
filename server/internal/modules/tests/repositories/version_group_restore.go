package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) lockVersionAssets(ctx context.Context, tx pgx.Tx, versionID string) error {
	rows, err := tx.Query(ctx, `WITH assets AS (
		SELECT q.media_asset_id AS id FROM app.test_version_questions q JOIN app.test_version_sections sec ON sec.id=q.test_version_section_id
		WHERE sec.test_version_id=$1 AND q.media_asset_id IS NOT NULL
		UNION SELECT a.media_asset_id FROM app.test_version_group_assets a JOIN app.test_version_groups g ON g.id=a.group_id
		JOIN app.test_version_sections sec ON sec.id=g.test_version_section_id WHERE sec.test_version_id=$1
	) SELECT id::text FROM assets ORDER BY id`, versionID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(ids) > 0 && s.media == nil {
		return fmt.Errorf("restore: media reference locks unavailable")
	}
	for _, id := range ids {
		if err := s.media.LockForVersionUse(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Postgres) restoreSnapshotGroups(ctx context.Context, tx pgx.Tx, req domain.VersionRequest, now time.Time, copies []copiedSnapshotSection) error {
	rows, err := tx.Query(ctx, `SELECT id::text FROM app.test_sections WHERE test_id=$1 ORDER BY ordinal`, req.ID)
	if err != nil {
		return err
	}
	destinations, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(destinations) != len(copies) {
		return fmt.Errorf("restore: incomplete section mapping")
	}
	groups := NewGroupsPostgres(db.NewContext(tx), s.groupQuestions, s.media)
	for i, section := range copies {
		if err := restoreSectionGroups(ctx, tx, groups, req, now, destinations[i], section); err != nil {
			return err
		}
	}
	return nil
}

func restoreSectionGroups(ctx context.Context, tx pgx.Tx, groups *GroupsPostgres, req domain.VersionRequest, now time.Time, destination string, section copiedSnapshotSection) error {
	rows, err := tx.Query(ctx, `SELECT coalesce(question_id::text,''),coalesce(group_id::text,'') FROM app.test_version_units
		WHERE test_version_section_id=$1 ORDER BY ordinal`, section.SourceID)
	if err != nil {
		return err
	}
	units, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.DraftUnit])
	if err != nil || len(units) == 0 {
		return err
	}
	copiedGroups := make(map[string]string)
	for _, unit := range units {
		if unit.GroupID == "" {
			continue
		}
		source, err := readFrozenGroup(ctx, tx, unit.GroupID)
		if err != nil {
			return err
		}
		bundle, err := detachedGroupCopy(source)
		if err != nil {
			return err
		}
		var updated time.Time
		if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, req.ID).Scan(&updated); err != nil {
			return err
		}
		stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &destination, ExpectedTestUpdatedAt: updated,
			ActorID: req.ActorID, Now: now, IP: req.IP, UserAgent: req.UserAgent})
		if err != nil {
			return err
		}
		copiedGroups[unit.GroupID] = stored.Bundle.Group.ID
	}
	return replaceCopiedUnits(ctx, tx, destination, units, section.Questions, copiedGroups)
}

func replaceCopiedUnits(ctx context.Context, tx pgx.Tx, sectionID string, units []domain.DraftUnit, questions, groups map[string]string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_section_units WHERE test_section_id=$1`, sectionID); err != nil {
		return err
	}
	for i, unit := range units {
		var question, group *string
		if unit.QuestionID != "" {
			id, exists := questions[unit.QuestionID]
			if !exists {
				return domain.ErrUnknownQuestion
			}
			question = &id
		} else {
			id, exists := groups[unit.GroupID]
			if !exists {
				return &domain.GroupError{Rule: groupMembershipRule}
			}
			group = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,question_id,group_id) VALUES ($1,$2,$3,$4)`, sectionID, i, question, group); err != nil {
			return err
		}
	}
	return nil
}
