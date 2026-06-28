package agent

import "testing"

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
