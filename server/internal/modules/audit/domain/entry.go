package domain

import "time"

// Actor is the account an entry was done by, as the log's join to users reads
// it.
type Actor struct {
	ID        string
	Name      string
	AvatarKey string
}

// EntityRef names the row an entry is about.
type EntityRef struct {
	Type string
	ID   string
}

// Record is a row of the log as it is stored.
type Record struct {
	ID         int64
	OccurredAt time.Time
	// Actor is nil for the system, and for an account since removed.
	Actor      *Actor
	Action     string
	EntityType string
	// EntityID is nil for an entry about no single row.
	EntityID  *string
	IP        *string
	UserAgent *string
	// Diff is the JSON the entry was written with, nil when it had none.
	Diff []byte
}

// Ref is the row the record is about, and false when it is about none.
func (r Record) Ref() (EntityRef, bool) {
	if r.EntityID == nil {
		return EntityRef{}, false
	}
	return EntityRef{Type: r.EntityType, ID: *r.EntityID}, true
}

// Entry is a Record read for a screen or a file: its category, the name of the
// thing it is about, the actor's photo and the diff shown without a secret.
type Entry struct {
	Record
	// Category is nil for an entry only the unfiltered list holds.
	Category *Category
	// Label is nil when the row is gone or has no name.
	Label *string
	// AvatarURL is empty when the actor has no photo.
	AvatarURL string
	Presentation
}

// NewEntry reads rec. label and avatarURL are what the caller resolved for it.
func NewEntry(rec Record, label *string, avatarURL string) Entry {
	entry := Entry{Record: rec, Label: label, AvatarURL: avatarURL, Presentation: Present(rec.Diff)}
	if category, ok := CategoryOf(rec.EntityType); ok {
		entry.Category = &category
	}
	return entry
}
