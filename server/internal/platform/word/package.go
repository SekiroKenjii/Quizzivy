// Package word reads bounded, untrusted Word packages into source evidence; it
// does not infer assessment structure or answer keys.
package word

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strings"
)

func declaredEntries(src io.ReaderAt, size int64) (uint64, bool) {
	const eocdLength, zip64LocatorLength = 22, 20
	window := min(size, int64(math.MaxUint16+eocdLength))
	tail := make([]byte, window)
	if _, err := src.ReadAt(tail, size-window); err != nil && !errors.Is(err, io.EOF) {
		return 0, false
	}
	at := bytes.LastIndex(tail, []byte{0x50, 0x4b, 0x05, 0x06})
	if at < 0 || len(tail)-at < eocdLength {
		return 0, false
	}
	entries := uint64(binary.LittleEndian.Uint16(tail[at+10:]))
	if entries != math.MaxUint16 || at < zip64LocatorLength {
		return entries, true
	}
	locator := tail[at-zip64LocatorLength : at]
	if !bytes.Equal(locator[:4], []byte{0x50, 0x4b, 0x06, 0x07}) {
		return entries, true
	}
	record := make([]byte, 56)
	offset := int64(binary.LittleEndian.Uint64(locator[8:]))
	if offset < 0 || offset > size-int64(len(record)) {
		return 0, false
	}
	if _, err := src.ReadAt(record, offset); err != nil || !bytes.Equal(record[:4], []byte{0x50, 0x4b, 0x06, 0x06}) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(record[32:]), true
}

// Limits bounds archive expansion and the total XML work performed by Inspect.
type Limits struct {
	CompressedBytes int64
	ExpandedBytes   uint64
	Entries         int
	XMLBytes        int64
	XMLTotalBytes   int64
	LocatorBytes    int64
	XMLNodes        int
	XMLDepth        int
}

// DefaultLimits returns initial inspection limits, pending the real-file capacity benchmark.
func DefaultLimits() Limits {
	return Limits{CompressedBytes: 25 << 20, ExpandedBytes: 200 << 20, Entries: 4096, XMLBytes: 16 << 20, XMLTotalBytes: 32 << 20, LocatorBytes: 32 << 20, XMLNodes: 200000, XMLDepth: 64}
}

var (
	// ErrLimit identifies a configured resource bound exceeded during inspection.
	ErrLimit = errors.New("word: processing limit exceeded")
	// ErrInvalidPackage identifies a malformed or inconsistent Word package.
	ErrInvalidPackage = errors.New("word: invalid document package")
	// ErrActiveContent identifies executable or embedded active content.
	ErrActiveContent = errors.New("word: active content is not supported")
	// ErrLegacyOrLocked identifies an input requiring legacy conversion or decryption.
	ErrLegacyOrLocked = errors.New("word: legacy or encrypted document requires normalization")
)

type archive struct {
	files map[string]*zip.File
	order []string
	limit Limits
}

func openArchive(ctx context.Context, src io.ReaderAt, size int64, limits Limits) (*archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limits.CompressedBytes < 1 || limits.ExpandedBytes < 1 || limits.Entries < 1 || limits.XMLBytes < 1 || limits.XMLBytes == math.MaxInt64 || limits.XMLTotalBytes < 1 || limits.LocatorBytes < 1 || limits.XMLNodes < 1 || limits.XMLDepth < 1 {
		return nil, fmt.Errorf("%w: nonpositive configuration", ErrLimit)
	}
	if size < 1 || size > limits.CompressedBytes {
		return nil, fmt.Errorf("%w: compressed bytes", ErrLimit)
	}
	var signature [8]byte
	_, _ = src.ReadAt(signature[:], 0)
	if signature == [8]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1} {
		return nil, ErrLegacyOrLocked
	}
	if declared, ok := declaredEntries(src, size); ok && declared > uint64(limits.Entries) {
		return nil, fmt.Errorf("%w: archive entries", ErrLimit)
	}
	zr, err := zip.NewReader(src, size)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read ZIP", ErrInvalidPackage)
	}
	if len(zr.File) > limits.Entries {
		return nil, fmt.Errorf("%w: archive entries", ErrLimit)
	}
	a := &archive{files: make(map[string]*zip.File), limit: limits}
	var expanded uint64
	for _, file := range zr.File {
		if err := validateEntry(file, a.files); err != nil {
			return nil, err
		}
		if file.UncompressedSize64 > limits.ExpandedBytes-expanded {
			return nil, fmt.Errorf("%w: expanded bytes", ErrLimit)
		}
		expanded += file.UncompressedSize64
		a.files[file.Name] = file
		a.order = append(a.order, file.Name)
	}
	return a, nil
}

func validateEntry(file *zip.File, files map[string]*zip.File) error {
	name := file.Name
	if !safePartName(strings.TrimSuffix(name, "/")) || !file.Mode().IsRegular() && !file.FileInfo().IsDir() {
		return fmt.Errorf("%w: unsafe archive entry", ErrInvalidPackage)
	}
	if _, exists := files[name]; exists {
		return fmt.Errorf("%w: duplicate archive entry", ErrInvalidPackage)
	}
	if file.Flags&1 != 0 {
		return ErrLegacyOrLocked
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "vbaproject") || strings.Contains(lower, "/embeddings/") || strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".dll") {
		return ErrActiveContent
	}
	return nil
}

func safePartName(name string) bool {
	return name != "" && name != "." && name != ".." && path.Clean(name) == name && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\:\x00")
}

func (a *archive) readXML(ctx context.Context, name string, budget *xmlBudget) (*element, error) {
	file, ok := a.files[name]
	if !ok {
		return nil, fmt.Errorf("%w: required part absent", ErrInvalidPackage)
	}
	if file.UncompressedSize64 > uint64(a.limit.XMLBytes) || file.UncompressedSize64 > uint64(budget.bytes) {
		return nil, fmt.Errorf("%w: XML part bytes", ErrLimit)
	}
	r, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open part", ErrInvalidPackage)
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: r}, a.limit.XMLBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPackage, err)
	}
	if int64(len(data)) > a.limit.XMLBytes {
		return nil, fmt.Errorf("%w: XML part bytes", ErrLimit)
	}
	budget.bytes -= int64(len(data))
	if budget.bytes < 0 {
		return nil, fmt.Errorf("%w: total XML bytes", ErrLimit)
	}
	return parseXML(ctx, data, budget, a.limit.XMLDepth)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
