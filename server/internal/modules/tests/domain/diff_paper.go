package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
	"slices"
	"strconv"
	"strings"

	questionsdomain "quizzivy/internal/modules/questions/domain"
)

type paper struct {
	questions []paperQuestion
	total     int64
	owners    map[string]int
}

type paperQuestion struct {
	question DraftQuestion
	id       string
	number   int
	section  string
	group    *GroupBundle
	parts    questionParts
}

type questionParts struct {
	kind        string
	prompt      string
	explanation string
	media       string
	points      int64
	options     string
	blanks      string
	answer      string
	correct     []int
	gaps        map[string]int
}

func newPaper(d DraftContent) paper {
	p := paper{owners: map[string]int{}}
	for _, section := range d.Sections {
		sectionPrint := canonicalJSON(section.Title, section.Instructions)
		groups := groupsByMember(section.Groups)
		for _, q := range section.Questions {
			id := strings.ToLower(questionKey(q))
			p.owners[id] = len(p.questions)
			p.questions = append(p.questions, paperQuestion{
				question: q, id: questionKey(q), number: len(p.questions) + 1,
				section: sectionPrint, group: groups[id], parts: newParts(q),
			})
			units, _ := questionsdomain.PointUnits(q.Points)
			p.total += units
		}
	}
	return p
}

func questionKey(q DraftQuestion) string {
	if q.FrozenID != "" {
		return q.FrozenID
	}
	return q.SourceID
}

func groupsByMember(groups []GroupBundle) map[string]*GroupBundle {
	out := map[string]*GroupBundle{}
	for i := range groups {
		for _, member := range groups[i].Group.Members {
			out[strings.ToLower(member.QuestionID)] = &groups[i]
		}
	}
	return out
}

func newParts(q DraftQuestion) questionParts {
	promptContent, gaps := canonicalContent(q.PromptContent)
	explanationContent, _ := canonicalContent(q.ExplanationContent)
	units, _ := questionsdomain.PointUnits(q.Points)
	correct := correctPositions(q.Options)
	return questionParts{
		kind:        q.Type,
		prompt:      canonicalJSON(q.Prompt, promptContent),
		explanation: canonicalJSON(q.Explanation, explanationContent),
		media:       canonicalJSON(lowered(q.MediaAssetID), q.MaxPlays, q.AllowSeek, q.ShowTranscript, q.Transcript),
		points:      units,
		options:     canonicalJSON(optionPrints(q.Options)),
		blanks:      canonicalJSON(blankPrints(q.Blanks, gaps)),
		answer:      canonicalJSON(correct, acceptedAnswers(q.Blanks), q.SampleAnswer),
		correct:     correct,
		gaps:        gaps,
	}
}

func (p questionParts) fingerprint() string {
	return canonicalJSON(p.kind, p.prompt, p.explanation, p.media, p.points, p.options, p.blanks, p.answer)
}

func lowered(id *string) *string {
	if id == nil {
		return nil
	}
	out := strings.ToLower(*id)
	return &out
}

type optionPrint struct {
	Text    string
	Content string
}

func orderedOptions(options []DraftOption) []DraftOption {
	ordered := slices.Clone(options)
	slices.SortStableFunc(ordered, func(a, b DraftOption) int { return a.Ordinal - b.Ordinal })
	return ordered
}

func optionPrints(options []DraftOption) []optionPrint {
	prints := make([]optionPrint, 0, len(options))
	for _, option := range orderedOptions(options) {
		text, _ := canonicalContent(option.Content)
		prints = append(prints, optionPrint{Text: option.Text, Content: text})
	}
	return prints
}

func correctPositions(options []DraftOption) []int {
	correct := []int{}
	for position, option := range orderedOptions(options) {
		if option.IsCorrect {
			correct = append(correct, position)
		}
	}
	return correct
}

type blankPrint struct {
	Ordinal       int
	Gap           int
	CaseSensitive bool
}

func orderedBlanks(blanks []DraftBlank) []DraftBlank {
	ordered := slices.Clone(blanks)
	slices.SortStableFunc(ordered, func(a, b DraftBlank) int { return a.Ordinal - b.Ordinal })
	return ordered
}

func blankPrints(blanks []DraftBlank, gaps map[string]int) []blankPrint {
	prints := make([]blankPrint, 0, len(blanks))
	for _, blank := range orderedBlanks(blanks) {
		prints = append(prints, blankPrint{Ordinal: blank.Ordinal, Gap: gapPosition(blank.GapID, gaps), CaseSensitive: blank.CaseSensitive})
	}
	return prints
}

func gapPosition(id *string, gaps map[string]int) int {
	if id == nil {
		return -1
	}
	if position, found := gaps[*id]; found {
		return position
	}
	return -1
}

func acceptedAnswers(blanks []DraftBlank) [][]string {
	accepted := make([][]string, 0, len(blanks))
	for _, blank := range orderedBlanks(blanks) {
		answers := slices.Clone(blank.AcceptedAnswers)
		slices.Sort(answers)
		accepted = append(accepted, answers)
	}
	return accepted
}

func canonicalJSON(values ...any) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// canonicalContent returns a rich-content document in a form that does not
// depend on the gap ids it carries, and the position of each of those ids in
// reading order. A document that does not parse is returned as it is.
func canonicalContent(raw json.RawMessage) (string, map[string]int) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	document, err := content.Parse(trimmed)
	if err != nil {
		return string(trimmed), nil
	}
	ids := document.GapIDs()
	rename := make(map[string]string, len(ids))
	positions := make(map[string]int, len(ids))
	for position, id := range ids {
		rename[id] = "g" + strconv.Itoa(position)
		positions[id] = position
	}
	renamed, err := document.WithGapIDs(rename)
	if err != nil {
		return string(trimmed), positions
	}
	encoded, err := renamed.MarshalJSON()
	if err != nil {
		return string(trimmed), positions
	}
	return string(encoded), positions
}

type pairing struct {
	toFrom []int
	fromTo []int
}

func match(from, to paper) pairing {
	pairs := pairing{toFrom: filled(len(to.questions)), fromTo: filled(len(from.questions))}
	pairs.bySource(from, to)
	pairs.byContent(from, to)
	return pairs
}

func filled(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = -1
	}
	return out
}

func (p *pairing) bySource(from, to paper) {
	queues := map[string][]int{}
	for i, q := range from.questions {
		if q.question.SourceID != "" {
			key := strings.ToLower(q.question.SourceID)
			queues[key] = append(queues[key], i)
		}
	}
	for j, q := range to.questions {
		if q.question.SourceID == "" {
			continue
		}
		key := strings.ToLower(q.question.SourceID)
		if len(queues[key]) == 0 {
			continue
		}
		p.pair(queues[key][0], j)
		queues[key] = queues[key][1:]
	}
}

func (p *pairing) byContent(from, to paper) {
	queues := map[string][]int{}
	for i, q := range from.questions {
		if p.fromTo[i] < 0 {
			key := q.parts.fingerprint()
			queues[key] = append(queues[key], i)
		}
	}
	for j, q := range to.questions {
		if p.toFrom[j] >= 0 {
			continue
		}
		key := q.parts.fingerprint()
		if len(queues[key]) == 0 {
			continue
		}
		p.pair(queues[key][0], j)
		queues[key] = queues[key][1:]
	}
}

func (p *pairing) pair(from, to int) {
	p.fromTo[from] = to
	p.toFrom[to] = from
}
