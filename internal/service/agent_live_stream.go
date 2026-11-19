package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"time"
)

func (r *AgentLLMRuntime) streamRecorder(ctx context.Context, metadata AgentLLMCallContext, runID int64, modelKey string) (func(llm.StreamEvent) error, func(context.Context)) {
	buffer := make([]domain.AgentStreamEvent, 0, 8)
	var sequence int64
	frozen := 0
	lastFlush := time.Now().Add(-time.Second)
	flush := func(flushContext context.Context) {
		if len(buffer) == 0 {
			return
		}
		lastFlush = time.Now()
		frozen = len(buffer)
		writeCtx, cancel := context.WithTimeout(flushContext, 300*time.Millisecond)
		defer cancel()
		if err := r.runManager.AppendStreamEvents(writeCtx, buffer); err != nil {
			metrics.AgentStreamWriteFailuresTotal.Inc()
			return
		}
		buffer = buffer[:0]
		frozen = 0
	}
	record := func(event llm.StreamEvent) error {
		entry := domain.AgentStreamEvent{UserID: metadata.UserID, SessionID: metadata.SessionID, TurnID: metadata.TurnID, RunID: runID, Kind: event.Kind, ItemID: event.ItemID, Name: event.Name, Delta: event.Delta, Metadata: domain.AgentJSON{"stage": string(metadata.Stage), "model_key": modelKey, "phase": event.Phase, "replace": event.Replace}, CreatedAt: r.now().UTC()}
		if len(buffer) > frozen {
			last := &buffer[len(buffer)-1]
			if !event.Replace && last.Metadata["replace"] == false && last.Kind == entry.Kind && last.ItemID == entry.ItemID && last.Name == entry.Name && last.Metadata["phase"] == event.Phase && len(last.Delta)+len(entry.Delta) <= 64*1024 {
				last.Delta += entry.Delta
				entry.Kind = ""
			}
		}
		if entry.Kind != "" && len(buffer) < 64 && len(entry.Delta) <= 64*1024 {
			sequence++
			serial := sequence
			entry.Sequence = &serial
			buffer = append(buffer, entry)
		}
		if time.Since(lastFlush) >= 200*time.Millisecond {
			flush(ctx)
		}
		return nil
	}
	return record, flush
}
