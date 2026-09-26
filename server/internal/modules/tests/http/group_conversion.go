package http

import (
	"context"
	"encoding/json"
	"quizzivy/gen/openapi"
	mediahttp "quizzivy/internal/modules/media/http"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/shared/content"
	"slices"
	"strings"
)

func readGroupBundle(in openapi.QuestionGroupBundle) (domain.GroupBundle, error) {
	raw, err := json.Marshal(in.Group)
	if err != nil {
		return domain.GroupBundle{}, err
	}
	var group domain.QuestionGroup
	if err := json.Unmarshal(raw, &group); err != nil {
		return domain.GroupBundle{}, err
	}
	bundle := domain.GroupBundle{Group: group, Questions: make([]domain.GroupQuestion, len(in.Questions))}
	for i, q := range in.Questions {
		bundle.Questions[i] = domain.GroupQuestion{ID: q.Id.String(), Input: questionshttp.ToQuestionInput(q.Input)}
	}
	return bundle, nil
}

func (h Tests) writeGroup(ctx context.Context, in domain.StoredGroup) (openapi.StoredQuestionGroup, error) {
	out := openapi.StoredQuestionGroup{Revision: in.Revision, ArchivedAt: in.ArchivedAt, CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt, TestUpdatedAt: in.TestUpdatedAt, Assets: []openapi.MediaAsset{}, UnavailableAssetIds: []openapi.Uuid{}}
	if in.OwnerSectionID != nil {
		out.OwnerSectionId = httpapi.Ptr(httpapi.ParseUUID(*in.OwnerSectionID))
	}
	raw, err := json.Marshal(in.Bundle.Group)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out.Bundle.Group); err != nil {
		return out, err
	}
	out.Bundle.Questions = make([]openapi.GroupQuestionInput, len(in.Bundle.Questions))
	for i, q := range in.Bundle.Questions {
		input, err := questionshttp.ToAPIInput(q.Input)
		if err != nil {
			return out, err
		}
		out.Bundle.Questions[i] = openapi.GroupQuestionInput{Id: httpapi.ParseUUID(q.ID), Input: input}
	}
	ids, err := groupAssetIDs(in.Bundle)
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		if h.media == nil {
			out.UnavailableAssetIds = append(out.UnavailableAssetIds, httpapi.ParseUUID(id))
			continue
		}
		asset, err := h.media.Get(ctx, id)
		if err != nil {
			out.UnavailableAssetIds = append(out.UnavailableAssetIds, httpapi.ParseUUID(id))
			continue
		}
		url, err := h.media.SignedURL(ctx, asset)
		if err != nil {
			out.UnavailableAssetIds = append(out.UnavailableAssetIds, httpapi.ParseUUID(id))
			continue
		}
		out.Assets = append(out.Assets, mediahttp.ToAPIMediaAsset(asset, url))
	}
	return out, nil
}

func groupAssetIDs(bundle domain.GroupBundle) ([]string, error) {
	seen := map[string]bool{}
	for _, q := range bundle.Questions {
		if q.Input.MediaAssetID != nil {
			seen[strings.ToLower(*q.Input.MediaAssetID)] = true
		}
	}
	for _, material := range bundle.Group.Stimuli {
		document, err := content.Parse(material.Content)
		if err != nil {
			return nil, err
		}
		for _, asset := range document.Assets() {
			seen[strings.ToLower(asset.ID)] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}
