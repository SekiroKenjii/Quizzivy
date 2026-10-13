package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"quizzivy/internal/core/adapters"
	aicommand "quizzivy/internal/modules/ai/application/command"
	aiports "quizzivy/internal/modules/ai/application/ports"
	aidomain "quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/imports/application/reader"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/pdftext/tests/scanpdf"
)

var scanReader pdftext.Reader

type ledger struct {
	mu     sync.Mutex
	events []string
}

func (l *ledger) note(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, fmt.Sprintf(format, args...))
}

type holds struct {
	ledger *ledger
	refuse error
}

func (h *holds) Handle(_ context.Context, c aicommand.Reserve) (aidomain.Reservation, error) {
	h.ledger.note("reserve %s %d for %s", c.Feature, c.Ceiling, c.Actor.ID)
	if h.refuse != nil {
		return aidomain.Reservation{}, h.refuse
	}
	return aidomain.Reservation{ID: "hold-1", UserID: c.Actor.ID, Feature: c.Feature, Credits: c.Ceiling}, nil
}

type settlements struct {
	ledger    *ledger
	got       []aicommand.Finish
	cancelled bool
}

func (s *settlements) Handle(ctx context.Context, c aicommand.Finish) (aidomain.Settled, error) {
	s.cancelled = s.cancelled || ctx.Err() != nil
	s.ledger.note("finish %d in %d out", c.Response.Usage.InputTokens, c.Response.Usage.OutputTokens)
	s.got = append(s.got, c)
	return aidomain.Settled{Charged: c.Response.Usage.InputTokens/1000 + c.Response.Usage.OutputTokens/100}, nil
}

type provider struct {
	ledger  *ledger
	answers map[int]string
	fail    map[int]error
	calls   []aiports.Request
	during  func(page int)
	after   map[int]int
	done    sync.WaitGroup
	mu      sync.Mutex
}

func (p *provider) Complete(_ context.Context, req aiports.Request) (aiports.Response, error) {
	var page, total int
	if _, err := fmt.Sscanf(req.Blocks[1].Text, "This is page %d of %d", &page, &total); err != nil {
		return aiports.Response{}, err
	}
	p.mu.Lock()
	p.calls = append(p.calls, req)
	p.mu.Unlock()
	p.ledger.note("complete page %d", page)
	if p.during != nil {
		p.during(page)
	}
	if err := p.fail[page]; err != nil {
		if p.after[page] > 0 {
			p.done.Wait()
		}
		return aiports.Response{}, err
	}
	if p.after != nil {
		defer p.done.Done()
	}
	return aiports.Response{Object: json.RawMessage(p.answers[page]), Usage: aiports.Usage{InputTokens: 2000, OutputTokens: 1200}, RequestID: fmt.Sprintf("req_%d", page)}, nil
}

func lines(readable bool, text ...string) string {
	raw, _ := json.Marshal(map[string]any{"readable": readable, "lines": text})
	return string(raw)
}

type rig struct {
	ledger      *ledger
	holds       *holds
	settlements *settlements
	provider    *provider
	reader      reader.Reader
}

func newRig(answers map[int]string) *rig {
	l := &ledger{}
	r := &rig{ledger: l, holds: &holds{ledger: l}, settlements: &settlements{ledger: l}, provider: &provider{ledger: l, answers: answers, fail: map[int]error{}}}
	r.reader = reader.Reader{Pages: adapters.PDFPages{PDF: &scanReader}, Reserve: r.holds, Finish: r.settlements, Completer: r.provider}
	return r
}

var twoPages = map[int]string{
	1: lines(true, "Part I. Choose the best answer", "1. Which one is a fruit?", "A. apple\tB. chair\tC. table\tD. door"),
	2: lines(true, "2. Which one is a colour?", "A. dog\tB. cat\tC. green\tD. run"),
}

