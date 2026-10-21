package matchkey_test

import (
	"slices"
	"testing"

	"quizzivy/internal/shared/matchkey"
)

func TestKeysFollowTheStoredOrdinalUpToTheBounds(t *testing.T) {
	var options, items []string
	for ordinal := range 12 {
		options = append(options, matchkey.Option(ordinal))
	}
	for ordinal := range 20 {
		items = append(items, matchkey.Item(ordinal))
	}
	wantOptions := []string{"i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x", "xi", "xii"}
	wantItems := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T"}
	if !slices.Equal(options, wantOptions) {
		t.Errorf("options %v, want %v", options, wantOptions)
	}
	if !slices.Equal(items, wantItems) {
		t.Errorf("items %v, want %v", items, wantItems)
	}
	if got := matchkey.Item(26); got != "AA" {
		t.Errorf("item 26 is %q, want AA", got)
	}
	if matchkey.Option(-1) != "" || matchkey.Item(-1) != "" {
		t.Error("a negative ordinal has a key")
	}
}
