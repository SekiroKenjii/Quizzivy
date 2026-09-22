package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strconv"
)

// ListQuestions implements GET /admin/questions -- the §8 bank, with type and
// tag filters plus accent-insensitive search (D-11).
func (h Questions) ListQuestions(ctx context.Context, request openapi.ListQuestionsRequestObject) (openapi.ListQuestionsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.ListInput{}
	if request.Params.Type != nil {
		for _, t := range *request.Params.Type {
			in.Types = append(in.Types, domain.Type(t))
		}
	}
	if request.Params.Tag != nil {
		in.Tags = append(in.Tags, *request.Params.Tag...)
	}
	in.HasAudio = request.Params.HasAudio
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in})
	found, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}

	facets, err := h.app.Queries.Facets.Handle(ctx, query.Facets{Input: in})
	if err != nil {
		return nil, err
	}

	tagList, err := h.app.Queries.Tags.Handle(ctx, query.Tags{Input: in})
	if err != nil {
		return nil, err
	}

	countsResult, err := h.app.Queries.Counts.Handle(ctx, query.Counts{Input: in})
	bankTotal := countsResult.Total
	if err != nil {
		return nil, err
	}

	out := openapi.ListQuestions200JSONResponse{
		Items: make([]openapi.AdminQuestion, len(found)),
		Facets: openapi.QuestionTypeFacets{
			All:            facets.All,
			SingleChoice:   facets.ByType[domain.SingleChoice],
			MultipleChoice: facets.ByType[domain.MultipleChoice],
			TrueFalse:      facets.ByType[domain.TrueFalse],
			FillBlank:      facets.ByType[domain.FillBlank],
			ShortAnswer:    facets.ByType[domain.ShortAnswer],
		},
		Tags:      tagList,
		BankTotal: bankTotal,
		Page:      page.Number,
		PageSize:  page.Size,
		Total:     page.Total,
	}
	for i, q := range found {
		out.Items[i], err = h.toAPIQuestion(ctx, q)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h Questions) GetQuestion(ctx context.Context, request openapi.GetQuestionRequestObject) (openapi.GetQuestionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	q, err := h.app.Queries.Get.Handle(ctx, query.Get{ID: request.Id.String()})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy câu hỏi."))}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := h.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.GetQuestion200JSONResponse(out), nil
}

