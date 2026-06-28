package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"messagefeed/internal/domain"
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

type PlanningComplexity string

const (
	PlanningComplexitySimple   PlanningComplexity = "simple"
	PlanningComplexityStandard PlanningComplexity = "standard"
	PlanningComplexityComplex  PlanningComplexity = "complex"
)

type PlanSpec struct {
	Goal                   string
	Intent                 string
	TaskType               string
	Complexity             PlanningComplexity
	RequiresSubAgent       bool
	DirectAnswerAllowed    bool
	RequiredCapabilities   []string
	Subtasks               []PlanSubtask
	EvidenceRequirements   []PlanEvidenceRequirement
	MaxIterations          int
	FinalAnswerConstraints []string
	Metadata               domain.AgentJSON
}

type PlanSubtask struct {
	Title           string
	Prompt          string
	ContextSummary  string
	CapabilityKeys  []string
	ExpectedOutput  string
	FailureStrategy string
	MaxRetries      int
}

type PlanEvidenceRequirement struct {
	EvidenceType string
	Summary      string
	MinimumCount int
	Freshness    string
	Required     bool
}

type PlanOutput struct {
	Plan  domain.AgentPlan
	Steps []domain.AgentPlanStep
}

func (p *Planner) Build(ctx context.Context, input PlanInput) PlanOutput {
	return p.build(ctx, input, plannerBuildRequest{
		CapabilityKeys: p.defaultFallbackCapabilities(),
		PlannerName:    "fallback-local-context-v1",
		ExecutionMode:  "fallback_local_context",
	})
}

func (p *Planner) BuildFromSpec(ctx context.Context, input PlanInput, spec PlanSpec) PlanOutput {
	normalized := normalizePlanSpec(input.Goal, spec)
	capabilityKeys := capabilityKeysFromPlanSpec(normalized)
	executionMode := "subagent_execution"
	if normalized.DirectAnswerAllowed && len(capabilityKeys) == 0 {
		executionMode = "direct_answer"
	}
	return p.build(ctx, input, plannerBuildRequest{
		CapabilityKeys:       capabilityKeys,
		PlannerName:          "main-agent-structured-v1",
		Summary:              structuredPlanSummary(input.Goal, normalized, capabilityKeys),
		ImpactSummary:        structuredImpactSummary(normalized, capabilityKeys),
		ExecutionMode:        executionMode,
		Spec:                 normalized,
		HasSpec:              true,
		AllowDirectExecution: normalized.DirectAnswerAllowed && len(capabilityKeys) == 0,
	})
}

type plannerBuildRequest struct {
	CapabilityKeys       []string
	PlannerName          string
	Summary              string
	ImpactSummary        string
	ExecutionMode        string
	Spec                 PlanSpec
	HasSpec              bool
	AllowDirectExecution bool
}

