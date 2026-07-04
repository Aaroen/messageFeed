package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const (
	agentWeChatFeedbackLLMTimeout       = 4 * time.Second
	agentWeChatFeedbackMaxRunes         = 180
	agentWeChatFeedbackTemplateFileName = "configs/agent_wechat_feedback.zh-CN.json"
)

type agentWeChatFeedbackRequest struct {
	Stage       string
	UserMessage string
	Plan        domain.AgentPlan
	Step        domain.AgentPlanStep
	Control     agentWeChatFeedbackControl
	ErrorText   string
	Cause       error
	ProgressURL string
	ApprovalURL string
}

type agentWeChatFeedbackControl struct {
	ActionKey           string
	Handler             string
	Type                string
	Status              string
	Summary             string
	Changed             bool
	PlanID              int64
	ScheduledTaskID     int64
	ScheduledTaskStatus string
	Metadata            domain.AgentJSON
}

type agentWeChatFeedbackTemplateFile struct {
	Templates map[string]string `json:"templates"`
}

type agentWeChatFeedbackTemplateData struct {
	Stage               string
	Status              string
	Goal                string
	Summary             string
	StepTitle           string
	StepSummary         string
	Error               string
	ErrorType           string
	TimedOut            bool
	ThinkingTimedOut    bool
	ProgressURL         string
	ApprovalURL         string
	ActionKey           string
	Handler             string
	ControlType         string
	ControlStatus       string
	ControlSummary      string
	Changed             bool
	ScheduledTaskID     int64
	ScheduledTaskStatus string
}

// outboundNotificationContext 为企业微信发送动作创建独立短超时上下文。
// 发送消息不应继承后台执行上下文的取消状态，否则执行超时后无法把失败原因告知用户。
func (s *AgentConversationService) outboundNotificationContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := defaultAgentNotificationTimeout
	if s != nil && s.notificationTimeout > 0 {
		timeout = s.notificationTimeout
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

// generateAgentWeChatFeedbackText 只把结构化事实交给模型生成用户可见文本。
// 当模型不可用时，使用外部配置模板兜底；流程代码不拼接任何企微回复文案。
func (s *AgentConversationService) generateAgentWeChatFeedbackText(ctx context.Context, request agentWeChatFeedbackRequest) string {
	payload := request.agentWeChatFeedbackPayload()
	body, err := json.Marshal(payload)
	if err != nil {
		return finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData()))
	}
	if s == nil || s.llmClient == nil {
		return finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData()))
	}
	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), agentWeChatFeedbackLLMTimeout)
	defer cancel()
	response, err := s.llmClient.Chat(llmCtx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentWeChatFeedbackSystemPrompt()},
			{Role: "user", Content: agentWeChatFeedbackUserPrompt(body)},
		},
		Temperature: 0.35,
		MaxTokens:   96,
	})
	if err != nil {
		return finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData()))
	}
	text := normalizeAgentWeChatFeedbackText(response.Content)
	if text == "" {
		text = renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData())
	}
	if agent.PlainTextReplyViolation(text) != "" {
		text = s.repairAgentWeChatFeedbackPlainText(ctx, request, text)
	}
	return finalizeAgentWeChatFeedbackText(text)
}

// repairAgentWeChatFeedbackPlainText 复用 Agent 纯文本格式契约修复企微短反馈。
// 如果模型仍不遵守格式，则回到集中配置模板，避免 Markdown 直接进入用户侧。
func (s *AgentConversationService) repairAgentWeChatFeedbackPlainText(ctx context.Context, request agentWeChatFeedbackRequest, text string) string {
	if s == nil || s.llmClient == nil {
		return renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData())
	}
	current := strings.TrimSpace(text)
	for attempt := 0; attempt < 2; attempt++ {
		violation := agent.PlainTextReplyViolation(current)
		if violation == "" {
			return current
		}
		llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), agentWeChatFeedbackLLMTimeout)
		response, err := s.llmClient.Chat(llmCtx, llm.ChatRequest{
			Messages: []llm.ChatMessage{
				{Role: "system", Content: agentWeChatFeedbackSystemPrompt()},
				{Role: "user", Content: agent.PlainTextReplyRepairPrompt(current, violation, attempt+1)},
			},
			Temperature: 0.2,
			MaxTokens:   96,
		})
		cancel()
		if err != nil {
			break
		}
		current = normalizeAgentWeChatFeedbackText(response.Content)
	}
	if agent.PlainTextReplyViolation(current) == "" && current != "" {
		return current
	}
	return renderAgentWeChatFeedbackTemplate(request.fallbackTemplateKey(), request.templateData())
}

