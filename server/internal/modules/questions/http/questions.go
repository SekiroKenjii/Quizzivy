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

func msgQuestionNotFound(ctx context.Context) string {
	return httpx.Text(ctx, "Không tìm thấy câu hỏi.", "The question was not found.")
}

// ListQuestions implements GET /teacher/questions -- the §8 bank, with type and
// tag filters plus accent-insensitive search (D-11).
func (h Questions) ListQuestions(ctx context.Context, request openapi.ListQuestionsRequestObject) (openapi.ListQuestionsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.ListInput{Scope: httpapi.ScopeFromContext(ctx).Own()}
	questionListDimensions(&in, request.Params)
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
	out.Facets.Levels.PreA1 = facets.ByLevel["pre_a1"]
	out.Facets.Levels.A1 = facets.ByLevel["a1"]
	out.Facets.Levels.A2 = facets.ByLevel["a2"]
	out.Facets.Levels.B1 = facets.ByLevel["b1"]
	out.Facets.Levels.B2 = facets.ByLevel["b2"]
	out.Facets.Levels.C1 = facets.ByLevel["c1"]
	out.Facets.Levels.C2 = facets.ByLevel["c2"]
	out.Facets.Skills.Grammar = facets.BySkill["grammar"]
	out.Facets.Skills.Vocabulary = facets.BySkill["vocabulary"]
	out.Facets.Skills.Reading = facets.BySkill["reading"]
	out.Facets.Skills.Listening = facets.BySkill["listening"]
	out.Facets.Skills.Writing = facets.BySkill["writing"]
	out.Facets.Skills.Speaking = facets.BySkill["speaking"]
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
	q, err := h.app.Queries.Get.Handle(ctx, query.Get{ID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx)})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgQuestionNotFound(ctx)))}, nil
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
	req, ok := writeRequest(ctx, "", ToQuestionInput(*request.Body))
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	q, err := h.app.Commands.Create.Handle(ctx, command.Create{Request: req})
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
	req, ok := writeRequest(ctx, request.Id.String(), domain.Input{})
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	q, err := h.app.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: req})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.DuplicateQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgQuestionNotFound(ctx)))}, nil
	}
	if resp, handled := questionWriteError(ctx, err); handled {
		return openapi.DuplicateQuestion400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
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
	req, ok := writeRequest(ctx, request.Id.String(), ToQuestionInput(*request.Body))
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	q, err := h.app.Commands.Update.Handle(ctx, command.Update{Request: req})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.UpdateQuestion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgQuestionNotFound(ctx)))}, nil
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
	req, ok := writeRequest(ctx, request.Id.String(), domain.Input{})
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Request: req})
	switch {
	case err == nil:
		return openapi.DeleteQuestion204Response{}, nil
	case errors.Is(err, domain.ErrReferenced):
		resp := httpapi.Error(ctx, openapi.QUESTIONREFERENCED,
			httpx.Text(ctx, "Câu hỏi đang được dùng trong một đề nháp nên không thể xoá.",
				"The question is used in a draft test, so it cannot be deleted."))
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
			httpapi.NotFound(ctx, msgQuestionNotFound(ctx)))}, nil
	default:
		return nil, err
	}
}

func questionWriteError(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	var invalid *domain.ValidationError
	if errors.As(err, &invalid) {
		resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Dữ liệu câu hỏi không hợp lệ.", "The question data is not valid."))
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
		resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Không tìm thấy tệp đính kèm.", "The attachment was not found."))
		resp.Error.Details = &map[string]interface{}{"mediaAssetId": httpx.Text(ctx, "Tệp không tồn tại hoặc đã bị xoá.", "The file does not exist or has been deleted.")}
		return resp, true
	}
	return openapi.ErrorResponse{}, false
}

// ToQuestionInput projects the API write shape into the shared question validation input.
func ToQuestionInput(body openapi.QuestionInput) domain.Input {
	in := domain.Input{
		Type:               domain.Type(body.Type),
		Level:              (*domain.Level)(body.Level),
		Skill:              (*domain.Skill)(body.Skill),
		Prompt:             body.Prompt,
		PromptContent:      body.PromptContent,
		Transcript:         body.Transcript,
		Points:             strconv.FormatFloat(float64(body.Points), 'f', -1, 64),
		Explanation:        body.Explanation,
		ExplanationContent: body.ExplanationContent,
		SampleAnswer:       body.SampleAnswer,
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
			option.ID = optionID(o.Id)
			in.Options = append(in.Options, option)
		}
	}
	if body.Blanks != nil {
		for _, b := range *body.Blanks {
			blank := domain.BlankInput{
				GapID:           b.GapId,
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
		Id:                 httpapi.ParseUUID(q.ID),
		Type:               openapi.QuestionType(q.Type),
		Level:              (*openapi.QuestionLevel)(q.Level),
		Skill:              (*openapi.QuestionSkill)(q.Skill),
		Prompt:             q.Prompt,
		PromptContent:      q.PromptContent,
		Points:             points,
		Explanation:        q.Explanation,
		ExplanationContent: q.ExplanationContent,
		SampleAnswer:       q.SampleAnswer,
		Tags:               q.Tags,
		UsedInTests:        &q.UsedInTests,
		Transcript:         q.Transcript,
		CreatedAt:          q.CreatedAt,
		UpdatedAt:          q.UpdatedAt,
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
			GapId: b.GapID,
			Id:    httpapi.ParseUUID(b.ID), Ordinal: b.Ordinal,
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

	updated, err := h.app.Commands.AddTags.Handle(ctx, command.AddTags{IDs: ids, Tags: request.Body.Tags, Scope: httpapi.ScopeFromContext(ctx)})
	if err != nil {
		return nil, err
	}
	return openapi.TagQuestions200JSONResponse{Updated: updated}, nil
}

func writeRequest(ctx context.Context, id string, input domain.Input) (domain.WriteRequest, bool) {
	actor, ok := httpapi.ActorFromContext(ctx)
	return domain.WriteRequest{ID: id, Input: input, ActorID: actor.ID, All: actor.Scope.All, IP: actor.IP, UserAgent: actor.UserAgent}, ok
}

func optionID(id *openapi.Uuid) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func questionListDimensions(in *domain.ListInput, params openapi.ListQuestionsParams) {
	if params.Type != nil {
		for _, t := range *params.Type {
			in.Types = append(in.Types, domain.Type(t))
		}
	}
	if params.Level != nil {
		for _, level := range *params.Level {
			in.Levels = append(in.Levels, domain.Level(level))
		}
	}
	if params.Skill != nil {
		for _, skill := range *params.Skill {
			in.Skills = append(in.Skills, domain.Skill(skill))
		}
	}
	if params.TagMatch != nil {
		in.TagMatch = string(*params.TagMatch)
	}
}
