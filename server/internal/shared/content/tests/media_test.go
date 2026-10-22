package content_test

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/shared/content"
)

// mediaFixture is one case of api/testdata/question-media-v1.json, the
// projection and media rules of DG-109 (D-064), which the web editor's mirror
// reads too once it builds media (R6-14).
type mediaFixture struct {
	Name        string          `json:"name"`
	Profile     string          `json:"profile"`
	Document    json.RawMessage `json:"document"`
	Valid       bool            `json:"valid"`
	PlainText   string          `json:"plainText"`
	PlacesAudio bool            `json:"placesAudio"`
	Assets      []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"assets"`
}

func mediaFixtures(t testing.TB) []mediaFixture {
	t.Helper()
	raw, err := os.ReadFile("../../../../../api/testdata/question-media-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []mediaFixture
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func parseProfile(t testing.TB, profile string, raw []byte) (content.Document, error) {
	t.Helper()
	switch profile {
	case "prompt":
		return content.ParsePromptWithMedia(raw)
	case "explanation":
		return content.ParseExplanationWithMedia(raw)
	}
	t.Fatalf("unknown profile %q", profile)
	return content.Document{}, nil
}

func TestQuestionMediaContractCases(t *testing.T) {
	for _, tc := range mediaFixtures(t) {
		t.Run(tc.Name, func(t *testing.T) {
			doc, err := parseProfile(t, tc.Profile, tc.Document)
			if !tc.Valid {
				if !errors.Is(err, content.ErrInvalidDocument) {
					t.Fatalf("want rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if doc.PlainText() != tc.PlainText {
				t.Fatalf("projection: %q, want %q", doc.PlainText(), tc.PlainText)
			}
			if doc.PlacesQuestionAudio() != tc.PlacesAudio {
				t.Fatalf("places audio: %v", doc.PlacesQuestionAudio())
			}
			assets := doc.Assets()
			if len(assets) != len(tc.Assets) {
				t.Fatalf("assets: %v", assets)
			}
			for i, asset := range assets {
				if asset.ID != tc.Assets[i].ID || asset.Kind != tc.Assets[i].Kind {
					t.Fatalf("asset %d: %v", i, asset)
				}
			}
			if !doc.HasMedia() {
				t.Fatal("a document with media reports none")
			}
		})
	}
}

// TestTodaysQuestionParsersStillRefuseEveryMediaBlock pins the rule that holds
// until R6-13: the parsers the bank, publish and the projections use today
// accept no media node of DG-109's shapes, so the bank write refuses the
// field (400 VALIDATION_FAILED with the field named).
func TestTodaysQuestionParsersStillRefuseEveryMediaBlock(t *testing.T) {
	for _, tc := range mediaFixtures(t) {
		if !tc.Valid {
			continue
		}
		if _, err := content.ParseQuestionPrompt(tc.Document); err == nil {
			t.Errorf("%s: ParseQuestionPrompt accepted media", tc.Name)
		}
		if _, err := content.ParseQuestion(tc.Document); err == nil {
			t.Errorf("%s: ParseQuestion accepted media", tc.Name)
		}
	}
}

// TestMediaParsersAgreeWithTodaysOnProse shows the media parsers are a strict
// superset: prose today's parsers accept projects the same and has no media.
func TestMediaParsersAgreeWithTodaysOnProse(t *testing.T) {
	prompt := []byte(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Fill ","marks":[]},{"type":"gap","id":"g1","label":"1"}]},{"type":"list","ordered":true,"start":2,"items":[[{"type":"paragraph","content":[]}]]}]}`)
	explanation := []byte(`{"format":"semantic_v1","blocks":[{"type":"heading","level":2,"content":[{"type":"text","text":"Câu hỏi","marks":[]}]},{"type":"table","rows":[[{"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"A","marks":[]}]}]}]]}]}`)
	old, err := content.ParseQuestionPrompt(prompt)
	if err != nil {
		t.Fatal(err)
	}
	withMedia, err := content.ParsePromptWithMedia(prompt)
	if err != nil || withMedia.PlainText() != old.PlainText() || withMedia.HasMedia() || !slices.Equal(withMedia.GapIDs(), old.GapIDs()) {
		t.Fatalf("prompt: %v %q", err, withMedia.PlainText())
	}
	oldExplanation, err := content.ParseQuestion(explanation)
	if err != nil {
		t.Fatal(err)
	}
	withMedia, err = content.ParseExplanationWithMedia(explanation)
	if err != nil || withMedia.PlainText() != oldExplanation.PlainText() || withMedia.HasMedia() {
		t.Fatalf("explanation: %v %q", err, withMedia.PlainText())
	}
}

// TestRewritesKeepTheMediaRules shows Normalize and WithAssetIDs validate a
// question document again under its own rules, not a material's: a decorative
// image and the audio marker would not parse as a material.
func TestRewritesKeepTheMediaRules(t *testing.T) {
	raw := []byte(`{"format":"semantic_v1","blocks":[{"type":"audio"},{"type":"image","assetId":"01935000-0000-7000-8000-00000000000a","alt":""},{"type":"paragraph","content":[{"type":"text","text":"Café","marks":[]}]}]}`)
	doc, err := content.ParsePromptWithMedia(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Parse(raw); err == nil {
		t.Fatal("a material accepted a bare audio marker")
	}
	composed, err := content.Normalize(doc)
	if err != nil || composed.PlainText() != "Café" || !composed.PlacesQuestionAudio() {
		t.Fatalf("normalize: %v %q", err, composed.PlainText())
	}
	rebound, err := doc.WithAssetIDs(map[string]string{"01935000-0000-7000-8000-00000000000a": "01935000-0000-7000-8000-0000000000aa"})
	if err != nil || len(rebound.Assets()) != 1 || rebound.Assets()[0].ID != "01935000-0000-7000-8000-0000000000aa" || !rebound.PlacesQuestionAudio() {
		t.Fatalf("rebind: %v %v", err, rebound.Assets())
	}
}

func TestMediaBlockCapMatchesOpenAPI(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromFile("../../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"QuestionPromptContent", "QuestionExplanationContent"} {
		got, ok := spec.Components.Schemas[name].Value.Extensions["x-media-blocks"].(float64)
		if !ok || int(got) != content.MaxMediaBlocks {
			t.Errorf("%s x-media-blocks = %v, want %d", name, spec.Components.Schemas[name].Value.Extensions["x-media-blocks"], content.MaxMediaBlocks)
		}
	}
}
