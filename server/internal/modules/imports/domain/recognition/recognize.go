package recognition

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/domain"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const Version = "rules-v1"
const sourceExplicit = "source_explicit"
const inferredStructure = "inferred_structure"
const reviewRequired = "review_required"
const blocking = "blocking"
const unknown = "unknown"
const examRole = "exam"
const keyRole = "answer_key"

var questionPattern = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(?:(?:question|câu)[\s\p{Zs}]+([0-9]{1,4})(?:[\s\p{Zs}]*[.):][\s\p{Zs}]*|[\s\p{Zs}]+|$)|([0-9]{1,4})[\s\p{Zs}]*[.):][\s\p{Zs}]*)`)
var sectionPattern = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(part|section|phần|bài)[\s\p{Zs}]+([0-9]+|[ivxlcdm]+)(?:[\s\p{Zs}]*[.\-:]|[\s\p{Zs}]+|$)`)
var optionPattern = regexp.MustCompile(`(?:^|[\s\p{Zs}])([A-H])[\s\p{Zs}]*[.)][\s\p{Zs}]*`)
var keyHeading = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(answer[\s\p{Zs}]*key|answers|đáp[\s\p{Zs}]*án)[\s\p{Zs}]*[:.\-]?[\s\p{Zs}]*$`)
var inlineKey = regexp.MustCompile(`(?i)(?:^|[\s\p{Zs}])(?:answer|đáp[\s\p{Zs}]*án)[\s\p{Zs}]*:[\s\p{Zs}]*([A-H](?:[\s\p{Zs}]*[,;/][\s\p{Zs}]*[A-H])*)[\s\p{Zs}]*$`)
var keyEntry = regexp.MustCompile(`(?i)(?:^|[\s\p{Zs}])(?:(?:question|câu)[\s\p{Zs}]+)?([0-9]{1,4})[\s\p{Zs}]*[.):\-][\s\p{Zs}]*([A-H](?:[\s\p{Zs}]*[,;/][\s\p{Zs}]*[A-H])*)`)
var paperHeading = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(đáp[\s\p{Zs}]*án[\s\p{Zs}]+)?đề[\s\p{Zs}]+(?:số[\s\p{Zs}]+)?([0-9]+)\b`)

type recognizer struct {
	ctx         context.Context
	out         domain.Candidate
	coverage    map[string]int
	section     int
	question    int
	inKeys      bool
	keySection  string
	paper       string
	doc         domain.EvidenceDocument
	lastNumber  int
	markKeys    map[string]domain.CandidateKey
	tables      map[string]bool
	tableBlocks map[string][]domain.EvidenceBlock
}

// Recognize parses explicit numbering and option/key labels, preserving ambiguous evidence as findings and never solving missing answers.
func Recognize(ctx context.Context, docs []domain.EvidenceDocument, p domain.RecognitionProfile) (domain.Candidate, error) {
	if err := validateEvidence(docs, p); err != nil {
		return domain.Candidate{}, err
	}
	r := recognizer{ctx: ctx, section: -1, question: -1, coverage: map[string]int{}, markKeys: map[string]domain.CandidateKey{}, tables: map[string]bool{}, out: domain.Candidate{Version: domain.CandidateVersion, RecognizerVersion: Version, Profile: p, Sources: []string{}, Sections: []domain.CandidateSection{}, Questions: []domain.CandidateQuestion{}, Keys: []domain.CandidateKey{}, Coverage: []domain.BlockCoverage{}, Issues: []domain.CandidateIssue{}}}
	ordered := slices.Clone(docs)
	slices.SortStableFunc(ordered, func(a, b domain.EvidenceDocument) int {
		if a.Role == b.Role {
			return 0
		}
		if a.Role == examRole {
			return -1
		}
		return 1
	})
	for _, d := range ordered {
		if err := r.document(d); err != nil {
			return domain.Candidate{}, err
		}
	}

	for _, q := range r.out.Questions {
		if key, ok := r.markKeys[q.ID]; ok {
			r.out.Keys = append(r.out.Keys, key)
		}
	}
	r.reconcile()
	r.checkCoverage(docs)
	if len(r.out.Questions) == 0 {
		r.issue("NO_RECOGNIZED_QUESTIONS", blocking, "", "", nil)
	}
	raw, err := json.Marshal(r.out)
	if err != nil {
		return domain.Candidate{}, err
	}
	if err := domain.ValidateRunResult(raw); err != nil {
		return domain.Candidate{}, err
	}
	return r.out, ctx.Err()
}

