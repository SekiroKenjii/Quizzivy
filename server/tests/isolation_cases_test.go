//go:build e2e

package e2e

import (
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (p *party) with(kind, id string) *party {
	q := *p
	q.ids = maps.Clone(p.ids)
	q.ids[kind] = id
	return &q
}

func imageDoc(own *party) map[string]any {
	return map[string]any{"format": "semantic_v1", "blocks": []any{
		map[string]any{"type": "image", "assetId": own.id("media"), "alt": "Hình minh hoạ"},
	}}
}

func textDoc(text string) map[string]any {
	return map[string]any{"format": "semantic_v1", "blocks": []any{
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []any{}}}},
	}}
}

func optionDoc(own *party, slot string) map[string]any {
	if strings.Contains(slot, "/options/") {
		return imageDoc(own)
	}
	return textDoc("giáo viên")
}

func choiceInput(own *party) map[string]any {
	return map[string]any{
		"type": "single_choice", "prompt": "Từ nào là danh từ?", "points": 1, "mediaAssetId": own.id("media"),
		"options": []any{
			map[string]any{"text": "nhanh", "isCorrect": false},
			map[string]any{"text": "giáo viên", "isCorrect": true},
		},
	}
}

func groupBundle(own *party, groupID, slot string) map[string]any {
	questionID := uuid.NewString()
	group := map[string]any{
		"id": groupID, "title": "Nhóm cách ly",
		"members":    []any{map[string]any{"questionId": questionID, "optionOrder": "fixed"}},
		"stimuli":    []any{map[string]any{"id": uuid.NewString(), "title": "Bài đọc", "content": imageDoc(own), "gaps": []any{}}},
		"recordings": []any{},
	}
	if strings.HasPrefix(slot, "/bundle/group/recordings/") {
		group["stimuli"].([]any)[0].(map[string]any)["content"] = map[string]any{"format": "semantic_v1", "blocks": []any{
			map[string]any{"type": "audio", "assetId": own.id("audio"), "label": "Bài nghe"},
		}}
		group["recordings"] = []any{map[string]any{
			"id": uuid.NewString(), "assetId": own.id("audio"),
			"policy": map[string]any{"maxPlays": 2, "allowSeek": false, "showTranscriptAfterSubmit": false},
		}}
	}
	return map[string]any{"group": group, "questions": []any{map[string]any{"id": questionID, "input": choiceInput(own)}}}
}

func (x *iso) testUpdatedAt(own *party) string {
	return own.c.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+own.id("test"), nil)["updatedAt"].(string)
}

func assignmentBody(own *party) map[string]any {
	now := time.Now()
	return map[string]any{
		"testVersionId":   own.id("test-version"),
		"targets":         map[string]any{"classIds": []any{own.id("class")}, "studentIds": []any{own.id("student")}},
		"window":          map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(2 * time.Hour))},
		"durationMinutes": 30, "maxAttempts": 1,
		"review": map[string]any{"showScore": true, "showCorrectAnswers": false, "showExplanations": false},
		"integrity": map[string]any{
			"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
		},
	}
}

func reviewBody(own *party, slot string) map[string]any {
	origins := map[string]any{"type": "teacher_entered", "prompt": "teacher_entered", "options": "teacher_entered", "answer": "teacher_entered", "points": "teacher_entered"}
	question := func(id string) map[string]any {
		return map[string]any{
			"id": id, "label": "1", "type": "single_choice", "prompt": imageDoc(own),
			"options": []any{map[string]any{"id": "a", "label": "A", "content": optionDoc(own, slot)}},
			"blanks":  []any{},
			"answer":  map[string]any{"state": "known", "optionIds": []any{"a"}, "evidence": []any{}},
			"points":  "1", "origins": origins, "source": []any{},
		}
	}
	item := map[string]any{"question": question("q1")}
	if strings.Contains(slot, "/group/") {
		item = map[string]any{"group": map[string]any{
			"id": "g1", "stimulus": imageDoc(own), "gaps": []any{}, "source": []any{}, "questions": []any{question("q2")},
		}}
	}
	return map[string]any{
		"expectedRevision": 1, "title": "Ôn tập", "acknowledged": []any{},
		"sections": []any{map[string]any{"id": "s1", "title": "Phần 1", "origin": "teacher_entered", "source": []any{}, "items": []any{item}}},
	}
}

