package service

import (
	"context"
	"encoding/json"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgentRecordedSemanticReview(test *testing.T) {
	path := os.Getenv("AGENT_RECORDED_SEMANTIC_REVIEW")
	if path == "" {
		test.Skip("提供实际质量评价记录后执行协议验收")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		test.Fatal(err)
	}
	var recorded struct {
		Raw          string
		Reply        string
		Sources      []domain.AgentFactSource
		Requirements []agent.PlanEvidenceRequirement
		Now          time.Time
	}
	if err := json.Unmarshal(encoded, &recorded); err != nil {
		test.Fatal(err)
	}
	review, err := agentAnswerParseSemanticReview(recorded.Raw, recorded.Reply, recorded.Sources, recorded.Requirements, recorded.Now)
	if err != nil || review.Status != "passed" {
		test.Fatalf("实际质量评价未通过解析与引用校验：%v，状态：%s", err, review.Status)
	}
}

func TestAgentAnswerQualityUsesCurrentFormatAndActualReply(test *testing.T) {
	formats, _, requireJSON, err := agentAnswerEffectiveConstraints([]string{"只回答十个汉字"}, "只回复三个大写英文字母")
	if err != nil || requireJSON || agentAnswerFormatViolation("GEN", formats, false) != "" {
		test.Fatalf("当前格式未覆盖历史格式：%v", err)
	}
	for _, invalid := range []string{"GEN。", "gen", "中国队", "GEN\n结果"} {
		if agentAnswerFormatViolation(invalid, formats, false) == "" {
			test.Fatalf("未拒绝违反格式的候选回答：%q", invalid)
		}
	}
	service := &AgentConversationService{}
	result, err := service.evaluateAgentAnswerQuality(context.Background(), agentAnswerQualityInput{Reply: "GEN", CurrentMessage: "只回复三个大写英文字母", Now: time.Now().UTC()})
	if err != nil || !result.Passed || result.AnswerQuality["reply_char_count"] != 3 || result.AnswerQuality["reply_hash"] != agentAnswerContentHash("GEN") {
		test.Fatalf("候选正文未被实际校验：%#v，%v", result, err)
	}
}

func TestAgentAnswerQualityKeepsUnrecognizedCurrentConstraint(test *testing.T) {
	_, semantic, _, err := agentAnswerEffectiveConstraints(nil, "必须按照时间顺序回答")
	if err != nil || len(semantic) == 0 || semantic[0] != "必须按照时间顺序回答" {
		test.Fatalf("当前语义约束丢失：%#v，%v", semantic, err)
	}
	result, err := (&AgentConversationService{}).evaluateAgentAnswerQuality(context.Background(), agentAnswerQualityInput{Reply: "已经完成。", CurrentMessage: "必须按照时间顺序回答", Now: time.Now().UTC()})
	if err == nil || result.Passed || result.Status != "unverified" {
		test.Fatalf("无法验证的语义约束被标记为通过：%#v", result)
	}
}

func TestAgentAnswerFormatSeparatesSourceDescriptionFromOutput(test *testing.T) {
	for _, current := range []string{
		"读取本会话最早一条用户消息中要求回复的三个大写英文字母，并转为小写。最终仅回复三个小写英文字母，不要附加解释。",
		"从十个汉字的原文提取代号，只回复三个小写英文字母。",
	} {
		formats, _, _, err := agentAnswerEffectiveConstraints([]string{"只回复三个大写英文字母"}, current)
		if err != nil || len(formats) != 1 || formats[0].Unit != "小写英文字母" || agentAnswerFormatViolation("abc", formats, false) != "" || agentAnswerFormatViolation("ABC", formats, false) == "" {
			test.Fatalf("来源描述影响当前输出要求：%+v，%v", formats, err)
		}
	}
	for _, current := range []string{"请仅回复这三个大写英文字母", "回复为三个大写英文字母", "字数为三个大写英文字母"} {
		formats, _, err := agentAnswerParseFormat(current)
		if err != nil || len(formats) != 1 || agentAnswerFormatViolation("ABC", formats, false) != "" || agentAnswerFormatViolation("AB", formats, false) == "" {
			test.Fatalf("明确输出要求未执行：%+v，%v", formats, err)
		}
	}
}

func TestAgentAnswerQualityRequestPreservesFollowupContext(test *testing.T) {
	createdAt := time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)
	input := agentAnswerQualityInput{
		Reply: "abc", CurrentMessage: "将刚才约定的代号转为小写",
		SourceMessages: []agent.ContextMessage{{Role: domain.AgentTranscriptRoleUser, Content: "本次约定的代号为 ABC", TranscriptEntryID: 91, TurnID: 12, CreatedAt: createdAt}, {Role: domain.AgentTranscriptRoleAssistant, Content: "ABC"}},
	}
	sources := append(agentAnswerConversationSources(input), domain.AgentFactSource{CanonicalRef: "artifact:320", Content: "外部事实正文", CreatedAt: createdAt})
	request, err := agentAnswerQualityRequest(input, []string{input.CurrentMessage}, nil, sources)
	if err != nil {
		test.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(request.Messages[len(request.Messages)-1].Content), &payload); err != nil {
		test.Fatal(err)
	}
	if !strings.Contains(string(payload["source_result"]), "本次约定的代号为 ABC") || !strings.Contains(string(payload["current_user_message"]), input.CurrentMessage) {
		test.Fatal("追问质量评价未保留原始问答与当前要求")
	}
	var projected []map[string]any
	if err := json.Unmarshal(payload["sources"], &projected); err != nil {
		test.Fatal(err)
	}
	if len(projected) != 2 || projected[0]["turn_id"] != float64(12) || projected[0]["transcript_entry_id"] != float64(91) || projected[0]["message_created_at"] != createdAt.Format(time.RFC3339Nano) || projected[0]["captured_at"] != nil || projected[1]["captured_at"] != createdAt.Format(time.RFC3339Nano) {
		test.Fatal("对话时间和外部事实抓取时间未被分别保留")
	}
}

