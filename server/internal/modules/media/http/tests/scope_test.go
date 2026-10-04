package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type resolved map[string]access.Principal

func (r resolved) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func contextAs(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/media": access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/media", nil)
	req.Pattern = "GET /teacher/media"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func principals() map[string]access.Principal {
	return map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentMediaWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	}
}

func TestTheLibraryListsAndTotalsTheCallersOwnAssets(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID}
			var listed, totalled, faceted, measured access.Scope
			app := &application.Application{Queries: application.Queries{
				List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
					listed = q.Input.Scope
					return query.ListResult{}, nil
				}),
				TotalBytes: cqrs.HandlerFunc[query.TotalBytes, int64](func(_ context.Context, q query.TotalBytes) (int64, error) {
					totalled = q.Scope
					return 0, nil
				}),
				Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(_ context.Context, q query.Facets) (domain.Facets, error) {
					faceted = q.Input.Scope
					return domain.Facets{}, nil
				}),
				Usage: cqrs.HandlerFunc[query.Usage, domain.Usage](func(_ context.Context, q query.Usage) (domain.Usage, error) {
					measured = q.Scope
					return domain.Usage{}, nil
				}),
			}}
			if _, err := mediahttp.NewMedia(app).ListMedia(contextAs(t, principal), openapi.ListMediaRequestObject{}); err != nil {
				t.Fatal(err)
			}
			if listed != want || totalled != want {
				t.Errorf("the library listed in %+v and totalled in %+v, want the caller's own rows %+v for both", listed, totalled, want)
			}
			if faceted != want || measured != want {
				t.Errorf("the library counted its tabs in %+v and its bytes in %+v, want the caller's own rows %+v for both", faceted, measured, want)
			}
		})
	}
}

func TestTheLibrarysFiltersReachEveryFigureThatFollowsThem(t *testing.T) {
	principal := principals()["an Admin"]
	audio, search, unused := openapi.MediaKind("audio"), "sân bay", true
	var listed, faceted domain.ListInput
	var totalled query.TotalBytes
	app := &application.Application{Queries: application.Queries{
		List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
			listed = q.Input
			return query.ListResult{Items: []domain.Asset{{ID: uuid.NewString(), Kind: domain.KindImage, DisplayName: "Bản đồ.png", Width: new(1200), Height: new(800), QuestionCount: 2}}}, nil
		}),
		TotalBytes: cqrs.HandlerFunc[query.TotalBytes, int64](func(_ context.Context, q query.TotalBytes) (int64, error) {
			totalled = q
			return 240_000, nil
		}),
		Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(_ context.Context, q query.Facets) (domain.Facets, error) {
			faceted = q.Input
			return domain.Facets{All: 8, Audio: 5, Image: 3, Unused: 1}, nil
		}),
		Usage: cqrs.HandlerFunc[query.Usage, domain.Usage](func(context.Context, query.Usage) (domain.Usage, error) {
			return domain.Usage{AudioBytes: 11, ImageBytes: 7, QuotaBytes: 100}, nil
		}),
	}}
	response, err := mediahttp.NewMedia(app).ListMedia(contextAs(t, principal), openapi.ListMediaRequestObject{
		Params: openapi.ListMediaParams{Kind: &audio, Q: &search, Unused: &unused},
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]domain.ListInput{
		"the list": listed, "the facets": faceted,
		"the total": {Kind: totalled.Kind, Query: totalled.Query, Unused: totalled.Unused, Scope: totalled.Scope},
	} {
		if in.Kind == nil || *in.Kind != domain.KindAudio || in.Query != search || !in.Unused || in.Scope != (access.Scope{UserID: principal.UserID}) {
			t.Errorf("%s read %+v, want audio, the search, unused and the caller's own rows", name, in)
		}
	}
	body := response.(openapi.ListMedia200JSONResponse).Body
	if body.Facets != (openapi.MediaFacets{All: 8, Audio: 5, Image: 3, Unused: 1}) {
		t.Errorf("facets = %+v", body.Facets)
	}
	if body.Usage != (openapi.MediaUsage{AudioBytes: 11, ImageBytes: 7, QuotaBytes: 100}) || body.TotalBytes != 240_000 {
		t.Errorf("usage = %+v and totalBytes = %d", body.Usage, body.TotalBytes)
	}
	item := body.Items[0]
	if item.DisplayName != "Bản đồ.png" || item.QuestionCount != 2 || item.DefaultMaxPlays != nil || item.Width == nil || *item.Width != 1200 || item.Height == nil || *item.Height != 800 {
		t.Errorf("the row is %+v, want its name, its size in pixels, its question count and no play limit", item)
	}
}

