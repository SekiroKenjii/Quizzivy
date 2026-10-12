package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"
)

func readAttempt(t *testing.T, held *openapi.Uuid, answer domain.Session) (*httptest.ResponseRecorder, []query.Get) {
	t.Helper()
	var received []query.Get
	app := &application.Application{Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Session](func(_ context.Context, q query.Get) (domain.Session, error) {
		received = append(received, q)
		return answer, nil
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	response := serve(t, uuid.New(), func(w http.ResponseWriter, r *http.Request) {
		out, err := transport.GetAttempt(r.Context(), openapi.GetAttemptRequestObject{Id: uuid.New(), Params: openapi.GetAttemptParams{Session: held}})
		if err != nil {
			t.Fatal(err)
		}
		if err := out.VisitGetAttemptResponse(w); err != nil {
			t.Fatal(err)
		}
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", response.Code, response.Body.String())
	}
	return response, received
}

func sessionMembers(t *testing.T, response *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v in %s", err, response.Body.String())
	}
	return body
}

func TestTheSessionAReaderNamesReachesTheQuery(t *testing.T) {
	held := uuid.New()
	_, received := readAttempt(t, &held, domain.Session{SessionID: uuid.NewString()})

	if len(received) != 1 || received[0].HeldSession != held.String() {
		t.Errorf("the query received %+v, want one naming session %s", received, held)
	}
}

func TestAReaderThatNamesNoSessionSendsNone(t *testing.T) {
	_, received := readAttempt(t, nil, domain.Session{SessionID: uuid.NewString()})

	if len(received) != 1 || received[0].HeldSession != "" {
		t.Errorf("the query received %+v, want one naming no session", received)
	}
}

func TestASupersededSessionIsSaidInThePayload(t *testing.T) {
	named := uuid.New()
	response, _ := readAttempt(t, &named, domain.Session{Superseded: true, SessionID: named.String(), BeaconToken: ""})
	body := sessionMembers(t, response)

	if got := string(body["superseded"]); got != "true" {
		t.Errorf("superseded is %q, want true: %s", got, response.Body.String())
	}
	if got, want := string(body["sessionId"]), `"`+named.String()+`"`; got != want {
		t.Errorf("sessionId is %s, want %s", got, want)
	}
	if got := string(body["beaconToken"]); got != `""` {
		t.Errorf("beaconToken is %s, want the empty string", got)
	}
}

func TestAnOrdinaryPayloadDoesNotMentionSuperseded(t *testing.T) {
	held := uuid.New()
	response, _ := readAttempt(t, &held, domain.Session{Superseded: false, SessionID: held.String(), BeaconToken: "fixture"})
	body := sessionMembers(t, response)

	if raw, present := body["superseded"]; present {
		t.Errorf("superseded is in the payload as %s, want it absent: %s", raw, response.Body.String())
	}
	if got, want := string(body["sessionId"]), `"`+held.String()+`"`; got != want {
		t.Errorf("sessionId is %s, want %s", got, want)
	}
	if got := string(body["beaconToken"]); got != `"fixture"` {
		t.Errorf("beaconToken is %s, want the one the query returned", got)
	}
}
