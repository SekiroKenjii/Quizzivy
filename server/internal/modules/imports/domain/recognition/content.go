package recognition

import (
	"encoding/json"
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"sort"
	"strings"
	"unicode"
)

type segment struct {
	line       *line
	start, end int
}

func (s segment) text() string { return matchable(s.line.text[s.start:s.end]) }

func (s segment) refs() []domain.SourceRef { return s.line.refs(s.start, s.end) }

func (s segment) empty() bool { return !visible(s.line.text[s.start:s.end]) }

type textNode struct {
	Type  string   `json:"type"`
	Text  string   `json:"text"`
	Marks []string `json:"marks"`
}

type gapNode struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type paragraphNode struct {
	Type    string `json:"type"`
	Content []any  `json:"content"`
}

type documentNode struct {
	Format string          `json:"format"`
	Blocks []paragraphNode `json:"blocks"`
}

type gapNaming func(s segment, g gap) (id, label string, ok bool)

type richOptions struct {
	name            gapNaming
	joinWraps       bool
	dropUniformMark bool
}

func plainContent(text string) json.RawMessage {
	raw, _ := json.Marshal(documentNode{Format: "semantic_v1", Blocks: []paragraphNode{{Type: "paragraph", Content: []any{textNode{Type: "text", Text: text, Marks: []string{}}}}}})
	return raw
}

func gapContent(id, label string) json.RawMessage {
	raw, _ := json.Marshal(documentNode{Format: "semantic_v1", Blocks: []paragraphNode{{Type: "paragraph", Content: []any{gapNode{Type: "gap", ID: id, Label: label}}}}})
	return raw
}

func richContent(segments []segment, o richOptions) json.RawMessage {
	var blocks []paragraphNode
	var previous string
	for _, s := range segments {
		if s.empty() {
			continue
		}
		nodes := inlineNodes(s, o)
		current := strings.TrimSpace(s.text())
		if o.joinWraps && len(blocks) > 0 && softWrapped(previous, current) {
			last := &blocks[len(blocks)-1]
			last.Content = append(last.Content, textNode{Type: "text", Text: " ", Marks: []string{}})
			last.Content = append(last.Content, nodes...)
		} else {
			blocks = append(blocks, paragraphNode{Type: "paragraph", Content: nodes})
		}
		previous = current
	}
	if len(blocks) == 0 {
		return nil
	}
	raw, _ := json.Marshal(documentNode{Format: "semantic_v1", Blocks: blocks})
	return raw
}

var functionWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true, "of": true, "to": true, "in": true, "on": true,
	"at": true, "by": true, "for": true, "from": true, "with": true, "between": true, "as": true, "than": true, "that": true, "is": true, "are": true,
}

func softWrapped(previous, next string) bool {
	if previous == "" || next == "" {
		return false
	}
	words := strings.Fields(previous)
	if functionWords[strings.ToLower(words[len(words)-1])] {
		return true
	}
	last := []rune(previous)[len([]rune(previous))-1]
	first := []rune(next)[0]
	if strings.ContainsRune(".?!:;\"”’)", last) || unicode.IsUpper(first) {
		return false
	}
	if first == '(' {
		runes := []rune(next)
		return len(runes) > 1 && unicode.IsDigit(runes[1])
	}
	return !strings.ContainsRune("-–—•*\"“‘'", first)
}

func inlineNodes(s segment, o richOptions) []any {
	start, end := trimmed(s.line.text, s.start, s.end)
	var gaps []gap
	if o.name != nil {
		gaps = scanGaps(s.line.text, start, end)
	}
	uniform := uniformMarks(s.line, start, end, o.dropUniformMark)
	var nodes []any
	position := start
	for _, g := range gaps {
		id, label, ok := o.name(s, g)
		if !ok {
			continue
		}
		nodes = appendText(nodes, s.line, position, g.labelStart, uniform)
		if g.labelStart > start && wordRune(s.line.text[g.labelStart-1]) {
			nodes = append(nodes, textNode{Type: "text", Text: " ", Marks: []string{}})
		}
		nodes = append(nodes, gapNode{Type: "gap", ID: id, Label: label})
		if g.end < end && wordRune(s.line.text[g.end]) {
			nodes = append(nodes, textNode{Type: "text", Text: " ", Marks: []string{}})
		}
		position = g.end
	}
	return appendText(nodes, s.line, position, end, uniform)
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func trimmed(text []rune, start, end int) (int, int) {
	start = skipSpace(text, start)
	return start, trimRight(text, start, end)
}

func uniformMarks(l *line, start, end int, drop bool) []string {
	if !drop {
		return nil
	}
	var out []string
	for _, mark := range []string{"bold", "italic", "underline"} {
		if l.marked(start, end, mark) {
			out = append(out, mark)
		}
	}
	return out
}

func appendText(nodes []any, l *line, start, end int, dropped []string) []any {
	var run strings.Builder
	var marks []string
	flush := func() {
		if run.Len() > 0 {
			nodes = append(nodes, textNode{Type: "text", Text: run.String(), Marks: marks})
			run.Reset()
		}
	}
	for i := start; i < end; i++ {
		current := l.marksAt(i, dropped)
		if run.Len() > 0 && !slices.Equal(current, marks) {
			flush()
		}
		marks = current
		r := l.text[i]
		if r == '\t' {
			r = ' '
		}
		run.WriteRune(r)
	}
	flush()
	return nodes
}

var allowedMarks = []string{"bold", "italic", "underline", "strike", "superscript", "subscript"}

func (l *line) marksAt(offset int, dropped []string) []string {
	out := []string{}
	i := l.firstPieceEndingAfter(offset)
	if i >= len(l.pieces) || l.pieces[i].at > offset {
		return out
	}
	p := l.pieces[i]
	at := offset - p.at + p.from
	spans := p.block.Spans
	j := sort.Search(len(spans), func(k int) bool { return spans[k].End > at })
	if j >= len(spans) || spans[j].Start > at {
		return out
	}
	for _, m := range allowedMarks {
		if slices.Contains(spans[j].Marks, m) && !slices.Contains(dropped, m) {
			out = append(out, m)
		}
	}
	if slices.Contains(out, "superscript") && slices.Contains(out, "subscript") {
		out = slices.DeleteFunc(out, func(m string) bool { return m == "subscript" })
	}
	return out
}
