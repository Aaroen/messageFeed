package agent

import (
	"encoding/json"
	"messagefeed/internal/llm"
	"strings"
)

const MCPProtocolVersion = "2025-11-25"

// MCPToolsCapability 对应 MCP initialize 响应中的 capabilities.tools。
// 当前内部工具目录是静态注册表，因此 listChanged 为 false。
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// MCPServerCapabilities 描述本项目当前已实现的 MCP server 能力面。
// 本轮先完成 tools 能力契约，后续再按需暴露 prompts/resources/tasks。
type MCPServerCapabilities struct {
	Tools MCPToolsCapability `json:"tools"`
}

// MCPToolExecution 对应 MCP Tool.execution。
// taskSupport 只表示工具是否支持 MCP task-augmented execution，不替代业务层调度能力。
type MCPToolExecution struct {
	TaskSupport string `json:"taskSupport,omitempty"`
}

// MCPToolDescriptor 是内部 capability 面向模型和未来 MCP server 暴露时的标准工具描述。
// 字段命名保持 MCP 原始 JSON 形态，避免后续适配时再次转换语义。
type MCPToolDescriptor struct {
	Name         string            `json:"name"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description,omitempty"`
	InputSchema  map[string]any    `json:"inputSchema"`
	OutputSchema map[string]any    `json:"outputSchema,omitempty"`
	Annotations  map[string]any    `json:"annotations,omitempty"`
	Execution    *MCPToolExecution `json:"execution,omitempty"`
	Meta         map[string]any    `json:"_meta,omitempty"`
}

