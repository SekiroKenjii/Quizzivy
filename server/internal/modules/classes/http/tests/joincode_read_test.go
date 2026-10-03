package http_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func answering(active domain.ActiveJoinCode, err error) classeshttp.Classes {
	return classeshttp.NewClasses(&application.Application{Queries: application.Queries{
		ActiveCode: cqrs.HandlerFunc[query.ActiveCode, domain.ActiveJoinCode](func(context.Context, query.ActiveCode) (domain.ActiveJoinCode, error) {
			return active, err
		}),
	}})
}

func TestAJoinCodeIsReadBackGroupedAndNeverCached(t *testing.T) {
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	uses := 3
	expires := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	issued := domain.IssuedCode{Hint: "P9QR", ExpiresAt: expires, MaxUses: &uses, UsesCount: 1}
	for label, c := range map[string]struct {
		active     domain.ActiveJoinCode
		wantCode   *string
		wantLegacy bool
	}{
		"a sealed code":                 {domain.ActiveJoinCode{IssuedCode: issued, Code: "K7M3P9QR"}, new("K7M3-P9QR"), false},
		"a legacy code":                 {domain.ActiveJoinCode{IssuedCode: issued, Legacy: true}, nil, true},
		"a code under a key no one has": {domain.ActiveJoinCode{IssuedCode: issued}, nil, false},
	} {
		response, err := answering(c.active, nil).GetJoinCode(ctx, openapi.GetJoinCodeRequestObject{Id: uuid.New()})
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		ok, isOK := response.(openapi.GetJoinCode200JSONResponse)
		if !isOK {
			t.Fatalf("%s answered %T", label, response)
		}
		if ok.Headers.CacheControl == nil || *ok.Headers.CacheControl != "no-store" {
			t.Errorf("%s: Cache-Control %v, want no-store", label, ok.Headers.CacheControl)
		}
		body := ok.Body
		if (body.Code == nil) != (c.wantCode == nil) || (body.Code != nil && *body.Code != *c.wantCode) {
			t.Errorf("%s: code %v, want %v", label, body.Code, c.wantCode)
		}
		if body.Legacy != c.wantLegacy || body.Hint != "P9QR" || !body.ExpiresAt.Equal(expires) || body.MaxUses == nil || *body.MaxUses != 3 || body.UsesCount != 1 {
			t.Errorf("%s: body %+v", label, body)
		}
	}
}

func TestAClassWithoutAnActiveCodeAnswers404(t *testing.T) {
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	for label, err := range map[string]error{"no active code": domain.ErrNoActiveCode, "another teacher's class": domain.ErrClassNotFound} {
		response, got := answering(domain.ActiveJoinCode{}, err).GetJoinCode(ctx, openapi.GetJoinCodeRequestObject{Id: uuid.New()})
		if got != nil {
			t.Fatalf("%s: %v", label, got)
		}
		if _, ok := response.(openapi.GetJoinCode404JSONResponse); !ok {
			t.Errorf("%s answered %T, want 404", label, response)
		}
	}
}
