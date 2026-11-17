package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/service"
	"net/http"
	"time"
)

type agentModelsService interface {
	DiscoverModels(context.Context, service.CurrentAuth, int64) (service.AgentProviderModels, error)
	SaveModels(context.Context, service.CurrentAuth, int64, service.AgentProviderModelsInput) error
	SelectableModels(context.Context, service.CurrentAuth) ([]domain.AgentLLMModel, error)
	GuestModels(context.Context) ([]domain.AgentLLMModel, error)
	GuestChat(context.Context, string, service.AgentGuestChatInput, func(llm.StreamEvent) error) (service.AgentGuestChatResult, error)
	GuestHistory(context.Context, string) (service.AgentGuestHistory, error)
}

func registerAgentModelsRoutes(private, public *gin.RouterGroup, configService agentLLMConfigService) {
	models, ok := configService.(agentModelsService)
	if !ok {
		return
	}
	private.GET("/agent/models", func(c *gin.Context) {
		result, err := models.SelectableModels(c.Request.Context(), currentAuth(c))
		if err != nil {
			RenderError(c, err, "读取可选模型失败")
			return
		}
		Success(c, result)
	})
	private.GET("/agent/llm-provider-configs/:id/models", func(c *gin.Context) {
		id, ok := parseAgentLLMConfigID(c)
		if !ok {
			return
		}
		result, err := models.DiscoverModels(c.Request.Context(), currentAuth(c), id)
		if err != nil {
			RenderError(c, err, "同步供应商模型目录失败")
			return
		}
		Success(c, result)
	})
	private.PUT("/agent/llm-provider-configs/:id/models", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		id, ok := parseAgentLLMConfigID(c)
		if !ok {
			return
		}
		var input service.AgentProviderModelsInput
		if err := c.ShouldBindJSON(&input); err != nil {
			Error(c, http.StatusBadRequest, http.StatusBadRequest, "模型选择参数无效")
			return
		}
		if err := models.SaveModels(c.Request.Context(), currentAuth(c), id, input); err != nil {
			RenderError(c, err, "保存模型选择失败")
			return
		}
		Success(c, gin.H{"saved": true})
	})
	public.GET("/public/agent/models", func(c *gin.Context) {
		result, err := models.GuestModels(c.Request.Context())
		if err != nil {
			RenderError(c, err, "读取游客模型失败")
			return
		}
		Success(c, result)
	})
	public.GET("/public/agent/history", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		result, err := models.GuestHistory(c.Request.Context(), c.ClientIP())
		if err != nil {
			RenderError(c, err, "读取游客历史失败")
			return
		}
		Success(c, result)
	})
	public.POST("/public/agent/chat", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 96<<10)
		var input service.AgentGuestChatInput
		if err := c.ShouldBindJSON(&input); err != nil {
			Error(c, http.StatusBadRequest, http.StatusBadRequest, "游客对话参数无效")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
		defer cancel()
		started := false
		write := func(kind string, value any) error {
			encoded, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if !started {
				c.Header("Content-Type", "text/event-stream")
				c.Header("Cache-Control", "no-store")
				c.Header("X-Accel-Buffering", "no")
				started = true
			}
			if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", kind, encoded); err != nil {
				return err
			}
			c.Writer.Flush()
			return nil
		}
		response, err := models.GuestChat(ctx, c.ClientIP(), input, func(event llm.StreamEvent) error {
			return write("delta", gin.H{"kind": event.Kind, "item_id": event.ItemID, "delta": event.Delta, "replace": event.Replace})
		})
		if err != nil {
			if started {
				_ = write("failed", gin.H{"message": "供应商模型未完成回答，请重新发送"})
			} else {
				RenderError(c, err, "游客对话请求失败")
			}
			return
		}
		_ = write("completed", gin.H{"content": response.Content, "model": response.Model, "usage": response.Usage, "conversation_id": response.ConversationID, "turn_id": response.TurnID})
	})
}