func (request agentWeChatFeedbackRequest) agentWeChatFeedbackPayload() domain.AgentJSON {
	errorText := strings.TrimSpace(request.ErrorText)
	if errorText == "" && request.Cause != nil {
		errorText = request.Cause.Error()
	}
	progressURL := strings.TrimSpace(request.ProgressURL)
	if progressURL == "" && request.Plan.ID > 0 {
		progressURL = ""
	}
	thinkingTimedOut := isAgentThinkingTimeoutError(request.Cause, errorText)
	payload := domain.AgentJSON{
		"schema":           agentWeChatFeedbackPayloadSchema(),
		"stage":            strings.TrimSpace(request.Stage),
		"user_message":     safeSummary(request.UserMessage, 300),
		"goal":             safeSummary(request.Plan.Goal, 300),
		"summary":          safeSummary(request.Plan.Summary, 300),
		"status":           string(request.Plan.Status),
		"error":            safeSummary(errorText, 300),
		"error_type":       agentFeedbackErrorType(request.Cause, errorText),
		"timed_out":        thinkingTimedOut || isAgentTimeoutError(request.Cause) || strings.Contains(strings.ToLower(errorText), "deadline exceeded"),
		"thinking_timeout": thinkingTimedOut,
		"progress_url":     progressURL,
		"approval_url":     strings.TrimSpace(request.ApprovalURL),
	}
	if request.Step.ID > 0 || strings.TrimSpace(request.Step.Title) != "" {
		payload["step"] = domain.AgentJSON{
			"title":      safeSummary(request.Step.Title, 160),
			"summary":    safeSummary(firstNonEmptyString(request.Step.OutputSummary, request.Step.InputSummary), 220),
			"status":     string(request.Step.Status),
			"capability": request.Step.CapabilityKey,
		}
	}
	if control := request.controlPayload(); len(control) > 0 {
		payload["control"] = control
	}
	return payload
}

func (request agentWeChatFeedbackRequest) templateData() agentWeChatFeedbackTemplateData {
	errorText := strings.TrimSpace(request.ErrorText)
	if errorText == "" && request.Cause != nil {
		errorText = request.Cause.Error()
	}
	thinkingTimedOut := isAgentThinkingTimeoutError(request.Cause, errorText)
	return agentWeChatFeedbackTemplateData{
		Stage:               strings.TrimSpace(request.Stage),
		Status:              string(request.Plan.Status),
		Goal:                safeSummary(request.Plan.Goal, 300),
		Summary:             safeSummary(request.Plan.Summary, 300),
		StepTitle:           safeSummary(request.Step.Title, 160),
		StepSummary:         safeSummary(firstNonEmptyString(request.Step.OutputSummary, request.Step.InputSummary), 220),
		Error:               safeSummary(errorText, 300),
		ErrorType:           agentFeedbackErrorType(request.Cause, errorText),
		TimedOut:            thinkingTimedOut || isAgentTimeoutError(request.Cause) || strings.Contains(strings.ToLower(errorText), "deadline exceeded"),
		ThinkingTimedOut:    thinkingTimedOut,
		ProgressURL:         strings.TrimSpace(request.ProgressURL),
		ApprovalURL:         strings.TrimSpace(request.ApprovalURL),
		ActionKey:           strings.TrimSpace(request.Control.ActionKey),
		Handler:             strings.TrimSpace(request.Control.Handler),
		ControlType:         strings.TrimSpace(request.Control.Type),
		ControlStatus:       strings.TrimSpace(request.Control.Status),
		ControlSummary:      safeSummary(request.Control.Summary, 220),
		Changed:             request.Control.Changed,
		ScheduledTaskID:     request.Control.ScheduledTaskID,
		ScheduledTaskStatus: strings.TrimSpace(request.Control.ScheduledTaskStatus),
	}
}

