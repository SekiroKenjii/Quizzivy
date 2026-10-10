package http_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	notificationshttp "quizzivy/internal/modules/notifications/http"
	"quizzivy/internal/shared/cqrs"
)

type steps struct {
	order        []string
	materialised []command.MaterialiseDue
	failWith     error
}

func (s *steps) app() *application.Application {
	return &application.Application{
		Commands: application.Commands{
			MaterialiseDue: cqrs.HandlerFunc[command.MaterialiseDue, int](func(_ context.Context, cmd command.MaterialiseDue) (int, error) {
				s.order = append(s.order, "materialise")
				s.materialised = append(s.materialised, cmd)
				return 0, s.failWith
			}),
			MarkRead: cqrs.HandlerFunc[command.MarkRead, cqrs.Nothing](func(context.Context, command.MarkRead) (cqrs.Nothing, error) {
				s.order = append(s.order, "mark")
				return cqrs.Nothing{}, nil
			}),
		},
		Queries: application.Queries{
			Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(context.Context, query.Summary) (domain.Summary, error) {
				s.order = append(s.order, "summary")
				return domain.Summary{UnreadNotifications: 3}, nil
			}),
			List: cqrs.HandlerFunc[query.List, domain.Page](func(context.Context, query.List) (domain.Page, error) {
				s.order = append(s.order, "list")
				return domain.Page{}, nil
			}),
		},
	}
}

func readSummary(t *testing.T, transport notificationshttp.Notifications) string {
	t.Helper()
	return answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.GetMySummary(ctx, openapi.GetMySummaryRequestObject{})
		if err != nil {
			return err
		}
		return out.VisitGetMySummaryResponse(w)
	}).Body.String()
}

func readList(t *testing.T, transport notificationshttp.Notifications) int {
	t.Helper()
	return answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.ListNotifications(ctx, openapi.ListNotificationsRequestObject{})
		if err != nil {
			return err
		}
		return out.VisitListNotificationsResponse(w)
	}).Code
}

func TestTheSummaryAndTheListMaterialiseTheCallersDueItemsFirst(t *testing.T) {
	s := &steps{}
	transport := notificationshttp.NewNotifications(s.app())
	before := time.Now()
	if got := readSummary(t, transport); got != `{"unreadNotifications":3}`+"\n" {
		t.Errorf("the summary is %s", got)
	}
	if code := readList(t, transport); code != http.StatusOK {
		t.Errorf("the list answered %d", code)
	}
	after := time.Now()

	if strings.Join(s.order, ",") != "materialise,summary,materialise,list" {
		t.Errorf("the calls ran in the order %v, want each read preceded by the materialisation", s.order)
	}
	for _, cmd := range s.materialised {
		if cmd.UserID != caller {
			t.Errorf("materialised for %q, want the caller %q", cmd.UserID, caller)
		}
		if cmd.Now.Before(before) || cmd.Now.After(after) {
			t.Errorf("materialised at %v, want the moment of the request between %v and %v", cmd.Now, before, after)
		}
	}
}

func TestMarkingReadDoesNotMaterialise(t *testing.T) {
	s := &steps{}
	transport := notificationshttp.NewNotifications(s.app())
	ids := []openapi.Uuid{{}}
	answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.MarkNotificationsRead(ctx, openapi.MarkNotificationsReadRequestObject{Body: &openapi.MarkNotificationsReadJSONRequestBody{Ids: &ids}})
		if err != nil {
			return err
		}
		return out.VisitMarkNotificationsReadResponse(w)
	})
	if strings.Join(s.order, ",") != "mark" {
		t.Errorf("marking read ran %v, want only the mark", s.order)
	}
}

func TestAFailedMaterialisationIsLoggedAndTheReadStillAnswers(t *testing.T) {
	s := &steps{failWith: errors.New("the database is away")}
	var log bytes.Buffer
	transport := notificationshttp.NewNotifications(s.app()).WithLogger(slog.New(slog.NewTextHandler(&log, nil)))
	if got := readSummary(t, transport); got != `{"unreadNotifications":3}`+"\n" {
		t.Errorf("the summary is %s after a failed materialisation", got)
	}
	if code := readList(t, transport); code != http.StatusOK {
		t.Errorf("the list answered %d after a failed materialisation", code)
	}
	if strings.Count(log.String(), "due notifications not materialised") != 2 || !strings.Contains(log.String(), "the database is away") {
		t.Errorf("the failures were logged as %q", log.String())
	}
}

func TestAnApplicationWithoutMaterialisingStillServesTheReads(t *testing.T) {
	app := &application.Application{Queries: application.Queries{
		Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(context.Context, query.Summary) (domain.Summary, error) {
			return domain.Summary{UnreadNotifications: 1}, nil
		}),
	}}
	if got := readSummary(t, notificationshttp.NewNotifications(app)); got != `{"unreadNotifications":1}`+"\n" {
		t.Errorf("the summary is %s", got)
	}
}
