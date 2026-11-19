package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"messagefeed/internal/agent"
	"messagefeed/internal/config"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
)

type AgentLLMCallStage string

const (
	AgentLLMCallStageRoute                AgentLLMCallStage = "route"
	AgentLLMCallStagePlanning             AgentLLMCallStage = "planning"
	AgentLLMCallStageFollowupIntent       AgentLLMCallStage = "followup_intent"
	AgentLLMCallStageFollowupAnswer       AgentLLMCallStage = "followup_answer"
	AgentLLMCallStageGeneration           AgentLLMCallStage = "generation"
	AgentLLMCallStageFeedback             AgentLLMCallStage = "feedback"
	AgentLLMCallStageRepair               AgentLLMCallStage = "repair"
	AgentLLMCallStageMemoryClassification AgentLLMCallStage = "memory_classification"
	AgentLLMCallStageRerank               AgentLLMCallStage = "rerank"
	AgentLLMCallStageEvidence             AgentLLMCallStage = "evidence"
	AgentLLMCallPromptVersion                               = "agent-llm-call-v1"
)

type AgentLLMCallContext struct {
	UserID        int64
	SessionID     int64
	TurnID        int64
	ParentRunID   int64
	Stage         AgentLLMCallStage
	PromptVersion string
}

type agentLLMCallContextKey struct{}

func WithAgentLLMCallContext(ctx context.Context, metadata AgentLLMCallContext) context.Context {
	if metadata.PromptVersion == "" {
		metadata.PromptVersion = AgentLLMCallPromptVersion
	}
	return context.WithValue(ctx, agentLLMCallContextKey{}, metadata)
}

func WithAgentLLMRuntimeRunManager(manager *agent.RunManager) AgentLLMRuntimeOption {
	return func(runtime *AgentLLMRuntime) {
		runtime.runManager = manager
	}
}

func WithAgentLLMRuntimeDefaultConfig(configuration config.LLMConfig) AgentLLMRuntimeOption {
	return func(runtime *AgentLLMRuntime) {
		fingerprint := sha256.Sum256([]byte(configuration.Provider + "\x00" + configuration.BaseURL + "\x00" + configuration.Model))
		runtime.defaultModelConfig = domain.AgentJSON{
			"source":     "environment",
			"config_key": "environment:" + hex.EncodeToString(fingerprint[:]),
			"provider":   configuration.Provider,
			"model":      configuration.Model,
		}
	}
}

func agentLLMCallContextFromContext(ctx context.Context) (AgentLLMCallContext, error) {
	if ctx == nil {
		return AgentLLMCallContext{}, agentLLMCallContextError("agent llm call context is missing")
	}
	metadata, ok := ctx.Value(agentLLMCallContextKey{}).(AgentLLMCallContext)
	if !ok || metadata.UserID < 1 || metadata.SessionID < 1 || metadata.TurnID < 1 || metadata.ParentRunID < 0 {
		return AgentLLMCallContext{}, agentLLMCallContextError("agent llm call requires an actual user, session and turn")
	}
	if userID := agentLLMUserIDFromContext(ctx); userID > 0 && userID != metadata.UserID {
		return AgentLLMCallContext{}, agentLLMCallContextError("agent llm call user does not match the authenticated user")
	}
	switch metadata.Stage {
	case AgentLLMCallStageRoute, AgentLLMCallStagePlanning, AgentLLMCallStageFollowupIntent,
		AgentLLMCallStageFollowupAnswer, AgentLLMCallStageGeneration, AgentLLMCallStageFeedback,
		AgentLLMCallStageRepair, AgentLLMCallStageMemoryClassification, AgentLLMCallStageRerank,
		AgentLLMCallStageEvidence:
	default:
		return AgentLLMCallContext{}, agentLLMCallContextError("agent llm call stage is invalid")
	}
	if strings.TrimSpace(metadata.PromptVersion) == "" {
		return AgentLLMCallContext{}, agentLLMCallContextError("agent llm prompt version is missing")
	}
	return metadata, nil
}

func agentLLMCallContextError(message string) error {
	return domain.NewAppError(domain.ErrorKindInvalidInput, "agent_llm_call_context_invalid", message, "service.agent_llm_runtime.chat", false, nil)
}

type agentLLMResolvedClient struct {
	client        AgentConversationLLM
	configuration domain.AgentJSON
	configID      int64
	modelKey      string
}

