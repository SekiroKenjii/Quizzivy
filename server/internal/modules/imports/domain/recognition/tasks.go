package recognition

import (
	"regexp"
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
