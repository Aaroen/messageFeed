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

func emptyMCPToolResultPrompt() string {
	return "MCP 工具调用完成，但 content[] 中没有可用文本内容。"
}

func mcpToolScopeDeniedText() string {
	return "MCP tools/call 被拒绝：该工具不在当前已批准的 capability scope 内。"
}
