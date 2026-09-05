package http

import (
	"context"
	"time"

	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the media application this transport needs; nil when object storage is off.
type Service interface {
	Upload(ctx context.Context, in application.UploadInput) (domain.Asset, error)
	SignedURL(ctx context.Context, asset domain.Asset) (string, error)
	List(ctx context.Context, in domain.ListInput) ([]domain.Asset, paging.Page, error)
	TotalBytes(ctx context.Context, kind *domain.Kind) (int64, error)
	Delete(ctx context.Context, in domain.DeleteInput) error
	MintForStudent(ctx context.Context, studentID, assetID string) (application.SignedURLResult, error)
	Get(ctx context.Context, id string) (domain.Asset, error)
	SignedURLTTL() time.Duration
}

type Media struct {
	media Service
}

func NewMedia(media Service) Media {
	return Media{media: media}
}