func freshGroup(_ *iso, own *party) *party {
	created := own.c.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
		"bundle": map[string]any{
			"group":     map[string]any{"id": uuid.NewString(), "title": "Nhóm dùng một lần", "members": []any{}, "stimuli": []any{}, "recordings": []any{}},
			"questions": []any{},
		},
	})
	return own.with("question-group", created["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string))
}

func freshTest(_ *iso, own *party) *party {
	return own.with("test", id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề dùng một lần"})))
}

func freshQuestion(_ *iso, own *party) *party {
	return own.with("question", id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/questions", choiceInput(own))))
}

func freshImport(_ *iso, own *party) *party {
	return own.with("import", id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/imports", map[string]any{"requestId": uuid.NewString(), "title": "Nhập dùng một lần"})))
}

func freshReviewedImport(x *iso, own *party) *party {
	return own.with("reviewed-import", x.w.reviewedImport(own.c, own.userID, "dung-mot-lan-"+nonce(x.t)))
}

func freshMedia(x *iso, own *party) *party {
	upload := own.c.send(http.MethodPost, "/teacher/media", new(filePayload(x.t, "dung-mot-lan.png", tinyPNG(x.t))))
	if upload.status != http.StatusCreated {
		x.t.Fatalf("fresh media: %d %s", upload.status, upload.body)
	}
	return own.with("media", upload.json["id"].(string))
}

func freshAssignment(_ *iso, own *party) *party {
	body := assignmentBody(own)
	body["draft"] = true
	body["targets"] = map[string]any{"classIds": []any{}, "studentIds": []any{}}
	return own.with("assignment", id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", body)))
}

func freshStudent(x *iso, own *party) *party {
	created := own.c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "dung-mot-lan-" + nonce(x.t) + "@example.com", "fullName": "Học viên dùng một lần", "classIds": []any{own.id("class")},
	})
	return own.with("student", created["user"].(map[string]any)["id"].(string))
}

func freshOverride(x *iso, own *party) *party {
	student := freshStudent(x, own).id("student")
	own.c.must(http.StatusOK, http.MethodPut, "/teacher/assignments/"+own.id("assignment")+"/student-overrides", map[string]any{
		"studentIds": []any{student}, "extraAttempts": 1, "reason": "Dùng một lần",
	})
	return own.with("override", student)
}

func freshAttempt(x *iso, own *party) *party {
	created := own.c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "lam-bai-" + nonce(x.t) + "@example.com", "fullName": "Học viên làm bài", "classIds": []any{own.id("class")},
	})
	user := created["user"].(map[string]any)
	learner := x.w.signedIn(user["email"].(string), created["temporaryPassword"].(string))
	session := learner.must(http.StatusOK, http.MethodPost, "/app/assignments/"+own.id("assignment")+"/attempts", nil)
	attempt := id(session["attempt"].(map[string]any))
	learner.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attempt+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	return own.with("attempt", attempt)
}

func freshPublishedTest(_ *iso, own *party) *party {
	question := id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/questions", choiceInput(own)))
	test := own.c.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề đã phát hành dùng một lần"})
	test = own.c.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{question}}},
	})
	section := test["sections"].([]any)[0].(map[string]any)["id"].(string)
	version := id(own.c.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil))
	return own.with("test", id(test)).with("section", section).with("test-version", version).with("question", question)
}

func freshClass(x *iso, own *party) *party {
	return own.with("class", own.c.class("Lớp dùng một lần "+nonce(x.t)))
}

func freshMembership(x *iso, own *party) *party {
	return freshStudent(x, freshClass(x, own))
}

func reason(string) map[string]any { return map[string]any{"reason": "Kiểm tra cách ly"} }

const (
	promptAsset       = "body /sections/-/items/-/question/prompt/blocks/-/assetId"
	stimulusAsset     = "body /sections/-/items/-/group/stimulus/blocks/-/assetId"
	memberPromptAsset = "body /sections/-/items/-/group/questions/-/prompt/blocks/-/assetId"
	optionAsset       = "body /sections/-/items/-/question/options/-/content/blocks/-/assetId"
	memberOptionAsset = "body /sections/-/items/-/group/questions/-/options/-/content/blocks/-/assetId"

	storedUnchecked = "a review stores asset references unchecked, whoever owns them; the commit check proves they resolve in the owner's scope"
	textOnly        = "an option holds one text paragraph, so an asset is refused whatever it names"
	notMember       = "removing someone who is not a member answers as removing a member does"
	outsideEvent    = "an event naming a question outside the attempt's paper is kept without it; the cross-reference scan proves the id is not stored"
	outsideAnswer   = "an answer to a question outside the attempt's paper is dropped; the cross-reference scan proves no row names it"
	ownOnly         = "marks only the caller's own notifications; any other id is skipped"
)