func (r *AgentLLMRuntime) clientForUser(ctx context.Context, userID int64) (agentLLMResolvedClient, error) {
	if r == nil {
		return agentLLMResolvedClient{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_unavailable", "llm runtime is unavailable", "service.agent_llm_runtime.chat", true, nil)
	}
	selection, selected := ctx.Value(agentModelSelectionKey{}).(domain.AgentModelSelection)
	if selected && (selection.UserID != userID || selection.ConfigID < 1 || selection.Model == "") {
		return agentLLMResolvedClient{}, fmt.Errorf("%w: 模型选择与当前用户不匹配", domain.ErrInvalidInput)
	}
	if r.store != nil {
		var configuration domain.AgentLLMProviderConfig
		var err error
		if selected {
			configuration, err = r.store.GetAgentLLMProviderConfig(ctx, userID, selection.ConfigID)
		} else {
			configuration, err = r.store.GetDefaultAgentLLMProviderConfig(ctx, userID)
		}
		if selected && err != nil {
			return agentLLMResolvedClient{}, err
		}
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return agentLLMResolvedClient{}, err
		}
		if err == nil {
			if selected {
				if !configuration.Enabled {
					return agentLLMResolvedClient{}, fmt.Errorf("%w: 模型供应商已停用", domain.ErrInvalidInput)
				}
				store, ok := r.store.(agentLLMModelStore)
				if !ok {
					return agentLLMResolvedClient{}, fmt.Errorf("模型目录存储不可用")
				}
				models, err := store.ListAgentLLMModels(ctx, userID, selection.ConfigID)
				if err != nil {
					return agentLLMResolvedClient{}, err
				}
				allowed := false
				for _, model := range models {
					if model.Model == selection.Model && model.Enabled {
						allowed = true
						break
					}
				}
				if !allowed {
					return agentLLMResolvedClient{}, fmt.Errorf("%w: 选择的模型尚未启用", domain.ErrInvalidInput)
				}
				configuration.Model = selection.Model
			}
			client, err := r.clientForConfig(configuration)
			if err != nil {
				return agentLLMResolvedClient{}, err
			}
			return agentLLMResolvedClient{
				client:   client,
				configID: configuration.ID,
				modelKey: configuration.Provider + "/" + configuration.Model,
				configuration: domain.AgentJSON{
					"source":          "user_provider_config",
					"config_id":       configuration.ID,
					"config_key":      agentLLMRuntimeCacheKey(configuration),
					"config_version":  configuration.UpdatedAt.UTC().Format(time.RFC3339Nano),
					"provider":        configuration.Provider,
					"model":           configuration.Model,
					"protocol_mode":   string(configuration.ProtocolMode),
					"timeout_seconds": configuration.TimeoutSeconds,
					"max_attempts":    configuration.MaxRetries,
				},
			}, nil
		}
	}
	if selected {
		return agentLLMResolvedClient{}, fmt.Errorf("模型选择的供应商不可用")
	}
	if r.defaultClient == nil {
		return agentLLMResolvedClient{}, domain.NewAppError(domain.ErrorKindUnavailable, "llm_unavailable", "llm client is unavailable", "service.agent_llm_runtime.chat", true, nil)
	}
	provider, _ := r.defaultModelConfig["provider"].(string)
	model, _ := r.defaultModelConfig["model"].(string)
	return agentLLMResolvedClient{client: r.defaultClient, configuration: r.defaultModelConfig, modelKey: provider + "/" + model}, nil
}

