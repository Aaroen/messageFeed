package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strconv"
	"strings"
)

type agentWorkflowStore interface {
	CreateAgentWorkflow(context.Context, domain.AgentWorkflow) (domain.AgentWorkflow, error)
	GetAgentWorkflow(context.Context, int64, int64) (domain.AgentWorkflow, error)
	ListAgentWorkflows(context.Context, int64) ([]domain.AgentWorkflow, error)
	ReviewAgentWorkflow(context.Context, int64, int64, string) error
}

type agentWorkflowDraft struct {
	Name  string                   `json:"name"`
	Steps []agentWorkflowDraftStep `json:"steps"`
}

type agentWorkflowDraftStep struct {
	Capability string           `json:"capability"`
	Arguments  domain.AgentJSON `json:"arguments"`
}

var agentWorkflowSchema = llm.SchemaFor[agentWorkflowDraft]("agent_workflow_draft_v1", "name", "steps")

func (s *AgentConversationService) runAgentWorkflowCommand(ctx context.Context, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, input ReceiveWeChatWorkAppMessageInput, command string) (string, error) {
	store, ok := s.repository.(agentWorkflowStore)
	if !ok {
		return "", fmt.Errorf("工作流存储不可用")
	}
	action, value, _ := strings.Cut(strings.TrimSpace(command), " ")
	value = strings.TrimSpace(value)
	switch action {
	case "draft":
		if value == "" {
			return "请提供工作流要执行的任务。", nil
		}
		return s.draftAgentWorkflow(ctx, store, turn, value)
	case "list":
		workflows, err := store.ListAgentWorkflows(ctx, account.UserID)
		if err != nil || len(workflows) == 0 {
			return "当前没有可用的工作流。", err
		}
		var lines []string
		for _, workflow := range workflows {
			lines = append(lines, fmt.Sprintf("%d. %s（%s）", workflow.ID, workflow.Name, agentWorkflowStatus(workflow.Status)))
		}
		return strings.Join(lines, "\n"), nil
	case "show", "publish", "archive", "run":
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			return "请提供有效的工作流编号。", nil
		}
		workflow, err := store.GetAgentWorkflow(ctx, account.UserID, id)
		if err != nil {
			return "", err
		}
		switch action {
		case "show":
			return s.formatAgentWorkflow(workflow), nil
		case "archive":
			return fmt.Sprintf("工作流 %d 已停用。", id), store.ReviewAgentWorkflow(ctx, account.UserID, id, "archived")
		case "publish":
			if err := s.validateAgentWorkflow(workflow); err != nil {
				return "", err
			}
			return fmt.Sprintf("工作流 %d 已启用。", id), store.ReviewAgentWorkflow(ctx, account.UserID, id, "published")
		case "run":
			if workflow.Status != "published" {
				return "请审核并启用这个工作流后再执行。", nil
			}
			return s.executeAgentWorkflow(ctx, account, session, turn, input, workflow)
		}
	}
	return "可用命令：`/workflow draft 任务描述`、`/workflow list`、`/workflow show 编号`、`/workflow publish 编号`、`/workflow run 编号`、`/workflow archive 编号`。", nil
}

func (s *AgentConversationService) draftAgentWorkflow(ctx context.Context, store agentWorkflowStore, turn domain.AgentTurn, goal string) (string, error) {
	if s.llmClient == nil {
		return "", fmt.Errorf("工作流草稿模型不可用")
	}
	var tools []agent.MCPToolDescriptor
	for _, capability := range s.capabilityRegistry.List() {
		if agentWorkflowCapabilityAllowed(capability) {
			tools = append(tools, capability.MCPDescriptor())
		}
	}
	payload, err := json.Marshal(domain.AgentJSON{"goal": goal, "allowed_tools": tools, "schema": agentWorkflowSchema.Schema})
	if err != nil {
		return "", err
	}
	ctx = WithAgentLLMCallContext(ctx, AgentLLMCallContext{UserID: turn.UserID, SessionID: turn.SessionID, TurnID: turn.ID, Stage: AgentLLMCallStagePlanning, PromptVersion: agentWorkflowSchema.Name})
	request := llm.ChatRequest{Messages: []llm.ChatMessage{
		{Role: "system", Content: "为当前用户生成可复用的只读工作流草稿。使用工具名称或注册键，依次声明一至八个独立步骤及参数。不要生成代码、表达式、权限或新工具。仅返回 JSON。"},
		{Role: "user", Content: string(payload)},
	}, OutputSchema: agentWorkflowSchema}
	response, err := s.llmClient.Chat(ctx, request)
	if ctx.Err() == nil && llm.CanRetryStructuredOutput(err) {
		request.SchemaLocalOnly = true
		response, err = s.llmClient.Chat(ctx, request)
	}
	if err != nil {
		return "", err
	}
	content, err := agentWorkflowSchema.Validate(response.Content)
	if err != nil {
		return "", err
	}
	var draft agentWorkflowDraft
	if err := json.Unmarshal([]byte(content), &draft); err != nil {
		return "", err
	}
	workflow := domain.AgentWorkflow{UserID: turn.UserID, SourceTurnID: turn.ID, Name: safeSummary(draft.Name, 100)}
	for _, step := range draft.Steps {
		key := strings.ReplaceAll(step.Capability, "__", ".")
		capability, _ := s.capabilityRegistry.Get(key)
		workflow.Steps = append(workflow.Steps, domain.AgentWorkflowStep{Capability: key, Version: capability.Version, Arguments: step.Arguments})
	}
	if err := s.validateAgentWorkflow(workflow); err != nil {
		return "", err
	}
	workflow, err = store.CreateAgentWorkflow(ctx, workflow)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\n审核通过后发送 `/workflow publish %d`。", s.formatAgentWorkflow(workflow), workflow.ID), nil
}

