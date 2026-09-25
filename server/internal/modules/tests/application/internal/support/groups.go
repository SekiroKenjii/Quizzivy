package support

import (
	"context"
	"quizzivy/internal/modules/tests/application/ports"
	"quizzivy/internal/modules/tests/domain"
	"slices"
	"strings"
	"time"
)

// Groups supplies complete-graph operations with a shared clock and repository-verified media kinds.
type Groups struct {
	Repo  domain.GroupRepository
	Media ports.GroupMediaKinds
	Now   func() time.Time
}

// Prepare resolves member attachments once per asset before the common aggregate validation.
func (s *Groups) Prepare(ctx context.Context, bundle domain.GroupBundle) (domain.GroupBundle, error) {
	if s.Repo == nil {
		return domain.GroupBundle{}, domain.ErrGroupUnavailable
	}
	kinds := map[string]string{}
	bundle.Questions = slices.Clone(bundle.Questions)
	for i := range bundle.Questions {
		question := &bundle.Questions[i]
		question.MediaAssetKind = nil
		if question.Input.MediaAssetID == nil {
			continue
		}
		if s.Media == nil {
			return domain.GroupBundle{}, domain.ErrGroupUnavailable
		}
		id := strings.ToLower(*question.Input.MediaAssetID)
		kind, exists := kinds[id]
		if !exists {
			var err error
			kind, err = s.Media.Kind(ctx, id)
			if err != nil {
				return domain.GroupBundle{}, err
			}
			kinds[id] = kind
		}
		question.MediaAssetKind = &kind
	}
	if err := bundle.Validate(); err != nil {
		return domain.GroupBundle{}, err
	}
	return bundle, nil
}
