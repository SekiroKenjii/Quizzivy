package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

const msgStudentNotFound = "Không tìm thấy học viên."

// ListStudents backs §8's students table (G-07) and the two pickers that add a
// student to a class (G-06) or to an assignment (G-01).
func (h Identity) ListStudents(ctx context.Context, request openapi.ListStudentsRequestObject) (openapi.ListStudentsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.StudentQuery{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.ClassId != nil {
		in.ClassID = request.Params.ClassId.String()
	}
	if request.Params.Status != nil {
		in.Status = domain.StudentStatus(*request.Params.Status)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	listStudentsResult, err := h.app.Queries.ListStudents.Handle(ctx, query.ListStudents{Query: in})
	found, page := listStudentsResult.Items, listStudentsResult.Page
	if err != nil {
		return nil, err
	}

	facets, err := h.app.Queries.StudentFacets.Handle(ctx, query.StudentFacets{Query: in})
	if err != nil {
		return nil, err
	}

	out := openapi.ListStudents200JSONResponse{
		Items: make([]openapi.StudentRow, len(found)),
		Facets: openapi.StudentFacets{
			Total:           facets.Total,
			ActiveLast7Days: facets.ActiveLast7Days,
		},
		Page:     page.Number,
		PageSize: page.Size,
		Total:    page.Total,
	}
	for i, student := range found {
		out.Items[i] = toAPIStudent(student)
	}
	return out, nil
}

func (h Identity) GetStudent(ctx context.Context, request openapi.GetStudentRequestObject) (openapi.GetStudentResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	student, err := h.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: request.Id.String()})
	if errors.Is(err, domain.ErrStudentNotFound) {
		return openapi.GetStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetStudent200JSONResponse(toAPIStudent(student)), nil
}

func (h Identity) CreateStudent(ctx context.Context, request openapi.CreateStudentRequestObject) (openapi.CreateStudentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	classIDs := make([]string, len(httpapi.Deref(request.Body.ClassIds)))
	for i, id := range httpapi.Deref(request.Body.ClassIds) {
		classIDs[i] = id.String()
	}

	createStudentResult, err := h.app.Commands.CreateStudent.Handle(ctx, command.CreateStudent{Request: req, Input: domain.NewStudent{
		Email:    string(request.Body.Email),
		FullName: request.Body.FullName,
		ClassIDs: classIDs,
	}})
	student, temporary := createStudentResult.Student, createStudentResult.TemporaryPassword
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrEmailTaken):
		return openapi.CreateStudent409JSONResponse(httpapi.Error(ctx, openapi.EMAILTAKEN,
			"Địa chỉ email này đã được dùng cho một tài khoản khác.")), nil
	default:
		return nil, err
	}

	return openapi.CreateStudent201JSONResponse{
		User:              toAPIStudent(student),
		TemporaryPassword: temporary,
	}, nil
}

func (h Identity) UpdateStudent(ctx context.Context, request openapi.UpdateStudentRequestObject) (openapi.UpdateStudentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.StudentPatch{
		ID:       request.Id.String(),
		FullName: request.Body.FullName,
		Disabled: request.Body.Disabled,
	}
	if request.Body.Email != nil {
		email := string(*request.Body.Email)
		in.Email = &email
	}

	student, err := h.app.Commands.UpdateStudent.Handle(ctx, command.UpdateStudent{Request: req, Input: in})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrStudentNotFound):
		return openapi.UpdateStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound))}, nil
	case errors.Is(err, domain.ErrEmailTaken):
		return openapi.UpdateStudent409JSONResponse(httpapi.Error(ctx, openapi.EMAILTAKEN,
			"Địa chỉ email này đã được dùng cho một tài khoản khác.")), nil
	default:
		return nil, err
	}
	return openapi.UpdateStudent200JSONResponse(toAPIStudent(student)), nil
}

// ResetStudentPassword is §5.4's answer to having no email provider: the
// teacher is the reset flow. The password is returned once and never stored.
func (h Identity) ResetStudentPassword(ctx context.Context, request openapi.ResetStudentPasswordRequestObject) (openapi.ResetStudentPasswordResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	temporary, err := h.app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: req, ID: request.Id.String()})
	if errors.Is(err, domain.ErrStudentNotFound) {
		return openapi.ResetStudentPassword404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.ResetStudentPassword200JSONResponse{TemporaryPassword: temporary}, nil
}

func studentRequest(ctx context.Context) (domain.WriteRequest, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return domain.WriteRequest{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return domain.WriteRequest{
		ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func toAPIStudent(student domain.Student) openapi.StudentRow {
	providers := make([]openapi.StudentRowLinkedProviders, 0, len(student.LinkedProviders))
	for _, p := range student.LinkedProviders {
		providers = append(providers, openapi.StudentRowLinkedProviders(p))
	}

	classes := make([]openapi.StudentClass, len(student.Classes))
	for i, c := range student.Classes {
		classes[i] = openapi.StudentClass{
			Id:        httpapi.ParseUUID(c.ID),
			Name:      c.Name,
			JoinedVia: openapi.StudentClassJoinedVia(c.JoinedVia),
			JoinedAt:  c.JoinedAt,
		}
	}

	return openapi.StudentRow{
		Id:                 httpapi.ParseUUID(student.ID),
		Email:              openapi_types.Email(student.Email),
		FullName:           student.FullName,
		HasPassword:        student.HasPassword,
		LinkedProviders:    providers,
		MustChangePassword: student.MustChangePassword,
		CreatedAt:          student.CreatedAt,
		DisabledAt:         student.DisabledAt,
		Classes:            classes,
		Stats:              httpapi.StudentStats(student.Stats),
	}
}
