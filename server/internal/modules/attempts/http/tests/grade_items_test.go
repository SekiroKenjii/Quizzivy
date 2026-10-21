package http_test

import (
	"context"
	"encoding/json"
	"net/http"
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

func TestAGradeItemWithoutPointsIsRefusedBeforeAnythingIsSaved(t *testing.T) {
	points, band, criteria, both := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for name, tc := range map[string]struct {
		items   string
		refused []string
	}{
		"points only":     {items: `[{"questionId":"` + points + `","points":1}]`},
		"no points":       {items: `[{"questionId":"` + points + `","points":1},{"questionId":"` + band + `"}]`, refused: []string{band}},
		"a band":          {items: `[{"questionId":"` + band + `","band":6.5}]`, refused: []string{band}},
		"criteria":        {items: `[{"questionId":"` + criteria + `","criteria":[{"criterionId":"` + uuid.NewString() + `","band":7}]}]`, refused: []string{criteria}},
		"points and band": {items: `[{"questionId":"` + both + `","points":1,"band":6}]`, refused: []string{both}},
	} {
		var body openapi.GradeAttemptJSONRequestBody
		if err := json.Unmarshal([]byte(`{"items":`+tc.items+`}`), &body); err != nil {
			t.Fatal(err)
		}
		saved := false
		transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
			Grade: cqrs.HandlerFunc[command.Grade, domain.Score](func(context.Context, command.Grade) (domain.Score, error) {
				saved = true
				return domain.Score{}, nil
			}),
		}}, nil, nil, nil)
		recorder := teacherRefusedIn(t, "", func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.GradeAttempt(ctx, openapi.GradeAttemptRequestObject{Id: uuid.New(), Body: &body})
			if err != nil {
				return err
			}
			return response.VisitGradeAttemptResponse(w)
		})
		if tc.refused == nil {
			if recorder.Code != http.StatusOK || !saved {
				t.Errorf("%s: answered %d, saved %v", name, recorder.Code, saved)
			}
			continue
		}
		var refusal struct {
			Error struct {
				Code    openapi.ErrorCode `json:"code"`
				Details struct {
					QuestionIDs []string `json:"questionIds"`
				} `json:"details"`
			} `json:"error"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &refusal); err != nil {
			t.Fatalf("%s: %v in %s", name, err, recorder.Body.String())
		}
		if recorder.Code != http.StatusUnprocessableEntity || refusal.Error.Code != openapi.GRADENEEDSPOINTS || saved {
			t.Errorf("%s: answered %d %s, saved %v", name, recorder.Code, refusal.Error.Code, saved)
		}
		if !slices.Equal(refusal.Error.Details.QuestionIDs, tc.refused) {
			t.Errorf("%s: details name %v, want %v", name, refusal.Error.Details.QuestionIDs, tc.refused)
		}
	}
}
