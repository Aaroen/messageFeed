package domain

import "time"

type AgentTurnCompletion struct {
	Turn               AgentTurn
	Plan               AgentPlan
	ControllerRunID    int64
	WorkerID           string
	TranscriptMetadata AgentJSON
	CompletedAt        time.Time
	QueueEffects       bool
	DeliveryPlanID     int64
}

type AgentTurnEffect struct {
	TurnID    int64
	UserID    int64
	Operation string
	PlanID    int64
	Attempt   int
}

type AgentTurnCompletionResult struct {
	Turn           AgentTurn
	Plan           AgentPlan
	InboundMessage AgentInboundMessage
}
