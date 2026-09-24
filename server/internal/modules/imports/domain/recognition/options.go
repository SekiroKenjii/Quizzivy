package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"strings"
	"unicode/utf8"
)

func (r *recognizer) options(b domain.EvidenceBlock, start, end int) error {
	text := b.Text[byteOffset(b.Text, start):byteOffset(b.Text, end)]
	matches := optionPattern.FindAllStringSubmatchIndex(text, 65)
	if len(matches) > 64 || len(matches)+len(r.out.Questions[r.question].Options) > 64 {
		return domain.ErrTooLarge
	}
	if len(matches) == 0 {
		return r.generatedOrContinuation(b, start, end, text)
	}
	if strings.TrimSpace(text[:matches[0][0]]) != "" {
		return nil
	}
	for i, m := range matches {
		a := start + utf8.RuneCountInString(text[:m[1]])
		z := end
		if i+1 < len(matches) {
			z = start + utf8.RuneCountInString(text[:matches[i+1][0]])
		}
		labelStart := start + utf8.RuneCountInString(text[:m[0]])
		if err := r.addOption(b, text[m[2]:m[3]], a, z, labelStart, false); err != nil {
			return err
		}
	}
	return nil
}

func (r *recognizer) addOption(b domain.EvidenceBlock, label string, start, end, labelStart int, generated bool) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	q := &r.out.Questions[r.question]
	if len(q.Options) >= 64 {
		return domain.ErrTooLarge
	}
	a, z := trimRange(b.Text, start, end)
	mark := ""
	if marked(b, a, z, r.out.Profile.AnswerMark) {
		mark = r.out.Profile.AnswerMark
	}
	content, err := prose(b, a, z, mark)
	if err != nil {
		return err
	}
	id := identity(q.ID, b.ID, label, "option")
	for _, previous := range q.Options {
		if previous.Label == label {
			r.issue("DUPLICATE_OPTION_LABEL", blocking, q.ID, "options", []domain.SourceRef{r.ref(b, labelStart, end)})
			id = identity(id, "duplicate", string(rune(len(q.Options))))
		}
	}
	q.Options = append(q.Options, domain.CandidateOption{ID: id, Label: label, Content: content})
	q.Type = "single_choice"
	ref := r.ref(b, a, z)
	q.Fields = append(q.Fields, domain.FieldEvidence{Field: "options/" + id, Origin: sourceExplicit, Refs: []domain.SourceRef{ref}})
	r.use(b, labelStart, end, q.ID, "options/"+id, generated)
	if mark != "" {
		key, exists := r.markKeys[q.ID]
		if !exists {
			key = domain.CandidateKey{ID: identity(q.ID, "mark-key"), Origin: "confirmed_convention", QuestionID: q.ID, QuestionLabel: q.Label, SectionLabel: q.SectionID, OptionLabels: []string{}, Evidence: []domain.SourceRef{}}
		}
		key.OptionLabels = append(key.OptionLabels, label)
		key.Evidence = append(key.Evidence, ref)
		r.markKeys[q.ID] = key
	}
	return nil
}

func (r *recognizer) generatedOrContinuation(b domain.EvidenceBlock, start, end int, text string) error {
	generated := optionPattern.FindStringSubmatch(b.Numbering)
	if generated == nil {
		if len(r.out.Questions[r.question].Options) == 0 && strings.TrimSpace(text) != "" {
			return r.continuation(b, start, end)
		}
		return nil
	}
	if err := r.addOption(b, generated[1], start, end, start, true); err != nil {
		return err
	}
	return nil
}
