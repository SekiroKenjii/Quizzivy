package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"
)

func gradeThrough(t *testing.T, accept, items string, grade func(command.Grade) (domain.Score, error)) *httptest.ResponseRecorder {
	t.Helper()
	var body openapi.GradeAttemptJSONRequestBody
	if err := json.Unmarshal([]byte(`{"items":`+items+`}`), &body); err != nil {
		t.Fatal(err)
	}
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Grade: cqrs.HandlerFunc[command.Grade, domain.Score](func(_ context.Context, cmd command.Grade) (domain.Score, error) {
			return grade(cmd)
		}),
	}}, nil, nil, nil)
	return teacherRefusedIn(t, accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GradeAttempt(ctx, openapi.GradeAttemptRequestObject{Id: uuid.New(), Body: &body})
		if err != nil {
			return err
		}
		return response.VisitGradeAttemptResponse(w)
	})
}

func TestEveryGradeShapeRefusalAnswers422WithItsCodeInTheCallersLanguage(t *testing.T) {
	for _, tc := range []struct {
		shape  domain.GradeShape
		code   openapi.ErrorCode
		vi, en string
	}{
		{domain.NeedsPoints, openapi.GRADENEEDSPOINTS, "Câu này cần được chấm bằng điểm số.", "This question needs a score in points."},
		{domain.BandNotAllowed, openapi.GRADEBANDNOTALLOWED, "Chỉ bài luận mới được chấm theo band.", "Only an essay is graded on a band."},
		{domain.NeedsBand, openapi.GRADENEEDSBAND, "Hãy chấm bài luận này bằng một band, không kèm điểm số hay tiêu chí.", "Give this essay one band, with no points or criteria."},
		{domain.CriteriaIncomplete, openapi.GRADECRITERIAINCOMPLETE, "Hãy chấm band cho từng tiêu chí của bài luận này, mỗi tiêu chí một lần.", "Give a band for each of this essay's criteria, once each."},
	} {
		refused := uuid.NewString()
		for accept, message := range map[string]string{"": tc.vi, "en": tc.en} {
			recorder := gradeThrough(t, accept, `[{"questionId":"`+refused+`","points":1}]`, func(command.Grade) (domain.Score, error) {
				return domain.Score{}, &domain.GradeShapeError{Shape: tc.shape, QuestionIDs: []string{refused}}
			})
			var refusal struct {
				Error struct {
					Code    openapi.ErrorCode `json:"code"`
					Message string            `json:"message"`
					Details struct {
						QuestionIDs []string `json:"questionIds"`
					} `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &refusal); err != nil {
				t.Fatalf("%s: %v in %s", tc.shape, err, recorder.Body.String())
			}
			if recorder.Code != http.StatusUnprocessableEntity || refusal.Error.Code != tc.code || refusal.Error.Message != message {
				t.Errorf("%s in %q: answered %d %s %q, want 422 %s %q", tc.shape, accept, recorder.Code, refusal.Error.Code, refusal.Error.Message, tc.code, message)
			}
			if !slices.Equal(refusal.Error.Details.QuestionIDs, []string{refused}) {
				t.Errorf("%s: details name %v, want %v", tc.shape, refusal.Error.Details.QuestionIDs, refused)
			}
		}
	}
}

func TestTheGradeBodyReachesTheCommandAsSentForTheRepositoryToJudge(t *testing.T) {
	points, band, criteria, criterion := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	var got []domain.GradeItem
	band7 := 7.0
	recorder := gradeThrough(t, "", `[{"questionId":"`+points+`","points":1.5},{"questionId":"`+band+`","band":6.5,"points":2},`+
		`{"questionId":"`+criteria+`","criteria":[{"criterionId":"`+criterion+`","band":7}]}]`, func(cmd command.Grade) (domain.Score, error) {
		got = cmd.Items
		return domain.Score{Earned: 3, Total: 9, Band: &band7}, nil
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(got) != 3 || *got[0].Points != 1.5 || got[0].Band != nil || got[0].Criteria != nil {
		t.Fatalf("points item %+v", got)
	}
	if *got[1].Band != 6.5 || *got[1].Points != 2 {
		t.Errorf("band item %+v: the shape is the repository's to judge, so both reach it", got[1])
	}
	if got[2].Points != nil || got[2].Band != nil || !slices.Equal(got[2].Criteria, []domain.CriterionBand{{CriterionID: criterion, Band: 7}}) {
		t.Errorf("criteria item %+v", got[2])
	}
	var score openapi.AttemptScore
	if err := json.Unmarshal(recorder.Body.Bytes(), &score); err != nil || score.Band == nil || *score.Band != 7 {
		t.Errorf("score %s (%v): the paper's band is not answered", recorder.Body.String(), err)
	}
}
