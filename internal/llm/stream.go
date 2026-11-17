package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type StreamEvent struct {
	Kind    string
	ItemID  string
	Name    string
	Delta   string
	Phase   string
	Replace bool
}

func (c *OpenAICompatibleClient) chatWithStream(ctx context.Context, request ChatRequest, protocol llmProtocol, host string, span trace.Span) (ChatResponse, error) {
	var payload any
	path := "/chat/completions"
	if protocol == llmProtocolResponses {
		path = "/responses"
		payload = responsesRequest{Model: c.model, Input: responsesInputFromChatMessages(request.Messages), Tools: responsesToolsFromDomain(request.Tools), ToolChoice: request.ToolChoice, Temperature: request.Temperature, MaxOutputTokens: request.MaxTokens}
	} else {
		payload = chatCompletionRequest{Model: c.model, Messages: chatCompletionMessagesFromDomain(request.Messages), Tools: chatCompletionToolsFromDomain(request.Tools), ToolChoice: request.ToolChoice, Temperature: request.Temperature, MaxTokens: request.MaxTokens}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		return ChatResponse{}, err
	}
	body["stream"] = true
	if request.ToolChoice == "" {
		delete(body, "tool_choice")
	}
	if protocol == llmProtocolChatCompletions {
		body["stream_options"] = map[string]any{"include_usage": true}
	}
	encoded, err = json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return ChatResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	started := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return ChatResponse{}, err
	}
	defer httpResponse.Body.Close()
	defer func() {
		recordLLMExternalHTTPRequest(openAICompatibleExternalHTTPOperation, host, strconv.Itoa(httpResponse.StatusCode), time.Since(started))
	}()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		data, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
		if err != nil {
			return ChatResponse{}, err
		}
		return ChatResponse{}, newLLMRouteError(protocol, httpResponse.StatusCode, providerErrorMessage(data, httpResponse.StatusCode), false, nil)
	}
	if !strings.HasPrefix(httpResponse.Header.Get("Content-Type"), "text/event-stream") {
		return ChatResponse{}, fmt.Errorf("模型流式响应的 Content-Type 无效")
	}
	response, err := decodeChatStream(httpResponse.Body, protocol, request.OnEvent)
	if response.Usage != nil {
		recordLLMUsage(c.provider, c.model, response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens, span)
	}
	return response, err
}

