package prompts

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// ScoreInput is what "Suggest a score" sends: the question's prompt, type and points, the accepted or sample answers (an essay's sample answer and rubric criterion labels), the allowed scores or bands and the student's answer text. Nothing says who the student is.
type ScoreInput struct {
	Answer   domain.GradedAnswer
	Language domain.Language
}

// Suggestion is a score the assistant proposes, with its confidence, its reasons and a comment for the student. For a short answer Score is one of AllowedScores and Band is nil. For an essay Band is one of Bands and Score is PointsOfBand; with rubric criteria, Criteria holds a band for each in the rubric's order, Band is CriteriaBand of them, and Reasons one line for each.
type Suggestion struct {
	Score      float64
	Band       *float64
	Criteria   []CriterionBand
	Confidence string
	Reasons    []string
	Comment    string
}

// CriterionBand is the band suggested for one rubric criterion.
type CriterionBand struct {
	CriterionID string
	Band        float64
}

// The confidences a suggestion may state.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

const scoreSystem = "You help an English teacher grade. You suggest a score; the teacher decides. " + untrusted + "Reply with JSON only."

const scoreSchema = `{"type":"object","additionalProperties":false,"required":["score","confidence","reasons","comment"],"properties":{"score":{"type":"number"},"confidence":{"type":"string","enum":["high","medium","low"]},"reasons":{"type":"array","items":{"type":"string"}},"comment":{"type":"string"}}}`

// AllowedScores is the set a suggestion is moved to: the multiples of 0.5 from 0 to points, and points itself when it is not one.
func AllowedScores(points float64) []float64 {
	var out []float64
	for v := 0.0; v < points; v += 0.5 {
		out = append(out, v)
	}
	return append(out, points)
}

func scoreList(scores []float64) string {
	parts := make([]string, len(scores))
	for i, s := range scores {
		parts[i] = strconv.FormatFloat(s, 'f', -1, 64)
	}
	return strings.Join(parts, ", ")
}

// Score builds the request for "Suggest a score": a band for an essay, a score for a short answer.
func Score(in ScoreInput) Prompt {
	if in.Answer.Type == Essay {
		return essayScore(in)
	}
	a := in.Answer
	key := "Accepted answers: " + strings.Join(a.Accepted, " | ")
	if len(a.Accepted) == 0 {
		key = "No accepted answers are stored; judge the answer by the question."
	}
	user := fmt.Sprintf("Question (%s point): \"\"\"%s\"\"\"\n%s\nStudent answer: \"\"\"%s\"\"\"\nAllowed scores: %s.\n"+
		"Mark as an English teacher would: the answer must fit the question and the accepted answers, spelling and word form count, capital letters do not.\n"+
		"Write the reasons and the comment in %s. The comment is 1-2 friendly, specific sentences to the student.\n"+
		"Put one of the allowed scores in \"score\", how sure you are (high, medium or low) in \"confidence\", 1-3 short points in \"reasons\" and the comment in \"comment\".",
		strconv.FormatFloat(a.Points, 'f', -1, 64), clip(a.Prompt, 1500), key, clip(a.Answer, 4000), scoreList(AllowedScores(a.Points)), in.Language.Name())
	return Prompt{Feature: domain.FeatureScore, Tier: domain.TierCapable, System: scoreSystem, User: user, Schema: json.RawMessage(scoreSchema), MaxTokens: 700}
}

// DecodeScore reads the provider's answer and moves the score to the nearest allowed one, the lower on a tie. A score that is not a number is ErrUnusable; an unknown confidence reads as medium. For an essay every band is moved to the nearest of Bands, the lower on a tie; with rubric criteria each criterion must have a band (the first, for one named twice) and the essay's band is CriteriaBand of them whatever the model's own band said, and a criterion left without one is ErrUnusable.
func DecodeScore(raw json.RawMessage, in ScoreInput) (Suggestion, error) {
	if in.Answer.Type == Essay {
		return decodeEssayScore(raw, in)
	}
	var answer struct {
		Score      float64  `json:"score"`
		Confidence string   `json:"confidence"`
		Reasons    []string `json:"reasons"`
		Comment    string   `json:"comment"`
	}
	if err := decode(raw, &answer); err != nil {
		return Suggestion{}, err
	}
	if math.IsNaN(answer.Score) || math.IsInf(answer.Score, 0) {
		return Suggestion{}, ErrUnusable
	}
	out := Suggestion{Score: nearest(AllowedScores(in.Answer.Points), answer.Score), Confidence: confidence(answer.Confidence), Comment: clip(answer.Comment, 1000)}
	out.Reasons = distinct(answer.Reasons, 4, 300)
	return out, nil
}

func confidence(stated string) string {
	switch stated {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return stated
	}
	return ConfidenceMedium
}

func nearest(allowed []float64, score float64) float64 {
	best := allowed[0]
	for _, v := range allowed[1:] {
		if math.Abs(v-score) < math.Abs(best-score) {
			best = v
		}
	}
	return best
}
