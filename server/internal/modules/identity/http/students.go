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

func msgStudentNotFound(ctx context.Context) string {
	return httpx.Text(ctx, "Không tìm thấy học viên.", "The student was not found.")
}

func msgStudentForbidden(ctx context.Context) string {
	return httpx.Text(ctx, "Bạn không có quyền thao tác trên tài khoản này.",
		"You do not have permission to act on this account.")
}

// ListStudents backs §8's students table (G-07) and the two pickers that add a
// student to a class (G-06) or to an assignment (G-01), over the students the
// caller reaches.
func (h Identity) ListStudents(ctx context.Context, request openapi.ListStudentsRequestObject) (openapi.ListStudentsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.StudentQuery{Scope: httpapi.ScopeFromContext(ctx)}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	for _, id := range httpapi.Deref(request.Params.ClassId) {
		in.ClassIDs = append(in.ClassIDs, id.String())
	}
	in.MustChange = request.Params.MustChangePassword
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
	student, err := h.app.Queries.GetStudent.Handle(ctx, query.GetStudent{ID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx)})
	if errors.Is(err, domain.ErrStudentNotFound) {
		return openapi.GetStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound(ctx)))}, nil
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
	case errors.Is(err, domain.ErrClassNotFound):
		return openapi.CreateStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy lớp học.", "The class was not found.")))}, nil
	case errors.Is(err, domain.ErrEmailTaken):
		return openapi.CreateStudent409JSONResponse(httpapi.Error(ctx, openapi.EMAILTAKEN,
			httpx.Text(ctx, "Địa chỉ email này đã được dùng cho một tài khoản khác.",
				"This email address is already used by another account."))), nil
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
			httpapi.NotFound(ctx, msgStudentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrEmailTaken):
		return openapi.UpdateStudent409JSONResponse(httpapi.Error(ctx, openapi.EMAILTAKEN,
			httpx.Text(ctx, "Địa chỉ email này đã được dùng cho một tài khoản khác.",
				"This email address is already used by another account."))), nil
	case errors.Is(err, domain.ErrForbidden):
		return openapi.UpdateStudent403JSONResponse(httpapi.Error(ctx, openapi.FORBIDDEN, msgStudentForbidden(ctx))), nil
	case errors.Is(err, domain.ErrStudentShared):
		return openapi.UpdateStudent403JSONResponse(httpapi.Error(ctx, openapi.STUDENTSHARED,
			httpx.Text(ctx, "Học viên này còn thuộc lớp hoặc bài giao của giáo viên khác, hoặc do người khác tạo, nên chỉ quản trị viên mới đổi được email.",
				"This student is still in another teacher's class or assignment, or was created by someone else, so only an admin can change the email."))), nil
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
	switch {
	case err == nil:
		return openapi.ResetStudentPassword200JSONResponse{TemporaryPassword: temporary}, nil
	case errors.Is(err, domain.ErrStudentNotFound):
		return openapi.ResetStudentPassword404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrForbidden):
		return openapi.ResetStudentPassword403JSONResponse(httpapi.Error(ctx, openapi.FORBIDDEN, msgStudentForbidden(ctx))), nil
	case errors.Is(err, domain.ErrStudentShared):
		return openapi.ResetStudentPassword403JSONResponse(httpapi.Error(ctx, openapi.STUDENTSHARED,
			httpx.Text(ctx, "Học viên này còn thuộc lớp hoặc bài giao của giáo viên khác, hoặc do người khác tạo, nên chỉ quản trị viên mới đặt lại được mật khẩu.",
				"This student is still in another teacher's class or assignment, or was created by someone else, so only an admin can reset the password."))), nil
	default:
		return nil, err
	}
}

// ResetStudentsPasswords resets up to 40 students, each as ResetStudentPassword
// resets one. The answer is the only place the temporary passwords exist.
func (h Identity) ResetStudentsPasswords(ctx context.Context, request openapi.ResetStudentsPasswordsRequestObject) (openapi.ResetStudentsPasswordsResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	ids := make([]string, len(request.Body.StudentIds))
	for i, id := range request.Body.StudentIds {
		ids[i] = id.String()
	}
	result, err := h.app.Commands.ResetStudentsPasswords.Handle(ctx, command.ResetStudentsPasswords{Request: req, IDs: ids})
	if err != nil {
		return nil, err
	}

	var out openapi.ResetStudentsPasswords200JSONResponse
	out.Headers.CacheControl = httpapi.Ptr("no-store")
	out.Body.Items = make([]openapi.StudentPasswordReset, len(result.Reset))
	for i, reset := range result.Reset {
		out.Body.Items[i] = openapi.StudentPasswordReset{
			StudentId:         httpapi.ParseUUID(reset.StudentID),
			FullName:          reset.FullName,
			Email:             openapi_types.Email(reset.Email),
			TemporaryPassword: reset.TemporaryPassword,
		}
	}
	out.Body.Failed = make([]openapi.StudentResetFailure, len(result.Failed))
	for i, failure := range result.Failed {
		out.Body.Failed[i] = openapi.StudentResetFailure{StudentId: httpapi.ParseUUID(failure.StudentID), Code: resetFailureCode(failure.Reason)}
	}
	return out, nil
}

func resetFailureCode(reason error) openapi.ErrorCode {
	switch {
	case errors.Is(reason, domain.ErrStudentNotFound):
		return openapi.NOTFOUND
	case errors.Is(reason, domain.ErrForbidden):
		return openapi.FORBIDDEN
	case errors.Is(reason, domain.ErrStudentShared):
		return openapi.STUDENTSHARED
	default:
		return openapi.INTERNAL
	}
}

func studentRequest(ctx context.Context) (domain.WriteRequest, bool) {
	who, ok := httpapi.ActorFromContext(ctx)
	principal, _ := httpx.PrincipalFromContext(ctx)
	return domain.WriteRequest{ActorID: who.ID, All: who.Scope.All, Grants: principal.Access.Permissions, IP: who.IP, UserAgent: who.UserAgent}, ok
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