func validateEvidence(docs []domain.EvidenceDocument, p domain.RecognitionProfile) error {
	if len(docs) < 1 || len(docs) > 2 || !validProfile(p) {
		return domain.ErrInvalid
	}

	roles := map[string]bool{}
	sources := map[string]bool{}
	total, blocks := 0, 0
	examText := false
	for _, d := range docs {
		if d.Version != "ooxml-blocks-v1" || d.SourceID == "" || len(d.SourceID) > 128 || sources[d.SourceID] || roles[d.Role] || (d.Role != examRole && d.Role != keyRole) {
			return domain.ErrInvalid
		}
		roles[d.Role] = true
		sources[d.SourceID] = true
		size, hasText, err := validateDocument(d)
		if err != nil {
			return err
		}
		total += size
		blocks += len(d.Blocks)
		examText = examText || (d.Role == examRole && hasText)
	}
	if total > 8<<20 || blocks > 20000 {
		return domain.ErrTooLarge
	}
	if !roles[examRole] || !examText {
		return domain.ErrUnsupported
	}
	return nil
}
func validateDocument(d domain.EvidenceDocument) (int, bool, error) {
	total := 0
	hasText := false
	ids := map[string]bool{}
	for _, b := range d.Blocks {
		if b.ID == "" || len(b.ID) > 200 || ids[b.ID] || len(b.Numbering) > 256 || !utf8.ValidString(b.Text) {
			return 0, false, domain.ErrInvalid
		}
		ids[b.ID] = true
		total += len(b.Text)
		hasText = hasText || (b.Main && b.Safe && strings.TrimSpace(b.Text) != "")
		if err := validateSpans(b); err != nil {
			return 0, false, err
		}
	}
	return total, hasText, nil
}
func validateSpans(b domain.EvidenceBlock) error {
	if len(b.Text) > 400000 || len(b.Spans) > 20000 {
		return domain.ErrTooLarge
	}
	position := 0
	length := utf8.RuneCountInString(b.Text)
	for _, span := range b.Spans {
		if span.Start < position || span.End < span.Start || span.End > length {
			return domain.ErrInvalid
		}
		position = span.End
	}
	return nil
}

func (r *recognizer) document(d domain.EvidenceDocument) error {
	r.startDocument(d)
	for _, b := range d.Blocks {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		for _, reason := range b.Reasons {
			r.issue(reason, reviewRequired, b.ID, "source", []domain.SourceRef{r.ref(b, 0, utf8.RuneCountInString(b.Text))})
		}
		if !b.Main || !b.Safe || b.Kind != "paragraph" {
			continue
		}
		if err := r.block(b); err != nil {
			return err
		}
	}
	return nil
}

func (r *recognizer) block(b domain.EvidenceBlock) error {
	text := strings.TrimSpace(b.Text)
	end := utf8.RuneCountInString(b.Text)
	if text == "" {
		return nil
	}
	if r.paperHeading(b, text, end) {
		return nil
	}
	if keyHeading.MatchString(text) {
		r.inKeys = true
		r.question = -1
		r.keySection = ""
		r.use(b, 0, end, b.ID, "answer_heading", false)
		return nil
	}
	if r.sectionHeading(b, text, end) {
		return nil
	}
	if r.inKeys {
		if b.TableID != "" {
			r.tableKeys(b.TableID)
			if entry, ok := r.coverage[r.doc.SourceID+"/"+b.ID]; ok && len(r.out.Coverage[entry].Uses) > 0 {
				return nil
			}
		}
		return r.parseKeys(b)
	}
	return r.questionBlock(b)
}

