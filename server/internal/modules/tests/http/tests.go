package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strconv"
)

const msgTestNotFound = "Không tìm thấy đề."

func (h Tests) ListTests(ctx context.Context, request openapi.ListTestsRequestObject) (openapi.ListTestsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.ListInput{}
	if request.Params.Status != nil {
		status := domain.Status(*request.Params.Status)
		in.Status = &status
	}
	if request.Params.Tag != nil {
		in.Tags = append(in.Tags, *request.Params.Tag...)
	}
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

	out := openapi.ListTests200JSONResponse{
		Items: make([]openapi.Test, len(found)),
		Facets: openapi.TestStatusFacets{
			All:       facets.All,
			Draft:     facets.Draft,
			Published: facets.Published,
			Archived:  facets.Archived,
		},
		Tags:     tagList,
		Page:     page.Number,
		PageSize: page.Size,
		Total:    page.Total,
	}
	for i, t := range found {
		if out.Items[i], err = toAPITest(t); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h Tests) GetTest(ctx context.Context, request openapi.GetTestRequestObject) (openapi.GetTestResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	t, err := h.app.Queries.Get.Handle(ctx, query.Get{ID: request.Id.String()})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgTestNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := toAPITest(t)
	if err != nil {
		return nil, err
	}
	return openapi.GetTest200JSONResponse(out), nil
}

func (h Tests) CreateTest(ctx context.Context, request openapi.CreateTestRequestObject) (openapi.CreateTestResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, "")
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := h.app.Commands.Create.Handle(ctx, command.Create{Request: req, Title: request.Body.Title, Description: request.Body.Description})
	if err != nil {
		return nil, err
	}
	out, err := toAPITest(t)
	if err != nil {
		return nil, err
	}
	return openapi.CreateTest201JSONResponse(out), nil
}

func (h Tests) UpdateTest(ctx context.Context, request openapi.UpdateTestRequestObject) (openapi.UpdateTestResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := h.app.Commands.Update.Handle(ctx, command.Update{Request: req, Input: toUpdateInput(*request.Body)})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrStaleWrite):
		return openapi.UpdateTest409JSONResponse(httpapi.Error(ctx, openapi.STALEWRITE,
			"Đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi lưu.")), nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.UpdateTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgTestNotFound))}, nil
	case errors.Is(err, domain.ErrUnknownQuestion):
		resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Đề tham chiếu câu hỏi không tồn tại.")
		resp.Error.Details = &map[string]interface{}{"sections": "Một câu hỏi trong đề đã bị xoá."}
		return openapi.UpdateTest400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	default:
		var invalid *domain.ValidationError
		if errors.As(err, &invalid) {
			return openapi.UpdateTest400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
				testValidationError(ctx, invalid))}, nil
		}
		return nil, err
	}

	out, err := toAPITest(t)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateTest200JSONResponse(out), nil
}

func (h Tests) DuplicateTest(ctx context.Context, request openapi.DuplicateTestRequestObject) (openapi.DuplicateTestResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := h.app.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: req})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.DuplicateTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgTestNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := toAPITest(t)
	if err != nil {
		return nil, err
	}
	return openapi.DuplicateTest201JSONResponse(out), nil
}

