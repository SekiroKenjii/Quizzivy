package tabular

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html/charset"
)

const (
	maxEntries            = 1000
	maxExpandedBytes      = 32 << 20
	unzipXMLBytes         = 16 << 20
	maxXMLPartBytes       = 1 << 20
	maxSharedStringsBytes = 2 << 20
	maxWorksheetBytes     = 8 << 20
	maxXMLDepth           = 64
	maxTagBytes           = 64 << 10
	encryptedEntry        = 0x1
	workbookPart          = "xl/workbook.xml"
	rootRelationshipsPart = "_rels/.rels"
)

var (
	compoundFileSignature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	encryptedPackageName  = utf16LE("EncryptedPackage")
	namedReadParts        = map[string]bool{
		workbookPart:                 true,
		"xl/_rels/workbook.xml.rels": true,
		rootRelationshipsPart:        true,
		"xl/styles.xml":              true,
		"xl/sharedstrings.xml":       true,
		"xl/calcchain.xml":           true,
	}
)

type xlsxSource struct {
	book *excelize.File
	rows *excelize.Rows
	line int
}

func openXLSX(src io.ReaderAt, size int64) (source, error) {
	archive, err := inspectArchive(src, size)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(archive.File, func(entry *zip.File) bool { return isSheetPart(partName(entry.Name)) }) {
		return nil, ErrLayout
	}
	if err := verifyParts(archive.File); err != nil {
		return nil, err
	}
	book, err := excelize.OpenReader(io.NewSectionReader(src, 0, size), excelize.Options{
		UnzipSizeLimit:    maxExpandedBytes,
		UnzipXMLSizeLimit: unzipXMLBytes,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrFormat, err), closeWorkbook(book))
	}
	dropUnreadParts(book)
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.Join(fmt.Errorf("%w: the workbook has no sheet", ErrFormat), book.Close())
	}
	rows, err := book.Rows(sheets[0])
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrFormat, err), book.Close())
	}
	return &xlsxSource{book: book, rows: rows}, nil
}

func (s *xlsxSource) next() ([]string, int, error) {
	if !s.rows.Next() {
		if err := s.rows.Error(); err != nil {
			return nil, 0, fmt.Errorf("%w: %w", ErrFormat, err)
		}
		return nil, 0, io.EOF
	}
	s.line++
	cells, err := s.rows.Columns()
	if err != nil {
		return nil, 0, fmt.Errorf("%w: row %d: %w", ErrFormat, s.line, err)
	}
	return cells, s.line, nil
}

func (s *xlsxSource) close() error {
	return errors.Join(s.rows.Close(), s.book.Close())
}

func closeWorkbook(book *excelize.File) error {
	if book == nil {
		return nil
	}
	return book.Close()
}

func dropUnreadParts(book *excelize.File) {
	book.Pkg.Range(func(key, _ any) bool {
		if name, _ := key.(string); !isReadPart(partName(name)) {
			book.Pkg.Delete(key)
		}
		return true
	})
}

func inspectArchive(src io.ReaderAt, size int64) (*zip.Reader, error) {
	compound, err := isCompoundFile(src)
	if err != nil {
		return nil, err
	}
	if compound {
		return nil, classifyCompoundFile(src, size)
	}
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	if len(archive.File) > maxEntries {
		return nil, fmt.Errorf("%w: %d archive entries, limit %d", ErrTooManyEntries, len(archive.File), maxEntries)
	}
	var expanded uint64
	for _, entry := range archive.File {
		if err := inspectEntry(entry, expanded); err != nil {
			return nil, err
		}
		expanded += entry.UncompressedSize64
	}
	return archive, nil
}

