package domain_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/audit/domain"
)

func TestADiffOfOldAndNewPairsBecomesChangesInFieldOrder(t *testing.T) {
	got := domain.Present([]byte(`{"status": {"old": "in_progress", "new": "voided"}, "deadline_at": {"old": "2001-03-15T10:00:00Z", "new": null}}`))
	want := []domain.Change{
		{Field: "deadline_at", From: "2001-03-15T10:00:00Z", To: nil},
		{Field: "status", From: "in_progress", To: "voided"},
	}
	if !reflect.DeepEqual(got.Changes, want) || got.Details != nil {
		t.Errorf("changes %+v and details %+v, want %+v and none", got.Changes, got.Details, want)
	}
}

func TestADiffOfAnyOtherShapeBecomesDetails(t *testing.T) {
	got := domain.Present([]byte(`{"from": "Nguyễn A", "to": "Nguyễn B"}`))
	if len(got.Changes) != 0 || !reflect.DeepEqual(got.Details, map[string]any{"from": "Nguyễn A", "to": "Nguyễn B"}) {
		t.Errorf("changes %+v, details %+v", got.Changes, got.Details)
	}
}

func TestAMixedDiffIsSplitKeyByKey(t *testing.T) {
	got := domain.Present([]byte(`{"closes_at": {"old": 1, "new": 2}, "reason": "maintenance", "windowId": "w1"}`))
	if len(got.Changes) != 1 || got.Changes[0].Field != "closes_at" {
		t.Errorf("changes %+v", got.Changes)
	}
	if !reflect.DeepEqual(got.Details, map[string]any{"reason": "maintenance", "windowId": "w1"}) {
		t.Errorf("details %+v", got.Details)
	}
}

func TestAnObjectWithMoreThanOldAndNewIsNotAChange(t *testing.T) {
	got := domain.Present([]byte(`{"credits": {"old": 1, "new": 2, "unit": "tokens"}, "before": {"custom": false}, "after": {"custom": true}}`))
	if len(got.Changes) != 0 || len(got.Details) != 3 {
		t.Errorf("changes %+v, details %+v, want all three keys as details", got.Changes, got.Details)
	}
}

func TestNothingStoredIsNothingToShow(t *testing.T) {
	for _, stored := range [][]byte{nil, {}, []byte("  "), []byte("null"), []byte("{}"), []byte("not json")} {
		got := domain.Present(stored)
		if got.Changes == nil || len(got.Changes) != 0 || len(got.Details) != 0 {
			t.Errorf("%q shows %+v, want an empty list of changes and no details", stored, got)
		}
	}
}

func TestADiffThatIsNotAnObjectIsTheOneDetailValue(t *testing.T) {
	got := domain.Present([]byte(`["a", "b"]`))
	if !reflect.DeepEqual(got.Details, map[string]any{"value": []any{"a", "b"}}) {
		t.Errorf("details %+v", got.Details)
	}
}

