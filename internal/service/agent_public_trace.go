package service

import "messagefeed/internal/domain"

func agentPublicTraceContent(content domain.AgentJSON) domain.AgentJSON {
	var sanitize func(any) any
	sanitize = func(value any) any {
		switch typed := value.(type) {
		case domain.AgentJSON:
			return sanitize(map[string]any(typed))
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, entry := range typed {
				if key == "response_items" || key == "ResponseItems" || key == "encrypted_content" {
					continue
				}
				result[key] = sanitize(entry)
			}
			return result
		case []any:
			result := make([]any, len(typed))
			for index, entry := range typed {
				result[index] = sanitize(entry)
			}
			return result
		default:
			return value
		}
	}
	return domain.AgentJSON(sanitize(content).(map[string]any))
}
