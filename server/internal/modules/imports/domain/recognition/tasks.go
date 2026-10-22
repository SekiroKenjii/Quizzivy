package recognition

import (
	questions "quizzivy/internal/modules/questions/domain"
	"regexp"
	"slices"
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

var (
	trueFalseTask = regexp.MustCompile(`(?i)true\s*(?:\(T\)\s*)?(?:or|/)\s*false|\bT\s*/\s*F\b|đúng\s*(?:\([TĐ]\)\s*)?(?:hay|hoặc|/)\s*sai`)
	tfngTask      = regexp.MustCompile(`(?is)\btrue\b.{0,80}?\bfalse\b.{0,80}?\bnot\s+given\b|\bT\s*[/,]\s*F\s*(?:[/,]|\bor\b)\s*NG\b|(?:đúng|dung).{0,40}?sai.{0,40}?(?:không|khong)\s+(?:có|co)\s+(?:thông|thong)\s+tin`)
)

var taskPatterns = []struct {
	task    taskSet
	pattern *regexp.Regexp
}{
	{taskPronunciation, regexp.MustCompile(`(?i)pronounc|\bsounds? differently`)},
	{taskStress, regexp.MustCompile(`(?i)\bstress`)},
	{taskTrueFalse, trueFalseTask},
	{taskErrorCorrection, regexp.MustCompile(`(?i)mistake|needs? correction|\berrors?\b|incorrect|sửa lỗi`)},
	{taskTransformation, regexp.MustCompile(`(?i)means? the same|closest in meaning|rewrite|finish the (?:second )?sentence|viết lại`)},
	{taskWriting, regexp.MustCompile(`(?i)write (?:complete|full)? ?sentences|given (?:words|clues|cues)|words given|from the (?:words|cues)|rearrange|reorder|in the correct order|meaningful sentences|make questions|sắp xếp`)},
	{taskCloze, regexp.MustCompile(`(?i)fill (?:in )?(?:the|each)? ?(?:blank|gap|space)|numbered (?:blank|gap)`)},
	{taskReading, regexp.MustCompile(`(?i)\bread\b.*\b(?:passage|text|paragraph)|đọc`)},
	{taskWordForm, regexp.MustCompile(`(?i)form of the word|word in (?:capitals|brackets)|correct form of the word`)},
	{taskTFNG, tfngTask},
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

var imperative = regexp.MustCompile(`(?i)^[^\pL]*(choose|circle|find|read|complete|write|rewrite|finish|identify|decide|match|fill|give|put|listen|mark|use|make|supply|change|select|underline|pick|answer|look|arrange|combine|correct|each of the following|chọn|tìm|đọc|hoàn thành|viết|điền|sắp xếp|nối)\b`)

// headingLegend matches the lines of the IELTS answer legend that follow a question-range heading without an
// imperative: "Do the following statements agree…", "In boxes 1-4 on your answer sheet, write", "TRUE if…".
var headingLegend = regexp.MustCompile(`(?i)^[^\pL]*(?:do the (?:following )?statements\b|in boxes\b|on your answer sheet\b|(?:true|false|not given|yes|no)\s+if\b)`)

func isInstruction(text string) bool {
	text = strings.TrimSpace(text)
	first := strings.IndexFunc(text, unicode.IsLetter)
	if first < 0 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text[first:])
	return unicode.IsUpper(r) && imperative.MatchString(text)
}

// readsAsStatement reports whether an imperative-led line is a statement that only opens like a command (see
// declarative) and names no answer format. Only a heading that already has its instruction asks it: anywhere else
// an imperative line is an instruction, so a real one is never lost to the grammar (QC-R6GRM-1).
func readsAsStatement(text string) bool {
	text = strings.TrimSpace(text)
	m := imperative.FindStringSubmatchIndex(text)
	if _, after, labelled := strings.Cut(text, ":"); labelled && imperative.MatchString(strings.TrimSpace(after)) {
		// A label and then the command: "Use of English: choose the correct answer."
		return false
	}
	return m != nil && !answerFormat(text) && declarative(text[m[3]:])
}

// declarative reads what follows an imperative verb, rest, and reports whether the line is a statement that only
// opens like a command: "Use of mobile phones has grown…", "Mark Twain once wrote…", "Look at the river and you will
// see…", "Make no mistake: …", "Đọc sách là thói quen tốt…". It reads grammar, not task words, so an instruction it
// knows nothing about ("Choose TWO.", "Chọn phương án đúng nhất.") stays an instruction. The cues, all in the first
// sentence:
//   - the verb is a noun: "of" follows it ("Use of");
//   - the verb is a name: a capitalised word that is neither a function word nor a count follows it, and a lower-case
//     word follows that ("Mark Twain once"; not "Complete Table 1" or "Fill In The Blanks");
//   - "no" follows the verb and does not open a limit ("Make no mistake"; not "Write no more than");
//   - the command is the condition of a promise that is not the task's: "… and you will / you'll / you can …", but
//     not "… and you will find the answers";
//   - in Vietnamese, the verb phrase is the subject of a predicate: "là", "giúp", "khiến" or "ngày càng", in a
//     sentence that asks nothing.
func declarative(rest string) bool {
	sentence := rest
	if i := strings.IndexAny(sentence, ".?!:"); i >= 0 {
		sentence = sentence[:i]
	}
	words := strings.Fields(sentence)
	if len(words) == 0 {
		return false
	}
	switch next := strings.ToLower(words[0]); {
	case next == "of":
		return true
	case next == "no":
		return len(words) == 1 || !strings.EqualFold(words[1], "more")
	case len(words) > 1 && named(words[0]) && startsLower(words[1]):
		return true
	}
	if loc := promise.FindStringIndex(sentence); loc != nil && !taskPromise.MatchString(sentence[loc[1]:]) {
		return true
	}
	return vietnamesePredicate(sentence)
}

var (
	promise = regexp.MustCompile(`(?i)\band\s+you(?:'ll|\s+(?:will|would|can|could|may|might)\b)`)
	// vietnameseClause ends a Vietnamese first clause at a comma, a semicolon or a joining "và"/"rồi": a predicate in a
	// later clause ("Điền vào chỗ trống, mỗi chỗ trống là một từ", "Đọc đoạn văn và chọn những câu là ý chính") belongs
	// to the instruction (QC-R6GRM-6, QC-R6GRM-13). A compound subject ("Viết nhật ký và đọc sách là…") reads the same,
	// so the line stays the instruction and the draft points at it (vietnameseJoined, QC-R6GRM-10, D-075).
	vietnameseClause = regexp.MustCompile(`[,;]|\s(?:và|rồi)\s`)
	// vietnamesePunctuation ends a clause at a comma or a semicolon only.
	vietnamesePunctuation = regexp.MustCompile(`[,;]`)
	taskPromise           = regexp.MustCompile(`(?i)\b(?:answers?|questions?)\b`)
	vietnameseMarks       = [][]string{{"là"}, {"giúp"}, {"khiến"}, {"ngày", "càng"}}
	// vietnameseAsks holds the question words that make a sentence the task's question ("… là từ nào"), and the word
	// that makes "là" a gloss ("có nghĩa là"), not a predicate.
	vietnameseAsks = map[string]bool{"nào": true, "gì": true, "ai": true, "đâu": true, "nghĩa": true}
	// notNames holds the capitalised words of a title-case instruction ("Fill In The Blanks") that content.go's
	// functionWords leaves out.
	notNames = map[string]bool{"into": true, "out": true, "up": true, "down": true, "each": true, "every": true, "all": true, "your": true, "this": true, "these": true, "those": true, "which": true, "what": true, "any": true, "some": true, "yes": true, "not": true}
)

// named reports whether word reads as a name: capitalised, then lower case, and neither a function word, a count nor
// an answer word ("Write True if…", "Write Yes if…").
func named(word string) bool {
	word = strings.TrimSuffix(strings.TrimRight(word, ",;"), "'s")
	r, size := utf8.DecodeRuneInString(word)
	if !unicode.IsUpper(r) || len(word) == size || strings.ToLower(word[size:]) != word[size:] || !unicode.IsLetter([]rune(word[size:])[0]) {
		return false
	}
	lower := strings.ToLower(word)
	_, count := numberWords[lower]
	_, answer := trueFalseValue(word)
	return !notNames[lower] && !functionWords[lower] && !count && !answer
}

func startsLower(word string) bool {
	r, _ := utf8.DecodeRuneInString(word)
	return unicode.IsLower(r)
}

// vietnamesePredicate reports whether the first clause of a Vietnamese line holds a predicate word, matched as
// whole words (Go's \b only knows ASCII letters, so "là" inside "làm" must not count), and asks nothing: a sentence
// with a question word or a gloss ("Tìm từ khác loại là từ nào", "Chọn từ có nghĩa là …") is the task.
func vietnamesePredicate(sentence string) bool {
	return vietnamesePredicateBefore(sentence, vietnameseClause)
}

// vietnameseJoined reports whether text, an imperative-led line, holds a Vietnamese predicate only after a joining
// "và"/"rồi" in its first clause: a second command ("Đọc đoạn văn và chọn những câu là ý chính") or a passage with a
// compound subject ("Viết nhật ký và đọc sách là…"). The grammar cannot tell them apart, so the line stays the
// instruction and the draft points at it (D-075). A line that names an answer format is an instruction whatever its
// grammar, so it is not pointed at (QC-R6GRM-20).
func vietnameseJoined(text string) bool {
	m := imperative.FindStringSubmatchIndex(text)
	if m == nil {
		return false
	}
	sentence := text[m[2]:]
	if i := strings.IndexAny(sentence, ".?!:"); i >= 0 {
		sentence = sentence[:i]
	}
	return !answerFormat(text) && !vietnamesePredicateBefore(sentence, vietnameseClause) && vietnamesePredicateBefore(sentence, vietnamesePunctuation)
}

// vietnamesePredicateBefore is vietnamesePredicate with the first clause ending at end.
func vietnamesePredicateBefore(sentence string, end *regexp.Regexp) bool {
	clause := " " + norm.NFC.String(strings.ToLower(sentence)) + " "
	if loc := end.FindStringIndex(clause); loc != nil {
		clause = clause[:loc[0]]
	}
	words := strings.Fields(clause)
	if slices.ContainsFunc(words, func(w string) bool { return vietnameseAsks[w] }) {
		return false
	}
	for i := range words {
		for _, mark := range vietnameseMarks {
			if i+len(mark) <= len(words) && slices.Equal(words[i:i+len(mark)], mark) {
				return true
			}
		}
	}
	return false
}

// letterChoice matches a run of option letters that names an answer format: "A, B, C or D", "A–H", "A-E".
var letterChoice = regexp.MustCompile(`(?:^|[^\pL])[A-H](?:\s*,\s*[A-H])*\s*,?\s*(?:(?i:or|and|hoặc|và)\s+|[-–]\s*)[A-H](?:[^\pL]|$)`)

// answerFormat reports whether text names how to answer: True / False / Not given in either language, a word limit,
// "Choose N" or a run of option letters. A line that does is an instruction whatever grammar surrounds it.
func answerFormat(text string) bool {
	text = norm.NFC.String(text)
	return tasksOf(text).has(taskTFNG|taskTrueFalse) || chooseCount.MatchString(text) || wordLimit.MatchString(text) || vietnameseWordLimit.MatchString(text) || letterChoice.MatchString(text)
}

// vietnameseWordLimit matches "không quá hai từ", with a word boundary Go's ASCII-only \b cannot give.
// formatPatterns are the answer formats answerFormat looks for, as patterns a broken line can complete.
var formatPatterns = []*regexp.Regexp{trueFalseTask, tfngTask, chooseCount, wordLimit, vietnameseWordLimit, letterChoice}

var vietnameseWordLimit = regexp.MustCompile(`(?i)(?:^|[^\pL])không\s+quá\s+(?:\pL+|\d+)\s+từ(?:[^\pL]|$)`)

func normalizedInstruction(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.Trim(text, " .:;-–")), " "))
}