func TestANumberKeepsItsExactDigits(t *testing.T) {
	got := domain.Present([]byte(`{"bytes": 9007199254740993, "ratio": 0.10}`))
	encoded, err := json.Marshal(got.Details)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"bytes":9007199254740993,"ratio":0.10}` {
		t.Errorf("details encode as %s", encoded)
	}
}

func TestAStoredFullCodeComesBackHintedWhateverTheKeyIsCalled(t *testing.T) {
	for _, key := range []string{"code", "joinCode", "code_hash", "JOIN_CODE", "Code-Hash"} {
		stored := `{"` + key + `": "ABCD-EFGH-WXYZ", "other": "ABCD-EFGH-WXYZ"}`
		got := domain.Present([]byte(stored))
		if got.Details[key] != "••••-WXYZ" {
			t.Errorf("%s shows as %v, want ••••-WXYZ", key, got.Details[key])
		}
		if got.Details["other"] != "ABCD-EFGH-WXYZ" {
			t.Errorf("a key that is not a code was changed: %v", got.Details["other"])
		}
	}
}

func TestACodeInAChangeIsHintedOnBothSides(t *testing.T) {
	got := domain.Present([]byte(`{"code": {"old": "ABCDEFGH", "new": "QRSTUVWX"}}`))
	want := []domain.Change{{Field: "code", From: "••••-EFGH", To: "••••-UVWX"}}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("changes %+v, want %+v", got.Changes, want)
	}
}

func TestACodeIsHintedAtAnyDepth(t *testing.T) {
	got := domain.Present([]byte(`{"issued": {"joinCode": "ABCDEFGHJK", "uses": 3}, "list": [{"code": "ZZZZYYYYXXXX"}]}`))
	issued := got.Details["issued"].(map[string]any)
	list := got.Details["list"].([]any)[0].(map[string]any)
	if issued["joinCode"] != "••••-GHJK" || issued["uses"] == nil || list["code"] != "••••-XXXX" {
		t.Errorf("details %+v", got.Details)
	}
}

func TestAShortOrNonTextCodeShowsNoCharacterOfIt(t *testing.T) {
	got := domain.Present([]byte(`{"code": "ABC", "joinCode": 12345678, "code_hash": {"a": "b"}, "x": {"code": null}}`))
	if got.Details["code"] != "••••" || got.Details["joinCode"] != "••••" || got.Details["code_hash"] != "••••" {
		t.Errorf("details %+v", got.Details)
	}
	if inner := got.Details["x"].(map[string]any); inner["code"] != nil {
		t.Errorf("a null code was shown as %v, want null", inner["code"])
	}
}

func TestPasswordsTokensAndSecretsAreNeverShown(t *testing.T) {
	stored := `{
		"password": "hunter2", "temporaryPassword": "Tmp-1234", "token": "abc", "secret": "s",
		"Temporary_Password": "x", "refresh-token": "y", "passwordHash": "$2a$...", "apiKey": "k",
		"nested": {"password": "p", "keep": 1}, "list": [{"secret": "s", "keep": 2}],
		"status": {"old": "a", "new": "b"}, "token_change": 1
	}`
	got := domain.Present([]byte(stored))
	encoded, _ := json.Marshal(got)
	for _, leaked := range []string{"hunter2", "Tmp-1234", `"abc"`, `"s"`, `"x"`, `"y"`, "$2a$", `"k"`, `"p"`} {
		if strings.Contains(string(encoded), leaked) {
			t.Errorf("%s leaked into %s", leaked, encoded)
		}
	}
	nested := got.Details["nested"].(map[string]any)
	if len(nested) != 1 || nested["keep"] == nil {
		t.Errorf("nested %+v, want only keep", nested)
	}
	if len(got.Changes) != 1 || got.Changes[0].Field != "status" {
		t.Errorf("changes %+v", got.Changes)
	}
}

func TestAnyKeyNamedLikeACredentialIsNeverShown(t *testing.T) {
	keys := []string{
		"tokenHash", "resetToken", "sessionToken", "refreshTokenHash", "Authorization", "otp", "OTP",
		"passcode", "pin", "secretKey", "private_key", "ciphertext", "clientSecretHash", "x-api-key",
		"apiKey", "access.key", "Signing Key", "passwd", "pwd", "passphrase", "credentials",
		"cookieValue", "set-cookie", "totp", "newPasswordConfirmation", "mfa_secret_seed",
	}
	for _, key := range keys {
		stored := `{"` + key + `": "LEAK-VALUE", "kept": "visible"}`
		got := domain.Present([]byte(stored))
		encoded, _ := json.Marshal(got)
		if strings.Contains(string(encoded), "LEAK-VALUE") {
			t.Errorf("%q leaked into %s", key, encoded)
		}
		if got.Details["kept"] != "visible" {
			t.Errorf("a key beside %q was lost: %+v", key, got.Details)
		}
		nested := domain.Present([]byte(`{"outer": {"` + key + `": "LEAK-VALUE", "list": [{"` + key + `": "LEAK-VALUE"}]}}`))
		if encoded, _ := json.Marshal(nested); strings.Contains(string(encoded), "LEAK-VALUE") {
			t.Errorf("%q leaked at depth into %s", key, encoded)
		}
		paired := domain.Present([]byte(`{"` + key + `": {"old": "LEAK-VALUE", "new": "LEAK-VALUE"}}`))
		if len(paired.Changes) != 0 || len(paired.Details) != 0 {
			t.Errorf("%q shows %+v as a change", key, paired)
		}
	}
}

func TestAKeyThatOnlyLooksLikeACredentialIsStillShown(t *testing.T) {
	keys := []string{
		"keyword", "shipping", "mapping", "footprint", "hotkey", "primaryKey", "authorName", "pinned",
		"spinner", "options", "opinion", "monkey", "revoked", "familyId", "focusLossCount", "keyVersion",
	}
	for _, key := range keys {
		got := domain.Present([]byte(`{"` + key + `": "visible"}`))
		if got.Details[key] != "visible" {
			t.Errorf("%q was left out: %+v", key, got.Details)
		}
	}
}

func TestADroppedKeyTakesItsWholeChangeWithIt(t *testing.T) {
	got := domain.Present([]byte(`{"password": {"old": "a", "new": "b"}}`))
	if len(got.Changes) != 0 || len(got.Details) != 0 {
		t.Errorf("shows %+v", got)
	}
}

func TestAnEntryIsItsRecordItsCategoryAndItsPresentation(t *testing.T) {
	id := "0195c4b0-0000-7000-8000-000000000001"
	rec := domain.Record{
		ID: 7, OccurredAt: time.Date(2001, 3, 15, 5, 0, 0, 0, time.UTC), Action: "class.created",
		EntityType: "class", EntityID: &id, Diff: []byte(`{"name": {"old": null, "new": "Lớp 9A"}}`),
	}
	label := "Lớp 9A"
	entry := domain.NewEntry(rec, &label, "https://photos.example/a.png")

	if entry.Category == nil || *entry.Category != domain.Classes {
		t.Errorf("category %v, want classes", entry.Category)
	}
	if entry.Label == nil || *entry.Label != label || entry.AvatarURL == "" {
		t.Errorf("label %v, avatar %q", entry.Label, entry.AvatarURL)
	}
	if len(entry.Changes) != 1 || entry.Changes[0].To != "Lớp 9A" {
		t.Errorf("changes %+v", entry.Changes)
	}
	if ref, ok := rec.Ref(); !ok || ref.Type != "class" || ref.ID != id {
		t.Errorf("ref %+v, %v", ref, ok)
	}
}

func TestAnEntryAboutAnUncategorisedEntityHasNoCategory(t *testing.T) {
	entry := domain.NewEntry(domain.Record{Action: "maintenance.scheduled", EntityType: "maintenance_window"}, nil, "")
	if entry.Category != nil {
		t.Errorf("category %v, want none", *entry.Category)
	}
	if _, ok := entry.Ref(); ok {
		t.Error("an entry with no entity id has a ref")
	}
	if entry.Changes == nil {
		t.Error("changes are nil, want an empty list")
	}
}
