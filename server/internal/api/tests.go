package api

import (
	"context"
	"errors"
	"strconv"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/tests"
)

func (s *Server) ListTests(ctx context.Context, request openapi.ListTestsRequestObject) (openapi.ListTestsResponseObject, error) {
	if s.Deps.Tests == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := tests.ListInput{}
	if request.Params.Status != nil {
		status := tests.Status(*request.Params.Status)
		in.Status = &status
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

	found, next, err := s.Deps.Tests.List(ctx, in)
	if errors.Is(err, tests.ErrBadCursor) {
		return openapi.ListTests400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			authError(ctx, openapi.VALIDATIONFAILED, "Con trỏ phân trang không hợp lệ."))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := openapi.ListTests200JSONResponse{Items: make([]openapi.Test, len(found))}
	for i, t := range found {
		if out.Items[i], err = toAPITest(t); err != nil {
			return nil, err
		}
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

func (s *Server) GetTest(ctx context.Context, request openapi.GetTestRequestObject) (openapi.GetTestResponseObject, error) {
	if s.Deps.Tests == nil {
		return nil, httpx.ErrNotImplemented
	}
	t, err := s.Deps.Tests.Get(ctx, request.Id.String())
	if errors.Is(err, tests.ErrNotFound) {
		return openapi.GetTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy đề."))}, nil
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

func (s *Server) CreateTest(ctx context.Context, request openapi.CreateTestRequestObject) (openapi.CreateTestResponseObject, error) {
	if s.Deps.Tests == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, "")
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := s.Deps.Tests.Create(ctx, req, request.Body.Title, request.Body.Description)
	if err != nil {
		return nil, err
	}
	out, err := toAPITest(t)
	if err != nil {
		return nil, err
	}
	return openapi.CreateTest201JSONResponse(out), nil
}

func (s *Server) UpdateTest(ctx context.Context, request openapi.UpdateTestRequestObject) (openapi.UpdateTestResponseObject, error) {
	if s.Deps.Tests == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := s.Deps.Tests.Update(ctx, req, toUpdateInput(*request.Body))
	switch {
	case err == nil:
	case errors.Is(err, tests.ErrStaleWrite):
		return openapi.UpdateTest409JSONResponse(authError(ctx, openapi.STALEWRITE,
			"Đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi lưu.")), nil
	case errors.Is(err, tests.ErrNotFound):
		return openapi.UpdateTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy đề."))}, nil
	case errors.Is(err, tests.ErrUnknownQuestion):
		resp := authError(ctx, openapi.VALIDATIONFAILED, "Đề tham chiếu câu hỏi không tồn tại.")
		resp.Error.Details = &map[string]interface{}{"sections": "Một câu hỏi trong đề đã bị xoá."}
		return openapi.UpdateTest400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(resp)}, nil
	default:
		var invalid *tests.ValidationError
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

func (s *Server) DuplicateTest(ctx context.Context, request openapi.DuplicateTestRequestObject) (openapi.DuplicateTestResponseObject, error) {
	if s.Deps.Tests == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	t, err := s.Deps.Tests.Duplicate(ctx, req)
	if errors.Is(err, tests.ErrNotFound) {
		return openapi.DuplicateTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy đề."))}, nil
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

func testRequest(ctx context.Context, id string) (tests.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return tests.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return tests.Request{
		ID: id, ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func testValidationError(ctx context.Context, invalid *tests.ValidationError) openapi.ErrorResponse {
	resp := authError(ctx, openapi.VALIDATIONFAILED, "Dữ liệu đề không hợp lệ.")
	details := map[string]interface{}{}
	for _, f := range invalid.Fields {
		if _, seen := details[f.Field]; !seen {
			details[f.Field] = f.Message
		}
	}
	resp.Error.Details = &details
	return resp
}

func toUpdateInput(body openapi.UpdateTestJSONRequestBody) tests.UpdateInput {
	in := tests.UpdateInput{
		ExpectedUpdatedAt: body.ExpectedUpdatedAt,
		Title:             body.Title,
	}
	if body.Description != nil {
		in.Description = body.Description
		in.SetDescription = true
	}
	if body.Status != nil {
		status := tests.Status(*body.Status)
		in.Status = &status
	}
	if body.Sections != nil {
		in.SetSections = true
		in.Sections = make([]tests.SectionInput, len(*body.Sections))
		for i, sec := range *body.Sections {
			out := tests.SectionInput{Title: sec.Title, Instructions: sec.Instructions}
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

func toAPITest(t tests.Test) (openapi.Test, error) {
	points, err := strconv.ParseFloat(t.TotalPoints, 64)
	if err != nil {
		return openapi.Test{}, err
	}

	out := openapi.Test{
		Id:             parseUUID(t.ID),
		Title:          t.Title,
		Description:    t.Description,
		Status:         openapi.TestStatus(t.Status),
		CurrentVersion: t.CurrentVersion,
		TotalPoints:    points,
		QuestionCount:  t.QuestionCount,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		DeletedAt:      t.DeletedAt,
		Sections:       make([]openapi.TestSection, len(t.Sections)),
	}
	for i, sec := range t.Sections {
		ids := make([]openapi.Uuid, len(sec.QuestionIDs))
		for j, id := range sec.QuestionIDs {
			ids[j] = parseUUID(id)
		}
		out.Sections[i] = openapi.TestSection{
			Id:           parseUUID(sec.ID),
			Ordinal:      sec.Ordinal,
			Title:        sec.Title,
			Instructions: sec.Instructions,
			QuestionIds:  ids,
		}
	}
	return out, nil
}
