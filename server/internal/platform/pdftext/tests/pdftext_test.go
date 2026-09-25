package pdftext_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/pdftext"
)

type run struct {
	x, y float64
	text string
	size float64
}

func build(pages [][]run, encrypted bool) []byte {
	var objects []string
	pageRefs := make([]string, len(pages))
	for i, page := range pages {
		var content strings.Builder
		for _, r := range page {
			size := r.size
			if size == 0 {
				size = 12
			}
			fmt.Fprintf(&content, "BT /F1 %.2f Tf %.2f %.2f Td (%s) Tj ET\n", size, r.x, r.y, r.text)
		}
		stream := content.String()
		contentID := 4 + 2*i + 1
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentID),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
		pageRefs[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	head := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageRefs, " "), len(pages)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	all := append(head, objects...)
	trailer := fmt.Sprintf("<< /Size %d /Root 1 0 R >>", len(all)+1)
	if encrypted {
		all = append(all, "<< /Filter /Standard /V 1 /R 2 /O <"+strings.Repeat("ab", 32)+"> /U <"+strings.Repeat("cd", 32)+"> /P -44 >>")
		trailer = fmt.Sprintf("<< /Size %d /Root 1 0 R /Encrypt %d 0 R /ID [<%s><%s>] >>", len(all)+1, len(all), strings.Repeat("01", 16), strings.Repeat("01", 16))
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(all))
	for i, body := range all {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(all)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n%s\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
	return out.Bytes()
}

var reader pdftext.Reader

func read(t *testing.T, data []byte, limits pdftext.Limits) (pdftext.Document, error) {
	t.Helper()
	return reader.Read(context.Background(), data, limits)
}

func TestLinesComeBackInReadingOrderWithRunsJoined(t *testing.T) {
	doc, err := read(t, build([][]run{
		{{72, 760, "TEST 1", 0}, {72, 740, "1. Which word is different?", 0}, {220, 720, "B. two", 0}, {72, 720, "A. one", 0}},
		{{72, 760, "TEST 1", 0}, {72, 740, "2. Choose the answer.", 0}},
	}, false), pdftext.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range doc.Lines {
		got = append(got, fmt.Sprintf("%d|%s", l.Page, l.Text))
	}
	want := []string{"1|TEST 1", "1|1. Which word is different?", "1|A. one\tB. two", "2|TEST 1", "2|2. Choose the answer."}
	if doc.Pages != 2 || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("pages %d, lines:\n%s", doc.Pages, strings.Join(got, "\n"))
	}
}

func TestRunsOnOneLineAreJoinedByTheGapBetweenThem(t *testing.T) {
	doc, err := read(t, build([][]run{{
		{72, 760, "Hel", 0}, {90, 760, "lo", 0},
		{72, 740, "Choose", 0}, {116.34, 740, "the answer", 0},
		{72, 720, "Question ", 0}, {122.04, 720, "1.", 0},
		{72, 700, "x", 0}, {78.5, 704, "2", 7},
	}}, false), pdftext.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range doc.Lines {
		got = append(got, l.Text)
	}
	want := []string{"Hello", "Choose the answer", "Question 1.", "x2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q, want %q", got, want)
	}
}

func TestAPageWithoutTextIsAScanNotAnEmptyExam(t *testing.T) {
	if _, err := read(t, build([][]run{{}}, false), pdftext.DefaultLimits()); !errors.Is(err, pdftext.ErrNoText) {
		t.Fatalf("err = %v, want ErrNoText", err)
	}
}

func TestAPasswordProtectedDocumentIsReportedAsEncrypted(t *testing.T) {
	if _, err := read(t, build([][]run{{{72, 760, "secret", 0}}}, true), pdftext.DefaultLimits()); !errors.Is(err, pdftext.ErrEncrypted) {
		t.Fatalf("err = %v, want ErrEncrypted", err)
	}
}

func TestReadingStaysWithinItsLimits(t *testing.T) {
	two := build([][]run{{{72, 760, "one", 0}}, {{72, 760, "two", 0}}}, false)
	for name, limits := range map[string]pdftext.Limits{
		"pages": {Pages: 1, Runes: 1000, Timeout: time.Minute},
		"runes": {Pages: 10, Runes: 4, Timeout: time.Minute},
	} {
		if _, err := read(t, two, limits); !errors.Is(err, pdftext.ErrLimit) {
			t.Fatalf("%s: err = %v, want ErrLimit", name, err)
		}
	}
	if _, err := read(t, two, pdftext.Limits{Pages: 10, Runes: 1000, Timeout: time.Nanosecond}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an expired deadline answered %v", err)
	}
	if _, err := read(t, two, pdftext.DefaultLimits()); err != nil {
		t.Fatalf("the reader did not recover after a timeout: %v", err)
	}
}

func fanOut(levels int) []byte {
	var leaf bytes.Buffer
	deflate, _ := zlib.NewWriterLevel(&leaf, zlib.BestCompression)
	_, _ = deflate.Write(bytes.Repeat([]byte("q Q\n"), 250_000))
	_ = deflate.Close()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /X 5 0 R >> >> /Contents 4 0 R >>",
		"<< /Length 60 >>\nstream\n" + strings.Repeat("/X Do ", 10) + "\nendstream",
	}
	for level := range levels {
		next := len(objects) + 2
		if level == levels-1 {
			objects = append(objects, fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", leaf.Len(), leaf.String()))
			break
		}
		objects = append(objects, fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << /XObject << /X %d 0 R >> >> /Length 60 >>\nstream\n%s\nendstream", next, strings.Repeat("/X Do ", 10)))
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func TestTheDeadlineStopsADocumentThatWouldKeepPDFiumBusy(t *testing.T) {
	var own pdftext.Reader
	small := build([][]run{{{72, 760, "warm", 0}}}, false)
	if _, err := own.Read(context.Background(), small, pdftext.DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := own.Read(context.Background(), fanOut(4), pdftext.Limits{Pages: 10, Runes: 1000, Timeout: 2 * time.Second})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("a runaway document answered %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("PDFium kept running past the two-second deadline")
	}
	if _, err := own.Read(context.Background(), small, pdftext.DefaultLimits()); err != nil {
		t.Fatalf("the reader did not recover after a runaway document: %v", err)
	}
	_ = own.Close()
}

func TestSomethingThatIsNotAPDFIsInvalid(t *testing.T) {
	if _, err := read(t, []byte("%PDF-1.4\nnot really"), pdftext.DefaultLimits()); !errors.Is(err, pdftext.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}