func isolationCases() map[string]isoCase {
	fixed := func(v map[string]any) func(*iso, *party, string) any {
		return func(*iso, *party, string) any { return v }
	}
	session := func(v map[string]any) func(*iso, *party, string) any {
		return func(_ *iso, own *party, _ string) any {
			out := maps.Clone(v)
			out["sessionId"] = own.session
			return out
		}
	}
	seq := 0
	events := func(v map[string]any) func(*iso, *party, string) any {
		return func(_ *iso, own *party, _ string) any {
			seq++
			out := maps.Clone(v)
			out["sessionId"] = own.session
			out["events"] = []any{map[string]any{"kind": "tab_hidden", "occurredAt": rfc3339(time.Now()), "clientSeq": seq, "questionId": ""}}
			return out
		}
	}
	later := rfc3339(time.Now().Add(24 * time.Hour))
	return map[string]isoCase{
		"listQuestionGroups": {},
		"createQuestionGroup": {fresh: freshPublishedTest, as: map[string]string{"body /bundle/group/recordings/-/assetId": "audio"}, link: map[string][]string{"body /bundle/group/recordings/-/assetId": {"/bundle/group/stimuli/-/content/blocks/-/assetId"}}, body: func(x *iso, own *party, slot string) any {
			body := map[string]any{"bundle": groupBundle(own, uuid.NewString(), slot)}
			if slot == "/ownerSectionId" {
				body["ownerSectionId"] = own.id("section")
				body["expectedTestUpdatedAt"] = x.testUpdatedAt(own)
			}
			return body
		}},
		"getQuestionGroup": {},
		"updateQuestionGroup": {
			fresh: freshGroup,
			as:    map[string]string{"body /bundle/group/recordings/-/assetId": "audio"},
			body: func(_ *iso, own *party, slot string) any {
				return map[string]any{"bundle": groupBundle(own, own.id("question-group"), slot), "expectedRevision": 1}
			},
			link: map[string][]string{"path id": {"/bundle/group/id"}, "body /bundle/group/recordings/-/assetId": {"/bundle/group/stimuli/-/content/blocks/-/assetId"}},
		},
		"deleteQuestionGroup":  {query: map[string]string{"expectedRevision": "1"}, fresh: freshGroup},
		"archiveQuestionGroup": {body: fixed(map[string]any{"archived": true, "expectedRevision": 1}), fresh: freshGroup},
		"copyQuestionGroup": {fresh: freshPublishedTest, body: func(x *iso, own *party, slot string) any {
			body := map[string]any{"expectedRevision": 1}
			if slot == "/ownerSectionId" {
				body["ownerSectionId"] = own.id("section")
				body["expectedTestUpdatedAt"] = x.testUpdatedAt(own)
			}
			return body
		}},

		"listTests":  {query: map[string]string{"limit": "100"}},
		"createTest": {},
		"deleteTest": {fresh: freshTest},
		"getTest":    {},
		"updateTest": {
			fresh: freshPublishedTest,
			body: func(x *iso, own *party, slot string) any {
				section := map[string]any{"id": own.id("section"), "title": "Phần 1", "questionIds": []any{own.id("question")}}
				body := map[string]any{"expectedUpdatedAt": x.testUpdatedAt(own), "sections": []any{section}}
				if slot == "/sections/-/units/-/id" {
					body["outlineFormat"] = "group_v1"
					section["units"] = []any{map[string]any{"kind": "question", "id": own.id("question")}}
				}
				return body
			},
			link: map[string][]string{"body /sections/-/units/-/id": {"/sections/-/questionIds/-"}},
		},
		"publishTest":        {},
		"duplicateTest":      {},
		"listTestVersions":   {excuse: map[string]string{"path id": "a missing test lists no versions"}},
		"deleteTestVersion":  {params: map[string]string{"version": "1"}, fresh: freshPublishedTest},
		"getTestVersionDiff": {params: map[string]string{"version": "1"}, query: map[string]string{"against": "draft"}, fresh: freshPublishedTest},
		"setCurrentTestVersion": {params: map[string]string{"version": "1"}, fresh: freshPublishedTest, body: func(x *iso, own *party, _ string) any {
			return map[string]any{"expectedUpdatedAt": x.testUpdatedAt(own)}
		}},
		"createDraftFromTestVersion": {params: map[string]string{"version": "1"}, fresh: freshPublishedTest, body: func(x *iso, own *party, _ string) any {
			return map[string]any{"expectedUpdatedAt": x.testUpdatedAt(own)}
		}},
		"previewTest": {},

		"listQuestions":     {query: map[string]string{"limit": "100"}},
		"createQuestion":    {body: func(_ *iso, own *party, _ string) any { return choiceInput(own) }},
		"tagQuestions":      {excuse: map[string]string{"*": "tags only the questions the caller reaches"}, body: fixed(map[string]any{"questionIds": []any{""}, "tags": []any{"đã kiểm tra"}})},
		"getQuestion":       {},
		"updateQuestion":    {body: func(_ *iso, own *party, _ string) any { return choiceInput(own) }},
		"deleteQuestion":    {fresh: freshQuestion},
		"duplicateQuestion": {},

		"createWordImport": {},
		"listWordImports":  {},
		"getWordImport":    {},
		"pasteImportSource": {body: func(*iso, *party, string) any {
			return map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": "Question 1. Plain text"}
		}},
		"uploadImportSource":        {query: map[string]string{"role": "exam", "expectedRevision": "1"}, format: "docx"},
		"downloadImportSource":      {},
		"getWordImportCapabilities": {},
		"getWordImportLimits":       {},
		"processWordImport": {body: func(*iso, *party, string) any {
			return map[string]any{"requestId": uuid.NewString(), "expectedRevision": 1}
		}},
		"cancelWordImport":    {body: fixed(map[string]any{"expectedRevision": 1}), fresh: freshImport},
		"getWordImportReview": {as: map[string]string{"path id": "reviewed-import"}},
		"saveWordImportReview": {
			as: map[string]string{"path id": "reviewed-import"}, fresh: freshReviewedImport,
			excuse: map[string]string{
				promptAsset: storedUnchecked, stimulusAsset: storedUnchecked, memberPromptAsset: storedUnchecked,
			},
			blind: map[string]string{
				promptAsset: storedUnchecked, stimulusAsset: storedUnchecked, memberPromptAsset: storedUnchecked,
				optionAsset: textOnly, memberOptionAsset: textOnly,
			},
			body: func(_ *iso, own *party, slot string) any { return reviewBody(own, slot) },
		},
		"adoptWordImportReprocessed": {as: map[string]string{"path id": "reviewed-import"}, body: fixed(map[string]any{"expectedRevision": 1})},
		"getWordImportSource":        {query: map[string]string{"role": "exam"}},
		"commitWordImport": {body: func(*iso, *party, string) any {
			return map[string]any{"requestId": uuid.NewString(), "draftRevision": 1}
		}},

		"listMedia":    {},
		"uploadMedia":  {},
		"updateMedia":  {fresh: freshMedia, body: fixed(map[string]any{"displayName": "Tên đã đổi"})},
		"deleteMedia":  {fresh: freshMedia},
		"replaceMedia": {fresh: freshMedia, format: "png"},

		"listAssignments":        {excuse: map[string]string{"query classId": "a filter by a missing class lists nothing"}},
		"createAssignment":       {body: func(_ *iso, own *party, _ string) any { return assignmentBody(own) }},
		"deleteAssignment":       {fresh: freshAssignment},
		"getAssignment":          {},
		"updateAssignment":       {body: func(_ *iso, own *party, _ string) any { return assignmentBody(own) }},
		"listAnswersForQuestion": {},
		"reopenAssignment":       {body: fixed(map[string]any{"closesAt": later, "reason": "Kiểm tra cách ly"})},
		"getAssignmentMonitor":   {},
		"extendAssignment":       {body: fixed(map[string]any{"minutes": 10, "notify": false})},
		"duplicateAssignment":    {body: fixed(map[string]any{"classIds": []any{""}})},
		"getItemAnalysis":        {},
		"exportResultsCsv":       {},
		"listStudentOverrides":   {},
		"setStudentOverrides": {body: fixed(map[string]any{
			"studentIds": []any{""}, "extraAttempts": 1, "reason": "Kiểm tra cách ly",
		})},
		"deleteStudentOverride": {fresh: freshOverride},

		"listGradingQueue":    {},
		"listAttempts":        {},
		"getAttemptForReview": {},
		"getAttemptEvents":    {},
		"extendAttempt":       {body: fixed(map[string]any{"minutes": 10, "reason": "Kiểm tra cách ly"})},
		"resetAttempt":        {body: fixed(reason("")), fresh: freshAttempt},
		"voidAttempt":         {body: fixed(reason("")), fresh: freshAttempt},
		"setAttemptNote":      {body: fixed(map[string]any{"note": "Ghi chú cách ly"})},
		"flagAttempt":         {body: fixed(map[string]any{"flagged": true})},
		"gradeAttempt":        {body: fixed(map[string]any{"items": []any{map[string]any{"questionId": "", "points": 1}}})},
		"finishGrading":       {},
		"getDashboard":        {},
		"getTeacherSummary":   {},
		"listStudents":        {excuse: map[string]string{"query classId": "a filter by a missing class lists nothing"}},
		"createStudent": {body: func(x *iso, _ *party, _ string) any {
			return map[string]any{"email": "tao-" + nonce(x.t) + "@example.com", "fullName": "Học viên mới", "classIds": []any{""}}
		}},
		"getStudent":           {},
		"updateStudent":        {body: fixed(map[string]any{"fullName": "Tên đã sửa"})},
		"resetStudentPassword": {fresh: freshStudent},

		"listClasses":      {},
		"createClass":      {},
		"deleteClass":      {fresh: freshClass},
		"getClass":         {},
		"updateClass":      {body: fixed(map[string]any{"name": "Lớp đã đổi tên"})},
		"listClassMembers": {excuse: map[string]string{"path id": "a missing class lists no members"}},
		"addClassMember":   {body: fixed(map[string]any{"userId": ""})},
		"removeClassMember": {
			fresh:  freshMembership,
			excuse: map[string]string{"path userId": notMember},
			blind:  map[string]string{"path userId": notMember},
		},
		"getJoinCode":    {},
		"rotateJoinCode": {body: fixed(map[string]any{})},
		"revokeJoinCode": {},

		"listMyClasses":        {student: true},
		"joinClass":            {student: true},
		"listMyAssignments":    {student: true},
		"getMyAssignment":      {student: true},
		"startOrResumeAttempt": {student: true, body: fixed(map[string]any{"resume": ""})},
		"getAttempt":           {student: true},
		"saveAnswers": {
			student: true,
			excuse:  map[string]string{"body /events/-/questionId": outsideEvent, "body /answers/+": outsideAnswer},
			blind:   map[string]string{"body /events/-/questionId": outsideEvent, "body /answers/+": outsideAnswer},
			body:    events(map[string]any{"answers": map[string]any{"": map[string]any{"type": "choice", "optionIds": []any{}}}}),
		},
		"flushEvents": {
			student: true,
			excuse:  map[string]string{"body /events/-/questionId": outsideEvent},
			blind:   map[string]string{"body /events/-/questionId": outsideEvent},
			body:    events(map[string]any{}),
		},
		"recordAudioPlay": {student: true, body: fixed(map[string]any{"questionId": ""})},
		"recordGroupAudioPlay": {student: true, body: func(_ *iso, own *party, _ string) any {
			return map[string]any{"recordingId": "", "playId": uuid.NewString(), "sessionId": own.session}
		}},
		"submitAttempt":    {student: true, body: session(map[string]any{"reason": "manual"})},
		"getAttemptResult": {student: true},
		"getMediaUrl":      {student: true},

		"listNotifications": {},
		"markNotificationsRead": {
			excuse: map[string]string{"body /ids/-": ownOnly},
			blind:  map[string]string{"body /ids/-": ownOnly},
			body:   fixed(map[string]any{"ids": []any{""}}),
		},
		"getMySummary":                  {},
		"getNotificationPreferences":    {},
		"updateNotificationPreferences": {},
		"updatePreferences":             {},
	}
}