func (p *Planner) build(ctx context.Context, input PlanInput, request plannerBuildRequest) PlanOutput {
	now := p.now().UTC()
	capabilityKeys := uniqueStrings(request.CapabilityKeys)
	stepRequests := p.stepRequests(input.Goal, capabilityKeys, request)
	steps := make([]domain.AgentPlanStep, 0, len(stepRequests))
	allowedScopes := make([]string, 0, len(capabilityKeys))
	decisions := make([]PolicyResult, 0, len(capabilityKeys))
	for _, stepRequest := range stepRequests {
		key := stepRequest.CapabilityKey
		capability, ok := p.registry.Get(key)
		if !ok {
			decisions = append(decisions, PolicyResult{Decision: PolicyDecisionForbidden, Reason: "能力未注册"})
			continue
		}
		decision := p.policy.Decide(ctx, PolicyInput{Capability: capability, UserID: input.UserID})
		decision.Reason = plannerPolicyReasonText(decision, capability)
		decisions = append(decisions, decision)
		if decision.Decision != PolicyDecisionForbidden {
			allowedScopes = append(allowedScopes, capability.Key)
		}
		steps = append(steps, domain.AgentPlanStep{
			StepOrder:       len(steps) + 1,
			Status:          domain.AgentPlanStepStatusPending,
			CapabilityKey:   capability.Key,
			CapabilityScope: []string{capability.Key},
			Title:           planStepTitle(stepRequest, capability),
			InputSummary:    safePlanText(firstNonEmpty(stepRequest.InputSummary, input.Goal), 500),
			ExpectedOutput:  firstNonEmpty(stepRequest.ExpectedOutput, expectedCapabilityOutput(capability.Key)),
			FailureStrategy: firstNonEmpty(stepRequest.FailureStrategy, defaultStepFailureStrategy()),
			MaxRetries:      boundedStepRetries(stepRequest.MaxRetries),
			RetryMetadata:   stepRetryMetadata(stepRequest, capability, decision),
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	policyDecision, policyReason := aggregatePolicyForRequest(decisions, request.AllowDirectExecution)
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
	summary := firstNonEmpty(request.Summary, planSummary(input.Goal, capabilityKeys))
	impact := firstNonEmpty(request.ImpactSummary, impactSummary(capabilityKeys))
	plannerName := firstNonEmpty(request.PlannerName, "deterministic-minimal-v2")
	executionMode := firstNonEmpty(request.ExecutionMode, "tool_execution")
	metadata := domain.AgentJSON{
		"planner":               plannerName,
		"capabilities":          capabilityKeys,
		"planned_at":            now.Format(time.RFC3339),
		"external_read":         containsExternalCapability(p.registry, capabilityKeys),
		"execution_mode":        executionMode,
		"permission_governance": p.permissionGovernance(capabilityKeys, decisions),
		"budget_governance":     p.budgetGovernance(input.Goal, capabilityKeys),
		"planner_quality":       plannerQualityChecks(input.Goal, capabilityKeys, steps, policyDecision, request.AllowDirectExecution),
	}
	if request.HasSpec {
		metadata["main_agent_plan"] = planSpecMetadata(request.Spec)
		metadata["complexity"] = string(request.Spec.Complexity)
		metadata["max_iterations"] = request.Spec.MaxIterations
	}
	plan := domain.AgentPlan{
		UserID:             input.UserID,
		SessionID:          input.SessionID,
		TurnID:             input.TurnID,
		ControllerRunID:    input.ControllerRunID,
		Status:             status,
		Goal:               safePlanText(input.Goal, 1000),
		Summary:            summary,
		ImpactSummary:      impact,
		RiskLevel:          riskLevel,
		ConfirmationPolicy: confirmationPolicy,
		AllowedScopes:      allowedScopes,
		DedupeKey:          planDedupeKey(input.UserID, input.SessionID, input.TurnID, input.Goal),
		PolicyDecision:     string(policyDecision),
		PolicyReason:       policyReason,
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if status == domain.AgentPlanStatusApproved {
		approvedAt := now
		plan.ApprovedAt = &approvedAt
	}
	return PlanOutput{Plan: plan, Steps: steps}
}

type plannerStepRequest struct {
	CapabilityKey    string
	Title            string
	InputSummary     string
	ExpectedOutput   string
	FailureStrategy  string
	MaxRetries       int
	SubtaskIndex     int
	SubtaskPrompt    string
	ContextSummary   string
	ExecutionMode    string
	EvidenceRequired []PlanEvidenceRequirement
}

func (p *Planner) stepRequests(goal string, capabilityKeys []string, request plannerBuildRequest) []plannerStepRequest {
	if !request.HasSpec {
		requests := make([]plannerStepRequest, 0, len(capabilityKeys))
		for _, key := range capabilityKeys {
			requests = append(requests, plannerStepRequest{
				CapabilityKey:   key,
				InputSummary:    goal,
				ExpectedOutput:  expectedCapabilityOutput(key),
				FailureStrategy: defaultStepFailureStrategy(),
				MaxRetries:      1,
				ExecutionMode:   request.ExecutionMode,
			})
		}
		return requests
	}
	seen := map[string]struct{}{}
	requests := []plannerStepRequest{}
	for index, subtask := range request.Spec.Subtasks {
		for _, key := range uniqueStrings(subtask.CapabilityKeys) {
			seen[key] = struct{}{}
			requests = append(requests, plannerStepRequest{
				CapabilityKey:    key,
				Title:            subtask.Title,
				InputSummary:     firstNonEmpty(subtask.Prompt, subtask.ContextSummary, goal),
				ExpectedOutput:   subtask.ExpectedOutput,
				FailureStrategy:  subtask.FailureStrategy,
				MaxRetries:       subtask.MaxRetries,
				SubtaskIndex:     index + 1,
				SubtaskPrompt:    subtask.Prompt,
				ContextSummary:   subtask.ContextSummary,
				ExecutionMode:    request.ExecutionMode,
				EvidenceRequired: request.Spec.EvidenceRequirements,
			})
		}
	}
	for _, key := range capabilityKeys {
		if _, ok := seen[key]; ok {
			continue
		}
		requests = append(requests, plannerStepRequest{
			CapabilityKey:    key,
			InputSummary:     firstNonEmpty(request.Spec.Intent, goal),
			ExpectedOutput:   expectedCapabilityOutput(key),
			FailureStrategy:  defaultStepFailureStrategy(),
			MaxRetries:       1,
			ExecutionMode:    request.ExecutionMode,
			EvidenceRequired: request.Spec.EvidenceRequirements,
		})
	}
	return requests
}

func (p *Planner) defaultFallbackCapabilities() []string {
	return []string{"feed.query_recent_items"}
}

func normalizePlanSpec(goal string, spec PlanSpec) PlanSpec {
	spec.Goal = safePlanText(firstNonEmpty(spec.Goal, goal), 1000)
	spec.Intent = safePlanText(firstNonEmpty(spec.Intent, spec.Goal), 1000)
	spec.TaskType = strings.TrimSpace(spec.TaskType)
	if spec.TaskType == "" {
		spec.TaskType = "general"
	}
	spec.RequiredCapabilities = uniqueStrings(spec.RequiredCapabilities)
	normalizedSubtasks := make([]PlanSubtask, 0, len(spec.Subtasks))
	for _, subtask := range spec.Subtasks {
		subtask.Title = safePlanText(subtask.Title, 120)
		subtask.Prompt = safePlanText(subtask.Prompt, 1000)
		subtask.ContextSummary = safePlanText(subtask.ContextSummary, 1000)
		subtask.CapabilityKeys = uniqueStrings(subtask.CapabilityKeys)
		subtask.ExpectedOutput = safePlanText(subtask.ExpectedOutput, 500)
		subtask.FailureStrategy = safePlanText(subtask.FailureStrategy, 500)
		if len(subtask.CapabilityKeys) == 0 && strings.TrimSpace(subtask.Prompt) == "" && strings.TrimSpace(subtask.Title) == "" {
			continue
		}
		normalizedSubtasks = append(normalizedSubtasks, subtask)
	}
	spec.Subtasks = normalizedSubtasks
	spec.EvidenceRequirements = normalizeEvidenceRequirements(spec.EvidenceRequirements)
	spec.FinalAnswerConstraints = normalizeTextList(spec.FinalAnswerConstraints, 300)
	spec.Complexity = normalizePlanningComplexity(spec.Complexity, spec)
	spec.MaxIterations = normalizeMaxIterations(spec.MaxIterations, spec)
	if spec.Metadata == nil {
		spec.Metadata = domain.AgentJSON{}
	}
	return spec
}

func normalizeEvidenceRequirements(requirements []PlanEvidenceRequirement) []PlanEvidenceRequirement {
	output := make([]PlanEvidenceRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		requirement.EvidenceType = safePlanText(requirement.EvidenceType, 80)
		requirement.Summary = safePlanText(requirement.Summary, 300)
		requirement.Freshness = safePlanText(requirement.Freshness, 120)
		if requirement.MinimumCount < 0 {
			requirement.MinimumCount = 0
		}
		if requirement.MinimumCount > 20 {
			requirement.MinimumCount = 20
		}
		if requirement.EvidenceType == "" && requirement.Summary == "" {
			continue
		}
		output = append(output, requirement)
	}
	return output
}

func normalizeTextList(values []string, limit int) []string {
	if len(values) == 0 {
		return nil
	}
	output := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = safePlanText(value, limit)
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

func normalizePlanningComplexity(value PlanningComplexity, spec PlanSpec) PlanningComplexity {
	switch PlanningComplexity(strings.ToLower(strings.TrimSpace(string(value)))) {
	case PlanningComplexitySimple:
		return PlanningComplexitySimple
	case PlanningComplexityComplex:
		return PlanningComplexityComplex
	case PlanningComplexityStandard:
		return PlanningComplexityStandard
	default:
		return inferPlanningComplexity(spec)
	}
}

func inferPlanningComplexity(spec PlanSpec) PlanningComplexity {
	capabilityCount := len(capabilityKeysFromPlanSpec(spec))
	if spec.DirectAnswerAllowed && capabilityCount == 0 {
		return PlanningComplexitySimple
	}
	if spec.RequiresSubAgent || len(spec.Subtasks) > 1 || capabilityCount > 2 || len(spec.EvidenceRequirements) > 2 {
		return PlanningComplexityComplex
	}
	if capabilityCount > 0 || len(spec.Subtasks) == 1 {
		return PlanningComplexityStandard
	}
	return PlanningComplexitySimple
}

func normalizeMaxIterations(value int, spec PlanSpec) int {
	if value < 0 {
		return 0
	}
	if value > 3 {
		return 3
	}
	if value > 0 {
		return value
	}
	if spec.RequiresSubAgent || spec.Complexity == PlanningComplexityComplex {
		return 1
	}
	return 0
}

func capabilityKeysFromPlanSpec(spec PlanSpec) []string {
	keys := append([]string(nil), spec.RequiredCapabilities...)
	for _, subtask := range spec.Subtasks {
		keys = append(keys, subtask.CapabilityKeys...)
	}
	return uniqueStrings(keys)
}

func structuredPlanSummary(goal string, spec PlanSpec, capabilityKeys []string) string {
	intent := firstNonEmpty(spec.Intent, goal)
	if len(capabilityKeys) == 0 && spec.DirectAnswerAllowed {
		return fmt.Sprintf("主 Agent 直接回答计划：%q", safePlanText(intent, 120))
	}
	return fmt.Sprintf("主 Agent 执行计划：%q；能力范围：%s", safePlanText(intent, 120), strings.Join(capabilityKeys, ", "))
}

func structuredImpactSummary(spec PlanSpec, capabilityKeys []string) string {
	if len(capabilityKeys) == 0 && spec.DirectAnswerAllowed {
		return "主 Agent 可在不调用工具的情况下直接回答。"
	}
	if spec.RequiresSubAgent || len(spec.Subtasks) > 0 {
		return "主 Agent 在后端权限和预算校验后派发有界子 Agent 执行。"
	}
	return impactSummary(capabilityKeys)
}

func planSpecMetadata(spec PlanSpec) domain.AgentJSON {
	subtasks := make([]domain.AgentJSON, 0, len(spec.Subtasks))
	for index, subtask := range spec.Subtasks {
		subtasks = append(subtasks, domain.AgentJSON{
			"index":            index + 1,
			"title":            subtask.Title,
			"prompt":           subtask.Prompt,
			"context_summary":  subtask.ContextSummary,
			"capability_keys":  append([]string(nil), subtask.CapabilityKeys...),
			"expected_output":  subtask.ExpectedOutput,
			"failure_strategy": subtask.FailureStrategy,
			"max_retries":      subtask.MaxRetries,
		})
	}
	return domain.AgentJSON{
		"goal":                     spec.Goal,
		"intent":                   spec.Intent,
		"task_type":                spec.TaskType,
		"complexity":               string(spec.Complexity),
		"requires_sub_agent":       spec.RequiresSubAgent,
		"direct_answer_allowed":    spec.DirectAnswerAllowed,
		"required_capabilities":    append([]string(nil), spec.RequiredCapabilities...),
		"subtasks":                 subtasks,
		"evidence_requirements":    evidenceRequirementMetadata(spec.EvidenceRequirements),
		"max_iterations":           spec.MaxIterations,
		"final_answer_constraints": append([]string(nil), spec.FinalAnswerConstraints...),
		"metadata":                 clonePlannerAgentJSON(spec.Metadata),
	}
}

func evidenceRequirementMetadata(requirements []PlanEvidenceRequirement) []domain.AgentJSON {
	items := make([]domain.AgentJSON, 0, len(requirements))
	for _, requirement := range requirements {
		items = append(items, domain.AgentJSON{
			"evidence_type": requirement.EvidenceType,
			"summary":       requirement.Summary,
			"minimum_count": requirement.MinimumCount,
			"freshness":     requirement.Freshness,
			"required":      requirement.Required,
		})
	}
	return items
}

func clonePlannerAgentJSON(input domain.AgentJSON) domain.AgentJSON {
	if input == nil {
		return domain.AgentJSON{}
	}
	output := make(domain.AgentJSON, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func aggregatePolicyForRequest(results []PolicyResult, allowDirect bool) (PolicyDecision, string) {
	if len(results) == 0 && allowDirect {
		return PolicyDecisionAllow, "主 Agent 选择直接回答，不需要工具执行。"
	}
	return aggregatePolicy(results)
}

func planStepTitle(request plannerStepRequest, capability Capability) string {
	if strings.TrimSpace(request.Title) != "" {
		return safePlanText(request.Title, 120)
	}
	return capability.Name
}

func defaultStepFailureStrategy() string {
	return "瞬时失败时最多重试一次；否则向主 Agent 返回结构化观察，并请求澄清或停止执行。"
}

func boundedStepRetries(value int) int {
	if value < 1 {
		return 1
	}
	if value > 3 {
		return 3
	}
	return value
}

func stepRetryMetadata(request plannerStepRequest, capability Capability, decision PolicyResult) domain.AgentJSON {
	metadata := domain.AgentJSON{
		"permission": capabilityPermissionMetadata(capability, decision),
	}
	if request.ExecutionMode != "" || request.SubtaskIndex > 0 || request.SubtaskPrompt != "" || request.ContextSummary != "" || len(request.EvidenceRequired) > 0 {
		metadata["sub_agent"] = domain.AgentJSON{
			"execution_mode":         request.ExecutionMode,
			"subtask_index":          request.SubtaskIndex,
			"prompt":                 request.SubtaskPrompt,
			"context_summary":        request.ContextSummary,
			"evidence_requirements":  evidenceRequirementMetadata(request.EvidenceRequired),
			"capability_bound_scope": []string{capability.Key},
		}
	}
	return metadata
}

func aggregatePolicy(results []PolicyResult) (PolicyDecision, string) {
	if len(results) == 0 {
		return PolicyDecisionForbidden, "未选择可执行能力。"
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

func plannerPolicyReasonText(result PolicyResult, capability Capability) string {
	reason := strings.TrimSpace(result.Reason)
	switch reason {
	case "missing authenticated user":
		return "缺少已认证用户。"
	case "capability is not registered":
		return "能力未注册。"
	case "state-changing capability requires approval":
		return "状态变更类能力需要确认。"
	case "scheduled capability requires approval":
		return "定时类能力需要确认。"
	case "high risk capability requires approval":
		return "高风险能力需要确认。"
	case "tool-level confirmation enforced":
		return "该能力允许进入工具级确认校验；未确认时工具只返回确认请求。"
	case "external read-only capability with bounded fetch policy":
		return "外部只读能力允许在有界抓取策略内执行。"
	case "read-only capability":
		return "只读能力允许执行。"
	}
	if reason != "" {
		return reason
	}
	if result.Decision == PolicyDecisionForbidden {
		return "能力策略拒绝执行。"
	}
	if result.Decision == PolicyDecisionPrompt || capability.Mutates || capability.Schedulable || capability.Risk == CapabilityRiskHigh {
		return "该能力需要确认后执行。"
	}
	return "能力策略允许执行。"
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
		decision := PolicyResult{Decision: PolicyDecisionForbidden, Reason: "能力未注册"}
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
		"boundary":              "仅允许执行已注册的能力范围；变更状态或定时类能力必须先经过确认。",
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
		"tool_confirmation":     capabilityUsesToolConfirmation(capability),
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
	// 0 表示回复生成阶段暂不设置请求层最高 token 上限。
	replyBudget := 0
	status := "within_budget"
	degradation := ""
	if contextChars > contextBudget || toolCalls > toolBudget || externalCalls > externalBudget {
		status = "degraded"
		degradation = "减少来源数量、缩短时间范围，或在继续前请求确认。"
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

func plannerQualityChecks(goal string, keys []string, steps []domain.AgentPlanStep, policyDecision PolicyDecision, directAnswer bool) domain.AgentJSON {
	checks := []domain.AgentJSON{
		{"key": "goal_coverage", "status": qualityStatus(strings.TrimSpace(goal) != "" && (len(steps) > 0 || directAnswer)), "summary": "已选择的能力或直接回答模式应覆盖用户目标。"},
		{"key": "evidence_required", "status": qualityStatus(directAnswer || capabilitySetHasEvidence(keys)), "summary": "计划应包含可产出证据的能力、本地来源引用或直接回答模式。"},
		{"key": "risk_explained", "status": qualityStatus(policyDecision != ""), "summary": "计划应记录策略裁决和风险级别。"},
		{"key": "failure_strategy", "status": qualityStatus(directAnswer || allStepsHaveFailureStrategy(steps)), "summary": "每个可执行步骤都应具有有界重试或停止策略。"},
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
		return "对话记录引用和原始消息片段。"
	case "web.search":
		return "候选结果 URL、来源信息和摘要。"
	case "web.fetch_page":
		return "HTTP 元数据和受限长度的页面正文片段。"
	case "web.extract_page":
		return "标题、站点、发布时间元数据、摘要和主要链接。"
	case "agent.schedule_message":
		return "确认请求或已入队通知任务引用。"
	case "agent.schedule_task":
		return "确认请求或已持久化的定时 Agent 任务引用。"
	case "repo.search":
		return "候选仓库 URL、描述、语言、许可证和更新时间。"
	case "repo.inspect_remote":
		return "远端仓库元数据、README 摘要和许可证信息，不克隆到本地。"
	default:
		return "结构化观察和面向用户的摘要。"
	}
}

func planSummary(goal string, capabilityKeys []string) string {
	return fmt.Sprintf("执行计划：%q；能力范围：%s", safePlanText(goal, 120), strings.Join(capabilityKeys, ", "))
}

func impactSummary(capabilityKeys []string) string {
	if containsString(capabilityKeys, "agent.schedule_task") || containsString(capabilityKeys, "agent.schedule_message") {
		return "明确确认后可能创建定时 Agent 任务或外发通知。"
	}
	if containsString(capabilityKeys, "web.search") || containsString(capabilityKeys, "web.fetch_page") || containsString(capabilityKeys, "web.extract_page") || containsString(capabilityKeys, "repo.search") || containsString(capabilityKeys, "repo.inspect_remote") {
		return "执行有界外部 HTTP 只读访问，并记录来源。"
	}
	return "只读访问本地上下文和订阅条目。"
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