func (h Questions) CreateQuestion(ctx context.Context, request openapi.CreateQuestionRequestObject) (openapi.CreateQuestionResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	q, err := h.app.Commands.Create.Handle(ctx, command.Create{Request: domain.WriteRequest{
		Input:     toQuestionInput(*request.Body),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	if resp, handled := questionWriteError(ctx, err); handled {
		return openapi.CreateQuestion400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := h.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.CreateQuestion201JSONResponse(out), nil
}

func (h Questions) DuplicateQuestion(ctx context.Context, request openapi.DuplicateQuestionRequestObject) (openapi.DuplicateQuestionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	q, err := h.app.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: domain.WriteRequest{
		ID:        request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.DuplicateQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy câu hỏi."))}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := h.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.DuplicateQuestion201JSONResponse(out), nil
}

func (h Questions) UpdateQuestion(ctx context.Context, request openapi.UpdateQuestionRequestObject) (openapi.UpdateQuestionResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	q, err := h.app.Commands.Update.Handle(ctx, command.Update{Request: domain.WriteRequest{
		ID:        request.Id.String(),
		Input:     toQuestionInput(*request.Body),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	if errors.Is(err, domain.ErrNotFound) {
		return nil, httpx.ErrNotImplemented
	}
	if resp, handled := questionWriteError(ctx, err); handled {
		return openapi.UpdateQuestion400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := h.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateQuestion200JSONResponse(out), nil
}

func (h Questions) DeleteQuestion(ctx context.Context, request openapi.DeleteQuestionRequestObject) (openapi.DeleteQuestionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Request: domain.WriteRequest{
		ID:        request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	switch {
	case err == nil:
		return openapi.DeleteQuestion204Response{}, nil
	case errors.Is(err, domain.ErrReferenced):
		resp := httpapi.Error(ctx, openapi.QUESTIONREFERENCED,
			"Câu hỏi đang được dùng trong một đề nháp nên không thể xoá.")
		var blocked *domain.ReferencedError
		if errors.As(err, &blocked) {
			refs := make([]openapi.ReferencingTest, len(blocked.Tests))
			for i, ref := range blocked.Tests {
				refs[i] = openapi.ReferencingTest{Id: httpapi.ParseUUID(ref.ID), Title: ref.Title}
			}
			resp.Error.Details = &map[string]interface{}{"tests": refs}
		}
		return openapi.DeleteQuestion409JSONResponse(resp), nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy câu hỏi."))}, nil
	default:
		return nil, err
	}
}

func questionWriteError(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	var invalid *domain.ValidationError
	if errors.As(err, &invalid) {
		resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Dữ liệu câu hỏi không hợp lệ.")
		details := map[string]interface{}{}
		for _, f := range invalid.Fields {
			if _, seen := details[f.Field]; !seen {
				details[f.Field] = f.Message
			}
		}
		resp.Error.Details = &details
		return resp, true
	}
	if errors.Is(err, domain.ErrMediaNotFound) {
		resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Không tìm thấy tệp đính kèm.")
		resp.Error.Details = &map[string]interface{}{"mediaAssetId": "Tệp không tồn tại hoặc đã bị xoá."}
		return resp, true
	}
	return openapi.ErrorResponse{}, false
}

func toQuestionInput(body openapi.QuestionInput) domain.Input {
	in := domain.Input{
		Type:         domain.Type(body.Type),
		Prompt:       body.Prompt,
		Transcript:   body.Transcript,
		Points:       strconv.FormatFloat(float64(body.Points), 'f', 2, 64),
		Explanation:  body.Explanation,
		SampleAnswer: body.SampleAnswer,
	}
	if body.MediaAssetId != nil {
		id := body.MediaAssetId.String()
		in.MediaAssetID = &id
	}
	if body.Audio != nil {
		in.Audio = &domain.AudioPolicy{
			MaxPlays:                  body.Audio.MaxPlays,
			AllowSeek:                 body.Audio.AllowSeek,
			ShowTranscriptAfterSubmit: body.Audio.ShowTranscriptAfterSubmit,
		}
	}
	if body.Tags != nil {
		in.Tags = *body.Tags
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if body.Options != nil {
		for _, o := range *body.Options {
			option := domain.OptionInput{Text: o.Text, Content: o.Content, IsCorrect: o.IsCorrect}
			if o.Id != nil {
				id := o.Id.String()
				option.ID = &id
			}
			in.Options = append(in.Options, option)
		}
	}
	if body.Blanks != nil {
		for _, b := range *body.Blanks {
			blank := domain.BlankInput{
				Ordinal:         b.Ordinal,
				AcceptedAnswers: b.AcceptedAnswers,
			}
			if b.CaseSensitive != nil {
				blank.CaseSensitive = *b.CaseSensitive
			}
			in.Blanks = append(in.Blanks, blank)
		}
	}
	return in
}

func (h Questions) toAPIQuestion(ctx context.Context, q domain.Question) (openapi.AdminQuestion, error) {
	points, err := strconv.ParseFloat(q.Points, 64)
	if err != nil {
		return openapi.AdminQuestion{}, err
	}

	out := openapi.AdminQuestion{
		Id:           httpapi.ParseUUID(q.ID),
		Type:         openapi.QuestionType(q.Type),
		Prompt:       q.Prompt,
		Points:       points,
		Explanation:  q.Explanation,
		SampleAnswer: q.SampleAnswer,
		Tags:         q.Tags,
		UsedInTests:  &q.UsedInTests,
		Transcript:   q.Transcript,
		CreatedAt:    q.CreatedAt,
		UpdatedAt:    q.UpdatedAt,
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	if q.UsedIn != nil {
		refs := make([]openapi.ReferencingTest, len(q.UsedIn))
		for i, ref := range q.UsedIn {
			refs[i] = openapi.ReferencingTest{Id: httpapi.ParseUUID(ref.ID), Title: ref.Title}
		}
		out.UsedIn = &refs
	}
	if q.Audio != nil {
		out.Audio = &openapi.AudioPolicy{
			MaxPlays:                  q.Audio.MaxPlays,
			AllowSeek:                 q.Audio.AllowSeek,
			ShowTranscriptAfterSubmit: q.Audio.ShowTranscriptAfterSubmit,
		}
	}

	options := make([]openapi.AdminQuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = openapi.AdminQuestionOption{
			Id: httpapi.ParseUUID(o.ID), Ordinal: o.Ordinal, Text: o.Text, Content: o.Content, IsCorrect: o.IsCorrect,
		}
	}
	out.Options = &options

	blanks := make([]openapi.AdminQuestionBlank, len(q.Blanks))
	for i, b := range q.Blanks {
		blanks[i] = openapi.AdminQuestionBlank{
			Id: httpapi.ParseUUID(b.ID), Ordinal: b.Ordinal,
			AcceptedAnswers: b.AcceptedAnswers, CaseSensitive: b.CaseSensitive,
		}
	}
	out.Blanks = &blanks
	if q.MediaAssetID != nil && h.media != nil {
		if asset, err := h.media.Get(ctx, *q.MediaAssetID); err == nil {
			url, err := h.media.SignedURL(ctx, asset)
			if err != nil {
				return openapi.AdminQuestion{}, err
			}
			api := mediahttp.ToAPIMediaAsset(asset, url)
			out.Media = &api
		}
	}
	return out, nil
}

// TagQuestions implements A-06's bulk "Gắn thẻ".
func (h Questions) TagQuestions(ctx context.Context, request openapi.TagQuestionsRequestObject) (openapi.TagQuestionsResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	ids := make([]string, len(request.Body.QuestionIds))
	for i, id := range request.Body.QuestionIds {
		ids[i] = id.String()
	}

	updated, err := h.app.Commands.AddTags.Handle(ctx, command.AddTags{IDs: ids, Tags: request.Body.Tags})
	if err != nil {
		return nil, err
	}
	return openapi.TagQuestions200JSONResponse{Updated: updated}, nil
}
