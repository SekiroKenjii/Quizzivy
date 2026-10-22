package prompts

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// MaxEssay is how many characters of an essay are sent.
const MaxEssay = 6000

const essaySchema = `{"type":"object","additionalProperties":false,"required":["band","criteria","confidence","reasons","comment"],"properties":{"band":{"type":"number"},"criteria":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["criterion","band","reason"],"properties":{"criterion":{"type":"integer"},"band":{"type":"number"},"reason":{"type":"string"}}}},"confidence":{"type":"string","enum":["high","medium","low"]},"reasons":{"type":"array","items":{"type":"string"}},"comment":{"type":"string"}}}`

// Bands are the thirteen essay bands, 3 to 9 in half steps.
func Bands() []float64 {
	out := make([]float64, 0, 13)
	for halves := 6; halves <= 18; halves++ {
		out = append(out, float64(halves)/2)
	}
	return out
}

// CriteriaBand is the band a rubric derives from its criteria's bands, as grading derives it: their mean rounded half up to 0.5, counted in half bands so 6.25 is 6.5 and 6.125 is 6.0.
func CriteriaBand(bands []float64) float64 {
	halves := 0
	for _, b := range bands {
		halves += int(math.Round(b * 2))
	}
	n := len(bands)
	return float64((2*halves+n)/(2*n)) / 2
}

// PointsOfBand is what a band gives a question worth points, as grading stores it: band × points ÷ 9 rounded half up to the cent, counted in half bands and cents.
func PointsOfBand(band, points float64) float64 {
	halves := int64(math.Round(band * 2))
	cents := int64(math.Round(points * 100))
	return float64((halves*cents+9)/18) / 100
}

func essayScore(in ScoreInput) Prompt {
	a := in.Answer
	var b strings.Builder
	fmt.Fprintf(&b, "Essay task: \"\"\"%s\"\"\"\n", clip(a.Prompt, 1500))
	if len(a.Accepted) > 0 {
		fmt.Fprintf(&b, "The teacher's sample answer: \"\"\"%s\"\"\"\n", clip(strings.Join(a.Accepted, "\n"), 1500))
	}
	fmt.Fprintf(&b, "Student essay: \"\"\"%s\"\"\"\nBands: %s.\n", clip(a.Answer, MaxEssay), scoreList(Bands()))
	if len(a.Criteria) > 0 {
		b.WriteString("Rubric criteria:\n")
		for i, c := range a.Criteria {
			fmt.Fprintf(&b, "%d. %s\n", i+1, oneLine(c.Label))
		}
		fmt.Fprintf(&b, "Give every criterion one band in \"criteria\": criterion = its number, band = one of the bands, reason = one short line on what earns it. Put the mean of their bands in \"band\" and leave \"reasons\" empty.\n")
	} else {
		b.WriteString("Put one of the bands in \"band\", leave \"criteria\" empty, and put 1-3 short points in \"reasons\".\n")
	}
	fmt.Fprintf(&b, "Mark as an experienced English examiner would: task response, organisation, range and accuracy of vocabulary and grammar.\n"+
		"Write the reasons and the comment in %s. The comment is 1-2 sentences to the student: one strength and the most useful next step.\n"+
		"Put how sure you are (high, medium or low) in \"confidence\" and the comment in \"comment\".", in.Language.Name())
	return Prompt{Feature: domain.FeatureScore, Tier: domain.TierCapable, System: scoreSystem, User: b.String(), Schema: json.RawMessage(essaySchema), MaxTokens: 900}
}

func decodeEssayScore(raw json.RawMessage, in ScoreInput) (Suggestion, error) {
	var answer struct {
		Band     float64 `json:"band"`
		Criteria []struct {
			Criterion int     `json:"criterion"`
			Band      float64 `json:"band"`
			Reason    string  `json:"reason"`
		} `json:"criteria"`
		Confidence string   `json:"confidence"`
		Reasons    []string `json:"reasons"`
		Comment    string   `json:"comment"`
	}
	if err := decode(raw, &answer); err != nil {
		return Suggestion{}, err
	}
	out := Suggestion{Confidence: confidence(answer.Confidence), Comment: clip(answer.Comment, 1000)}
	rubric := in.Answer.Criteria
	if len(rubric) == 0 {
		if !finite(answer.Band) {
			return Suggestion{}, ErrUnusable
		}
		band := nearest(Bands(), answer.Band)
		out.Band, out.Score, out.Reasons = &band, PointsOfBand(band, in.Answer.Points), distinct(answer.Reasons, 4, 300)
		return out, nil
	}
	given := make([]*float64, len(rubric))
	reasons := make([]string, len(rubric))
	for _, c := range answer.Criteria {
		i := c.Criterion - 1
		if i < 0 || i >= len(rubric) || given[i] != nil {
			continue
		}
		if !finite(c.Band) {
			return Suggestion{}, ErrUnusable
		}
		band := nearest(Bands(), c.Band)
		given[i], reasons[i] = &band, clip(oneLine(c.Reason), 300)
	}
	bands := make([]float64, len(rubric))
	for i, c := range rubric {
		if given[i] == nil {
			return Suggestion{}, fmt.Errorf("%w: criterion %d has no band", ErrUnusable, i+1)
		}
		bands[i] = *given[i]
		out.Criteria = append(out.Criteria, CriterionBand{CriterionID: c.ID, Band: bands[i]})
		out.Reasons = append(out.Reasons, criterionReason(c.Label, reasons[i], bands[i]))
	}
	band := CriteriaBand(bands)
	out.Band, out.Score = &band, PointsOfBand(band, in.Answer.Points)
	return out, nil
}

func criterionReason(label, reason string, band float64) string {
	if reason == "" {
		reason = strconv.FormatFloat(band, 'f', 1, 64)
	}
	return clip(oneLine(label)+": "+reason, 300)
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
