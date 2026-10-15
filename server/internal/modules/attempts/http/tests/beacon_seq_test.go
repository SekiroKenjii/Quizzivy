package http_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"
)

func beaconBody(session uuid.UUID, seq int64) string {
	return `{"beaconToken":"token","sessionId":"` + session.String() +
		`","events":[{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":` + strconv.FormatInt(seq, 10) + `}]}`
}

func TestABeaconEventOutsideTheColumnsRangeIsRefusedAndNothingIsFlushed(t *testing.T) {
	session := uuid.New()
	for name, seq := range map[string]int64{"one past the column": 2147483648, "a 32-bit wrap": 4294967296, "negative": -1} {
		t.Run(name, func(t *testing.T) {
			var flushed []command.Flush
			app := &application.Application{Commands: application.Commands{Flush: cqrs.HandlerFunc[command.Flush, cqrs.Nothing](func(_ context.Context, c command.Flush) (cqrs.Nothing, error) {
				flushed = append(flushed, c)
				return cqrs.Nothing{}, nil
			})}}
			transport := attemptshttp.NewAttempts(app, nil, nil, nil)
			body := beaconBody(session, seq)

			response := serve(t, uuid.New(), func(w http.ResponseWriter, r *http.Request) {
				out, err := transport.FlushEvents(r.Context(), openapi.FlushEventsRequestObject{Id: uuid.New(), TextBody: &body})
				if err != nil {
					t.Fatal(err)
				}
				if err := out.VisitFlushEventsResponse(w); err != nil {
					t.Fatal(err)
				}
			})

			if response.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400: %s", response.Code, response.Body.String())
			}
			if len(flushed) != 0 {
				t.Errorf("%d flushes reached the command, want none", len(flushed))
			}
		})
	}
}

func TestABeaconEventAtTheColumnsLargestNumberIsFlushed(t *testing.T) {
	session := uuid.New()
	var flushed []command.Flush
	app := &application.Application{Commands: application.Commands{Flush: cqrs.HandlerFunc[command.Flush, cqrs.Nothing](func(_ context.Context, c command.Flush) (cqrs.Nothing, error) {
		flushed = append(flushed, c)
		return cqrs.Nothing{}, nil
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)
	body := beaconBody(session, 2147483647)

	response := serve(t, uuid.New(), func(w http.ResponseWriter, r *http.Request) {
		out, err := transport.FlushEvents(r.Context(), openapi.FlushEventsRequestObject{Id: uuid.New(), TextBody: &body})
		if err != nil {
			t.Fatal(err)
		}
		if err := out.VisitFlushEventsResponse(w); err != nil {
			t.Fatal(err)
		}
	})

	if response.Code != http.StatusAccepted || len(flushed) != 1 || flushed[0].Input.Events[0].ClientSeq != 2147483647 {
		t.Errorf("status %d with %d flushes, want 202 with one at 2147483647", response.Code, len(flushed))
	}
}

func TestABeaconBodyThatBreaksTheContractIsRefusedAndNothingIsFlushed(t *testing.T) {
	session := uuid.New()
	event := func(fields string) string { return "{" + fields + "}" }
	valid := `"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":1`
	many := make([]string, 501)
	for i := range many {
		many[i] = event(valid)
	}
	for name, events := range map[string]string{
		"a kind of 41 characters": event(`"kind":"` + strings.Repeat("k", 41) + `","occurredAt":"2026-10-04T08:00:00Z","clientSeq":1`),
		"an empty kind":           event(`"kind":"","occurredAt":"2026-10-04T08:00:00Z","clientSeq":1`),
		"no kind":                 event(`"occurredAt":"2026-10-04T08:00:00Z","clientSeq":1`),
		"no occurredAt":           event(`"kind":"paste","clientSeq":1`),
		"no clientSeq":            event(`"kind":"paste","occurredAt":"2026-10-04T08:00:00Z"`),
		"a null clientSeq":        event(`"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":null`),
		"a fractional clientSeq":  event(`"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":1.5`),
		"501 events":              strings.Join(many, ","),
	} {
		t.Run(name, func(t *testing.T) {
			var flushed []command.Flush
			app := &application.Application{Commands: application.Commands{Flush: cqrs.HandlerFunc[command.Flush, cqrs.Nothing](func(_ context.Context, c command.Flush) (cqrs.Nothing, error) {
				flushed = append(flushed, c)
				return cqrs.Nothing{}, nil
			})}}
			transport := attemptshttp.NewAttempts(app, nil, nil, nil)
			body := `{"beaconToken":"token","sessionId":"` + session.String() + `","events":[` + events + `]}`

			response := serve(t, uuid.New(), func(w http.ResponseWriter, r *http.Request) {
				out, err := transport.FlushEvents(r.Context(), openapi.FlushEventsRequestObject{Id: uuid.New(), TextBody: &body})
				if err != nil {
					t.Fatal(err)
				}
				if err := out.VisitFlushEventsResponse(w); err != nil {
					t.Fatal(err)
				}
			})

			if response.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400: %s", response.Code, response.Body.String())
			}
			if len(flushed) != 0 {
				t.Errorf("%d flushes reached the command, want none", len(flushed))
			}
		})
	}
}
