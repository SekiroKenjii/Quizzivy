package join_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/join"
)

// /join/preview is unauthenticated and takes a bearer secret. What it returns
// on success, and what it refuses to distinguish on failure, are both §6.5
// requirements rather than presentation choices.

func issueCode(t *testing.T, svc *join.Service, classID, teacherID string) string {
	t.Helper()
	rotated, err := svc.Rotate(context.Background(), join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}
	return rotated.Code
}

func TestPreviewReturnsTheClassAndTheTeacher(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	got, err := svc.Preview(context.Background(), code)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if got.Outcome != join.PreviewOK {
		t.Fatalf("outcome = %v, want PreviewOK", got.Outcome)
	}
	if got.ClassID != classID {
		t.Errorf("classId = %s, want %s", got.ClassID, classID)
	}
	if got.ClassName == "" || got.TeacherName == "" {
		t.Errorf("className = %q teacherName = %q; both are required", got.ClassName, got.TeacherName)
	}
}

func TestPreviewAcceptsTheCodeHoweverItWasTyped(t *testing.T) {
	// A student reads this off a QR code, a message, or a whiteboard.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID) // grouped XXXX-XXXX

	plain := join.Normalize(code)
	for _, typed := range []string{code, plain, "  " + plain + "  ", lower(plain), lower(code)} {
		got, err := svc.Preview(context.Background(), typed)
		if err != nil {
			t.Fatalf("Preview(%q): %v", typed, err)
		}
		if got.Outcome != join.PreviewOK {
			t.Errorf("Preview(%q) = %v, want PreviewOK", typed, got.Outcome)
		}
	}
}

func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func TestTheFourRefusalsAreDistinguishable(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	ctx := context.Background()

	t.Run("invalid", func(t *testing.T) {
		got, err := svc.Preview(ctx, "ZZZZ-ZZZZ")
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != join.PreviewInvalid {
			t.Errorf("outcome = %v, want PreviewInvalid", got.Outcome)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		classID, teacherID, _ := makeClass(t, pool)
		code := issueCode(t, svc, classID, teacherID)
		if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`UPDATE app.classes SET self_join_enabled = true WHERE id = $1`, classID); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Preview(ctx, code)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != join.PreviewRevoked {
			t.Errorf("outcome = %v, want PreviewRevoked", got.Outcome)
		}
	})

	t.Run("expired", func(t *testing.T) {
		classID, teacherID, _ := makeClass(t, pool)
		code := issueCode(t, svc, classID, teacherID)
		if _, err := pool.Exec(ctx,
			`UPDATE app.class_join_codes
			    SET created_at = now() - interval '2 days',
			        expires_at = now() - interval '1 day'
			  WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Preview(ctx, code)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != join.PreviewExpired {
			t.Errorf("outcome = %v, want PreviewExpired", got.Outcome)
		}
	})

	t.Run("exhausted", func(t *testing.T) {
		classID, teacherID, _ := makeClass(t, pool)
		one := 1
		rotated, err := svc.Rotate(ctx, join.RotateRequest{
			ClassID: classID, ActorUserID: teacherID, MaxUses: &one})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`UPDATE app.class_join_codes SET uses_count = max_uses
			  WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Preview(ctx, rotated.Code)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != join.PreviewExhausted {
			t.Errorf("outcome = %v, want PreviewExhausted", got.Outcome)
		}
	})
}

func TestAClosedClassIsIndistinguishableFromANonexistentOne(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, classID); err != nil {
		t.Fatal(err)
	}

	closed, err := svc.Preview(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	nonexistent, err := svc.Preview(ctx, "ZZZZ-ZZZZ")
	if err != nil {
		t.Fatal(err)
	}
	if closed.Outcome != nonexistent.Outcome {
		t.Errorf("closed class = %v, nonexistent = %v; they must be identical",
			closed.Outcome, nonexistent.Outcome)
	}
	if closed.ClassName != "" || closed.ClassID != "" {
		t.Errorf("a refusal carried class data: %+v", closed)
	}
}

func TestAClosedClassLeaksNothingEvenWhenTheCodeIsAlsoExpired(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = false WHERE id = $1;`, classID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.class_join_codes
		    SET created_at = now() - interval '2 days',
		        expires_at = now() - interval '1 day'
		  WHERE class_id = $1`, classID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Preview(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != join.PreviewInvalid {
		t.Errorf("outcome = %v, want PreviewInvalid", got.Outcome)
	}
}

func TestNoRefusalCarriesClassData(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{code, "ZZZZ-ZZZZ", "", "not a code at all"} {
		got, err := svc.Preview(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome == join.PreviewOK {
			t.Fatalf("Preview(%q) unexpectedly succeeded", input)
		}
		if got.ClassID != "" || got.ClassName != "" || got.TeacherName != "" {
			t.Errorf("Preview(%q) refusal carried %+v", input, got)
		}
	}
}

func TestPreviewUsesTheServiceClockForExpiry(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	svc.SetClock(func() time.Time { return time.Now().AddDate(0, 0, join.DefaultExpiryDays+1) })
	got, err := svc.Preview(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != join.PreviewExpired {
		t.Errorf("outcome = %v, want PreviewExpired once the clock passes the expiry", got.Outcome)
	}
}
