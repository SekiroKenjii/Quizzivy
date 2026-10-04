package domain

import (
	"context"
	"time"
)

// Repository stores notifications and preferences. Every read and every
// write but Upsert and DeleteBefore is bounded by the user it is given: it
// never returns or changes another user's row, whatever ids it is handed.
type Repository interface {
	// Upsert writes n in one statement: a new row, or n merged into the row
	// its user holds under the same dedupe key, which becomes unread. It
	// writes nothing, and reports false, when the user switched off the event
	// that governs n's kind.
	Upsert(ctx context.Context, n Notice) (bool, error)
	// List returns up to q.Size() of q.UserID's notifications older than
	// q.Before, newest first, and where the next page starts.
	List(ctx context.Context, q ListQuery) (Page, error)
	// MarkRead marks those of ids that are userID's and unread.
	MarkRead(ctx context.Context, userID string, ids []string) error
	// MarkAllRead marks every unread notification userID has.
	MarkAllRead(ctx context.Context, userID string) error
	// Unread counts userID's unread notifications.
	Unread(ctx context.Context, userID string) (int, error)
	// Preferences returns the switches userID has saved, and only those.
	Preferences(ctx context.Context, userID string) ([]Preference, error)
	// SavePreferences stores prefs for userID and returns them as stored.
	SavePreferences(ctx context.Context, userID string, prefs []Preference) ([]Preference, error)
	// DeleteBefore deletes every notification first written before cutoff,
	// and returns how many went.
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