func TestAgentAnswerQualitySeparatesExecutionFromAnswer(test *testing.T) {
	plan := domain.AgentPlan{Steps: []domain.AgentPlanStep{{ID: 11, Status: domain.AgentPlanStepStatusSkipped, OutputSummary: "未执行"}}}
	metadata := buildAgentExecutionCompletenessMetadata(plan, time.Now().UTC())
	if metadata["evaluates_answer_content"] != false || metadata["score"] != nil || metadata["evidence_refs"] != nil {
		test.Fatalf("执行状态被当作回答质量：%#v", metadata)
	}
	result, err := (&AgentConversationService{}).evaluateAgentAnswerQuality(context.Background(), agentAnswerQualityInput{Plan: plan, Reply: "过长回答", CurrentMessage: "只回复三个大写英文字母", Now: time.Now().UTC()})
	if err == nil || result.Passed || result.Status != "failed" {
		test.Fatalf("步骤终结掩盖了回答约束失败：%#v", result)
	}
}

func TestAgentAnswerEvidenceRequiresCompleteDistinctAuthorizedBody(test *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	plan := domain.AgentPlan{UserID: 7, SessionID: 47, TurnID: 111, Steps: []domain.AgentPlanStep{{ArtifactRefs: []string{"artifact:320", "artifact:321"}}}}
	source := domain.AgentFactSource{CanonicalRef: "artifact:320", FactType: domain.AgentFactTypeArtifact, FactID: 320, UserID: 7, SessionID: 47, TurnID: 111, Content: "2026年11月15日发布的完整证据正文。", CreatedAt: now}
	source.Metadata = domain.AgentJSON{"content_status": "available", "content_hash": agentAnswerContentHash(source.Content), "run_status": "succeeded"}
	duplicate := source
	duplicate.CanonicalRef, duplicate.FactID = "artifact:321", 321
	valid, _, err := agentAnswerEvidenceSources(plan, []domain.AgentFactSource{source, duplicate}, now)
	if err != nil || len(valid) != 1 {
		test.Fatalf("独立证据未按照正文去重：%d，%v", len(valid), err)
	}
	legacy := source
	legacy.Content, legacy.Summary = "", "历史独立摘要"
	legacy.Metadata = domain.AgentJSON{"content_status": "missing"}
	valid, _, err = agentAnswerEvidenceSources(plan, []domain.AgentFactSource{legacy}, now)
	if err != nil || len(valid) != 0 {
		test.Fatal("历史摘要被计为完整正文")
	}
	for _, altered := range []domain.AgentFactSource{
		{CanonicalRef: "plan_step:320", FactType: domain.AgentFactTypePlanStep, FactID: 320, UserID: 7, SessionID: 47, TurnID: 111},
		{CanonicalRef: source.CanonicalRef, FactType: source.FactType, FactID: source.FactID, UserID: 8, SessionID: 47, TurnID: 111, Content: source.Content, Metadata: source.Metadata},
	} {
		valid, _, err = agentAnswerEvidenceSources(plan, []domain.AgentFactSource{altered}, now)
		if err == nil && len(valid) != 0 {
			test.Fatal("结构编号或越权正文被计为事实证据")
		}
	}
}

