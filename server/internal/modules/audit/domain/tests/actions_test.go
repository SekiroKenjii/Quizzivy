package domain_test

import (
	"slices"
	"testing"

	"quizzivy/internal/modules/audit/domain"
)

func TestFoldRemovesCaseAndAccentsIncludingTheVietnameseD(t *testing.T) {
	cases := map[string]string{
		"Đã Phát Hành Bài Kiểm Tra": "da phat hanh bai kiem tra",
		"NGHÉ":                      "nghe",
		"ông Đỗ":                    "ong do",
		"Plain ASCII":               "plain ascii",
		"ơ ư ă â ê ô":               "o u a a e o",
		"":                          "",
	}
	for in, want := range cases {
		if got := domain.Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoldAgreesWithTheDatabasesUnaccentOnWhatTheCatalogueSays(t *testing.T) {
	for _, a := range domain.Actions() {
		for _, sentence := range []string{a.VI, a.EN} {
			folded := domain.Fold(sentence)
			for _, r := range folded {
				if r > 0x7F {
					t.Errorf("Fold(%q) = %q still holds %q", sentence, folded, r)
				}
			}
		}
	}
}

func TestASearchMatchesTheSentenceInEitherLanguageWithoutAccents(t *testing.T) {
	cases := []struct {
		q    string
		want string
	}{
		{"phát hành bài kiểm tra", "test.published"},
		{"PHAT HANH BAI KIEM TRA", "test.published"},
		{"published a test", "test.published"},
		{"  Published A Test  ", "test.published"},
		{"mã tham gia lớp", "class.join_code_issued"},
		{"ma tham gia lop", "class.join_code_revoked"},
	}
	for _, c := range cases {
		if got := domain.MatchingActions(c.q); !slices.Contains(got, c.want) {
			t.Errorf("%q matches %v, want it to include %s", c.q, got, c.want)
		}
	}
}

func TestASearchMatchesOnlyTheActionsWhoseSentenceHoldsIt(t *testing.T) {
	got := domain.MatchingActions("published")
	want := []string{"assignment.published", "test.published"}
	if !slices.Equal(got, want) {
		t.Errorf("\"published\" matches %v, want %v", got, want)
	}
	if got := domain.MatchingActions("zzzzzz"); len(got) != 0 {
		t.Errorf("a nonsense search matches %v", got)
	}
	if got := domain.MatchingActions("   "); got != nil {
		t.Errorf("a blank search matches %v, want none", got)
	}
}

func TestASearchDoesNotMatchTheKey(t *testing.T) {
	if got := domain.MatchingActions("join_code"); len(got) != 0 {
		t.Errorf("a search for the key's spelling matches %v", got)
	}
}

func TestSentenceFallsBackToTheKeyForAnActionItDoesNotKnow(t *testing.T) {
	if got := domain.Sentence("test.published", domain.English); got != "Published a test" {
		t.Errorf("English: %q", got)
	}
	if got := domain.Sentence("test.published", domain.Vietnamese); got != "Đã phát hành bài kiểm tra" {
		t.Errorf("Vietnamese: %q", got)
	}
	if got := domain.Sentence("widget.frobnicated_twice", domain.English); got != "widget frobnicated twice" {
		t.Errorf("unknown: %q", got)
	}
}
