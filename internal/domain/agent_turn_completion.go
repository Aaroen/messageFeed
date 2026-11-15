package domain

import "time"

type AgentTurnCompletion struct {
	Turn               AgentTurn
	Plan               AgentPlan
	ControllerRunID    int64
	WorkerID           string
	TranscriptMetadata AgentJSON
	CompletedAt        time.Time
}

type AgentTurnCompletionResult struct {
	Turn           AgentTurn
	Plan           AgentPlan
	InboundMessage AgentInboundMessage
}
