package support

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"quizzivy/internal/modules/organization/application/ports"
	"quizzivy/internal/modules/organization/domain"
)

// Service carries what the handlers share: the store, the snapshot, the logo
// store and checker (nil without object storage), the clock and whether this
// server signs anyone in with Google.
type Service struct {
	Repo             domain.Repository
	Snapshot         *Snapshot
	Logos            ports.LogoStore
	Checker          ports.LogoChecker
	Now              func() time.Time
	GoogleConfigured bool

	mu    sync.Mutex
	bytes map[string][]byte
}

// LogoKey is where a logo's bytes are stored.
func LogoKey(logo domain.Logo) string {
	return "branding/" + hex.EncodeToString(logo.SHA256) + "." + domain.LogoExtension
}

// LogoBytes returns logo's bytes, from memory after the first read: a logo's
// bytes never change under its key. Only the logos in use are remembered.
func (s *Service) LogoBytes(ctx context.Context, logo domain.Logo, inUse ...*domain.Logo) ([]byte, error) {
	if s.Logos == nil {
		return nil, domain.ErrStorageOff
	}
	key := LogoKey(logo)
	s.mu.Lock()
	cached, ok := s.bytes[key]
	s.mu.Unlock()
	if ok {
		return cached, nil
	}
	data, err := s.Logos.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	keep := map[string][]byte{key: data}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range inUse {
		if other == nil {
			continue
		}
		if b, ok := s.bytes[LogoKey(*other)]; ok {
			keep[LogoKey(*other)] = b
		}
	}
	s.bytes = keep
	return data, nil
}
