package wordconvert

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"quizzivy/internal/platform/word"
)

// Manifest records conversion lineage and the private source rendition; page locations are not mappings to individual question coordinates.
type Manifest struct {
	Version      string   `json:"version"`
	Renderer     string   `json:"renderer"`
	SourceFormat string   `json:"sourceFormat"`
	Pages        []string `json:"pages"`
	Normalized   *string  `json:"normalized"`
	PDF          string   `json:"pdf"`
	Findings     []string `json:"findings"`
}

// Artifact identifies a validated private output with byte identity for durable storage.
type Artifact struct {
	Name, Kind, SHA256 string
	Bytes              int64
}

// Result owns a private disk job; Close removes its transient files after the caller has stored every required artifact.
type Result struct {
	Manifest              Manifest
	ImageID, SourceSHA256 string
	Artifacts             []Artifact
	root                  *os.Root
	job                   string
	allowed               map[string]bool
	once                  sync.Once
	closeErr              error
}

func (r *Result) Open(name string) (io.ReadCloser, error) {
	if !r.allowed[name] {
		return nil, ErrSource
	}
	return r.root.Open(name)
}

func (r *Result) Close() error {
	r.once.Do(func() {
		r.closeErr = r.root.Close()
		if err := os.RemoveAll(r.job); err != nil {
			r.closeErr = err
		}
	})
	return r.closeErr
}

var pageName = regexp.MustCompile(`^page-[0-9]{1,2}\.png$`)

func readResult(ctx context.Context, job, image, checksum, format string) (*Result, error) {
	root, err := os.OpenRoot(filepath.Join(job, "work", "output"))
	if err != nil {
		return nil, ErrConversion
	}
	r := &Result{root: root, job: job, ImageID: image, SourceSHA256: checksum, allowed: map[string]bool{}}
	if err := r.load(ctx, format); err != nil {
		_ = root.Close()
		return nil, err
	}
	return r, nil
}

func (r *Result) load(ctx context.Context, format string) error {
	f, err := r.regular("manifest.json", 16<<10)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(f, 16<<10))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&r.Manifest)
	_ = f.Close()
	if err != nil || r.Manifest.Version != "libreoffice-rendition-v1" || r.Manifest.SourceFormat != format || !strings.HasPrefix(r.Manifest.Renderer, "LibreOffice ") || len(r.Manifest.Renderer) > 128 || r.Manifest.PDF != "source.pdf" || len(r.Manifest.Pages) < 1 || len(r.Manifest.Pages) > 60 {
		return ErrConversion
	}
	if err := r.addNormalized(ctx, format); err != nil {
		return err
	}

	if err := r.addArtifact(ctx, "source.pdf", "rendition_pdf", 64<<20); err != nil {
		return err
	}
	for index, page := range r.Manifest.Pages {
		expected := fmt.Sprintf("page-%0*d.png", len(strconv.Itoa(len(r.Manifest.Pages))), index+1)
		if !pageName.MatchString(page) || page != expected || r.allowed[page] {
			return ErrConversion
		}
		if err := r.addArtifact(ctx, page, "rendition_page", 16<<20); err != nil {
			return err
		}
	}
	var total int64
	for _, a := range r.Artifacts {
		total += a.Bytes
	}
	if total > 128<<20 {
		return ErrLimit
	}
	return nil
}

func (r *Result) regular(name string, maxBytes int64) (*os.File, error) {
	info, err := r.root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxBytes {
		return nil, ErrConversion
	}
	f, err := r.root.Open(name)
	if err != nil {
		return nil, ErrConversion
	}
	return f, nil
}

func (r *Result) addArtifact(ctx context.Context, name, kind string, maxBytes int64) error {
	f, err := r.regular(name, maxBytes)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := validateArtifact(ctx, f, kind, info.Size()); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	count, err := io.Copy(hash, cancelReader{ctx: ctx, r: f})
	if err != nil {
		return err
	}
	r.Artifacts = append(r.Artifacts, Artifact{Name: name, Kind: kind, SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Bytes: count})
	r.allowed[name] = true
	return nil
}

func validateArtifact(ctx context.Context, f *os.File, kind string, size int64) error {
	switch kind {
	case "normalized":
		_, err := word.Inspect(ctx, f, size, word.DefaultLimits())
		return err
	case "rendition_page":
		config, err := png.DecodeConfig(cancelReader{ctx: ctx, r: f})
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1600 || config.Height > 1600 {
			return ErrConversion
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err = png.Decode(cancelReader{ctx: ctx, r: f})
		return err
	case "rendition_pdf":
		var header [5]byte
		if _, err := f.ReadAt(header[:], 0); err != nil || string(header[:]) != "%PDF-" {
			return ErrConversion
		}
	}
	return ctx.Err()
}

func (r *Result) addNormalized(ctx context.Context, format string) error {
	if format == legacyFormat {
		if r.Manifest.Normalized == nil || *r.Manifest.Normalized != "normalized.docx" {
			return ErrConversion
		}
		if err := r.addArtifact(ctx, "normalized.docx", "normalized", 25<<20); err != nil {
			return err
		}
	} else if r.Manifest.Normalized != nil {
		return ErrConversion
	}
	return nil
}
