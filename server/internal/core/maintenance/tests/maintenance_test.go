//go:build integration

package maintenance_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/maintenance"
	"quizzivy/internal/platform/db"
)

func setup(t *testing.T) (context.Context, pgx.Tx, string, string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	student := uuid.NewString()
	var attempt string
	err = tx.QueryRow(ctx, `
	 WITH student AS (
	   INSERT INTO app.users(id,email,full_name,role,password_hash,must_change_password)
	   VALUES($1::uuid,$1::text || '@example.com','Private student','student','hash',true) RETURNING id
	 ), teacher AS (
	   INSERT INTO app.users(email,full_name,role) VALUES($1::text || '-teacher@example.com','Teacher','admin') RETURNING id
	 ), test AS (
	   INSERT INTO app.tests(title,status,current_version,created_by)
	   SELECT 'Maintenance fixture','published',1,id FROM teacher RETURNING id,created_by
	 ), version AS (
	   INSERT INTO app.test_versions(test_id,version,total_points,published_by)
	   SELECT id,1,1,created_by FROM test RETURNING id,test_id,published_by
	 ), assignment AS (
	   INSERT INTO app.assignments(test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by,published_at)
	   SELECT test_id,id,now()-interval '1 hour',now()+interval '1 hour',45,published_by,now() FROM version
	   RETURNING id,test_version_id
	 )
	 INSERT INTO app.attempts(assignment_id,test_version_id,student_id,attempt_no,status,session_id,shuffle_seed,beacon_token_hash,deadline_at,submitted_at)
	 SELECT id,test_version_id,$1::uuid,1,'submitted',uuidv7(),1,sha256('fixture'::bytea),now()+interval '1 hour',now() FROM assignment
	 RETURNING id::text`, student).Scan(&attempt)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, tx, student, attempt
}

func TestRetentionUsesReceiptTimeAndPreservesAudit(t *testing.T) {
	ctx, tx, _, attempt := setup(t)
	if _, err := tx.Exec(ctx, `UPDATE app.assignments SET opens_at='1899-01-01',closes_at='1900-01-01' WHERE id=(SELECT assignment_id FROM app.attempts WHERE id=$1)`, attempt); err != nil {
		t.Fatal(err)
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.attempt_events(attempt_id,session_id,kind,occurred_at,received_at)
	 VALUES ($1,uuidv7(),'window_blur',now(), '1900-01-01'),
	        ($1,uuidv7(),'window_focus',now(), '1900-01-02'),
	        ($1,uuidv7(),'window_blur','1900-01-01',now()),
	        ($1,uuidv7(),'window_focus',now(),(CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - interval '13 months') AT TIME ZONE 'UTC')`, attempt)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := maintenance.RetainEvents(ctx, tx, false, 1)
	if err != nil || preview.Rows != 1 || preview.Applied {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	var before int
	if err := tx.QueryRow(ctx, `SELECT count(id) FROM app.audit_log`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	result, err := maintenance.RetainEvents(ctx, tx, true, 1)
	if err != nil || result.Rows != 1 || !result.Applied {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	var events, audits int
	if err := tx.QueryRow(ctx, `SELECT count(id) FROM app.attempt_events WHERE attempt_id=$1`, attempt).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(id) FROM app.audit_log`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if events != 3 || audits != before+1 {
		t.Fatalf("events=%d audits=%d before=%d", events, audits, before)
	}
	if _, err := maintenance.RetainEvents(ctx, tx, true, 0); err == nil {
		t.Fatal("accepted unbounded batch")
	}
}

func TestAnonymizationIsExplicitAtomicAndIdempotent(t *testing.T) {
	ctx, tx, student, attempt := setup(t)
	_, err := tx.Exec(ctx, `INSERT INTO app.user_identities(user_id,provider,provider_user_id,email_at_link) VALUES($1::uuid,'google',$1::text,$1::text || '@example.com')`, student)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.AnonymizeStudent(ctx, tx, student, false); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := tx.QueryRow(ctx, `SELECT full_name FROM app.users WHERE id=$1`, student).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Private student" {
		t.Fatal("dry run changed identity")
	}
	for range 2 {
		if _, err := maintenance.AnonymizeStudent(ctx, tx, student, true); err != nil {
			t.Fatal(err)
		}
	}
	var safe bool
	err = tx.QueryRow(ctx, `SELECT password_hash IS NULL AND NOT must_change_password AND disabled_at IS NOT NULL
	  AND email = 'anonymous-' || id::text || '@anonymous.invalid'
	  AND NOT EXISTS(SELECT 1 FROM app.user_identities WHERE user_id=$1)
	  AND EXISTS(SELECT 1 FROM app.attempts WHERE id=$2)
	  AND (SELECT count(id) FROM app.audit_log WHERE entity_id=$1 AND action='student.anonymized')=1
	  FROM app.users WHERE id=$1`, student, attempt).Scan(&safe)
	if err != nil || !safe {
		t.Fatalf("identity/credentials/history invariant: safe=%v err=%v", safe, err)
	}
}

func TestAnonymizationRejectsAdminAndActiveAttempt(t *testing.T) {
	ctx, tx, student, attempt := setup(t)
	if _, err := tx.Exec(ctx, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL WHERE id=$1`, attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.AnonymizeStudent(ctx, tx, student, true); err == nil {
		t.Fatal("anonymized active student")
	}
	var teacher string
	if err := tx.QueryRow(ctx, `SELECT created_by::text FROM app.assignments WHERE id=(SELECT assignment_id FROM app.attempts WHERE id=$1)`, attempt).Scan(&teacher); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.AnonymizeStudent(ctx, tx, teacher, true); err == nil {
		t.Fatal("anonymized admin")
	}
}

func TestApplicationRoleStillCannotPruneEvents(t *testing.T) {
	ctx, tx, _, _ := setup(t)
	var forbidden bool
	if err := tx.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app','app.attempt_events','DELETE') OR has_table_privilege('quizzivy_app','app.audit_log','DELETE') OR has_table_privilege('quizzivy_app','app.attempt_events','UPDATE') OR has_table_privilege('quizzivy_app','app.audit_log','UPDATE')`).Scan(&forbidden); err != nil {
		t.Fatal(err)
	}
	if forbidden {
		t.Fatal("app role can mutate append-only records")
	}
}

func TestRetentionKeepsEventsUntilTheAssignmentCloses(t *testing.T) {
	ctx, tx, _, attempt := setup(t)
	if _, err := tx.Exec(ctx, `INSERT INTO app.attempt_events(attempt_id,session_id,kind,occurred_at,received_at) VALUES($1,uuidv7(),'window_blur','1900-01-01','1900-01-01')`, attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.RetainEvents(ctx, tx, true, 10000); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.attempt_events WHERE attempt_id=$1)`, attempt).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("pruned an open assignment's evidence")
	}
}
