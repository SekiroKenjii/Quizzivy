//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

func TestATeacherReadsBackTheCodeTheyIssued(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	rotated, err := svc.Commands.Rotate.Handle(context.Background(), command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}})
	if err != nil {
		t.Fatal(err)
	}
	canonical := domain.JoinCodes.Normalize(rotated.Code)

	for who, scope := range map[string]access.Scope{"the teacher": {UserID: teacherID}, "scope.all": everyone} {
		got, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID})
		if err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if got.Code != canonical || got.Legacy || got.Hint != rotated.Hint || !got.ExpiresAt.Equal(rotated.ExpiresAt.Truncate(time.Microsecond)) ||
			got.MaxUses == nil || *got.MaxUses != *rotated.MaxUses || got.UsesCount != 0 || got.ClassID != classID {
			t.Errorf("%s read %+v, want the code %s it issued", who, got, canonical)
		}
	}
}

func TestAnotherTeachersJoinCodeAnswersAsAMissingClass(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	_, otherTeacher, _ := makeClassRow(t, pool)
	issueCode(t, svc, classID, teacherID)
	for who, scope := range map[string]access.Scope{"another teacher": {UserID: otherTeacher}, "no one": {}} {
		if _, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID}); !errors.Is(err, domain.ErrClassNotFound) {
			t.Errorf("%s: %v, want ErrClassNotFound", who, err)
		}
	}
	if _, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: everyone, ClassID: "0193a000-0000-7000-8000-00000000ffff"}); !errors.Is(err, domain.ErrClassNotFound) {
		t.Errorf("a missing class: %v, want ErrClassNotFound", err)
	}
}

func TestAClassWithoutALiveCodeHasNoneToRead(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	scope := access.Scope{UserID: teacherID}
	if _, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID}); !errors.Is(err, domain.ErrNoActiveCode) {
		t.Errorf("never issued: %v, want ErrNoActiveCode", err)
	}
	issueCode(t, svc, classID, teacherID)
	if _, err := svc.Commands.Revoke.Handle(context.Background(), command.Revoke{Request: domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID}); !errors.Is(err, domain.ErrNoActiveCode) {
		t.Errorf("revoked: %v, want ErrNoActiveCode", err)
	}
}

func TestALegacyCodeIsReadAsLegacyWithoutItsCode(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	grouped := legacyCode(t, pool, classID, teacherID)
	got, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: access.Scope{UserID: teacherID}, ClassID: classID})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Legacy || got.Code != "" || got.Hint != domain.JoinCodes.Hint(domain.JoinCodes.Normalize(grouped)) {
		t.Errorf("a legacy code read as %+v", got)
	}
}

func TestACodeUnderAKeyThisServerLacksIsReadWithoutItsCode(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, withKeys(pool, mustJoinCodeKeys(joinKeyA, nil)), classID, teacherID)
	scope := access.Scope{UserID: teacherID}

	lost, err := withKeys(pool, mustJoinCodeKeys(joinKeyB, nil)).Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID})
	if err != nil || lost.Code != "" || lost.Legacy {
		t.Errorf("with the sealing key gone: %+v (%v), want no code and not legacy", lost, err)
	}
	rotating, err := withKeys(pool, mustJoinCodeKeys(joinKeyB, joinKeyA)).Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID})
	if err != nil || rotating.Code != domain.JoinCodes.Normalize(code) {
		t.Errorf("with the sealing key as previous: %+v (%v)", rotating, err)
	}
}

func TestATamperedCiphertextIsAnErrorNotACode(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	issueCode(t, svc, classID, teacherID)
	if _, err := pool.Exec(context.Background(), `
		UPDATE app.class_join_codes
		   SET code_ciphertext = set_byte(code_ciphertext, 20, get_byte(code_ciphertext, 20) # 1)
		 WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: access.Scope{UserID: teacherID}, ClassID: classID})
	if err == nil {
		t.Errorf("a tampered ciphertext read as %+v", got)
	}
}
