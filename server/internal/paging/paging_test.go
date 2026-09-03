package paging_test

import (
	"testing"

	"quizzivy/internal/paging"
)

func TestClampFillsInWhatTheCallerLeftOut(t *testing.T) {
	number, size, offset := paging.Clamp(0, 0, 20, 100)
	if number != 1 || size != 20 || offset != 0 {
		t.Errorf("Clamp(0, 0, 20, 100) = %d, %d, %d; want 1, 20, 0", number, size, offset)
	}
	if _, size, _ := paging.Clamp(1, 500, 20, 100); size != 100 {
		t.Errorf("limit above max = %d, want 100", size)
	}
	if _, _, offset := paging.Clamp(3, 20, 20, 100); offset != 40 {
		t.Errorf("page 3 of 20 starts at %d, want 40", offset)
	}
}

// A page from a query string, multiplied by a limit, overflows into a
// negative OFFSET that Postgres refuses.
func TestClampBoundsAPageTheArithmeticCannotHold(t *testing.T) {
	const int64Max = 1<<63 - 1
	for _, page := range []int{paging.MaxPage + 1, 1 << 40, int64Max} {
		number, size, offset := paging.Clamp(page, 100, 20, 100)
		if number != paging.MaxPage {
			t.Errorf("Clamp(%d, ...) served page %d, want %d", page, number, paging.MaxPage)
		}
		if offset < 0 {
			t.Errorf("Clamp(%d, ...) offset %d is negative", page, offset)
		}
		if want := (paging.MaxPage - 1) * size; offset != want {
			t.Errorf("Clamp(%d, ...) offset %d, want %d", page, offset, want)
		}
	}
}
