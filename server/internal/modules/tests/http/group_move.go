package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func readQuestionMove(request openapi.MoveTestQuestionRequestObject, mutation model.GroupMutation) model.QuestionMove {
	body := request.Body
	move := model.QuestionMove{
		TestID: request.Id.String(), QuestionID: body.QuestionId.String(), ExpectedTestUpdatedAt: body.ExpectedUpdatedAt,
		To:             domain.MoveTarget{SectionID: body.To.SectionId.String(), Index: body.To.Index},
		ExpectedGroups: make([]domain.GroupRevision, len(body.ExpectedGroupRevisions)),
		Actor:          mutation.Actor, Grants: mutation.Grants,
	}
	if body.To.GroupId != nil {
		group := body.To.GroupId.String()
		move.To.GroupID = &group
	}
	for i, entry := range body.ExpectedGroupRevisions {
		move.ExpectedGroups[i] = domain.GroupRevision{GroupID: entry.GroupId.String(), Revision: entry.Revision}
	}
	return move
}

func (h Tests) moveReply(ctx context.Context, result domain.MoveQuestionResult) (openapi.MoveTestQuestionResponseObject, error) {
	test, err := toAPITest(result.Test)
	if err != nil {
		return nil, err
	}
	groups := make([]openapi.StoredQuestionGroup, len(result.Groups))
	for i, group := range result.Groups {
		if groups[i], err = h.writeGroup(ctx, group); err != nil {
			return nil, err
		}
	}
	return openapi.MoveTestQuestion200JSONResponse{Test: test, QuestionId: httpapi.ParseUUID(result.QuestionID), Copied: result.Copied, Groups: groups}, nil
}

func moveFailure(ctx context.Context, err error) (*groupResponse, error) {
	if errors.Is(err, domain.ErrNotFound) {
		problem := httpapi.Error(ctx, openapi.NOTFOUND, httpx.Text(ctx, "Không tìm thấy đề, câu hỏi, phần hoặc nhóm.",
			"The test, question, section or group was not found."))
		return &groupResponse{status: 404, problem: &problem}, nil
	}
	return groupFailure(ctx, err)
}

func moveRuleStatus(rule string) int {
	if rule == domain.RuleGroupGapBound {
		return 422
	}
	return 400
}

func moveRuleMessage(ctx context.Context, rule string) string {
	switch rule {
	case domain.RuleGroupGapBound:
		return httpx.Text(ctx, "Câu hỏi đang gắn với chỗ trống trong ngữ liệu của nhóm. Hãy gỡ liên kết trong Ngữ liệu chung trước khi chuyển.",
			"The question is bound to a gap in the group's material. Unbind it in Shared content before moving it.")
	case domain.RuleMoveOutlineOnly:
		return httpx.Text(ctx, "Thao tác này chỉ chuyển câu hỏi vào, ra khỏi hoặc trong một nhóm.",
			"This operation only moves a question into, out of or within a group.")
	case domain.RuleMoveGroupSection:
		return httpx.Text(ctx, "Nhóm đích không thuộc phần đã chọn.", "The destination group does not belong to the chosen section.")
	case domain.RuleMoveRevisions:
		return httpx.Text(ctx, "Danh sách phiên bản nhóm phải gồm đúng các nhóm bị ảnh hưởng, mỗi nhóm một lần.",
			"The group revisions must name exactly the groups the move touches, once each.")
	default:
		return httpx.Text(ctx, "Câu hỏi có ở nhiều phần của đề nên chưa xác định được vị trí nguồn.",
			"The question is in more than one section of the test, so its source place is unclear.")
	}
}
