package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Upload struct {
	ImportID, UploadID, Role, Filename string
	ExpectedRevision                   int64
	Actor                              actor.Actor
	Body                               io.Reader
	End                                func() error
}

// UploadHandler bounds concurrent intake and local staging; durable reservations precede every object write.
// Legacy admits .doc files, checked only by signature here and converted in isolation by the worker.
type UploadHandler struct {
	Repo      domain.Repository
	Store     ports.ObjectStore
	Inspector ports.Inspector
	Quotas    domain.Quotas
	WorkDir   string
	Slots     chan struct{}
	Legacy    bool
}

func (h UploadHandler) Handle(ctx context.Context, in Upload) (domain.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case h.Slots <- struct{}{}:
		defer func() { <-h.Slots }()
	default:
		return domain.Receipt{}, domain.ErrBusy
	}
	format := sourceFormat(in.Filename, h.Legacy)
	if format == "" {
		return domain.Receipt{}, domain.ErrUnsupported
	}
	if _, err := h.Repo.Get(ctx, in.ImportID); err != nil {
		return domain.Receipt{}, err
	}
	file, err := os.CreateTemp(h.WorkDir, "source-*")
	if err != nil {
		return domain.Receipt{}, errors.New("imports: create staging file")
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	n, checksum, err := stageSource(ctx, in, file)
	if err != nil {
		return domain.Receipt{}, err
	}
	if err := h.inspect(ctx, file, n, format); err != nil {
		return domain.Receipt{}, err
	}
	source, err := h.Repo.Reserve(ctx, domain.Reserve{Actor: in.Actor, Source: domain.Source{ImportID: in.ImportID, UploadID: in.UploadID, ExpectedRevision: in.ExpectedRevision, Role: in.Role, Filename: in.Filename, Format: format, Bytes: n, SHA256: checksum}}, h.Quotas)
	if err != nil {
		return domain.Receipt{}, err
	}
	if source.Ready {
		return h.Repo.Finish(ctx, domain.Finish{ImportID: in.ImportID, SourceID: source.ID, Actor: in.Actor})
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return domain.Receipt{}, errors.New("imports: seek staging file")
	}
	if err := h.Store.Put(ctx, source.StorageKey, "application/octet-stream", file, n); err != nil {
		return domain.Receipt{}, err
	}
	receipt, err := h.Repo.Finish(ctx, domain.Finish{ImportID: in.ImportID, SourceID: source.ID, Actor: in.Actor})
	if errors.Is(err, domain.ErrConflict) {
		_ = h.Store.Delete(context.WithoutCancel(ctx), source.StorageKey)
	}
	return receipt, err
}

func stageSource(ctx context.Context, in Upload, file *os.File) (int64, []byte, error) {
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, in.Body}, domain.MaxSourceBytes+1))
	if n > domain.MaxSourceBytes {
		return 0, nil, domain.ErrTooLarge
	}
	if err != nil {
		return 0, nil, err
	}
	if n == 0 {
		return 0, nil, domain.ErrInvalid
	}
	if in.End != nil {
		if err := in.End(); err != nil {
			return 0, nil, err
		}
	}
	return n, hash.Sum(nil), nil
}

var oleSignature = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

func (h UploadHandler) inspect(ctx context.Context, file io.ReaderAt, size int64, format string) error {
	if format == "docx" {
		return h.Inspector.Inspect(ctx, file, size)
	}
	header := make([]byte, len(oleSignature))
	if _, err := file.ReadAt(header, 0); err != nil || !bytes.Equal(header, oleSignature) {
		return domain.ErrInvalid
	}
	return nil
}

func sourceFormat(name string, legacy bool) string {
	if name == "" || utf8.RuneCountInString(name) > 255 || strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return ""
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".docx"):
		return "docx"
	case legacy && strings.HasSuffix(lower, ".doc"):
		return "doc"
	}
	return ""
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