func (r *AgentLLMRuntime) chatWithCallTrace(ctx context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	metadata, err := agentLLMCallContextFromContext(ctx)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if r == nil || r.runManager == nil {
		return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_trace_unavailable", "agent llm run manager is unavailable", "service.agent_llm_runtime.chat", true, nil)
	}
	resolved, err := r.clientForUser(ctx, metadata.UserID)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if selection, selected := ctx.Value(agentModelSelectionKey{}).(domain.AgentModelSelection); selected {
		request.ReasoningEffort = selection.ReasoningEffort
	}
	content := agentLLMRequestSnapshot(metadata, request, resolved.configuration)
	if _, err := json.Marshal(content); err != nil {
		return llm.ChatResponse{}, err
	}
	runInput := agent.CreateRunInput{
		ParentRunID: metadata.ParentRunID,
		SessionID:   metadata.SessionID,
		TurnID:      metadata.TurnID,
		TaskPacket: domain.AgentJSON{
			"user_id": metadata.UserID,
			"stage":   string(metadata.Stage),
		},
		ModelKey: resolved.modelKey,
		TraceID:  observability.TraceID(ctx),
	}
	var controller domain.AgentRun
	if metadata.ParentRunID == 0 {
		controller, err = r.runManager.CreateControllerRun(ctx, runInput)
		if err != nil {
			return llm.ChatResponse{}, err
		}
		if controller.ID < 1 {
			return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_trace_unavailable", "模型调用的 controller 尚未持久化。", "service.agent_llm_runtime.chat", true, nil)
		}
		runInput.ParentRunID = controller.ID
	}
	run, err := r.runManager.CreateExecutorRun(ctx, runInput)
	if err != nil {
		if controller.ID > 0 {
			_, closeErr := r.runManager.FailRun(ctx, controller, err)
			err = errors.Join(err, closeErr)
		}
		return llm.ChatResponse{}, err
	}
	if run.ID < 1 {
		return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_llm_trace_unavailable", "agent llm call run was not persisted", "service.agent_llm_runtime.chat", true, nil)
	}
	finishRuns := func(finishContext context.Context, cause error, resultRef string) error {
		var finishErr error
		for _, target := range []domain.AgentRun{run, controller} {
			if target.ID < 1 {
				continue
			}
			var updateErr error
			if cause != nil {
				_, updateErr = r.runManager.FailRun(finishContext, target, cause)
			} else {
				_, updateErr = r.runManager.CompleteRun(finishContext, target, resultRef)
			}
			finishErr = errors.Join(finishErr, updateErr)
		}
		return finishErr
	}
	requestTraceCtx, stopRequestTrace := context.WithTimeout(ctx, 500*time.Millisecond)
	requestTrace, err := r.runManager.SaveContextTrace(requestTraceCtx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       "llm_request",
		PromptVersion:   metadata.PromptVersion,
		ModelKey:        resolved.modelKey,
		Content:         content,
		RedactionStatus: "not_required",
	})
	stopRequestTrace()
	if err != nil || requestTrace.ID < 1 {
		metrics.AgentLLMTraceWriteFailuresTotal.WithLabelValues("request_snapshot").Inc()
	}
	record, flush := r.streamRecorder(ctx, metadata, run.ID, resolved.modelKey)
	request.OnEvent = record
	response, callErr := resolved.client.Chat(ctx, request)
	if callErr == nil && (response.IncompleteReason != "" || response.FinishReason == "length") {
		callErr = domain.NewAppError(domain.ErrorKindUnavailable, "llm_response_incomplete", "agent llm response is incomplete", "service.agent_llm_runtime.chat", true, nil)
	}
	if callErr == nil && request.OutputSchema != nil {
		_, callErr = request.OutputSchema.Validate(response.Content)
	}
	traceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	flush(traceCtx)
	responseContent := agentLLMResponseSnapshot(metadata, response, requestTrace, callErr)
	responseContent["request_dispatched"] = true
	modelKey := resolved.modelKey
	if response.Provider != "" && response.Model != "" {
		modelKey = response.Provider + "/" + response.Model
	}
	tokens := 0
	if response.Usage != nil {
		tokens = response.Usage.TotalTokens
	}
	responseTrace, traceErr := r.runManager.SaveContextTrace(traceCtx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       "llm_response",
		PromptVersion:   metadata.PromptVersion,
		ModelKey:        modelKey,
		Content:         responseContent,
		RedactionStatus: "not_required",
		TokenEstimate:   tokens,
	})
	if traceErr == nil && responseTrace.ID < 1 {
		traceErr = fmt.Errorf("agent llm response snapshot was not persisted")
	}
	if traceErr != nil {
		metrics.AgentLLMTraceWriteFailuresTotal.WithLabelValues("response_snapshot").Inc()
	}
	run.ModelKey = modelKey
	resultRef := ""
	if responseTrace.ID > 0 {
		resultRef = fmt.Sprintf("llm_response:%d", responseTrace.ID)
	}
	if err := finishRuns(traceCtx, callErr, resultRef); err != nil {
		metrics.AgentLLMTraceWriteFailuresTotal.WithLabelValues("run_finish").Inc()
	}
	if callErr == nil && resolved.configID > 0 {
		if err := r.store.MarkAgentLLMProviderConfigUsed(traceCtx, metadata.UserID, resolved.configID, r.now().UTC()); err != nil {
			metrics.AgentLLMTraceWriteFailuresTotal.WithLabelValues("configuration_used").Inc()
		}
	}
	return response, callErr
}

func agentLLMRequestSnapshot(metadata AgentLLMCallContext, request llm.ChatRequest, configuration domain.AgentJSON) domain.AgentJSON {
	return domain.AgentJSON{
		"stage":               string(metadata.Stage),
		"user_id":             metadata.UserID,
		"session_id":          metadata.SessionID,
		"turn_id":             metadata.TurnID,
		"parent_run_id":       metadata.ParentRunID,
		"prompt_version":      metadata.PromptVersion,
		"model_configuration": configuration,
		"request": domain.AgentJSON{
			"messages":          request.Messages,
			"tools":             request.Tools,
			"tool_choice":       request.ToolChoice,
			"temperature":       request.Temperature,
			"max_tokens":        request.MaxTokens,
			"reasoning_effort":  request.ReasoningEffort,
			"output_schema":     request.OutputSchema,
			"schema_local_only": request.SchemaLocalOnly,
		},
	}
}

func agentLLMResponseSnapshot(metadata AgentLLMCallContext, response llm.ChatResponse, requestTrace domain.AgentRunContextTrace, callErr error) domain.AgentJSON {
	content := domain.AgentJSON{
		"stage":                string(metadata.Stage),
		"request_trace_id":     requestTrace.ID,
		"request_content_hash": requestTrace.ContentHash,
		"response_id":          response.ResponseID,
		"protocol":             response.Protocol,
		"response_items":       response.ResponseItems,
		"reasoning_content":    response.ReasoningContent,
		"provider":             response.Provider,
		"model":                response.Model,
		"finish_reason":        response.FinishReason,
		"incomplete_reason":    response.IncompleteReason,
		"usage":                response.Usage,
		"content":              response.Content,
		"tool_calls":           response.ToolCalls,
	}
	if callErr != nil {
		content["error"] = callErr.Error()
	}
	return content
}
