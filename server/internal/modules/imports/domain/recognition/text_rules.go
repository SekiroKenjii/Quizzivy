package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"regexp"
	"strings"
)

type examRules struct{ text bool }

var (
	textColonLabel = regexp.MustCompile(`^\s*(\d{1,3})(?:\.(\d{1,2}))?\s*:`)
	textRomanLabel = regexp.MustCompile(`^\s*([IVXL]{1,6})\s*$`)
	textInlineKey  = regexp.MustCompile(`(?i)(?:^|\s)(?:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)\s*(\S.*?)\s*$`)
	textAnswerOnly = regexp.MustCompile(`^\s*(?i:answer\s*[:-]|(?:ans|đáp\s*án|key)\s*:)\s*\S`)
)

func (r examRules) label(text []rune) (label, bool) {
	if l, ok := questionLabel(text); ok {
		return l, true
	}
	if !r.text {
		return label{}, false
	}
	s := matchable(text)
	if m := textColonLabel.FindStringSubmatchIndex(s); m != nil {
		return newLabel(s, m, m[2], m[4], ""), true
	}
	return label{}, false
}

func (r examRules) roman(text []rune) (int, int, bool) {
	if value, end, ok := roman(text); ok {
		return value, end, true
	}
	if !r.text {
		return 0, 0, false
	}
	s := matchable(text)
	m := textRomanLabel.FindStringSubmatchIndex(s)
	if m == nil {
		return 0, 0, false
	}
	value, ok := romanValue(s[m[2]:m[3]])
	return value, runeCount(s, m[1]), ok
}

func (r examRules) gaps(text []rune, from, to int) []gap {
	if !r.text {
		return scanGaps(text, from, to)
	}
	return scanGapsWith(text, from, to, true)
}

func (r examRules) inlineKey() *regexp.Regexp {
	if r.text {
		return textInlineKey
	}
	return inlineKey
}

func (r examRules) answerOnly() *regexp.Regexp {
	if r.text {
		return textAnswerOnly
	}
	return answerOnly
}

func (r examRules) option(l *line, o option) optionPart {
	part := optionPart{letter: o.letter, seg: segment{l, o.start, o.end}}
	if !r.text {
		return part
	}
	if part.seg.start < part.seg.end && l.text[part.seg.start] == '*' {
		part.star = append(part.star, l.refs(part.seg.start, part.seg.start+1)...)
		part.seg.start = skipSpace(l.text, part.seg.start+1)
	}
	if part.seg.end > part.seg.start && l.text[part.seg.end-1] == '*' {
		part.star = append(part.star, l.refs(part.seg.end-1, part.seg.end)...)
		part.seg.end = trimRight(l.text, part.seg.start, part.seg.end-1)
	}
	return part
}

func (q *questionBuilder) collectStars() {
	var values []string
	var refs []domain.SourceRef
	for _, o := range q.options {
		if len(o.star) > 0 {
			values = append(values, string(o.letter))
			refs = append(refs, o.star...)
		}
	}
	if len(values) > 0 {
		q.keys = append(q.keys, keyValue{value: strings.Join(values, ","), source: refs, origin: keyInline})
	}
}

func isNotGiven(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "Not given")
}