func (r *recognizer) questionBlock(b domain.EvidenceBlock) error {
	end := utf8.RuneCountInString(b.Text)
	key := inlineKey.FindStringSubmatchIndex(b.Text)
	contentEnd := end
	if key != nil {
		contentEnd = utf8.RuneCountInString(b.Text[:key[0]])
	}
	label, start := questionMatch(b.Text[:byteOffset(b.Text, contentEnd)])
	generated := false
	if label == "" {
		if number, _ := questionMatch(b.Numbering); number != "" {
			label = number
			generated = true
		}
	}

	if label != "" {
		if err := r.newQuestion(b, label, start, contentEnd, generated); err != nil {
			return err
		}
	} else if r.question >= 0 {
		if err := r.options(b, 0, contentEnd); err != nil {
			return err
		}
	}
	if key != nil && r.question >= 0 {
		q := r.out.Questions[r.question]
		ref := r.ref(b, contentEnd, end)
		r.addKey(q.SectionID, q.Label, q.ID, optionLabels(b.Text[key[2]:key[3]]), ref)
		r.use(b, contentEnd, end, q.ID, "answer", false)
	}
	return nil
}

func (r *recognizer) newSection(b domain.EvidenceBlock, label, title string, inferred bool) {
	id := identity(r.doc.SourceID, b.ID, "section")
	refs := []domain.SourceRef{}
	if !inferred {
		refs = append(refs, r.ref(b, 0, utf8.RuneCountInString(b.Text)))
		r.use(b, 0, utf8.RuneCountInString(b.Text), id, "section", false)
	}
	r.out.Sections = append(r.out.Sections, domain.CandidateSection{ID: id, Label: label, Title: title, Evidence: refs})
	r.section = len(r.out.Sections) - 1
	r.question = -1
	r.lastNumber = 0
	if inferred {
		r.issue("SECTION_BOUNDARY_INFERRED", reviewRequired, id, "section", []domain.SourceRef{r.ref(b, 0, 0)})
	}
}

func (r *recognizer) newQuestion(b domain.EvidenceBlock, label string, start, end int, generated bool) error {
	if len(r.out.Questions) >= 2000 {
		return domain.ErrTooLarge
	}
	n, _ := strconv.Atoi(label)
	if r.section < 0 {
		r.newSection(b, "", "", true)
	} else if n <= r.lastNumber {
		r.newSection(b, r.out.Sections[r.section].Label, "", true)
	}
	r.lastNumber = n
	id := identity(r.doc.SourceID, b.ID, "question")
	opts := optionPattern.FindAllStringSubmatchIndex(b.Text[byteOffset(b.Text, start):byteOffset(b.Text, end)], 2)
	promptEnd := end
	if len(opts) >= 2 && b.Text[byteOffset(b.Text, start):][opts[0][2]:opts[0][3]] == "A" {
		promptEnd = start + utf8.RuneCountInString(b.Text[byteOffset(b.Text, start):][:opts[0][0]])
	}
	a, z := trimRange(b.Text, start, promptEnd)
	prompt, err := prose(b, a, z, "")
	if err != nil {
		return err
	}
	ref := r.ref(b, a, z)
	q := domain.CandidateQuestion{ID: id, SectionID: r.out.Sections[r.section].ID, Label: label, Type: unknown, Prompt: prompt, Options: []domain.CandidateOption{}, Answer: domain.CandidateAnswer{State: unknown, OptionIDs: []string{}, Evidence: []domain.SourceRef{}}, Points: "1", Fields: []domain.FieldEvidence{{Field: "prompt", Origin: sourceExplicit, Refs: []domain.SourceRef{ref}}, {Field: "type", Origin: inferredStructure, Refs: []domain.SourceRef{ref}}, {Field: "points", Origin: "defaulted", Refs: []domain.SourceRef{}}}}
	r.out.Questions = append(r.out.Questions, q)
	r.question = len(r.out.Questions) - 1
	r.use(b, 0, start, id, "question_label", generated)
	r.use(b, start, promptEnd, id, "prompt", false)
	r.issue("SCORING_DEFAULTED", reviewRequired, id, "points", nil)
	r.issue("QUESTION_BOUNDARY_INFERRED", reviewRequired, id, "prompt", []domain.SourceRef{ref})
	if promptEnd < end {
		return r.options(b, promptEnd, end)
	}
	return nil
}

