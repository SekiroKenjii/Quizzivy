// Package content models bounded, versioned learner content independently of editors and persistence.
package content

import (
	"bytes"
	"errors"
	"slices"
)

// MaxBytes and the other limits bound a single document before recursive semantic validation.
const (
	MaxBytes   = 1 << 20
	MaxNodes   = 2048
	MaxDepth   = 16
	MaxValues  = MaxNodes * 12
	MaxStrings = 200000
	MaxText    = 100000
	MaxURL     = 2000
	MaxRows    = 50
	MaxColumns = 12
)

// MaxMediaBlocks bounds the images and audio blocks of one question prompt or explanation together (D-064).
const MaxMediaBlocks = 8

const semanticFormat = "semantic_v1"

// ErrInvalidDocument rejects invalid or over-budget content without exposing its text.
var ErrInvalidDocument = errors.New("invalid content document")

// AssetReference identifies a required asset and kind; it grants no access or existence guarantee.
type AssetReference struct {
	ID   string
	Kind string
}

// Document is validated content with immutable JSON, search text and ordered references; its zero value is invalid.
type Document struct {
	data        []byte
	format      string
	plainText   string
	gaps        []string
	assets      []AssetReference
	profile     profile
	placesAudio bool
}

// Parse validates untrusted JSON, including duplicate keys and Unicode, without fetching links or assets.
func Parse(raw []byte) (Document, error) {
	return parse(raw, materialProfile)
}

func parse(raw []byte, p profile) (Document, error) {
	value, ok := decode(raw)
	if !ok {
		return Document{}, ErrInvalidDocument
	}
	v := validator{profile: p, gaps: make(map[string]bool), assets: make(map[string]string)}
	text, ok := v.document(value)
	if !ok || v.nodes > MaxNodes || v.characters > MaxText {
		return Document{}, ErrInvalidDocument
	}

	return Document{data: bytes.Clone(raw), format: kind(value, "format"), plainText: text, gaps: v.gapIDs, assets: v.assetRefs, profile: p, placesAudio: v.audioMarker}, nil
}

// reparse validates a rewritten encoding under the rules d was parsed with.
func (d Document) reparse(raw []byte) (Document, error) { return parse(raw, d.profile) }

// MarshalJSON returns an independent JSON encoding; the zero value cannot be serialized as valid content.
func (d Document) MarshalJSON() ([]byte, error) {
	if len(d.data) == 0 {
		return nil, ErrInvalidDocument
	}
	return bytes.Clone(d.data), nil
}

// Format identifies the reader contract, not an editor's internal representation.
func (d Document) Format() string { return d.format }

// PlainText preserves legacy Markdown exactly and projects semantic content in document order without metadata.
func (d Document) PlainText() string { return d.plainText }

// GapIDs returns unique stable gap identities in document order, independently of their display labels.
func (d Document) GapIDs() []string { return slices.Clone(d.gaps) }

// Assets returns deduplicated asset references in first-use order for the enclosing write's authorization and locks.
func (d Document) Assets() []AssetReference { return slices.Clone(d.assets) }

// PlacesQuestionAudio reports whether a prompt holds the audio marker, which places the question's own audio and names no asset.
func (d Document) PlacesQuestionAudio() bool { return d.placesAudio }

// HasMedia reports whether the document holds an image or audio block of any kind.
func (d Document) HasMedia() bool { return len(d.assets) != 0 || d.placesAudio }
