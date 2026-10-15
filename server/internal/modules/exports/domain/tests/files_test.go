package domain_test

import (
	"strings"
	"testing"

	"quizzivy/internal/modules/exports/domain"
)

func TestTheCatalogueNamesEveryFileAndColumnInBothLanguages(t *testing.T) {
	seen := map[string]bool{}
	for _, file := range domain.Files(true) {
		if seen[file.Name] {
			t.Errorf("%s is listed twice", file.Name)
		}
		seen[file.Name] = true
		if !strings.HasSuffix(file.Name, ".csv") {
			t.Errorf("%s is not a CSV file", file.Name)
		}
		if strings.TrimSpace(file.VI) == "" || strings.TrimSpace(file.EN) == "" {
			t.Errorf("%s lacks a sentence in one language", file.Name)
		}
		if len(file.Columns) == 0 {
			t.Errorf("%s has no columns", file.Name)
		}
		names := map[string]bool{}
		for _, c := range file.Columns {
			if c.Name == "" || strings.TrimSpace(c.VI) == "" || strings.TrimSpace(c.EN) == "" {
				t.Errorf("%s: column %q lacks a name or a sentence in one language", file.Name, c.Name)
			}
			if names[c.Name] {
				t.Errorf("%s: column %s is listed twice", file.Name, c.Name)
			}
			names[c.Name] = true
			if c.Name != strings.ToLower(c.Name) || strings.ContainsAny(c.Name, " -") {
				t.Errorf("%s: column %q is not snake_case", file.Name, c.Name)
			}
		}
		if got := file.Header(); len(got) != len(file.Columns) || got[0] != file.Columns[0].Name {
			t.Errorf("%s: the header %v is not the columns in order", file.Name, got)
		}
	}
}

func TestNoFileHoldsASecret(t *testing.T) {
	banned := []string{"hash", "token", "secret", "session", "code", "key", "salt"}
	allowed := map[string]bool{"must_change_password": true, "password_changed_at": true, "builtin_key": true}
	for _, file := range domain.Files(true) {
		for _, c := range file.Columns {
			if allowed[c.Name] {
				continue
			}
			for _, word := range banned {
				if strings.Contains(c.Name, word) {
					t.Errorf("%s: column %s looks like a secret (%q)", file.Name, c.Name, word)
				}
			}
			if strings.Contains(c.Name, "password") {
				t.Errorf("%s: column %s looks like a credential", file.Name, c.Name)
			}
		}
	}
}

func TestAnswersAreAFileOnlyWhenAsked(t *testing.T) {
	without := domain.Files(false)
	with := domain.Files(true)
	if len(with) != len(without)+1 {
		t.Fatalf("asking for answers added %d files, want 1", len(with)-len(without))
	}
	for _, file := range without {
		if file.Name == domain.AnswersFile {
			t.Fatalf("%s is in an export that did not ask for answers", file.Name)
		}
	}
	if last := with[len(with)-1]; last.Name != domain.AnswersFile {
		t.Fatalf("the last file is %s, want %s", last.Name, domain.AnswersFile)
	}
	if again := domain.Files(false); len(again) != len(without) {
		t.Fatal("asking for answers changed the catalogue for the next export")
	}
}

func TestEveryJoiningColumnPointsAtAnotherFilesID(t *testing.T) {
	ids := map[string]bool{}
	for _, file := range domain.Files(true) {
		ids[strings.TrimSuffix(file.Name, ".csv")] = true
	}
	for _, file := range domain.Files(true) {
		for _, c := range file.Columns {
			if !strings.Contains(c.EN, ".csv)") {
				continue
			}
			target := c.EN[strings.LastIndex(c.EN, "(")+1 : len(c.EN)-1]
			if !ids[strings.TrimSuffix(target, ".csv")] {
				t.Errorf("%s.%s points at %s, which is not a file of the export", file.Name, c.Name, target)
			}
			if !strings.Contains(c.VI, "("+target+")") {
				t.Errorf("%s.%s names %s in English but not in Vietnamese: %s", file.Name, c.Name, target, c.VI)
			}
		}
	}
}

func TestTheReadmeIsVietnameseThenEnglishAndDescribesEveryColumn(t *testing.T) {
	files := domain.Files(true)
	readme := domain.Readme(files)
	split := strings.Index(readme, "\r\n==========\r\n")
	if split < 0 {
		t.Fatal("the README has no divider between its two languages")
	}
	vi, en := readme[:split], readme[split:]
	if !strings.HasPrefix(vi, "BẢN XUẤT DỮ LIỆU QUIZZIVY") {
		t.Errorf("the README does not open in Vietnamese: %.40q", vi)
	}
	if !strings.Contains(en, "QUIZZIVY DATA EXPORT") {
		t.Error("the English half has no title")
	}
	for _, file := range files {
		for half, language := range map[string]string{vi: "vi", en: "en"} {
			if !strings.Contains(half, "\r\n"+file.Name+"\r\n") {
				t.Errorf("the %s half does not name %s", language, file.Name)
			}
		}
		if !strings.Contains(vi, file.VI) || !strings.Contains(en, file.EN) {
			t.Errorf("%s: a description is missing from its half", file.Name)
		}
		for _, c := range file.Columns {
			if !strings.Contains(vi, "  "+c.Name+": "+c.VI+"\r\n") {
				t.Errorf("%s.%s: the Vietnamese half lacks its description", file.Name, c.Name)
			}
			if !strings.Contains(en, "  "+c.Name+": "+c.EN+"\r\n") {
				t.Errorf("%s.%s: the English half lacks its description", file.Name, c.Name)
			}
		}
	}
}

func TestTheReadmeUsesWindowsLineEndings(t *testing.T) {
	readme := domain.Readme(domain.Files(false))
	if strings.Contains(strings.ReplaceAll(readme, "\r\n", ""), "\n") {
		t.Error("a line of the README ends in a bare line feed")
	}
}

func TestTheReadmeOfAnExportWithoutAnswersDoesNotDescribeThem(t *testing.T) {
	readme := domain.Readme(domain.Files(false))
	if strings.Contains(readme, domain.AnswersFile) {
		t.Errorf("the README describes %s, which the ZIP does not hold", domain.AnswersFile)
	}
}

func TestTheStorageKeyIsPerExportAndInsideExports(t *testing.T) {
	if got := domain.StorageKey("0198f000-0000-7000-8000-000000000001"); got != "exports/0198f000-0000-7000-8000-000000000001.zip" {
		t.Fatalf("storage key = %q", got)
	}
}
