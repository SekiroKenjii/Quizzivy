//go:build integration

package application_test

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
)

func forgetMemberAt(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		owned := `SELECT id FROM app.users WHERE email = $1`
		for _, stmt := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id IN (` + owned + `)`,
			`DELETE FROM app.class_members WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.user_identities WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.users WHERE email = $1`,
		} {
			if _, err := pool.Exec(context.Background(), stmt, email); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
}

func accountsAt(t *testing.T, pool *pgxpool.Pool, email, subject string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.users WHERE email = $1)
		     + (SELECT count(*) FROM app.user_identities WHERE provider_user_id = $2)`, email, subject).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func isMember(t *testing.T, pool *pgxpool.Pool, classID, userID string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.class_members WHERE class_id = $1::uuid AND user_id = $2::uuid`, classID, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func joiningOver(t *testing.T, pool *pgxpool.Pool) (*application.Application, *atomic.Bool) {
	t.Helper()
	var on atomic.Bool
	on.Store(true)
	return newSvc(t, pool).WithSignInPolicy(switchableJoining(&on)), &on
}

func TestJoiningOffAnswersEveryStateOfARealCodeInThePreviewAsAnUnknownOne(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	for name, c := range map[string]struct {
		prepare func(t *testing.T, svc *application.Application, classID, teacherID string) string
		want    domain.PreviewOutcome
	}{
		"a live code": {
			func(t *testing.T, svc *application.Application, classID, teacherID string) string {
				return issueCode(t, svc, classID, teacherID)
			},
			domain.PreviewOK,
		},
		"an expired code": {
			func(t *testing.T, svc *application.Application, classID, teacherID string) string {
				code := issueCode(t, svc, classID, teacherID)
				if _, err := pool.Exec(ctx, `UPDATE app.class_join_codes
					SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day'
					WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
					t.Fatal(err)
				}
				return code
			},
			domain.PreviewExpired,
		},
		"a revoked code": {
			func(t *testing.T, svc *application.Application, classID, teacherID string) string {
				code := issueCode(t, svc, classID, teacherID)
				if _, err := svc.Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}}); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `UPDATE app.classes SET self_join_enabled = true WHERE id = $1`, classID); err != nil {
					t.Fatal(err)
				}
				return code
			},
			domain.PreviewRevoked,
		},
		"a code with no uses left": {
			func(t *testing.T, svc *application.Application, classID, teacherID string) string {
				one := 1
				rotated, err := svc.Commands.Rotate.Handle(ctx, command.Rotate{Request: domain.RotateRequest{ClassID: classID, ActorUserID: teacherID, MaxUses: &one}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `UPDATE app.class_join_codes SET uses_count = max_uses WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
					t.Fatal(err)
				}
				return rotated.Code
			},
			domain.PreviewExhausted,
		},
		"the code of a class closed to joining": {
			func(t *testing.T, svc *application.Application, classID, teacherID string) string {
				code := issueCode(t, svc, classID, teacherID)
				if _, err := pool.Exec(ctx, `UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, classID); err != nil {
					t.Fatal(err)
				}
				return code
			},
			domain.PreviewInvalid,
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc, joining := joiningOver(t, pool)
			classID, teacherID, _ := makeClassRow(t, pool)
			code := c.prepare(t, svc, classID, teacherID)

			if got := previewing(t, svc, code).Outcome; got != c.want {
				t.Fatalf("with joining on, the preview answered %v, want %v", got, c.want)
			}
			unknown := previewing(t, svc, "ZZZZ-ZZZZ")
			joining.Store(false)

			refused := previewing(t, svc, code)

			if !reflect.DeepEqual(refused, unknown) || refused.Outcome != domain.PreviewInvalid {
				t.Errorf("with joining off, the preview answered %+v, want what an unknown code answers, %+v", refused, unknown)
			}
		})
	}
}

