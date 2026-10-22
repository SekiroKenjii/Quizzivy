package content

import "strings"

// profile names the rules a document's media blocks follow. A group material may place images and labelled
// recordings anywhere; a question's prompt and explanation follow DG-109's narrower rules (D-064).
type profile uint8

const (
	materialProfile profile = iota
	promptProfile
	explanationProfile
)

func isMedia(block any) bool {
	t := kind(block, typeKey)
	return t == image || t == audio
}

// questionMedia validates an image or audio block of a prompt or explanation. Media stand only among the document's own
// blocks, never in a table cell or a list item, and at most MaxMediaBlocks of them. An image's alt may be empty, which
// marks it decorative, and projects as written; an alt of only white space is refused, because empty is the one way to
// say decorative and a screen reader would be handed a blank description (QC-R6D-2). A prompt's audio is a bare marker placing the question's own audio, once;
// an explanation's names its asset. Neither projects to any text.
func (v *validator) questionMedia(value any) (string, bool) {
	v.media++
	if v.nested > 0 || v.media > MaxMediaBlocks {
		return "", false
	}
	if kind(value, typeKey) == image {
		obj, ok := object(value, typeKey, assetKey, "alt")
		if !ok || !v.reference(value, obj) {
			return "", false
		}
		alt, ok := v.string(obj["alt"], 0, 1000)
		if ok && alt != "" && strings.TrimSpace(alt) == "" {
			return "", false
		}
		return alt, ok
	}
	if v.profile == promptProfile {
		if _, ok := object(value, typeKey); !ok || v.audioMarker {
			return "", false
		}
		v.audioMarker = true
		return "", true
	}
	obj, ok := object(value, typeKey, assetKey)
	return "", ok && v.reference(value, obj)
}
