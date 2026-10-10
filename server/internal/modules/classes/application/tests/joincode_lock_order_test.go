//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

type teacherAction func(ctx context.Context, svc *application.Application, classID, teacherID string) error

func redeemWhileTheTeacherActs(t *testing.T, pool *pgxpool.Pool, act teacherAction) (classID string, joined domain.EnrolResult) {
	t.Helper()
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, newSvc(t, pool), classID, teacherID)
	m := newMember(t)
	dropUser(t, pool, m.Email)
	teacherPool, teacherPid := ownConnection(t)
	studentPool, studentPid := ownConnection(t)
	release, gate := holding(t, pool, `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Giữ chỗ', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, m.Email)

	type enrolment struct {
		result domain.EnrolResult
		err    error
	}
	enrol := make(chan enrolment, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := withKeys(studentPool, joinKeys).Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code})
		enrol <- enrolment{result, err}
	}()
	waitUntilBlocked(t, pool, studentPid, gate, "the redemption, holding the code row, on its new account's email")

	acted := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		acted <- act(ctx, withKeys(teacherPool, joinKeys), classID, teacherID)
	}()
	waitUntilBlocked(t, pool, teacherPid, studentPid, "the teacher, holding the class row, on the code row")
	release()

	redeemed := finished(t, "the redemption", enrol)
	if err := finished(t, "the teacher's action", acted); err != nil {
		t.Fatalf("the teacher's only attempt, which met a redemption holding the code row: %v", err)
	}
	if redeemed.err != nil || redeemed.result.Outcome != domain.PreviewOK {
		t.Fatalf("the redemption answered %+v (%v), want the student enrolled", redeemed.result, redeemed.err)
	}
	if n := memberCount(t, pool, classID); n != 2 {
		t.Errorf("the class has %d members, want the student it started with and the one who redeemed", n)
	}
	return classID, redeemed.result
}

func TestATeachersRotateOrRevokeAndARedemptionNeverDeadlock(t *testing.T) {
	pool := newPool(t)

	t.Run("rotate", func(t *testing.T) {
		classID, _ := redeemWhileTheTeacherActs(t, pool, func(ctx context.Context, svc *application.Application, classID, teacherID string) error {
			_, err := svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
			return err
		})
		if n := activeCodeCount(t, pool, classID); n != 1 {
			t.Errorf("the class holds %d active codes after the rotation, want one", n)
		}
		if !selfJoinEnabled(t, pool, classID) {
			t.Error("the class is closed after a rotation")
		}
	})

	t.Run("revoke", func(t *testing.T) {
		classID, _ := redeemWhileTheTeacherActs(t, pool, func(ctx context.Context, svc *application.Application, classID, teacherID string) error {
			_, err := svc.Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}})
			return err
		})
		if n := activeCodeCount(t, pool, classID); n != 0 {
			t.Errorf("the class holds %d active codes after the revocation, want none", n)
		}
		if selfJoinEnabled(t, pool, classID) {
			t.Error("the class is open after a revocation")
		}
	})
}

func TestARotateInFlightStillHoldsBackEveryOtherWriterOfTheClass(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := legacyClassRow(t, pool)
	rotatePool, rotatePid := ownConnection(t)
	revokePool, revokePid := ownConnection(t)
	updatePool, updatePid := ownConnection(t)
	jobPool, jobPid := ownConnection(t)
	release, gate := holding(t, pool, `SELECT 1 FROM app.users WHERE id = $1 FOR UPDATE`, teacherID)
	renamed := "Lớp đổi tên"

	rotate := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(rotatePool, joinKeys).Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
		rotate <- err
	}()
	waitUntilBlocked(t, pool, rotatePid, gate, "the teacher's Rotate, on its new row's creator")

	revoke := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(revokePool, joinKeys).Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}})
		revoke <- err
	}()
	waitUntilBlocked(t, pool, revokePid, rotatePid, "the teacher's Revoke, on the class row")

	update := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := withKeys(updatePool, joinKeys).Commands.Update.Handle(ctx, command.Update{
			ClassID: classID, Input: domain.UpdateInput{Name: &renamed}, Scope: access.Scope{UserID: teacherID}})
		update <- err
	}()
	waitUntilBlocked(t, pool, updatePid, revokePid, "a rename of the class, in line behind Revoke for the class row")

	job := startRotation(rotationOver(jobPool, nil, classID))
	waitUntilBlocked(t, pool, jobPid, updatePid, "the legacy rotation, in line behind the rename for the class row")
	release()

	for what, done := range map[string]chan error{"Rotate": rotate, "Revoke": revoke, "the rename": update} {
		if err := finished(t, what, done); err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
	if got := finished(t, "the legacy rotation", job); got.err != nil || got.run != (domain.LegacyRotation{Found: 1}) {
		t.Errorf("the legacy rotation answered %+v (%v), want the class found and left to its teacher's Rotate", got.run, got.err)
	}
	if n := activeCodeCount(t, pool, classID); n != 0 {
		t.Errorf("the class holds %d active codes, want none: Revoke ran after Rotate", n)
	}
	if selfJoinEnabled(t, pool, classID) {
		t.Error("the class is open, although Revoke ran after Rotate")
	}
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT name FROM app.classes WHERE id = $1`, classID).Scan(&name); err != nil || name != renamed {
		t.Errorf("the class is named %q (%v), want %q", name, err, renamed)
	}
	if n := rotationAudits(t, pool, classID); n != 0 {
		t.Errorf("%d legacy rotation audit rows for a class its teacher rotated first", n)
	}
}
