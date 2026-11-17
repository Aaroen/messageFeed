package domain

import "time"

type AgentStreamEvent struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	SessionID int64     `json:"session_id"`
	TurnID    int64     `json:"turn_id"`
	RunID     int64     `json:"run_id"`
	Kind      string    `json:"kind"`
	ItemID    string    `json:"item_id"`
	Name      string    `json:"name"`
	Delta     string    `json:"delta"`
	Metadata  AgentJSON `json:"metadata" gorm:"column:metadata_json;serializer:json;type:jsonb"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentWorkspaceData struct {
	Session AgentSession
	Turns   []AgentTurn
	Runs    []AgentRun
	Plans   []AgentPlan
	Events  []AgentStreamEvent
	Cursor  int64
	HasMore bool
}
