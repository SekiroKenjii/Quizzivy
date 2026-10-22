package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
)

// SameContent reports whether q and other hold the same authored content, whatever their identities: the type, the
// prompt and its rich content, the options and which are correct, the blanks and their accepted answers, the points,
// level and skill, the tags, the explanation, the sample answer, the media and the audio policy. It does not look at the
// question, option, blank and gap ids, which a copy remaps, nor at where a question sits. Both sides are put through
// Copy with the same fixed identities and the results compared, so a copy that nobody has touched is always the same
// content as its source, and so is any copy of it.
func (q GroupQuestion) SameContent(other GroupQuestion) bool {
	a, ok := q.contentKey()
	if !ok {
		return false
	}
	b, ok := other.contentKey()
	return ok && reflect.DeepEqual(a, b)
}

type contentKey struct {
	Type               string
	Level              string
	Skill              string
	Prompt             string
	PromptContent      any
	Points             string
	Explanation        string
	ExplanationContent any
	SampleAnswer       string
	MediaAssetID       string
	MediaAssetKind     string
	MediaAlt           string
	Audio              *audioKey
	Transcript         string
	Tags               []string
	Options            []optionKey
	Blanks             []blankKey
	SelectCount        int
	MaxWords           int
	MatchAllowReuse    string
	MatchItems         []matchItemKey
	Rubric             []rubricKey
}

type matchItemKey struct {
	ID            string
	Text          string
	CorrectOption int
}

type rubricKey struct {
	ID    string
	Label string
}

type audioKey struct {
	MaxPlays       int
	Unlimited      bool
	AllowSeek      bool
	ShowTranscript bool
	AllowPause     bool
}

type optionKey struct {
	ID        string
	Content   any
	Text      string
	IsCorrect bool
}

type blankKey struct {
	ID              string
	GapID           string
	Ordinal         int
	CaseSensitive   bool
	AcceptedAnswers []string
}

func (q GroupQuestion) contentKey() (contentKey, bool) {
	var next int
	fixed, err := q.Copy(func() string {
		next++
		return fmt.Sprintf("00000000-0000-7000-8000-%012d", next)
	})
	if err != nil {
		return contentKey{}, false
	}
	in := fixed.Input
	key := contentKey{
		Type: string(in.Type), Level: textOf(in.Level), Skill: textOf(in.Skill), Prompt: in.Prompt, Points: in.Points,
		Explanation: textOf(in.Explanation), SampleAnswer: textOf(in.SampleAnswer), MediaAssetID: textOf(in.MediaAssetID),
		MediaAssetKind: textOf(fixed.MediaAssetKind), MediaAlt: textOf(in.MediaAlt), Transcript: textOf(in.Transcript),
		Tags: sortedCopy(in.Tags), Options: make([]optionKey, 0, len(in.Options)), Blanks: make([]blankKey, 0, len(in.Blanks)),
	}
	var ok bool
	if key.PromptContent, ok = documentOf(in.PromptContent); !ok {
		return contentKey{}, false
	}
	if key.ExplanationContent, ok = documentOf(in.ExplanationContent); !ok {
		return contentKey{}, false
	}
	if in.Audio != nil {
		key.Audio = &audioKey{AllowSeek: in.Audio.AllowSeek, ShowTranscript: in.Audio.ShowTranscriptAfterSubmit, AllowPause: in.Audio.PauseAllowed()}
		if in.Audio.MaxPlays == nil {
			key.Audio.Unlimited = true
		} else {
			key.Audio.MaxPlays = *in.Audio.MaxPlays
		}
	}
	for _, option := range in.Options {
		content, ok := documentOf(option.Content)
		if !ok {
			return contentKey{}, false
		}
		key.Options = append(key.Options, optionKey{ID: textOf(option.ID), Content: content, Text: option.Text, IsCorrect: option.IsCorrect})
	}
	for _, blank := range in.Blanks {
		key.Blanks = append(key.Blanks, blankKey{ID: textOf(blank.ID), GapID: textOf(blank.GapID), Ordinal: blank.Ordinal,
			CaseSensitive: blank.CaseSensitive, AcceptedAnswers: sortedCopy(blank.AcceptedAnswers)})
	}
	if in.SelectCount != nil {
		key.SelectCount = *in.SelectCount
	}
	if in.MaxWords != nil {
		key.MaxWords = *in.MaxWords
	}
	if in.MatchAllowReuse != nil {
		key.MatchAllowReuse = fmt.Sprint(*in.MatchAllowReuse)
	}
	for _, item := range in.MatchItems {
		key.MatchItems = append(key.MatchItems, matchItemKey{ID: textOf(item.ID), Text: item.Text, CorrectOption: item.CorrectOption})
	}
	for _, criterion := range in.Rubric {
		key.Rubric = append(key.Rubric, rubricKey{ID: textOf(criterion.ID), Label: criterion.Label})
	}
	return key, true
}

func textOf[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func sortedCopy(values []string) []string {
	out := append([]string{}, values...)
	slices.Sort(out)
	return out
}

func documentOf(raw json.RawMessage) (any, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, true
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, false
	}
	return value, true
}
