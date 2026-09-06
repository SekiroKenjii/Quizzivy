package domain

// The managers are stateless; callers reach them through these values.
var (
	Grading       GradingManager
	Deal          DealManager
	Timelines     TimelineManager
	Interventions InterventionManager
)
