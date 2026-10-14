package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

const (
	moveTest     = "01935000-0000-7000-8000-0000000000a1"
	moveQuestion = "01935000-0000-7000-8000-0000000000a2"
	moveSection  = "01935000-0000-7000-8000-0000000000a3"
	moveGroup    = "01935000-0000-7000-8000-0000000000a4"
	moveOther    = "01935000-0000-7000-8000-0000000000a5"
)

func moveBody(t *testing.T, group string) *openapi.MoveTestQuestionJSONRequestBody {
	t.Helper()
	var body openapi.MoveTestQuestionJSONRequestBody
	input := `{"expectedUpdatedAt":"2026-10-14T03:00:00.123456Z","questionId":"` + moveQuestion + `",
		"to":{"sectionId":"` + moveSection + `","groupId":` + group + `,"index":2},
		"expectedGroupRevisions":[{"groupId":"` + moveGroup + `","revision":7},{"groupId":"` + moveOther + `","revision":3}]}`
	if err := json.Unmarshal([]byte(input), &body); err != nil {
		t.Fatal(err)
	}
	return &body
}

func movingWith(handle func(command.MoveQuestion) (domain.MoveQuestionResult, error)) testshttp.Tests {
	return testshttp.NewTests(&application.Application{Commands: application.Commands{
		MoveQuestion: cqrs.HandlerFunc[command.MoveQuestion, domain.MoveQuestionResult](func(_ context.Context, in command.MoveQuestion) (domain.MoveQuestionResult, error) {
			return handle(in)
		}),
	}}, nil)
}

func TestMoveCarriesTheRequestAndTheCallerToTheCommand(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	for name, group := range map[string]string{"into a group": `"` + moveGroup + `"`, "out of a group": `null`} {
		t.Run(name, func(t *testing.T) {
			var seen command.MoveQuestion
			transport := movingWith(func(in command.MoveQuestion) (domain.MoveQuestionResult, error) {
				seen = in
				return domain.MoveQuestionResult{}, domain.ErrForbidden
			})

			_, err := transport.MoveTestQuestion(contextAs(t, principal), openapi.MoveTestQuestionRequestObject{Id: uuid.MustParse(moveTest), Body: moveBody(t, group)})
			if err != nil {
				t.Fatal(err)
			}

			move := seen.Move
			if move.TestID != moveTest || move.QuestionID != moveQuestion || move.To.SectionID != moveSection || move.To.Index != 2 {
				t.Errorf("ids or index lost: %+v", move)
			}
			if !move.ExpectedTestUpdatedAt.Equal(time.Date(2026, 10, 14, 3, 0, 0, 123456000, time.UTC)) {
				t.Errorf("the test revision lost its precision: %v", move.ExpectedTestUpdatedAt)
			}
			if (group == `null`) != (move.To.GroupID == nil) || (move.To.GroupID != nil && *move.To.GroupID != moveGroup) {
				t.Errorf("the destination group is %v for %s", move.To.GroupID, group)
			}
			if len(move.ExpectedGroups) != 2 || move.ExpectedGroups[0] != (domain.GroupRevision{GroupID: moveGroup, Revision: 7}) || move.ExpectedGroups[1].GroupID != moveOther {
				t.Errorf("the group revisions changed: %+v", move.ExpectedGroups)
			}
			if move.Actor.ID != principal.UserID || move.Actor.Scope != (access.Scope{UserID: principal.UserID}) {
				t.Errorf("the actor is %+v", move.Actor)
			}
			if !move.Grants.SubsetOf(principal.Permissions) || !principal.Permissions.SubsetOf(move.Grants) {
				t.Errorf("the command got grants %v, want the principal's", move.Grants.Keys())
			}
		})
	}
}

