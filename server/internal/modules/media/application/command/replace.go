package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"time"
)

// Replace stores a same-kind file and repoints editable references through a pool-owned transaction.
type Replace struct {
	ID         string
	Scope      access.Scope
	Filename   string
	Body       io.Reader
	UploaderID string
	IP         string
	UserAgent  string
}
type ReplaceHandler struct{ *support.Service }

func (s ReplaceHandler) Handle(ctx context.Context, cmd Replace) (domain.ReplaceResult, error) {
	target, err := s.Repo.FindReplacementTarget(ctx, cmd.Scope, cmd.ID)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	intake, err := s.Receive(cmd.Body)
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	defer intake.Close()
	if intake.Kind != target.Asset.Kind {
		return domain.ReplaceResult{}, domain.ErrKindMismatch
	}
	id, err := support.NewAssetID()
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	key := fmt.Sprintf("%s/%s%s", intake.Kind, id, support.ExtensionFor(intake.MimeType))
	body, err := intake.Body()
	if err != nil {
		return domain.ReplaceResult{}, err
	}
	if err := s.Object.Put(ctx, key, intake.MimeType, body, intake.Bytes); err != nil {
		return domain.ReplaceResult{}, s.compensate(ctx, key, err)
	}
	in := domain.ReplaceInput{ID: cmd.ID, Scope: cmd.Scope, Asset: domain.InsertInput{ID: id, Kind: intake.Kind, StorageKey: key, MimeType: intake.MimeType, Bytes: intake.Bytes, DurationMs: intake.DurationMs, OriginalFilename: support.SanitiseFilename(cmd.Filename), ChecksumSHA256: intake.Checksum, UploaderID: cmd.UploaderID, OwnerID: target.OwnerID, DisplayName: &target.Asset.DisplayName, DefaultMaxPlays: target.Asset.DefaultMaxPlays, Width: intake.Width, Height: intake.Height, QuotaBytes: s.Quota, Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent)}}
	result, err := s.Repo.Replace(ctx, in)
	if err != nil {
		return domain.ReplaceResult{}, s.replacementFailure(ctx, key, err)
	}
	assets := []domain.Asset{result.Asset}
	if err := s.Describe(ctx, assets); err != nil {
		return domain.ReplaceResult{}, &domain.ReplacementError{Outcome: domain.ReplacementCommitted, Cause: err}
	}
	result.Asset = assets[0]
	return result, nil
}
func (s ReplaceHandler) replacementFailure(ctx context.Context, key string, err error) error {
	var failure *domain.ReplacementError
	if !errors.As(err, &failure) {
		return &domain.ReplacementError{Outcome: domain.ReplacementUnknown, Cause: err}
	}
	if failure.Outcome != domain.ReplacementNotCommitted {
		return err
	}
	return s.compensate(ctx, key, err)
}
func (s ReplaceHandler) compensate(ctx context.Context, key string, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Object.Delete(cleanupCtx, key); err != nil {
		return &domain.ReplacementCleanupError{Cause: cause, Cleanup: err}
	}
	return cause
}
