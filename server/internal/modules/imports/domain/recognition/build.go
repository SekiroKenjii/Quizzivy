package recognition

import (
	"encoding/json"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/content"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	singleChoice   = "single_choice"
	multipleChoice = "multiple_choice"
	trueFalse      = "true_false"
	fillBlank      = "fill_blank"
	shortAnswer    = "short_answer"
	defaultPoints  = "1"
)

type builder struct {
	exam      *exam
	tfSection map[*sectionBuilder]bool
	byLabel   map[string][]*questionBuilder
	byBlank   map[string]*questionBuilder
	blankKeys map[*questionBuilder]map[string][]keyValue
	linked    map[*questionBuilder]string
	notices   []domain.Finding
}

func newBuilder(e *exam) *builder {
	b := &builder{exam: e, byLabel: map[string][]*questionBuilder{}, byBlank: map[string]*questionBuilder{}, blankKeys: map[*questionBuilder]map[string][]keyValue{}, linked: map[*questionBuilder]string{}}
	for _, q := range e.all() {
		b.byLabel[q.label.text] = append(b.byLabel[q.label.text], q)
		for _, g := range q.gaps() {
			if g.label != "" && g.label != q.label.text {
				b.byBlank[g.label] = q
			}
		}
	}
	return b
}

func (e *exam) all() []*questionBuilder {
	var out []*questionBuilder
	for _, s := range e.sections {
		for _, it := range s.items {
			if it.question != nil {
				out = append(out, it.question)
			}
			if it.group != nil {
				out = append(out, it.group.questions...)
			}
		}
	}
	return out
}

func (q *questionBuilder) gaps() []gap {
	if q.openCloze {
		return []gap{{label: q.label.text}}
	}
	var out []gap
	for _, s := range q.stem {
		out = append(out, scanGaps(s.line.text, s.start, s.end)...)
	}
	return out
}

func (q *questionBuilder) tasks() taskSet {
	t := tasksOf(q.instructionText())
	if q.group != nil {
		t |= tasksOf(q.group.instructionText())
	}
	if q.section != nil {
		t |= q.section.tasks
	}
	return t
}

func (b *builder) apply(book keyBook, chosen int) {
	if len(book.order) == 0 {
		return
	}
	paper, ok := book.choose(b.exam.paper, chosen)
	if !ok {
		b.notice(domain.CodeKeyPaperAmbiguous, domain.ReviewRequired, "", paperList(book.order), len(book.order), nil)
		return
	}
	var unmatched []domain.SourceRef
	for _, entry := range paper.entries {
		targets := b.targets(entry)
		if len(targets) == 0 && !b.applyBlank(entry) {
			unmatched = append(unmatched, entry.source...)
		}
		for _, q := range targets {
			value := entry.keyValue
			value.ambiguous = len(targets) > 1
			q.keys = append(q.keys, value)
		}
	}
	if len(unmatched) > 0 {
		b.notice(domain.CodeUnmatchedKey, domain.ReviewRequired, "", "", len(unmatched), unmatched)
	}
	for _, l := range paper.stray {
		b.unassigned(l)
	}
}

