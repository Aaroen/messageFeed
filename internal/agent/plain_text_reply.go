package agent

import (
	"encoding/json"
	"messagefeed/internal/domain"
	"net/url"
	"regexp"
	"strings"
)

var (
	plainTextHeadingPattern        = regexp.MustCompile(`^#{1,6}\s+`)
	plainTextListMarkerPattern     = regexp.MustCompile(`^([-*+])\s+`)
	plainTextOrderedListPattern    = regexp.MustCompile(`^\d+[.)]\s+`)
	plainTextLinkPattern           = regexp.MustCompile(`!?\[[^\]]+\]\([^)]+\)`)
	plainTextHorizontalRulePattern = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)
)

// PlainTextReplyViolation 返回用户可见回复中的 Markdown 形态原因。
// 该函数只做格式协议校验，不分析用户意图，也不改写回复事实。
func PlainTextReplyViolation(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	switch {
	case strings.Contains(text, "```"):
		return "markdown_code_fence"
	case strings.Contains(text, "`"):
		return "markdown_inline_code"
	case strings.Contains(text, "**") || strings.Contains(text, "__"):
		return "markdown_emphasis"
	case plainTextLinkPattern.MatchString(text):
		return "markdown_link_or_image"
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case plainTextHeadingPattern.MatchString(trimmed):
			return "markdown_heading"
		case strings.HasPrefix(trimmed, ">"):
			return "markdown_blockquote"
		case plainTextListMarkerPattern.MatchString(trimmed):
			return "markdown_list"
		case plainTextOrderedListPattern.MatchString(trimmed):
			return "markdown_ordered_list"
		case looksLikeMarkdownTableLine(trimmed):
			return "markdown_table"
		case plainTextHorizontalRulePattern.MatchString(trimmed):
			return "markdown_horizontal_rule"
		}
	}
	return ""
}

func looksLikeMarkdownTableLine(line string) bool {
	if !strings.Contains(line, "|") {
		return false
	}
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	return strings.Count(line, "|") >= 2
}

// PlainTextReplyRepairPrompt 集中维护“纯文本重写”提示。
// 业务流程只传入草稿和格式错误原因，具体话术约束都由这里统一管理。
func PlainTextReplyRepairPrompt(draft string, violation string, attempt int) string {
	payload := domain.AgentJSON{
		"instruction": "请把 draft 改写为普通微信聊天纯文本。保留事实、结论、来源 URL 和必要数字；不要新增事实；不要使用 Markdown 标题、加粗、代码块、表格、引用块、列表符号、反引号或链接标题格式；只输出改写后的文本。",
		"violation":   strings.TrimSpace(violation),
		"attempt":     attempt,
		"draft":       strings.TrimSpace(draft),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(draft)
	}
	return string(body)
}

// PlainTextReplyCompletenessViolation 返回最终回复中明显的结构性截断原因。
// 该函数只检查残缺 URL、未闭合括号和悬空结尾等通用形态，不根据具体业务内容补结论。
func PlainTextReplyCompletenessViolation(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty_reply"
	}
	if hasIncompleteURLTail(text) {
		return "incomplete_url"
	}
	if hasUnclosedReplyDelimiter(text) {
		return "unclosed_delimiter"
	}
	if hasDanglingReplyEnding(text) {
		return "dangling_ending"
	}
	return ""
}

func hasIncompleteURLTail(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, suffix := range []string{"https://", "http://", "https:", "http:", "https", "http", "www."} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return false
	}
	last := strings.Trim(fields[len(fields)-1], " \t\r\n，。；;、）)]】》>\"'")
	if strings.HasPrefix(last, "http://") || strings.HasPrefix(last, "https://") {
		parsed, err := url.Parse(last)
		return err != nil || parsed.Scheme == "" || parsed.Host == ""
	}
	return false
}

func hasUnclosedReplyDelimiter(text string) bool {
	pairs := [][2]string{
		{"(", ")"},
		{"（", "）"},
		{"[", "]"},
		{"【", "】"},
		{"「", "」"},
		{"“", "”"},
		{"《", "》"},
	}
	for _, pair := range pairs {
		if strings.Count(text, pair[0]) > strings.Count(text, pair[1]) {
			return true
		}
	}
	return false
}

func hasDanglingReplyEnding(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	for _, suffix := range []string{"：", ":", "，", ",", "；", ";", "、", "（", "(", "【", "[", "《", "「", "/", "-", "以及"} {
		if strings.HasSuffix(text, suffix) {
			return true
		}
	}
	return false
}

func PlainTextReplyCompletenessRepairPrompt(draft string, violation string, attempt int) string {
	payload := domain.AgentJSON{
		"instruction": "请基于上文和 draft 重新输出完整的微信聊天纯文本最终回答。修复残缺 URL、未闭合括号、悬空冒号或明显未完成的句子；保留已有事实、结论、来源 URL 和必要数字；不要新增无法从上下文推出的事实；如果 draft 中的来源链接残缺且上文没有完整链接，请删除残缺链接并说明证据不足；不要使用 Markdown；只输出完整最终答复。",
		"violation":   strings.TrimSpace(violation),
		"attempt":     attempt,
		"draft":       strings.TrimSpace(draft),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(draft)
	}
	return string(body)
}
