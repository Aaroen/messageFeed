package agent

import "strings"

const (
	TaskTypeQuestionAnswer = "question_answer"
	TaskTypeSearch         = "search"
	TaskTypeNewsAnalysis   = "news_analysis"
	TaskTypeProjectStatus  = "project_status"

	TaskDomainGeneral = "general"
	TaskDomainFinance = "finance"
	TaskDomainTech    = "technology"
	TaskDomainProject = "project"

	TaskFreshnessRealtime   = "realtime"
	TaskFreshnessRecent     = "recent"
	TaskFreshnessHistorical = "historical"
)

type TaskSpec struct {
	RawText              string
	TaskType             string
	Domain               string
	Freshness            string
	RequiresExternal     bool
	NeedsAnalysis        bool
	QueryTerms           []string
	RequiredTerms        []string
	PreferredTerms       []string
	EvidenceTypes        []string
	ExcludedTerms        []string
	LowQualityTerms      []string
	MinimumEvidenceCount int
}

func BuildTaskSpec(message string) TaskSpec {
	raw := strings.TrimSpace(message)
	terms := taskSpecTerms(raw)
	spec := TaskSpec{
		RawText:              raw,
		TaskType:             TaskTypeQuestionAnswer,
		Domain:               classifyTaskDomain(raw),
		Freshness:            classifyTaskFreshness(raw),
		NeedsAnalysis:        taskSpecContainsAny(raw, []string{"分析", "解读", "判断", "影响", "总结", "评估"}),
		QueryTerms:           terms,
		MinimumEvidenceCount: 1,
	}
	if spec.Freshness == TaskFreshnessRealtime || taskSpecContainsAny(raw, []string{"搜索", "查询", "查找", "检索", "新闻", "资讯", "消息"}) {
		spec.RequiresExternal = true
	}
	if taskSpecContainsAny(raw, []string{"进度", "实现", "文件新增", "提交", "推送", "部署", "复盘", "项目"}) && spec.Domain == TaskDomainProject {
		spec.TaskType = TaskTypeProjectStatus
		spec.RequiresExternal = false
	} else if spec.RequiresExternal && (spec.NeedsAnalysis || taskSpecContainsAny(raw, []string{"新闻", "资讯", "消息", "最新"})) {
		spec.TaskType = TaskTypeNewsAnalysis
		spec.MinimumEvidenceCount = 2
	} else if spec.RequiresExternal {
		spec.TaskType = TaskTypeSearch
	}
	spec.applyDomainDefaults()
	return spec
}

func (s TaskSpec) RequestsSearch() bool {
	return s.TaskType == TaskTypeSearch || s.TaskType == TaskTypeNewsAnalysis || s.RequiresExternal
}

func (s TaskSpec) PromptText() string {
	parts := []string{
		"任务类型=" + s.TaskType,
		"领域=" + s.Domain,
		"时效=" + s.Freshness,
		"需要外部检索=" + taskSpecBoolText(s.RequiresExternal),
	}
	if len(s.QueryTerms) > 0 {
		parts = append(parts, "核心词="+strings.Join(s.QueryTerms, "、"))
	}
	if len(s.EvidenceTypes) > 0 {
		parts = append(parts, "证据类型="+strings.Join(s.EvidenceTypes, "、"))
	}
	if len(s.ExcludedTerms) > 0 {
		parts = append(parts, "排除内容="+strings.Join(s.ExcludedTerms, "、"))
	}
	return strings.Join(parts, "；")
}

