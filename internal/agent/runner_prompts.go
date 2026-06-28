package agent

import (
	"encoding/json"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strings"
)

// promptedToolActionResponse 是非原生工具调用兼容层的模型输出契约。
// 当 OpenAI-compatible 服务不返回 tool_calls 时，runner 会要求模型按该 JSON 选择工具或收敛回答。
type promptedToolActionResponse struct {
	Action    string         `json:"action"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
	Content   string         `json:"content"`
	Reason    string         `json:"reason"`
}

// buildPromptedToolActionMessages 构造“工具兼容模式”的模型请求。
// 该提示词不判断具体用户意图，只把已授权工具目录、当前观察状态和输出 JSON 契约交给模型。
func buildPromptedToolActionMessages(messages []llm.ChatMessage, tools []MCPToolDescriptor, requireToolCall bool, hasObservations bool) []llm.ChatMessage {
	output := append([]llm.ChatMessage(nil), messages...)
	payload := domain.AgentJSON{
		"instruction":          promptedToolActionInstruction(requireToolCall, hasObservations),
		"available_tools":      promptedToolCatalog(tools),
		"require_tool_call":    requireToolCall,
		"has_observations":     hasObservations,
		"required_json_schema": promptedToolActionSchema(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"instruction":"return a JSON tool action or final answer"}`)
	}
	output = append(output, llm.ChatMessage{
		Role:    "user",
		Content: string(body),
	})
	return output
}

func promptedToolActionInstruction(requireToolCall bool, hasObservations bool) string {
	if requireToolCall {
		return "当前模型服务没有返回原生 tool_calls。请根据用户任务和 available_tools 选择一个必须执行的工具，并只返回严格 JSON；action 必须是 tool_call。"
	}
	if hasObservations {
		return "当前模型服务没有返回原生 tool_calls。请根据已有工具观察判断下一步：证据足够则返回 action=final 和用户可见回答；证据不足则从 available_tools 选择一个工具并返回 action=tool_call。只返回严格 JSON。"
	}
	return "请根据用户任务判断是否需要工具。需要工具时返回 action=tool_call；不需要工具时返回 action=final。只返回严格 JSON。"
}

func promptedToolCatalog(tools []MCPToolDescriptor) []domain.AgentJSON {
	items := make([]domain.AgentJSON, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		items = append(items, domain.AgentJSON{
			"name":         name,
			"title":        strings.TrimSpace(tool.Title),
			"description":  strings.TrimSpace(tool.Description),
			"inputSchema":  tool.InputSchema,
			"annotations":  tool.Annotations,
			"execution":    tool.Execution,
			"structured":   true,
			"mcp_version":  MCPProtocolVersion,
			"responseType": "tools/call",
		})
	}
	return items
}

func promptedToolActionSchema() domain.AgentJSON {
	return domain.AgentJSON{
		"action":    "string，tool_call 或 final。",
		"tool_name": "string，当 action=tool_call 时必须等于 available_tools 中某个 name。",
		"arguments": "object，当 action=tool_call 时必须符合对应工具 inputSchema。",
		"content":   "string，当 action=final 时填写面向用户的最终回答。",
		"reason":    "string，简短说明选择依据，供审计使用。",
	}
}

func parsePromptedToolAction(raw string) (promptedToolActionResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return promptedToolActionResponse{}, fmt.Errorf("prompted tool action response is empty")
	}
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return promptedToolActionResponse{}, fmt.Errorf("prompted tool action response is not JSON")
	}
	var decoded promptedToolActionResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &decoded); err != nil {
		return promptedToolActionResponse{}, fmt.Errorf("prompted tool action JSON parse failed: %w", err)
	}
	decoded.Action = strings.TrimSpace(decoded.Action)
	decoded.ToolName = strings.TrimSpace(decoded.ToolName)
	decoded.Content = strings.TrimSpace(decoded.Content)
	decoded.Reason = strings.TrimSpace(decoded.Reason)
	if decoded.Arguments == nil {
		decoded.Arguments = map[string]any{}
	}
	return decoded, nil
}

func promptedToolActionAllowed(action promptedToolActionResponse, tools []MCPToolDescriptor) bool {
	if strings.TrimSpace(action.ToolName) == "" {
		return false
	}
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == action.ToolName {
			return true
		}
	}
	return false
}

