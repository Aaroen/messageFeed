package agent

import (
	"errors"
	"messagefeed/internal/domain"
	"testing"
)

func TestCapabilityMCPDescriptorUsesNativeToolShape(t *testing.T) {
	capability := Capability{
		Key:            "web.search",
		Name:           "搜索网页",
		Description:    "根据查询词返回候选网页。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_web",
		ExternalAccess: true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
			"required": []string{"query"},
		},
	}

	tool := capability.MCPDescriptor()
	if tool.Name != "web.search" {
		t.Fatalf("name = %q, want web.search", tool.Name)
	}
	if tool.InputSchema["type"] != "object" {
		t.Fatalf("inputSchema = %#v", tool.InputSchema)
	}
	if tool.Annotations["readOnlyHint"] != true || tool.Annotations["openWorldHint"] != true {
		t.Fatalf("annotations = %#v", tool.Annotations)
	}
	if tool.Meta["messagefeed.capability_key"] != "web.search" || tool.Meta["messagefeed.protocol_version"] != MCPProtocolVersion {
		t.Fatalf("meta = %#v", tool.Meta)
	}
}

func TestCapabilityMCPDescriptorNormalizesEmptyInputSchema(t *testing.T) {
	tool := Capability{Key: "content.summarize_text", Name: "总结文本"}.MCPDescriptor()
	if tool.InputSchema["type"] != "object" {
		t.Fatalf("inputSchema = %#v", tool.InputSchema)
	}
	if tool.InputSchema["additionalProperties"] != false {
		t.Fatalf("inputSchema = %#v", tool.InputSchema)
	}
}

func TestMCPTextToolResultJoinsTextContent(t *testing.T) {
	result := MCPToolResult{
		Content: []MCPTextContent{
			{Type: "text", Text: "第一段"},
			{Type: "text", Text: "第二段"},
		},
	}
	if result.TextContent() != "第一段\n第二段" {
		t.Fatalf("text content = %q", result.TextContent())
	}
}

func TestToolOutcomeForStatus(t *testing.T) {
	testCases := []struct {
		status         string
		canonical      string
		runStatus      domain.AgentRunStatus
		traceStatus    domain.AgentTraceEventStatus
		planStepStatus domain.AgentPlanStepStatus
		isError        bool
		evidenceUsable bool
	}{
		{"succeeded", "succeeded", domain.AgentRunStatusSucceeded, domain.AgentTraceEventSucceeded, domain.AgentPlanStepStatusCompleted, false, true},
		{"empty", "empty", domain.AgentRunStatusSucceeded, domain.AgentTraceEventSucceeded, domain.AgentPlanStepStatusSkipped, false, false},
		{"skipped", "skipped", domain.AgentRunStatusCanceled, domain.AgentTraceEventSkipped, domain.AgentPlanStepStatusSkipped, false, false},
		{"failed", "failed", domain.AgentRunStatusFailed, domain.AgentTraceEventFailed, domain.AgentPlanStepStatusFailed, true, false},
		{"rejected", "rejected", domain.AgentRunStatusFailed, domain.AgentTraceEventFailed, domain.AgentPlanStepStatusFailed, true, false},
		{"blocked", "rejected", domain.AgentRunStatusFailed, domain.AgentTraceEventFailed, domain.AgentPlanStepStatusFailed, true, false},
		{"requires_confirmation", "requires_confirmation", domain.AgentRunStatusInputRequired, domain.AgentTraceEventSkipped, domain.AgentPlanStepStatusPending, false, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.status, func(t *testing.T) {
			outcome, err := ToolOutcomeForStatus(testCase.status)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Status != testCase.canonical || outcome.RunStatus != testCase.runStatus || outcome.TraceStatus != testCase.traceStatus || outcome.PlanStepStatus != testCase.planStepStatus || outcome.IsError != testCase.isError || outcome.EvidenceUsable != testCase.evidenceUsable {
				t.Fatalf("工具状态映射不符合约定: %#v", outcome)
			}
		})
	}
	for _, status := range []string{"", " ", "success", "unknown"} {
		if _, err := ToolOutcomeForStatus(status); err == nil {
			t.Fatalf("未知状态 %q 应当返回错误", status)
		}
	}
}

func TestNormalizeToolResult(t *testing.T) {
	transportErr := errors.New("工具连接中断")
	testCases := []struct {
		name       string
		status     string
		isError    bool
		callErr    error
		wantStatus string
		wantError  bool
		wantErr    bool
	}{
		{"查询成功", "succeeded", false, nil, "succeeded", false, false},
		{"正常空结果", "empty", false, nil, "empty", false, false},
		{"证据被拒绝", "empty", true, nil, "rejected", true, false},
		{"调用被拒绝", "skipped", true, nil, "rejected", true, false},
		{"工具未执行", "skipped", false, nil, "skipped", false, false},
		{"工具失败", "failed", false, nil, "failed", true, false},
		{"权限阻断", "blocked", false, nil, "rejected", true, false},
		{"等待用户确认", "requires_confirmation", false, nil, "requires_confirmation", false, false},
		{"状态冲突", "succeeded", true, nil, "failed", true, true},
		{"确认状态冲突", "requires_confirmation", true, nil, "failed", true, true},
		{"未知状态", "unknown", false, nil, "failed", true, true},
		{"空状态", "", false, nil, "failed", true, true},
		{"连接错误", "succeeded", false, transportErr, "failed", true, true},
		{"无状态连接错误", "", false, transportErr, "failed", true, true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			input := NewMCPTextCallToolResult("完整工具正文", testCase.isError, CapabilityObservation{Status: testCase.status})
			result, err := NormalizeToolResult(input, testCase.callErr)
			if (err != nil) != testCase.wantErr || result.Observation.Status != testCase.wantStatus || result.Result.IsError != testCase.wantError {
				t.Fatalf("工具结果不符合约定: result=%#v, err=%v", result, err)
			}
			if testCase.callErr != nil && !errors.Is(err, testCase.callErr) {
				t.Fatalf("原始连接错误未保留: %v", err)
			}
			if result.TextContent() != "完整工具正文" {
				t.Fatalf("工具正文发生变化: %q", result.TextContent())
			}
		})
	}
}
