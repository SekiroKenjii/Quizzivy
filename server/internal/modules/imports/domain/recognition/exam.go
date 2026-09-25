package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxTitle = 200
	maxSkip  = 20
)

type optionPart struct {
	letter rune
	seg    segment
}

type questionBuilder struct {
	id          string
	label       label
	openCloze   bool
	irregular   bool
	instruction *segment
	stem        []segment
	options     []optionPart
	keys        []keyValue
	source      []domain.SourceRef
	section     *sectionBuilder
	group       *groupBuilder
}

func (q *questionBuilder) nextOption() rune {
	if len(q.options) == 0 {
		return 'A'
	}
	return q.options[len(q.options)-1].letter + 1
}

func (q *questionBuilder) instructionText() string {
	if q.instruction == nil {
		return ""
	}
	return strings.TrimSpace(q.instruction.text())
}

type groupBuilder struct {
	id          string
	label       string
	instruction *segment
	stimulus    []segment
	questions   []*questionBuilder
	accepting   bool
	source      []domain.SourceRef
}

func (g *groupBuilder) instructionText() string {
	if g.instruction == nil {
		return ""
	}
	return strings.TrimSpace(g.instruction.text())
}

func (g *groupBuilder) admits(q *questionBuilder) bool {
	if !g.accepting {
		return false
	}
	if g.label != "" {
		return q.label.sub > 0 && q.label.parent() == g.label
	}
	return q.label.sub == 0 && q.instruction == nil
}

type item struct {
	question *questionBuilder
	group    *groupBuilder
}

func (i item) instruction() string {
	if i.group != nil {
		return i.group.instructionText()
	}
	return i.question.instructionText()
}

type sectionBuilder struct {
	id           string
	irregular    bool
	title        string
	instructions string
	ordinal      int
	tasks        taskSet
	origin       domain.Origin
	items        []item
	source       []domain.SourceRef
}

type exam struct {
	source     string
	title      string
	titleRefs  []domain.SourceRef
	paper      int
	sections   []*sectionBuilder
	section    *sectionBuilder
	question   *questionBuilder
	group      *groupBuilder
	lastNumber int
	lastRoman  int
	explicit   bool
	keyword    string
	keyLines   []*line
	extraPaper []*line
	questions  int
}

func parseExam(source string, lines []*line) *exam {
	e := &exam{source: source}
	for _, l := range lines {
		switch {
		case e.extraPaper != nil:
			e.extraPaper = append(e.extraPaper, l)
		case e.keyLines != nil:
			e.keyLines = append(e.keyLines, l)
		case e.keySection(l):
		case e.titleLine(l):
		case e.anotherPaper(l):
		case e.sectionLine(l):
		case e.questionLine(l):
		case e.optionLine(l):
		case e.answerLine(l):
		default:
			e.textLine(l)
		}
	}
	e.closeQuestion()
	e.closeGroup()
	if !e.explicit {
		e.inferSections()
	}
	return e
}

func (e *exam) started() bool { return e.section != nil || e.group != nil || e.question != nil }

func (e *exam) keySection(l *line) bool {
	text := l.String()
	if !keyHeading.MatchString(text) && !(paperHeading.MatchString(text) && answerPrefix(text)) {
		return false
	}
	e.closeQuestion()
	e.closeGroup()
	e.keyLines = []*line{l}
	return true
}

