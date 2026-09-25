package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"strconv"
	"strings"
	"unicode"
)

type keyOrigin int

const (
	keyInline keyOrigin = iota
	keySameFile
	keyCompanion
)

type keyValue struct {
	value     string
	source    []domain.SourceRef
	origin    keyOrigin
	ambiguous bool
}

type keyEntry struct {
	keyValue
	section int
	label   label
}

type keyPaper struct {
	entries []keyEntry
	stray   []*line
}

type keyBook struct {
	papers map[int]*keyPaper
	order  []int
}

func (b *keyBook) paper(number int) *keyPaper {
	p, ok := b.papers[number]
	if !ok {
		p = &keyPaper{}
		b.papers[number] = p
		b.order = append(b.order, number)
	}
	return p
}

func (b *keyBook) choose(exam, chosen int) (*keyPaper, bool) {
	switch {
	case chosen > 0:
		p, ok := b.papers[chosen]
		return p, ok
	case len(b.order) == 1:
		return b.papers[b.order[0]], true
	case exam > 0 && b.papers[exam] != nil:
		return b.papers[exam], true
	}
	return nil, false
}

func parseKeys(lines []*line, paper int, origin keyOrigin) keyBook {
	book := keyBook{papers: map[int]*keyPaper{}}
	current, section, last := book.paper(paper), 0, label{}
	for _, l := range lines {
		text := l.String()
		if m := paperHeading.FindStringSubmatch(text); m != nil {
			number, _ := strconv.Atoi(m[1])
			current, section, last = book.paper(number), 0, label{}
			l.consumeAll()
			continue
		}
		if keyHeading.MatchString(text) {
			l.consumeAll()
			continue
		}
		from := 0
		if value, end, ok := roman(l.text); ok {
			section, from = value, end
			l.consume(0, end)
		}
		entries := lineEntries(l, from, origin)
		if len(entries) == 0 {
			if e, ok := numberedEntry(l, last, origin); ok {
				entries = []keyEntry{e}
			}
		}
		if len(entries) == 0 {
			if !current.continueLast(l, from) && visible(l.text[from:]) {
				current.stray = append(current.stray, l)
			}
			continue
		}
		for i := range entries {
			entries[i].section = section
		}
		last = entries[len(entries)-1].label
		current.entries = append(current.entries, entries...)
	}
	return book
}

func (p *keyPaper) continueLast(l *line, from int) bool {
	if len(p.entries) == 0 || !visible(l.text[from:]) {
		return false
	}
	start, end := trimmed(l.text, from, len(l.text))
	last := &p.entries[len(p.entries)-1]
	last.value += "\n" + matchable(l.text[start:end])
	last.source = append(last.source, l.refs(start, end)...)
	l.consume(start, end)
	return true
}

func lineEntries(l *line, from int, origin keyOrigin) []keyEntry {
	var out []keyEntry
	for _, chunk := range tabChunks(l.text, from) {
		out = append(out, chunkEntries(l, chunk[0], chunk[1], origin)...)
	}
	return out
}

func tabChunks(text []rune, from int) [][2]int {
	var out [][2]int
	start := from
	for i := from; i <= len(text); i++ {
		if i < len(text) && text[i] != '\t' {
			continue
		}
		if visible(text[start:i]) {
			out = append(out, [2]int{start, i})
		}
		start = i + 1
	}
	return out
}

func chunkEntries(l *line, a, z int, origin keyOrigin) []keyEntry {
	var out []keyEntry
	pos := skipSpace(l.text, a)
	lb, ok := questionLabel(l.text[pos:z])
	for ok {
		valueStart := pos + lb.end
		next, nextLabel, found := nextKeyLabel(l.text, valueStart, z, lb)
		stop := next
		if !found {
			stop = z
		}
		start, end := trimmed(l.text, skipPunctuation(l.text, valueStart, stop), stop)
		if start < end {
			out = append(out, keyEntry{keyValue: keyValue{value: matchable(l.text[start:end]), source: l.refs(pos, end), origin: origin}, label: lb})
			l.consume(pos, end)
		}
		pos, lb, ok = next, nextLabel, found
	}
	return out
}

func skipPunctuation(text []rune, from, to int) int {
	for from < to && (unicode.IsSpace(text[from]) || strings.ContainsRune(".:)", text[from])) {
		from++
	}
	return from
}

func nextKeyLabel(text []rune, from, to int, current label) (int, label, bool) {
	for i := from + 1; i < to; i++ {
		if !unicode.IsSpace(text[i-1]) || !labelInitial(text[i]) {
			continue
		}
		lb, ok := questionLabel(text[i:to])
		if ok && lb.start == 0 && follows(current, lb) {
			return i, lb, true
		}
	}
	return to, label{}, false
}

func labelInitial(r rune) bool {
	return unicode.IsDigit(r) || strings.ContainsRune("QqCc", r)
}

func follows(previous, next label) bool {
	if next.keyword != "" {
		return true
	}
	if previous.sub > 0 && next.number == previous.number {
		return next.sub == previous.sub+1
	}
	return next.sub == 0 && next.number == previous.number+1
}

func numberedEntry(l *line, last label, origin keyOrigin) (keyEntry, bool) {
	lb, ok := questionLabel([]rune(l.numbering))
	if !ok || last.number == 0 || !follows(last, lb) || !visible(l.text) {
		return keyEntry{}, false
	}
	start, end := trimmed(l.text, 0, len(l.text))
	l.consumeAll()
	return keyEntry{keyValue: keyValue{value: matchable(l.text[start:end]), source: l.refs(start, end), origin: origin}, label: lb}, true
}