func TestJoiningOffEnrolsNoNewMemberAndSpendsNoUseOfARealCode(t *testing.T) {
	pool := newPool(t)
	svc, joining := joiningOver(t, pool)
	notices := &joinNotices{}
	svc = svc.WithNotifier(notices)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	m := newMember(t)
	forgetMemberAt(t, pool, m.Email)
	enrol := command.EnrolNewMember{Member: m, Code: code, Meta: domain.Meta{IP: "203.0.113.44", UserAgent: "go-test"}}
	unknown, err := svc.Commands.EnrolNewMember.Handle(context.Background(), command.EnrolNewMember{Member: m, Code: "ZZZZ-ZZZZ"})
	if err != nil {
		t.Fatal(err)
	}
	members, uses := memberCount(t, pool, classID), usesCount(t, pool, classID)
	joining.Store(false)

	refused, err := svc.Commands.EnrolNewMember.Handle(context.Background(), enrol)

	if err != nil {
		t.Fatalf("EnrolNewMember: %v", err)
	}
	if !reflect.DeepEqual(refused, unknown) || refused.Outcome != domain.PreviewInvalid {
		t.Errorf("answered %+v, want what an unknown code answers, %+v", refused, unknown)
	}
	if n := accountsAt(t, pool, m.Email, m.ProviderUserID); n != 0 {
		t.Errorf("the refusal left %d account rows behind", n)
	}
	if got := memberCount(t, pool, classID); got != members {
		t.Errorf("members = %d, want %d", got, members)
	}
	if got := usesCount(t, pool, classID); got != uses {
		t.Errorf("uses_count = %d, want %d", got, uses)
	}
	if notices.count() != 0 {
		t.Errorf("the teacher %s was told of %d joins", teacherID, notices.count())
	}

	joining.Store(true)
	taken, err := svc.Commands.EnrolNewMember.Handle(context.Background(), enrol)
	if err != nil || taken.Outcome != domain.PreviewOK {
		t.Fatalf("with joining back on: %+v, %v", taken, err)
	}
	if got := memberCount(t, pool, classID); got != members+1 {
		t.Errorf("members = %d after joining came back, want %d", got, members+1)
	}
}

func TestJoiningOffEnrolsNoExistingMemberAndSpendsNoUseOfARealCode(t *testing.T) {
	pool := newPool(t)
	svc, joining := joiningOver(t, pool)
	notices := &joinNotices{}
	svc = svc.WithNotifier(notices)
	classID, teacherID, _ := makeClassRow(t, pool)
	studentID := outsider(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	unknown, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: "ZZZZ-ZZZZ"})
	if err != nil {
		t.Fatal(err)
	}
	members, uses := memberCount(t, pool, classID), usesCount(t, pool, classID)
	joining.Store(false)

	refused, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code})

	if err != nil {
		t.Fatalf("EnrolExisting: %v", err)
	}
	if !reflect.DeepEqual(refused, unknown) || refused.Outcome != domain.PreviewInvalid {
		t.Errorf("answered %+v, want what an unknown code answers, %+v", refused, unknown)
	}
	if isMember(t, pool, classID, studentID) {
		t.Error("the student was enrolled")
	}
	if got := memberCount(t, pool, classID); got != members {
		t.Errorf("members = %d, want %d", got, members)
	}
	if got := usesCount(t, pool, classID); got != uses {
		t.Errorf("uses_count = %d, want %d", got, uses)
	}
	if notices.count() != 0 {
		t.Errorf("the teacher was told of %d joins", notices.count())
	}

	joining.Store(true)
	taken, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code})
	if err != nil || taken.Outcome != domain.PreviewOK || !isMember(t, pool, classID, studentID) {
		t.Fatalf("with joining back on: %+v, %v", taken, err)
	}
}

func TestJoiningOnLeavesTheRealPreviewAsItWas(t *testing.T) {
	pool := newPool(t)
	svc, _ := joiningOver(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	got, err := svc.Queries.Preview.Handle(context.Background(), query.Preview{Code: code})

	if err != nil || got.Outcome != domain.PreviewOK || got.ClassID != classID || got.ClassName == "" || got.TeacherName == "" {
		t.Fatalf("Preview: %+v, %v", got, err)
	}
}
