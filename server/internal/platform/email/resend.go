package email

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// ResendEndpoint is where Resend accepts a message.
const ResendEndpoint = "https://api.resend.com/emails"

const (
	resendTimeout  = 15 * time.Second
	maxAnswerBytes = 64 << 10
)

var (
	errorName = regexp.MustCompile(`^[a-z_]{1,40}$`)
	tagValue  = regexp.MustCompile(`[^A-Za-z0-9_-]`)
)

// Resend sends through Resend's HTTP API with one authenticated POST per
// message. 429, 5xx and a failed connection are transient; any other 4xx is
// permanent. Endpoint and Client are for tests; empty and nil mean
// ResendEndpoint and a client with a 15-second timeout.
type Resend struct {
	APIKey   string
	From     string
	Endpoint string
	Client   *http.Client
}

type resendTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type resendBody struct {
	From    string      `json:"from"`
	To      []string    `json:"to"`
	Subject string      `json:"subject"`
	HTML    string      `json:"html"`
	Text    string      `json:"text"`
	ReplyTo []string    `json:"reply_to,omitempty"`
	Tags    []resendTag `json:"tags,omitempty"`
}

// Send posts m and answers Resend's id for it.
func (r Resend) Send(ctx context.Context, m Message) (string, error) {
	body := resendBody{From: r.From, To: []string{m.To}, Subject: m.Subject, HTML: m.HTML, Text: m.Text}
	if m.ReplyTo != "" {
		body.ReplyTo = []string{m.ReplyTo}
	}
	names := make([]string, 0, len(m.Tags))
	for name := range m.Tags {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body.Tags = append(body.Tags, resendTag{Name: tagValue.ReplaceAllString(name, "_"), Value: tagValue.ReplaceAllString(m.Tags[name], "_")})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", &SendError{Kind: ErrPermanent, Code: "encode"}
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = ResendEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", &SendError{Kind: ErrPermanent, Code: "request"}
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: resendTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", &SendError{Kind: ErrTransient, Code: "network"}
	}
	defer func() { _ = res.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(res.Body, maxAnswerBytes))
	if err != nil && res.StatusCode < 300 {
		return "", &SendError{Kind: ErrTransient, Code: "network"}
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		var sent struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(answer, &sent) != nil || sent.ID == "" {
			return "", &SendError{Kind: ErrTransient, Code: "answer_unreadable"}
		}
		return sent.ID, nil
	}
	return "", classify(res.StatusCode, answer)
}

func classify(status int, answer []byte) *SendError {
	code := "http_" + strconv.Itoa(status)
	var failure struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(answer, &failure) == nil && errorName.MatchString(failure.Name) {
		code += ":" + failure.Name
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		return &SendError{Kind: ErrTransient, Code: code}
	}
	return &SendError{Kind: ErrPermanent, Code: code}
}
