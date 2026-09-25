// Package recognition reconstructs a reviewable exam draft from Word source evidence.
// It reads structure and explicit keys only; it never solves, invents or silently drops content.
package recognition

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Version identifies the rule set so stored drafts can be traced to the recognizer that produced them.
const Version = "rules-v2"

const (
	evidenceVersion = "ooxml-blocks-v1"
	examRole        = "exam"
	keyRole         = "answer_key"
	maxQuestions    = 2000
)

// Recognize builds the machine draft for one exam and an optional companion answer key.
func Recognize(ctx context.Context, docs []domain.EvidenceDocument, p domain.RecognitionProfile) (domain.Draft, error) {
	if err := validateEvidence(docs); err != nil {
		return domain.Draft{}, err
	}
	examDoc, keyDoc := byRole(docs)
	lines := documentLines(examDoc)
	e := parseExam(examDoc.SourceID, lines)
	if e.questions > maxQuestions {
		return domain.Draft{}, domain.ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return domain.Draft{}, err
	}
	b := newBuilder(e)
	if len(e.keyLines) > 0 {
		b.apply(parseKeys(e.keyLines, e.paper, keySameFile), 0)
	}
	if keyDoc != nil {
		b.apply(parseKeys(documentLines(keyDoc), 0, keyCompanion), p.KeyPaper)
	}
	draft := b.draft()
	b.unassignedRuns(lines)
	b.extraPapers()
	b.sourceNotices(docs)
	draft.Notices = append(draft.Notices, b.notices...)
	return draft, ctx.Err()
}

func byRole(docs []domain.EvidenceDocument) (*domain.EvidenceDocument, *domain.EvidenceDocument) {
	var exam, key *domain.EvidenceDocument
	for i := range docs {
		if docs[i].Role == examRole {
			exam = &docs[i]
		} else {
			key = &docs[i]
		}
	}
	return exam, key
}

func validateEvidence(docs []domain.EvidenceDocument) error {
	if len(docs) < 1 || len(docs) > 2 {
		return domain.ErrInvalid
	}
	roles := map[string]bool{}
	total, blocks, examText := 0, 0, false
	for _, d := range docs {
		if d.Version != evidenceVersion || d.SourceID == "" || len(d.SourceID) > 128 || roles[d.Role] || (d.Role != examRole && d.Role != keyRole) {
			return domain.ErrInvalid
		}
		roles[d.Role] = true
		for _, b := range d.Blocks {
			if b.ID == "" || len(b.ID) > 200 || !utf8.ValidString(b.Text) || !orderedSpans(b) {
				return domain.ErrInvalid
			}
			total += len(b.Text)
			examText = examText || (d.Role == examRole && readable(&b))
		}
		blocks += len(d.Blocks)
	}
	if total > 8<<20 || blocks > 20000 {
		return domain.ErrTooLarge
	}
	if !examText {
		return domain.ErrUnsupported
	}
	return nil
}

func orderedSpans(b domain.EvidenceBlock) bool {
	position, length := 0, utf8.RuneCountInString(b.Text)
	for _, span := range b.Spans {
		if span.Start < position || span.End < span.Start || span.End > length {
			return false
		}
		position = span.End
	}
	return true
}

func identity(parts ...string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(parts, "\x00"))).String()
}
