package handler

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type agentWorkspaceService interface {
	GetWorkspace(context.Context, service.CurrentAuth, int64, int64) (service.AgentWorkspaceResponse, error)
	GetWorkspaceState(context.Context, service.CurrentAuth, int64) (service.AgentWorkspaceResponse, error)
	GetWorkspaceEvents(context.Context, service.CurrentAuth, int64, int64) ([]domain.AgentStreamEvent, error)
}

func (h agentSessionHandler) cancelTurn(c *gin.Context) {
	turnID, err := strconv.ParseInt(c.Param("turn_id"), 10, 64)
	if err != nil || turnID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid agent turn id")
		return
	}
	control, ok := h.service.(interface {
		CancelWorkspaceTurn(context.Context, service.CurrentAuth, int64) (service.AgentTurnResponse, error)
	})
	if !ok {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent cancellation unavailable")
		return
	}
	turn, err := control.CancelWorkspaceTurn(c.Request.Context(), currentAuth(c), turnID)
	if err != nil {
		RenderError(c, err, "cancel agent turn failed")
		return
	}
	Success(c, turn)
}

func (h agentSessionHandler) workspace(c *gin.Context) {
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	workspace, ok := h.service.(agentWorkspaceService)
	if !ok {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent workspace unavailable")
		return
	}
	before, err := strconv.ParseInt(c.DefaultQuery("before_turn_id", "0"), 10, 64)
	if err != nil || before < 0 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid turn cursor")
		return
	}
	result, err := workspace.GetWorkspace(c.Request.Context(), currentAuth(c), sessionID, before)
	if err != nil {
		RenderError(c, err, "load agent workspace failed")
		return
	}
	Success(c, result)
}

func (h agentSessionHandler) workspaceStream(c *gin.Context) {
	sessionID, ok := parseAgentSessionID(c)
	if !ok {
		return
	}
	workspace, ok := h.service.(agentWorkspaceService)
	if !ok {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "agent workspace unavailable")
		return
	}
	cursorValue := c.GetHeader("Last-Event-ID")
	if cursorValue == "" {
		cursorValue = c.DefaultQuery("after_id", "0")
	}
	cursor, err := strconv.ParseInt(cursorValue, 10, 64)
	if err != nil || cursor < 0 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid event cursor")
		return
	}
	initial, err := workspace.GetWorkspace(c.Request.Context(), currentAuth(c), sessionID, 0)
	if err != nil {
		RenderError(c, err, "load agent workspace failed")
		return
	}
	if cursor > initial.Cursor {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "event cursor exceeds session history")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	write := func(kind string, id int64, data any) error {
		return writeAgentProgressSSE(c.Writer, flusher, kind, strconv.FormatInt(id, 10), data)
	}
	if cursor == 0 {
		cursor = initial.Cursor
		if err := write("snapshot", cursor, initial); err != nil {
			return
		}
	}
	if c.Query("once") == "true" {
		for cursor < initial.Cursor {
			events, err := workspace.GetWorkspaceEvents(c.Request.Context(), currentAuth(c), sessionID, cursor)
			if err != nil {
				_ = write("stream_error", cursor, gin.H{"message": "执行事件读取失败"})
				return
			}
			if len(events) == 0 {
				return
			}
			for _, event := range events {
				if event.ID > initial.Cursor {
					return
				}
				if err := write("delta", event.ID, event); err != nil {
					return
				}
				cursor = event.ID
			}
		}
		return
	}
	eventsTicker := time.NewTicker(300 * time.Millisecond)
	snapshotTicker := time.NewTicker(2 * time.Second)
	defer eventsTicker.Stop()
	defer snapshotTicker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-eventsTicker.C:
			events, err := workspace.GetWorkspaceEvents(c.Request.Context(), currentAuth(c), sessionID, cursor)
			if err != nil {
				_ = write("stream_error", cursor, gin.H{"message": "执行事件读取失败"})
				return
			}
			for _, event := range events {
				if err := write("delta", event.ID, event); err != nil {
					return
				}
				cursor = event.ID
			}
		case <-snapshotTicker.C:
			result, err := workspace.GetWorkspaceState(c.Request.Context(), currentAuth(c), sessionID)
			if err != nil {
				_ = write("stream_error", cursor, gin.H{"message": "会话状态读取失败"})
				return
			}
			if err := write("state", cursor, result); err != nil {
				return
			}
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
