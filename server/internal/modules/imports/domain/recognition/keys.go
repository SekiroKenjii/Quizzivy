package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const conflicting = "conflicting"

func optionLabels(raw string) []string {
	return strings.FieldsFunc(strings.ToUpper(raw), func(r rune) bool { return r == ',' || r == ';' || r == '/' || unicode.IsSpace(r) })
}

func (r *recognizer) addKey(section, label, id string, options []string, ref domain.SourceRef) {
	r.out.Keys = append(r.out.Keys, domain.CandidateKey{ID: identity(ref.SourceID, ref.BlockID, label, strconv.Itoa(ref.Start), "key"), Origin: sourceExplicit, SectionLabel: section, QuestionLabel: label, QuestionID: id, OptionLabels: options, Evidence: []domain.SourceRef{ref}})
}

func (r *recognizer) parseKeys(b domain.EvidenceBlock) error {
	matches := keyEntry.FindAllStringSubmatchIndex(b.Text, 10001)
	if len(matches)+len(r.out.Keys) > 10000 {
		return domain.ErrTooLarge
	}
	for _, m := range matches {
		if m[1] < len(b.Text) {
			next, _ := utf8.DecodeRuneInString(b.Text[m[1]:])
			if unicode.IsLetter(next) || unicode.IsDigit(next) {
				continue
			}
		}
		a, z := utf8.RuneCountInString(b.Text[:m[0]]), utf8.RuneCountInString(b.Text[:m[1]])
		label := b.Text[m[2]:m[3]]
		r.addKey(r.keySection, label, "", optionLabels(b.Text[m[4]:m[5]]), r.ref(b, a, z))
		r.use(b, a, z, r.out.Keys[len(r.out.Keys)-1].ID, "answer_key", false)
	}
	return nil
}

func (r *recognizer) reconcile() {
	sections := map[string]string{}
	for _, s := range r.out.Sections {
		sections[s.ID] = s.Label
	}
	byLabel := map[string][]string{}
	bySection := map[string][]string{}
	ids := map[string]bool{}
	for _, q := range r.out.Questions {
		ids[q.ID] = true
		label := normalizedLabel(q.Label)
		byLabel[label] = append(byLabel[label], q.ID)
		key := sections[q.SectionID] + "/" + label
		bySection[key] = append(bySection[key], q.ID)
	}
	matched := make(map[string][]domain.CandidateKey)
	for i := range r.out.Keys {
		key := &r.out.Keys[i]
		candidates := byLabel[normalizedLabel(key.QuestionLabel)]
		if key.SectionLabel != "" {
			candidates = bySection[key.SectionLabel+"/"+normalizedLabel(key.QuestionLabel)]
		}
		if key.QuestionID != "" {
			candidates = nil
			if ids[key.QuestionID] {
				candidates = []string{key.QuestionID}
			}
		}

		if len(candidates) != 1 {
			r.issue("AMBIGUOUS_ANSWER_MAPPING", blocking, key.ID, "answer", key.Evidence)
			continue
		}
		key.QuestionID = candidates[0]
		matched[key.QuestionID] = append(matched[key.QuestionID], *key)
	}
	for i := range r.out.Questions {
		r.questionAnswers(i, matched[r.out.Questions[i].ID])
	}
}

func (r *recognizer) questionAnswers(index int, keys []domain.CandidateKey) {
	q := &r.out.Questions[index]
	if len(q.Options) < 2 {
		r.issue("UNRECOGNIZED_INTERACTION", blocking, q.ID, "type", nil)
		q.Type = unknown
	}
	if len(keys) == 0 {
		r.issue("MISSING_ANSWER", blocking, q.ID, "answer", nil)
		return
	}
	for _, key := range keys {
		options, ok := resolveLabels(q.Options, key.OptionLabels)
		if !ok {
			q.Answer.State = conflicting
			r.issue("DANGLING_ANSWER_LABEL", blocking, q.ID, "answer", key.Evidence)
			continue
		}
		q.Answer.Evidence = append(q.Answer.Evidence, key.Evidence...)
		if q.Answer.State == unknown {
			q.Answer.State = "known"
			q.Answer.OptionIDs = options
		} else if !slices.Equal(q.Answer.OptionIDs, options) {
			q.Answer.State = conflicting
		}
	}
	if q.Answer.State == conflicting {
		q.Answer.OptionIDs = []string{}
		r.issue("CONFLICTING_ANSWER_KEYS", blocking, q.ID, "answer", q.Answer.Evidence)
	} else if len(q.Answer.OptionIDs) > 1 {
		q.Type = "multiple_choice"
	}
}

func resolveLabels(options []domain.CandidateOption, labels []string) ([]string, bool) {
	ids := []string{}
	seen := map[string]bool{}
	for _, label := range labels {
		if seen[label] {
			return nil, false
		}
		seen[label] = true
		matches := []string{}
		for _, option := range options {
			if option.Label == label {
				matches = append(matches, option.ID)
			}
		}
		if len(matches) != 1 {
			return nil, false
		}
		ids = append(ids, matches[0])
	}
	slices.Sort(ids)
	return ids, len(ids) > 0
}

func normalizedLabel(label string) string {
	n, err := strconv.Atoi(label)
	if err != nil {
		return label
	}
	return strconv.Itoa(n)
}
