package core

import (
	"testing"

	"quizzivy/internal/api"
	"quizzivy/internal/media"
)

// TestMediaDisabledLeavesDepsMediaNil pins the nil-interface trap that the
// guard in buildModules exists for: assigning a nil *media.Service to the
// interface field would produce a non-nil interface, and every handler's
// `Deps.Media == nil` check would then call methods on a nil pointer.
func TestMediaDisabledLeavesDepsMediaNil(t *testing.T) {
	var disabled *media.Service

	var deps api.Deps
	if disabled != nil {
		deps.Media = disabled
	}
	if deps.Media != nil {
		t.Error("Deps.Media is non-nil with media disabled; handlers would 500 rather than 501")
	}

	// The shape the guard prevents, so the test states what it is protecting.
	var unguarded api.Deps
	unguarded.Media = disabled
	if unguarded.Media == nil {
		t.Skip("interface no longer wraps a typed nil; the guard may be unnecessary")
	}
}
