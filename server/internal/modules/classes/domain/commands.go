package domain

import (
	"errors"
	"quizzivy/internal/shared/access"
	"strings"
	"time"
)

// ListInput selects a page of the classes Scope reaches: the caller's own, or
// every teacher's under scope.all; a zero Scope matches nothing. Query matches
// the name, accent-folded on both sides like every other search here (D-11).
type ListInput struct {
	Query string
	Page  int
	Limit int
	// One of active (the default), joinable, archived, all.
	Status string
	Scope  access.Scope
}

// MembersInput selects a page of one class's roster. Query matches name or
// email.
type MembersInput struct {
	Query string
	Page  int
	Limit int
}

// UpdateInput carries only the fields the caller actually sent, so a PATCH that
// renames a class cannot silently clear its description.
type UpdateInput struct {
	Name *string
	// nil means "the caller did not send it".
	Description     *string
	ScheduleLabel   TextPatch
	Room            TextPatch
	SelfJoinEnabled *bool
}

// TextPatch is an edit of an optional text: Set is false when the caller did
// not send it, and a nil Value clears it.
type TextPatch struct {
	Set   bool
	Value *string
}

// LabelOf is a schedule label or a room as it is stored: trimmed, and none at
// all when nothing is left, so the columns' checks never meet an empty string.
func LabelOf(raw *string) *string {
	if raw == nil {
		return nil
	}
	label := strings.TrimSpace(*raw)
	if label == "" {
		return nil
	}
	return &label
}

type CreateInput struct {
	Name            string
	Description     *string
	ScheduleLabel   *string
	Room            *string
	SelfJoinEnabled bool
	ActorUserID     string
	Now             time.Time
	IP              *string
	UserAgent       *string
}

// ArchiveInput archives or restores a class ActorUserID teaches, or any class
// with All, the actor's scope.all; another teacher's class answers ErrNotFound.
// AddMemberInput, RemoveMemberInput, RotateInput and RevokeInput reach classes
// the same way.
type ArchiveInput struct {
	ClassID     string
	Archived    bool
	ActorUserID string
	All         bool
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type AddMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	All         bool
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type RemoveMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	All         bool
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// RotateInput issues a sealed code: CodeID is chosen before the insert because
// the ciphertext is bound to it, and CodeHash is the keyed lookup hash under
// KeyID.
type RotateInput struct {
	ClassID     string
	ActorUserID string
	All         bool
	CodeID      string
	CodeHash    []byte
	Ciphertext  []byte
	KeyID       int16
	Hint        string
	ExpiresAt   time.Time
	MaxUses     *int
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type RevokeInput struct {
	ClassID     string
	ActorUserID string
	All         bool
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// RotateRequest and RevokeRequest reach a class ActorUserID teaches, or any
// class with All, the actor's scope.all.
type RotateRequest struct {
	ClassID       string
	ActorUserID   string
	All           bool
	ExpiresInDays *int
	MaxUses       *int
	IP            string
	UserAgent     string
}

// Rotated is the newly issued code with its metadata.
type Rotated struct {
	Code      string
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
}

type RevokeRequest struct {
	ClassID     string
	ActorUserID string
	All         bool
	IP          string
	UserAgent   string
}

// LegacyCodeClass is a class whose active join code is a legacy one that can
// still be redeemed. TeacherID is nil for a class that names no teacher.
type LegacyCodeClass struct {
	ClassID   string
	ClassName string
	TeacherID *string
}

// LegacyRotationInput replaces the legacy code of ClassID with a sealed one:
// CodeID is chosen before the insert because the ciphertext is bound to it,
// and CodeHash is the keyed lookup hash under KeyID. Now is the instant the
// old code must not have expired at, the instant it is revoked at and the
// instant the new one is created at.
type LegacyRotationInput struct {
	ClassID    string
	CodeID     string
	CodeHash   []byte
	Ciphertext []byte
	KeyID      int16
	Hint       string
	Now        time.Time
}

// LegacyRotation counts one run of the legacy join-code rotation: the classes
// Found holding a usable legacy code, those whose code the run Rotated, those
// that Failed and still hold theirs, the Teachers told, and the teachers whose
// notification failed, as NotifyFailed. A class another run rotated first is
// counted in Found alone.
type LegacyRotation struct {
	Found        int
	Rotated      int
	Failed       int
	Teachers     int
	NotifyFailed int
}

// ErrRotationContended is a legacy rotation the database aborted as a
// deadlock or serialization victim. Nothing was written, and the same
// rotation may be tried again.
var ErrRotationContended = errors.New("join: the legacy rotation lost a deadlock or a serialization conflict")

// Defaults from §6.1 and O-06. Expiry is the spec's; the use cap is the
// deliberate change -- §6.1 defaults to unlimited, which means a forwarded code
// works until it expires, and forwarding rather than guessing is the realistic
// threat (R-02).
const (
	DefaultExpiryDays = 30
	DefaultMaxUses    = 40
)

// DefaultZone is the calendar zone a code's last day is counted in when the
// issuer's own cannot be read.
const DefaultZone = "Asia/Ho_Chi_Minh"

// CodeExpiry is when a code issued at now stops working: 23:59:59 on the day
// that is days after today, today being the date in zone at now. The day is
// counted on the wall clock of zone, so a daylight-saving change between now
// and then moves no code to the wrong day.
func CodeExpiry(now time.Time, days int, zone *time.Location) time.Time {
	year, month, day := now.In(zone).Date()
	return time.Date(year, month, day+days, 23, 59, 59, 0, zone)
}

// ZoneOrDefault resolves the name of a calendar zone, and falls back to
// DefaultZone, and then to its fixed offset, when the name is empty or not a
// zone this server knows.
func ZoneOrDefault(name string) *time.Location {
	if name != "" && name != "Local" {
		if zone, err := time.LoadLocation(name); err == nil {
			return zone
		}
	}
	if zone, err := time.LoadLocation(DefaultZone); err == nil {
		return zone
	}
	return time.FixedZone(DefaultZone, 7*60*60)
}
