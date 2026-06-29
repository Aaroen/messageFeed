package agent

import (
	"encoding/json"
	"messagefeed/internal/domain"
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
