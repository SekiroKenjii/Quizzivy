package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// MediaLocks is what the media module does inside a question write for the
// media its prose shows (DG-109). LockForVersionUse takes the asset's row lock
// and answers domain.ErrMediaNotFound for a missing, deleted or purged asset.
// ReadableKinds is the media module's one definition of what a scope may read,
// with each readable asset's kind, keyed by its canonical id; it runs after
// the locks, so a write never authorizes itself.
type MediaLocks interface {
	LockForVersionUse(ctx context.Context, tx pgx.Tx, assetID string) error
	ReadableKinds(ctx context.Context, tx pgx.Tx, scope access.Scope, assetIDs []string) (map[string]string, error)
}

// WithMedia supplies the media locks a write needs once its prose shows media.
func (s *Postgres) WithMedia(media MediaLocks) *Postgres {
	s.media = media
	return s
}

// groupAuthorized reads every live asset: a group member's media were
// authorized against the group's scope by the group write, under the same
// locks, before the member is written.
var groupAuthorized = access.Scope{All: true}

// lockProseMedia checks the media a write's prompt and explanation show, as the
// question-media attach path checks its asset (D-064 ruling 5), in the write's
// transaction: each asset is locked in id order, then the writer must be able
// to read it, and its kind must be its block's. Another teacher's asset is
// refused exactly as a missing one is, and the field a refusal names comes from
// the references in field order, never from which lock failed. It runs on the prose the write will
// store, after an omitted document has been preserved.
func (s *Postgres) lockProseMedia(ctx context.Context, tx pgx.Tx, in domain.WriteInput, groupMember bool) error {
	refs, err := content.QuestionMedia(in.Input.PromptContent, in.Input.ExplanationContent)
	if err != nil || len(refs) == 0 {
		return err
	}
	if s.media == nil {
		return fmt.Errorf("questions: media locks unavailable")
	}
	ids := proseAssetIDs(refs)
	var gone []string
	for _, id := range ids {
		err := s.media.LockForVersionUse(ctx, tx, id)
		if errors.Is(err, domain.ErrMediaNotFound) {
			gone = append(gone, strings.ToLower(id))
			continue
		}
		if err != nil {
			return fmt.Errorf("questions: lock media %s: %w", id, err)
		}
	}
	scope := access.Scope{UserID: in.ActorID, All: in.All}
	if groupMember {
		scope = groupAuthorized
	}
	kinds, err := s.media.ReadableKinds(ctx, tx, scope, ids)
	if err != nil {
		return err
	}
	for _, id := range gone {
		delete(kinds, id)
	}
	return usableKinds(refs, kinds)
}

// proseAssetIDs is the distinct assets refs name, in id order, the order every
// writer locks them in.
func proseAssetIDs(refs []content.FieldAsset) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// usableKinds refuses, in field order, the first reference to an asset that is
// missing or the writer may not read, alike, or whose kind is not its block's.
// Naming the field from this walk alone, never from the lock order, keeps a
// refusal from telling which of two ids exists (QC-R6M-1).
func usableKinds(refs []content.FieldAsset, kinds map[string]string) error {
	for _, ref := range refs {
		kind, readable := kinds[strings.ToLower(ref.ID)]
		switch {
		case !readable:
			return &domain.ProseMediaError{Field: fieldName(ref.Field)}
		case kind != ref.Kind:
			return &domain.ProseMediaError{Field: fieldName(ref.Field), WrongKind: true}
		}
	}
	return nil
}

func fieldName(field string) string {
	if field == content.ExplanationField {
		return "explanationContent"
	}
	return "promptContent"
}
