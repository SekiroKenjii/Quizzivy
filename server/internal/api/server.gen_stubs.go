package api

import (
	"context"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
)

// Server implements the generated StrictServerInterface.
//
// Every operation is a stub until its phase builds it. They return
// httpx.ErrNotImplemented, which the strict handler's error hook renders as a
// 501 in the standard error envelope -- so an unbuilt endpoint is honestly
// unbuilt rather than a 404 that looks like a routing bug.
//
// This file is generated once from the interface and then owned by hand: as
// each operation is implemented, its stub is replaced in place.
type Server struct {
	Deps Deps
}

// Deps is what handlers need. It grows as phases add capability.
type Deps struct {
	DB     DB
	Auth   AuthService
	Tokens TokenVerifier

	// RefreshTTL and CookieSecure shape the §5.2 refresh cookie. CookieSecure
	// is false only for plain-http localhost; everywhere else it must be true,
	// or the cookie travels in the clear.
	RefreshTTL   time.Duration
	CookieSecure bool
}

var _ openapi.StrictServerInterface = (*Server)(nil)

func (s *Server) ListAssignments(_ context.Context, _ openapi.ListAssignmentsRequestObject) (openapi.ListAssignmentsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateAssignment(_ context.Context, _ openapi.CreateAssignmentRequestObject) (openapi.CreateAssignmentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAssignment(_ context.Context, _ openapi.GetAssignmentRequestObject) (openapi.GetAssignmentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UpdateAssignment(_ context.Context, _ openapi.UpdateAssignmentRequestObject) (openapi.UpdateAssignmentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAssignmentMonitor(_ context.Context, _ openapi.GetAssignmentMonitorRequestObject) (openapi.GetAssignmentMonitorResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListAttempts(_ context.Context, _ openapi.ListAttemptsRequestObject) (openapi.ListAttemptsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptForReview(_ context.Context, _ openapi.GetAttemptForReviewRequestObject) (openapi.GetAttemptForReviewResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptEvents(_ context.Context, _ openapi.GetAttemptEventsRequestObject) (openapi.GetAttemptEventsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ExtendAttempt(_ context.Context, _ openapi.ExtendAttemptRequestObject) (openapi.ExtendAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) FinishGrading(_ context.Context, _ openapi.FinishGradingRequestObject) (openapi.FinishGradingResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GradeAttempt(_ context.Context, _ openapi.GradeAttemptRequestObject) (openapi.GradeAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ResetAttempt(_ context.Context, _ openapi.ResetAttemptRequestObject) (openapi.ResetAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) VoidAttempt(_ context.Context, _ openapi.VoidAttemptRequestObject) (openapi.VoidAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListClasses(_ context.Context, _ openapi.ListClassesRequestObject) (openapi.ListClassesResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateClass(_ context.Context, _ openapi.CreateClassRequestObject) (openapi.CreateClassResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetClass(_ context.Context, _ openapi.GetClassRequestObject) (openapi.GetClassResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UpdateClass(_ context.Context, _ openapi.UpdateClassRequestObject) (openapi.UpdateClassResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) RevokeJoinCode(_ context.Context, _ openapi.RevokeJoinCodeRequestObject) (openapi.RevokeJoinCodeResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) RotateJoinCode(_ context.Context, _ openapi.RotateJoinCodeRequestObject) (openapi.RotateJoinCodeResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListClassMembers(_ context.Context, _ openapi.ListClassMembersRequestObject) (openapi.ListClassMembersResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) AddClassMember(_ context.Context, _ openapi.AddClassMemberRequestObject) (openapi.AddClassMemberResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) RemoveClassMember(_ context.Context, _ openapi.RemoveClassMemberRequestObject) (openapi.RemoveClassMemberResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetDashboard(_ context.Context, _ openapi.GetDashboardRequestObject) (openapi.GetDashboardResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListMedia(_ context.Context, _ openapi.ListMediaRequestObject) (openapi.ListMediaResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UploadMedia(_ context.Context, _ openapi.UploadMediaRequestObject) (openapi.UploadMediaResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) DeleteMedia(_ context.Context, _ openapi.DeleteMediaRequestObject) (openapi.DeleteMediaResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListQuestions(_ context.Context, _ openapi.ListQuestionsRequestObject) (openapi.ListQuestionsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateQuestion(_ context.Context, _ openapi.CreateQuestionRequestObject) (openapi.CreateQuestionResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) DeleteQuestion(_ context.Context, _ openapi.DeleteQuestionRequestObject) (openapi.DeleteQuestionResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetQuestion(_ context.Context, _ openapi.GetQuestionRequestObject) (openapi.GetQuestionResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UpdateQuestion(_ context.Context, _ openapi.UpdateQuestionRequestObject) (openapi.UpdateQuestionResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListStudents(_ context.Context, _ openapi.ListStudentsRequestObject) (openapi.ListStudentsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateStudent(_ context.Context, _ openapi.CreateStudentRequestObject) (openapi.CreateStudentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetStudent(_ context.Context, _ openapi.GetStudentRequestObject) (openapi.GetStudentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UpdateStudent(_ context.Context, _ openapi.UpdateStudentRequestObject) (openapi.UpdateStudentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ResetStudentPassword(_ context.Context, _ openapi.ResetStudentPasswordRequestObject) (openapi.ResetStudentPasswordResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListTests(_ context.Context, _ openapi.ListTestsRequestObject) (openapi.ListTestsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateTest(_ context.Context, _ openapi.CreateTestRequestObject) (openapi.CreateTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetTest(_ context.Context, _ openapi.GetTestRequestObject) (openapi.GetTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UpdateTest(_ context.Context, _ openapi.UpdateTestRequestObject) (openapi.UpdateTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) DuplicateTest(_ context.Context, _ openapi.DuplicateTestRequestObject) (openapi.DuplicateTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) PreviewTest(_ context.Context, _ openapi.PreviewTestRequestObject) (openapi.PreviewTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) PublishTest(_ context.Context, _ openapi.PublishTestRequestObject) (openapi.PublishTestResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListTestVersions(_ context.Context, _ openapi.ListTestVersionsRequestObject) (openapi.ListTestVersionsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListMyAssignments(_ context.Context, _ openapi.ListMyAssignmentsRequestObject) (openapi.ListMyAssignmentsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetMyAssignment(_ context.Context, _ openapi.GetMyAssignmentRequestObject) (openapi.GetMyAssignmentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) StartOrResumeAttempt(_ context.Context, _ openapi.StartOrResumeAttemptRequestObject) (openapi.StartOrResumeAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttempt(_ context.Context, _ openapi.GetAttemptRequestObject) (openapi.GetAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) SaveAnswers(_ context.Context, _ openapi.SaveAnswersRequestObject) (openapi.SaveAnswersResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) RecordAudioPlay(_ context.Context, _ openapi.RecordAudioPlayRequestObject) (openapi.RecordAudioPlayResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) FlushEvents(_ context.Context, _ openapi.FlushEventsRequestObject) (openapi.FlushEventsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptResult(_ context.Context, _ openapi.GetAttemptResultRequestObject) (openapi.GetAttemptResultResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) SubmitAttempt(_ context.Context, _ openapi.SubmitAttemptRequestObject) (openapi.SubmitAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListMyClasses(_ context.Context, _ openapi.ListMyClassesRequestObject) (openapi.ListMyClassesResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) JoinClass(_ context.Context, _ openapi.JoinClassRequestObject) (openapi.JoinClassResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetMediaUrl(_ context.Context, _ openapi.GetMediaUrlRequestObject) (openapi.GetMediaUrlResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GoogleAuth(_ context.Context, _ openapi.GoogleAuthRequestObject) (openapi.GoogleAuthResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) UnlinkGoogle(_ context.Context, _ openapi.UnlinkGoogleRequestObject) (openapi.UnlinkGoogleResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) LinkGoogle(_ context.Context, _ openapi.LinkGoogleRequestObject) (openapi.LinkGoogleResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) PreviewJoinCode(_ context.Context, _ openapi.PreviewJoinCodeRequestObject) (openapi.PreviewJoinCodeResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}