func requiredToolCallRetryPrompt(attempt int) string {
	return "上一轮没有执行已授权工具。当前计划要求先取得工具观察，再基于观察结果回答；本轮必须调用至少一个已授权工具。"
}

func emptyLLMResponseRetryPrompt(attempt int, finalOnly bool, hasObservations bool, hasTools bool) string {
	switch {
	case finalOnly:
		return "上一轮模型没有返回内容。请只基于以上工具观察生成最终回答，不要再请求工具；如果证据不足，请直接说明证据不足。"
	case hasObservations:
		return "上一轮模型没有返回内容。当前已经有工具观察，请优先基于已有证据生成最终回答；只有证据明显不足时才继续调用已授权工具。"
	case hasTools:
		return "上一轮模型没有返回内容。请根据用户任务选择已授权工具调用，或者在不需要工具时直接给出回答；本轮必须返回内容或工具调用。"
	default:
		return "上一轮模型没有返回内容。请直接根据已有上下文给出回答；如果无法回答，请说明缺少哪些信息。"
	}
}

func emptyMCPToolResultPrompt() string {
	return "MCP 工具调用完成，但 content[] 中没有可用文本内容。"
}

func llmResponseShapeRetryPrompt(err error, attempt int, finalOnly bool, hasObservations bool, hasTools bool) string {
	if isUnparsedToolCallError(err) {
		return unparsedToolCallRetryPrompt(finalOnly, hasObservations, hasTools)
	}
	return emptyLLMResponseRetryPrompt(attempt, finalOnly, hasObservations, hasTools)
}

// promptedToolActionRepairPrompt 是兼容工具动作的格式修复提示。
// 业务流程只传入结构化状态；具体可见给模型的协议要求在这里集中维护。
func promptedToolActionRepairPrompt(err error, attempt int, requireToolCall bool, hasObservations bool, hasTools bool) string {
	payload := domain.AgentJSON{
		"instruction":          promptedToolActionRepairInstruction(requireToolCall, hasObservations, hasTools),
		"previous_error":       strings.TrimSpace(err.Error()),
		"attempt":              attempt,
		"required_json_schema": promptedToolActionSchema(),
	}
	body, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return promptedToolActionRepairInstruction(requireToolCall, hasObservations, hasTools)
	}
	return string(body)
}

func promptedToolActionRepairInstruction(requireToolCall bool, hasObservations bool, hasTools bool) string {
	switch {
	case requireToolCall:
		return "上一轮输出不符合兼容工具动作契约。请重新输出严格 JSON，必须包含 action=tool_call、tool_name 和 arguments；不要输出工具调用标记、Markdown 代码块或仅包含参数的 JSON。"
	case hasTools && hasObservations:
		return "上一轮输出不符合兼容工具动作契约。请重新输出严格 JSON：证据足够时使用 action=final 和 content；仍需工具时使用 action=tool_call、tool_name 和 arguments。不要输出工具调用标记、Markdown 代码块或仅包含参数的 JSON。"
	case hasTools:
		return "上一轮输出不符合兼容工具动作契约。请重新输出严格 JSON：需要工具时使用 action=tool_call、tool_name 和 arguments；不需要工具时使用 action=final 和 content。不要输出工具调用标记、Markdown 代码块或仅包含参数的 JSON。"
	default:
		return "上一轮输出不符合最终回答契约。请重新输出严格 JSON，使用 action=final 和 content；不要输出工具调用标记、Markdown 代码块或仅包含参数的 JSON。"
	}
}

func unparsedToolCallRetryPrompt(finalOnly bool, hasObservations bool, hasTools bool) string {
	switch {
	case finalOnly:
		return "上一轮返回了无法执行的工具调用标记。当前阶段不再允许调用工具；请只基于已有工具观察生成最终回答。"
	case hasTools && hasObservations:
		return "上一轮返回了无法执行的工具调用标记。需要继续使用工具时必须返回原生 tool_calls；如果工具能力不可用，请按兼容 JSON 契约选择 action=tool_call 或 action=final。"
	case hasTools:
		return "上一轮返回了无法执行的工具调用标记。需要工具时必须返回原生 tool_calls；否则请直接给出最终回答。"
	default:
		return "上一轮返回了无法执行的工具调用标记。请不要输出工具调用标记，直接给出最终回答。"
	}
}

func mcpToolScopeDeniedText() string {
	return "MCP tools/call 被拒绝：该工具不在当前已批准的 capability scope 内。"
}