func testRequest(ctx context.Context, id string) (domain.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return domain.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return domain.Request{
		ID: id, ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func testValidationError(ctx context.Context, invalid *domain.ValidationError) openapi.ErrorResponse {
	resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Dữ liệu đề không hợp lệ.")
	details := map[string]interface{}{}
	for _, f := range invalid.Fields {
		if _, seen := details[f.Field]; !seen {
			details[f.Field] = f.Message
		}
	}
	resp.Error.Details = &details
	return resp
}

func toUpdateInput(body openapi.UpdateTestJSONRequestBody) domain.UpdateInput {
	in := domain.UpdateInput{
		ExpectedUpdatedAt: body.ExpectedUpdatedAt,
		Title:             body.Title,
	}
	if body.Description != nil {
		in.Description = body.Description
		in.SetDescription = true
	}
	if body.Status != nil {
		status := domain.Status(*body.Status)
		in.Status = &status
	}
	if body.Sections != nil {
		in.SetSections = true
		in.Sections = make([]domain.SectionInput, len(*body.Sections))
		for i, sec := range *body.Sections {
			out := domain.SectionInput{Title: sec.Title, Instructions: sec.Instructions}
			if sec.Id != nil {
				out.ID = sec.Id.String()
			}
			out.QuestionIDs = make([]string, len(sec.QuestionIds))
			for j, id := range sec.QuestionIds {
				out.QuestionIDs[j] = id.String()
			}
			in.Sections[i] = out
		}
	}
	return in
}

func toAPITest(t domain.Test) (openapi.Test, error) {
	points, err := strconv.ParseFloat(t.TotalPoints, 64)
	if err != nil {
		return openapi.Test{}, err
	}

	out := openapi.Test{
		Id:             httpapi.ParseUUID(t.ID),
		Title:          t.Title,
		Description:    t.Description,
		Status:         openapi.TestStatus(t.Status),
		CurrentVersion: t.CurrentVersion,
		TotalPoints:    points,
		QuestionCount:  t.QuestionCount,
		AudioCount:     t.AudioCount,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		DeletedAt:      t.DeletedAt,
		Sections:       make([]openapi.TestSection, len(t.Sections)),
	}
	for i, sec := range t.Sections {
		ids := make([]openapi.Uuid, len(sec.QuestionIDs))
		for j, id := range sec.QuestionIDs {
			ids[j] = httpapi.ParseUUID(id)
		}
		out.Sections[i] = openapi.TestSection{
			Id:           httpapi.ParseUUID(sec.ID),
			Ordinal:      sec.Ordinal,
			Title:        sec.Title,
			Instructions: sec.Instructions,
			QuestionIds:  ids,
		}
	}
	return out, nil
}

func (h Tests) ListTestVersions(ctx context.Context, request openapi.ListTestVersionsRequestObject) (openapi.ListTestVersionsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	versions, err := h.app.Queries.ListVersions.Handle(ctx, query.ListVersions{TestID: request.Id.String()})
	if err != nil {
		return nil, err
	}

	items := make([]openapi.TestVersion, len(versions))
	for i, v := range versions {
		points, err := strconv.ParseFloat(v.TotalPoints, 64)
		if err != nil {
			return nil, err
		}
		items[i] = openapi.TestVersion{
			Id:            httpapi.ParseUUID(v.ID),
			Version:       v.Version,
			TotalPoints:   points,
			QuestionCount: v.QuestionCount,
			AudioCount:    v.AudioCount,
			ManualCount:   v.ManualCount,
			PublishedAt:   v.PublishedAt,
			PublishedBy:   v.PublishedBy,
		}
	}
	return openapi.ListTestVersions200JSONResponse{Items: items}, nil
}

func (h Tests) PreviewTest(ctx context.Context, request openapi.PreviewTestRequestObject) (openapi.PreviewTestResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	version := 0
	if request.Params.Version != nil {
		version = *request.Params.Version
	}

	previewResult, err := h.app.Queries.Preview.Handle(ctx, query.Preview{TestID: request.Id.String(), Version: version})
	resolved, questions := previewResult.Version, previewResult.Questions
	if errors.Is(err, domain.ErrNotPublished) {
		return openapi.PreviewTest409JSONResponse(httpapi.Error(ctx,
			openapi.TESTNOTPUBLISHED, "Đề này chưa được phát hành.")), nil
	}
	if err != nil {
		return nil, err
	}

	out, err := h.toStudentQuestions(ctx, questions)
	if err != nil {
		return nil, err
	}
	groups, err := h.toStudentGroups(ctx, previewResult.Groups)
	if err != nil {
		return nil, err
	}
	sections := toPreviewSections(previewResult.Sections)
	return openapi.PreviewTest200JSONResponse{Version: resolved, Questions: out, Groups: &groups, Sections: &sections}, nil
}

// toStudentQuestions maps the frozen rows to the student payload.
//
// The signed URL is minted here rather than in the store because it is an
// HTTP-layer concern with its own TTL (§11.2), and because the store
// deliberately never selects anything a student may not see.
func (h Tests) toStudentQuestions(
	ctx context.Context, questions []domain.PreviewQuestion,
) ([]openapi.StudentQuestion, error) {
	out := make([]openapi.StudentQuestion, len(questions))
	for i, q := range questions {
		sq, err := toStudentQuestion(q)
		if err != nil {
			return nil, err
		}
		asset, err := h.previewAsset(ctx, q.MediaAssetID)
		if err != nil {
			return nil, err
		}
		if asset != nil {
			sq.Media = asset
		}
		out[i] = sq
	}
	return out, nil
}

func toStudentQuestion(q domain.PreviewQuestion) (openapi.StudentQuestion, error) {
	points, err := strconv.ParseFloat(q.Points, 64)
	if err != nil {
		return openapi.StudentQuestion{}, err
	}
	sq := openapi.StudentQuestion{
		Id:            httpapi.ParseUUID(q.ID),
		SectionId:     httpapi.ParseUUID(q.SectionID),
		Type:          openapi.QuestionType(q.Type),
		Prompt:        q.Prompt,
		PromptContent: q.PromptContent,
		Points:        points,
	}
	if len(q.Options) > 0 {
		options := make([]openapi.StudentOption, len(q.Options))
		for j, o := range q.Options {
			options[j] = openapi.StudentOption{Id: httpapi.ParseUUID(o.ID), Text: o.Text, Content: o.Content}
		}
		sq.Options = &options
	}
	if len(q.Blanks) > 0 {
		blanks := make([]openapi.StudentBlank, len(q.Blanks))
		for j, b := range q.Blanks {
			blanks[j] = openapi.StudentBlank{
				GapId:         b.GapID,
				Id:            httpapi.ParseUUID(b.ID),
				Ordinal:       b.Ordinal,
				CaseSensitive: b.CaseSensitive,
			}
		}
		sq.Blanks = &blanks
	}
	if q.AllowSeek != nil && q.ShowScript != nil {
		sq.Audio = &openapi.AudioPolicy{
			MaxPlays:                  q.MaxPlays,
			AllowSeek:                 *q.AllowSeek,
			ShowTranscriptAfterSubmit: *q.ShowScript,
		}
	}
	return sq, nil
}

func (h Tests) previewAsset(ctx context.Context, assetID *string) (*openapi.MediaAsset, error) {
	if assetID == nil || h.media == nil {
		return nil, nil
	}
	asset, err := h.media.Get(ctx, *assetID)
	if err != nil {
		return nil, err
	}
	url, err := h.media.SignedURL(ctx, asset)
	if err != nil {
		return nil, err
	}
	out := mediahttp.ToAPIMediaAsset(asset, url)
	return &out, nil
}