// MCPTextContent 是 MCP ToolResult content[] 中最常用的文本内容块。
// 当前业务工具只返回文本和结构化摘要，图片、音频和资源链接后续按需扩展。
type MCPTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult 对应 MCP tools/call 的 result。
// 执行错误使用 isError 表达，协议级错误仍由调用方返回 AppError。
type MCPToolResult struct {
	Content           []MCPTextContent `json:"content"`
	StructuredContent map[string]any   `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}

// MCPCallToolInput 是 runner 调用内部工具服务时使用的 MCP tools/call 请求上下文。
// name/arguments 来自模型工具调用；用户、会话和审计字段只在服务端执行与留痕使用。
type MCPCallToolInput struct {
	Capability      Capability
	Tool            MCPToolDescriptor
	UserID          int64
	SessionID       int64
	TurnID          int64
	ControllerRunID int64
	Message         string
	ExternalUserID  string
	CallID          string
	Name            string
	RawArguments    string
	RequestID       string
	TraceID         string
}

// MCPCallToolResult 是内部 tools/call 的执行结果。
// Result 使用 MCP 原生 ToolResult；Observation 保存本项目的审计与证据索引。
type MCPCallToolResult struct {
	Result      MCPToolResult
	Observation CapabilityObservation
}

// MCPToolsListResult 对应 MCP tools/list 的 result。
// 暂未实现分页时 nextCursor 为空。
type MCPToolsListResult struct {
	Tools      []MCPToolDescriptor `json:"tools"`
	NextCursor string              `json:"nextCursor,omitempty"`
}

// DefaultMCPServerCapabilities 返回当前服务端可声明的 MCP 能力。
func DefaultMCPServerCapabilities() MCPServerCapabilities {
	return MCPServerCapabilities{
		Tools: MCPToolsCapability{ListChanged: false},
	}
}

// MCPDescriptor 把内部 capability 转换为 MCP Tool。
// 权限、预算和用户确认仍由后端执行阶段校验，annotations 只作为模型和 UI 提示。
func (capability Capability) MCPDescriptor() MCPToolDescriptor {
	descriptor := MCPToolDescriptor{
		Name:        strings.TrimSpace(capability.Key),
		Title:       strings.TrimSpace(capability.Name),
		Description: strings.TrimSpace(capability.Description),
		InputSchema: normalizeMCPInputSchema(capability.InputSchema),
		Annotations: capability.MCPAnnotations(),
		Meta:        capability.MCPMeta(),
	}
	if capability.Schedulable {
		descriptor.Execution = &MCPToolExecution{TaskSupport: "optional"}
	}
	return descriptor
}

// MCPAnnotations 仅放 MCP 标准 ToolAnnotations 字段。
// 自有风险和数据域信息放入 _meta，避免把服务端安全策略误表达为可被信任的注解。
func (capability Capability) MCPAnnotations() map[string]any {
	readOnly := !capability.Mutates
	annotations := map[string]any{
		"title":           strings.TrimSpace(capability.Name),
		"readOnlyHint":    readOnly,
		"destructiveHint": capability.Mutates && capability.Risk == CapabilityRiskHigh,
		"idempotentHint":  readOnly,
		"openWorldHint":   capability.ExternalAccess,
	}
	return compactAnyMap(annotations)
}

// MCPMeta 保存本项目执行策略需要的自有元数据。
// 这些字段用于审计和前端展示，不作为模型绕过服务端策略的依据。
func (capability Capability) MCPMeta() map[string]any {
	meta := map[string]any{
		"messagefeed.capability_key":         strings.TrimSpace(capability.Key),
		"messagefeed.mode":                   string(capability.Mode),
		"messagefeed.risk":                   string(capability.Risk),
		"messagefeed.data_domain":            strings.TrimSpace(capability.DataDomain),
		"messagefeed.mutates":                capability.Mutates,
		"messagefeed.external_access":        capability.ExternalAccess,
		"messagefeed.schedulable":            capability.Schedulable,
		"messagefeed.reusable":               capability.Reusable,
		"messagefeed.requires_confirmation":  capabilityRequiresMCPConfirmation(capability),
		"messagefeed.protocol_version":       MCPProtocolVersion,
		"messagefeed.server_authoritative":   true,
		"messagefeed.annotations_untrusted":  true,
		"messagefeed.execution_policy_owner": "backend",
	}
	return compactAnyMap(meta)
}

// ListMCPTools 输出 MCP tools/list 兼容结果。
// 调用方负责在传入前做用户 scope、模式和隐藏能力过滤。
func ListMCPTools(capabilities []Capability) MCPToolsListResult {
	tools := make([]MCPToolDescriptor, 0, len(capabilities))
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Key) == "" {
			continue
		}
		tools = append(tools, capability.MCPDescriptor())
	}
	return MCPToolsListResult{Tools: tools}
}

// LLMToolDefinitionFromMCP 将 MCP Tool 转为上游模型函数调用定义。
// OpenAI-compatible 模型函数名不稳定支持点号，因此继续用双下划线编码 capability key。
func LLMToolDefinitionFromMCP(tool MCPToolDescriptor) llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:         toolNameForCapabilityKey(tool.Name),
		Title:        tool.Title,
		Description:  tool.Description,
		InputSchema:  normalizeMCPInputSchema(tool.InputSchema),
		OutputSchema: cloneMCPMap(tool.OutputSchema),
		Annotations:  cloneMCPMap(tool.Annotations),
		Meta:         cloneMCPMap(tool.Meta),
	}
}

// NewMCPTextToolResult 构造兼容 MCP tools/call 的文本结果。
func NewMCPTextToolResult(text string, isError bool) MCPToolResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return MCPToolResult{IsError: isError}
	}
	return MCPToolResult{
		Content: []MCPTextContent{{Type: "text", Text: text}},
		IsError: isError,
	}
}

// NewMCPTextCallToolResult 统一构造文本型 tools/call 结果。
func NewMCPTextCallToolResult(text string, isError bool, observation CapabilityObservation) MCPCallToolResult {
	return MCPCallToolResult{
		Result:      NewMCPTextToolResult(text, isError),
		Observation: observation,
	}
}

// TextContent 将 MCP content[] 中的文本块合并成模型可消费的工具结果文本。
// 非文本内容后续接入资源或图片时不在这里臆造文本。
func (result MCPToolResult) TextContent() string {
	var builder strings.Builder
	for _, item := range result.Content {
		if item.Type != "text" {
			continue
		}
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func (result MCPCallToolResult) TextContent() string {
	return result.Result.TextContent()
}

// normalizeMCPInputSchema 保证每个工具都有合法 object 根 schema。
// MCP 要求 inputSchema 不能为 null，无参数工具使用 additionalProperties=false。
func normalizeMCPInputSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
		}
	}
	normalized := cloneMCPMap(schema)
	if strings.TrimSpace(asString(normalized["type"])) == "" {
		normalized["type"] = "object"
	}
	return normalized
}

// cloneMCPMap 通过 JSON 往返复制 schema/metadata，避免调用方意外修改注册表原值。
func cloneMCPMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	body, err := json.Marshal(input)
	if err != nil {
		output := make(map[string]any, len(input))
		for key, value := range input {
			output[key] = value
		}
		return output
	}
	var output map[string]any
	if err := json.Unmarshal(body, &output); err != nil {
		output := make(map[string]any, len(input))
		for key, value := range input {
			output[key] = value
		}
		return output
	}
	return output
}

func compactAnyMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		case nil:
			continue
		}
		output[key] = value
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func capabilityRequiresMCPConfirmation(capability Capability) bool {
	return capability.Risk == CapabilityRiskHigh ||
		capability.Mutates ||
		capability.Schedulable ||
		capabilityUsesToolConfirmation(capability)
}
