package domain

import (
	"quizzivy/internal/shared/content"
	"regexp"
	"slices"
	"unicode/utf8"
)

const (
	maxSections   = 100
	maxItems      = 500
	maxMembers    = 200
	maxOptions    = 26
	maxBlanks     = 20
	maxAccepted   = 20
	maxDecisions  = 2000
	maxIDLength   = 64
	maxTextLength = 10000
)

var (
	pointsPattern = regexp.MustCompile(`^\d{1,6}(\.\d{1,2})?$`)
	gapPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	questionTypes = []string{"single_choice", "multiple_choice", "true_false", "fill_blank", "short_answer", UnsupportedType}
	origins       = []Origin{SourceExplicit, InferredStructure, Defaulted, TeacherEntered}
)

type editChecker struct {
	ids map[string]bool
	ok  bool
}

// ValidateEdit accepts a teacher's draft when its shape is sound; content problems remain findings rather than rejections.
func ValidateEdit(d Draft) error {
	c := editChecker{ids: map[string]bool{}, ok: true}
	c.check(len(d.Sections) <= maxSections && runes(d.Title) <= maxTitle && len(d.Acknowledged) <= maxDecisions)
	for _, id := range d.Acknowledged {
		c.check(id != "" && len(id) <= maxIDLength)
	}
	for _, s := range d.Sections {
		c.section(s)
	}
	if !c.ok {
		return ErrBadDraft
	}
	return nil
}

func (c *editChecker) check(condition bool) { c.ok = c.ok && condition }

func (c *editChecker) claim(id string) {
	c.check(id != "" && len(id) <= maxIDLength && !c.ids[id])
	c.ids[id] = true
}

func (c *editChecker) section(s DraftSection) {
	c.claim(s.ID)
	c.check(runes(s.Title) >= 1 && runes(s.Title) <= maxTitle && runes(s.Instructions) <= 2000 && len(s.Items) <= maxItems && validOrigin(s.Origin))
	for _, it := range s.Items {
		c.check((it.Question == nil) != (it.Group == nil))
		if it.Question != nil {
			c.question(*it.Question)
		}
		if it.Group != nil {
			c.group(*it.Group)
		}
	}
}

func (c *editChecker) group(g DraftGroup) {
	c.claim(g.ID)
	c.check(runes(g.Label) <= 32 && runes(g.Instructions) <= 2000 && len(g.Questions) <= maxMembers && len(g.Gaps) <= maxMembers)
	if len(g.Stimulus) > 0 {
		_, err := content.Parse(g.Stimulus)
		c.check(err == nil)
	}
	members := map[string]bool{}
	for _, q := range g.Questions {
		c.question(q)
		members[q.ID] = true
	}
	for _, link := range g.Gaps {
		c.check(gapPattern.MatchString(link.GapID) && members[link.QuestionID] && (link.BlankGapID == "" || gapPattern.MatchString(link.BlankGapID)))
	}
}

func (c *editChecker) question(q DraftQuestion) {
	c.claim(q.ID)
	c.check(runes(q.Label) <= 32 && runes(q.Task) <= 32 && slices.Contains(questionTypes, q.Type) && pointsPattern.MatchString(q.Points))
	c.check(len(q.Options) <= maxOptions && len(q.Blanks) <= maxBlanks && len(q.Source) <= maxMembers)
	c.check(validOrigin(q.Origins.Type) && validOrigin(q.Origins.Prompt) && validOrigin(q.Origins.Options) && validOrigin(q.Origins.Answer) && validOrigin(q.Origins.Points))
	_, err := content.Parse(q.Prompt)
	c.check(err == nil)
	if q.Excluded != nil {
		c.check(runes(q.Excluded.Reason) >= 1 && runes(q.Excluded.Reason) <= 500)
	}
	options := map[string]bool{}
	for _, o := range q.Options {
		c.check(o.ID != "" && len(o.ID) <= maxIDLength && !options[o.ID] && runes(o.Label) >= 1 && runes(o.Label) <= 8)
		options[o.ID] = true
		_, err := content.ParseOption(o.Content)
		c.check(err == nil)
	}
	for _, b := range q.Blanks {
		c.check(gapPattern.MatchString(b.GapID) && runes(b.Label) <= 32 && len(b.Accepted) <= maxAccepted)
		for _, a := range b.Accepted {
			c.check(runes(a) >= 1 && runes(a) <= 500)
		}
	}
	c.answer(q.Answer, options)
}

func (c *editChecker) answer(a DraftAnswer, options map[string]bool) {
	c.check(a.State == AnswerKnown || a.State == AnswerUnknown || a.State == AnswerConflict)
	c.check(len(a.OptionIDs) <= maxOptions && runes(a.Text) <= maxTextLength && len(a.Candidates) <= 20 && len(a.Evidence) <= maxMembers)
	for _, id := range a.OptionIDs {
		c.check(options[id])
	}
	for _, candidate := range a.Candidates {
		c.check(runes(candidate.Value) <= maxTextLength)
	}
}

func validOrigin(o Origin) bool { return slices.Contains(origins, o) }

func runes(s string) int { return utf8.RuneCountInString(s) }