func paperList(papers []int) string {
	parts := make([]string, len(papers))
	for i, p := range papers {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

func (b *builder) targets(entry keyEntry) []*questionBuilder {
	targets := b.byLabel[entry.label.text]
	if len(targets) > 1 && entry.section > 0 {
		inSection := slices.DeleteFunc(slices.Clone(targets), func(q *questionBuilder) bool { return q.section.ordinal != entry.section })
		if len(inSection) > 0 {
			targets = inSection
		}
	}
	return targets
}

func (b *builder) applyBlank(entry keyEntry) bool {
	q := b.byBlank[entry.label.text]
	if q == nil {
		return false
	}
	if b.blankKeys[q] == nil {
		b.blankKeys[q] = map[string][]keyValue{}
	}
	b.blankKeys[q][entry.label.text] = append(b.blankKeys[q][entry.label.text], entry.keyValue)
	return true
}

func (b *builder) markTrueFalseSections() {
	b.tfSection = map[*sectionBuilder]bool{}
	sawTrue := map[*sectionBuilder]bool{}
	other := map[*sectionBuilder]bool{}
	for _, q := range b.exam.all() {
		if len(q.options) > 0 || q.openCloze {
			continue
		}
		for _, k := range q.keys {
			switch value, ok := trueFalseValue(k.value); {
			case !ok:
				other[q.section] = true
			case value == "T":
				sawTrue[q.section] = true
			}
		}
	}
	for s := range sawTrue {
		b.tfSection[s] = !other[s]
	}
}

func (b *builder) draft() domain.Draft {
	b.markTrueFalseSections()
	d := domain.Draft{Version: domain.DraftVersion, Title: b.exam.title, Sections: []domain.DraftSection{}, Notices: []domain.Finding{}, Acknowledged: []string{}}
	for i, s := range b.exam.sections {
		id := s.id
		if id == "" {
			id = identity(b.exam.source, "section", strconv.Itoa(i))
		}
		section := domain.DraftSection{ID: id, Title: s.title, Instructions: s.instructions, Origin: s.origin, Items: []domain.DraftItem{}, Source: nonNil(s.source)}
		if section.Title == "" {
			section.Title = b.exam.rangeTitle(s)
		}
		for _, it := range s.items {
			if it.group != nil {
				g := b.group(it.group)
				section.Items = append(section.Items, domain.DraftItem{Group: &g})
				continue
			}
			q := b.question(it.question)
			section.Items = append(section.Items, domain.DraftItem{Question: &q})
		}
		d.Sections = append(d.Sections, section)
	}
	return d
}

func (b *builder) group(g *groupBuilder) domain.DraftGroup {
	members := map[string]*questionBuilder{}
	for _, q := range g.questions {
		members[q.label.text] = q
	}
	out := domain.DraftGroup{ID: g.id, Label: g.label, Instructions: g.instructionText(), Gaps: []domain.GapLink{}, Questions: []domain.DraftQuestion{}, Source: nonNil(g.source)}
	name := func(_ segment, gp gap) (string, string, bool) {
		q := members[gp.label]
		if q == nil || b.linked[q] != "" {
			return "", "", false
		}
		id := "gap-" + strings.ReplaceAll(gp.label, ".", "-")
		b.linked[q] = gp.label
		link := domain.GapLink{GapID: id, QuestionID: q.id}
		if q.openCloze {
			link.BlankGapID = blankID(0)
		}
		out.Gaps = append(out.Gaps, link)
		return id, gp.label, true
	}
	out.Stimulus = b.valid(richContent(g.stimulus, richOptions{name: name, joinWraps: true}), content.Parse, g.id)
	if out.Stimulus == nil {
		out.Gaps = out.Gaps[:0]
		for _, q := range g.questions {
			delete(b.linked, q)
		}
	}
	for _, q := range g.questions {
		out.Questions = append(out.Questions, b.question(q))
	}
	b.flagColored(g.id, g.stimulus)
	return out
}

func (b *builder) question(q *questionBuilder) domain.DraftQuestion {
	tasks := q.tasks()
	gaps := q.gaps()
	values := slices.Clone(q.keys)
	for _, vs := range b.blankKeys[q] {
		values = append(values, vs...)
	}
	if b.tfSection[q.section] && len(q.options) == 0 {
		tasks |= taskTrueFalse
	}
	kind := decideType(q, tasks, len(gaps), values)
	out := domain.DraftQuestion{ID: q.id, Label: q.label.text, Task: tasks.name(), Type: kind, Options: []domain.DraftOption{}, Blanks: []domain.DraftBlank{}, Points: defaultPoints, Source: nonNil(q.source)}
	out.Origins = domain.Origins{Type: domain.InferredStructure, Prompt: domain.SourceExplicit, Options: domain.SourceExplicit, Answer: domain.SourceExplicit, Points: domain.Defaulted}
	switch kind {
	case singleChoice, multipleChoice, trueFalse:
		out.Options, out.Origins.Options = b.options(q, kind)
		out.Answer = choiceAnswer(out.Options, q.keys)
		if out.Answer.State == domain.AnswerKnown && len(out.Answer.OptionIDs) > 1 && kind == singleChoice {
			out.Type = multipleChoice
		}
	case fillBlank:
		out.Blanks, out.Answer = blanks(q, gaps, q.keys, b.blankKeys[q])
	case shortAnswer:
		out.Answer = textAnswer(q.keys)
	default:
		out.Options, out.Origins.Options = b.options(q, kind)
		out.Answer = unresolved(q.keys)
	}
	if out.Answer.State == domain.AnswerUnknown {
		out.Origins.Answer = domain.Defaulted
	}
	out.Prompt, out.Origins.Prompt = b.prompt(q, kind == fillBlank)
	b.flagQuestion(q)
	return out
}

func (b *builder) flagColored(target string, segments []segment) {
	var colored []domain.SourceRef
	for _, s := range segments {
		if s.line.colored(s.start, s.end) {
			colored = append(colored, s.refs()...)
		}
	}
	if len(colored) > 0 {
		b.notice(domain.CodeColoredText, domain.ReviewRequired, target, "", 1, colored)
	}
}

func (b *builder) flagQuestion(q *questionBuilder) {
	segments := slices.Clone(q.stem)
	for _, o := range q.options {
		segments = append(segments, o.seg)
	}
	b.flagColored(q.id, segments)
	if q.irregular {
		b.notice(domain.CodeNumberingIrregular, domain.ReviewRequired, q.id, "", 1, q.source)
	}
}

func decideType(q *questionBuilder, tasks taskSet, gaps int, values []keyValue) string {
	switch {
	case q.openCloze:
		return fillBlank
	case len(q.options) >= 2 && trueFalseOptions(q.options):
		return trueFalse
	case len(q.options) >= 1:
		return singleChoice
	case tasks.has(taskTrueFalse) || anyValue(values, isTrueFalseWord):
		return trueFalse
	case anyValue(values, isLetter):
		return domain.UnsupportedType
	case gaps > 0 && !tasks.has(taskTransformation|taskWriting|taskErrorCorrection) && allValues(values, isShortAnswer):
		return fillBlank
	}
	return shortAnswer
}

func anyValue(values []keyValue, test func(string) bool) bool {
	return slices.ContainsFunc(values, func(v keyValue) bool { return test(v.value) })
}

func allValues(values []keyValue, test func(string) bool) bool {
	return !slices.ContainsFunc(values, func(v keyValue) bool { return !test(v.value) })
}

func trueFalseOptions(options []optionPart) bool {
	if len(options) != 2 {
		return false
	}
	first, _ := trueFalseValue(options[0].seg.text())
	second, _ := trueFalseValue(options[1].seg.text())
	return first == "T" && second == "F"
}

func trueFalseValue(v string) (string, bool) {
	switch strings.ToLower(strings.Trim(v, " .")) {
	case "t", "true", "đúng", "dung":
		return "T", true
	case "f", "false", "sai":
		return "F", true
	}
	return "", false
}

func isTrueFalseWord(v string) bool {
	switch strings.ToLower(strings.Trim(v, " .")) {
	case "true", "false", "đúng", "sai":
		return true
	}
	return false
}

var letterList = regexp.MustCompile(`^\(?[A-Ha-h]\)?(?:\s*(?:,|;|/|&|\s)\s*\(?[A-Ha-h]\)?)*\.?$`)

func isLetter(v string) bool { return letterList.MatchString(strings.TrimSpace(v)) }

func letters(v string) []string {
	var out []string
	for _, r := range strings.ToUpper(v) {
		if r >= 'A' && r <= 'H' {
			out = append(out, string(r))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func isShortAnswer(v string) bool {
	return len(strings.Fields(v)) <= 5 && !strings.ContainsAny(v, "→?!") && !strings.Contains(v, "->")
}

func (b *builder) options(q *questionBuilder, kind string) ([]domain.DraftOption, domain.Origin) {
	if kind == trueFalse && len(q.options) == 0 {
		return []domain.DraftOption{
			{ID: identity(q.id, "option", "T"), Label: "T", Content: plainContent("True")},
			{ID: identity(q.id, "option", "F"), Label: "F", Content: plainContent("False")},
		}, domain.InferredStructure
	}
	out := make([]domain.DraftOption, 0, len(q.options))
	for _, o := range q.options {
		raw := b.valid(richContent([]segment{o.seg}, richOptions{dropUniformMark: true}), content.ParseOption, q.id)
		if raw == nil {
			raw = plainContent(strings.TrimSpace(o.seg.text()))
		}
		out = append(out, domain.DraftOption{ID: identity(q.id, "option", string(o.letter)), Label: string(o.letter), Content: raw})
	}
	return out, domain.SourceExplicit
}

func choiceAnswer(options []domain.DraftOption, values []keyValue) domain.DraftAnswer {
	return settle(values, func(v string) (string, bool) {
		if tf, ok := trueFalseValue(v); ok {
			if id := optionByTruth(options, tf); id != "" {
				return id, true
			}
		}
		if isLetter(v) {
			var ids []string
			for _, l := range letters(v) {
				id := optionByLabel(options, l)
				if id == "" {
					return "", false
				}
				ids = append(ids, id)
			}
			return strings.Join(ids, ","), true
		}
		return optionByText(options, v)
	}, func(a *domain.DraftAnswer, normalized string) { a.OptionIDs = strings.Split(normalized, ",") })
}

func optionByLabel(options []domain.DraftOption, label string) string {
	for _, o := range options {
		if strings.EqualFold(o.Label, label) {
			return o.ID
		}
	}
	return ""
}

func optionByTruth(options []domain.DraftOption, truth string) string {
	for _, o := range options {
		d, err := content.Parse(o.Content)
		if err != nil {
			continue
		}
		if value, ok := trueFalseValue(d.PlainText()); ok && value == truth {
			return o.ID
		}
	}
	return ""
}

func optionByText(options []domain.DraftOption, v string) (string, bool) {
	want := strings.ToLower(strings.TrimSpace(v))
	for _, o := range options {
		d, err := content.Parse(o.Content)
		if err == nil && strings.ToLower(strings.TrimSpace(d.PlainText())) == want {
			return o.ID, true
		}
	}
	return "", false
}

func textAnswer(values []keyValue) domain.DraftAnswer {
	return settle(values, func(v string) (string, bool) { return strings.TrimSpace(v), v != "" }, func(a *domain.DraftAnswer, normalized string) { a.Text = normalized })
}

func unresolved(values []keyValue) domain.DraftAnswer {
	return settle(values, func(string) (string, bool) { return "", false }, nil)
}

func settle(values []keyValue, parse func(string) (string, bool), accept func(*domain.DraftAnswer, string)) domain.DraftAnswer {
	a := domain.DraftAnswer{State: domain.AnswerUnknown, OptionIDs: []string{}, Evidence: []domain.SourceRef{}}
	if len(values) == 0 {
		return a
	}
	agreed, conflict := "", false
	for _, v := range values {
		a.Evidence = append(a.Evidence, v.source...)
		normalized, ok := parse(v.value)
		switch {
		case !ok || v.ambiguous:
			conflict = true
		case agreed == "":
			agreed = normalized
		case agreed != normalized:
			conflict = true
		}
		a.Candidates = appendCandidate(a.Candidates, v)
	}
	if conflict || agreed == "" || accept == nil {
		a.State = domain.AnswerConflict
		return a
	}
	a.State = domain.AnswerKnown
	a.Candidates = nil
	accept(&a, agreed)
	return a
}

func appendCandidate(candidates []domain.KeyValue, v keyValue) []domain.KeyValue {
	for i := range candidates {
		if candidates[i].Value == v.value {
			candidates[i].Evidence = append(candidates[i].Evidence, v.source...)
			return candidates
		}
	}
	return append(candidates, domain.KeyValue{Value: v.value, Evidence: slices.Clone(v.source)})
}

func blanks(q *questionBuilder, gaps []gap, question []keyValue, byLabel map[string][]keyValue) ([]domain.DraftBlank, domain.DraftAnswer) {
	out := make([]domain.DraftBlank, len(gaps))
	answer := domain.DraftAnswer{State: domain.AnswerKnown, OptionIDs: []string{}, Evidence: []domain.SourceRef{}}
	split := splitBlanks(question, len(gaps))
	for i, g := range gaps {
		out[i] = domain.DraftBlank{GapID: blankID(i), Label: blankLabel(g, i), Accepted: []string{}}
		values := byLabel[g.label]
		if values == nil && split != nil {
			values = split[i]
		}
		a := settle(values, func(v string) (string, bool) { return strings.TrimSpace(v), strings.TrimSpace(v) != "" }, func(a *domain.DraftAnswer, normalized string) { a.Text = normalized })
		answer.Evidence = append(answer.Evidence, a.Evidence...)
		answer.Candidates = append(answer.Candidates, a.Candidates...)
		switch a.State {
		case domain.AnswerKnown:
			out[i].Accepted = alternatives(a.Text)
		case domain.AnswerConflict:
			answer.State = domain.AnswerConflict
		default:
			if answer.State == domain.AnswerKnown {
				answer.State = domain.AnswerUnknown
			}
		}
	}
	if answer.State == domain.AnswerKnown {
		answer.Candidates = nil
	}
	return out, answer
}

var blankSeparators = []*regexp.Regexp{
	regexp.MustCompile(`\s+[-–]\s*|\s*[-–]\s+`),
	regexp.MustCompile(`\s*[-–]\s*`),
	regexp.MustCompile(`\s*[,;]\s*`),
	regexp.MustCompile(`\s*/\s*`),
}

func splitBlanks(values []keyValue, count int) [][]keyValue {
	if len(values) == 0 {
		return nil
	}
	out := make([][]keyValue, count)
	for _, v := range values {
		parts := splitInto(v.value, count)
		if parts == nil {
			for i := range out {
				out[i] = append(out[i], keyValue{value: v.value, source: v.source, origin: v.origin, ambiguous: true})
			}
			continue
		}
		for i, p := range parts {
			out[i] = append(out[i], keyValue{value: p, source: v.source, origin: v.origin, ambiguous: v.ambiguous})
		}
	}
	return out
}

func splitInto(value string, count int) []string {
	if count == 1 {
		return []string{value}
	}
	for _, separator := range blankSeparators {
		if parts := separator.Split(strings.TrimSpace(value), -1); len(parts) == count && !slices.Contains(parts, "") {
			return parts
		}
	}
	return nil
}

func alternatives(v string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == '/' || r == '\n' }) {
		if part = strings.TrimSpace(part); part != "" && !slices.Contains(out, part) {
			out = append(out, part)
		}
	}
	return out
}

func blankID(i int) string { return "blank-" + strconv.Itoa(i+1) }

func blankLabel(g gap, i int) string {
	if g.label != "" {
		return g.label
	}
	return strconv.Itoa(i + 1)
}

func (b *builder) prompt(q *questionBuilder, withBlanks bool) (json.RawMessage, domain.Origin) {
	if q.openCloze {
		return gapContent(blankID(0), q.label.text), domain.InferredStructure
	}
	segments := q.stem
	if !withBlanks {
		segments = withoutAnswerLine(segments)
	}
	if q.instruction != nil && b.exam.explicit {
		segments = append([]segment{*q.instruction}, segments...)
	}
	if len(segments) > 0 {
		o := richOptions{dropUniformMark: true, joinWraps: true}
		parse := content.ParseQuestion
		if withBlanks {
			counter := 0
			o.name = func(s segment, g gap) (string, string, bool) {
				if q.instruction != nil && s == *q.instruction {
					return "", "", false
				}
				id, label := blankID(counter), blankLabel(g, counter)
				counter++
				return id, label, true
			}
			parse = content.ParseQuestionPrompt
		}
		if raw := b.valid(richContent(segments, o), parse, q.id); raw != nil {
			return raw, domain.SourceExplicit
		}
	}
	return plainContent(b.fallbackPrompt(q)), domain.InferredStructure
}

func withoutAnswerLine(segments []segment) []segment {
	out := slices.Clone(segments)
	for len(out) > 0 {
		last := out[len(out)-1]
		cut, trailing := answerLineStart(last)
		if !trailing {
			break
		}
		if cut > skipSpace(last.line.text, last.start) {
			out[len(out)-1] = segment{last.line, last.start, cut}
			continue
		}
		out = out[:len(out)-1]
	}
	return out
}

func answerLineStart(s segment) (int, bool) {
	start, end := trimmed(s.line.text, s.start, s.end)
	gaps := scanGaps(s.line.text, start, end)
	if len(gaps) == 0 {
		return end, false
	}
	g := gaps[len(gaps)-1]
	if visible(slices.DeleteFunc(slices.Clone(s.line.text[g.end:end]), func(r rune) bool { return strings.ContainsRune(".?!…", r) })) {
		return end, false
	}
	return trimRight(s.line.text, start, g.labelStart), true
}

func (b *builder) fallbackPrompt(q *questionBuilder) string {
	if label := b.linked[q]; label != "" {
		return "(" + label + ")"
	}
	for _, text := range []string{q.instructionText(), sectionInstruction(q.section)} {
		if text != "" {
			return text
		}
	}
	return "(" + q.label.text + ")"
}

func sectionInstruction(s *sectionBuilder) string {
	if s == nil {
		return ""
	}
	text := s.instructions
	if text == "" {
		text = s.title
	}
	if _, end, ok := roman([]rune(text)); ok {
		return strings.TrimSpace(string([]rune(text)[end:]))
	}
	if m := namedSection.FindStringIndex(text); m != nil && s.origin == domain.SourceExplicit {
		return strings.TrimSpace(text[m[1]:])
	}
	return text
}

func (b *builder) valid(raw json.RawMessage, parse func([]byte) (content.Document, error), target string) json.RawMessage {
	if raw == nil {
		return nil
	}
	if _, err := parse(raw); err != nil {
		b.notice(domain.CodeFormattingSimplified, domain.ReviewRequired, target, "", 1, nil)
		return nil
	}
	return raw
}

func nonNil(refs []domain.SourceRef) []domain.SourceRef {
	if refs == nil {
		return []domain.SourceRef{}
	}
	return refs
}
