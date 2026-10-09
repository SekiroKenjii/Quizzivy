package domain_test

import (
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestPublishingValidatesAltTextAgainstTheKindOfTheMedia(t *testing.T) {
	asset, image, audio, alt, on := "0195a000-0000-7000-8000-000000000001", "image", "audio", "Một chú mèo", true
	withImage := base()
	withImage.MediaAssetID, withImage.MediaAssetKind, withImage.MediaAlt = &asset, &image, &alt
	if err := domain.Publishing.Validate(draftWith(withImage)); err != nil {
		t.Fatalf("an image question with alt text was refused: %v", err)
	}

	for name, edit := range map[string]func(*domain.DraftQuestion){
		"an audio file": func(q *domain.DraftQuestion) {
			q.MediaAssetKind, q.AllowSeek, q.ShowTranscript = &audio, &on, &on
		},
		"no media": func(q *domain.DraftQuestion) { q.MediaAssetID, q.MediaAssetKind = nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			q := withImage
			edit(&q)
			v := onlyViolation(t, draftWith(q))
			if v.Rule != domain.QuestionValid || v.QuestionID != q.SourceID {
				t.Fatalf("violation = %+v, want %s on %s", v, domain.QuestionValid, q.SourceID)
			}
		})
	}
}
