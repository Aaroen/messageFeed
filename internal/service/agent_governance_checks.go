package service

import "strings"

func agentGovernanceCheck(key string, ready bool, summary string) AgentDeploymentCheckResponse {
	return AgentDeploymentCheckResponse{
		Key:     key,
		Status:  readyIf(ready),
		Summary: summary,
	}
}

func agentGovernanceTextCheck(key string, summary string) AgentDeploymentCheckResponse {
	return agentGovernanceCheck(key, strings.TrimSpace(summary) != "", summary)
}
