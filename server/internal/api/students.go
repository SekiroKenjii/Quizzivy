package api

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/students"
)

const msgStudentNotFound = "Không tìm thấy học viên."

// ListStudents backs §8's students table (G-07) and the two pickers that add a
// student to a class (G-06) or to an assignment (G-01).
func (s *Server) ListStudents(ctx context.Context, request openapi.ListStudentsRequestObject) (openapi.ListStudentsResponseObject, error) {
	if s.Deps.Students == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := students.ListInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.ClassId != nil {
		in.ClassID = request.Params.ClassId.String()
	}
	if request.Params.Status != nil {
		in.Status = students.Status(*request.Params.Status)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, page, err := s.Deps.Students.List(ctx, in)
	if err != nil {
		return nil, err
	}

	facets, err := s.Deps.Students.Facets(ctx, in)
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

func (s *Server) GetStudent(ctx context.Context, request openapi.GetStudentRequestObject) (openapi.GetStudentResponseObject, error) {
	if s.Deps.Students == nil {
		return nil, httpx.ErrNotImplemented
	}
	student, err := s.Deps.Students.Get(ctx, request.Id.String())
	if errors.Is(err, students.ErrNotFound) {
		return openapi.GetStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgStudentNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetStudent200JSONResponse(toAPIStudent(student)), nil
}

func (s *Server) CreateStudent(ctx context.Context, request openapi.CreateStudentRequestObject) (openapi.CreateStudentResponseObject, error) {
	if s.Deps.Students == nil || s.Deps.Auth == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	temporary, hash, err := s.Deps.Auth.NewTemporaryPassword(ctx)
	if err != nil {
		return nil, err
	}

	classIDs := make([]string, len(deref(request.Body.ClassIds)))
	for i, id := range deref(request.Body.ClassIds) {
		classIDs[i] = id.String()
	}

	student, err := s.Deps.Students.Create(ctx, req, students.CreateInput{
		Email:    string(request.Body.Email),
		FullName: request.Body.FullName,
		ClassIDs: classIDs,
		Hash:     hash,
		Now:      time.Now(),
	})
	switch {
	case err == nil:
	case errors.Is(err, students.ErrEmailTaken):
		return openapi.CreateStudent409JSONResponse(authError(ctx, openapi.EMAILTAKEN,
			"Địa chỉ email này đã được dùng cho một tài khoản khác.")), nil
	default:
		return nil, err
	}

	return openapi.CreateStudent201JSONResponse{
		User:              toAPIStudent(student),
		TemporaryPassword: temporary,
	}, nil
}

func (s *Server) UpdateStudent(ctx context.Context, request openapi.UpdateStudentRequestObject) (openapi.UpdateStudentResponseObject, error) {
	if s.Deps.Students == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	in := students.UpdateInput{
		ID:       request.Id.String(),
		FullName: request.Body.FullName,
		Disabled: request.Body.Disabled,
		Now:      time.Now(),
	}
	if request.Body.Email != nil {
		email := string(*request.Body.Email)
		in.Email = &email
	}

	student, err := s.Deps.Students.Update(ctx, req, in)
	switch {
	case err == nil:
	case errors.Is(err, students.ErrNotFound):
		return openapi.UpdateStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgStudentNotFound))}, nil
	case errors.Is(err, students.ErrEmailTaken):
		return openapi.UpdateStudent409JSONResponse(authError(ctx, openapi.EMAILTAKEN,
			"Địa chỉ email này đã được dùng cho một tài khoản khác.")), nil
	default:
		return nil, err
	}
	return openapi.UpdateStudent200JSONResponse(toAPIStudent(student)), nil
}

// ResetStudentPassword is §5.4's answer to having no email provider: the
// teacher is the reset flow. The password is returned once and never stored.
func (s *Server) ResetStudentPassword(ctx context.Context, request openapi.ResetStudentPasswordRequestObject) (openapi.ResetStudentPasswordResponseObject, error) {
	if s.Deps.Students == nil || s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	temporary, hash, err := s.Deps.Auth.NewTemporaryPassword(ctx)
	if err != nil {
		return nil, err
	}

	err = s.Deps.Students.ResetPassword(ctx, req, request.Id.String(), hash, time.Now())
	if errors.Is(err, students.ErrNotFound) {
		return openapi.ResetStudentPassword404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgStudentNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.ResetStudentPassword200JSONResponse{TemporaryPassword: temporary}, nil
}

func studentRequest(ctx context.Context) (students.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return students.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return students.Request{
		ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func toAPIStudent(student students.Student) openapi.StudentRow {
	providers := make([]openapi.StudentRowLinkedProviders, 0, len(student.LinkedProviders))
	for _, p := range student.LinkedProviders {
		providers = append(providers, openapi.StudentRowLinkedProviders(p))
	}

	classes := make([]openapi.StudentClass, len(student.Classes))
	for i, c := range student.Classes {
		classes[i] = openapi.StudentClass{
			Id:        parseUUID(c.ID),
			Name:      c.Name,
			JoinedVia: openapi.StudentClassJoinedVia(c.JoinedVia),
			JoinedAt:  c.JoinedAt,
		}
	}

	return openapi.StudentRow{
		Id:                 parseUUID(student.ID),
		Email:              openapi_types.Email(student.Email),
		FullName:           student.FullName,
		HasPassword:        student.HasPassword,
		LinkedProviders:    providers,
		MustChangePassword: student.MustChangePassword,
		CreatedAt:          student.CreatedAt,
		DisabledAt:         student.DisabledAt,
		Classes:            classes,
		Stats:              toAPIStudentStats(student.Stats),
	}
}

// toAPIStudentStats is the one mapping for G-07's figures, wherever a roster shows them.
func toAPIStudentStats(in students.Stats) openapi.StudentStats {
	stats := openapi.StudentStats{
		SubmittedCount: in.SubmittedCount,
		FlaggedCount:   in.FlaggedCount,
		Activity: openapi.StudentActivity{
			Live:          in.LiveAttempt,
			LastAttemptAt: in.LastAttemptAt,
		},
	}
	if in.ScoreEarned != nil && in.ScoreTotal != nil {
		stats.Score = &openapi.AttemptScore{
			Earned:        *in.ScoreEarned,
			Total:         *in.ScoreTotal,
			PendingManual: in.PendingManual,
		}
	}
	return stats
}

func deref[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}
