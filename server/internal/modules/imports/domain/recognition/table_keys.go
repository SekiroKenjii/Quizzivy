package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var tableNumber = regexp.MustCompile(`^[0-9]{1,4}$`)
var tableAnswer = regexp.MustCompile(`(?i)^[A-H](?:\s*[,;/]\s*[A-H])*$`)

type cellPosition struct{ row, column int }

func (r *recognizer) tableKeys(id string) {
	key := r.doc.SourceID + "/" + id
	if r.tables[key] {
		return
	}
	r.tables[key] = true
	pairs, uses := tableKeyPairs(r.tableBlocks[id], id)
	for _, pair := range pairs {
		number, answer := pair[0], pair[1]
		if uses[answer.ID] != 1 {
			continue
		}
		r.addKey(r.keySection, strings.TrimSpace(number.Text), "", optionLabels(strings.TrimSpace(answer.Text)), r.ref(number, 0, utf8.RuneCountInString(number.Text)))
		k := &r.out.Keys[len(r.out.Keys)-1]
		k.Evidence = append(k.Evidence, r.ref(answer, 0, utf8.RuneCountInString(answer.Text)))
		r.use(number, 0, utf8.RuneCountInString(number.Text), k.ID, "answer_key", false)
		r.use(answer, 0, utf8.RuneCountInString(answer.Text), k.ID, "answer_key", false)
	}
}

func tableKeyPairs(blocks []domain.EvidenceBlock, id string) ([][2]domain.EvidenceBlock, map[string]int) {
	cells, ambiguous, numbers := keyTableCells(blocks, id)
	pairs := [][2]domain.EvidenceBlock{}
	uses := map[string]int{}
	for _, number := range numbers {
		if ambiguous[cellPosition{number.Row, number.Column}] {
			continue
		}
		options := []domain.EvidenceBlock{}
		for _, pos := range []cellPosition{{number.Row, number.Column + 1}, {number.Row + 1, number.Column}} {
			b, ok := cells[pos]
			if ok && !ambiguous[pos] && tableAnswer.MatchString(strings.TrimSpace(b.Text)) {
				options = append(options, b)
			}
		}
		if len(options) != 1 {
			continue
		}
		answer := options[0]
		pairs = append(pairs, [2]domain.EvidenceBlock{number, answer})
		uses[answer.ID]++
	}
	return pairs, uses
}

func keyTableCells(blocks []domain.EvidenceBlock, id string) (map[cellPosition]domain.EvidenceBlock, map[cellPosition]bool, []domain.EvidenceBlock) {
	cells := map[cellPosition]domain.EvidenceBlock{}
	ambiguous := map[cellPosition]bool{}
	numbers := []domain.EvidenceBlock{}
	for _, b := range blocks {
		if b.TableID != id || !b.Main || !b.Safe || !b.Meaningful || slices.Contains(b.Reasons, "TABLE_GRID_REQUIRES_REVIEW") {
			continue
		}
		pos := cellPosition{b.Row, b.Column}
		if _, exists := cells[pos]; exists {
			ambiguous[pos] = true
		}
		cells[pos] = b
		if tableNumber.MatchString(strings.TrimSpace(b.Text)) {
			numbers = append(numbers, b)
		}
	}
	return cells, ambiguous, numbers
}
