package domain

import (
	"regexp"
	"sort"
	"strconv"
)

// QuestionManager reads prompts: which blanks a prompt names, in what order.
type QuestionManager struct{}

// PromptPlaceholders returns the distinct ordinals a prompt refers to, sorted.
func (QuestionManager) PromptPlaceholders(prompt string) []int {
	seen := map[int]bool{}
	for _, m := range placeholderPattern.FindAllStringSubmatch(prompt, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 {
			continue
		}
		seen[n] = true
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

var Questions QuestionManager

// placeholderPattern matches the 1-indexed {{n}} fill_blank markers.
var placeholderPattern = regexp.MustCompile(`\{\{(\d+)\}\}`)
