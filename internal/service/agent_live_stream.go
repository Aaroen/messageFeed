package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"time"
)

func (r *AgentLLMRuntime) streamRecorder(ctx context.Context, metadata AgentLLMCallContext, runID int64, modelKey string) (func(llm.StreamEvent) error, func(context.Context) error) {
	buffer := make([]domain.AgentStreamEvent, 0, 8)
	lastFlush := time.Now().Add(-time.Second)
	flush := func(flushContext context.Context) error {
		if len(buffer) == 0 {
			return nil
		}
		if err := r.runManager.AppendStreamEvents(flushContext, buffer); err != nil {
			return err
		}
		buffer = buffer[:0]
		lastFlush = time.Now()
		return nil
	}
	record := func(event llm.StreamEvent) error {
		entry := domain.AgentStreamEvent{UserID: metadata.UserID, SessionID: metadata.SessionID, TurnID: metadata.TurnID, RunID: runID, Kind: event.Kind, ItemID: event.ItemID, Name: event.Name, Delta: event.Delta, Metadata: domain.AgentJSON{"stage": string(metadata.Stage), "model_key": modelKey, "phase": event.Phase, "replace": event.Replace}, CreatedAt: r.now().UTC()}
		if len(buffer) > 0 {
			last := &buffer[len(buffer)-1]
			if !event.Replace && last.Metadata["replace"] == false && last.Kind == entry.Kind && last.ItemID == entry.ItemID && last.Name == entry.Name {
				last.Delta += entry.Delta
			} else {
				buffer = append(buffer, entry)
			}
		} else {
			buffer = append(buffer, entry)
		}
		if time.Since(lastFlush) >= 200*time.Millisecond {
			return flush(ctx)
		}
		return nil
	}
	return record, flush
}
