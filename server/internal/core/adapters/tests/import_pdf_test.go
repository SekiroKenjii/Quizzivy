package adapters_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/platform/pdftext"
)

var pdfReader pdftext.Reader

func pdfOf(pages [][]string, encrypted bool) []byte {
	var objects []string
	refs := make([]string, len(pages))
	for i, lines := range pages {
		var content strings.Builder
		for j, line := range lines {
			fmt.Fprintf(&content, "BT /F1 11 Tf 72 %d Td (%s) Tj ET\n", 780-18*j, line)
		}
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()))
		refs[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	all := append([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(refs, " "), len(pages)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}, objects...)
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

func linesOf(pages ...[]string) pdftext.Document {
	doc := pdftext.Document{Pages: len(pages)}
	for i, page := range pages {
		for j, text := range page {
			doc.Lines = append(doc.Lines, pdftext.Line{Page: i + 1, Text: text, Top: float64(780 - 18*j), Left: 72})
		}
	}
	return doc
}

func hiddenLines(e domain.EvidenceDocument) []string {
	var out []string
	for _, b := range e.Blocks {
		if !b.Main {
			if b.Safe || !slices.Equal(b.Reasons, []string{"ANCILLARY_CONTENT_REQUIRES_REVIEW"}) {
				t := fmt.Sprintf("%s kept safe or unexplained: %+v", b.ID, b)
				out = append(out, t)
				continue
			}
			out = append(out, b.ID+" "+b.Text)
		}
	}
	return out
}

func TestPDFEvidenceKeepsRunningHeadersAndPageNumbersOutOfTheExam(t *testing.T) {
	header := "Trường THCS Minh Khai - Đề kiểm tra học kỳ"
	evidence := adapters.PDFEvidence(linesOf(
		[]string{header, "TEST 1", "1. Choose the odd one out", "A. one\tB. two", "Trang 1/3"},
		[]string{header, "2. Choose the odd one out", "A. red\tB. blue", "Trang 2/3"},
		[]string{header, "3. Choose the odd one out", "A. cat\tB. dog", "- 3 -"},
	), "source-1", "exam")
	want := []string{"p1-l1 " + header, "p1-l5 Trang 1/3", "p2-l1 " + header, "p2-l4 Trang 2/3", "p3-l1 " + header, "p3-l4 - 3 -"}
	if got := hiddenLines(evidence); !slices.Equal(got, want) {
		t.Fatalf("hidden lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if evidence.Version != pdftext.Version || evidence.SourceID != "source-1" || evidence.Role != "exam" || len(evidence.Blocks) != 13 {
		t.Fatalf("evidence identity: %+v", evidence)
	}
	if !slices.Contains(evidence.Findings, domain.EvidenceFinding{Code: "PDF_MARKS_UNAVAILABLE", Main: true}) {
		t.Fatal("the missing underline, bold and colour marks went unreported")
	}
}

func TestPDFEvidenceHidesNothingThatDoesNotRepeat(t *testing.T) {
	evidence := adapters.PDFEvidence(linesOf(
		[]string{"Đề số 1", "1. Choose the odd one out", "A. one\tB. two", "12"},
		[]string{"2. Choose the odd one out", "A. red\tB. blue", "3. Choose the odd one out", "A. cat\tB. dog"},
	), "source-1", "exam")
	if got, want := hiddenLines(evidence), []string{"p1-l4 12"}; !slices.Equal(got, want) {
		t.Fatalf("hidden lines = %q, want %q", got, want)
	}
}

func TestAPDFExamIsRecognizedWithItsAnswerKey(t *testing.T) {
	exam := adapters.PDFEvidence(linesOf([]string{
		"Part I. Choose the best answer",
		"1. Which one is a fruit?",
		"A. apple\tB. chair\tC. table\tD. door",
		"2. Which one is a colour?",
		"A. dog\tB. cat\tC. green\tD. run",
	}), "exam-source", "exam")
	key := adapters.PDFEvidence(linesOf([]string{"ĐÁP ÁN", "1. A\t2. C"}), "key-source", "answer_key")
	draft, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{exam, key}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	questions := draft.Questions()
	if len(questions) != 2 {
		t.Fatalf("recognized %d questions, want 2", len(questions))
	}
	for i, want := range []int{0, 2} {
		q := questions[i]
		if q.Answer.State != domain.AnswerKnown || !slices.Equal(q.Answer.OptionIDs, []string{q.Options[want].ID}) {
			t.Fatalf("question %d answer = %+v", i+1, q.Answer)
		}
		if len(q.Source) == 0 || q.Source[0].SourceID != "exam-source" || !strings.HasPrefix(q.Source[0].BlockID, "p1-l") {
			t.Fatalf("question %d does not point back at its PDF line: %+v", i+1, q.Source)
		}
	}
	marks := slices.IndexFunc(draft.Notices, func(f domain.Finding) bool {
		return f.Code == domain.CodeSourceObject && f.Field == "PDF_MARKS_UNAVAILABLE"
	})
	if marks < 0 || draft.Notices[marks].Severity != domain.Informational {
		t.Fatalf("notices = %+v", draft.Notices)
	}
}

func extractPDF(t *testing.T, engine adapters.ImportProcessing, data []byte) (ports.StageOutput, error) {
	t.Helper()
	return engine.Extract(context.Background(), ports.DocumentInput{OriginalID: "original", Identity: "original", Role: "exam", Format: "pdf", Body: bytes.NewReader(data), Bytes: int64(len(data))})
}

func TestPDFExtractionStagesTheTextAsEvidence(t *testing.T) {
	engine := adapters.ImportProcessing{PDF: &pdfReader, WorkDir: t.TempDir()}
	out, err := extractPDF(t, engine, pdfOf([][]string{{"TEST 1", "1. Choose the odd one out"}, {"2. Choose the odd one out"}}, false))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = out.Close() })
	var manifest domain.ExtractionManifest
	if err := json.Unmarshal(out.Plan.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != pdftext.Version || manifest.Identity != "original" || manifest.MainPart != "pdf" || manifest.BlockCount != 3 || len(manifest.Chunks) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if out.Plan.Stage != "extraction" || out.Plan.ComponentVersion != engine.ExtractionVersion("pdf") || engine.ExtractionVersion("pdf") == engine.ExtractionVersion("docx") {
		t.Fatalf("plan = %+v", out.Plan)
	}
	file, err := out.Open(manifest.EvidenceFile)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	var evidence domain.EvidenceDocument
	if err := json.Unmarshal(body, &evidence); err != nil {
		t.Fatal(err)
	}
	var text []string
	for _, b := range evidence.Blocks {
		text = append(text, b.ID+" "+b.Text)
	}
	if want := []string{"p1-l1 TEST 1", "p1-l2 1. Choose the odd one out", "p2-l1 2. Choose the odd one out"}; !slices.Equal(text, want) {
		t.Fatalf("evidence blocks = %q, want %q", text, want)
	}
}

func TestPDFExtractionFailuresNameWhatTheTeacherMustChange(t *testing.T) {
	long := make([][]string, 61)
	for i := range long {
		long[i] = []string{fmt.Sprintf("Page %d text", i+1)}
	}
	cases := []struct {
		name   string
		reader *pdftext.Reader
		data   []byte
		code   string
	}{
		{"not a PDF", &pdfReader, []byte("%PDF-1.4\nbroken"), "PDF_INVALID"},
		{"a scan without text", &pdfReader, pdfOf([][]string{{}}, false), "PDF_NO_TEXT"},
		{"a password", &pdfReader, pdfOf([][]string{{"secret"}}, true), "PDF_PROTECTED"},
		{"too many pages", &pdfReader, pdfOf(long, false), "PDF_TOO_LARGE"},
		{"no PDF reader", nil, pdfOf([][]string{{"text"}}, false), "SOURCE_UNSUPPORTED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := extractPDF(t, adapters.ImportProcessing{PDF: c.reader, WorkDir: t.TempDir()}, c.data)
			var failure worker.Failure
			if !errors.As(err, &failure) || failure.Code != c.code || failure.Retryable {
				t.Fatalf("err = %v, want terminal %s", err, c.code)
			}
		})
	}
}
