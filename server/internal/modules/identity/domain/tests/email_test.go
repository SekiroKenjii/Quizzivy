package domain_test

import (
	"testing"

	"quizzivy/internal/modules/identity/domain"
)

func TestAnAddressThatReadsAsItWritesHasNothingHidden(t *testing.T) {
	for _, address := range []string{
		"mai@x.test", "Mai.Nguyen+lop10@X.Test", "hoc.sinh.é@example.com", "bücher@bücher.de", "qc@[1.2.3.4]", "李@例え.jp",
	} {
		if r, found := domain.HiddenInEmail(address); found {
			t.Errorf("HiddenInEmail(%q) = %U, true; want nothing", address, r)
		}
	}
}

func TestTheFirstCharacterNoReaderCanSeeIsNamed(t *testing.T) {
	for _, r := range []rune{
		0x00, 0x07, 0x09, 0x0a, 0x20, 0xa0, 0xad, 0x115f, 0x1160, 0x2003, 0x200b, 0x200c, 0x200d, 0x2028, 0x2029, 0x202e,
		0x2066, 0x2800, 0x3000, 0x3164, 0xe000, 0xfeff, 0xffa0, 0xfffe,
	} {
		address := "a" + string(r) + "b" + string(rune(0x200b)) + "@x.test"
		if got, found := domain.HiddenInEmail(address); !found || got != r {
			t.Errorf("HiddenInEmail(%q) = %U, %v; want %U, true", address, got, found, r)
		}
	}
}

func TestAnAddressIsJudgedAsGivenNotComposed(t *testing.T) {
	decomposed := "e\u0301@x.test"
	if r, found := domain.HiddenInEmail(decomposed); found {
		t.Errorf("a combining accent is %U, hidden = %v; want an address that is kept as typed", r, found)
	}
}
