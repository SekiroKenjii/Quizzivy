package http_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func recordingReads(seen map[string]access.Scope) *application.Application {
	return &application.Application{Queries: application.Queries{
		List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
			seen["listTests"] = q.Input.Scope
			return query.ListResult{}, nil
		}),
		Facets: cqrs.HandlerFunc[query.Facets, domain.StatusFacets](func(_ context.Context, q query.Facets) (domain.StatusFacets, error) {
			seen["listTests facets"] = q.Input.Scope
			return domain.StatusFacets{}, nil
		}),
		Tags: cqrs.HandlerFunc[query.Tags, []string](func(_ context.Context, q query.Tags) ([]string, error) {
			seen["listTests tags"] = q.Input.Scope
			return nil, nil
		}),
		Groups: cqrs.HandlerFunc[query.Groups, query.GroupsResult](func(_ context.Context, q query.Groups) (query.GroupsResult, error) {
			seen["listQuestionGroups"] = q.Input.Scope
			return query.GroupsResult{}, nil
		}),
		Get: cqrs.HandlerFunc[query.Get, domain.Test](func(_ context.Context, q query.Get) (domain.Test, error) {
			seen["getTest"] = q.Scope
			return domain.Test{}, domain.ErrNotFound
		}),
		Group: cqrs.HandlerFunc[query.Group, domain.StoredGroup](func(_ context.Context, q query.Group) (domain.StoredGroup, error) {
			seen["getQuestionGroup"] = q.Scope
			return domain.StoredGroup{}, domain.ErrNotFound
		}),
		ListVersions: cqrs.HandlerFunc[query.ListVersions, []domain.Version](func(_ context.Context, q query.ListVersions) ([]domain.Version, error) {
			seen["listTestVersions"] = q.Scope
			return nil, nil
		}),
		Preview: cqrs.HandlerFunc[query.Preview, query.PreviewResult](func(_ context.Context, q query.Preview) (query.PreviewResult, error) {
			seen["previewTest"] = q.Scope
			return query.PreviewResult{}, domain.ErrNotPublished
		}),
	}, Commands: application.Commands{
		Publish: cqrs.HandlerFunc[command.Publish, domain.Version](func(_ context.Context, c command.Publish) (domain.Version, error) {
			seen["publishTest"] = c.Request.Scope
			return domain.Version{}, domain.ErrDraftNotFound
		}),
	}}
}

func TestTheTestAndGroupListsRunInTheCallersOwnScopeAndReadsByIdInTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			seen := map[string]access.Scope{}
			h := testshttp.NewTests(recordingReads(seen), authoringMedia{})
			ctx := contextAs(t, principal)
			id := uuid.New()
			for op, call := range map[string]func() (any, error){
				"listTests": func() (any, error) { return h.ListTests(ctx, openapi.ListTestsRequestObject{}) },
				"listQuestionGroups": func() (any, error) {
					return h.ListQuestionGroups(ctx, openapi.ListQuestionGroupsRequestObject{})
				},
				"getTest": func() (any, error) { return h.GetTest(ctx, openapi.GetTestRequestObject{Id: id}) },
				"getQuestionGroup": func() (any, error) {
					return h.GetQuestionGroup(ctx, openapi.GetQuestionGroupRequestObject{Id: id})
				},
				"listTestVersions": func() (any, error) {
					return h.ListTestVersions(ctx, openapi.ListTestVersionsRequestObject{Id: id})
				},
				"previewTest": func() (any, error) {
					return h.PreviewTest(ctx, openapi.PreviewTestRequestObject{Id: id})
				},
				"publishTest": func() (any, error) {
					return h.PublishTest(ctx, openapi.PublishTestRequestObject{Id: id})
				},
			} {
				if _, err := call(); err != nil {
					t.Fatalf("%s: %v", op, err)
				}
			}
			own := access.Scope{UserID: principal.UserID}
			for _, op := range []string{"listTests", "listTests facets", "listTests tags", "listQuestionGroups"} {
				if got, ok := seen[op]; !ok || got != own {
					t.Errorf("%s ran in %+v (reached %v), want the caller's own rows %+v", op, got, ok, own)
				}
			}
			byID := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			for _, op := range []string{"getTest", "getQuestionGroup", "listTestVersions", "previewTest", "publishTest"} {
				if got, ok := seen[op]; !ok || got != byID {
					t.Errorf("%s ran in %+v (reached %v), want the caller's scope %+v", op, got, ok, byID)
				}
			}
		})
	}
}
