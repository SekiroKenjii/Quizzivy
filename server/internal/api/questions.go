package api

import (
	"context"
	"errors"
	"strconv"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/questions"
)

// ListQuestions implements GET /admin/questions -- the §8 bank, with type and
// tag filters plus accent-insensitive search (D-11).
func (s *Server) ListQuestions(ctx context.Context, request openapi.ListQuestionsRequestObject) (openapi.ListQuestionsResponseObject, error) {
	if s.Deps.Questions == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := questions.ListInput{}
	if request.Params.Type != nil {
		t := questions.Type(*request.Params.Type)
		in.Type = &t
	}
	if request.Params.Tag != nil {
		in.Tag = *request.Params.Tag
	}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Cursor != nil {
		in.Cursor = string(*request.Params.Cursor)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, next, err := s.Deps.Questions.List(ctx, in)
	if errors.Is(err, questions.ErrBadCursor) {
		return openapi.ListQuestions400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			authError(ctx, openapi.VALIDATIONFAILED, "Con trỏ phân trang không hợp lệ."))}, nil
	}
	if err != nil {
		return nil, err
	}

	facets, err := s.Deps.Questions.Facets(ctx, in)
	if err != nil {
		return nil, err
	}

	out := openapi.ListQuestions200JSONResponse{
		Items: make([]openapi.AdminQuestion, len(found)),
		Facets: openapi.QuestionTypeFacets{
			All:            facets.All,
			SingleChoice:   facets.ByType[questions.SingleChoice],
			MultipleChoice: facets.ByType[questions.MultipleChoice],
			TrueFalse:      facets.ByType[questions.TrueFalse],
			FillBlank:      facets.ByType[questions.FillBlank],
			ShortAnswer:    facets.ByType[questions.ShortAnswer],
		},
	}
	for i, q := range found {
		out.Items[i], err = s.toAPIQuestion(ctx, q)
		if err != nil {
			return nil, err
		}
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

func (s *Server) GetQuestion(ctx context.Context, request openapi.GetQuestionRequestObject) (openapi.GetQuestionResponseObject, error) {
	if s.Deps.Questions == nil {
		return nil, httpx.ErrNotImplemented
	}
	q, err := s.Deps.Questions.Get(ctx, request.Id.String())
	if errors.Is(err, questions.ErrNotFound) {
		return openapi.GetQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy câu hỏi."))}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := s.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.GetQuestion200JSONResponse(out), nil
}

func (s *Server) CreateQuestion(ctx context.Context, request openapi.CreateQuestionRequestObject) (openapi.CreateQuestionResponseObject, error) {
	if s.Deps.Questions == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	q, err := s.Deps.Questions.Create(ctx, questions.WriteRequest{
		Input:     toQuestionInput(*request.Body),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	if resp, handled := questionWriteError(ctx, err); handled {
		return openapi.CreateQuestion400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := s.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.CreateQuestion201JSONResponse(out), nil
}

func (s *Server) UpdateQuestion(ctx context.Context, request openapi.UpdateQuestionRequestObject) (openapi.UpdateQuestionResponseObject, error) {
	if s.Deps.Questions == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	q, err := s.Deps.Questions.Update(ctx, questions.WriteRequest{
		ID:        request.Id.String(),
		Input:     toQuestionInput(*request.Body),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	if errors.Is(err, questions.ErrNotFound) {
		return nil, httpx.ErrNotImplemented
	}
	if resp, handled := questionWriteError(ctx, err); handled {
		return openapi.UpdateQuestion400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := s.toAPIQuestion(ctx, q)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateQuestion200JSONResponse(out), nil
}

func (s *Server) DeleteQuestion(ctx context.Context, request openapi.DeleteQuestionRequestObject) (openapi.DeleteQuestionResponseObject, error) {
	if s.Deps.Questions == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := s.Deps.Questions.Delete(ctx, questions.WriteRequest{
		ID:        request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.DeleteQuestion204Response{}, nil
	case errors.Is(err, questions.ErrReferenced):
		return openapi.DeleteQuestion409JSONResponse(authError(ctx, openapi.QUESTIONREFERENCED,
			"Câu hỏi đang được dùng trong một đề nháp nên không thể xoá.")), nil
	case errors.Is(err, questions.ErrNotFound):
		return openapi.DeleteQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy câu hỏi."))}, nil
	default:
		return nil, err
	}
}

// questionWriteError maps the cross-field failures to a 400 carrying per-field
// details, so the client can put each message beside its input rather than
// showing one banner for a form with three problems.
func questionWriteError(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	var invalid *questions.ValidationError
	if errors.As(err, &invalid) {
		resp := authError(ctx, openapi.VALIDATIONFAILED, "Dữ liệu câu hỏi không hợp lệ.")
		details := map[string]interface{}{}
		for _, f := range invalid.Fields {
			if _, seen := details[f.Field]; !seen {
				details[f.Field] = f.Message
			}
		}
		resp.Error.Details = &details
		return resp, true
	}
	if errors.Is(err, questions.ErrMediaNotFound) {
		resp := authError(ctx, openapi.VALIDATIONFAILED, "Không tìm thấy tệp đính kèm.")
		resp.Error.Details = &map[string]interface{}{"mediaAssetId": "Tệp không tồn tại hoặc đã bị xoá."}
		return resp, true
	}
	return openapi.ErrorResponse{}, false
}

// toQuestionInput converts the generated body into the domain input.
//
// Points crosses here as a decimal STRING. The wire type is a float64 and the
// column is numeric(8,2); formatting with 'f' and two places is what stops a
// binary fraction becoming 2.4999999999 in the database (§13.2).
func toQuestionInput(body openapi.QuestionInput) questions.Input {
	in := questions.Input{
		Type:         questions.Type(body.Type),
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
		in.Audio = &questions.AudioPolicy{
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
			in.Options = append(in.Options, questions.OptionInput{Text: o.Text, IsCorrect: o.IsCorrect})
		}
	}
	if body.Blanks != nil {
		for _, b := range *body.Blanks {
			blank := questions.BlankInput{
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

// toAPIQuestion renders a bank question, resolving its media asset to a signed
// URL when there is one.
func (s *Server) toAPIQuestion(ctx context.Context, q questions.Question) (openapi.AdminQuestion, error) {
	points, err := strconv.ParseFloat(q.Points, 64)
	if err != nil {
		return openapi.AdminQuestion{}, err
	}

	out := openapi.AdminQuestion{
		Id:           parseUUID(q.ID),
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
			Id: parseUUID(o.ID), Ordinal: o.Ordinal, Text: o.Text, IsCorrect: o.IsCorrect,
		}
	}
	out.Options = &options

	blanks := make([]openapi.AdminQuestionBlank, len(q.Blanks))
	for i, b := range q.Blanks {
		blanks[i] = openapi.AdminQuestionBlank{
			Id: parseUUID(b.ID), Ordinal: b.Ordinal,
			AcceptedAnswers: b.AcceptedAnswers, CaseSensitive: b.CaseSensitive,
		}
	}
	out.Blanks = &blanks
	if q.MediaAssetID != nil && s.Deps.Media != nil {
		if asset, err := s.Deps.Media.Get(ctx, *q.MediaAssetID); err == nil {
			url, err := s.Deps.Media.SignedURL(ctx, asset)
			if err != nil {
				return openapi.AdminQuestion{}, err
			}
			api := toAPIMediaAsset(asset, url)
			out.Media = &api
		}
	}
	return out, nil
}
