package support

import (
	"container/list"
	"context"
	"sync"
	"time"

	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// PrincipalTTL is how long a resolved principal is served from memory. A write
// on this machine forgets the entry at once; any other machine sees the change
// within this long.
const PrincipalTTL = 10 * time.Second

// MaxPrincipals bounds the principal cache; the least recently used entry goes
// first.
const MaxPrincipals = 10_000

type cached struct {
	userID    string
	principal access.Principal
	expires   time.Time
}

type roleGrants struct {
	revision int64
	grants   access.Set
}

// Service resolves principals through two caches: principals by user for
// PrincipalTTL, and role grants by (role, revision), so a miss costs one user
// read plus one grants read per role revision.
type Service struct {
	repo      domain.Repository
	manager   domain.PermissionManager
	catalogue []access.Key
	max       int
	now       func() time.Time

	mu         sync.Mutex
	entries    map[string]*list.Element
	order      *list.List
	grants     map[string]roleGrants
	generation uint64
}

// NewService builds the service over the repository, expanding the Admin's
// wildcard to catalogue.
func NewService(repo domain.Repository, catalogue []access.Key) *Service {
	return &Service{
		repo:      repo,
		catalogue: catalogue,
		max:       MaxPrincipals,
		now:       time.Now,
		entries:   map[string]*list.Element{},
		order:     list.New(),
		grants:    map[string]roleGrants{},
	}
}

// SetClock replaces the time source. Tests only.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// SetLimit replaces the principal cache's bound. Tests only.
func (s *Service) SetLimit(limit int) { s.max = limit }

// Resolve returns the user's principal, from memory while it is fresh.
func (s *Service) Resolve(ctx context.Context, userID string) (access.Principal, error) {
	now := s.now()
	s.mu.Lock()
	generation := s.generation
	if el, ok := s.entries[userID]; ok {
		entry := el.Value.(*cached)
		if now.Before(entry.expires) {
			s.order.MoveToFront(el)
			s.mu.Unlock()
			return entry.principal, nil
		}
		s.order.Remove(el)
		delete(s.entries, userID)
	}
	s.mu.Unlock()

	row, err := s.repo.Principal(ctx, userID)
	if err != nil {
		return access.Principal{}, err
	}
	grants, err := s.RoleGrants(ctx, row.Role)
	if err != nil {
		return access.Principal{}, err
	}
	principal := access.Principal{
		UserID:      userID,
		RoleID:      row.Role.ID,
		BuiltinKey:  row.Role.Builtin,
		Permissions: s.manager.Effective(row.Role, grants, s.catalogue),
		Epoch:       row.Epoch,
		Disabled:    row.Disabled,
	}
	s.remember(userID, principal, now.Add(PrincipalTTL), generation)
	return principal, nil
}

// RoleGrants returns a role's stored grants, read once per revision.
func (s *Service) RoleGrants(ctx context.Context, role domain.Role) (access.Set, error) {
	s.mu.Lock()
	known, ok := s.grants[role.ID]
	s.mu.Unlock()
	if ok && known.revision == role.Revision {
		return known.grants, nil
	}
	grants, err := s.repo.Grants(ctx, role.ID)
	if err != nil {
		return access.Set{}, err
	}
	s.mu.Lock()
	if current, ok := s.grants[role.ID]; !ok || current.revision <= role.Revision {
		s.grants[role.ID] = roleGrants{revision: role.Revision, grants: grants}
	}
	s.mu.Unlock()
	return grants, nil
}

// Effective expands a role's grants into its permissions.
func (s *Service) Effective(role domain.Role, grants access.Set) access.Set {
	return s.manager.Effective(role, grants, s.catalogue)
}

// Forget drops a user's cached principal, so the next request reads it again.
// A resolve that was already reading when Forget ran does not cache what it
// read.
func (s *Service) Forget(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	if el, ok := s.entries[userID]; ok {
		s.order.Remove(el)
		delete(s.entries, userID)
	}
}

// ForgetAll drops every cached principal and every cached role's grants.
func (s *Service) ForgetAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	s.entries = map[string]*list.Element{}
	s.order.Init()
	s.grants = map[string]roleGrants{}
}

// Len returns the number of cached principals. Tests only.
func (s *Service) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.order.Len()
}

func (s *Service) remember(userID string, principal access.Principal, expires time.Time, generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation {
		return
	}
	if el, ok := s.entries[userID]; ok {
		s.order.Remove(el)
	}
	s.entries[userID] = s.order.PushFront(&cached{userID: userID, principal: principal, expires: expires})
	for s.order.Len() > s.max {
		oldest := s.order.Back()
		s.order.Remove(oldest)
		delete(s.entries, oldest.Value.(*cached).userID)
	}
}