func byteOffset(s string, codepoints int) int {
	if codepoints == 0 {
		return 0
	}
	count := 0
	for pos := range s {
		if count == codepoints {
			return pos
		}
		count++
	}
	return len(s)
}

func identity(parts ...string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(parts, "\x00"))).String()
}
func (r *recognizer) ref(b domain.EvidenceBlock, start, end int) domain.SourceRef {
	return domain.SourceRef{SourceID: r.doc.SourceID, BlockID: b.ID, Start: start, End: end}
}
func (r *recognizer) use(b domain.EvidenceBlock, start, end int, entity, field string, generated bool) {
	i, ok := r.coverage[r.doc.SourceID+"/"+b.ID]
	if !ok {
		return
	}
	r.out.Coverage[i].Uses = append(r.out.Coverage[i].Uses, domain.CoverageUse{EntityID: entity, Field: field, Start: start, End: end, GeneratedLabel: generated})
}
func (r *recognizer) issue(code, severity, entity, field string, refs []domain.SourceRef) {
	if refs == nil {
		refs = []domain.SourceRef{}
	}
	r.out.Issues = append(r.out.Issues, domain.CandidateIssue{ID: identity(code, entity, field, fmt.Sprint(refs)), Code: code, Severity: severity, EntityID: entity, Field: field, Evidence: refs})
}

func questionMatch(text string) (string, int) {
	match := questionPattern.FindStringSubmatchIndex(text)
	if match == nil {
		return "", 0
	}
	for i := 2; i < len(match); i += 2 {
		if match[i] >= 0 {
			return text[match[i]:match[i+1]], utf8.RuneCountInString(text[:match[1]])
		}
	}
	return "", 0
}

func validProfile(p domain.RecognitionProfile) bool {
	if p.Version != "auto-v1" {
		return false
	}
	if p.AnswerMark != "" && ((p.AnswerMark != "bold" && p.AnswerMark != "underline") || p.ConfirmedBy == "") {
		return false
	}
	return true
}

func (r *recognizer) sectionHeading(b domain.EvidenceBlock, text string, end int) bool {
	if match := sectionPattern.FindStringSubmatch(text); match != nil {
		label := strings.ToUpper(match[1] + " " + match[2])
		if r.paper != "" {
			label = r.paper + "/" + label
		}
		if r.inKeys {
			r.keySection = label
			r.use(b, 0, end, b.ID, "answer_section", false)
		} else {
			r.newSection(b, label, text, false)
		}
		return true
	}
	return false
}

func (r *recognizer) paperHeading(b domain.EvidenceBlock, text string, end int) bool {
	if match := paperHeading.FindStringSubmatch(text); match != nil {
		r.paper = "ĐỀ " + match[2]
		if match[1] != "" {
			r.inKeys = true
		}
		if r.inKeys {
			r.keySection = r.paper
			r.use(b, 0, end, b.ID, "answer_section", false)
		} else {
			r.newSection(b, r.paper, text, false)
		}
		return true
	}
	return false
}

func (r *recognizer) startDocument(d domain.EvidenceDocument) {
	r.doc = d
	r.out.Sources = append(r.out.Sources, d.SourceID)
	r.inKeys = d.Role == keyRole
	r.keySection = ""
	r.paper = ""
	r.tableBlocks = map[string][]domain.EvidenceBlock{}
	r.question = -1
	for _, code := range d.Findings {
		r.issue(code, reviewRequired, d.SourceID, "source", []domain.SourceRef{{SourceID: d.SourceID}})
	}
	for _, b := range d.Blocks {
		if b.TableID != "" {
			r.tableBlocks[b.TableID] = append(r.tableBlocks[b.TableID], b)
		}
		if b.Meaningful {
			r.coverage[d.SourceID+"/"+b.ID] = len(r.out.Coverage)
			r.out.Coverage = append(r.out.Coverage, domain.BlockCoverage{SourceID: d.SourceID, BlockID: b.ID, Uses: []domain.CoverageUse{}})
		}
	}
}
