package httpx

import (
	"context"
	"net/http"

	"golang.org/x/text/language"
)

var messageLanguages = language.NewMatcher([]language.Tag{language.Vietnamese, language.English})

// Text returns en when the request's Accept-Language prefers English to
// Vietnamese, and vi otherwise, including when the header is absent or names
// neither. It reads the header WithRequestMeta recorded.
func Text(ctx context.Context, vi, en string) string {
	return pick(RequestMetaFromContext(ctx).Language, vi, en)
}

// TextFor is Text for a handler that holds the request and runs outside
// WithRequestMeta.
func TextFor(r *http.Request, vi, en string) string {
	return pick(r.Header.Get("Accept-Language"), vi, en)
}

func pick(accept, vi, en string) string {
	if _, index := language.MatchStrings(messageLanguages, accept); index == 1 {
		return en
	}
	return vi
}
