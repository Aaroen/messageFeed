package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strings"
)

const mainAgentPlanSpecMaxAttempts = 2

// mainAgentPlanSpecResult 保存主 Agent 规划模型的可审计结果。
// service 层后续会把其中的 PlanSpec 交给 planner 转为持久化计划。
type mainAgentPlanSpecResult struct {
	Spec      agent.PlanSpec
	Provider  string
	Model     string
	Raw       string
	Attempts  int
	Validated bool
}

// mainAgentPlanSpecJSON 对应模型返回的 JSON 契约。
// 该结构只承担反序列化职责，转换为 agent.PlanSpec 后再进入领域层。
type mainAgentPlanSpecJSON struct {
	Goal                   string                             `json:"goal"`
	Intent                 string                             `json:"intent"`
	TaskType               string                             `json:"task_type"`
	Complexity             string                             `json:"complexity"`
	RequiresSubAgent       bool                               `json:"requires_sub_agent"`
	DirectAnswerAllowed    bool                               `json:"direct_answer_allowed"`
	RequiredCapabilities   []string                           `json:"required_capabilities"`
	Subtasks               []mainAgentPlanSubtaskJSON         `json:"subtasks"`
	EvidenceRequirements   []mainAgentEvidenceRequirementJSON `json:"evidence_requirements"`
	MaxIterations          int                                `json:"max_iterations"`
	FinalAnswerConstraints []string                           `json:"final_answer_constraints"`
	Metadata               map[string]any                     `json:"metadata"`
}

// mainAgentPlanSubtaskJSON 描述模型拆出的一个子 Agent 执行单元。
// capability_keys 只能引用后端注册表中的能力，后续会再次校验。
type mainAgentPlanSubtaskJSON struct {
	Title           string   `json:"title"`
	Prompt          string   `json:"prompt"`
	ContextSummary  string   `json:"context_summary"`
	CapabilityKeys  []string `json:"capability_keys"`
	ExpectedOutput  string   `json:"expected_output"`
	FailureStrategy string   `json:"failure_strategy"`
	MaxRetries      int      `json:"max_retries"`
}

// mainAgentEvidenceRequirementJSON 描述模型认为最终回答需要满足的证据条件。
// 当前阶段只持久化到计划元数据，后续质量评估可继续复用。
type mainAgentEvidenceRequirementJSON struct {
	EvidenceType string `json:"evidence_type"`
	Summary      string `json:"summary"`
	MinimumCount int    `json:"minimum_count"`
	Freshness    string `json:"freshness"`
	Required     bool   `json:"required"`
}

