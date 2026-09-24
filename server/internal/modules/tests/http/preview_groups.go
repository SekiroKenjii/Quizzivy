package http

import (
	"context"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
)

func (h Tests) toStudentGroups(ctx context.Context, groups []domain.PreviewGroup) ([]openapi.StudentGroup, error) {
	out := make([]openapi.StudentGroup, len(groups))
	for i, group := range groups {
		item, err := h.toStudentGroup(ctx, group)
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func (h Tests) toStudentGroup(ctx context.Context, group domain.PreviewGroup) (openapi.StudentGroup, error) {
	item := openapi.StudentGroup{
		Id: httpapi.ParseUUID(group.ID), SectionId: httpapi.ParseUUID(group.SectionID), Title: group.Title,
		Instructions: group.Instructions, QuestionIds: make([]openapi.Uuid, len(group.QuestionIDs)),
		Stimuli:    make([]openapi.GroupStimulus, len(group.Stimuli)),
		Recordings: make([]openapi.StudentGroupRecording, len(group.Recordings)), Assets: []openapi.MediaAsset{},
	}
	for j, id := range group.QuestionIDs {
		item.QuestionIds[j] = httpapi.ParseUUID(id)
	}
	for j, material := range group.Stimuli {
		converted, err := toPreviewStimulus(material)
		if err != nil {
			return openapi.StudentGroup{}, err
		}
		item.Stimuli[j] = converted
	}
	for j, recording := range group.Recordings {
		item.Recordings[j] = openapi.StudentGroupRecording{
			Id: httpapi.ParseUUID(recording.ID), AssetId: httpapi.ParseUUID(recording.AssetID),
			Policy: openapi.AudioPolicy{MaxPlays: recording.Policy.MaxPlays, AllowSeek: recording.Policy.AllowSeek, ShowTranscriptAfterSubmit: recording.Policy.ShowTranscriptAfterSubmit},
		}
	}
	for _, id := range group.AssetIDs {
		asset, err := h.previewAsset(ctx, &id)
		if err != nil {
			return openapi.StudentGroup{}, err
		}
		if asset != nil {
			item.Assets = append(item.Assets, *asset)
		}
	}
	return item, nil
}

func toPreviewStimulus(material domain.GroupStimulus) (openapi.GroupStimulus, error) {
	out := openapi.GroupStimulus{Id: httpapi.ParseUUID(material.ID), Title: material.Title, Content: material.Content, Gaps: make([]openapi.GroupGapBinding, len(material.Gaps))}
	for i, gap := range material.Gaps {
		var err error
		if gap.Kind == "blank" && gap.BlankGapID != nil {
			err = out.Gaps[i].FromGroupBlankGap(openapi.GroupBlankGap{GapId: gap.GapID, QuestionId: httpapi.ParseUUID(gap.QuestionID), BlankGapId: *gap.BlankGapID})
		} else {
			err = out.Gaps[i].FromGroupQuestionGap(openapi.GroupQuestionGap{GapId: gap.GapID, QuestionId: httpapi.ParseUUID(gap.QuestionID)})
		}
		if err != nil {
			return openapi.GroupStimulus{}, err
		}
	}
	return out, nil
}

func toPreviewSections(sections []domain.PreviewSection) []openapi.StudentSection {
	out := make([]openapi.StudentSection, len(sections))
	for i, section := range sections {
		out[i] = openapi.StudentSection{Id: httpapi.ParseUUID(section.ID), Title: section.Title, Instructions: section.Instructions}
	}
	return out
}
