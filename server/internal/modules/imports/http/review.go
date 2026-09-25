package http

import (
	"context"
	"encoding/json"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/actor"
)

func (h Imports) GetWordImportCapabilities(ctx context.Context, _ openapi.GetWordImportCapabilitiesRequestObject) (openapi.GetWordImportCapabilitiesResponseObject, error) {
	if h.app == nil {
		return openapi.GetWordImportCapabilities200JSONResponse{}, nil
	}
	v, err := h.app.Queries.Capabilities.Handle(ctx, query.Capabilities{})
	if err != nil {
		return nil, err
	}
	return openapi.GetWordImportCapabilities200JSONResponse{IntakeEnabled: true, ProcessingEnabled: v.Processing}, nil
}

func (h Imports) GetWordImportLimits(ctx context.Context, _ openapi.GetWordImportLimitsRequestObject) (openapi.GetWordImportLimitsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Queries.Limits.Handle(ctx, query.Limits{})
	if err != nil {
		return nil, err
	}
	out := openapi.GetWordImportLimits200JSONResponse{MaxBytes: v.MaxBytes}
	for _, f := range v.Formats {
		out.Formats = append(out.Formats, openapi.ImportLimitsFormats(f))
	}
	return out, nil
}

func (h Imports) ProcessWordImport(ctx context.Context, request openapi.ProcessWordImportRequestObject) (openapi.ProcessWordImportResponseObject, error) {
	a, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := command.Process{ImportID: request.Id.String(), RequestID: request.Body.RequestId.String(), ExpectedRevision: request.Body.ExpectedRevision, Actor: actor.Actor(a)}
	if request.Body.KeyPaper != nil {
		in.KeyPaper = *request.Body.KeyPaper
	}
	v, err := h.app.Commands.Process.Handle(ctx, in)
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.ProcessWordImport202JSONResponse(toImport(v)), nil
}

func (h Imports) CancelWordImport(ctx context.Context, request openapi.CancelWordImportRequestObject) (openapi.CancelWordImportResponseObject, error) {
	a, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Commands.Cancel.Handle(ctx, command.Cancel{ImportID: request.Id.String(), ExpectedRevision: request.Body.ExpectedRevision, Actor: actor.Actor(a)})
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.CancelWordImport200JSONResponse(toImport(v)), nil
}

func (h Imports) GetWordImportReview(ctx context.Context, request openapi.GetWordImportReviewRequestObject) (openapi.GetWordImportReviewResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Queries.Review.Handle(ctx, query.Review{ImportID: request.Id.String()})
	if err != nil {
		return importFailure(ctx, err)
	}
	out, err := toReview(v)
	return openapi.GetWordImportReview200JSONResponse(out), err
}

func (h Imports) SaveWordImportReview(ctx context.Context, request openapi.SaveWordImportReviewRequestObject) (openapi.SaveWordImportReviewResponseObject, error) {
	a, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	sections, err := reshape[[]domain.DraftSection](request.Body.Sections)
	if err != nil {
		return importFailure(ctx, domain.ErrBadDraft)
	}
	v, err := h.app.Commands.SaveReview.Handle(ctx, command.SaveReview{ImportID: request.Id.String(), ExpectedRevision: request.Body.ExpectedRevision, Title: request.Body.Title, Sections: sections, Acknowledged: request.Body.Acknowledged, Actor: actor.Actor(a)})
	if err != nil {
		return importFailure(ctx, err)
	}
	out, err := toReview(v)
	return openapi.SaveWordImportReview200JSONResponse(out), err
}

func (h Imports) AdoptWordImportReprocessed(ctx context.Context, request openapi.AdoptWordImportReprocessedRequestObject) (openapi.AdoptWordImportReprocessedResponseObject, error) {
	a, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Commands.Adopt.Handle(ctx, command.Adopt{ImportID: request.Id.String(), ExpectedRevision: request.Body.ExpectedRevision, Actor: actor.Actor(a)})
	if err != nil {
		return importFailure(ctx, err)
	}
	out, err := toReview(v)
	return openapi.AdoptWordImportReprocessed200JSONResponse(out), err
}

func (h Imports) GetWordImportSource(ctx context.Context, request openapi.GetWordImportSourceRequestObject) (openapi.GetWordImportSourceResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: request.Id.String(), Role: string(request.Params.Role)})
	if err != nil {
		return importFailure(ctx, err)
	}
	out := openapi.ImportSourceView{SourceId: v.Evidence.SourceID, Role: request.Params.Role, Filename: v.Filename, Blocks: []openapi.ImportSourceBlock{}}
	for _, b := range v.Evidence.Blocks {
		if b.Main && b.Kind == "paragraph" {
			out.Blocks = append(out.Blocks, toSourceBlock(b))
		}
	}
	noStore := "no-store"
	return openapi.GetWordImportSource200JSONResponse{Body: out, Headers: openapi.GetWordImportSource200ResponseHeaders{CacheControl: &noStore}}, nil
}

func (h Imports) CommitWordImport(ctx context.Context, request openapi.CommitWordImportRequestObject) (openapi.CommitWordImportResponseObject, error) {
	a, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Commands.Commit.Handle(ctx, command.Commit{ImportID: request.Id.String(), RequestID: request.Body.RequestId.String(), DraftRevision: request.Body.DraftRevision, Actor: actor.Actor(a)})
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.CommitWordImport200JSONResponse{TestId: httpapi.ParseUUID(v.TestID), Import: toImport(v.Import)}, nil
}

func toReview(v domain.ReviewState) (openapi.ImportReview, error) {
	out := openapi.ImportReview{ImportId: httpapi.ParseUUID(v.Draft.ImportID), Revision: v.Draft.Revision, Ready: v.Review.Ready, Reprocessed: v.Draft.Reprocessed, UpdatedAt: v.Draft.UpdatedAt}
	s := v.Review.Summary
	out.Summary = openapi.ImportReviewSummary{Sections: s.Sections, Groups: s.Groups, Questions: s.Questions, Included: s.Included, Excluded: s.Excluded, TotalPoints: s.TotalPoints, AnswersKnown: s.AnswersKnown, AnswersMissing: s.AnswersMissing, AnswersConflicting: s.AnswersConflicting, Blocking: s.Blocking, NeedsDecision: s.NeedsDecision}
	var err error
	if out.Draft, err = reshape[openapi.ImportDraft](v.Draft.Draft); err != nil {
		return out, err
	}
	out.Findings, err = reshape[[]openapi.ImportFinding](v.Review.Findings)
	return out, err
}

func toSourceBlock(b domain.EvidenceBlock) openapi.ImportSourceBlock {
	out := openapi.ImportSourceBlock{Id: b.ID, Text: b.Text, Spans: make([]openapi.ImportSourceSpan, len(b.Spans))}
	for i, span := range b.Spans {
		marks := make([]openapi.ContentMark, len(span.Marks))
		for j, m := range span.Marks {
			marks[j] = openapi.ContentMark(m)
		}
		colored := span.Colored
		out.Spans[i] = openapi.ImportSourceSpan{Start: span.Start, End: span.End, Marks: marks, Colored: &colored}
	}
	if b.TableID != "" {
		table, row, column := b.TableID, b.Row, b.Column
		out.TableId, out.Row, out.Column = &table, &row, &column
	}
	return out
}

func reshape[T any](from any) (T, error) {
	var out T
	raw, err := json.Marshal(from)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out)
}