// buildMainAgentPlanSpec 通过主 Agent 模型生成结构化计划，后端只做结构和 capability 校验。
func (s *AgentConversationService) buildMainAgentPlanSpec(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	controllerRun domain.AgentRun,
	input ReceiveWeChatWorkAppMessageInput,
) (mainAgentPlanSpecResult, error) {
	if s == nil || s.llmClient == nil {
		return mainAgentPlanSpecResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "main_agent_planner_unavailable", "主 Agent 规划模型不可用，无法生成执行计划。", "service.agent.main_planner", true, nil)
	}
	var lastErr error
	var lastRaw string
	for attempt := 1; attempt <= mainAgentPlanSpecMaxAttempts; attempt++ {
		// 每次尝试都把上一次错误和原始返回带回模型，允许模型自行修正 JSON 结构。
		request := s.mainAgentPlanSpecRequest(account, session, turn, input, attempt, lastErr, lastRaw)
		s.recordMainAgentPlanSpecTrace(ctx, controllerRun, "main_agent_plan_spec_request", domain.AgentJSON{
			"attempt":  attempt,
			"messages": mainAgentPlannerMessagesMetadata(request.Messages),
		}, "")
		response, err := s.llmClient.Chat(ctx, request)
		if err != nil {
			// 模型调用失败属于规划阶段失败，记录后进入下一次有限重试。
			lastErr = err
			s.recordMainAgentPlanSpecTrace(ctx, controllerRun, "main_agent_plan_spec_error", domain.AgentJSON{
				"attempt": attempt,
				"error":   err.Error(),
			}, "")
			continue
		}
		lastRaw = strings.TrimSpace(response.Content)
		// 解析和校验分开处理：解析保证 JSON 契约，校验保证能力边界。
		spec, parseErr := parseMainAgentPlanSpecJSON(lastRaw)
		if parseErr == nil {
			parseErr = s.validateMainAgentPlanSpec(spec)
		}
		if parseErr != nil {
			// 无效计划不会落库为可执行计划，只作为 controller trace 留给 Web 详情排查。
			lastErr = parseErr
			s.recordMainAgentPlanSpecTrace(ctx, controllerRun, "main_agent_plan_spec_invalid", domain.AgentJSON{
				"attempt":        attempt,
				"provider":       response.Provider,
				"model":          response.Model,
				"raw_response":   safeSummary(lastRaw, 3000),
				"validate_error": parseErr.Error(),
			}, response.Provider+"/"+response.Model)
			continue
		}
		result := mainAgentPlanSpecResult{
			Spec:      spec,
			Provider:  response.Provider,
			Model:     response.Model,
			Raw:       lastRaw,
			Attempts:  attempt,
			Validated: true,
		}
		// 有效计划同时保存摘要和截断后的原始返回，便于后续核对模型实际规划过程。
		s.recordMainAgentPlanSpecTrace(ctx, controllerRun, "main_agent_plan_spec_valid", domain.AgentJSON{
			"attempt":      attempt,
			"provider":     response.Provider,
			"model":        response.Model,
			"raw_response": safeSummary(lastRaw, 3000),
			"spec":         mainAgentPlanSpecMetadata(spec),
		}, response.Provider+"/"+response.Model)
		return result, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("主 Agent 未返回可用计划。")
	}
	return mainAgentPlanSpecResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "main_agent_plan_spec_invalid", lastErr.Error(), "service.agent.main_planner", true, nil)
}

// mainAgentPlanSpecRequest 构造主 Agent 规划请求。
// 请求载荷以 JSON 形式传递用户消息、当前时间、能力目录和输出结构，避免通过后端关键词规则预先判断意图。
func (s *AgentConversationService) mainAgentPlanSpecRequest(
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	attempt int,
	previousErr error,
	previousRaw string,
) llm.ChatRequest {
	userPayload := domain.AgentJSON{
		"user_id":           account.UserID,
		"session_id":        session.ID,
		"turn_id":           turn.ID,
		"provider":          input.Provider,
		"message_type":      input.MsgType,
		"user_message":      input.TextContent,
		"current_time":      s.now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		"capabilities":      s.mainAgentCapabilityCatalog(),
		"required_schema":   mainAgentPlanSpecSchemaHint(),
		"output_language":   "zh-CN",
		"reply_constraints": mainAgentPlanSpecReplyConstraints(),
	}
	if attempt > 1 {
		userPayload["repair_required"] = true
		if previousErr != nil {
			userPayload["previous_error"] = previousErr.Error()
		}
		if strings.TrimSpace(previousRaw) != "" {
			// 原始返回只保留摘要，避免把过长或敏感内容写入规划修复请求。
			userPayload["previous_raw_response"] = safeSummary(previousRaw, 2000)
		}
	}
	// 用户载荷使用 JSON，避免把 capability catalog 和 schema 说明拼成不可解析长文本。
	payloadBytes, _ := json.Marshal(userPayload)
	return llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: mainAgentPlanSpecSystemPrompt()},
			{Role: "user", Content: string(payloadBytes)},
		},
		Temperature: 0.1,
		MaxTokens:   3200,
	}
}

