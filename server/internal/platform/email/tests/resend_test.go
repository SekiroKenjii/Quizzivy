package email_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/platform/email"
)

func sampleMessage() email.Message {
	return email.Message{
		To: "an@example.com", Subject: "Chủ đề", HTML: "<p>x</p>", Text: "x", ReplyTo: "front@example.com",
		IdempotencyKey: "0192a7c0-0000-7000-8000-000000000001",
		Tags:           map[string]string{"template": "account_invite"},
	}
}

func TestResendPostsTheMessageWithItsHeadersAndAnswersTheProviderID(t *testing.T) {
	var gotHeader http.Header
	var gotBody map[string]any
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader, gotMethod = r.Header.Clone(), r.Method
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("body: %v", err)
		}
		_, _ = io.WriteString(w, `{"id":"re_123"}`)
	}))
	defer server.Close()

	id, err := email.Resend{APIKey: "re_key", From: "Quizzivy <no-reply@quizzivy.com>", Endpoint: server.URL}.Send(context.Background(), sampleMessage())
	if err != nil || id != "re_123" {
		t.Fatalf("send: %q %v", id, err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method %s", gotMethod)
	}
	for header, want := range map[string]string{
		"Authorization": "Bearer re_key", "Idempotency-Key": "0192a7c0-0000-7000-8000-000000000001", "Content-Type": "application/json",
	} {
		if got := gotHeader.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if gotBody["from"] != "Quizzivy <no-reply@quizzivy.com>" || gotBody["subject"] != "Chủ đề" || gotBody["html"] != "<p>x</p>" || gotBody["text"] != "x" {
		t.Errorf("body %v", gotBody)
	}
	if to, _ := gotBody["to"].([]any); len(to) != 1 || to[0] != "an@example.com" {
		t.Errorf("to %v", gotBody["to"])
	}
	if rt, _ := gotBody["reply_to"].([]any); len(rt) != 1 || rt[0] != "front@example.com" {
		t.Errorf("reply_to %v", gotBody["reply_to"])
	}
	if tags, _ := gotBody["tags"].([]any); len(tags) != 1 {
		t.Errorf("tags %v", gotBody["tags"])
	}
	for _, off := range []string{"track_clicks", "track_opens", "click_tracking", "open_tracking"} {
		if _, ok := gotBody[off]; ok {
			t.Errorf("the body sets %s; tracking is a domain setting that stays off", off)
		}
	}
}

func TestResendClassifiesFailures(t *testing.T) {
	for _, c := range []struct {
		status    int
		body      string
		transient bool
		code      string
	}{
		{http.StatusTooManyRequests, `{"name":"rate_limit_exceeded"}`, true, "http_429:rate_limit_exceeded"},
		{http.StatusInternalServerError, ``, true, "http_500"},
		{http.StatusBadGateway, `<html>`, true, "http_502"},
		{http.StatusServiceUnavailable, `{"name":"x"}`, true, "http_503:x"},
		{http.StatusBadRequest, `{"name":"validation_error","message":"to an@example.com"}`, false, "http_400:validation_error"},
		{http.StatusUnauthorized, `{"name":"missing_api_key"}`, false, "http_401:missing_api_key"},
		{http.StatusUnprocessableEntity, `{"name":"Bad Name With Spaces"}`, false, "http_422"},
		{http.StatusForbidden, ``, false, "http_403"},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		}))
		_, err := email.Resend{APIKey: "k", From: "f@x.com", Endpoint: server.URL}.Send(context.Background(), sampleMessage())
		server.Close()
		var failure *email.SendError
		if !errors.As(err, &failure) {
			t.Fatalf("%d: %v is not a SendError", c.status, err)
		}
		if got := errors.Is(err, email.ErrTransient); got != c.transient || errors.Is(err, email.ErrPermanent) == c.transient {
			t.Errorf("%d: transient = %v, want %v", c.status, got, c.transient)
		}
		if failure.Code != c.code {
			t.Errorf("%d: code %q, want %q", c.status, failure.Code, c.code)
		}
		if strings.Contains(err.Error(), "example.com") {
			t.Errorf("%d: the error repeats the message or the key: %v", c.status, err)
		}
	}
}

func TestResendTreatsAConnectionFailureAndAnUnreadableSuccessAsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `not json`) }))
	_, err := email.Resend{APIKey: "k", From: "f@x.com", Endpoint: server.URL}.Send(context.Background(), sampleMessage())
	server.Close()
	if !errors.Is(err, email.ErrTransient) {
		t.Errorf("an unreadable 200: %v", err)
	}
	_, err = email.Resend{APIKey: "k", From: "f@x.com", Endpoint: server.URL}.Send(context.Background(), sampleMessage())
	if !errors.Is(err, email.ErrTransient) {
		t.Errorf("a closed server: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("the error carries the address: %v", err)
	}
}
