// Package http is the AI module's transport: the teacher's suggestions, usage and credit requests under /teacher/ai and the credits console under /admin/ai. With no provider configured every operation answers 501 AI_OFF before it reads anything. No operation here writes a grade, a question or a comment.
package http

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

// AI serves the AI assistant's operations over the application. A false enabled, the assistant without a provider, answers every operation 501.
type AI struct {
	app     *application.Application
	enabled bool
}

// NewAI returns the transport over app; the assistant is on when enabled is true.
func NewAI(app *application.Application, enabled bool) AI {
	return AI{app: app, enabled: enabled}
}

func (h AI) off(ctx context.Context) *failure {
	f, _ := failureOf(ctx, domain.ErrOff)
	return f
}

func (h AI) caller(ctx context.Context) (httpapi.Actor, access.Set, bool) {
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok || h.app == nil || who.ID == "" {
		return httpapi.Actor{}, access.Set{}, false
	}
	principal, _ := httpx.PrincipalFromContext(ctx)
	return who, principal.Access.Permissions, true
}

func failed[R any](ctx context.Context, err error) (R, error) {
	var none R
	f, err := failureOf(ctx, err)
	if f == nil {
		return none, err
	}
	out, ok := any(f).(R)
	if !ok {
		return none, errors.New("ai: a refusal does not answer this operation")
	}
	return out, nil
}

func usageOf(u domain.Usage) openapi.AiUsage {
	return openapi.AiUsage{Credits: int(u.Credits), Left: int(u.Left)}
}

func languageOf(l *openapi.AiLanguage) domain.Language {
	if l == nil {
		return ""
	}
	return domain.Language(*l)
}

func strings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

// GenerateAiQuestions implements POST /teacher/ai/questions/generate.
func (h AI) GenerateAiQuestions(ctx context.Context, request openapi.GenerateAiQuestionsRequestObject) (openapi.GenerateAiQuestionsResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	body := request.Body
	q := query.Generate{
		Actor: who, Source: httpapi.DerefString(body.Source), Types: strings(body.Types), Count: int(body.Count),
		Level: string(body.Level), Explanations: body.Explanations, Language: domain.Language(body.Language),
	}
	if body.TestId != nil {
		q.TestID = body.TestId.String()
	}
	if body.StimulusId != nil {
		q.StimulusID = body.StimulusId.String()
	}
	if body.SectionId != nil {
		q.SectionID = body.SectionId.String()
	}
	if body.Standalone != nil {
		q.Standalone = *body.Standalone
	}
	got, err := h.app.Queries.Generate.Handle(ctx, q)
	if err != nil {
		return failed[openapi.GenerateAiQuestionsResponseObject](ctx, err)
	}
	out := openapi.GenerateAiQuestions200JSONResponse{Items: make([]openapi.AiGeneratedQuestion, len(got.Items)), Usage: usageOf(got.Usage)}
	for i, item := range got.Items {
		out.Items[i] = openapi.AiGeneratedQuestion{Type: openapi.AiQuestionType(item.Type), Prompt: item.Prompt, AnswerKey: item.AnswerKey, Explanation: item.Explanation}
		if len(item.Options) > 0 {
			out.Items[i].Options = &item.Options
		}
	}
	return out, nil
}

// CheckTestWithAi implements POST /teacher/ai/tests/{id}/check.
func (h AI) CheckTestWithAi(ctx context.Context, request openapi.CheckTestWithAiRequestObject) (openapi.CheckTestWithAiResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Check.Handle(ctx, query.Check{Actor: who, TestID: request.Id.String(), Checks: strings(request.Body.Checks), Language: languageOf(request.Body.Language)})
	if err != nil {
		return failed[openapi.CheckTestWithAiResponseObject](ctx, err)
	}
	out := openapi.CheckTestWithAi200JSONResponse{Findings: make([]openapi.AiFinding, len(got.Findings)), Usage: usageOf(got.Usage)}
	for i, f := range got.Findings {
		out.Findings[i] = openapi.AiFinding{
			Severity: openapi.AiFindingSeverity(f.Severity), QuestionId: httpapi.ParseUUID(f.QuestionID), Number: f.Number,
			Type: openapi.QuestionType(f.Type), Title: f.Title, Issue: f.Issue, Fix: fixOf(f.Fix),
		}
	}
	return out, nil
}

