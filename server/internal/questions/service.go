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

// ErrMediaNotFound is a well-formed asset id that resolves to nothing.
var ErrMediaNotFound = errors.New("questions: media asset not found")

// resolveMediaKind reads the kind from the database rather than the request, so
// a caller cannot declare an image to be audio.
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
// version snapshot path.
func (s *Service) GetIncludingDeleted(ctx context.Context, id string) (Question, error) {
	return s.store.GetIncludingDeleted(ctx, id)
}

func (s *Service) List(ctx context.Context, in ListInput) ([]Question, string, error) {
	return s.store.List(ctx, in)
}

func (s *Service) Facets(ctx context.Context, in ListInput) (TypeFacets, error) {
	return s.store.Facets(ctx, in)
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

func (s *Service) AddTags(ctx context.Context, ids []string, tags []string) (int, error) {
	return s.store.AddTags(ctx, ids, tags)
}
