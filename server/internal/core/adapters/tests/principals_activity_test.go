package adapters_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/core/adapters"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	identitycommand "quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func principalsWith(resolve func(accessquery.ResolvePrincipal) (access.Principal, error), stamped *[]string) adapters.Principals {
	return adapters.Principals{
		Query: cqrs.HandlerFunc[accessquery.ResolvePrincipal, access.Principal](func(_ context.Context, q accessquery.ResolvePrincipal) (access.Principal, error) {
			return resolve(q)
		}),
		Activity: cqrs.HandlerFunc[identitycommand.StampActivity, cqrs.Nothing](func(_ context.Context, c identitycommand.StampActivity) (cqrs.Nothing, error) {
			*stamped = append(*stamped, c.UserID)
			return cqrs.Nothing{}, nil
		}),
	}
}

func TestAResolvedUserIsStampedAsActive(t *testing.T) {
	var stamped []string
	p := principalsWith(func(q accessquery.ResolvePrincipal) (access.Principal, error) {
		return access.Principal{UserID: q.UserID}, nil
	}, &stamped)
	if _, err := p.Resolve(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if len(stamped) != 1 || stamped[0] != "user-1" {
		t.Errorf("stamped %v, want [user-1]", stamped)
	}
}

func TestADisabledOrUnknownUserIsNotStamped(t *testing.T) {
	var stamped []string
	p := principalsWith(func(q accessquery.ResolvePrincipal) (access.Principal, error) {
		if q.UserID == "ghost" {
			return access.Principal{}, accessdomain.ErrUnknownUser
		}
		return access.Principal{UserID: q.UserID, Disabled: true}, nil
	}, &stamped)
	if _, err := p.Resolve(context.Background(), "off"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Resolve(context.Background(), "ghost"); !errors.Is(err, httpx.ErrUnknownPrincipal) {
		t.Fatalf("ghost: %v", err)
	}
	if len(stamped) != 0 {
		t.Errorf("stamped %v, want nobody", stamped)
	}
}

func TestWithoutAnActivityPortResolvingStillWorks(t *testing.T) {
	p := adapters.Principals{Query: cqrs.HandlerFunc[accessquery.ResolvePrincipal, access.Principal](func(_ context.Context, q accessquery.ResolvePrincipal) (access.Principal, error) {
		return access.Principal{UserID: q.UserID}, nil
	})}
	if got, err := p.Resolve(context.Background(), "user-1"); err != nil || got.UserID != "user-1" {
		t.Fatalf("%v, %v", got, err)
	}
}
