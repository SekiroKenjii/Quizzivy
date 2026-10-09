package domain_test

import (
	"strings"
	"testing"

	"quizzivy/internal/modules/questions/domain"
)

func shortAnswerWithAlt(alt *string) domain.Input {
	return domain.Input{Type: domain.ShortAnswer, Prompt: "Mô tả bức ảnh", Points: "1.00", MediaAssetID: id(), MediaAlt: alt}
}

func TestAltTextIsAcceptedOnlyForAnImage(t *testing.T) {
	cases := []struct {
		name    string
		in      domain.Input
		kind    *string
		refused bool
	}{
		{"an image with alt text", shortAnswerWithAlt(text("Một chú mèo ngồi trên ghế")), image(), false},
		{"an image without alt text", shortAnswerWithAlt(nil), image(), false},
		{"an audio file with alt text", func() domain.Input {
			in := shortAnswerWithAlt(text("Một chú mèo"))
			in.Audio = &domain.AudioPolicy{}
			return in
		}(), audio(), true},
		{"an audio file without alt text", func() domain.Input {
			in := shortAnswerWithAlt(nil)
			in.Audio = &domain.AudioPolicy{}
			return in
		}(), audio(), false},
		{"no media at all with alt text", domain.Input{Type: domain.ShortAnswer, Prompt: "Hỏi", Points: "1.00", MediaAlt: text("Một chú mèo")}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.in.Validate(c.kind)
			if got := hasField(t, err, "mediaAlt"); got != c.refused {
				t.Fatalf("mediaAlt refused = %v, want %v (err = %v)", got, c.refused, err)
			}
			if !c.refused && err != nil {
				t.Fatalf("a valid question was refused: %v", err)
			}
		})
	}
}

func TestAltTextHoldsOneToAThousandCharacters(t *testing.T) {
	cases := []struct {
		name    string
		alt     string
		refused bool
	}{
		{"one character", "đ", false},
		{"a thousand characters", strings.Repeat("đ", domain.MaxMediaAltLength), false},
		{"a thousand and one characters", strings.Repeat("đ", domain.MaxMediaAltLength+1), true},
		{"empty", "", true},
		{"only spaces", " \t\n ", true},
		{"a NUL character", "mèo\x00chó", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := shortAnswerWithAlt(text(c.alt)).Validate(image())
			if got := hasField(t, err, "mediaAlt"); got != c.refused {
				t.Fatalf("mediaAlt refused = %v, want %v (err = %v)", got, c.refused, err)
			}
		})
	}
}