func TestAnUpdateCarriesTheCallersScopeAll(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			var seen domain.UpdateInput
			app := &application.Application{Commands: application.Commands{Update: cqrs.HandlerFunc[command.Update, domain.Asset](func(_ context.Context, in command.Update) (domain.Asset, error) {
				seen = in.Input
				return domain.Asset{}, domain.ErrNotFound
			})}}
			id, renamed := uuid.New(), "Tên mới"
			response, err := mediahttp.NewMedia(app).UpdateMedia(contextAs(t, principal), openapi.UpdateMediaRequestObject{Id: id, Body: &openapi.MediaUpdate{DisplayName: &renamed}})
			if err != nil {
				t.Fatal(err)
			}
			if seen.ID != id.String() || seen.ActorID != principal.UserID || seen.All != principal.Permissions.Has(access.ScopeAll) {
				t.Errorf("the update ran on %s as %s with All=%v, want %s as %s with All=%v", seen.ID, seen.ActorID, seen.All, id, principal.UserID, principal.Permissions.Has(access.ScopeAll))
			}
			if _, ok := response.(openapi.UpdateMedia404JSONResponse); !ok {
				t.Errorf("an asset outside the caller's scope answered %T, want the 404 a missing asset gets", response)
			}
		})
	}
}

func TestAnUpdateTellsAnAbsentLimitFromAClearedOne(t *testing.T) {
	for name, c := range map[string]struct {
		body  string
		set   bool
		plays *int
	}{
		"left out":  {`{"displayName":"Tên mới"}`, false, nil},
		"cleared":   {`{"defaultMaxPlays":null}`, true, nil},
		"unlimited": {`{"defaultMaxPlays":0}`, true, new(0)},
		"two plays": {`{"defaultMaxPlays":2}`, true, new(2)},
	} {
		var body openapi.MediaUpdate
		if err := json.Unmarshal([]byte(c.body), &body); err != nil {
			t.Fatal(err)
		}
		var seen domain.UpdateInput
		app := &application.Application{Commands: application.Commands{Update: cqrs.HandlerFunc[command.Update, domain.Asset](func(_ context.Context, in command.Update) (domain.Asset, error) {
			seen = in.Input
			return domain.Asset{}, nil
		})}}
		if _, err := mediahttp.NewMedia(app).UpdateMedia(contextAs(t, principals()["a Teacher"]), openapi.UpdateMediaRequestObject{Id: uuid.New(), Body: &body}); err != nil {
			t.Fatal(err)
		}
		if seen.SetDefaultMaxPlays != c.set || (seen.DefaultMaxPlays == nil) != (c.plays == nil) || (c.plays != nil && *seen.DefaultMaxPlays != *c.plays) {
			t.Errorf("a limit %s reached the command as set=%v %v, want set=%v %v", name, seen.SetDefaultMaxPlays, seen.DefaultMaxPlays, c.set, c.plays)
		}
	}
}

func TestWithoutObjectStorageTheLibraryAnswersNotImplemented(t *testing.T) {
	transport, ctx := mediahttp.NewMedia(nil), contextAs(t, principals()["a Teacher"])
	renamed := "Tên mới"
	for name, call := range map[string]func() error{
		"updateMedia": func() error {
			_, err := transport.UpdateMedia(ctx, openapi.UpdateMediaRequestObject{Id: uuid.New(), Body: &openapi.MediaUpdate{DisplayName: &renamed}})
			return err
		},
		"listMedia": func() error {
			_, err := transport.ListMedia(ctx, openapi.ListMediaRequestObject{})
			return err
		},
		"deleteMedia": func() error {
			_, err := transport.DeleteMedia(ctx, openapi.DeleteMediaRequestObject{Id: uuid.New()})
			return err
		},
	} {
		if err := call(); !errors.Is(err, httpx.ErrNotImplemented) {
			t.Errorf("%s without object storage: %v, want the 501 the router turns ErrNotImplemented into", name, err)
		}
	}
}

func TestADeleteCarriesTheCallersScopeAll(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			var seen domain.DeleteInput
			app := &application.Application{Commands: application.Commands{Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, in command.Delete) (cqrs.Nothing, error) {
				seen = in.Input
				return cqrs.Nothing{}, domain.ErrNotFound
			})}}
			response, err := mediahttp.NewMedia(app).DeleteMedia(contextAs(t, principal), openapi.DeleteMediaRequestObject{Id: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			if seen.ActorID != principal.UserID || seen.All != principal.Permissions.Has(access.ScopeAll) {
				t.Errorf("the delete ran as %s with All=%v, want %s with All=%v", seen.ActorID, seen.All, principal.UserID, principal.Permissions.Has(access.ScopeAll))
			}
			if _, ok := response.(openapi.DeleteMedia404JSONResponse); !ok {
				t.Errorf("an asset outside the caller's scope answered %T, want the 404 a missing asset gets", response)
			}
		})
	}
}