func (request agentWeChatFeedbackRequest) fallbackTemplateKey() string {
	stage := strings.TrimSpace(request.Stage)
	if stage == "" {
		stage = "progress"
	}
	data := request.templateData()
	if stage == "failed" && data.ThinkingTimedOut {
		return "failed_thinking_timeout"
	}
	if stage == "failed" && data.TimedOut {
		return "failed_timeout"
	}
	if stage == "button_callback" && request.Plan.ID < 1 {
		return "button_callback_no_plan"
	}
	if request.Step.ID > 0 && request.Step.Status == domain.AgentPlanStepStatusFailed {
		return "step_failed"
	}
	return stage
}

func (request agentWeChatFeedbackRequest) controlPayload() domain.AgentJSON {
	control := domain.AgentJSON{}
	if value := strings.TrimSpace(request.Control.ActionKey); value != "" {
		control["action_key"] = value
	}
	if value := strings.TrimSpace(request.Control.Handler); value != "" {
		control["handler"] = value
	}
	if value := strings.TrimSpace(request.Control.Type); value != "" {
		control["type"] = value
	}
	if value := strings.TrimSpace(request.Control.Status); value != "" {
		control["status"] = value
	}
	if value := strings.TrimSpace(request.Control.Summary); value != "" {
		control["summary"] = safeSummary(value, 300)
	}
	if request.Control.Changed {
		control["changed"] = true
	}
	if request.Control.PlanID > 0 {
		control["plan_id"] = request.Control.PlanID
	}
	if request.Control.ScheduledTaskID > 0 {
		control["scheduled_task_id"] = request.Control.ScheduledTaskID
	}
	if value := strings.TrimSpace(request.Control.ScheduledTaskStatus); value != "" {
		control["scheduled_task_status"] = value
	}
	if len(request.Control.Metadata) > 0 {
		control["metadata"] = request.Control.Metadata
	}
	return control
}

func finalizeAgentWeChatFeedbackText(text string) string {
	text = normalizeAgentWeChatFeedbackText(text)
	return sanitizeAgentReportText(limitRunes(text, agentWeChatFeedbackMaxRunes))
}

func normalizeAgentWeChatFeedbackText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"")
	return strings.TrimSpace(text)
}

func isAgentTimeoutError(err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") || strings.Contains(strings.ToLower(err.Error()), "timeout"))
}

// isAgentThinkingTimeoutError 只做工程错误归类，不生成任何用户可见话术。
// 用户最终看到的表述仍由模型提示词或集中模板基于该结构化字段生成。
func isAgentThinkingTimeoutError(cause error, errorText string) bool {
	var appErr *domain.AppError
	if errors.As(cause, &appErr) && appErr.Code == "llm_thinking_timeout" {
		return true
	}
	return strings.Contains(strings.ToLower(errorText), "llm_thinking_timeout")
}

func agentFeedbackErrorType(cause error, errorText string) string {
	if isAgentThinkingTimeoutError(cause, errorText) {
		return "thinking_timeout"
	}
	if isAgentTimeoutError(cause) || strings.Contains(strings.ToLower(errorText), "deadline exceeded") {
		return "timeout"
	}
	if strings.TrimSpace(errorText) != "" {
		return "error"
	}
	return ""
}

func renderAgentWeChatFeedbackTemplate(key string, data agentWeChatFeedbackTemplateData) string {
	templates := loadAgentWeChatFeedbackTemplates()
	source := strings.TrimSpace(templates[strings.TrimSpace(key)])
	if source == "" {
		return ""
	}
	parsed, err := template.New("agent_wechat_feedback").Option("missingkey=zero").Parse(source)
	if err != nil {
		return ""
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return ""
	}
	return strings.TrimSpace(output.String())
}

func loadAgentWeChatFeedbackTemplates() map[string]string {
	for _, path := range agentWeChatFeedbackTemplateCandidates() {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var decoded agentWeChatFeedbackTemplateFile
		if err := json.Unmarshal(body, &decoded); err != nil {
			continue
		}
		if len(decoded.Templates) > 0 {
			return decoded.Templates
		}
	}
	return map[string]string{}
}

func agentWeChatFeedbackTemplateCandidates() []string {
	candidates := make([]string, 0, 8)
	if configured := strings.TrimSpace(os.Getenv("AGENT_WECHAT_FEEDBACK_TEMPLATE_PATH")); configured != "" {
		candidates = append(candidates, configured)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return append(candidates, agentWeChatFeedbackTemplateFileName)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidates = append(candidates, filepath.Join(dir, agentWeChatFeedbackTemplateFileName))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return candidates
}

func limitRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}
