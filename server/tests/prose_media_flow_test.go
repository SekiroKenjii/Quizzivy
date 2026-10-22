//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func proseWith(text string, media ...map[string]any) map[string]any {
	blocks := []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []any{}}}}}
	for _, m := range media {
		blocks = append(blocks, m)
	}
	return map[string]any{"format": "semantic_v1", "blocks": blocks}
}

// TestPromptMediaReachesThePaperAndExplanationMediaOnlyTheReleasedResult is
// T-R6.5b's projection (D-064 ruling 6): the paper signs the images its prompt
// shows, and nothing a student is sent before the result names an
// explanation's media; the result signs it only when the policy shows
// explanations.
func TestPromptMediaReachesThePaperAndExplanationMediaOnlyTheReleasedResult(t *testing.T) {
	w := bootWithStorage(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	tag := nonce(t)
	upload := func(name string) string {
		t.Helper()
		picture := teacher.send(http.MethodPost, "/teacher/media", new(filePayload(t, name, tinyPNG(t))))
		if picture.status != http.StatusCreated {
			t.Fatalf("%s: %d %s", name, picture.status, picture.body)
		}
		return picture.json["id"].(string)
	}
	shown, worked := upload("ban-do.png"), upload("loi-giai.png")
	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Nhìn bản đồ.\n\nBản đồ đảo", "points": 1,
		"promptContent": proseWith("Nhìn bản đồ.", map[string]any{"type": "image", "assetId": shown, "alt": "Bản đồ đảo"}),
		"explanation":   "Cảng ở phía bắc.", "explanationContent": proseWith("Cảng ở phía bắc.", map[string]any{"type": "image", "assetId": worked, "alt": ""}),
		"options": []map[string]any{{"text": "bắc", "isCorrect": true}, {"text": "nam", "isCorrect": false}},
	})
	edited := objects(teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(question), nil)["contentAssets"])
	if len(edited) != 2 || edited[0]["id"] != shown || edited[1]["id"] != worked || edited[1]["url"] == nil {
		t.Errorf("the editor's question signs %v, want the prompt's image then the explanation's", edited)
	}
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề có bản đồ " + tag})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{id(question)}}},
	})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)

	sit := func(explanations bool) (*client, map[string]any, string) {
		t.Helper()
		classID := teacher.class(fmt.Sprintf("Lớp bản đồ %v %s", explanations, tag))
		now := time.Now()
		assignment := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", map[string]any{
			"testVersionId":   id(version),
			"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
			"window":          map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(2 * time.Hour))},
			"durationMinutes": 30, "maxAttempts": 1,
			"review":    map[string]any{"showScore": true, "showCorrectAnswers": true, "showExplanations": explanations},
			"integrity": map[string]any{"requireFullscreen": false, "blockCopyPaste": false, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000},
		})
		student := w.enrolledStudent(teacher, classID)
		session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
		return student, session, id(session["attempt"].(map[string]any))
	}
	mentions := func(body map[string]any, asset string) bool {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(raw), asset)
	}
	signed := func(label string, assets any, want string) {
		t.Helper()
		list := objects(assets)
		if len(list) != 1 || list[0]["id"] != want || !strings.HasPrefix(fmt.Sprint(list[0]["url"]), "http") {
			t.Errorf("%s signs %v, want the one asset %s with its URL", label, assets, want)
		}
	}

	student, session, attemptID := sit(false)
	paper := objects(session["questions"])[0]
	signed("the paper", paper["promptAssets"], shown)
	reloaded := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID, nil)
	for label, body := range map[string]map[string]any{"the paper": session, "the reloaded paper": reloaded} {
		if mentions(body, worked) {
			t.Errorf("%s names the explanation's image", label)
		}
	}
	student.must(http.StatusOK, http.MethodGet, "/app/media/"+shown+"/url", nil)
	if status, _ := student.call(http.MethodGet, "/app/media/"+worked+"/url", nil); status != http.StatusForbidden {
		t.Errorf("a sitting student minted the explanation's image: %d", status)
	}
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	withheld := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)
	if mentions(withheld, worked) {
		t.Error("a result that withholds explanations names the explanation's image")
	}
	signed("the result's prompt", objects(withheld["questions"])[0]["promptAssets"], shown)

	student, session, attemptID = sit(true)
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	released := objects(student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)["questions"])[0]
	signed("the released result's explanation", released["explanationAssets"], worked)
	if status, _ := student.call(http.MethodGet, "/app/media/"+worked+"/url", nil); status != http.StatusForbidden {
		t.Errorf("a student minted the explanation's image outside the result: %d", status)
	}
}
