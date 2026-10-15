package domain_test

import (
	"strings"
	"testing"

	"quizzivy/internal/modules/identity/domain"
)

func TestAFullNameIsComposedAndItsSpaceTrimmedAndFolded(t *testing.T) {
	for in, want := range map[string]string{
		"Nguyễn Văn An":                      "Nguyễn Văn An",
		"  Nguyễn   Văn\tAn  ":               "Nguyễn Văn An",
		"Nguyễn Văn":                      "Nguyễn Văn",
		"Trần Thị　Bình":                      "Trần Thị Bình",
		"Lê\nVăn\r\nCường":                   "Lê Văn Cường",
		"O'Brien-Smith, Jr.":                 "O'Brien-Smith, Jr.",
		"李小龍":                                "李小龍",
		"R2-D2":                              "R2-D2",
		"+":                                  "+",
		"An" + string(rune(0x2028)) + "Bình": "An Bình",
	} {
		got, fault, hidden := domain.CleanFullName(in)
		if fault != domain.FullNameOK || hidden != 0 || got != want {
			t.Errorf("CleanFullName(%q) = %q, %v, %U; want %q", in, got, fault, hidden, want)
		}
	}
}

func TestAFullNameWithNothingInItIsRefusedAsEmpty(t *testing.T) {
	for _, in := range []string{"", "     ", "\t\n", " ", "　　", "\u0085", "\r\n"} {
		if _, fault, _ := domain.CleanFullName(in); fault != domain.FullNameEmpty {
			t.Errorf("CleanFullName(%q) fault = %v, want empty", in, fault)
		}
	}
}

func TestAFullNameLongerThanTheLimitIsRefusedAfterItIsComposed(t *testing.T) {
	at := strings.Repeat("ế", domain.MaxFullNameLength)
	if _, fault, _ := domain.CleanFullName(at); fault != domain.FullNameOK {
		t.Errorf("a name of %d characters: fault %v", domain.MaxFullNameLength, fault)
	}
	if _, fault, _ := domain.CleanFullName(at + "a"); fault != domain.FullNameTooLong {
		t.Errorf("a name of %d characters: fault %v, want too long", domain.MaxFullNameLength+1, fault)
	}
	decomposed := strings.Repeat("ế", domain.MaxFullNameLength)
	if _, fault, _ := domain.CleanFullName(decomposed); fault != domain.FullNameOK {
		t.Errorf("a name that is %d characters once composed: fault %v", domain.MaxFullNameLength, fault)
	}
}

func TestAFullNameWithACharacterNoReaderCanSeeIsRefusedAndNamesIt(t *testing.T) {
	for _, r := range []rune{
		0x0000, 0x0007, 0x00ad, 0x0378, 0x200b, 0x200c, 0x200d, 0x200e, 0x2060, 0x202e, 0x2066,
		0x2800, 0x3164, 0xe000, 0xfdd0, 0xfeff, 0xffa0, 0xfffe, 0xe0001, 0x10ffff,
	} {
		for _, in := range []string{"An" + string(r) + "Bình", string(r) + "An", "An" + string(r)} {
			if _, fault, hidden := domain.CleanFullName(in); fault != domain.FullNameHidden || hidden != r {
				t.Errorf("CleanFullName(%q) = fault %v, hidden %U; want hidden %U", in, fault, hidden, r)
			}
		}
		if _, fault, _ := domain.CleanFullName(string(r)); fault == domain.FullNameOK {
			t.Errorf("CleanFullName(%U) alone was kept", r)
		}
	}
}

func TestAFullNameWithNoLetterDigitPunctuationOrSymbolIsRefused(t *testing.T) {
	for _, in := range []string{"́", "́̂", "́ ̂"} {
		if _, fault, _ := domain.CleanFullName(in); fault != domain.FullNameNoGlyph {
			t.Errorf("CleanFullName(%q) fault = %v, want no glyph", in, fault)
		}
	}
	if _, fault, _ := domain.CleanFullName("́ An"); fault != domain.FullNameOK {
		t.Errorf("a name with a letter beside a lone accent: fault %v", fault)
	}
}
