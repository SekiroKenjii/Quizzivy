package validation_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/oapi-codegen/runtime/types"

	"quizzivy/internal/shared/validation"
)

func TestParseEmailAdmitsWhatTheAPICanStoreAndPrintBack(t *testing.T) {
	for in, want := range map[string]string{
		"a@example.com":                   "a@example.com",
		"first.last+tag@example.com":      "first.last+tag@example.com",
		"  a@example.com  ":               "a@example.com",
		"Nguyễn Văn A <a@example.com>":    "a@example.com",
		"<a@example.com>":                 "a@example.com",
		`"a"@example.com`:                 "a@example.com",
		`"first.last"@example.com`:        "first.last@example.com",
		"a@example.com (note)":            "a@example.com",
		"hoc.sinh.é@example.com":          "hoc.sinh.é@example.com",
		"bücher@bücher.de":                "bücher@bücher.de",
		"qc@[1.2.3.4]":                    "qc@[1.2.3.4]",
		"a!#$%&'*+/=?^_`{|}~-b@e.example": "a!#$%&'*+/=?^_`{|}~-b@e.example",
	} {
		got, err := validation.ParseEmail(in)
		if err != nil || got != want {
			t.Errorf("ParseEmail(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestParseEmailRefusesWhatTheMarshallerCannotPrint(t *testing.T) {
	for _, in := range []string{
		`"qc tp"@example.com`,
		`"a..b"@example.com`,
		`".a"@example.com`,
		`"a."@example.com`,
		`"a\"b"@example.com`,
		`"a\\b"@example.com`,
		`"a\ b"@example.com`,
		`"a@b"@example.com`,
		`"a,b"@example.com`,
		`"a(b"@example.com`,
		`"a<b"@example.com`,
		`"a:b"@example.com`,
		`"a;b"@example.com`,
		`"a[b"@example.com`,
		"\"a\tb\"@example.com",
		`Name <"a b"@example.com>`,
		`"qc tp"@example.com (note)`,
		"a b@example.com",
		"a..b@example.com",
		".a@example.com",
		"a.@example.com",
		"a@",
		"@example.com",
		"example.com",
		"",
		"a@example.com, b@example.com",
	} {
		if got, err := validation.ParseEmail(in); err == nil {
			t.Errorf("ParseEmail(%q) = %q, nil; want a refusal", in, got)
		} else if !errors.Is(err, validation.ErrEmail) {
			t.Errorf("ParseEmail(%q) error %v does not wrap ErrEmail", in, err)
		}
	}
}

func pipeline(in string) (stored, printed string, err error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return "", "", err
	}
	var email types.Email
	if err := json.Unmarshal(raw, &email); err != nil {
		return "", "", err
	}
	body, err := json.Marshal(email)
	if err != nil {
		return string(email), "", err
	}
	err = json.Unmarshal(body, &printed)
	return string(email), printed, err
}

func candidates() []string {
	var out []string
	for c := rune(0); c < 128; c++ {
		s := string(c)
		for _, local := range []string{s, "a" + s, s + "a", "a" + s + "b", "a" + s + s + "b"} {
			out = append(out,
				local+"@example.com",
				`"`+local+`"@example.com`,
				`Name <`+local+`@example.com>`,
				`Name <"`+local+`"@example.com>`,
			)
		}
		out = append(out,
			`"a\`+s+`b"@example.com`,
			"a@"+s+"example.com",
			"a@example"+s+"com",
			"a@example.com"+s,
			s+"a@example.com",
		)
	}
	for c := rune(0); c < 128; c++ {
		for d := rune(0); d < 128; d++ {
			pair := string(c) + string(d)
			out = append(out, "a"+pair+"b@example.com", `"a`+pair+`b"@example.com`, `"a\`+pair+`b"@example.com`)
		}
	}
	return out
}

func TestEveryAddressParseEmailAdmitsSurvivesTheGeneratedTypes(t *testing.T) {
	var admitted int
	for _, in := range candidates() {
		want, err := validation.ParseEmail(in)
		if err != nil {
			continue
		}
		admitted++
		stored, printed, err := pipeline(in)
		if err != nil {
			t.Errorf("ParseEmail admitted %q but the generated type fails on it: %v", in, err)
			continue
		}
		if stored != want || printed != want {
			t.Errorf("ParseEmail admitted %q as %q but the type stores %q and prints %q", in, want, stored, printed)
		}
	}
	if admitted == 0 {
		t.Fatal("the sweep admitted nothing, so it proves nothing")
	}
}

func TestParseEmailRefusesNothingTheGeneratedTypesRoundTrip(t *testing.T) {
	for _, in := range candidates() {
		stored, printed, err := pipeline(in)
		if err != nil || printed != stored {
			continue
		}
		if got, err := validation.ParseEmail(in); err != nil || got != stored {
			t.Errorf("the generated type round-trips %q as %q but ParseEmail = %q, %v", in, stored, got, err)
		}
	}
}

func TestTheGeneratedTypeStillStoresWhatItCannotPrint(t *testing.T) {
	var email types.Email
	if err := json.Unmarshal([]byte(`"\"qc tp\"@example.com"`), &email); err != nil {
		t.Fatalf("the generated type now refuses the quoted address itself: %v; the rule that refuses it can go", err)
	}
	if _, err := json.Marshal(email); err == nil {
		t.Fatalf("the generated type now prints %q; the rule that refuses it can go", string(email))
	}
}
