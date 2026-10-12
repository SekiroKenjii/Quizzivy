package support

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

// QueueContext attaches only represented frozen groups and their teacher transcripts.
func (r *Review) QueueContext(ctx context.Context, queue domain.GradingQueue) (domain.GradingQueue, error) {
	cache := map[string]*domain.SharedReviewContext{}
	for i := range queue.Items {
		item := &queue.Items[i]
		if item.Question.GroupID == "" {
			continue
		}
		key := item.VersionID + ":" + item.Question.GroupID
		shared, exists := cache[key]
		if !exists {
			question, err := r.QuestionContext(ctx, domain.ByQuestion{VersionID: item.VersionID, Question: item.Question})
			if err != nil {
				return domain.GradingQueue{}, err
			}
			shared = question.SharedContext
			cache[key] = shared
		}
		item.SharedContext = shared
	}
	return queue, nil
}
