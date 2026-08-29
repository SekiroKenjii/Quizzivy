package media

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrBadCursor is returned for a cursor that was not produced by nextCursor.
// The contract calls the cursor opaque and says never to construct one, so a
// malformed value is a client bug, answered with a 400 rather than a 500.
var ErrBadCursor = errors.New("media: malformed cursor")

// Page-size bounds for the library listing.
const (
	DefaultLimit = 24
	MaxLimit     = 100
)

// cursor is a position in the (created_at DESC, id DESC) ordering.
//
// Both halves are needed even though `id` is a uuidv7 and therefore already
// time-ordered. The sort key the index supports is created_at, and two assets
// uploaded in the same microsecond tie on it; without the id tiebreak a tied
// pair can be served twice or skipped entirely at a page boundary. The id makes
// the ordering a strict total order, which is what keyset pagination requires
// to be correct (§13.8).
type cursor struct {
	createdAt time.Time
	id        string
}

// encodeCursor renders a position opaquely. base64url of a fixed two-field
// string: not a secret -- it names a row the caller was just shown -- but not
// something a client should be tempted to parse or assemble either.
func encodeCursor(c cursor) string {
	raw := c.createdAt.UTC().Format(time.RFC3339Nano) + "|" + c.id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor is deliberately strict. It is parsing attacker-controlled input
// that goes on to be a query parameter, so every field is validated to its exact
// type before it can reach the database.
func decodeCursor(s string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, ErrBadCursor
	}
	at, id, found := strings.Cut(string(raw), "|")
	if !found {
		return cursor{}, ErrBadCursor
	}
	createdAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return cursor{}, ErrBadCursor
	}
	if _, err := uuid.Parse(id); err != nil {
		return cursor{}, ErrBadCursor
	}
	return cursor{createdAt: createdAt, id: id}, nil
}

// ListInput selects a page of the library.
type ListInput struct {
	Kind   *Kind
	Cursor string // empty for the first page
	Limit  int    // clamped to [1, MaxLimit]
}

// List returns one page of live assets, newest first, plus the cursor for the
// next page or "" when this was the last one.
//
// Keyset rather than OFFSET (§13.8): an upload that lands mid-pagination shifts
// every subsequent OFFSET page by one, which duplicates a row for the reader.
// Keyset asks for "older than this exact row", so a concurrent insert is simply
// not on the page.
func (s *Store) List(ctx context.Context, in ListInput) ([]Asset, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	var after *cursor
	if in.Cursor != "" {
		c, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, "", err
		}
		after = &c
	}
	const q = `
		SELECT id::text, kind::text, storage_key, mime_type, bytes, duration_ms,
		       original_filename, checksum_sha256, created_at
		  FROM app.media_assets
		 WHERE deleted_at IS NULL
		   AND ($1::app.media_kind IS NULL OR kind = $1::app.media_kind)
		   AND ($2::timestamptz IS NULL
		        OR (created_at, id) < ($2::timestamptz, $3::uuid))
		 ORDER BY created_at DESC, id DESC
		 LIMIT $4`

	var kindArg *string
	if in.Kind != nil {
		k := string(*in.Kind)
		kindArg = &k
	}
	var atArg *time.Time
	var idArg *string
	if after != nil {
		atArg, idArg = &after.createdAt, &after.id
	}

	rows, err := s.pool.Query(ctx, q, kindArg, atArg, idArg, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("media: list assets: %w", err)
	}
	defer rows.Close()

	assets := make([]Asset, 0, limit)
	for rows.Next() {
		var a Asset
		var kind string
		if err := rows.Scan(&a.ID, &kind, &a.StorageKey, &a.MimeType, &a.Bytes,
			&a.DurationMs, &a.OriginalFilename, &a.ChecksumSHA256, &a.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("media: scan asset: %w", err)
		}
		a.Kind = Kind(kind)
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("media: list assets: %w", err)
	}

	if len(assets) <= limit {
		return assets, "", nil
	}
	assets = assets[:limit]
	last := assets[len(assets)-1]
	return assets, encodeCursor(cursor{createdAt: last.CreatedAt, id: last.ID}), nil
}