func readSSEFrames(reader io.Reader, consume func([]byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var data []string
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		frame := strings.Join(data, "\n")
		data = nil
		return consume([]byte(frame))
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func decodeChatStream(reader io.Reader, protocol llmProtocol, emit func(StreamEvent) error) (ChatResponse, error) {
	response := ChatResponse{Protocol: string(protocol)}
	items := map[int]responsesOutputItem{}
	calls := map[int]ToolCall{}
	finished, ended := false, false
	err := readSSEFrames(reader, func(data []byte) error {
		if string(data) == "[DONE]" {
			ended = true
			return nil
		}
		if ended {
			return fmt.Errorf("模型在流式结束标记之后继续返回内容")
		}
		if protocol == llmProtocolResponses {
			var event struct {
				Type         string              `json:"type"`
				Delta        string              `json:"delta"`
				Text         string              `json:"text"`
				Arguments    string              `json:"arguments"`
				ItemID       string              `json:"item_id"`
				OutputIndex  int                 `json:"output_index"`
				ContentIndex int                 `json:"content_index"`
				SummaryIndex int                 `json:"summary_index"`
				Item         responsesOutputItem `json:"item"`
				Response     responsesResponse   `json:"response"`
				Error        json.RawMessage     `json:"error"`
			}
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}
			itemID := event.ItemID
			if itemID == "" {
				itemID = event.Item.ID
			}
			switch event.Type {
			case "error", "response.failed":
				return fmt.Errorf("模型流式调用失败：%s", providerErrorMessage(data, 502))
			case "response.output_text.delta":
				return emit(StreamEvent{Kind: "text", ItemID: fmt.Sprintf("%s:text:%d", itemID, event.ContentIndex), Delta: event.Delta})
			case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
				return emit(StreamEvent{Kind: "reasoning", ItemID: fmt.Sprintf("%s:reasoning:%d", itemID, event.SummaryIndex), Delta: event.Delta})
			case "response.function_call_arguments.delta":
				return emit(StreamEvent{Kind: "tool", ItemID: itemID, Delta: event.Delta})
			case "response.output_item.added", "response.output_item.done":
				items[event.OutputIndex] = event.Item
				if event.Item.Type == "function_call" {
					return emit(StreamEvent{Kind: "tool", ItemID: itemID, Name: event.Item.Name, Delta: event.Item.Arguments, Replace: event.Type == "response.output_item.done"})
				}
				if event.Type == "response.output_item.done" && event.Item.Type == "message" {
					var detail struct {
						Phase string `json:"phase"`
					}
					if err := json.Unmarshal(event.Item.Raw, &detail); err != nil {
						return err
					}
					for index, part := range event.Item.Content {
						if part.Type == "output_text" || part.Type == "text" {
							if err := emit(StreamEvent{Kind: "text", ItemID: fmt.Sprintf("%s:text:%d", itemID, index), Delta: part.Text, Phase: detail.Phase, Replace: true}); err != nil {
								return err
							}
						}
					}
				}
				if event.Type == "response.output_item.done" && event.Item.Type == "reasoning" {
					for index, part := range event.Item.Summary {
						if err := emit(StreamEvent{Kind: "reasoning", ItemID: fmt.Sprintf("%s:reasoning:%d", itemID, index), Delta: part.Text, Replace: true}); err != nil {
							return err
						}
					}
				}
			case "response.completed", "response.incomplete":
				if finished {
					return fmt.Errorf("模型返回重复完成事件")
				}
				finished = true
				decoded := event.Response
				if len(decoded.Output) == 0 {
					indices := make([]int, 0, len(items))
					for index := range items {
						indices = append(indices, index)
					}
					sort.Ints(indices)
					for _, index := range indices {
						decoded.Output = append(decoded.Output, items[index])
					}
				}
				if event.Type == "response.incomplete" {
					decoded.Status = "incomplete"
				}
				var err error
				response, err = chatResponseFromResponses(decoded)
				return err
			}
			return nil
		}
		var chunk struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
				Delta        struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int                            `json:"index"`
						ID       string                         `json:"id"`
						Function chatCompletionToolCallFunction `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return err
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return fmt.Errorf("模型流式调用失败：%s", providerErrorMessage(data, 502))
		}
		if chunk.ID != "" {
			response.ResponseID = chunk.ID
		}
		if chunk.Model != "" {
			response.Model = chunk.Model
		}
		var usage chatCompletionResponse
		if err := json.Unmarshal(data, &usage); err != nil {
			return err
		}
		if usage.Usage != nil {
			response.Usage = &TokenUsage{InputTokens: usage.Usage.PromptTokens, OutputTokens: usage.Usage.CompletionTokens, TotalTokens: usage.Usage.TotalTokens}
			if usage.Usage.CompletionTokensDetails != nil {
				response.Usage.ReasoningTokens = usage.Usage.CompletionTokensDetails.ReasoningTokens
			}
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				return fmt.Errorf("模型返回了未请求的额外候选回答")
			}
			if choice.Delta.Content != "" {
				response.Content += choice.Delta.Content
				if err := emit(StreamEvent{Kind: "text", ItemID: "message:text:0", Delta: choice.Delta.Content}); err != nil {
					return err
				}
			}
			if choice.Delta.ReasoningContent != "" {
				response.ReasoningContent += choice.Delta.ReasoningContent
				if err := emit(StreamEvent{Kind: "reasoning", ItemID: "message:reasoning:0", Delta: choice.Delta.ReasoningContent}); err != nil {
					return err
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				call := calls[delta.Index]
				call.ID += delta.ID
				call.Name += delta.Function.Name
				call.Arguments += delta.Function.Arguments
				calls[delta.Index] = call
				if err := emit(StreamEvent{Kind: "tool", ItemID: fmt.Sprintf("call:%d", delta.Index), Name: call.Name, Delta: delta.Function.Arguments}); err != nil {
					return err
				}
			}
			if choice.FinishReason != "" {
				response.FinishReason, finished = choice.FinishReason, true
			}
		}
		return nil
	})
	if err != nil {
		return response, err
	}
	if !finished {
		return response, fmt.Errorf("模型流式连接结束时缺少完成事件")
	}
	if protocol == llmProtocolChatCompletions {
		indices := make([]int, 0, len(calls))
		for index := range calls {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			response.ToolCalls = append(response.ToolCalls, calls[index])
		}
		if response.FinishReason == "length" || response.FinishReason == "content_filter" {
			response.IncompleteReason = response.FinishReason
			return response, incompleteChatResponseError(response)
		}
	}
	response.Content = strings.TrimSpace(response.Content)
	if response.Content == "" && len(response.ToolCalls) == 0 {
		return response, fmt.Errorf("模型流式回答为空")
	}
	return response, nil
}
