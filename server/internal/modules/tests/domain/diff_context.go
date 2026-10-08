package domain

import (
	"slices"
	"strconv"
	"strings"
)

func pairLabels(from, to paper, pairs pairing) (map[string]string, map[string]string) {
	fromLabels := make(map[string]string, len(from.questions))
	toLabels := make(map[string]string, len(to.questions))
	for i, q := range from.questions {
		fromLabels[strings.ToLower(q.id)] = "q" + strconv.Itoa(i)
	}
	for j, q := range to.questions {
		if i := pairs.toFrom[j]; i >= 0 {
			toLabels[strings.ToLower(q.id)] = "q" + strconv.Itoa(i)
			continue
		}
		toLabels[strings.ToLower(q.id)] = "n" + strconv.Itoa(j)
	}
	return fromLabels, toLabels
}

type contexts struct {
	encoder *encoder
	paper   paper
	labels  map[string]string
	prints  map[*GroupBundle]string
}

func newContexts(e *encoder, p paper, labels map[string]string) *contexts {
	return &contexts{encoder: e, paper: p, labels: labels, prints: map[*GroupBundle]string{}}
}

func (c *contexts) of(q paperQuestion) string {
	if q.group == nil {
		return ""
	}
	body, found := c.prints[q.group]
	if !found {
		body = c.print(*q.group)
		c.prints[q.group] = body
	}
	return c.encoder.json(body, optionOrder(*q.group, q.id))
}

func optionOrder(bundle GroupBundle, questionID string) string {
	for _, member := range bundle.Group.Members {
		if strings.EqualFold(member.QuestionID, questionID) {
			return member.OptionOrder
		}
	}
	return ""
}

type contextPrint struct {
	Title        string
	Instructions string
	Stimuli      []stimulusPrint
	Recordings   []string
}

type stimulusPrint struct {
	Title   string
	Content string
	Gaps    []string
}

func (c *contexts) print(bundle GroupBundle) string {
	group := bundle.Group
	instructions, _ := canonicalContent(group.Instructions)
	body := contextPrint{Title: group.Title, Instructions: instructions, Stimuli: []stimulusPrint{}, Recordings: []string{}}
	for _, stimulus := range group.Stimuli {
		body.Stimuli = append(body.Stimuli, c.stimulus(stimulus))
	}
	for _, recording := range group.Recordings {
		policy := recording.Policy
		body.Recordings = append(body.Recordings, c.encoder.json(strings.ToLower(recording.AssetID), policy.MaxPlays, policy.AllowSeek, policy.ShowTranscriptAfterSubmit, recording.Transcript))
	}
	slices.Sort(body.Recordings)
	return c.encoder.json(body)
}

func (c *contexts) stimulus(stimulus GroupStimulus) stimulusPrint {
	text, gaps := canonicalContent(stimulus.Content)
	body := stimulusPrint{Title: stimulus.Title, Content: text, Gaps: []string{}}
	for _, binding := range stimulus.Gaps {
		body.Gaps = append(body.Gaps, c.encoder.json(gapPosition(&binding.GapID, gaps), binding.Kind, c.label(binding.QuestionID), gapPosition(binding.BlankGapID, c.gapsOf(binding.QuestionID))))
	}
	slices.Sort(body.Gaps)
	return body
}

func (c *contexts) label(questionID string) string {
	if label, found := c.labels[strings.ToLower(questionID)]; found {
		return label
	}
	return "?" + strings.ToLower(questionID)
}

func (c *contexts) gapsOf(questionID string) map[string]int {
	index, found := c.paper.owners[strings.ToLower(questionID)]
	if !found {
		return nil
	}
	return c.paper.questions[index].parts.gaps
}