func answerPrefix(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, prefix := range []string{"đáp án", "dap an", "answer"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func (e *exam) titleLine(l *line) bool {
	if e.title != "" || e.started() || isInstruction(l.String()) {
		return false
	}
	if _, ok := questionLabel(l.text); ok {
		return false
	}
	if _, _, ok := roman(l.text); ok {
		return false
	}
	text := strings.TrimSpace(l.String())
	if utf8.RuneCountInString(text) > maxTitle {
		return false
	}
	e.title = text
	e.titleRefs = l.refs(0, len(l.text))
	if m := paperTitle.FindStringSubmatch(text); m != nil {
		e.paper, _ = strconv.Atoi(m[1])
	}
	l.consumeAll()
	return true
}

func (e *exam) anotherPaper(l *line) bool {
	m := paperHeading.FindStringSubmatch(l.String())
	if m == nil || e.paper == 0 {
		return false
	}
	if n, _ := strconv.Atoi(m[1]); n == e.paper {
		return false
	}
	e.closeQuestion()
	e.closeGroup()
	e.extraPaper = []*line{l}
	return true
}

func (e *exam) sectionLine(l *line) bool {
	if value, end, ok := roman(l.text); ok && e.headingRest(l, end) && e.romanInSequence(value, l.text[end:]) {
		e.openSection(l, value)
		e.section.irregular = value != e.lastRoman+1
		e.lastRoman = value
		return true
	}
	if m := namedSection.FindStringSubmatchIndex(l.String()); m != nil {
		ordinal, _ := strconv.Atoi(l.String()[m[2]:m[3]])
		e.openSection(l, ordinal)
		return true
	}
	return false
}

func (e *exam) romanInSequence(value int, rest []rune) bool {
	if value == e.lastRoman+1 {
		return true
	}
	return e.lastRoman > 0 && value >= e.lastRoman && value <= e.lastRoman+maxSkip && isInstruction(matchable(rest))
}

func (e *exam) headingRest(l *line, end int) bool {
	if !visible(l.text[end:]) {
		return false
	}
	_, isOption := optionStart(l.text[end:])
	return !isOption
}

func (e *exam) openSection(l *line, ordinal int) {
	e.closeQuestion()
	e.closeGroup()
	text := strings.TrimSpace(l.String())
	s := &sectionBuilder{id: identity(e.source, l.pieces[0].block.ID, "section"), ordinal: ordinal, tasks: tasksOf(text), origin: domain.SourceExplicit, source: l.refs(0, len(l.text))}
	s.title, s.instructions = sectionTitle(text)
	e.sections = append(e.sections, s)
	e.section = s
	e.explicit = true
	l.consumeAll()
}

func sectionTitle(text string) (string, string) {
	runes := []rune(text)
	if len(runes) <= maxTitle {
		return text, ""
	}
	cut := strings.LastIndex(string(runes[:maxTitle-1]), " ")
	if cut <= 0 {
		cut = len(string(runes[:maxTitle-1]))
	}
	return strings.TrimSpace(text[:cut]) + "…", text
}

func (e *exam) ensureSection() *sectionBuilder {
	if e.section == nil {
		e.section = &sectionBuilder{origin: domain.InferredStructure}
		e.sections = append(e.sections, e.section)
	}
	return e.section
}

func (e *exam) questionLine(l *line) bool {
	lb, ok := questionLabel(l.text)
	if !ok || !e.acceptLabel(lb) || gapReference(l.text, lb) {
		return false
	}
	if lb.keyword != "" && e.keyword == "" {
		e.keyword = lb.keyword
	}
	if lb.sub > 0 {
		e.adoptParent(lb)
	}
	q := &questionBuilder{id: identity(e.source, l.pieces[0].block.ID, "question", lb.text), label: lb, source: l.refs(lb.start, len(l.text))}
	q.irregular = lb.keyword == "" && lb.sub == 0 && e.lastNumber > 0 && lb.number != e.lastNumber+1 && lb.number != 1
	l.consume(lb.start, lb.end)
	end := e.inlineKey(l, q, lb.end)
	options := scanOptions(l.text[:end], lb.end, 'A')
	switch {
	case len(options) >= 2:
		if visible(l.text[lb.end:options[0].at]) {
			q.stem = append(q.stem, segment{l, lb.end, options[0].at})
		}
		q.options = e.options(l, options)
	case visible(l.text[lb.end:end]) && isInstruction(matchable(l.text[lb.end:end])):
		q.instruction = &segment{l, lb.end, end}
	case visible(l.text[lb.end:end]):
		q.stem = append(q.stem, segment{l, lb.end, end})
	}
	l.consume(lb.end, end)
	e.attach(q)
	return true
}

func gapReference(text []rune, lb label) bool {
	if lb.sub == 0 {
		return false
	}
	gaps := scanGaps(text, lb.end, len(text))
	return len(gaps) > 0 && gaps[0].start == skipSpace(text, lb.end)
}

func (e *exam) acceptLabel(lb label) bool {
	switch {
	case lb.keyword != "":
		return true
	case lb.sub > 0:
		return e.hasParent(lb)
	default:
		return lb.number == e.lastNumber+1 || lb.number == 1 || e.awaitingFirstQuestion() || e.forwardJump(lb.number)
	}
}

func (e *exam) forwardJump(number int) bool {
	return e.lastNumber > 0 && number > e.lastNumber && number <= e.lastNumber+maxSkip
}

func (e *exam) awaitingFirstQuestion() bool {
	if e.section == nil || e.question != nil {
		return false
	}
	for _, it := range e.section.items {
		if it.question != nil || (it.group != nil && len(it.group.questions) > 0) {
			return false
		}
	}
	return true
}

func (e *exam) hasParent(lb label) bool {
	parent := strconv.Itoa(lb.number)
	return (e.group != nil && e.group.label == parent) || (e.question != nil && e.question.label.text == parent)
}

func (e *exam) adoptParent(lb label) {
	parent := strconv.Itoa(lb.number)
	if e.group != nil && e.group.label == parent {
		return
	}
	q := e.question
	if q == nil || q.label.text != parent || len(q.options) > 0 {
		return
	}
	g := &groupBuilder{id: identity(q.id, "group"), label: parent, instruction: q.instruction, stimulus: q.stem, accepting: true, source: q.source}
	s := q.section
	for i := range s.items {
		if s.items[i].question == q {
			s.items[i] = item{group: g}
		}
	}
	e.questions--
	e.question = nil
	e.group = g
}

func (e *exam) attach(q *questionBuilder) {
	e.closeQuestion()
	s := e.ensureSection()
	q.section = s
	if e.group != nil && !e.group.admits(q) {
		e.closeGroup()
	}
	if e.group != nil {
		q.group = e.group
		e.group.questions = append(e.group.questions, q)
	} else {
		s.items = append(s.items, item{question: q})
	}
	if q.label.sub == 0 {
		e.lastNumber = q.label.number
	}
	e.question = q
	e.questions++
}

func (e *exam) options(l *line, options []option) []optionPart {
	parts := make([]optionPart, 0, len(options))
	for _, o := range options {
		parts = append(parts, optionPart{letter: o.letter, seg: segment{l, o.start, o.end}})
	}
	return parts
}

func (e *exam) inlineKey(l *line, q *questionBuilder, from int) int {
	text := matchable(l.text[from:])
	m := inlineKey.FindStringSubmatchIndex(text)
	if m == nil {
		return len(l.text)
	}
	start := from + runeCount(text, m[0])
	q.keys = append(q.keys, keyValue{value: strings.TrimSpace(text[m[2]:m[3]]), source: l.refs(start, len(l.text)), origin: keyInline})
	l.consume(start, len(l.text))
	return start
}

func (e *exam) optionLine(l *line) bool {
	q := e.question
	if q == nil {
		return false
	}
	first := q.nextOption()
	start := skipSpace(l.text, 0)
	if start >= len(l.text) || !optionLabelAt(l.text, start, start, first) {
		return false
	}
	end := e.inlineKey(l, q, start)
	q.options = append(q.options, e.options(l, scanOptions(l.text[:end], start, first))...)
	q.source = append(q.source, l.refs(start, end)...)
	l.consumeAll()
	return true
}

func (e *exam) answerLine(l *line) bool {
	if e.question == nil || !answerOnly.MatchString(l.String()) {
		return false
	}
	e.inlineKey(l, e.question, 0)
	return true
}

func (e *exam) textLine(l *line) {
	whole := segment{l, 0, len(l.text)}
	text := l.String()
	switch {
	case e.question != nil && len(e.question.options) == 0:
		e.question.stem = append(e.question.stem, whole)
		e.question.source = append(e.question.source, whole.refs()...)
	case isInstruction(text) && !e.continuesStimulus(text):
		e.closeQuestion()
		e.closeGroup()
		e.ensureSection()
		e.group = &groupBuilder{id: identity(e.source, l.pieces[0].block.ID, "group"), instruction: &whole, accepting: true, source: whole.refs()}
		e.section.items = append(e.section.items, item{group: e.group})
	case e.group != nil && e.group.accepting && len(e.group.questions) == 0:
		e.group.stimulus = append(e.group.stimulus, whole)
		e.group.source = append(e.group.source, whole.refs()...)
	case e.section != nil && len(e.section.items) == 0 && e.question == nil:
		e.group = &groupBuilder{id: identity(e.source, l.pieces[0].block.ID, "group"), stimulus: []segment{whole}, accepting: true, source: whole.refs()}
		e.section.items = append(e.section.items, item{group: e.group})
	default:
		return
	}
	l.consumeAll()
}

func (e *exam) continuesStimulus(text string) bool {
	g := e.group
	if g == nil || !g.accepting || len(g.questions) > 0 || len(g.stimulus) == 0 {
		return false
	}
	return softWrapped(strings.TrimSpace(g.stimulus[len(g.stimulus)-1].text()), strings.TrimSpace(text))
}

func (e *exam) closeQuestion() { e.question = nil }

func (e *exam) closeGroup() {
	if e.group == nil {
		return
	}
	e.group.accepting = false
	if len(e.group.questions) == 0 {
		e.openCloze(e.group)
	}
	if len(e.group.questions) == 0 && e.section != nil {
		e.dissolveEmptyGroup(e.group)
	}
	e.group = nil
}

func (e *exam) openCloze(g *groupBuilder) {
	var found []*questionBuilder
	seen := map[string]bool{}
	for _, s := range g.stimulus {
		for _, gp := range scanGaps(s.line.text, s.start, s.end) {
			lb, ok := questionLabel([]rune(gp.label + "."))
			if gp.label == "" || !ok || seen[lb.text] {
				continue
			}
			seen[lb.text] = true
			found = append(found, &questionBuilder{id: identity(g.id, "cloze", lb.text), label: lb, openCloze: true, source: s.line.refs(gp.labelStart, gp.end), section: e.section, group: g})
		}
	}
	if len(found) < 2 {
		return
	}
	for i, q := range found {
		expected := e.lastNumber + 1 + i
		q.irregular = e.lastNumber > 0 && q.label.number != expected
	}
	g.questions = found
	e.questions += len(found)
	e.lastNumber = max(e.lastNumber, found[len(found)-1].label.number)
}

func (e *exam) dissolveEmptyGroup(g *groupBuilder) {
	items := e.section.items
	for i := range items {
		if items[i].group == g {
			e.section.items = append(items[:i], items[i+1:]...)
			for _, s := range append(append([]segment{}, g.stimulus...), instructionSegments(g)...) {
				unconsume(s)
			}
			return
		}
	}
}

func instructionSegments(g *groupBuilder) []segment {
	if g.instruction == nil {
		return nil
	}
	return []segment{*g.instruction}
}

func unconsume(s segment) {
	if s.line.used == nil {
		return
	}
	for i := s.start; i < s.end; i++ {
		s.line.used[i] = false
	}
}

func (e *exam) inferSections() {
	if len(e.sections) != 1 {
		return
	}
	var out []*sectionBuilder
	var key string
	for _, it := range e.sections[0].items {
		k := normalizedInstruction(it.instruction())
		if len(out) == 0 || k != key || (it.group != nil && k == "") {
			out = append(out, &sectionBuilder{id: identity(e.source, "section", strconv.Itoa(len(out))), origin: domain.InferredStructure})
			key = k
		}
		current := out[len(out)-1]
		current.items = append(current.items, it)
		if it.question != nil {
			it.question.section = current
		}
	}
	for _, s := range out {
		instruction := s.items[0].instruction()
		if instruction == "" {
			s.title = e.rangeTitle(s)
			continue
		}
		s.title, s.instructions = sectionTitle(instruction)
		s.tasks = tasksOf(instruction)
		for _, it := range s.items {
			if it.group != nil {
				it.group.instruction = nil
			}
		}
	}
	e.sections = out
}

func (e *exam) rangeTitle(s *sectionBuilder) string {
	var labels []string
	for _, it := range s.items {
		if it.question != nil {
			labels = append(labels, it.question.label.text)
		}
		if it.group != nil {
			for _, q := range it.group.questions {
				labels = append(labels, q.label.text)
			}
		}
	}
	if len(labels) == 0 {
		return ""
	}
	span := labels[0]
	if len(labels) > 1 {
		span += "–" + labels[len(labels)-1]
	}
	switch e.keyword {
	case "question", "q":
		if len(labels) > 1 {
			return "Questions " + span
		}
		return "Question " + span
	case "câu", "cau":
		return "Câu " + span
	}
	return span
}
