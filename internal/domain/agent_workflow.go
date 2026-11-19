package domain

type AgentWorkflow struct {
	ID           int64
	UserID       int64
	SourceTurnID int64
	Name         string
	Status       string
	Steps        []AgentWorkflowStep `gorm:"serializer:json;type:jsonb"`
}

type AgentWorkflowStep struct {
	Capability string    `json:"capability"`
	Version    int       `json:"version"`
	Arguments  AgentJSON `json:"arguments"`
}