func fixOf(f prompts.Fix) openapi.AiFix {
	out := openapi.AiFix{Kind: openapi.AiFixKind(f.Kind), Description: f.Description}
	if f.Kind == prompts.FixNone {
		return out
	}
	out.Label = &f.Label
	payload := openapi.AiFixPayload{Text: f.Text}
	if f.OptionID != "" {
		payload.OptionId = new(httpapi.ParseUUID(f.OptionID))
	}
	if f.BlankID != "" {
		payload.BlankId = new(httpapi.ParseUUID(f.BlankID))
	}
	out.Payload = &payload
	return out
}

// MakeTestVariant implements POST /teacher/ai/tests/{id}/variant.
func (h AI) MakeTestVariant(ctx context.Context, request openapi.MakeTestVariantRequestObject) (openapi.MakeTestVariantResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Variant.Handle(ctx, query.Variant{Actor: who, TestID: request.Id.String(), Changes: strings(request.Body.Changes), Difficulty: string(request.Body.Difficulty)})
	if err != nil {
		return failed[openapi.MakeTestVariantResponseObject](ctx, err)
	}
	out := openapi.MakeTestVariant200JSONResponse{Changes: make([]openapi.AiVariantChange, len(got.Changes)), Usage: usageOf(got.Usage)}
	for i, c := range got.Changes {
		out.Changes[i] = openapi.AiVariantChange{
			QuestionId: httpapi.ParseUUID(c.QuestionID), Number: c.Number, Kind: openapi.AiVariantKind(c.Kind),
			Title: c.Title, Before: c.Before, After: c.After, Payload: variantPayload(c),
		}
	}
	return out, nil
}

func variantPayload(c prompts.Change) openapi.AiVariantPayload {
	switch c.Kind {
	case prompts.VariantReword:
		return openapi.AiVariantPayload{Prompt: &c.Prompt}
	case prompts.VariantReorder:
		order := make([]openapi.Uuid, len(c.OptionOrder))
		for i, id := range c.OptionOrder {
			order[i] = httpapi.ParseUUID(id)
		}
		return openapi.AiVariantPayload{OptionOrder: &order}
	}
	return openapi.AiVariantPayload{OptionId: new(httpapi.ParseUUID(c.OptionID)), Text: &c.Text}
}

// WriteAiExplanation implements POST /teacher/ai/questions/{id}/explanation.
func (h AI) WriteAiExplanation(ctx context.Context, request openapi.WriteAiExplanationRequestObject) (openapi.WriteAiExplanationResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	q := query.Explanation{Actor: who, QuestionID: request.Id.String(), Language: domain.Language(request.Body.Language)}
	q.Short = request.Body.Length != nil && *request.Body.Length == openapi.Short
	got, err := h.app.Queries.Explanation.Handle(ctx, q)
	if err != nil {
		return failed[openapi.WriteAiExplanationResponseObject](ctx, err)
	}
	return openapi.WriteAiExplanation200JSONResponse{Text: got.Text, Usage: usageOf(got.Usage)}, nil
}

// SuggestAiDistractors implements POST /teacher/ai/questions/{id}/distractors.
func (h AI) SuggestAiDistractors(ctx context.Context, request openapi.SuggestAiDistractorsRequestObject) (openapi.SuggestAiDistractorsResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Distractors.Handle(ctx, query.Distractors{Actor: who, QuestionID: request.Id.String()})
	if err != nil {
		return failed[openapi.SuggestAiDistractorsResponseObject](ctx, err)
	}
	out := openapi.SuggestAiDistractors200JSONResponse{Items: make([]openapi.AiDistractor, len(got.Items)), Usage: usageOf(got.Usage)}
	for i, d := range got.Items {
		out.Items[i] = openapi.AiDistractor{Text: d.Text, Why: d.Why}
	}
	return out, nil
}

