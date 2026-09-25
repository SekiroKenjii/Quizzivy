package domain

// GroupPlayInput identifies one listening gesture and the writable session reporting it.
type GroupPlayInput struct {
	AttemptID   string
	StudentID   string
	SessionID   string
	RecordingID string
	PlayID      string
}

// GroupPlays acknowledges a gesture with the current count for its frozen recording.
type GroupPlays struct {
	PlayID   string
	Plays    int
	MaxPlays *int
}
