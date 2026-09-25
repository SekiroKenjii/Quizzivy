//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

func newGroup(t *testing.T, tx *sql.Tx, author string, section *string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(`INSERT INTO app.question_groups (title, created_by, owner_section_id)
		VALUES ('Bài đọc', $1, $2) RETURNING id`, author, section).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func newGroupMaterial(t *testing.T, tx *sql.Tx, group string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(`INSERT INTO app.group_stimuli (group_id, ordinal, title, content)
		VALUES ($1, 0, 'Ngữ liệu', '{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"gap-1","label":"1"}]}]}') RETURNING id`, group).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func ownGroupQuestion(t *testing.T, tx *sql.Tx, author, group string, ordinal int) string {
	t.Helper()
	id := newQuestion(t, tx, author, "fill_blank", "[1]")
	mustExec(t, tx, `UPDATE app.questions SET context_group_id=$2, context_ordinal=$3, context_option_order='shuffle',
		prompt_content='{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"answer-1","label":"1"}]}]}' WHERE id=$1`, id, group, ordinal)
	mustExec(t, tx, `INSERT INTO app.question_blanks (question_id, ordinal, gap_id) VALUES ($1,1,'answer-1')`, id)
	return id
}

func groupConstraint(t *testing.T, tx *sql.Tx, constraint, query string, args ...any) {
	t.Helper()
	mustExec(t, tx, `SAVEPOINT invalid_group`)
	_, err := tx.Exec(query, args...)
	if err == nil {
		_, err = tx.Exec(`SET CONSTRAINTS ALL IMMEDIATE`)
	}
	if err == nil || !strings.Contains(err.Error(), constraint) {
		t.Fatalf("wanted %s, got %v", constraint, err)
	}
	mustExec(t, tx, `ROLLBACK TO SAVEPOINT invalid_group`)
	mustExec(t, tx, `RELEASE SAVEPOINT invalid_group`)
}

func TestGroupOwnershipCannotBePartiallyDroppedOrCrossMounted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newSection(t, tx, f.adminID)
		otherSection := newSection(t, tx, f.adminID)
		group := newGroup(t, tx, f.adminID, &section)
		question := ownGroupQuestion(t, tx, f.adminID, group, 0)
		groupConstraint(t, tx, "questions_context_complete", `UPDATE app.questions SET context_ordinal=NULL WHERE id=$1`, question)
		groupConstraint(t, tx, "questions_context_complete", `UPDATE app.questions SET deleted_at=now() WHERE id=$1`, question)
		groupConstraint(t, tx, "questions_context_option_order", `UPDATE app.questions SET context_option_order='fixed' WHERE id=$1`, question)
		groupConstraint(t, tx, "questions_context_group_id_fkey", `DELETE FROM app.question_groups WHERE id=$1`, group)
		groupConstraint(t, tx, "question_groups_owner_section_id_fkey", `DELETE FROM app.test_sections WHERE id=$1`, section)
		groupConstraint(t, tx, "test_section_units_group_owner", `INSERT INTO app.test_section_units (test_section_id, ordinal, group_id) VALUES ($1,0,$2)`, otherSection, group)
		groupConstraint(t, tx, "question_groups_archive_bank_only", `UPDATE app.question_groups SET archived_at=now() WHERE id=$1`, group)
		groupConstraint(t, tx, "question_groups_instructions_check", `UPDATE app.question_groups SET instructions='{}' WHERE id=$1`, group)
		bankGroup := newGroup(t, tx, f.adminID, nil)
		groupConstraint(t, tx, "test_section_units_group_owner", `INSERT INTO app.test_section_units (test_section_id, ordinal, group_id) VALUES ($1,0,$2)`, section, bankGroup)
		mustExec(t, tx, `INSERT INTO app.test_section_units (test_section_id, ordinal, group_id) VALUES ($1,0,$2)`, section, group)
		groupConstraint(t, tx, "test_section_units_kind", `INSERT INTO app.test_section_units (test_section_id, ordinal, group_id, question_id) VALUES ($1,1,$2,$3)`, section, group, question)
		mustExec(t, tx, `UPDATE app.question_groups SET archived_at=now() WHERE id=$1`, bankGroup)
	})
}

