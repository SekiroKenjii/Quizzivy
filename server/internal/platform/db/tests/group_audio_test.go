//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"
	"quizzivy/internal/platform/db"
)

func TestSharedAudioRollbackPreservesWrittenReceipts(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, 39); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, 40); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, 39); err != nil {
		t.Fatal(err)
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("empty ledger rollback changed the existing schema")
	}
	if err := goose.UpTo(conn, dir, 40); err != nil {
		t.Fatal(err)
	}
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		group, _, _ := newFrozenGroup(t, tx, section)
		asset := newAudioAsset(t, tx, f.adminID)
		var recording, assignment, attempt string
		if err := tx.QueryRow(`INSERT INTO app.test_version_group_recordings(group_id,media_asset_id) VALUES($1,$2) RETURNING id`, group, asset).Scan(&recording); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(`INSERT INTO app.assignments(test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by)
            SELECT v.test_id,v.id,now()-interval '1 hour',now()+interval '1 hour',30,$2
            FROM app.test_version_sections s JOIN app.test_versions v ON v.id=s.test_version_id WHERE s.id=$1 RETURNING id`, section, f.adminID).Scan(&assignment); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(`INSERT INTO app.attempts(assignment_id,test_version_id,student_id,attempt_no,session_id,shuffle_seed,beacon_token_hash,started_at,deadline_at)
            SELECT id,test_version_id,$2,1,uuidv7(),1,sha256('beacon'::bytea),now(),now()+interval '30 minutes' FROM app.assignments WHERE id=$1 RETURNING id`, assignment, f.studentID).Scan(&attempt); err != nil {
			t.Fatal(err)
		}
		groupConstraint(t, tx, "attempt_group_audio_plays_plays_check", `INSERT INTO app.attempt_group_audio_plays(attempt_id,recording_id,plays,last_played_at) VALUES($1,$2,0,now())`, attempt, recording)
		groupConstraint(t, tx, "agar_counter_fk", `INSERT INTO app.attempt_group_audio_receipts(attempt_id,recording_id,play_id,session_id,received_at) VALUES($1,$2,uuidv7(),uuidv7(),now())`, attempt, recording)
		mustExec(t, tx, `INSERT INTO app.attempt_group_audio_plays(attempt_id,recording_id,plays,last_played_at) VALUES($1,$2,1,now())`, attempt, recording)
		mustExec(t, tx, `INSERT INTO app.attempt_group_audio_receipts(attempt_id,recording_id,play_id,session_id,received_at) VALUES($1,$2,uuidv7(),uuidv7(),now())`, attempt, recording)
		groupConstraint(t, tx, "attempt_group_audio_plays_recording_id_fkey", `DELETE FROM app.test_version_group_recordings WHERE id=$1`, recording)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	before = schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, dir, 39); err == nil {
		t.Fatal("rollback discarded listening evidence")
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("refused rollback mutated schema")
	}
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM app.attempt_group_audio_receipts`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("receipt after refused rollback: %d, %v", count, err)
	}
	version, err := goose.GetDBVersion(conn)
	if err != nil || version != 40 {
		t.Fatalf("version after refused rollback: %d, %v", version, err)
	}
}
