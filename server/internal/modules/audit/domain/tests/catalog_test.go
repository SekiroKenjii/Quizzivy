package domain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"quizzivy/internal/modules/audit/domain"
)

var computedActions = []string{"ai.credit_request.approved", "ai.credit_request.declined"}

func catalogued() map[string]domain.Action {
	out := map[string]domain.Action{}
	for _, a := range domain.Actions() {
		out[a.Key] = a
	}
	return out
}

func locale(t *testing.T, name string) map[string]string {
	t.Helper()
	path := filepath.Join(serverRoot(t), "..", "web", "src", "lib", "i18n", "locales", name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var tree struct {
		Audit struct {
			Action map[string]string `json:"action"`
		} `json:"audit"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return tree.Audit.Action
}

func TestEveryActionTheServerWritesIsInTheCatalogue(t *testing.T) {
	actions, _ := scanServer(t)
	known := catalogued()
	for _, name := range sortedNames(actions) {
		if _, ok := known[name]; !ok {
			t.Errorf("the server writes the action %q (%s) and modules/audit/domain/actions.go does not list it: add it with its Vietnamese and English sentence, and the same two under audit.action in web/src/lib/i18n/locales/vi.json and en.json", name, actions[name][0])
		}
	}
	if len(actions) < 60 {
		t.Errorf("the scan found only %d actions: it is looking in the wrong place", len(actions))
	}
}

func TestEveryActionInTheCatalogueIsStillWritten(t *testing.T) {
	actions, _ := scanServer(t)
	for _, a := range domain.Actions() {
		if _, written := actions[a.Key]; !written && !slices.Contains(computedActions, a.Key) {
			t.Errorf("the catalogue holds %q, which no code writes any more: remove it, and its sentences from both locale files", a.Key)
		}
	}
}

func TestEveryEntityTheServerWritesHasACategoryOrIsAllOnly(t *testing.T) {
	_, entities := scanServer(t)
	for _, name := range sortedNames(entities) {
		_, categorised := domain.CategoryOf(name)
		if !categorised && !domain.AllOnly(name) {
			t.Errorf("the server writes entries about %q (%s) and modules/audit/domain/categories.go neither puts it in a category nor lists it as All-only", name, entities[name][0])
		}
	}
	if len(entities) < 15 {
		t.Errorf("the scan found only %d entities: it is looking in the wrong place", len(entities))
	}
}

func TestNoEntityIsBothInACategoryAndAllOnly(t *testing.T) {
	for _, entity := range domain.Entities() {
		_, categorised := domain.CategoryOf(entity)
		if categorised && domain.AllOnly(entity) {
			t.Errorf("%q is in a category and also listed as All-only", entity)
		}
	}
	seen := map[string]bool{}
	for _, entity := range domain.Entities() {
		if seen[entity] {
			t.Errorf("%q is listed twice", entity)
		}
		seen[entity] = true
	}
}

func TestTheCatalogueIsInKeyOrderWithOneSentencePerLanguage(t *testing.T) {
	keys := make([]string, 0, len(domain.Actions()))
	for _, a := range domain.Actions() {
		keys = append(keys, a.Key)
		if !actionShape.MatchString(a.Key) {
			t.Errorf("%q is not of the form noun.verb", a.Key)
		}
		for lang, sentence := range map[string]string{"vi": a.VI, "en": a.EN} {
			if strings.TrimSpace(sentence) == "" || strings.HasSuffix(sentence, ".") || sentence != strings.TrimSpace(sentence) {
				t.Errorf("%s sentence of %q is %q: it is blank, ends in a full stop or has spaces around it", lang, a.Key, sentence)
			}
		}
	}
	if !sort.StringsAreSorted(keys) {
		t.Errorf("modules/audit/domain/actions.go is not in key order")
	}
	if compact := slices.Compact(slices.Clone(keys)); len(compact) != len(keys) {
		t.Errorf("an action is listed twice")
	}
}

func TestBothLocaleFilesWordEveryActionAsTheCatalogueDoes(t *testing.T) {
	for name, want := range map[string]func(domain.Action) string{
		"vi": func(a domain.Action) string { return a.VI },
		"en": func(a domain.Action) string { return a.EN },
	} {
		words := locale(t, name)
		for _, a := range domain.Actions() {
			got, ok := words[a.Key]
			switch {
			case !ok:
				t.Errorf("%s.json lacks audit.action.%s, which the catalogue words as %q", name, a.Key, want(a))
			case got != want(a):
				t.Errorf("%s.json words audit.action.%s as %q and the catalogue as %q: the search matches the catalogue's, so they must be the same", name, a.Key, got, want(a))
			}
		}
		known := catalogued()
		for key := range words {
			if _, ok := known[key]; !ok {
				t.Errorf("%s.json has audit.action.%s, which is not an action in the catalogue", name, key)
			}
		}
	}
}

func sortedNames(f found) []string {
	out := f.names()
	sort.Strings(out)
	return out
}