// mainAgentCapabilityCatalog 将后端已注册能力转换为模型可读目录。
// 模型只能基于该目录选择 capability，后续仍会执行注册、权限和预算校验。
func (s *AgentConversationService) mainAgentCapabilityCatalog() []domain.AgentJSON {
	if s == nil || s.capabilityRegistry == nil {
		return nil
	}
	capabilities := s.capabilityRegistry.List()
	items := make([]domain.AgentJSON, 0, len(capabilities))
	for _, capability := range capabilities {
		// 隐藏能力不下发给模型，防止模型规划到内部专用或非用户可见入口。
		if capability.Mode == agent.CapabilityModeHidden {
			continue
		}
		tool := capability.MCPDescriptor()
		items = append(items, domain.AgentJSON{
			"key":             capability.Key,
			"name":            capability.Name,
			"description":     capability.Description,
			"mode":            string(capability.Mode),
			"risk":            string(capability.Risk),
			"data_domain":     capability.DataDomain,
			"mutates":         capability.Mutates,
			"external_access": capability.ExternalAccess,
			"schedulable":     capability.Schedulable,
			"mcp_tool":        tool,
			"inputSchema":     tool.InputSchema,
			"annotations":     tool.Annotations,
			"_meta":           tool.Meta,
		})
	}
	return items
}

// parseMainAgentPlanSpecJSON 从模型原始文本中解析 PlanSpec。
// 该函数只接受 JSON 对象，兼容代码块外壳，但不从自然语言中猜测计划内容。
func parseMainAgentPlanSpecJSON(raw string) (agent.PlanSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return agent.PlanSpec{}, fmt.Errorf("模型规划返回为空。")
	}
	// 兼容模型偶发包裹代码块的情况；结构化契约仍要求最终解析为单个 JSON 对象。
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return agent.PlanSpec{}, fmt.Errorf("模型规划未返回 JSON 对象。")
	}
	raw = raw[start : end+1]
	var decoded mainAgentPlanSpecJSON
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return agent.PlanSpec{}, fmt.Errorf("模型规划 JSON 解析失败：%w", err)
	}
	// 反序列化结构和领域结构分离，避免 service 直接依赖 planner 内部规范化细节。
	return decoded.toAgentPlanSpec(), nil
}

// toAgentPlanSpec 将传输层 JSON 结构转换为 agent 领域结构。
// 转换过程不做领域分类，只做数组压缩和类型映射。
func (j mainAgentPlanSpecJSON) toAgentPlanSpec() agent.PlanSpec {
	subtasks := make([]agent.PlanSubtask, 0, len(j.Subtasks))
	for _, subtask := range j.Subtasks {
		// 子任务字段原样保留模型语义，只对 capability key 做空值压缩。
		subtasks = append(subtasks, agent.PlanSubtask{
			Title:           subtask.Title,
			Prompt:          subtask.Prompt,
			ContextSummary:  subtask.ContextSummary,
			CapabilityKeys:  compactNonEmptyStrings(subtask.CapabilityKeys),
			ExpectedOutput:  subtask.ExpectedOutput,
			FailureStrategy: subtask.FailureStrategy,
			MaxRetries:      subtask.MaxRetries,
		})
	}
	requirements := make([]agent.PlanEvidenceRequirement, 0, len(j.EvidenceRequirements))
	for _, requirement := range j.EvidenceRequirements {
		// 证据要求由模型生成，后端不在此处按领域词表重写。
		requirements = append(requirements, agent.PlanEvidenceRequirement{
			EvidenceType: requirement.EvidenceType,
			Summary:      requirement.Summary,
			MinimumCount: requirement.MinimumCount,
			Freshness:    requirement.Freshness,
			Required:     requirement.Required,
		})
	}
	return agent.PlanSpec{
		Goal:                   j.Goal,
		Intent:                 j.Intent,
		TaskType:               j.TaskType,
		Complexity:             agent.PlanningComplexity(j.Complexity),
		RequiresSubAgent:       j.RequiresSubAgent,
		DirectAnswerAllowed:    j.DirectAnswerAllowed,
		RequiredCapabilities:   compactNonEmptyStrings(j.RequiredCapabilities),
		Subtasks:               subtasks,
		EvidenceRequirements:   requirements,
		MaxIterations:          j.MaxIterations,
		FinalAnswerConstraints: compactNonEmptyStrings(j.FinalAnswerConstraints),
		Metadata:               domain.AgentJSON(j.Metadata),
	}
}

