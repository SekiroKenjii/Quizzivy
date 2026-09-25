package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"sort"
	"strings"
	"unicode"
)

type piece struct {
	block    *domain.EvidenceBlock
	runes    []rune
	from, to int
	at       int
}

func (p piece) end() int { return p.at + p.to - p.from }

type line struct {
	source    string
	text      []rune
	pieces    []piece
	numbering string
	used      []bool
}

func (l *line) String() string { return matchable(l.text) }

func (l *line) append(b *domain.EvidenceBlock, runes []rune, from, to int, separator rune) {
	if len(l.pieces) > 0 {
		l.text = append(l.text, separator)
	}
	l.pieces = append(l.pieces, piece{block: b, runes: runes, from: from, to: to, at: len(l.text)})
	l.text = append(l.text, runes[from:to]...)
}

func (l *line) firstPieceEndingAfter(offset int) int {
	return sort.Search(len(l.pieces), func(i int) bool { return l.pieces[i].end() > offset })
}

func (l *line) refs(a, z int) []domain.SourceRef {
	var out []domain.SourceRef
	for i := l.firstPieceEndingAfter(a); i < len(l.pieces) && l.pieces[i].at < z; i++ {
		p := l.pieces[i]
		from, to := max(a, p.at)-p.at+p.from, min(z, p.end())-p.at+p.from
		if from < to {
			out = append(out, domain.SourceRef{SourceID: l.source, BlockID: p.block.ID, Start: from, End: to})
		}
	}
	return out
}

func (l *line) consume(a, z int) {
	if l.used == nil {
		l.used = make([]bool, len(l.text))
	}
	for i := max(a, 0); i < min(z, len(l.text)); i++ {
		l.used[i] = true
	}
}

func (l *line) consumeAll() { l.consume(0, len(l.text)) }

func (l *line) unused() [][2]int {
	var out [][2]int
	start := -1
	for i, r := range l.text {
		taken := l.used != nil && l.used[i]
		switch {
		case !taken && start < 0 && !unicode.IsSpace(r):
			start = i
		case taken && start >= 0:
			out = append(out, [2]int{start, i})
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, len(l.text)})
	}
	return slices.DeleteFunc(out, func(r [2]int) bool { return strings.TrimSpace(string(l.text[r[0]:r[1]])) == "" })
}

func (l *line) marked(a, z int, mark string) bool {
	seen := false
	for i := l.firstPieceEndingAfter(a); i < len(l.pieces) && l.pieces[i].at < z; i++ {
		p := l.pieces[i]
		from, to := max(a, p.at)-p.at+p.from, min(z, p.end())-p.at+p.from
		for _, span := range p.block.Spans {
			lo, hi := max(from, span.Start), min(to, span.End)
			if lo >= hi || !visible(p.runes[lo:hi]) {
				continue
			}
			if !slices.Contains(span.Marks, mark) {
				return false
			}
			seen = true
		}
	}
	return seen
}

func (l *line) colored(a, z int) bool {
	for i := l.firstPieceEndingAfter(a); i < len(l.pieces) && l.pieces[i].at < z; i++ {
		p := l.pieces[i]
		from, to := max(a, p.at)-p.at+p.from, min(z, p.end())-p.at+p.from
		for _, span := range p.block.Spans {
			if span.Colored && max(from, span.Start) < min(to, span.End) && visible(p.runes[max(from, span.Start):min(to, span.End)]) {
				return true
			}
		}
	}
	return false
}

func visible(runes []rune) bool {
	return slices.ContainsFunc(runes, func(r rune) bool { return !unicode.IsSpace(r) })
}

func readable(b *domain.EvidenceBlock) bool {
	return b.Main && b.Safe && b.Kind == "paragraph" && visible([]rune(b.Text))
}

func documentLines(d *domain.EvidenceDocument) []*line {
	var out []*line
	var row []*domain.EvidenceBlock
	flush := func() {
		out = append(out, rowLines(d.SourceID, row)...)
		row = row[:0]
	}
	for i := range d.Blocks {
		b := &d.Blocks[i]
		if !readable(b) {
			continue
		}
		if len(row) > 0 && (b.TableID != row[0].TableID || b.Row != row[0].Row) {
			flush()
		}
		if b.TableID != "" {
			row = append(row, b)
			continue
		}
		out = append(out, paragraphLines(d.SourceID, b)...)
	}
	if len(row) > 0 {
		flush()
	}
	return out
}

func paragraphLines(source string, b *domain.EvidenceBlock) []*line {
	runes := []rune(b.Text)
	var out []*line
	start := 0
	for i := 0; i <= len(runes); i++ {
		if i < len(runes) && runes[i] != '\n' {
			continue
		}
		if visible(runes[start:i]) {
			l := &line{source: source}
			l.append(b, runes, start, i, ' ')
			out = append(out, l)
		}
		start = i + 1
	}
	if len(out) > 0 {
		out[0].numbering = b.Numbering
	}
	return out
}

func rowLines(source string, row []*domain.EvidenceBlock) []*line {
	columns := map[int]bool{}
	for _, b := range row {
		columns[b.Column] = true
	}
	if len(columns) < 2 {
		var out []*line
		for _, b := range row {
			out = append(out, paragraphLines(source, b)...)
		}
		return out
	}
	l := &line{source: source}
	column := row[0].Column
	for _, b := range row {
		separator := ' '
		if b.Column != column {
			separator = '\t'
			column = b.Column
		}
		runes := []rune(strings.ReplaceAll(b.Text, "\n", " "))
		l.append(b, runes, 0, len(runes), separator)
	}
	return []*line{l}
}