func agentWorkflowCapabilityAllowed(capability agent.Capability) bool {
	if capability.Mode == agent.CapabilityModeHidden || capability.Risk != agent.CapabilityRiskLow || capability.Mutates || capability.ExternalAccess || !capability.Reusable {
		return false
	}
	switch capability.Key {
	case "feed.query_recent_items", "source.query_latest_items", "conversation.query_history", "content.summarize_text":
		return true
	}
	return false
}

func (s *AgentConversationService) validateAgentWorkflow(workflow domain.AgentWorkflow) error {
	if workflow.Name == "" || len(workflow.Steps) == 0 || len(workflow.Steps) > 8 {
		return fmt.Errorf("%w: 工作流须有名称和一至八个步骤", domain.ErrInvalidInput)
	}
	for _, step := range workflow.Steps {
		capability, ok := s.capabilityRegistry.Get(step.Capability)
		if !ok || !agentWorkflowCapabilityAllowed(capability) || capability.Version != step.Version {
			return domain.NewAppError(domain.ErrorKindForbidden, "agent_workflow_tool_forbidden", "工作流包含未经授权的工具或版本", "service.agent.workflow", false, nil)
		}
		arguments, err := json.Marshal(step.Arguments)
		if err != nil {
			return err
		}
		if err := capability.ValidateArguments(string(arguments)); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentConversationService) executeAgentWorkflow(ctx context.Context, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, input ReceiveWeChatWorkAppMessageInput, workflow domain.AgentWorkflow) (string, error) {
	if err := s.validateAgentWorkflow(workflow); err != nil {
		return "", err
	}
	controller, err := s.createControllerRun(ctx, account, domain.AgentInboundMessage{ID: turn.InboundMessageID}, session, turn, input)
	if err != nil {
		return "", err
	}
	executor := s.agentCapabilityExecutor()
	sections := []string{workflow.Name}
	for index, step := range workflow.Steps {
		capability, _ := s.capabilityRegistry.Get(step.Capability)
		if s.policyEngine.Decide(ctx, agent.PolicyInput{Capability: capability, UserID: account.UserID}).Decision != agent.PolicyDecisionAllow {
			return "", domain.NewAppError(domain.ErrorKindForbidden, "agent_workflow_tool_forbidden", "此工作流步骤未获得授权", "service.agent.workflow", false, nil)
		}
		arguments, err := json.Marshal(step.Arguments)
		if err != nil {
			return "", err
		}
		result, err := executor.CallTool(ctx, agent.MCPCallToolInput{Capability: capability, Tool: capability.MCPDescriptor(),
			UserID: account.UserID, SessionID: session.ID, TurnID: turn.ID, ControllerRunID: controller.ID,
			CallID: fmt.Sprintf("workflow-%d-%d", workflow.ID, index+1), RawArguments: string(arguments), RequestID: input.RequestID, TraceID: input.TraceID})
		if err != nil {
			return "", err
		}
		if result.Result.IsError {
			return "", fmt.Errorf("工作流步骤未完成：%s", capability.Name)
		}
		sections = append(sections, fmt.Sprintf("%d. %s\n%s", index+1, capability.Name, result.TextContent()))
	}
	if _, err := s.runManager.CompleteRun(ctx, controller, fmt.Sprintf("workflow:%d", workflow.ID)); err != nil {
		return "", err
	}
	return strings.Join(sections, "\n\n"), nil
}

func (s *AgentConversationService) formatAgentWorkflow(workflow domain.AgentWorkflow) string {
	lines := []string{fmt.Sprintf("工作流 %d：%s（%s）", workflow.ID, workflow.Name, agentWorkflowStatus(workflow.Status))}
	for index, step := range workflow.Steps {
		tool, _ := s.capabilityRegistry.Get(step.Capability)
		arguments, _ := json.Marshal(step.Arguments)
		lines = append(lines, fmt.Sprintf("%d. %s，版本 %d，参数：%s", index+1, firstNonEmptyString(tool.Name, step.Capability), step.Version, arguments))
	}
	return strings.Join(lines, "\n")
}

func agentWorkflowStatus(status string) string {
	switch status {
	case "draft":
		return "待审核"
	case "published":
		return "已启用"
	default:
		return "已停用"
	}
}
