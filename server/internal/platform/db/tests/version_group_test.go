//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"quizzivy/internal/platform/db"
	"testing"

	"github.com/pressly/goose/v3"
)

func newFrozenGroup(t *testing.T, tx *sql.Tx, section string) (string, string, string) {
	t.Helper()
	var group, question, material string
	if err := tx.QueryRow(`INSERT INTO app.test_version_groups (test_version_section_id,title) VALUES ($1,'Frozen context') RETURNING id`, section).Scan(&group); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`INSERT INTO app.test_version_questions (test_version_section_id,ordinal,type,prompt,points)
		VALUES ($1,0,'fill_blank','[1]',1) RETURNING id`, section).Scan(&question); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, `INSERT INTO app.test_version_blanks (test_version_question_id,ordinal,gap_id) VALUES ($1,1,'response')`, question)
	mustExec(t, tx, `INSERT INTO app.test_version_group_members (group_id,test_version_section_id,question_id,ordinal,option_order)
		VALUES ($1,$2,$3,0,'shuffle')`, group, section, question)
	mustExec(t, tx, `INSERT INTO app.test_version_units (test_version_section_id,ordinal,group_id) VALUES ($1,0,$2)`, section, group)
	if err := tx.QueryRow(`INSERT INTO app.test_version_group_stimuli (group_id,ordinal,title,content)
		VALUES ($1,0,'Frozen passage','{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"gap","label":"1"}]}]}') RETURNING id`, group).Scan(&material); err != nil {
		t.Fatal(err)
	}
	return group, question, material
}

func TestFrozenGroupReferencesCannotCrossSectionsOrContexts(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section, other := newVersion(t, tx, f.adminID), newVersion(t, tx, f.adminID)
		group, question, material := newFrozenGroup(t, tx, section)
		foreignGroup, foreignQuestion, _ := newFrozenGroup(t, tx, other)
		groupConstraint(t, tx, "tvgm_group_section_fk", `UPDATE app.test_version_group_members SET group_id=$1,ordinal=1 WHERE question_id=$2`, foreignGroup, question)
		groupConstraint(t, tx, "tvgm_question_section_fk", `UPDATE app.test_version_group_members SET group_id=$1,test_version_section_id=$2,ordinal=1 WHERE question_id=$3`, foreignGroup, other, question)
		groupConstraint(t, tx, "tvu_group_section_fk", `UPDATE app.test_version_units SET test_version_section_id=$1,ordinal=1 WHERE group_id=$2`, other, group)
		groupConstraint(t, tx, "tvgg_member_fk", `INSERT INTO app.test_version_group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,'gap','blank',$3,'response')`, material, group, foreignQuestion)
		groupConstraint(t, tx, "tvgg_blank_fk", `INSERT INTO app.test_version_group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,'gap','blank',$3,'missing')`, material, group, question)
		mustExec(t, tx, `INSERT INTO app.test_version_group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,'gap','blank',$3,'response')`, material, group, question)
		groupConstraint(t, tx, "tvgg_blank_fk", `DELETE FROM app.test_version_blanks WHERE test_version_question_id=$1`, question)
		mustExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
	})
}

func TestFrozenGroupMediaPolicyIsProtectedAndVersionDeletionIsWhole(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		group, question, material := newFrozenGroup(t, tx, section)
		audio, image := newAudioAsset(t, tx, f.adminID), newImageAsset(t, tx, f.adminID)
		var recording string
		groupConstraint(t, tx, "tvgr_asset_fk", `INSERT INTO app.test_version_group_recordings (group_id,media_asset_id) VALUES ($1,$2)`, group, image)
		if err := tx.QueryRow(`INSERT INTO app.test_version_group_recordings (group_id,media_asset_id,max_plays,transcript)
			VALUES ($1,$2,2,'Frozen transcript') RETURNING id`, group, audio).Scan(&recording); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx, `INSERT INTO app.test_version_group_assets (stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id)
			VALUES ($1,$2,$3,'audio',$4)`, material, group, audio, recording)
		mustExec(t, tx, `INSERT INTO app.test_version_group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,'gap','blank',$3,'response')`, material, group, question)
		groupConstraint(t, tx, "asset_fk", `DELETE FROM app.media_assets WHERE id=$1`, audio)
		mustExec(t, tx, `DELETE FROM app.test_versions WHERE id=(SELECT test_version_id FROM app.test_version_sections WHERE id=$1)`, section)
		mustExec(t, tx, `SET CONSTRAINTS ALL IMMEDIATE`)
		var remaining int
		if err := tx.QueryRow(`SELECT count(*) FROM app.test_version_group_assets WHERE group_id=$1`, group).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("version deletion stranded context: %d, %v", remaining, err)
		}
		mustExec(t, tx, `DELETE FROM app.media_assets WHERE id=$1`, audio)
	})
}

func TestAppCannotUpdateFrozenGroupTables(t *testing.T) {
	conn := migrated(t)
	for _, table := range []string{"test_version_groups", "test_version_group_members", "test_version_units", "test_version_group_stimuli", "test_version_group_gap_bindings", "test_version_group_recordings", "test_version_group_assets"} {
		var update, insert, read bool
		if err := conn.QueryRow(`SELECT has_table_privilege('quizzivy_app',$1,'UPDATE'),has_table_privilege('quizzivy_app',$1,'INSERT'),has_table_privilege('quizzivy_app',$1,'SELECT')`, "app."+table).Scan(&update, &insert, &read); err != nil || update || !insert || !read {
			t.Fatalf("snapshot privileges for %s: update=%t insert=%t read=%t, %v", table, update, insert, read, err)
		}
	}
}

func TestFrozenGroupRollbackPreservesIndexAndRejectsExistingSnapshots(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, 38); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, 39); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, 38); err != nil {
		t.Fatal(err)
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("intermediate rollback lost concurrent snapshot index")
	}
	if err := goose.UpTo(conn, dir, 39); err != nil {
		t.Fatal(err)
	}
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		newFrozenGroup(t, tx, section)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	before = schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, dir, 38); err == nil {
		t.Fatal("rollback discarded frozen context")
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("refused rollback mutated schema")
	}
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM app.test_version_groups`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("frozen graph after refused rollback: %d, %v", count, err)
	}
}
