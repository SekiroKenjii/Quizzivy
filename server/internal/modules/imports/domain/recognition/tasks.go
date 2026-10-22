package recognition

import (
	questions "quizzivy/internal/modules/questions/domain"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
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
	{taskTFNG, regexp.MustCompile(`(?is)\btrue\b.{0,80}?\bfalse\b.{0,80}?\bnot\s+given\b|\bT\s*[/,]\s*F\s*(?:[/,]|\bor\b)\s*NG\b|(?:đúng|dung).{0,40}?sai.{0,40}?(?:không|khong)\s+(?:có|co)\s+(?:thông|thong)\s+tin`)},
}

func tasksOf(text string) taskSet {
	text = norm.NFC.String(text)
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

// selectCountOf reads the N of a "Choose N letters" instruction, as a word or a digit. It is zero when there is
// none, when the text names two different counts (each belongs to its own questions, and which is not known), or
// when N is outside the bank's bound.
func selectCountOf(text string) int {
	return only(chooseCount.FindAllStringSubmatch(norm.NFC.String(text), -1), func(m []string) int { return countValue(m[1]) }, questions.MaxSelectCount)
}

// wordLimitOf reads the n of "NO MORE THAN n WORDS" or "ONE WORD ONLY", under the same rules as selectCountOf: a
// limit over the bank's 100 stays instruction text.
func wordLimitOf(text string) int {
	return only(wordLimit.FindAllStringSubmatch(norm.NFC.String(text), -1), func(m []string) int {
		if m[1] != "" {
			return countValue(m[1])
		}
		return countValue(m[2])
	}, questions.MaxWordsLimit)
}

// only is the one value every match reads, when it is at most limit; zero otherwise.
func only(matches [][]string, value func([]string) int, limit int) int {
	n := 0
	for _, m := range matches {
		switch v := value(m); {
		case v > limit:
			return 0
		case n == 0:
			n = v
		case v != n:
			return 0
		}
	}
	return n
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
