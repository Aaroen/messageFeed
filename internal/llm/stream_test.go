package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestResponsesStreamRetainsProtocolItemsAndFinalPhase(test *testing.T) {
	frames := []string{
		`{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"核验资料"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"核验资料"}],"encrypted_content":"opaque-state"}}`,
		`{"type":"response.output_text.delta","item_id":"msg_1","delta":"正在处理"}`,
		`{"type":"response.output_item.done","output_index":1,"item":{"type":"message","id":"msg_1","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"正在处理"}]}}`,
		`{"type":"response.output_item.done","output_index":2,"item":{"type":"message","id":"msg_2","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"完成核验"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","model":"configured-model","usage":{"input_tokens":8,"output_tokens":4,"total_tokens":12}}}`,
	}
	var stream strings.Builder
	for _, frame := range frames {
		fmt.Fprintf(&stream, "data: %s\n\n", frame)
	}
	var events []StreamEvent
	response, err := decodeChatStream(strings.NewReader(stream.String()), llmProtocolResponses, func(event StreamEvent) error { events = append(events, event); return nil })
	if err != nil {
		test.Fatal(err)
	}
	if response.Content != "完成核验" || response.ReasoningContent != "核验资料" || response.Usage.TotalTokens != 12 || len(response.ResponseItems) != 3 {
		test.Fatalf("完整输出解析失败：%+v", response)
	}
	input := responsesInputFromChatMessages([]ChatMessage{{Role: "assistant", Content: response.Content, ResponseItems: response.ResponseItems}})
	encoded, err := json.Marshal(input)
	if err != nil || !bytes.Contains(encoded, []byte(`"encrypted_content":"opaque-state"`)) || !bytes.Contains(encoded, []byte(`"phase":"final_answer"`)) {
		test.Fatalf("协议内容没有完整回传：%s，%v", encoded, err)
	}
	if len(events) < 4 || events[len(events)-1].Phase != "final_answer" || !events[len(events)-1].Replace {
		test.Fatalf("增量事件与完整条目没有收敛：%+v", events)
	}
}

func TestChatStreamAccumulatesInterleavedCalls(test *testing.T) {
	frames := []string{
		`{"id":"chat_1","choices":[{"index":0,"delta":{"reasoning_content":"检查会话","tool_calls":[{"index":1,"id":"call_b","function":{"name":"second","arguments":"{\"value\":"}},{"index":0,"id":"call_a","function":{"name":"first","arguments":"{}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`,
	}
	var stream strings.Builder
	for _, frame := range frames {
		fmt.Fprintf(&stream, "data: %s\n\n", frame)
	}
	stream.WriteString("data: [DONE]\n\n")
	response, err := decodeChatStream(strings.NewReader(stream.String()), llmProtocolChatCompletions, func(StreamEvent) error { return nil })
	if err != nil {
		test.Fatal(err)
	}
	if len(response.ToolCalls) != 2 || response.ToolCalls[0].ID != "call_a" || response.ToolCalls[1].Arguments != `{"value":1}` || response.ReasoningContent != "检查会话" || response.Usage.TotalTokens != 8 {
		test.Fatalf("交错工具调用解析失败：%+v", response)
	}
}

func TestStreamRejectsMissingCompletionAndFailedResponses(test *testing.T) {
	for _, body := range []string{
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"未完成\"}\n\n",
		"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\"}}\n\n",
		"data: {\"type\":\"response.incomplete\",\"response\":{\"status\":\"incomplete\"}}\n\n",
	} {
		if _, err := decodeChatStream(strings.NewReader(body), llmProtocolResponses, func(StreamEvent) error { return nil }); err == nil {
			test.Fatal("未完成的响应被识别为成功")
		}
	}
}