func TestAgentAnswerSemanticReviewValidatesEachRequirementAndQuote(test *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	sources := []domain.AgentFactSource{{CanonicalRef: "artifact:320", Content: "2026年11月15日，项目完成数据库验证。"}}
	requirements := []agent.PlanEvidenceRequirement{{Required: true, MinimumCount: 1, Freshness: "今天"}}
	review := agentAnswerSemanticReview{
		Status: "passed", ConstraintsStatus: "passed", EvidenceStatus: "passed", FreshnessStatus: "passed", Reason: "正文提供了明确依据。",
		SupportedClaims:  []agentAnswerSupportedClaim{{Claim: "完成数据库验证", SourceRef: "artifact:320", Quote: sources[0].Content}},
		RequiredEvidence: []agentAnswerEvidenceReview{{RequirementIndex: 0, Status: "passed", SourceRefs: []string{"artifact:320"}, FreshnessStatus: "passed", FreshnessQuotes: map[string]string{"artifact:320": sources[0].Content}}},
	}
	encoded, err := json.Marshal(review)
	if err != nil {
		test.Fatal(err)
	}
	for _, content := range []string{string(encoded), "```json\n" + string(encoded) + "\n```", "```\r\n" + string(encoded) + "\r\n```"} {
		if _, err := agentAnswerParseSemanticReview(content, "已经完成数据库验证。", sources, requirements, now); err != nil {
			test.Fatalf("有效的逐项评价未通过：%v", err)
		}
	}
	for _, invalid := range []string{
		"```json\n" + string(encoded),
		"```json\n" + string(encoded) + "\n```\n额外内容",
		"```json\n" + string(encoded) + "\n{}\n```",
		strings.Replace(string(encoded), "完成数据库验证", "发布了未经证实的内容", 1),
		strings.ReplaceAll(string(encoded), "artifact:320", "artifact:999"),
		strings.Replace(string(encoded), `"requirement_index":0`, `"requirement_index":1`, 1),
		strings.Replace(string(encoded), `"source_refs":["artifact:320"]`, `"source_refs":[]`, 1),
	} {
		if _, err := agentAnswerParseSemanticReview(invalid, "已经完成数据库验证。", sources, requirements, now); err == nil {
			test.Fatal("伪造主张、引用或缺失要求未被拒绝")
		}
	}
	if agentAnswerEvidenceDateValid("2026年11月14日发布。", "今天", now) || agentAnswerEvidenceDateValid("今天抓取。", "今天", now) {
		test.Fatal("过期正文或抓取时间被当作有效事件日期")
	}
}

