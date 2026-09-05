package media

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Internal, because sanitiseFilename is where the bug was and going through
// Upload would need a database to reach it.

func TestSanitiseFilenameKeepsValidUTF8(t *testing.T) {
	long := strings.Repeat("Bài nghe tiếng Anh lớp 9 — Unit 3: luyện nghe số ", 6) + ".mp3"
	if utf8.RuneCountInString(long) <= 200 {
		t.Fatalf("premise: the fixture must exceed the limit, got %d runes",
			utf8.RuneCountInString(long))
	}

	got := sanitiseFilename(long)
	if !utf8.ValidString(got) {
		t.Errorf("truncated to invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > 200 {
		t.Errorf("kept %d runes, want at most 200", n)
	}
}

func TestSanitiseFilenameLimitIsRunesNotBytes(t *testing.T) {
	name := strings.Repeat("ế", 300)
	got := sanitiseFilename(name)
	if n := utf8.RuneCountInString(got); n != 200 {
		t.Errorf("kept %d runes, want exactly 200 -- the limit is counting bytes", n)
	}
	if !utf8.ValidString(got) {
		t.Error("truncated to invalid UTF-8")
	}
}

func TestSanitiseFilenameStillStripsPaths(t *testing.T) {
	// The properties that were already right, kept from regressing.
	for _, tc := range []struct{ in, want string }{
		{"../../etc/passwd", "passwd"},
		{`C:\Users\thuong\bài nghe.mp3`, "bài nghe.mp3"},
		{"  ", "tệp-không-tên"},
		{"", "tệp-không-tên"},
		{".", "tệp-không-tên"},
		{"/", "tệp-không-tên"},
		{"bình thường.mp3", "bình thường.mp3"},
	} {
		if got := sanitiseFilename(tc.in); got != tc.want {
			t.Errorf("sanitiseFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
