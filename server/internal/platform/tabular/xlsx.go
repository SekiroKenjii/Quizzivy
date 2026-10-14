package tabular

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

const (
	maxEntries            = 1000
	maxExpandedBytes      = 32 << 20
	unzipXMLBytes         = 16 << 20
	maxXMLPartBytes       = 1 << 20
	maxSharedStringsBytes = 2 << 20
	encryptedEntry        = 0x1
)

var (
	compoundFileSignature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	encryptedPackageName  = utf16LE("EncryptedPackage")
)

type xlsxSource struct {
	book *excelize.File
	rows *excelize.Rows
	line int
}

func openXLSX(src io.ReaderAt, size int64) (source, error) {
	if err := inspectArchive(src, size); err != nil {
		return nil, err
	}
	book, err := excelize.OpenReader(io.NewSectionReader(src, 0, size), excelize.Options{
		UnzipSizeLimit:    maxExpandedBytes,
		UnzipXMLSizeLimit: unzipXMLBytes,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrFormat, err), closeWorkbook(book))
	}
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

func inspectArchive(src io.ReaderAt, size int64) error {
	var head [len(compoundFileSignature)]byte
	n, err := src.ReadAt(head[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("tabular: read file: %w", err)
	}
	if n == len(head) && head == compoundFileSignature {
		return classifyCompoundFile(src, size)
	}
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}
	if len(archive.File) > maxEntries {
		return fmt.Errorf("%w: %d archive entries, limit %d", ErrTooLarge, len(archive.File), maxEntries)
	}
	var expanded uint64
	for _, entry := range archive.File {
		if entry.Flags&encryptedEntry != 0 {
			return ErrEncrypted
		}
		if isMacroPart(entry.Name) {
			return ErrMacros
		}
		if limit := partLimit(entry.Name); limit > 0 && entry.UncompressedSize64 > limit {
			return fmt.Errorf("%w: %s expands to %d bytes, limit %d", ErrTooLarge, entry.Name, entry.UncompressedSize64, limit)
		}
		if entry.UncompressedSize64 > maxExpandedBytes-expanded {
			return fmt.Errorf("%w: expands past %d bytes", ErrTooLarge, maxExpandedBytes)
		}
		expanded += entry.UncompressedSize64
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

func isMacroPart(name string) bool {
	name = partName(name)
	return path.Base(name) == "vbaproject.bin" || strings.HasPrefix(name, "xl/macrosheets/")
}

func partLimit(name string) uint64 {
	name = partName(name)
	switch {
	case name == "xl/sharedstrings.xml":
		return maxSharedStringsBytes
	case strings.HasPrefix(name, "xl/worksheets/"):
		return 0
	case strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels"):
		return maxXMLPartBytes
	}
	return 0
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