func TestAgentAnswerConversationSourcesCannotReplaceExternalEvidence(test *testing.T) {
	input := agentAnswerQualityInput{SourceMessages: []agent.ContextMessage{{TranscriptEntryID: 91, TurnID: 12, Content: "本轮代号是 ABC"}}}
	sources := agentAnswerConversationSources(input)
	if len(sources) != 1 || sources[0].CanonicalRef != "transcript:91" || sources[0].Content != input.SourceMessages[0].Content {
		test.Fatal("原始对话来源未被完整保留")
	}
	review := agentAnswerSemanticReview{Status: "passed", ConstraintsStatus: "passed", EvidenceStatus: "passed", FreshnessStatus: "not_required", Reason: "原始对话提供会话内代号", SupportedClaims: []agentAnswerSupportedClaim{{Claim: "abc", SourceRef: "transcript:91", Quote: "本轮代号是 ABC"}}}
	encoded, err := json.Marshal(review)
	if err != nil {
		test.Fatal(err)
	}
	if _, err := agentAnswerParseSemanticReview(string(encoded), "abc", sources, nil, time.Now()); err != nil {
		test.Fatalf("真实对话引用未通过验证：%v", err)
	}
	review.RequiredEvidence = []agentAnswerEvidenceReview{{RequirementIndex: 0, Status: "passed", SourceRefs: []string{"transcript:91"}, FreshnessStatus: "not_required"}}
	encoded, err = json.Marshal(review)
	if err != nil {
		test.Fatal(err)
	}
	if _, err := agentAnswerParseSemanticReview(string(encoded), "abc", sources, []agent.PlanEvidenceRequirement{{Required: true, MinimumCount: 1}}, time.Now()); err == nil {
		test.Fatal("对话内容被计为外部事实证据")
	}
}

func TestAgentAnswerEvidenceDateUsesAccountDayAndRejectsUnknownWindow(test *testing.T) {
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		test.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 16, 0, 0, 0, time.UTC).In(location)
	if !agentAnswerEvidenceDateValid("2026年11月16日发布。", "今天", now) || agentAnswerEvidenceDateValid("2026年11月15日发布。", "今天", now) {
		test.Fatal("时效校验未使用账户时区中的日期")
	}
	if agentAnswerEvidenceDateValid("2020年11月15日发布。", "尚未定义的时效范围", now) {
		test.Fatal("无法解析的时效条件被标记为通过")
	}
}

func TestAgentAnswerHistoryRecallUsesConversationFreshness(test *testing.T) {
	sources := []domain.AgentFactSource{{CanonicalRef: "artifact:320", FactType: domain.AgentFactTypeArtifact, Content: "用户：本次约定代号为 ABC。"}}
	requirement := agent.PlanEvidenceRequirement{EvidenceType: "history_recall", Freshness: "recent_context", Required: true, MinimumCount: 1}
	review := agentAnswerSemanticReview{
		Status: "passed", ConstraintsStatus: "passed", EvidenceStatus: "passed", FreshnessStatus: "not_required", Reason: "会话历史工具产物包含用户约定的代号。",
		SupportedClaims:  []agentAnswerSupportedClaim{{Claim: "abc", SourceRef: "artifact:320", Quote: sources[0].Content}},
		RequiredEvidence: []agentAnswerEvidenceReview{{RequirementIndex: 0, Status: "passed", SourceRefs: []string{"artifact:320"}, FreshnessStatus: "not_required"}},
	}
	encoded, err := json.Marshal(review)
	if err != nil {
		test.Fatal(err)
	}
	if _, err := agentAnswerParseSemanticReview(string(encoded), "abc", sources, []agent.PlanEvidenceRequirement{requirement}, time.Now()); err != nil {
		test.Fatalf("近期会话证据被要求提供外部事件日期：%v", err)
	}
	for _, unrestricted := range []agent.PlanEvidenceRequirement{
		{EvidenceType: "conversation_history", Freshness: "any", Required: true, MinimumCount: 1},
		{EvidenceType: "conversation_history", Freshness: "recent_context", Required: true, MinimumCount: 1},
	} {
		if _, err := agentAnswerParseSemanticReview(string(encoded), "abc", sources, []agent.PlanEvidenceRequirement{unrestricted}, time.Now()); err != nil {
			test.Fatalf("无日期限制的会话证据未通过：%v", err)
		}
	}
	for _, changed := range []agent.PlanEvidenceRequirement{
		{EvidenceType: "external_fact", Freshness: "recent_context", Required: true, MinimumCount: 1},
		{EvidenceType: "history_recall", Freshness: "今天", Required: true, MinimumCount: 1},
	} {
		if _, err := agentAnswerParseSemanticReview(string(encoded), "abc", sources, []agent.PlanEvidenceRequirement{changed}, time.Now()); err == nil {
			test.Fatal("外部事实或明确日期要求未执行正文日期校验")
		}
	}
}