func isCompoundFile(src io.ReaderAt) (bool, error) {
	var head [len(compoundFileSignature)]byte
	n, err := src.ReadAt(head[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("tabular: read file: %w", err)
	}
	return n == len(head) && head == compoundFileSignature, nil
}

func inspectEntry(entry *zip.File, expanded uint64) error {
	if entry.Flags&encryptedEntry != 0 {
		return ErrEncrypted
	}
	if isMacroPart(entry.Name) {
		return ErrMacros
	}
	if limit, tooLarge := partLimit(entry.Name); limit > 0 && entry.UncompressedSize64 > limit {
		return fmt.Errorf("%w: %s expands to %d bytes, limit %d", tooLarge, entry.Name, entry.UncompressedSize64, limit)
	}
	if entry.UncompressedSize64 > maxExpandedBytes-expanded {
		return fmt.Errorf("%w: expands past %d bytes", ErrExpandedTooLarge, maxExpandedBytes)
	}
	return nil
}

func classifyCompoundFile(src io.ReaderAt, size int64) error {
	data, err := io.ReadAll(io.NewSectionReader(src, 0, size))
	if err != nil {
		return fmt.Errorf("tabular: read file: %w", err)
	}
	if bytes.Contains(data, encryptedPackageName) {
		return ErrEncrypted
	}
	return fmt.Errorf("%w: an old Excel file; save it as an .xlsx workbook", ErrFormat)
}

func verifyParts(files []*zip.File) error {
	for _, entry := range files {
		name := partName(entry.Name)
		if !isReadPart(name) {
			continue
		}
		if err := verifyPart(entry, name); err != nil {
			return fmt.Errorf("%s: %w", entry.Name, err)
		}
	}
	return nil
}

func verifyPart(entry *zip.File, name string) error {
	body, err := entry.Open()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}
	err = scanXML(body, name == rootRelationshipsPart)
	return errors.Join(err, body.Close())
}

func scanXML(r io.Reader, rootRelationships bool) error {
	dec := xml.NewDecoder(&tagLimiter{r: r})
	dec.CharsetReader = charset.NewReaderLabel
	guard := xmlGuard{rootRelationships: rootRelationships}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFormat, err)
		}
		if err := guard.observe(tok); err != nil {
			return err
		}
	}
}

var errTagTooLong = errors.New("an element's start tag is longer than the limit")

type tagState uint8

const (
	inText tagState = iota
	afterOpen
	inTag
	afterBang
	inComment
	inCDATA
	inInstruction
)

type tagLimiter struct {
	r      io.Reader
	state  tagState
	quote  byte
	length int
	prefix []byte
	tail   [2]byte
}

func (t *tagLimiter) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	for _, c := range p[:n] {
		if t.advance(c) {
			return 0, fmt.Errorf("%w (%d bytes)", errTagTooLong, maxTagBytes)
		}
	}
	return n, err
}

func (t *tagLimiter) advance(c byte) bool {
	switch t.state {
	case inText:
		if c == '<' {
			t.state = afterOpen
		}
	case afterOpen:
		return t.open(c)
	case inTag:
		return t.inTag(c)
	case afterBang:
		return t.bang(c)
	case inComment:
		t.leaveWhen(c, '-', 2)
	case inCDATA:
		t.leaveWhen(c, ']', 2)
	case inInstruction:
		t.leaveWhen(c, '?', 1)
	}
	return false
}

func (t *tagLimiter) open(c byte) bool {
	switch c {
	case '?':
		t.state = inInstruction
	case '!':
		t.state, t.prefix = afterBang, t.prefix[:0]
	default:
		t.state, t.quote, t.length = inTag, 0, 1
		return t.inTag(c)
	}
	return false
}

func (t *tagLimiter) bang(c byte) bool {
	t.prefix = append(t.prefix, c)
	switch prefix := string(t.prefix); {
	case prefix == "--":
		t.state = inComment
	case prefix == "[CDATA[":
		t.state = inCDATA
	case strings.HasPrefix("--", prefix) || strings.HasPrefix("[CDATA[", prefix):
	default:
		t.state, t.quote, t.length = inTag, 0, 2
		for _, b := range t.prefix {
			if t.inTag(b) {
				return true
			}
		}
	}
	return false
}

