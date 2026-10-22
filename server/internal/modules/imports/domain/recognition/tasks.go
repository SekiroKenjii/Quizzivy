package recognition

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type taskSet uint16

const (
	taskPronunciation taskSet = 1 << iota
	taskStress
	taskTrueFalse
	taskErrorCorrection
	taskTransformation
	taskWriting
	taskCloze
	taskReading
	taskWordForm
	taskTFNG
)

func (s taskSet) has(t taskSet) bool { return s&t != 0 }

func (s taskSet) name() string {
	for _, t := range taskNames {
		if s.has(t.task) {
			return t.name
		}
	}
	return ""
}

var taskNames = []struct {
	task taskSet
	name string
}{
	{taskPronunciation, "pronunciation"},
	{taskStress, "stress"},
	{taskErrorCorrection, "error_correction"},
	{taskWordForm, "word_form"},
	{taskTransformation, "transformation"},
	{taskWriting, "writing"},
}

var taskPatterns = []struct {
	task    taskSet
	pattern *regexp.Regexp
}{
	{taskPronunciation, regexp.MustCompile(`(?i)pronounc|\bsounds? differently`)},
	{taskStress, regexp.MustCompile(`(?i)\bstress`)},
	{taskTrueFalse, regexp.MustCompile(`(?i)true\s*(?:or|/)\s*false|\bT\s*/\s*F\b|đúng\s*(?:hay|hoặc|/)\s*sai`)},
	{taskErrorCorrection, regexp.MustCompile(`(?i)mistake|needs? correction|\berrors?\b|incorrect|sửa lỗi`)},
	{taskTransformation, regexp.MustCompile(`(?i)means? the same|closest in meaning|rewrite|finish the (?:second )?sentence|viết lại`)},
	{taskWriting, regexp.MustCompile(`(?i)write (?:complete|full)? ?sentences|given (?:words|clues|cues)|words given|from the (?:words|cues)|rearrange|reorder|in the correct order|meaningful sentences|make questions|sắp xếp`)},
	{taskCloze, regexp.MustCompile(`(?i)fill (?:in )?(?:the|each)? ?(?:blank|gap|space)|numbered (?:blank|gap)`)},
	{taskReading, regexp.MustCompile(`(?i)\bread\b.*\b(?:passage|text|paragraph)|đọc`)},
	{taskWordForm, regexp.MustCompile(`(?i)form of the word|word in (?:capitals|brackets)|correct form of the word`)},
	{taskTFNG, regexp.MustCompile(`(?is)\btrue\b.{0,80}?\bfalse\b.{0,80}?\bnot\s+given\b|\bT\s*[/,]\s*F\s*(?:[/,]|\bor\b)\s*NG\b|đúng.{0,40}?sai.{0,40}?không\s+có\s+thông\s+tin`)},
}

func tasksOf(text string) taskSet {
	var s taskSet
	for _, p := range taskPatterns {
		if p.pattern.MatchString(text) {
			s |= p.task
		}
	}
	return s
}

var (
	chooseCount = regexp.MustCompile(`(?i)\bchoose\s+(one|two|three|four|five|[1-5])\s+(?:correct\s+)?(?:letters?|answers?|options?)\b`)
	wordLimit   = regexp.MustCompile(`(?i)\bno\s+more\s+than\s+(one|two|three|four|five|six|seven|eight|nine|ten|\d{1,3})\s+words?\b|\b(one)\s+word\s+only\b`)
	numberWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10}
)

// selectCountOf reads the N of a "Choose N letters" instruction, as a word or a digit; zero when there is none.
func selectCountOf(text string) int {
	if m := chooseCount.FindStringSubmatch(text); m != nil {
		return countValue(m[1])
	}
	return 0
}

// wordLimitOf reads the n of "NO MORE THAN n WORDS" or "ONE WORD ONLY"; zero when there is none.
func wordLimitOf(text string) int {
	m := wordLimit.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	if m[1] != "" {
		return countValue(m[1])
	}
	return countValue(m[2])
}

func countValue(word string) int {
	if n, ok := numberWords[strings.ToLower(word)]; ok {
		return n
	}
	n, _ := strconv.Atoi(word)
	return n
}

var imperative = regexp.MustCompile(`(?i)^[^\pL]*(?:choose|circle|find|read|complete|write|rewrite|finish|identify|decide|match|fill|give|put|listen|mark|use|make|supply|change|select|underline|pick|answer|look|arrange|combine|correct|each of the following|chọn|tìm|đọc|hoàn thành|viết|điền|sắp xếp|nối)\b`)

func isInstruction(text string) bool {
	text = strings.TrimSpace(text)
	first := strings.IndexFunc(text, unicode.IsLetter)
	if first < 0 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text[first:])
	return unicode.IsUpper(r) && imperative.MatchString(text)
}

func normalizedInstruction(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.Trim(text, " .:;-–")), " "))
}
