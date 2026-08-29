package questions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Service validates a question against the rules a schema cannot express, then
// writes it.
type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service {
	return &Service{store: store, now: time.Now}
}

// ErrMediaNotFound is a question pointing at an asset that is not there.
// Separate from a validation failure because the field is well-formed -- the
// asset simply does not exist, or has been deleted since the picker listed it.
var ErrMediaNotFound = errors.New("questions: media asset not found")

// resolveMediaKind reads the asset's kind from the database.
//
// The kind is NEVER taken from the request. It is half of the composite FK
// [D-05], and the whole point of that FK is that "audio policy implies an audio
// asset" is enforced relationally rather than on the client's word. Accepting a
// caller-supplied kind would put the lie back in.
func (s *Service) resolveMediaKind(ctx context.Context, assetID *string) (*string, error) {
	if assetID == nil {
		return nil, nil
	}
	var kind string
	err := s.store.pool.QueryRow(ctx,
		`SELECT kind::text FROM app.media_assets WHERE id = $1 AND deleted_at IS NULL`,
		*assetID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMediaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("questions: resolve media kind: %w", err)
	}
	return &kind, nil
}

type WriteRequest struct {
	ID        string // empty to create
	Input     Input
	ActorID   string
	IP        string
	UserAgent string
}

func (s *Service) Create(ctx context.Context, req WriteRequest) (Question, error) {
	return s.write(ctx, req, false)
}

func (s *Service) Update(ctx context.Context, req WriteRequest) (Question, error) {
	if req.ID == "" {
		return Question{}, ErrNotFound
	}
	return s.write(ctx, req, true)
}

func (s *Service) write(ctx context.Context, req WriteRequest, update bool) (Question, error) {
	kind, err := s.resolveMediaKind(ctx, req.Input.MediaAssetID)
	if err != nil {
		return Question{}, err
	}
	// Validated against the RESOLVED kind, so "audio policy iff audio asset"
	// is decided by what the asset is rather than by what the request says.
	if err := req.Input.Validate(kind); err != nil {
		return Question{}, err
	}

	in := WriteInput{
		ID:             req.ID,
		Input:          req.Input,
		MediaAssetKind: kind,
		ActorID:        req.ActorID,
		Now:            s.now(),
		IP:             req.IP,
		UserAgent:      req.UserAgent,
	}
	if update {
		return s.store.Update(ctx, in)
	}
	return s.store.Create(ctx, in)
}

func (s *Service) Get(ctx context.Context, id string) (Question, error) {
	return s.store.Get(ctx, id)
}

// GetIncludingDeleted resolves a question whether or not it is deleted, for the
// version snapshot path (§13.2).
func (s *Service) GetIncludingDeleted(ctx context.Context, id string) (Question, error) {
	return s.store.GetIncludingDeleted(ctx, id)
}

func (s *Service) List(ctx context.Context, in ListInput) ([]Question, string, error) {
	return s.store.List(ctx, in)
}

func (s *Service) Delete(ctx context.Context, req WriteRequest) error {
	return s.store.SoftDelete(ctx, WriteInput{
		ID:        req.ID,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}
