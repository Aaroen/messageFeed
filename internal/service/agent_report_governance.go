package service

import (
	"regexp"
	"strings"
)

var agentSensitiveReportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|database_url|dsn)\s*[:=]\s*[^,\s]+`),
	regexp.MustCompile(`(?i)(bearer)\s+[a-z0-9._~+/=-]+`),
}

func sanitizeAgentReportText(text string) string {
	text = strings.TrimSpace(text)
	for _, pattern := range agentSensitiveReportPatterns {
		text = pattern.ReplaceAllString(text, "$1:[redacted]")
	}
	return text
}