func (t *tagLimiter) inTag(c byte) bool {
	t.length++
	switch {
	case t.quote != 0:
		if c == t.quote {
			t.quote = 0
		}
	case c == '"' || c == '\'':
		t.quote = c
	case c == '>':
		t.state = inText
	}
	return t.length > maxTagBytes
}

func (t *tagLimiter) leaveWhen(c, repeated byte, run int) {
	closes := c == '>' && t.tail[1] == repeated && (run == 1 || t.tail[0] == repeated)
	t.tail[0], t.tail[1] = t.tail[1], c
	if closes {
		t.state = inText
	}
}

type xmlGuard struct {
	rootRelationships bool
	depth             int
	rows              bool
	cells             int
}

func (g *xmlGuard) observe(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		return g.start(t)
	case xml.EndElement:
		g.depth--
	case xml.Directive:
		return fmt.Errorf("%w: a document type declaration", ErrFormat)
	}
	return nil
}

func (g *xmlGuard) start(el xml.StartElement) error {
	if g.depth++; g.depth > maxXMLDepth {
		return fmt.Errorf("%w: elements are nested more than %d deep", ErrFormat, maxXMLDepth)
	}
	switch el.Name.Local {
	case "row":
		g.rows, g.cells = true, 0
		return checkRowNumber(el)
	case "c":
		if g.cells++; g.rows && g.cells > maxGridColumns {
			return fmt.Errorf("%w: a row has more than %d cells", ErrTooManyColumns, maxGridColumns)
		}
	case "Relationship":
		if g.rootRelationships {
			return checkDocumentTarget(el)
		}
	}
	return nil
}

func checkRowNumber(el xml.StartElement) error {
	for _, attr := range el.Attr {
		if attr.Name.Local != "r" {
			continue
		}
		if n, err := strconv.Atoi(attr.Value); err != nil || n < 1 || n > maxGridRows {
			return fmt.Errorf("%w: row number %q is outside the sheet", ErrFormat, attr.Value)
		}
	}
	return nil
}

func checkDocumentTarget(el xml.StartElement) error {
	isDocument := slices.ContainsFunc(el.Attr, func(attr xml.Attr) bool {
		return attr.Name.Local == "Type" && strings.HasSuffix(strings.ToLower(attr.Value), "officedocument")
	})
	if !isDocument {
		return nil
	}
	for _, attr := range el.Attr {
		if attr.Name.Local == "Target" && strings.TrimPrefix(attr.Value, "/") != workbookPart {
			return fmt.Errorf("%w: the workbook is at %q", ErrLayout, attr.Value)
		}
	}
	return nil
}

func isMacroPart(name string) bool {
	name = partName(name)
	return path.Base(name) == "vbaproject.bin" || strings.HasPrefix(name, "xl/macrosheets/")
}

func isSheetPart(name string) bool {
	return strings.HasPrefix(name, "xl/worksheets/") && !strings.HasSuffix(name, ".rels")
}

func isReadPart(name string) bool {
	return namedReadParts[name] || isSheetPart(name) || (strings.HasPrefix(name, "xl/theme/") && strings.HasSuffix(name, ".xml"))
}

func partLimit(name string) (uint64, error) {
	name = partName(name)
	switch {
	case name == "xl/sharedstrings.xml":
		return maxSharedStringsBytes, ErrPartTooLarge
	case isSheetPart(name):
		return maxWorksheetBytes, ErrSheetTooLarge
	case strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels"):
		return maxXMLPartBytes, ErrPartTooLarge
	}
	return 0, nil
}

func partName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
}

func utf16LE(s string) []byte {
	var out []byte
	for _, unit := range utf16.Encode([]rune(s)) {
		out = binary.LittleEndian.AppendUint16(out, unit)
	}
	return out
}
