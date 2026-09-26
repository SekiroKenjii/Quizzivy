// Package pdftext reads the text layer of an untrusted PDF. PDFium runs as
// WebAssembly inside wazero with no filesystem, no network, bounded memory and a
// deadline, so a hostile document can at worst fail its own read.
package pdftext

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/klippa-app/go-pdfium"
	pdfiumerrors "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Version names this extraction, so stage reuse never mixes its output with another's.
const Version = "pdf-lines-v1"

var (
	// ErrEncrypted reports a document that needs a password to open.
	ErrEncrypted = errors.New("pdftext: the document is encrypted")
	// ErrNoText reports a document without a text layer, such as a scan.
	ErrNoText = errors.New("pdftext: the document has no text layer")
	// ErrLimit reports a document over the page or character limit.
	ErrLimit = errors.New("pdftext: the document exceeds the reading limits")
	// ErrInvalid reports a document PDFium cannot parse.
	ErrInvalid = errors.New("pdftext: the document cannot be read")
)

// Limits bounds one read: pages, characters of text, and wall time.
type Limits struct {
	Pages, Runes int
	Timeout      time.Duration
}

// DefaultLimits are 60 pages, 2 Mi characters and one minute; the character
// limit keeps the text within the recognizer's 8 MiB.
func DefaultLimits() Limits {
	return Limits{Pages: 60, Runes: 2 << 20, Timeout: time.Minute}
}

// Line is one line of text as laid out on a page: Page counts from 1, Top and
// Left are in points from the page's bottom-left corner.
type Line struct {
	Page      int
	Text      string
	Top, Left float64
}

// Document is a PDF's text in reading order: pages in order, lines top to bottom.
type Document struct {
	Pages int
	Lines []Line
}

// Reader reads one document at a time, each in a fresh PDFium sandbox so no
// memory outlives the read. It keeps only the compiled module, because compiling
// it takes seconds. Close releases that.
type Reader struct {
	mu    sync.Mutex
	cache wazero.CompilationCache
}

const memoryPages = 4096

// Read returns the text of data, or ErrEncrypted, ErrNoText, ErrLimit or ErrInvalid.
func (r *Reader) Read(ctx context.Context, data []byte, limits Limits) (Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	pool, err := r.start()
	if err != nil {
		return Document{}, err
	}
	defer func() { _ = pool.Close() }()
	instance, err := pool.GetInstanceWithContext(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Document{}, ctxErr
		}
		return Document{}, fmt.Errorf("pdftext: sandbox unavailable: %w", err)
	}
	killed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(killed)
		_ = instance.Kill()
	})
	defer func() {
		if stop() {
			_ = instance.Close()
			return
		}
		<-killed
	}()
	doc, err := read(instance, data, limits)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Document{}, ctxErr
	}
	return doc, err
}

// Close releases the compiled module; a later Read compiles it again.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		return nil
	}
	err := r.cache.Close(context.Background())
	r.cache = nil
	return err
}

func (r *Reader) start() (pdfium.Pool, error) {
	if r.cache == nil {
		r.cache = wazero.NewCompilationCache()
	}
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle: 1, MaxIdle: 1, MaxTotal: 1,
		FSConfig: wazero.NewFSConfig(),
		RuntimeConfig: wazero.NewRuntimeConfig().
			WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling).
			WithMemoryLimitPages(memoryPages).
			WithCloseOnContextDone(true).
			WithCompilationCache(r.cache),
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("pdftext: start sandbox: %w", err)
	}
	return pool, nil
}

func read(instance pdfium.Pdfium, data []byte, limits Limits) (Document, error) {
	opened, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		if errors.Is(err, pdfiumerrors.ErrPassword) || errors.Is(err, pdfiumerrors.ErrSecurity) {
			return Document{}, ErrEncrypted
		}
		return Document{}, ErrInvalid
	}
	defer func() {
		_, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: opened.Document})
	}()
	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: opened.Document})
	if err != nil {
		return Document{}, ErrInvalid
	}
	if count.PageCount < 1 || count.PageCount > limits.Pages {
		return Document{}, ErrLimit
	}
	out := Document{Pages: count.PageCount}
	runes := 0
	for page := range count.PageCount {
		found, err := pageLines(instance, opened.Document, page)
		if err != nil {
			return Document{}, err
		}
		for _, line := range found {
			runes += utf8.RuneCountInString(line.Text)
		}
		if runes > limits.Runes {
			return Document{}, ErrLimit
		}
		out.Lines = append(out.Lines, found...)
	}
	if runes == 0 {
		return Document{}, ErrNoText
	}
	return out, nil
}