func TestAScannedFileIsReadIntoLinesTheRecognizerAccepts(t *testing.T) {
	r := newRig(twoPages)
	read, err := r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1")
	if err != nil {
		t.Fatal(err)
	}
	if read.Pages != 2 || len(read.Unreadable) != 0 || len(read.Lines) != 5 {
		t.Fatalf("read %+v", read)
	}
	doc := pdftext.Document{Pages: read.Pages}
	for i, line := range read.Lines {
		doc.Lines = append(doc.Lines, pdftext.Line{Page: line.Page, Text: line.Text, Top: float64(-i)})
	}
	exam := adapters.AIEvidence(doc, read.Unreadable, "scan-1", "exam")
	key := adapters.PDFEvidence(pdftext.Document{Pages: 1, Lines: []pdftext.Line{{Page: 1, Text: "ĐÁP ÁN"}, {Page: 1, Text: "1. A\t2. C"}}}, "key-1", "answer_key")
	draft, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{exam, key}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	questions := draft.Questions()
	if len(questions) != 2 {
		t.Fatalf("recognized %d questions, want 2", len(questions))
	}
	if len(questions[0].Source) == 0 || questions[0].Source[0].SourceID != "scan-1" || !strings.HasPrefix(questions[0].Source[0].BlockID, "p1-l") {
		t.Fatalf("the first question does not point back at its page: %+v", questions[0].Source)
	}
	var read2 *domain.Finding
	for i, f := range draft.Notices {
		if f.Field == "AI_PAGE_READ" {
			read2 = &draft.Notices[i]
		}
	}
	if read2 == nil || read2.Count != 2 || read2.Severity != domain.Informational {
		t.Fatalf("the pages read by AI are not reported once each: %+v", draft.Notices)
	}
}

func TestTheCreditsAreHeldForEveryPageBeforeTheFirstIsSent(t *testing.T) {
	r := newRig(twoPages)
	if _, err := r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1"); err != nil {
		t.Fatal(err)
	}
	events := r.ledger.events
	if len(events) != 4 || events[0] != "reserve import 120 for teacher-1" || !strings.HasPrefix(events[3], "finish 4000 in 2400 out") {
		t.Fatalf("events = %q", events)
	}
	finish := r.settlements.got[0]
	if finish.Err != nil || finish.Tier != aiports.TierCapable || finish.Reservation.Credits != 120 || finish.Response.RequestID != "req_1" && finish.Response.RequestID != "req_2" {
		t.Fatalf("finish = %+v", finish)
	}
}

func TestEachPageIsSentAsAnImageWithTheExamsInstructionsAndNothingElse(t *testing.T) {
	r := newRig(twoPages)
	if _, err := r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1"); err != nil {
		t.Fatal(err)
	}
	if len(r.provider.calls) != 2 {
		t.Fatalf("%d calls, want 2", len(r.provider.calls))
	}
	for _, call := range r.provider.calls {
		if call.Tier != aiports.TierCapable || len(call.Blocks) != 2 || call.Blocks[0].Kind != aiports.BlockImage || call.Blocks[0].MediaType != "image/jpeg" || call.Blocks[1].Kind != aiports.BlockText {
			t.Fatalf("request = %+v", call)
		}
		if len(call.Blocks[0].Data) < 100 || strings.Contains(call.Blocks[1].Text+call.System, "teacher-1") {
			t.Fatalf("the request carries the wrong things: %d image bytes", len(call.Blocks[0].Data))
		}
	}
}

func TestAFailureMidFileSettlesWhatWasBilledReleasesTheRestAndFailsTheImport(t *testing.T) {
	r := newRig(map[int]string{1: twoPages[1], 2: twoPages[2], 3: twoPages[2]})
	r.provider.fail[3] = fmt.Errorf("%w: 529", aiports.ErrUnavailable)
	r.provider.after = map[int]int{3: 2}
	r.provider.done.Add(2)
	_, err := r.reader.Read(context.Background(), scanpdf.Build(3), "teacher-1")
	if !errors.Is(err, reader.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if len(r.settlements.got) != 1 {
		t.Fatalf("the reservation was ended %d times, want once", len(r.settlements.got))
	}
	finish := r.settlements.got[0]
	if finish.Err != nil || finish.Reservation.Credits != 180 {
		t.Fatalf("finish = %+v", finish)
	}
	if got := finish.Response.Usage; got.InputTokens != 4000 || got.OutputTokens != 2400 {
		t.Fatalf("settled usage = %+v, want the two pages that finished", got)
	}
}

func TestAFailureOnEveryPageReleasesTheWholeReservation(t *testing.T) {
	r := newRig(nil)
	for page := 1; page <= 2; page++ {
		r.provider.fail[page] = fmt.Errorf("%w: down", aiports.ErrUnavailable)
	}
	if _, err := r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1"); !errors.Is(err, reader.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if len(r.settlements.got) != 1 || r.settlements.got[0].Response.Usage != (aiports.Usage{}) {
		t.Fatalf("finish = %+v, want one call with no usage, which the credits model releases", r.settlements.got)
	}
}

