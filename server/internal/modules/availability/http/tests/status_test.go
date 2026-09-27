package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/availability/application"
	"quizzivy/internal/modules/availability/domain"
	availabilityhttp "quizzivy/internal/modules/availability/http"
)

type repo struct{ window *domain.Window }

func (r repo) Next(context.Context, time.Time) (*domain.Window, error) { return r.window, nil }

var now = time.Date(2026, 10, 1, 15, 0, 0, 0, time.FixedZone("ICT", 7*60*60))

func status(t *testing.T, window *domain.Window) *httptest.ResponseRecorder {
	t.Helper()
	app := application.New(repo{window: window}, nil)
	app.SetClock(func() time.Time { return now })
	out, err := availabilityhttp.NewAvailability(app).GetPublicStatus(context.Background(), openapi.GetPublicStatusRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if err := out.VisitGetPublicStatusResponse(response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestNoWindowIsNull(t *testing.T) {
	response := status(t, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	if got := response.Body.String(); got != "{\"maintenance\":null}\n" {
		t.Errorf("body %q, want maintenance null", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("Cache-Control %q, want public, max-age=30", got)
	}
}

func TestAComingWindowIsAnnouncedInUTC(t *testing.T) {
	response := status(t, &domain.Window{StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)})
	want := "{\"maintenance\":{\"active\":false,\"endsAt\":\"2026-10-01T10:00:00Z\",\"startsAt\":\"2026-10-01T09:00:00Z\"}}\n"
	if got := response.Body.String(); got != want {
		t.Errorf("body %q, want %q", got, want)
	}
}

func TestAWindowUnderWayIsActive(t *testing.T) {
	response := status(t, &domain.Window{StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)})
	want := "{\"maintenance\":{\"active\":true,\"endsAt\":\"2026-10-01T09:00:00Z\",\"startsAt\":\"2026-10-01T07:59:00Z\"}}\n"
	if got := response.Body.String(); got != want {
		t.Errorf("body %q, want %q", got, want)
	}
}

func TestWithoutTheModuleTheStatusIsNotImplemented(t *testing.T) {
	if _, err := availabilityhttp.NewAvailability(nil).GetPublicStatus(context.Background(), openapi.GetPublicStatusRequestObject{}); err == nil {
		t.Error("a transport with no application answered")
	}
}