func pageLines(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, page int) ([]Line, error) {
	text, err := instance.GetPageTextStructured(&requests.GetPageTextStructured{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: page}},
		Mode: requests.GetPageTextStructuredModeChars,
	})
	if err != nil {
		return nil, ErrInvalid
	}
	return lines(page+1, text.Chars), nil
}

type box struct{ left, right, top, bottom float64 }

func (b box) height() float64 { return b.top - b.bottom }

func (b box) middle() float64 { return (b.top + b.bottom) / 2 }

func (b box) sharesLine(o box) bool {
	thin, other := b, o
	if o.height() < b.height() {
		thin, other = o, b
	}
	switch {
	case other.height() <= 1:
		return math.Abs(thin.middle()-other.middle()) <= 3
	case thin.height() <= 1:
		return thin.middle() >= other.bottom && thin.middle() <= other.top
	}
	return min(b.top, o.top)-max(b.bottom, o.bottom) >= 0.3*thin.height()
}

func (b box) union(o box) box {
	return box{left: min(b.left, o.left), right: max(b.right, o.right), top: max(b.top, o.top), bottom: min(b.bottom, o.bottom)}
}

type segment struct {
	text   strings.Builder
	spaced bool
	area   box
	last   box
}

type textRow struct {
	segments []*segment
	area     box
}

func lines(page int, chars []*responses.GetPageTextStructuredChar) []Line {
	glyphs := glyphsOf(chars)
	em := typicalHeight(glyphs)
	segments := segmentsOf(glyphs, em)
	slices.SortStableFunc(segments, func(a, b *segment) int { return cmp.Compare(b.area.top, a.area.top) })
	var rows []*textRow
	for _, s := range segments {
		if n := len(rows); n > 0 && rows[n-1].area.sharesLine(s.area) {
			rows[n-1].segments = append(rows[n-1].segments, s)
			rows[n-1].area = rows[n-1].area.union(s.area)
			continue
		}
		rows = append(rows, &textRow{segments: []*segment{s}, area: s.area})
	}
	out := make([]Line, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.line(page, em))
	}
	return out
}

type glyph struct {
	r              rune
	area           box
	spaced, broken bool
}

func glyphsOf(chars []*responses.GetPageTextStructuredChar) []glyph {
	out := make([]glyph, 0, len(chars))
	spaced, broken := false, false
	for _, c := range chars {
		r, _ := utf8.DecodeRuneInString(c.Text)
		switch {
		case r == '\r' || r == '\n':
			broken = true
			continue
		case unicode.IsSpace(r):
			spaced = true
			continue
		case c.Text == "" || !unicode.IsPrint(r):
			continue
		}
		p := c.PointPosition
		area := box{left: p.Left, right: p.Right, top: max(p.Top, p.Bottom), bottom: min(p.Top, p.Bottom)}
		out = append(out, glyph{r: r, area: area, spaced: spaced, broken: broken})
		spaced, broken = false, false
	}
	return out
}

func typicalHeight(glyphs []glyph) float64 {
	heights := make([]float64, 0, len(glyphs))
	for _, g := range glyphs {
		if h := g.area.height(); h > 1 {
			heights = append(heights, h)
		}
	}
	if len(heights) == 0 {
		return 1
	}
	slices.Sort(heights)
	return heights[len(heights)/2]
}

func segmentsOf(glyphs []glyph, em float64) []*segment {
	var out []*segment
	var current *segment
	for _, g := range glyphs {
		if current != nil && current.continues(g.area, em) {
			if g.spaced || (g.broken && g.area.left-current.last.right > em/4) {
				current.text.WriteByte(' ')
			}
			current.text.WriteRune(g.r)
			current.area, current.last = current.area.union(g.area), g.area
			continue
		}
		current = &segment{spaced: g.spaced, area: g.area, last: g.area}
		current.text.WriteRune(g.r)
		out = append(out, current)
	}
	return out
}

func (s *segment) continues(g box, em float64) bool {
	gap := g.left - s.last.right
	scale := max(s.area.height(), em)
	return s.area.sharesLine(g) && gap >= -scale/2 && gap <= 2*scale
}

func (r *textRow) line(page int, em float64) Line {
	slices.SortStableFunc(r.segments, func(a, b *segment) int { return cmp.Compare(a.area.left, b.area.left) })
	height := max(r.area.height(), em)
	var text strings.Builder
	for i, s := range r.segments {
		if i > 0 {
			gap := s.area.left - r.segments[i-1].area.right
			switch {
			case gap > 2*height:
				text.WriteByte('\t')
			case s.spaced || gap > height/4:
				text.WriteByte(' ')
			}
		}
		text.WriteString(s.text.String())
	}
	return Line{Page: page, Text: text.String(), Top: r.area.top, Left: r.area.left}
}
