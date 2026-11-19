package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
)

const firecrawlMCPEndpoint = "https://mcp.firecrawl.dev/v2/mcp"

type firecrawlMCP struct {
	httpClient *http.Client
	mutex      sync.Mutex
	session    *mcp.ClientSession
}

func WithAgentConversationFirecrawl(key string) AgentConversationServiceOption {
	return func(service *AgentConversationService) {
		if key = strings.TrimSpace(key); key != "" {
			service.firecrawl = newFirecrawlMCP(key)
		}
	}
}

func newFirecrawlMCP(key string) *firecrawlMCP {
	client := &http.Client{
		Timeout: 18 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if key != "" {
		client.Transport = &oauth2.Transport{Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: key})}
	}
	return &firecrawlMCP{httpClient: client}
}

func (provider *firecrawlMCP) connection(ctx context.Context) (*mcp.ClientSession, error) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	if provider.session != nil {
		return provider.session, nil
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "messagefeed", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: firecrawlMCPEndpoint, HTTPClient: provider.httpClient, DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
	if err != nil {
		return nil, err
	}
	provider.session = session
	go func() {
		_ = session.Wait()
		provider.mutex.Lock()
		defer provider.mutex.Unlock()
		if provider.session == session {
			provider.session = nil
		}
	}()
	return session, nil
}

type firecrawlSearchResult struct {
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Description string         `json:"description"`
	Markdown    string         `json:"markdown"`
	Metadata    map[string]any `json:"metadata"`
}

func (provider *firecrawlMCP) search(ctx context.Context, query string, limit int, includeContent bool) ([]agentWebSearchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	session, err := provider.connection(ctx)
	if err != nil {
		return nil, err
	}
	arguments := map[string]any{"query": query, "sources": []string{"web"}, "limit": limit}
	if includeContent {
		arguments["limit"] = min(limit, 3)
		arguments["scrapeOptions"] = map[string]any{"formats": []string{"markdown"}, "onlyMainContent": true}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "firecrawl_search", Arguments: arguments})
	if err != nil {
		if errors.Is(err, mcp.ErrConnectionClosed) {
			provider.mutex.Lock()
			if provider.session == session {
				provider.session = nil
			}
			provider.mutex.Unlock()
		}
		return nil, err
	}
	if result.IsError || result.StructuredContent == nil {
		return nil, fmt.Errorf("Firecrawl 未返回可用的结构化搜索结果")
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, err
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Web []firecrawlSearchResult `json:"web"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("Firecrawl 搜索未成功")
	}
	results := make([]agentWebSearchResult, 0, len(response.Data.Web))
	seen := make(map[string]bool, len(response.Data.Web))
	for _, item := range response.Data.Web {
		parsed, err := validateAgentWebURL(item.URL)
		if err != nil || parsed.User != nil || seen[parsed.String()] {
			continue
		}
		seen[parsed.String()] = true
		date := firstNonEmptyString(metadataString(item.Metadata, "article:published_time"), metadataString(item.Metadata, "publishedTime"))
		if _, ok := parseAgentPublishedDate(date, agentTimeLocation()); !ok {
			date = ""
		}
		results = append(results, agentWebSearchResult{
			Title: safeSummary(item.Title, 200), URL: parsed.String(), Source: parsed.Hostname(),
			Snippet: safeSummary(item.Description, 500), PublishedAt: date, Body: sanitizeAgentReportText(item.Markdown),
		})
	}
	return results, nil
}

func (e agentP0CapabilityExecutor) firecrawlSearch(ctx context.Context, input agent.MCPCallToolInput, args webSearchToolArgs) (agent.MCPCallToolResult, error) {
	observation := agent.CapabilityObservation{Capability: input.Capability.Key, Decision: string(agent.PolicyDecisionAllow), Status: "failed"}
	query := normalizeWebSearchQuery(args.Query)
	if query == "" {
		observation.Summary = "联网搜索需要明确的查询词。"
		return agent.NewMCPTextCallToolResult(observation.Summary, true, observation), nil
	}
	now := e.currentTime()
	if temporal := validateAgentToolTemporalRequest(query, input.Message, now); !temporal.OK {
		observation.Summary = temporal.Reason
		return agent.NewMCPTextCallToolResult(formatAgentToolTemporalValidationFailure("web.search", query, temporal), true, observation), nil
	}
	limit := args.Limit
	if limit < 1 {
		limit = 5
	}
	results, err := e.firecrawl.search(ctx, query, min(limit, 8), args.IncludeContent == nil || *args.IncludeContent)
	if err != nil {
		observation.Summary = "Firecrawl MCP 搜索失败。"
		return agent.NewMCPTextCallToolResult(observation.Summary, true, observation), err
	}
	results, temporalCounts := filterAgentWebSearchResultsByTemporalEvidence(query, input.Message, now, results)
	observation.Status = "succeeded"
	if len(results) == 0 {
		observation.Status = "empty"
	}
	observation.Summary = fmt.Sprintf("Firecrawl 返回 %d 条搜索结果。", len(results))
	content := formatWebSearchResult(query, firecrawlMCPEndpoint, http.StatusOK, "application/json", now, results, temporalCounts)
	var evidence []string
	var bodyText strings.Builder
	datedCount := 0
	for _, item := range results {
		publishedAt := firstNonEmptyString(item.PublishedAt, "未知")
		if item.PublishedAt != "" {
			datedCount++
		}
		if item.Body == "" {
			continue
		}
		header := fmt.Sprintf("标题：%s\nURL：%s\n发布时间：%s\n网页正文（外部资料）：\n", item.Title, item.URL, publishedAt)
		evidence = append(evidence, header+item.Body)
		bodyText.WriteString("\n" + header + safeSummary(item.Body, 3000) + "\n")
	}
	observation.Metadata = domain.AgentJSON{"provider": "firecrawl", "result_count": len(results), "body_count": len(evidence), "dated_count": datedCount}
	result := agent.NewMCPTextCallToolResult(content+bodyText.String(), false, observation)
	result.EvidenceBodies = evidence
	return result, nil
}