func TestGroupGapBindingsKeepStableTargetsThroughAnswerRowReplacement(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		group := newGroup(t, tx, f.adminID, nil)
		other := newGroup(t, tx, f.adminID, nil)
		material := newGroupMaterial(t, tx, group)
		question := ownGroupQuestion(t, tx, f.adminID, group, 0)
		foreign := ownGroupQuestion(t, tx, f.adminID, other, 0)
		const insert = `INSERT INTO app.group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id) VALUES ($1,$2,'gap-1','blank',$3,$4)`
		groupConstraint(t, tx, "group_gap_bindings_member", insert, material, group, foreign, "answer-1")
		groupConstraint(t, tx, "group_gap_bindings_material", insert, material, other, foreign, "answer-1")
		groupConstraint(t, tx, "group_gap_bindings_blank", insert, material, group, question, "missing")
		groupConstraint(t, tx, "group_gap_bindings_kind", insert, material, group, question, nil)
		mustExec(t, tx, insert, material, group, question, "answer-1")
		groupConstraint(t, tx, "group_gap_bindings_blank", `DELETE FROM app.question_blanks WHERE question_id=$1`, question)
		mustExec(t, tx, `DELETE FROM app.question_blanks WHERE question_id=$1`, question)
		mustExec(t, tx, `INSERT INTO app.question_blanks (question_id,ordinal,gap_id) VALUES ($1,1,'answer-1')`, question)
		mustExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
		groupConstraint(t, tx, "group_stimuli_content_check", `UPDATE app.group_stimuli SET content='{}' WHERE id=$1`, material)
	})
}

func TestGroupMediaBindingsProtectKindScopeAndImmutableBytes(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		group := newGroup(t, tx, f.adminID, nil)
		other := newGroup(t, tx, f.adminID, nil)
		material := newGroupMaterial(t, tx, group)
		audio := newAudioAsset(t, tx, f.adminID)
		image := newImageAsset(t, tx, f.adminID)
		var recording, otherRecording string
		const record = `INSERT INTO app.group_recordings (group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit) VALUES ($1,$2,2,false,false) RETURNING id`
		if err := tx.QueryRow(record, group, audio).Scan(&recording); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(record, other, audio).Scan(&otherRecording); err != nil {
			t.Fatal(err)
		}
		groupConstraint(t, tx, "group_recordings_asset_key", record, group, audio)
		groupConstraint(t, tx, "group_recordings_media", record, group, image)
		const asset = `INSERT INTO app.group_stimulus_assets (stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id) VALUES ($1,$2,$3,'audio',$4)`
		groupConstraint(t, tx, "group_stimulus_assets_recording", asset, material, group, audio, otherRecording)
		groupConstraint(t, tx, "group_stimulus_assets_kind", asset, material, group, audio, nil)
		groupConstraint(t, tx, "group_stimulus_assets_media", asset, material, group, image, recording)
		mustExec(t, tx, asset, material, group, audio, recording)
		mustExec(t, tx, `INSERT INTO app.group_stimulus_assets (stimulus_id,group_id,media_asset_id,media_asset_kind) VALUES ($1,$2,$3,'image')`, material, group, image)
		groupConstraint(t, tx, "group_stimulus_assets_media", `DELETE FROM app.media_assets WHERE id=$1`, image)
		groupConstraint(t, tx, "group_stimulus_assets_recording", `DELETE FROM app.group_recordings WHERE id=$1`, recording)
		mustExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
	})
}

func TestGroupMemberOrdinalsCanBeReorderedAtomically(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		group := newGroup(t, tx, f.adminID, nil)
		first := ownGroupQuestion(t, tx, f.adminID, group, 0)
		second := ownGroupQuestion(t, tx, f.adminID, group, 1)
		groupConstraint(t, tx, "questions_context_ordinal_key", `UPDATE app.questions SET context_ordinal=1 WHERE id=$1`, first)
		mustExec(t, tx, `SET CONSTRAINTS app.questions_context_ordinal_key DEFERRED`)
		mustExec(t, tx, `UPDATE app.questions SET context_ordinal=1 WHERE id=$1`, first)
		mustExec(t, tx, `UPDATE app.questions SET context_ordinal=0 WHERE id=$1`, second)
		mustExec(t, tx, `SET CONSTRAINTS app.questions_context_ordinal_key IMMEDIATE`)
	})
}

func TestGroupGraphRollbackRestoresIntermediateIndexesAndRefusesDataLoss(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn, err := sql.Open("pgx", scratchDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, 36); err != nil {
		t.Fatal(err)
	}
	indexes := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, 37); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, 36); err != nil {
		t.Fatal(err)
	}
	if schemaSnapshot(t, conn) != indexes {
		t.Fatal("down-one lost the concurrent indexes from migration 36")
	}
	if err := goose.UpTo(conn, dir, 37); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`WITH author AS (INSERT INTO app.users (email,full_name,role) VALUES ('group-rollback@example.test','Teacher','admin') RETURNING id)
		INSERT INTO app.question_groups (title,created_by) SELECT 'Saved group', id FROM author`)
	if err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, dir, 36); err == nil {
		t.Fatal("rollback discarded an existing group")
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("refused rollback mutated the schema")
	}
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM app.question_groups`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("saved group after refused rollback: %d, %v", count, err)
	}
	version, err := goose.GetDBVersion(conn)
	if err != nil || version != 37 {
		t.Fatalf("version after refused rollback: %d, %v", version, err)
	}
}
