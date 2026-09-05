package application

import (
	"context"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/paging"
	"time"
)

// Service validates a question against the rules a schema cannot express, then
// writes it.
// MediaKinds answers what kind of asset an id names, or ErrMediaNotFound; the media module provides it.
type MediaKinds interface {
	Kind(ctx context.Context, assetID string) (string, error)
}

type Service struct {
	repo  domain.Repository
	media MediaKinds
	now   func() time.Time
}

func NewService(repo domain.Repository, media MediaKinds) *Service {
	return &Service{repo: repo, media: media, now: time.Now}
}

func (s *Service) resolveMediaKind(ctx context.Context, assetID *string) (*string, error) {
	if assetID == nil {
		return nil, nil
	}
	if s.media == nil {
		return nil, domain.ErrMediaNotFound
	}
	kind, err := s.media.Kind(ctx, *assetID)
	if err != nil {
		return nil, err
	}
	return &kind, nil
}

func (s *Service) Create(ctx context.Context, req domain.WriteRequest) (domain.Question, error) {
	return s.write(ctx, req, false)
}

func (s *Service) Update(ctx context.Context, req domain.WriteRequest) (domain.Question, error) {
	if req.ID == "" {
		return domain.Question{}, domain.ErrNotFound
	}
	return s.write(ctx, req, true)
}

func (s *Service) write(ctx context.Context, req domain.WriteRequest, update bool) (domain.Question, error) {
	kind, err := s.resolveMediaKind(ctx, req.Input.MediaAssetID)
	if err != nil {
		return domain.Question{}, err
	}
	if err := req.Input.Validate(kind); err != nil {
		return domain.Question{}, err
	}

	in := domain.WriteInput{
		ID:             req.ID,
		Input:          req.Input,
		MediaAssetKind: kind,
		ActorID:        req.ActorID,
		Now:            s.now(),
		IP:             req.IP,
		UserAgent:      req.UserAgent,
	}
	if update {
		return s.repo.Update(ctx, in)
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Question, error) {
	return s.repo.Get(ctx, id)
}

// Duplicate is A-06a's "Nhân bản": the same question again as a new bank row
// -- options, blanks, media and tags copied, ids fresh -- that no test holds
// yet. It goes through the same write as a create, so it is validated and
// audited like one.
func (s *Service) Duplicate(ctx context.Context, req domain.WriteRequest) (domain.Question, error) {
	source, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return domain.Question{}, err
	}
	req.ID = ""
	req.Input = inputOf(source)
	return s.write(ctx, req, false)
}

func inputOf(q domain.Question) domain.Input {
	in := domain.Input{
		Type:         q.Type,
		Prompt:       q.Prompt,
		MediaAssetID: q.MediaAssetID,
		Audio:        q.Audio,
		Transcript:   q.Transcript,
		Points:       q.Points,
		Explanation:  q.Explanation,
		SampleAnswer: q.SampleAnswer,
		Tags:         append([]string{}, q.Tags...),
	}
	for _, o := range q.Options {
		in.Options = append(in.Options, domain.OptionInput{Text: o.Text, IsCorrect: o.IsCorrect})
	}
	for _, b := range q.Blanks {
		in.Blanks = append(in.Blanks, domain.BlankInput{
			Ordinal: b.Ordinal, AcceptedAnswers: append([]string{}, b.AcceptedAnswers...), CaseSensitive: b.CaseSensitive,
		})
	}
	return in
}

// GetIncludingDeleted resolves a question whether or not it is deleted, for the
// version snapshot path.
func (s *Service) GetIncludingDeleted(ctx context.Context, id string) (domain.Question, error) {
	return s.repo.GetIncludingDeleted(ctx, id)
}

func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Question, paging.Page, error) {
	return s.repo.List(ctx, in)
}

func (s *Service) Facets(ctx context.Context, in domain.ListInput) (domain.TypeFacets, error) {
	return s.repo.Facets(ctx, in)
}

func (s *Service) Tags(ctx context.Context, in domain.ListInput) ([]string, error) {
	return s.repo.Tags(ctx, in)
}

func (s *Service) Counts(ctx context.Context, in domain.ListInput) (int, int, error) {
	return s.repo.Counts(ctx, in)
}

func (s *Service) Delete(ctx context.Context, req domain.WriteRequest) error {
	return s.repo.SoftDelete(ctx, domain.WriteInput{
		ID:        req.ID,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}

func (s *Service) AddTags(ctx context.Context, ids []string, tags []string) (int, error) {
	return s.repo.AddTags(ctx, ids, tags)
}
