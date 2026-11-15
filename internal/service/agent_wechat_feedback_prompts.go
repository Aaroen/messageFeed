package service

func agentWeChatFeedbackText(request agentWeChatFeedbackRequest) string {
	return finalizeAgentWeChatFeedbackText(renderAgentWeChatFeedbackTemplate(request.templateKey(), request.templateData()))
}
