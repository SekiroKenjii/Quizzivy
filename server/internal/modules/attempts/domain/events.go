package domain

// FlushInput carries the events and exactly one credential.
//
// The two paths exist because navigator.sendBeacon cannot set headers, so the
// pagehide flush has nowhere to put an Authorization header and carries a
// token in its body instead (D-03).
type FlushInput struct {
	AttemptID string
	SessionID string
	Events    []Event

	// StudentID is set when the request arrived with a verified access token.
	StudentID   string
	BeaconToken string
}
