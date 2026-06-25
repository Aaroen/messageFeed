package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"messagefeed/internal/domain"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Planner struct {
	registry *CapabilityRegistry
	policy   *PolicyEngine
	now      func() time.Time
}

type PlannerOptions struct {
	Registry *CapabilityRegistry
	Policy   *PolicyEngine
	Now      func() time.Time
}

func NewPlanner(options PlannerOptions) *Planner {
	registry := options.Registry
	if registry == nil {
		registry = NewP0CapabilityRegistry()
	}
	policy := options.Policy
	if policy == nil {
		policy = NewPolicyEngine()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Planner{registry: registry, policy: policy, now: now}
}

type PlanInput struct {
	UserID          int64
	SessionID       int64
	TurnID          int64
	ControllerRunID int64
	Goal            string
}

type PlanOutput struct {
	Plan  domain.AgentPlan
	Steps []domain.AgentPlanStep
}

func (p *Planner) Build(ctx context.Context, input PlanInput) PlanOutput {
	now := p.now().UTC()
	capabilityKeys := p.selectCapabilities(input.Goal)
	scheduleConfirmed := looksLikeScheduleConfirmation(strings.ToLower(strings.TrimSpace(input.Goal))) && containsString(capabilityKeys, "agent.schedule_task")
	steps := make([]domain.AgentPlanStep, 0, len(capabilityKeys))
	allowedScopes := make([]string, 0, len(capabilityKeys))
	decisions := make([]PolicyResult, 0, len(capabilityKeys))
	for _, key := range capabilityKeys {
		capability, ok := p.registry.Get(key)
		if !ok {
			decisions = append(decisions, PolicyResult{Decision: PolicyDecisionForbidden, Reason: "capability is not registered"})
			continue
		}
		decision := p.policy.Decide(ctx, PolicyInput{Capability: capability, UserID: input.UserID})
		decisions = append(decisions, decision)
		if decision.Decision != PolicyDecisionForbidden {
			allowedScopes = append(allowedScopes, capability.Key)
		}
		steps = append(steps, domain.AgentPlanStep{
			StepOrder:       len(steps) + 1,
			Status:          domain.AgentPlanStepStatusPending,
			CapabilityKey:   capability.Key,
			CapabilityScope: []string{capability.Key},
			Title:           capability.Name,
			InputSummary:    safePlanText(input.Goal, 500),
			ExpectedOutput:  expectedCapabilityOutput(capability.Key),
			FailureStrategy: "retry once when failure is transient; otherwise return structured observation to controller and ask for clarification or stop",
			MaxRetries:      1,
			RetryMetadata: domain.AgentJSON{
				"permission": capabilityPermissionMetadata(capability, decision),
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	policyDecision, policyReason := aggregatePolicy(decisions)
	if scheduleConfirmed && policyDecision == PolicyDecisionPrompt {
		policyDecision = PolicyDecisionAllow
		policyReason = "user explicitly confirmed previous scheduled task request"
	}
	status := domain.AgentPlanStatusDraft
	confirmationPolicy := "auto"
	switch policyDecision {
	case PolicyDecisionAllow:
		status = domain.AgentPlanStatusApproved
		confirmationPolicy = "auto"
	case PolicyDecisionPrompt:
		status = domain.AgentPlanStatusAwaitingApproval
		confirmationPolicy = "prompt"
	case PolicyDecisionForbidden:
		status = domain.AgentPlanStatusFailed
		confirmationPolicy = "forbidden"
	}
	riskLevel := aggregateRiskLevel(p.registry, capabilityKeys)
	plan := domain.AgentPlan{
		UserID:             input.UserID,
		SessionID:          input.SessionID,
		TurnID:             input.TurnID,
		ControllerRunID:    input.ControllerRunID,
		Status:             status,
		Goal:               safePlanText(input.Goal, 1000),
		Summary:            planSummary(input.Goal, capabilityKeys),
		ImpactSummary:      impactSummary(capabilityKeys),
		RiskLevel:          riskLevel,
		ConfirmationPolicy: confirmationPolicy,
		AllowedScopes:      allowedScopes,
		DedupeKey:          planDedupeKey(input.UserID, input.SessionID, input.TurnID, input.Goal),
		PolicyDecision:     string(policyDecision),
		PolicyReason:       policyReason,
		Metadata: domain.AgentJSON{
			"planner":               "deterministic-minimal-v2",
			"capabilities":          capabilityKeys,
			"planned_at":            now.Format(time.RFC3339),
			"external_read":         containsExternalCapability(p.registry, capabilityKeys),
			"permission_governance": p.permissionGovernance(capabilityKeys, decisions),
			"budget_governance":     p.budgetGovernance(input.Goal, capabilityKeys),
			"planner_quality":       plannerQualityChecks(input.Goal, capabilityKeys, steps, policyDecision),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if status == domain.AgentPlanStatusApproved {
		approvedAt := now
		plan.ApprovedAt = &approvedAt
	}
	return PlanOutput{Plan: plan, Steps: steps}
}

func (p *Planner) selectCapabilities(goal string) []string {
	text := strings.ToLower(strings.TrimSpace(goal))
	keys := []string{"feed.query_recent_items"}
	if strings.Contains(text, "来源") || strings.Contains(text, "source") {
		keys = append(keys, "source.query_latest_items")
	}
	if strings.Contains(text, "历史") || strings.Contains(text, "之前") || strings.Contains(text, "记得") || strings.Contains(text, "聊天") || strings.Contains(text, "偏好") || strings.Contains(text, "第一条") || strings.Contains(text, "最早") {
		keys = append(keys, "conversation.query_history")
	}
	if looksLikeScheduleRequest(text) || looksLikeScheduleConfirmation(text) {
		keys = append(keys, "agent.schedule_task")
	}
	if looksLikeWebRequest(text) {
		keys = append(keys, "web.search")
		if containsURL(text) {
			keys = append(keys, "web.fetch_page", "web.extract_page")
		}
	}
	if looksLikeRepoRequest(text) {
		keys = append(keys, "repo.search")
		if containsRepoRef(text) {
			keys = append(keys, "repo.inspect_remote")
		}
	}
	return uniqueStrings(keys)
}

func looksLikeScheduleRequest(text string) bool {
	for _, keyword := range []string{"提醒", "定时", "稍后", "明天", "后天", "每天", "每周", "日报", "周报", "到点", "闹钟", "schedule", "remind"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func looksLikeScheduleConfirmation(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, keyword := range []string{"确认创建", "确认上一条", "上一条任务", "上一条提醒", "是的", "确认", "可以", "对"} {
		if strings.Contains(trimmed, keyword) {
			return true
		}
	}
	return false
}

func looksLikeWebRequest(text string) bool {
	if containsURL(text) {
		return true
	}
	for _, keyword := range []string{"联网", "网页", "网站", "搜索", "最新", "新闻", "查一下", "fetch", "http"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func containsURL(text string) bool {
	for _, field := range strings.Fields(text) {
		field = strings.Trim(field, "，。,. ")
		parsed, err := url.Parse(field)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			return true
		}
	}
	return strings.Contains(text, "http://") || strings.Contains(text, "https://")
}

func looksLikeRepoRequest(text string) bool {
	for _, keyword := range []string{"github", "仓库", "repo", "repository", "代码库", "开源项目"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func containsRepoRef(text string) bool {
	if strings.Contains(text, "github.com/") {
		return true
	}
	for _, field := range strings.Fields(text) {
		parts := strings.Split(strings.Trim(field, "，。,. "), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return true
		}
	}
	return false
}

func aggregatePolicy(results []PolicyResult) (PolicyDecision, string) {
	if len(results) == 0 {
		return PolicyDecisionForbidden, "no executable capability selected"
	}
	reasons := make([]string, 0, len(results))
	decision := PolicyDecisionAllow
	for _, result := range results {
		if result.Reason != "" {
			reasons = append(reasons, result.Reason)
		}
		if result.Decision == PolicyDecisionForbidden {
			decision = PolicyDecisionForbidden
		}
		if decision != PolicyDecisionForbidden && result.Decision == PolicyDecisionPrompt {
			decision = PolicyDecisionPrompt
		}
	}
	return decision, strings.Join(uniqueStrings(reasons), "; ")
}

func aggregateRiskLevel(registry *CapabilityRegistry, keys []string) string {
	risk := CapabilityRiskLow
	for _, key := range keys {
		capability, ok := registry.Get(key)
		if !ok {
			continue
		}
		if capability.Risk == CapabilityRiskHigh {
			return string(CapabilityRiskHigh)
		}
		if capability.Risk == CapabilityRiskMedium {
			risk = CapabilityRiskMedium
		}
	}
	return string(risk)
}

func containsExternalCapability(registry *CapabilityRegistry, keys []string) bool {
	for _, key := range keys {
		capability, ok := registry.Get(key)
		if ok && capability.ExternalAccess {
			return true
		}
	}
	return false
}

func (p *Planner) permissionGovernance(keys []string, decisions []PolicyResult) domain.AgentJSON {
	items := make([]domain.AgentJSON, 0, len(keys))
	for index, key := range keys {
		capability, ok := p.registry.Get(key)
		if !ok {
			continue
		}
		decision := PolicyResult{Decision: PolicyDecisionForbidden, Reason: "capability is not registered"}
		if index < len(decisions) {
			decision = decisions[index]
		}
		items = append(items, capabilityPermissionMetadata(capability, decision))
	}
	return domain.AgentJSON{
		"items":                 items,
		"has_external_access":   containsExternalCapability(p.registry, keys),
		"has_state_change":      containsMutatingCapability(p.registry, keys),
		"requires_confirmation": requiresPrompt(decisions),
		"boundary":              "only registered capability scopes may execute; mutating and scheduled capabilities require approval",
	}
}

func capabilityPermissionMetadata(capability Capability, decision PolicyResult) domain.AgentJSON {
	decisionValue := string(decision.Decision)
	if decisionValue == "" {
		decisionValue = string(PolicyDecisionForbidden)
	}
	return domain.AgentJSON{
		"capability_key":        capability.Key,
		"risk":                  string(capability.Risk),
		"data_domain":           capability.DataDomain,
		"mode":                  string(capability.Mode),
		"external_access":       capability.ExternalAccess,
		"mutates":               capability.Mutates,
		"schedulable":           capability.Schedulable,
		"reusable":              capability.Reusable,
		"decision":              decisionValue,
		"reason":                decision.Reason,
		"requires_confirmation": decision.Decision == PolicyDecisionPrompt,
	}
}

func containsMutatingCapability(registry *CapabilityRegistry, keys []string) bool {
	for _, key := range keys {
		capability, ok := registry.Get(key)
		if ok && capability.Mutates {
			return true
		}
	}
	return false
}

func requiresPrompt(decisions []PolicyResult) bool {
	for _, decision := range decisions {
		if decision.Decision == PolicyDecisionPrompt {
			return true
		}
	}
	return false
}

func (p *Planner) budgetGovernance(goal string, keys []string) domain.AgentJSON {
	toolCalls := len(keys)
	externalCalls := 0
	for _, key := range keys {
		if capability, ok := p.registry.Get(key); ok && capability.ExternalAccess {
			externalCalls++
		}
	}
	contextChars := len([]rune(goal))
	contextBudget := 6000
	toolBudget := 8
	externalBudget := 4
	replyBudget := 768
	status := "within_budget"
	degradation := ""
	if contextChars > contextBudget || toolCalls > toolBudget || externalCalls > externalBudget {
		status = "degraded"
		degradation = "reduce sources, shorten time range, or ask for confirmation before continuing"
	}
	return domain.AgentJSON{
		"status":                status,
		"context_chars":         contextChars,
		"context_budget_chars":  contextBudget,
		"tool_calls":            toolCalls,
		"tool_call_budget":      toolBudget,
		"external_calls":        externalCalls,
		"external_call_budget":  externalBudget,
		"reply_token_budget":    replyBudget,
		"degradation_strategy":  degradation,
		"budget_policy_version": "deterministic-budget-v1",
	}
}

func plannerQualityChecks(goal string, keys []string, steps []domain.AgentPlanStep, policyDecision PolicyDecision) domain.AgentJSON {
	checks := []domain.AgentJSON{
		{"key": "goal_coverage", "status": qualityStatus(strings.TrimSpace(goal) != "" && len(steps) > 0), "summary": "selected capabilities should cover the user goal"},
		{"key": "evidence_required", "status": qualityStatus(capabilitySetHasEvidence(keys)), "summary": "plan should include evidence-producing capability or local source reference"},
		{"key": "risk_explained", "status": qualityStatus(policyDecision != ""), "summary": "plan records policy decision and risk level"},
		{"key": "failure_strategy", "status": qualityStatus(allStepsHaveFailureStrategy(steps)), "summary": "each step has a bounded retry or stop strategy"},
	}
	status := "passed"
	for _, check := range checks {
		if check["status"] != "passed" {
			status = "failed"
			break
		}
	}
	return domain.AgentJSON{"status": status, "checks": checks}
}

func qualityStatus(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func capabilitySetHasEvidence(keys []string) bool {
	for _, key := range keys {
		if strings.HasPrefix(key, "feed.") || strings.HasPrefix(key, "source.") || strings.HasPrefix(key, "conversation.") || strings.HasPrefix(key, "web.") || strings.HasPrefix(key, "repo.") {
			return true
		}
	}
	return false
}

func allStepsHaveFailureStrategy(steps []domain.AgentPlanStep) bool {
	if len(steps) == 0 {
		return false
	}
	for _, step := range steps {
		if strings.TrimSpace(step.FailureStrategy) == "" {
			return false
		}
	}
	return true
}

func expectedCapabilityOutput(key string) string {
	switch key {
	case "conversation.query_history":
		return "transcript entry references and exact message snippets"
	case "web.search":
		return "candidate result URLs with fetched source and summary"
	case "web.fetch_page":
		return "HTTP metadata and bounded page body snippet"
	case "web.extract_page":
		return "title, site, publication metadata, summary and links"
	case "agent.schedule_message":
		return "confirmation request or queued notification job reference"
	case "agent.schedule_task":
		return "confirmation request or persisted scheduled agent task reference"
	case "repo.search":
		return "repository candidates with URL, description, language, license and update time"
	case "repo.inspect_remote":
		return "remote repository metadata, README summary and license without local clone"
	default:
		return "structured observation and user-visible summary"
	}
}

func planSummary(goal string, capabilityKeys []string) string {
	return fmt.Sprintf("plan for %q using %s", safePlanText(goal, 120), strings.Join(capabilityKeys, ", "))
}

func impactSummary(capabilityKeys []string) string {
	if containsString(capabilityKeys, "agent.schedule_task") || containsString(capabilityKeys, "agent.schedule_message") {
		return "may create a scheduled agent task or outbound notification after explicit confirmation"
	}
	if containsString(capabilityKeys, "web.search") || containsString(capabilityKeys, "web.fetch_page") || containsString(capabilityKeys, "web.extract_page") || containsString(capabilityKeys, "repo.search") || containsString(capabilityKeys, "repo.inspect_remote") {
		return "performs bounded external HTTP reads and records sources"
	}
	return "read-only local context and feed query"
}

func planDedupeKey(userID int64, sessionID int64, turnID int64, goal string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%s", userID, sessionID, turnID, strings.TrimSpace(goal))))
	return fmt.Sprintf("agent_plan:%x", sum[:16])
}

func safePlanText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	return output
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func SortedCapabilityKeys(capabilities []Capability) []string {
	keys := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		keys = append(keys, capability.Key)
	}
	sort.Strings(keys)
	return keys
}
