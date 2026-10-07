package command

import (
	"context"
	"golang.org/x/text/unicode/norm"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Paste carries a private plaintext exam source and its replay identity.
type Paste struct {
	ImportID, UploadID, Text string
	ExpectedRevision         int64
	Actor                    actor.Actor
}

// PasteHandler shares file intake capacity, reservations and storage with uploads.
type PasteHandler struct{ Upload UploadHandler }

func (h PasteHandler) Handle(ctx context.Context, in Paste) (domain.Receipt, error) {
	text, characters, err := normalizeText(in.Text)
	if err != nil {
		return domain.Receipt{}, err
	}
	return h.Upload.handlePrepared(ctx, Upload{ImportID: in.ImportID, UploadID: in.UploadID, ExpectedRevision: in.ExpectedRevision, Role: "exam", Filename: "pasted-text.txt", Body: strings.NewReader(text), Actor: in.Actor}, "text", &characters)
}

func normalizeText(text string) (string, int, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return "", 0, domain.ErrInvalid
	}
	text = norm.NFC.String(text)
	characters := utf8.RuneCountInString(text)
	if characters > domain.MaxPasteCharacters {
		return "", 0, domain.ErrTooLarge
	}
	projection := strings.TrimPrefix(text, "\ufeff")
	if strings.TrimFunc(projection, unicode.IsSpace) == "" {
		return "", 0, domain.ErrInvalid
	}
	lines := 0
	for _, line := range strings.FieldsFunc(projection, func(r rune) bool { return r == '\r' || r == '\n' }) {
		if strings.TrimFunc(line, unicode.IsSpace) != "" {
			lines++
		}
	}
	if lines > domain.MaxPasteLines {
		return "", 0, domain.ErrTooLarge
	}
	return text, characters, nil
}
