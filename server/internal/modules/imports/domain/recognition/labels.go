package recognition

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	keywordLabel = regexp.MustCompile(`^\s*(?i:(question|câu|cau|q))\s*(\d{1,3})(?:\.(\d{1,2}))?\s*[.:)]?`)
	bareLabel    = regexp.MustCompile(`^\s*(\d{1,3})(?:\.(\d{1,2}))?\s*[.)]`)
	subLabel     = regexp.MustCompile(`^\s*(\d{1,3})\.(\d{1,2})(?:\s|$)`)
	romanLabel   = regexp.MustCompile(`^\s*([IVXL]{1,6})\s*[.):]`)
	namedSection = regexp.MustCompile(`^\s*(?i:part|section|phần|phan|bài tập|bai tap|bài|bai|exercise|task)\s+(\d{1,2}|[IVXivx]{1,5}|[A-Ha-h])\s*(?:[.:)\-–]|\s|$)`)
	paperTitle   = regexp.MustCompile(`(?i)(?:^|\s)(?:đề\s*(?:thi\s*)?(?:số|so)?|de\s*so|test|paper|exam)\s*(\d{1,3})\b`)
	paperHeading = regexp.MustCompile(`^\s*(?i:(?:đáp\s*án|dap\s*an|answer\s*key|answers?)\s*(?:của\s*)?)?(?i:đề\s*(?:số|so)?|de\s*so|test|paper)\s*(\d{1,3})\b`)
	keyHeading   = regexp.MustCompile(`^\s*(?i:đáp\s*án|dap\s*an|answer\s*keys?|answers|keys?)\s*[:.]?\s*$`)
	inlineKey    = regexp.MustCompile(`(?i)(?:^|\s)(?:answer|đáp\s*án|key)\s*:\s*(\S.*?)\s*$`)
	answerOnly   = regexp.MustCompile(`^\s*(?i:answer|đáp\s*án|key)\s*:\s*\S`)
)

type label struct {
	text        string
	number, sub int
	keyword     string
	start, end  int
}

func (l label) parent() string { return strconv.Itoa(l.number) }

func matchable(text []rune) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r != '\t' && r != '\n' && unicode.IsSpace(r) {
			r = ' '
		}
		b.WriteRune(r)
	}
	return b.String()
}

func questionLabel(text []rune) (label, bool) {
	s := matchable(text)
	if m := keywordLabel.FindStringSubmatchIndex(s); m != nil {
		return newLabel(s, m, m[4], m[6], strings.ToLower(s[m[2]:m[3]])), true
	}
	if m := bareLabel.FindStringSubmatchIndex(s); m != nil && !digitAt(s, m[1]) {
		return newLabel(s, m, m[2], m[4], ""), true
	}
	if m := subLabel.FindStringSubmatchIndex(s); m != nil {
		return newLabel(s, m, m[2], m[4], ""), true
	}
	return label{}, false
}

func newLabel(s string, m []int, numberAt, subAt int, keyword string) label {
	l := label{keyword: keyword, start: runeCount(s, m[0]+leadingSpace(s[m[0]:])), end: runeCount(s, m[1])}
	l.number, _ = strconv.Atoi(digits(s[numberAt:]))
	l.text = strconv.Itoa(l.number)
	if subAt >= 0 {
		l.sub, _ = strconv.Atoi(digits(s[subAt:]))
		l.text += "." + strconv.Itoa(l.sub)
	}
	return l
}

func digits(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		return s
	}
	return s[:end]
}

func digitAt(s string, i int) bool { return i < len(s) && s[i] >= '0' && s[i] <= '9' }

func runeCount(s string, byteOffset int) int { return utf8.RuneCountInString(s[:byteOffset]) }

func roman(text []rune) (value, end int, ok bool) {
	s := matchable(text)
	m := romanLabel.FindStringSubmatchIndex(s)
	if m == nil {
		return 0, 0, false
	}
	value, ok = romanValue(s[m[2]:m[3]])
	return value, runeCount(s, m[1]), ok
}

var romanDigits = map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50}

func romanValue(s string) (int, bool) {
	total := 0
	for i := 0; i < len(s); i++ {
		v := romanDigits[s[i]]
		if i+1 < len(s) && v < romanDigits[s[i+1]] {
			total -= v
		} else {
			total += v
		}
	}
	return total, total > 0 && toRoman(total) == s
}

func toRoman(n int) string {
	var b strings.Builder
	for _, step := range []struct {
		value  int
		symbol string
	}{{50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for ; n >= step.value; n -= step.value {
			b.WriteString(step.symbol)
		}
	}
	return b.String()
}

type option struct {
	letter         rune
	at, start, end int
}

func scanOptions(text []rune, from int, first rune) []option {
	var out []option
	next := first
	for i := from; i < len(text); i++ {
		if !optionLabelAt(text, i, from, next) {
			continue
		}
		if n := len(out); n > 0 {
			out[n-1].end = i
		}
		content := skipSpace(text, i+2)
		out = append(out, option{letter: next, at: i, start: content, end: len(text)})
		next++
		i = content - 1
	}
	for i := range out {
		out[i].end = trimRight(text, out[i].start, out[i].end)
	}
	return out
}

func optionLabelAt(text []rune, i, from int, letter rune) bool {
	if text[i] != letter || i+1 >= len(text) || (text[i+1] != '.' && text[i+1] != ')') {
		return false
	}
	if i > from && !unicode.IsSpace(text[i-1]) && text[i-1] != '(' {
		return false
	}
	return i+2 >= len(text) || !unicode.IsDigit(text[i+2])
}

func optionStart(text []rune) (rune, bool) {
	i := skipSpace(text, 0)
	for _, first := range []rune{'A', 'a'} {
		if i < len(text) && optionLabelAt(text, i, i, first) {
			return first, true
		}
	}
	return 0, false
}

func skipSpace(text []rune, i int) int {
	for i < len(text) && unicode.IsSpace(text[i]) {
		i++
	}
	return i
}

func trimRight(text []rune, start, end int) int {
	for end > start && unicode.IsSpace(text[end-1]) {
		end--
	}
	return end
}

type gap struct {
	start, end int
	label      string
	labelStart int
}

func scanGaps(text []rune, from, to int) []gap {
	var out []gap
	for i := from; i < to; {
		if !gapRune(text[i]) {
			i++
			continue
		}
		j, weight := i, 0
		for j < to && gapRune(text[j]) {
			weight += gapWeight(text[j])
			j++
		}
		if weight >= 4 {
			g := gap{start: i, end: j, labelStart: i}
			g.label, g.labelStart = labelBefore(text, from, i)
			out = append(out, g)
		}
		i = j
	}
	return out
}

func gapRune(r rune) bool { return r == '…' || r == '.' || r == '_' }

func gapWeight(r rune) int {
	if r == '…' {
		return 3
	}
	return 1
}

var gapLabel = regexp.MustCompile(`(?:\((\d{1,3}(?:\.\d{1,2})?)\)|\b[Qq](\d{1,3}\.\d{1,2}))\s*$`)

func labelBefore(text []rune, from, at int) (string, int) {
	prefix := matchable(text[from:at])
	m := gapLabel.FindStringSubmatchIndex(prefix)
	if m == nil {
		return "", at
	}
	value := ""
	for group := 2; group < len(m); group += 2 {
		if m[group] >= 0 {
			value = prefix[m[group]:m[group+1]]
		}
	}
	return value, from + runeCount(prefix, m[0]+leadingSpace(prefix[m[0]:]))
}

func leadingSpace(s string) int { return len(s) - len(strings.TrimLeftFunc(s, unicode.IsSpace)) }