func TestAFileOfMoreThanFortyPagesIsRefusedBeforeAnythingIsHeld(t *testing.T) {
	r := newRig(nil)
	_, err := r.reader.Read(context.Background(), scanpdf.Build(reader.MaxPages+1), "teacher-1")
	if !errors.Is(err, reader.ErrTooLong) {
		t.Fatalf("err = %v, want ErrTooLong", err)
	}
	if len(r.ledger.events) != 0 {
		t.Fatalf("events = %q, want none", r.ledger.events)
	}
	if _, err := r.reader.Pages.Draw(context.Background(), scanpdf.Build(reader.MaxPages), reader.MaxPages); err != nil {
		t.Fatalf("forty pages were refused: %v", err)
	}
}

func TestWhenTheCreditsFallShortNothingIsSent(t *testing.T) {
	for name, c := range map[string]struct {
		refuse error
		want   func(error) bool
	}{
		"the allowance": {&aidomain.Exhausted{Scope: aidomain.ScopeTeacher}, func(err error) bool {
			var short *reader.CreditsError
			return errors.As(err, &short) && short.Scope == aidomain.ScopeTeacher
		}},
		"the pool": {&aidomain.Exhausted{Scope: aidomain.ScopePool}, func(err error) bool {
			var short *reader.CreditsError
			return errors.As(err, &short) && short.Scope == aidomain.ScopePool
		}},
		"a pause": {aidomain.ErrPaused, func(err error) bool { return errors.Is(err, reader.ErrPaused) }},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(twoPages)
			r.holds.refuse = c.refuse
			_, err := r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1")
			if !c.want(err) {
				t.Fatalf("err = %v", err)
			}
			if len(r.provider.calls) != 0 || len(r.settlements.got) != 0 {
				t.Fatalf("calls %d, settlements %d: a refused hold sent or settled something", len(r.provider.calls), len(r.settlements.got))
			}
		})
	}
}

func TestAPageTheModelCannotReadIsAFindingOnThatPageNotAGap(t *testing.T) {
	r := newRig(map[int]string{1: twoPages[1], 2: lines(false), 3: twoPages[2]})
	read, err := r.reader.Read(context.Background(), scanpdf.Build(3), "teacher-1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(read.Unreadable, []int{2}) || read.Pages != 3 {
		t.Fatalf("read = %+v", read)
	}
	doc := pdftext.Document{Pages: read.Pages}
	for _, line := range read.Lines {
		doc.Lines = append(doc.Lines, pdftext.Line{Page: line.Page, Text: line.Text})
	}
	exam := adapters.AIEvidence(doc, read.Unreadable, "scan-1", "exam")
	draft, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{exam}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	var found *domain.Finding
	for i, f := range draft.Notices {
		if f.Field == "AI_PAGE_UNREADABLE" {
			found = &draft.Notices[i]
		}
	}
	if found == nil || found.Severity != domain.ReviewRequired || found.Count != 1 || len(found.Evidence) != 1 || found.Evidence[0].BlockID != "p2-l1" {
		t.Fatalf("notices = %+v", draft.Notices)
	}
}

func TestAPageTheProviderRefusesOrAnswersBadlyIsUnreadableAndTheRestAreRead(t *testing.T) {
	r := newRig(map[int]string{1: twoPages[1], 2: `{"unexpected":true}`, 3: twoPages[2]})
	read, err := r.reader.Read(context.Background(), scanpdf.Build(3), "teacher-1")
	if err != nil || !slices.Equal(read.Unreadable, []int{2}) {
		t.Fatalf("read = %+v, err = %v", read, err)
	}
	r = newRig(map[int]string{1: twoPages[1], 2: twoPages[2]})
	r.provider.fail[2] = fmt.Errorf("%w: refusal", aiports.ErrRefused)
	read, err = r.reader.Read(context.Background(), scanpdf.Build(2), "teacher-1")
	if err != nil || !slices.Equal(read.Unreadable, []int{2}) {
		t.Fatalf("read = %+v, err = %v", read, err)
	}
}

func TestACancelledReadStillEndsItsReservationOnAContextThatIsNotCancelled(t *testing.T) {
	r := newRig(twoPages)
	ctx, cancel := context.WithCancel(context.Background())
	r.provider.during = func(int) { cancel() }
	r.provider.fail[1] = context.Canceled
	r.provider.fail[2] = context.Canceled
	if _, err := r.reader.Read(ctx, scanpdf.Build(2), "teacher-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(r.settlements.got) != 1 || r.settlements.cancelled {
		t.Fatalf("finishes %d, run on a cancelled context: %v", len(r.settlements.got), r.settlements.cancelled)
	}
}
