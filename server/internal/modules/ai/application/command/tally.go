package command

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"quizzivy/internal/modules/ai/application/ports"
)

const maxRequestIDs = 200

// Billed reports whether a call that ended in err was charged by the provider: one that completed, one it refused (ports.ErrRefused) and one whose answer was not a complete JSON object (ports.ErrInvalidResponse). Every other failure billed nothing, as far as anyone can tell. Finish and Tally apply this one rule.
func Billed(err error) bool {
	return err == nil || errors.Is(err, ports.ErrRefused) || errors.Is(err, ports.ErrInvalidResponse)
}

// Tally sums what the calls made under one reservation were billed, so that a request of several calls ends with one Finish of the sum and not with one settlement per call. A call that billed nothing adds nothing. It is safe for concurrent use, and its zero value is an empty tally.
type Tally struct {
	mu         sync.Mutex
	usage      ports.Usage
	requestIDs []string
}

// Add records a call's response and error. It counts the usage of a billed call (see Billed) and keeps its request id; any other call is left out, whatever usage its response carries.
func (t *Tally) Add(response ports.Response, err error) {
	if !Billed(err) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.InputTokens += response.Usage.InputTokens
	t.usage.OutputTokens += response.Usage.OutputTokens
	t.usage.CacheReadTokens += response.Usage.CacheReadTokens
	t.usage.CacheWriteTokens += response.Usage.CacheWriteTokens
	if response.RequestID != "" {
		t.requestIDs = append(t.requestIDs, response.RequestID)
	}
}

// Response is the sum as the response Finish takes: the total usage of the billed calls, and their request ids in order, separated by commas. The ledger keeps at most 200 characters of request id, so ids that do not fit whole are left out of it (the pass logs them all); the id of one call is never cut.
func (t *Tally) Response() ports.Response {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := slices.Clone(t.requestIDs)
	slices.Sort(ids)
	var joined strings.Builder
	for _, id := range ids {
		size := len(id)
		if joined.Len() > 0 {
			size++
		}
		if joined.Len()+size > maxRequestIDs {
			break
		}
		if joined.Len() > 0 {
			joined.WriteByte(',')
		}
		joined.WriteString(id)
	}
	return ports.Response{Usage: t.usage, RequestID: joined.String()}
}