// validateMainAgentPlanSpec 校验模型计划是否满足最小可执行条件。
// 校验范围限定在结构完整性和 capability 注册表，不承担用户意图判断。
func (s *AgentConversationService) validateMainAgentPlanSpec(spec agent.PlanSpec) error {
	if strings.TrimSpace(spec.Intent) == "" && strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("intent 或 goal 至少需要一个。")
	}
	// 汇总顶层和子任务能力，统一做注册表校验，避免模型创造不存在的工具。
	keys := append([]string(nil), spec.RequiredCapabilities...)
	for _, subtask := range spec.Subtasks {
		keys = append(keys, subtask.CapabilityKeys...)
	}
	keys = compactNonEmptyStrings(keys)
	// 非直接回答计划必须具备至少一个可执行能力，否则无法形成完整闭环。
	if !spec.DirectAnswerAllowed && len(keys) == 0 {
		return fmt.Errorf("非直接回答计划必须选择至少一个 capability。")
	}
	for _, key := range keys {
		// 此处只校验能力是否注册；权限、预算和确认策略在后续治理阶段统一处理。
		if _, ok := s.capabilityRegistry.Get(key); !ok {
			return fmt.Errorf("capability 未注册：%s", key)
		}
	}
	return nil
}

// recordMainAgentPlanSpecTrace 持久化主 Agent 规划阶段的请求、错误或有效结果。
// Web 详情页依赖这些 trace 还原“理解任务 -> 生成计划”的过程。
func (s *AgentConversationService) recordMainAgentPlanSpecTrace(ctx context.Context, run domain.AgentRun, traceKind string, content domain.AgentJSON, modelKey string) {
	if s == nil || s.runManager == nil || run.ID == 0 {
		return
	}
	if strings.TrimSpace(modelKey) == "" {
		// 没有模型返回时使用 controller run 的 model key，保证 trace 可关联到本轮运行。
		modelKey = run.ModelKey
	}
	_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       traceKind,
		ModelKey:        modelKey,
		Content:         content,
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(fmt.Sprint(content)),
	})
}

// mainAgentPlannerMessagesMetadata 生成可审计但受限长度的模型消息摘要。
// 该摘要用于 Web 展示和排错，不参与模型二次规划。
func mainAgentPlannerMessagesMetadata(messages []llm.ChatMessage) []domain.AgentJSON {
	items := make([]domain.AgentJSON, 0, len(messages))
	for _, message := range messages {
		// trace 中保留截断后的消息，既能排查规划输入，又控制 Web 详情体积。
		items = append(items, domain.AgentJSON{
			"role":    message.Role,
			"content": safeSummary(message.Content, 3000),
		})
	}
	return items
}

// mainAgentPlanSpecMetadata 生成结构化计划摘要。
// 摘要用于计划 metadata，便于前端按流水线展示主 Agent 的规划产物。
func mainAgentPlanSpecMetadata(spec agent.PlanSpec) domain.AgentJSON {
	subtasks := make([]domain.AgentJSON, 0, len(spec.Subtasks))
	for index, subtask := range spec.Subtasks {
		// 计划摘要只保留子任务核心字段，完整提示词仍在模型原始响应摘要中可见。
		subtasks = append(subtasks, domain.AgentJSON{
			"index":           index + 1,
			"title":           subtask.Title,
			"capability_keys": subtask.CapabilityKeys,
			"prompt":          safeSummary(subtask.Prompt, 1000),
		})
	}
	return domain.AgentJSON{
		"goal":                  spec.Goal,
		"intent":                spec.Intent,
		"task_type":             spec.TaskType,
		"complexity":            string(spec.Complexity),
		"requires_sub_agent":    spec.RequiresSubAgent,
		"direct_answer_allowed": spec.DirectAnswerAllowed,
		"required_capabilities": spec.RequiredCapabilities,
		"subtasks":              subtasks,
		"max_iterations":        spec.MaxIterations,
	}
}