// SuggestAiScore implements POST /teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion.
func (h AI) SuggestAiScore(ctx context.Context, request openapi.SuggestAiScoreRequestObject) (openapi.SuggestAiScoreResponseObject, error) {
	who, grants, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Score.Handle(ctx, query.Score{Actor: who, Grants: grants, AttemptID: request.AttemptId.String(), QuestionID: request.QuestionId.String()})
	if err != nil {
		return failed[openapi.SuggestAiScoreResponseObject](ctx, err)
	}
	s := got.Suggestion
	return openapi.SuggestAiScore200JSONResponse{
		Score: s.Score, Confidence: openapi.AiScoreSuggestionConfidence(s.Confidence), Reasons: orEmpty(s.Reasons), Comment: s.Comment, Usage: usageOf(got.Usage),
	}, nil
}

func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// ExplainAssignmentResults implements POST /teacher/ai/assignments/{id}/insights.
func (h AI) ExplainAssignmentResults(ctx context.Context, request openapi.ExplainAssignmentResultsRequestObject) (openapi.ExplainAssignmentResultsResponseObject, error) {
	who, grants, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Insights.Handle(ctx, query.Insights{Actor: who, Grants: grants, AssignmentID: request.Id.String()})
	if err != nil {
		return failed[openapi.ExplainAssignmentResultsResponseObject](ctx, err)
	}
	out := openapi.ExplainAssignmentResults200JSONResponse{Basis: got.Basis, Items: make([]openapi.AiInsight, len(got.Items)), Usage: usageOf(got.Usage)}
	for i, in := range got.Items {
		types := make([]openapi.QuestionType, len(in.Types))
		for j, t := range in.Types {
			types[j] = openapi.QuestionType(t)
		}
		out.Items[i] = openapi.AiInsight{
			Kind: openapi.AiInsightKind(in.Kind), Title: in.Title, Body: in.Body, Action: openapi.AiInsightAction(in.Action),
			QuestionNumbers: in.QuestionNumbers, Types: types,
		}
	}
	return out, nil
}

// CleanUpQuestionBank implements POST /teacher/ai/bank/cleanup.
func (h AI) CleanUpQuestionBank(ctx context.Context, request openapi.CleanUpQuestionBankRequestObject) (openapi.CleanUpQuestionBankResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Queries.Cleanup.Handle(ctx, query.Cleanup{Actor: who, Checks: strings(request.Body.Checks), Language: languageOf(request.Body.Language)})
	if err != nil {
		return failed[openapi.CleanUpQuestionBankResponseObject](ctx, err)
	}
	out := openapi.CleanUpQuestionBank200JSONResponse{
		Scanned: got.Scanned, Truncated: got.Truncated, Usage: usageOf(got.Usage),
		Duplicates: make([]openapi.AiDuplicate, len(got.Findings.Duplicates)), Tags: make([]openapi.AiTagSuggestion, len(got.Findings.Tags)),
	}
	for i, d := range got.Findings.Duplicates {
		out.Duplicates[i] = openapi.AiDuplicate{A: questionRef(d.A), B: questionRef(d.B), Why: d.Why}
	}
	for i, t := range got.Findings.Tags {
		out.Tags[i] = openapi.AiTagSuggestion{QuestionId: httpapi.ParseUUID(t.Question.ID), Prompt: shorten(t.Question.Prompt), Tags: t.Tags}
		if t.Level != "" {
			out.Tags[i].Level = new(openapi.QuestionLevel(t.Level))
		}
	}
	return out, nil
}

func questionRef(q domain.BankQuestion) openapi.AiQuestionRef {
	return openapi.AiQuestionRef{QuestionId: httpapi.ParseUUID(q.ID), Prompt: shorten(q.Prompt), UsedInTests: q.UsedInTests}
}

func shorten(s string) string {
	runes := []rune(s)
	if len(runes) > 300 {
		runes = runes[:300]
	}
	return string(runes)
}
