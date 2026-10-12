//go:build integration

package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
)

func setArchived(t *testing.T, svc *application.Application, classID, teacherID string, archived bool) {
	t.Helper()
	if _, err := svc.Commands.Archive.Handle(context.Background(), command.Archive{
		ClassID: classID, Archived: archived, Actor: actor.Actor{ID: teacherID},
	}); err != nil {
		t.Fatalf("archive=%v: %v", archived, err)
	}
}

func rotateAs(svc *application.Application, classID, teacherID string, all bool) error {
	_, err := svc.Commands.Rotate.Handle(context.Background(), command.Rotate{Request: domain.RotateRequest{
		ClassID: classID, ActorUserID: teacherID, All: all,
	}})
	return err
}

func joinCodeAudits(t *testing.T, pool *pgxpool.Pool, actorID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1 AND entity = 'class_join_code'`,
		actorID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAnArchivedClassTakesNoNewJoinCodeAndKeepsItsCurrentOne(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	issued := issueCode(t, svc, classID, teacherID)
	setArchived(t, svc, classID, teacherID, true)
	rowsBefore, auditsBefore, openBefore := codeRows(t, pool, classID), joinCodeAudits(t, pool, teacherID), selfJoinEnabled(t, pool, classID)

	if err := rotateAs(svc, classID, teacherID, false); !errors.Is(err, domain.ErrClassArchived) {
		t.Fatalf("rotating the code of an archived class answered %v, want ErrClassArchived", err)
	}

	if rowsAfter := codeRows(t, pool, classID); !reflect.DeepEqual(rowsAfter, rowsBefore) {
		t.Errorf("the refused rotation changed the class's code rows:\n%+v\nwas\n%+v", rowsAfter, rowsBefore)
	}
	if n := joinCodeAudits(t, pool, teacherID); n != auditsBefore {
		t.Errorf("the refused rotation wrote %d audit rows", n-auditsBefore)
	}
	if selfJoinEnabled(t, pool, classID) != openBefore {
		t.Error("the refused rotation changed the class's self-join switch")
	}
	if kept := domain.JoinCodes.Format(currentCode(t, svc, classID).Code); kept != issued {
		t.Errorf("the class's code reads back as %s, want the one it had, %s", kept, issued)
	}

	setArchived(t, svc, classID, teacherID, false)
	reissued := issueCode(t, svc, classID, teacherID)
	if reissued == issued {
		t.Error("the restored class was issued the code it had before")
	}
	if n := activeCodeCount(t, pool, classID); n != 1 {
		t.Errorf("the restored class holds %d active codes after a rotation, want one", n)
	}
}

func TestAnArchivedClassWithoutACodeIsIssuedNone(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	setArchived(t, svc, classID, teacherID, true)

	if err := rotateAs(svc, classID, teacherID, false); !errors.Is(err, domain.ErrClassArchived) {
		t.Fatalf("rotating an archived class that has no code answered %v, want ErrClassArchived", err)
	}
	if rows := codeRows(t, pool, classID); len(rows) != 0 {
		t.Errorf("the class holds %d code rows, want none", len(rows))
	}
}

func TestRevokingTheCodeOfAnArchivedClassIsAllowed(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	issueCode(t, svc, classID, teacherID)
	setArchived(t, svc, classID, teacherID, true)

	if _, err := svc.Commands.Revoke.Handle(context.Background(), command.Revoke{Request: domain.RevokeRequest{
		ClassID: classID, ActorUserID: teacherID,
	}}); err != nil {
		t.Fatalf("revoking the code of an archived class: %v", err)
	}
	if n := activeCodeCount(t, pool, classID); n != 0 {
		t.Errorf("the archived class holds %d active codes after the revocation, want none", n)
	}
	if selfJoinEnabled(t, pool, classID) {
		t.Error("the archived class is open after a revocation")
	}
}

func TestOnlyAClassTheCallerReachesAnswersArchived(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	_, otherTeacherID, _ := makeClassRow(t, pool)
	setArchived(t, svc, classID, teacherID, true)
	rowsBefore := codeRows(t, pool, classID)

	for name, c := range map[string]struct {
		classID, actorID string
		all              bool
		want             error
	}{
		"the owner":               {classID, teacherID, false, domain.ErrClassArchived},
		"an Admin with scope.all": {classID, otherTeacherID, true, domain.ErrClassArchived},
		"another teacher":         {classID, otherTeacherID, false, domain.ErrClassNotFound},
		"a missing class":         {uuid.NewString(), teacherID, false, domain.ErrClassNotFound},
	} {
		if err := rotateAs(svc, c.classID, c.actorID, c.all); !errors.Is(err, c.want) {
			t.Errorf("%s: rotating answered %v, want %v", name, err, c.want)
		}
	}
	if rowsAfter := codeRows(t, pool, classID); !reflect.DeepEqual(rowsAfter, rowsBefore) {
		t.Errorf("a refused rotation changed the class's code rows:\n%+v\nwas\n%+v", rowsAfter, rowsBefore)
	}
}

func TestARotationThatMeetsAnArchiveInFlightSeesItAndRefuses(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClassRow(t, pool)
	issued := issueCode(t, newSvc(t, pool), classID, teacherID)
	rowsBefore := codeRows(t, pool, classID)
	rotatePool, rotatePid := ownConnection(t)
	archiving := newHeldTx(t, pool, `UPDATE app.classes SET archived_at = now() WHERE id = $1`, classID)

	rotated := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(rotatePool, joinKeys).Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
		rotated <- err
	}()
	waitUntilBlocked(t, pool, rotatePid, archiving.pid, "the rotation, on the class row an archive holds")
	if err := archiving.commit(); err != nil {
		t.Fatalf("commit the archive: %v", err)
	}

	if err := finished(t, "the rotation", rotated); !errors.Is(err, domain.ErrClassArchived) {
		t.Errorf("the rotation that waited for an archive answered %v, want ErrClassArchived", err)
	}
	if rowsAfter := codeRows(t, pool, classID); !reflect.DeepEqual(rowsAfter, rowsBefore) {
		t.Errorf("the rotation changed the class's code rows:\n%+v\nwas\n%+v", rowsAfter, rowsBefore)
	}
	if kept := domain.JoinCodes.Format(currentCode(t, newSvc(t, pool), classID).Code); kept != issued {
		t.Errorf("the class's code reads back as %s, want the one it had, %s", kept, issued)
	}
}

func TestAnArchiveThatMeetsARotationInFlightWaitsForIt(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClassRow(t, pool)
	rotatePool, rotatePid := ownConnection(t)
	archivePool, archivePid := ownConnection(t)
	release, gate := holding(t, pool, `SELECT 1 FROM app.users WHERE id = $1 FOR UPDATE`, teacherID)

	rotated := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(rotatePool, joinKeys).Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
		rotated <- err
	}()
	waitUntilBlocked(t, pool, rotatePid, gate, "the rotation, holding the class row, on its new code's creator")

	archived := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(archivePool, joinKeys).Commands.Archive.Handle(ctx, command.Archive{ClassID: classID, Archived: true, Actor: actor.Actor{ID: teacherID}})
		archived <- err
	}()
	waitUntilBlocked(t, pool, archivePid, rotatePid, "the archive, on the class row the rotation holds")
	release()

	if err := finished(t, "the rotation", rotated); err != nil {
		t.Errorf("the rotation that began before the archive: %v", err)
	}
	if err := finished(t, "the archive", archived); err != nil {
		t.Errorf("the archive that waited for the rotation: %v", err)
	}
	if n := activeCodeCount(t, pool, classID); n != 1 {
		t.Errorf("the class holds %d active codes, want the one the rotation issued first", n)
	}
	var archivedAt *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT archived_at FROM app.classes WHERE id = $1`, classID).Scan(&archivedAt); err != nil || archivedAt == nil {
		t.Errorf("the class's archived_at is %v (%v), want it set by the archive that ran second", archivedAt, err)
	}
}

func TestTheCodeOfAnArchivedClassDoesNotRedeem(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	if got := previewOf(t, svc, code); got.Outcome != domain.PreviewOK {
		t.Fatalf("a live class's code previews as %+v, want PreviewOK", got)
	}
	setArchived(t, svc, classID, teacherID, true)

	unknown := refusalOf(t, pool, svc, "ZZZZ-ZZZZ")
	if unknown != refusedAs(domain.PreviewInvalid) {
		t.Fatalf("an unknown code answers %+v", unknown)
	}
	if got := refusalOf(t, pool, svc, code); got != unknown {
		t.Errorf("the code of an archived class answers %+v, want what an unknown code answers", got)
	}
	if n := memberCount(t, pool, classID); n != 1 {
		t.Errorf("the archived class has %d members, want the student it started with", n)
	}
}
