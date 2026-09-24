package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
)

// ArtifactBody opens one immutable processor output; each invocation must return the same complete bytes and a fresh reader.
type ArtifactBody func(string) (io.ReadSeekCloser, error)

// ArtifactWriter uploads a complete stage outside transactions, then publishes it under the live lease; failed reservations remain tracked for recovery.
type ArtifactWriter struct {
	Repo   domain.Artifacts
	Store  ports.ArtifactStore
	Quotas domain.ArtifactQuotas
}

func (w ArtifactWriter) Write(ctx context.Context, c domain.Claim, p domain.ArtifactPlan, open ArtifactBody) (domain.ArtifactSet, error) {
	set, err := w.Repo.ReserveArtifacts(ctx, c, p, w.Quotas)
	if err != nil {
		return set, err
	}
	if set.Ready {
		return set, nil
	}
	for _, f := range set.Files {
		if f.Ready {
			continue
		}
		if err := ctx.Err(); err != nil {
			return set, err
		}
		body, err := open(f.Name)
		if err != nil {
			return set, err
		}
		err = w.Store.PutImmutable(ctx, f.StorageKey, f.ContentType, body, f.Bytes, f.SHA256)
		closeErr := body.Close()
		if err != nil {
			return set, err
		}
		if closeErr != nil {
			return set, closeErr
		}
		if err := w.Repo.ArtifactStored(ctx, c, set.ID, f.ID); err != nil {
			return set, err
		}
	}
	return w.Repo.FinishArtifacts(ctx, c, set.ID)
}

// FetchPrivate stages a bounded private source or artifact on disk and verifies its immutable identity before returning it; the caller closes and removes the file.
func FetchPrivate(ctx context.Context, store ports.ArtifactStore, workDir, key string, size int64, digest []byte) (*os.File, error) {
	if size <= 0 || size > domain.MaxArtifactBytes || len(digest) != sha256.Size {
		return nil, domain.ErrInvalid
	}
	body, length, err := store.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	if length != size {
		return nil, domain.ErrInvalid
	}
	file, err := os.CreateTemp(workDir, "private-*")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(artifactReader{ctx, body}, size+1))
	if err != nil {
		return nil, err
	}
	if n != size || !bytes.Equal(hash.Sum(nil), digest) {
		return nil, errors.New("imports: private object identity mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return file, nil
}

type artifactReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r artifactReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
