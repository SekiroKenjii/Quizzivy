package command

import (
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
type UploadHandler struct {
	Repo      domain.Repository
	Store     ports.ObjectStore
	Inspector ports.Inspector
	Quotas    domain.Quotas
	WorkDir   string
	Slots     chan struct{}
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
	if !validFilename(in.Filename) {
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
	if err := h.Inspector.Inspect(ctx, file, n); err != nil {
		return domain.Receipt{}, err
	}
	source, err := h.Repo.Reserve(ctx, domain.Reserve{Actor: in.Actor, Source: domain.Source{ImportID: in.ImportID, UploadID: in.UploadID, ExpectedRevision: in.ExpectedRevision, Role: in.Role, Filename: in.Filename, Format: "docx", Bytes: n, SHA256: checksum}}, h.Quotas)
	if err != nil {
		return domain.Receipt{}, err
	}
	if !source.Ready {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return domain.Receipt{}, errors.New("imports: seek staging file")
		}
		if err := h.Store.Put(ctx, source.StorageKey, "application/octet-stream", file, n); err != nil {
			return domain.Receipt{}, err
		}
	}
	return h.Repo.Finish(ctx, domain.Finish{ImportID: in.ImportID, SourceID: source.ID, Actor: in.Actor})
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

func validFilename(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= 255 && strings.HasSuffix(strings.ToLower(name), ".docx") && !strings.ContainsAny(name, "/\\") && strings.IndexFunc(name, unicode.IsControl) < 0
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