func TestMoveAnswersWhatTheBuilderGoesOnFrom(t *testing.T) {
	stored := domain.StoredGroup{
		Bundle:   domain.GroupBundle{Group: domain.QuestionGroup{ID: moveGroup, Title: "Nhóm", Members: []domain.GroupMember{}, Stimuli: []domain.GroupStimulus{}, Recordings: []domain.GroupRecording{}}},
		Revision: 8, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	updated := time.Date(2026, 10, 14, 3, 5, 0, 0, time.UTC)
	transport := movingWith(func(command.MoveQuestion) (domain.MoveQuestionResult, error) {
		return domain.MoveQuestionResult{
			Test: domain.Test{ID: moveTest, Title: "Đề", Status: domain.Draft, TotalPoints: "2", Skills: []string{}, CreatedAt: updated, UpdatedAt: updated,
				Sections: []domain.Section{{ID: moveSection, Title: "Phần 1", QuestionIDs: []string{}, Units: []domain.SectionUnit{{Kind: "group", ID: moveGroup}}}}},
			QuestionID: moveOther, Copied: true, Groups: []domain.StoredGroup{stored},
		}, nil
	})
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}

	response, err := transport.MoveTestQuestion(contextAs(t, principal), openapi.MoveTestQuestionRequestObject{Id: uuid.MustParse(moveTest), Body: moveBody(t, `"`+moveGroup+`"`)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitMoveTestQuestionResponse(recorder); err != nil {
		t.Fatal(err)
	}

	var answer openapi.MoveTestQuestionResult
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &answer) != nil {
		t.Fatalf("answered %d %s", recorder.Code, recorder.Body.String())
	}
	units := *answer.Test.Sections[0].Units
	if !answer.Copied || answer.QuestionId.String() != moveOther || !answer.Test.UpdatedAt.Equal(updated) ||
		len(answer.Groups) != 1 || answer.Groups[0].Revision != 8 || len(units) != 1 || units[0].Kind != openapi.DraftSectionUnitKindGroup {
		t.Fatalf("the answer lost what the client continues from: %s", recorder.Body.String())
	}
}

func TestMoveRefusalsAnswerTheirStatusCodeAndRule(t *testing.T) {
	gapBound := &domain.GroupError{Rule: domain.RuleGroupGapBound, StimulusID: "s1", GapID: "g1", QuestionID: moveQuestion}
	for name, c := range map[string]struct {
		err    error
		status int
		code   openapi.ErrorCode
		rule   string
	}{
		"a gap-bound member":            {gapBound, 422, openapi.VALIDATIONFAILED, domain.RuleGroupGapBound},
		"a move between standalones":    {&domain.GroupError{Rule: domain.RuleMoveOutlineOnly}, 400, openapi.VALIDATIONFAILED, domain.RuleMoveOutlineOnly},
		"a destination not the section": {&domain.GroupError{Rule: domain.RuleMoveGroupSection}, 400, openapi.VALIDATIONFAILED, domain.RuleMoveGroupSection},
		"revisions that are not exact":  {&domain.GroupError{Rule: domain.RuleMoveRevisions}, 400, openapi.VALIDATIONFAILED, domain.RuleMoveRevisions},
		"an ambiguous source":           {&domain.GroupError{Rule: domain.RuleMoveSourceAmbiguous}, 400, openapi.VALIDATIONFAILED, domain.RuleMoveSourceAmbiguous},
		"a full group":                  {&domain.GroupError{Rule: domain.RuleGroupLimits}, 422, openapi.VALIDATIONFAILED, domain.RuleGroupLimits},
		"a shared recording":            {&domain.GroupError{Rule: "group_audio_scope", QuestionID: moveQuestion}, 422, openapi.VALIDATIONFAILED, "group_audio_scope"},
		"an archived test":              {&domain.GroupError{Rule: "group_owner_archived"}, 409, openapi.TESTARCHIVED, "group_owner_archived"},
		"a stale revision":              {domain.ErrStaleWrite, 409, openapi.STALEWRITE, ""},
		"a missing key":                 {domain.ErrForbidden, 403, openapi.FORBIDDEN, ""},
		"an id the caller cannot reach": {domain.ErrNotFound, 404, openapi.NOTFOUND, ""},
	} {
		t.Run(name, func(t *testing.T) {
			transport := movingWith(func(command.MoveQuestion) (domain.MoveQuestionResult, error) {
				return domain.MoveQuestionResult{}, c.err
			})
			serve := func(ctx context.Context, w http.ResponseWriter) error {
				response, err := transport.MoveTestQuestion(ctx, openapi.MoveTestQuestionRequestObject{Id: uuid.MustParse(moveTest), Body: moveBody(t, `"`+moveGroup+`"`)})
				if err != nil {
					return err
				}
				return response.VisitMoveTestQuestionResponse(w)
			}

			vi, en := refusalIn(t, "", serve), refusalIn(t, "en", serve)

			if vi.status != c.status || vi.code != c.code {
				t.Fatalf("answered %d %s, want %d %s", vi.status, vi.code, c.status, c.code)
			}
			if c.rule != "" && vi.details["rule"] != c.rule {
				t.Fatalf("details %v, want rule %s", vi.details, c.rule)
			}
			if c.err == gapBound && (vi.details["stimulusId"] != "s1" || vi.details["gapId"] != "g1" || vi.details["questionId"] != moveQuestion) {
				t.Fatalf("a gap-bound refusal must name the stimulus, gap and question: %v", vi.details)
			}
			if vi.message == "" || en.message == "" || vi.message == en.message || strings.TrimSpace(en.message) != en.message {
				t.Fatalf("the message must be written in both languages: vi %q en %q", vi.message, en.message)
			}
		})
	}
}

func TestMoveIsUnimplementedWithoutItsCommandOrRepository(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	request := openapi.MoveTestQuestionRequestObject{Id: uuid.MustParse(moveTest), Body: moveBody(t, `"`+moveGroup+`"`)}

	if _, err := testshttp.NewTests(&application.Application{}, nil).MoveTestQuestion(contextAs(t, principal), request); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("a missing command answered %v", err)
	}
	unavailable := movingWith(func(command.MoveQuestion) (domain.MoveQuestionResult, error) {
		return domain.MoveQuestionResult{}, domain.ErrGroupUnavailable
	})
	if _, err := unavailable.MoveTestQuestion(contextAs(t, principal), request); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("a missing repository answered %v", err)
	}
}
