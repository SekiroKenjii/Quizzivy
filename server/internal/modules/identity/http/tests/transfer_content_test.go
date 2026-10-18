package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/cqrs"
)

type transferStub struct {
	got    command.TransferUserContent
	result domain.TransferredContent
	err    error
}

func (s *transferStub) call(t *testing.T, accept string, from, to uuid.UUID, notify *bool) served {
	t.Helper()
	h := identityhttp.NewIdentity(&application.Application{Commands: application.Commands{
		TransferUserContent: cqrs.HandlerFunc[command.TransferUserContent, domain.TransferredContent](func(_ context.Context, c command.TransferUserContent) (domain.TransferredContent, error) {
			s.got = c
			return s.result, s.err
		}),
	}}, 0, false, nil)
	return readAs(t, anAdmin(), accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.TransferUserContent(ctx, openapi.TransferUserContentRequestObject{Id: from, Body: &openapi.TransferUserContentJSONRequestBody{ToUserId: to, Notify: notify}})
		return render(w, openapi.TransferUserContentResponseObject.VisitTransferUserContentResponse, response, err)
	})
}

func TestATransferPassesTheCallerBothAccountsAndTheNotifyChoiceAndAnswersTheCounts(t *testing.T) {
	from, to, yes := uuid.New(), uuid.New(), true
	s := &transferStub{result: domain.TransferredContent{
		Moved:      domain.MovedContent{Classes: 1, Tests: 2, Questions: 3, QuestionGroups: 4, Media: 5, Assignments: 6},
		LeftBehind: domain.LeftBehind{Imports: 7, Students: 8},
	}}

	got := s.call(t, "", from, to, &yes)

	if got.err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	if s.got.ID != from.String() || s.got.ToID != to.String() || !s.got.Notify || s.got.Request.ActorID == "" || s.got.Request.IP != "203.0.113.9" || !s.got.Request.All {
		t.Errorf("command = %+v, want both accounts, notify and the caller", s.got)
	}
	var out map[string]map[string]float64
	if err := json.Unmarshal(got.body, &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]float64{
		"moved":      {"classes": 1, "tests": 2, "questions": 3, "questionGroups": 4, "media": 5, "assignments": 6},
		"leftBehind": {"imports": 7, "students": 8},
	}
	for part, counts := range want {
		for key, n := range counts {
			if out[part][key] != n {
				t.Errorf("%s.%s = %v, want %v: %s", part, key, out[part][key], n, got.body)
			}
		}
	}

	s.call(t, "", from, to, nil)
	if s.got.Notify {
		t.Error("a body without notify asked to notify")
	}
}

func TestATransferAnswersEachRefusalWithItsStatusAndCode(t *testing.T) {
	for _, r := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrUserNotFound, http.StatusNotFound, "NOT_FOUND"},
		{accessdomain.ErrTargetRoleNotAllowed, http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{domain.ErrUserNotDisabled, http.StatusConflict, "USER_NOT_DISABLED"},
		{domain.ErrRoleChanged, http.StatusConflict, "STALE_WRITE"},
		{fmt.Errorf("%w: deadlock detected", domain.ErrWriteContended), http.StatusConflict, "STALE_WRITE"},
		{domain.ErrInvalidRecipient, http.StatusUnprocessableEntity, "INVALID_RECIPIENT"},
	} {
		t.Run(r.code, func(t *testing.T) {
			for _, accept := range []string{"vi", "en"} {
				got := (&transferStub{err: r.err}).call(t, accept, uuid.New(), uuid.New(), nil)
				if got.err != nil || got.status != r.status {
					t.Fatalf("%s: answered %d, %v: %s", accept, got.status, got.err, got.body)
				}
				if code, _ := codeOf(t, got.body); code != r.code {
					t.Errorf("%s: code = %s, want %s", accept, code, r.code)
				}
			}
		})
	}
}

func TestATransferFaultIsLeftToTheServerErrorPathAndAMissingCommandIsNotImplemented(t *testing.T) {
	boom := errors.New("pq: connection reset")
	if got := (&transferStub{err: boom}).call(t, "", uuid.New(), uuid.New(), nil); !errors.Is(got.err, boom) {
		t.Errorf("answered %d with %v, want the fault returned", got.status, got.err)
	}
	h := identityhttp.NewIdentity(&application.Application{}, 0, false, nil)
	_, err := h.TransferUserContent(context.Background(), openapi.TransferUserContentRequestObject{Id: uuid.New(), Body: &openapi.TransferUserContentJSONRequestBody{ToUserId: uuid.New()}})
	if err == nil {
		t.Error("a server without the transfer answered it")
	}
}
