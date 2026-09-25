package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/word"
)

func (p ImportProcessing) Extract(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	raw, err := word.Extract(ctx, in.Body, in.Bytes, in.Identity, word.DefaultLimits())
	if err != nil {
		return ports.StageOutput{}, extractionFailure(err)
	}
	job, err := os.MkdirTemp(p.WorkDir, "extraction-")
	if err != nil {
		return ports.StageOutput{}, err
	}
	stage := &extractionFiles{ctx: ctx, root: job, allowed: map[string]bool{}, plan: domain.ArtifactPlan{SourceID: in.OriginalID, Role: in.Role, Stage: "extraction", ComponentVersion: p.ExtractionVersion()}}
	if err := stage.write(raw, in.Role); err != nil {
		_ = stage.close()
		return ports.StageOutput{}, err
	}
	return ports.StageOutput{Plan: stage.plan, Open: stage.open, Close: stage.close}, nil
}

func extractionFailure(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, word.ErrLimit):
		return worker.Failure{Code: "SOURCE_TOO_LARGE"}
	case errors.Is(err, word.ErrActiveContent), errors.Is(err, word.ErrLegacyOrLocked):
		return worker.Failure{Code: "SOURCE_UNSUPPORTED"}
	default:
		return worker.Failure{Code: "SOURCE_INVALID"}
	}
}

type extractionFiles struct {
	ctx     context.Context
	root    string
	plan    domain.ArtifactPlan
	allowed map[string]bool
	bytes   int64
}

func (s *extractionFiles) write(raw word.Extraction, role string) error {
	if len(raw.Blocks) > 20000 {
		return domain.ErrTooLarge
	}
	manifest := domain.ExtractionManifest{Version: raw.Version, Identity: raw.SourceID, MainPart: raw.MainPart, BlockCount: len(raw.Blocks), EvidenceFile: "evidence.json", InventoryFile: "inventory.json", Chunks: []domain.BlockChunk{}}
	if err := s.add(manifest.EvidenceFile, ImportEvidence(raw, role)); err != nil {
		return err
	}
	for start := 0; start < len(raw.Blocks); start += 100 {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		end := min(start+100, len(raw.Blocks))
		name := fmt.Sprintf("blocks-%04d.json", len(manifest.Chunks)+1)
		if err := s.add(name, raw.Blocks[start:end]); err != nil {
			return err
		}
		manifest.Chunks = append(manifest.Chunks, domain.BlockChunk{Name: name, First: start, Count: end - start})
	}
	raw.Blocks = nil
	if err := s.add(manifest.InventoryFile, raw); err != nil {
		return err
	}
	encoded, err := json.Marshal(manifest)
	s.plan.Manifest = encoded
	return err
}

func (s *extractionFiles) add(name string, value any) error {
	f, err := os.OpenFile(filepath.Join(s.root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	sink := &artifactSink{ctx: s.ctx, writer: io.MultiWriter(f, hash)}
	writeErr := json.NewEncoder(sink).Encode(value)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	s.bytes += sink.bytes
	if s.bytes > domain.MaxArtifactSetBytes {
		return domain.ErrTooLarge
	}
	s.plan.Files = append(s.plan.Files, domain.ArtifactSpec{Name: name, Kind: "source_blocks", ContentType: "application/json", Bytes: sink.bytes, SHA256: hash.Sum(nil)})
	s.allowed[name] = true
	return nil
}

func (s *extractionFiles) open(name string) (io.ReadSeekCloser, error) {
	if !s.allowed[name] {
		return nil, domain.ErrNotFound
	}
	return os.Open(filepath.Join(s.root, name))
}
func (s *extractionFiles) close() error { return os.RemoveAll(s.root) }

type artifactSink struct {
	ctx    context.Context
	writer io.Writer
	bytes  int64
}

func (w *artifactSink) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(data)) > domain.MaxArtifactBytes-w.bytes {
		return 0, domain.ErrTooLarge
	}
	n, err := w.writer.Write(data)
	w.bytes += int64(n)
	return n, err
}