func (s *TaskSpec) applyDomainDefaults() {
	switch s.Domain {
	case TaskDomainFinance:
		s.EvidenceTypes = appendUniqueStrings(s.EvidenceTypes, []string{"财经新闻", "行情数据", "资金流向", "公司公告", "宏观事件"}...)
		s.PreferredTerms = appendUniqueStrings(s.PreferredTerms, []string{"指数", "板块", "资金流向", "成交额", "公告", "财报"}...)
		s.LowQualityTerms = appendUniqueStrings(s.LowQualityTerms, []string{"开户", "开通", "新手", "教程", "零基础", "课程", "交易流程", "交易规则", "身份规划", "数字游民", "返佣", "开户链接", "券商开户"}...)
		s.ExcludedTerms = appendUniqueStrings(s.ExcludedTerms, s.LowQualityTerms...)
		if taskSpecContainsAny(s.RawText, []string{"港股", "恒指", "恒生", "南向资金", "港交所", "港美股"}) {
			s.RequiredTerms = appendUniqueStrings(s.RequiredTerms, []string{"港股", "恒生", "恒指", "南向资金", "港交所", "香港"}...)
			s.PreferredTerms = appendUniqueStrings(s.PreferredTerms, []string{"恒生指数", "恒生科技", "南向资金", "腾讯", "阿里", "美团", "港股通"}...)
		}
	case TaskDomainTech:
		s.EvidenceTypes = appendUniqueStrings(s.EvidenceTypes, []string{"官方文档", "仓库记录", "版本公告", "技术文章"}...)
		s.LowQualityTerms = appendUniqueStrings(s.LowQualityTerms, []string{"广告", "培训", "课程促销"}...)
		s.ExcludedTerms = appendUniqueStrings(s.ExcludedTerms, s.LowQualityTerms...)
	case TaskDomainProject:
		s.EvidenceTypes = appendUniqueStrings(s.EvidenceTypes, []string{"本地代码", "提交记录", "测试结果", "部署状态"}...)
	default:
		s.EvidenceTypes = appendUniqueStrings(s.EvidenceTypes, []string{"网页正文", "新闻", "内部记录"}...)
	}
	if s.TaskType == TaskTypeNewsAnalysis {
		s.EvidenceTypes = appendUniqueStrings(s.EvidenceTypes, "新闻", "事实摘要")
	}
}

func classifyTaskDomain(message string) string {
	switch {
	case taskSpecContainsAny(message, []string{"港股", "美股", "A股", "a股", "股票", "基金", "债券", "汇率", "黄金", "期货", "恒指", "恒生", "南向资金", "港交所", "券商", "IPO", "上市", "财报", "市场", "行情"}):
		return TaskDomainFinance
	case taskSpecContainsAny(message, []string{"代码", "仓库", "GitHub", "github", "Go", "golang", "数据库", "接口", "架构", "部署", "前端", "后端", "API", "api"}):
		return TaskDomainTech
	case taskSpecContainsAny(message, []string{"项目", "进度", "实现", "提交", "推送", "复盘", "文件新增", "部署上线"}):
		return TaskDomainProject
	default:
		return TaskDomainGeneral
	}
}

func classifyTaskFreshness(message string) string {
	if taskSpecContainsAny(message, []string{"最新", "今日", "今天", "实时", "刚刚", "当前", "现在", "本日"}) {
		return TaskFreshnessRealtime
	}
	if taskSpecContainsAny(message, []string{"近期", "最近", "本周", "这周", "本月", "这个月"}) {
		return TaskFreshnessRecent
	}
	return TaskFreshnessHistorical
}

func taskSpecTerms(message string) []string {
	normalized := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		"，", " ",
		"。", " ",
		"；", " ",
		"、", " ",
		"？", " ",
		"！", " ",
		",", " ",
		";", " ",
		"?", " ",
		"!", " ",
		"：", " ",
		":", " ",
	).Replace(strings.TrimSpace(message))
	for _, phrase := range []string{
		"重新", "请帮我", "帮我", "麻烦", "请",
		"搜索一下", "查询一下", "查找一下", "检索一下",
		"搜索", "查询", "查找", "检索",
		"最新的", "最新", "消息", "新闻", "资讯",
		"并分析一下", "并分析", "分析一下", "分析",
		"一下", "相关", "关于",
	} {
		normalized = strings.ReplaceAll(normalized, phrase, " ")
	}
	fields := strings.Fields(normalized)
	terms := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
	}
	return terms
}

func taskSpecContainsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(values)+len(additions))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

func taskSpecBoolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
