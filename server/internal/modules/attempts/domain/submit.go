package domain

// Reason records how an attempt ended. It is the contract's `reason`, kept out
// of the status column: status says what still has to happen to the attempt,
// this says what stopped it.
type Reason string

const (
	Manual       Reason = "manual"
	TimerExpired Reason = "timer_expired"
	AutoSubmit   Reason = "auto_submit"
)
