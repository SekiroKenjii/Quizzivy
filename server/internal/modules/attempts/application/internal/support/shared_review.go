package support

import (
	"context"
	"quizzivy/internal/modules/attempts/application/ports"
	"quizzivy/internal/modules/attempts/domain"
	testsquery "quizzivy/internal/modules/tests/application/query"
	"slices"
)

func readSharedContext(ctx context.Context, reader ports.GroupContexts, versionID string) (*domain.SharedReviewContext, error) {
	if reader == nil {
		return nil, domain.ErrGroupContextUnavailable
	}
	groups, err := reader.Handle(ctx, testsquery.GroupContexts{VersionID: versionID})
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, domain.ErrGroupContextUnavailable
	}
	return &domain.SharedReviewContext{Groups: groups, Transcripts: map[string]string{}}, nil
}

// ResultContext attaches learner-safe material only after result authorization and transcript release checks.
func (s *Service) ResultContext(ctx context.Context, result domain.Result) (domain.Result, error) {
	if !slices.ContainsFunc(result.Questions, func(q domain.ResultQuestion) bool { return q.GroupID != "" }) {
		return result, nil
	}
	shared, err := readSharedContext(ctx, s.Groups, result.Attempt.TestVersionID)
	if err != nil {
		return domain.Result{}, err
	}
	if shared.Transcripts, err = s.Store.ReleasedGroupTranscripts(ctx, result.Attempt.TestVersionID); err != nil {
		return domain.Result{}, err
	}
	if shared.AudioPlays, err = s.Store.GroupAudioPlays(ctx, result.Attempt.ID); err != nil {
		return domain.Result{}, err
	}
	result.SharedContext = shared
	return result, nil
}

// PaperContext attaches the immutable material and full shared transcript to a teacher's review.
func (r *Review) PaperContext(ctx context.Context, review domain.Review) (domain.Review, error) {
	if !slices.ContainsFunc(review.Questions, func(q domain.ReviewQuestion) bool { return q.GroupID != "" }) {
		return review, nil
	}
	shared, err := readSharedContext(ctx, r.Groups, review.Attempt.TestVersionID)
	if err != nil {
		return domain.Review{}, err
	}
	if shared.Transcripts, err = r.Repo.GroupTranscripts(ctx, review.Attempt.TestVersionID); err != nil {
		return domain.Review{}, err
	}
	if shared.AudioPlays, err = r.Repo.GroupAudioPlays(ctx, review.Attempt.ID); err != nil {
		return domain.Review{}, err
	}
	review.SharedContext = shared
	return review, nil
}

// QuestionContext supplies only the group needed for a teacher's cross-attempt question review.
func (r *Review) QuestionContext(ctx context.Context, question domain.ByQuestion) (domain.ByQuestion, error) {
	if question.Question.GroupID == "" {
		return question, nil
	}
	shared, err := readSharedContext(ctx, r.Groups, question.VersionID)
	if err != nil {
		return domain.ByQuestion{}, err
	}
	transcripts, err := r.Repo.GroupTranscripts(ctx, question.VersionID)
	if err != nil {
		return domain.ByQuestion{}, err
	}
	for i, group := range shared.Groups {
		if group.ID != question.Question.GroupID {
			continue
		}
		shared.Groups = slices.Clone(shared.Groups[i : i+1])
		for _, recording := range group.Recordings {
			if text, exists := transcripts[recording.ID]; exists {
				shared.Transcripts[recording.ID] = text
			}
		}
		question.SharedContext = shared
		return question, nil
	}
	return domain.ByQuestion{}, domain.ErrGroupContextUnavailable
}
