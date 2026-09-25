package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/content"
	"regexp"
	"slices"
	"strings"
)

// Summary counts what a commit would create and what still needs the teacher.
type Summary struct {
	Sections, Groups, Questions, Included, Excluded  int
	TotalPoints                                      string
	AnswersKnown, AnswersMissing, AnswersConflicting int
	Blocking, NeedsDecision                          int
}

// Review is a draft's current findings and summary; Ready means a commit may proceed.
type Review struct {
	Findings []Finding
	Summary  Summary
	Ready    bool
}

var labelReference = regexp.MustCompile(`(?i)\b(?:all|none|both|neither) of the above\b|\b[A-H]\s*(?:and|or|&|,)\s*[A-H]\b|\bboth [A-H]\b`)

type assessor struct {
	acknowledged map[string]bool
	findings     []Finding
	summary      Summary
	points       *big.Rat
}

// Assess derives every finding from the draft's current content, joins the source notices, and applies the teacher's acknowledgements.
func Assess(d Draft) Review {
	a := assessor{acknowledged: map[string]bool{}, points: new(big.Rat)}
	for _, id := range d.Acknowledged {
		a.acknowledged[id] = true
	}
	for i := range d.Sections {
		a.section(&d.Sections[i])
	}
	a.summary.Sections = len(d.Sections)
	a.summary.TotalPoints = a.points.FloatString(2)
	if a.summary.Included == 0 {
		a.add(CodeNoQuestions, Blocking, "", "", 1, nil)
	}
	a.scoring(d)
	for _, n := range d.Notices {
		a.add(n.Code, n.Severity, n.Target, n.Field, n.Count, n.Evidence, n.ID)
	}
	for _, f := range a.findings {
		switch {
		case f.Severity == Blocking:
			a.summary.Blocking++
		case f.Severity == ReviewRequired && !f.Acknowledged:
			a.summary.NeedsDecision++
		}
	}
	return Review{Findings: a.findings, Summary: a.summary, Ready: a.summary.Blocking == 0 && a.summary.NeedsDecision == 0}
}

func (a *assessor) section(s *DraftSection) {
	included := 0
	for _, it := range s.Items {
		if it.Group != nil {
			a.summary.Groups++
			included += a.group(it.Group)
		}
		if it.Question != nil {
			included += a.question(it.Question)
		}
	}
	if included == 0 {
		a.add(CodeEmptySection, Informational, s.ID, "", 1, s.Source)
	}
	a.options(s)
}

func (a *assessor) group(g *DraftGroup) int {
	included := 0
	for i := range g.Questions {
		included += a.question(&g.Questions[i])
	}
	if included > 0 && validGroupShape(*g) != nil {
		a.add(CodeInvalidGroup, Blocking, g.ID, "", 1, g.Source)
	}
	return included
}

func (a *assessor) question(q *DraftQuestion) int {
	a.summary.Questions++
	if q.Excluded != nil {
		a.summary.Excluded++
		return 0
	}
	a.summary.Included++
	if points, ok := new(big.Rat).SetString(q.Points); ok {
		a.points.Add(a.points, points)
	}
	if q.Type == UnsupportedType {
		a.add(CodeUnsupportedInteraction, Blocking, q.ID, "type", 1, q.Source)
		return 1
	}
	a.answer(q)
	if q.Answer.State == AnswerKnown || q.Type == "short_answer" {
		if field, invalid := invalidField(q); invalid {
			a.add(CodeInvalidQuestion, Blocking, q.ID, field, 1, q.Source)
		}
	}
	if q.Task == "pronunciation" && !hasMark(q, "underline") {
		a.add(CodeMissingUnderline, ReviewRequired, q.ID, "options", 1, q.Source, fingerprint(CodeMissingUnderline, q.ID, optionsDigest(q)))
	}
	if referencesLabels(q) {
		a.add(CodeOptionReference, ReviewRequired, q.ID, "options", 1, q.Source, fingerprint(CodeOptionReference, q.ID, optionsDigest(q)))
	}
	return 1
}

func (a *assessor) answer(q *DraftQuestion) {
	switch q.Answer.State {
	case AnswerConflict:
		a.summary.AnswersConflicting++
		code := CodeConflictingKeys
		if len(q.Answer.Candidates) < 2 {
			code = CodeUnusableKey
		}
		a.add(code, Blocking, q.ID, "answer", 1, q.Answer.Evidence)
	case AnswerUnknown:
		a.summary.AnswersMissing++
		if q.Type != "short_answer" {
			a.add(CodeMissingAnswer, Blocking, q.ID, "answer", 1, q.Source)
		}
	default:
		a.summary.AnswersKnown++
	}
}

func (a *assessor) options(s *DraftSection) {
	counts := map[int]int{}
	var choice []*DraftQuestion
	for _, it := range s.Items {
		if q := it.Question; q != nil && q.Excluded == nil && isChoice(q.Type) && q.Type != "true_false" {
			counts[len(q.Options)]++
			choice = append(choice, q)
		}
	}
	usual, most := 0, 0
	for n, c := range counts {
		if c > most || (c == most && n > usual) {
			usual, most = n, c
		}
	}
	for _, q := range choice {
		if len(q.Options) != usual && len(choice) > 2 {
			a.add(CodeIrregularOptions, ReviewRequired, q.ID, "options", 1, q.Source, fingerprint(CodeIrregularOptions, q.ID, optionsDigest(q)))
		}
	}
}

func (a *assessor) scoring(d Draft) {
	defaulted := 0
	for _, q := range d.Questions() {
		if q.Excluded == nil && q.Origins.Points == Defaulted {
			defaulted++
		}
	}
	if defaulted > 0 {
		a.add(CodeScoringDefaulted, Informational, "", "points", defaulted, nil)
	}
}

func (a *assessor) add(code string, severity Severity, target, field string, count int, evidence []SourceRef, id ...string) {
	f := Finding{Code: code, Severity: severity, Target: target, Field: field, Count: count, Evidence: evidence}
	if f.Evidence == nil {
		f.Evidence = []SourceRef{}
	}
	f.ID = fingerprint(code, target, field)
	if len(id) > 0 {
		f.ID = id[0]
	}
	f.Acknowledged = severity == ReviewRequired && a.acknowledged[f.ID]
	a.findings = append(a.findings, f)
}

func fingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func optionsDigest(q *DraftQuestion) string {
	raw, _ := json.Marshal(q.Options)
	return string(raw)
}

func hasMark(q *DraftQuestion, mark string) bool {
	needle := []byte(`"` + mark + `"`)
	if strings.Contains(string(q.Prompt), string(needle)) {
		return true
	}
	return slices.ContainsFunc(q.Options, func(o DraftOption) bool { return strings.Contains(string(o.Content), string(needle)) })
}

func referencesLabels(q *DraftQuestion) bool {
	return slices.ContainsFunc(q.Options, func(o DraftOption) bool { return labelReference.MatchString(plainText(o.Content)) })
}

func invalidField(q *DraftQuestion) (string, bool) {
	in, err := q.Input()
	if err != nil {
		return "prompt", true
	}
	var invalid *questions.ValidationError
	switch err := in.Validate(nil); {
	case err == nil:
		return "", false
	case errors.As(err, &invalid) && len(invalid.Fields) > 0:
		return invalid.Fields[0].Field, true
	}
	return "", true
}

func isChoice(t string) bool {
	return t == "single_choice" || t == "multiple_choice" || t == "true_false"
}

func plainText(raw json.RawMessage) string {
	d, err := content.Parse(raw)
	if err != nil {
		return ""
	}
	return d.PlainText()
}
