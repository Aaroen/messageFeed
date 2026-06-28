# messageFeed 实施进度台账

**最后更新**：2026-06-28

本文件是当前实现进度的主台账。每轮迭代除更新 `docs/nowdoit` 活动文档外，必须同步更新本文档以及相关设计文档，例如 `docs/agent-plan.md`。历史已完成活动文档保留在 `docs/nowdoit/archive/`。

## 1. 当前目标

最终目标保持为统一的 `messageFeed AI Agent`：

1. 用户可以从企业微信或 Web 发起 Agent 任务。
2. Web 端可以查看 Agent 实时进度、计划步骤、执行细节、证据、审批、回放和恢复状态。
3. 企业微信侧应向用户投递可在 Web 浏览器打开的进度地址。
4. 任务完成后，系统通过企业微信向用户汇报结果。
5. Agent 运行时、能力注册、策略审批、上下文记忆、调度、评测、审计和通知必须保持可追溯。
6. 代码结构必须维持清晰职责边界；大文件继续拆分治理，避免把新能力堆叠到既有长文件中。

## 2. 当前仓库状态

| 项目 | 当前状态 |
| --- | --- |
| 分支 | `master` |
| 工作区 | Agent 主闭环服务已完成结构拆分，主 Agent 模型 PlanSpec 已接入 service 主路径；提交推送后以 `git status -sb` 为准 |
| 当前活动文档 | `docs/nowdoit/agent-web-assistant-entry-plan.md` |
| 最近本轮验证 | 已通过 `go test ./... -count=1`、`npm run build`；使用 `.env` 真实 LLM 配置通过 `RUN_REAL_LLM_TESTS=1 go test ./internal/service -run TestAgentConversationServiceRealLLMFullFlowContracts -count=1 -timeout 10m -v` |
| 最近核对提交 | 以 `git log -1 --oneline` 为准；本文档作为实现进度台账，不替代 Git 提交记录 |

## 3. 已完成能力

### 3.1 Agent 基础运行时

- 已具备 Agent session、turn、run、plan、approval、scheduled task、eval、recovery、audit 等基础对象和服务面。
- Web 侧已有 Agent 任务入口、任务工作台、计划进度页、审批页、证据与回放相关视图。
- 后端已形成任务聚合接口，用于向 Web 工作台提供任务、SLA、成本、告警、部署验证、企业微信联动、进度证据、恢复策略和评测摘要。
- 已具备任务进度 URL 字段，并在 Web 发起任务后可跳转进度页。
- `AgentConversationService` 主闭环已完成结构拆分：`agent_main.go` 仅保留服务装配、构造、runner 初始化和 capability executor 组装；入口接收、会话解析、企微按钮控制、多轮续接、turn 执行管线、计划反馈、turn 结果、controller run 记录、通用工具和契约类型已迁入独立文件。

### 3.2 企业微信接入与交互

- 已具备企业微信自建应用 callback、OAuth、文本消息发送和模板卡片发送基础能力。
- 已具备企业微信审批按钮、回放、恢复、模板卡片、灰度、签收、最终报告、反馈闭环等多项治理摘要。
- 已完成真实交互自动化摘要 `real_interaction_automation` 的后端和前端展示。
- 已完成 `wechat_web_progress_link` 后端聚合摘要和审计快照，字段覆盖进度地址、地址来源、投递通道、模板状态、fallback 状态、浏览器目标和审计引用。
- 已接入企业微信进度通知真实投递：模板卡片优先，文本 fallback 保底，卡片和 fallback 均包含 Web 浏览器进度地址。
- `wechat_web_progress_link` 聚合摘要已读取真实 `agent.plan_progress_notification` 或 `agent.plan_started_feedback` 审计事件中的进度地址、模板状态和 fallback 状态。
- 已补齐 Web 发起任务完成后的企业微信最终报告投递：用户存在启用企业微信绑定时，最终报告通过模板卡片和文本结果组合投递，并记录真实投递审计。
- 企业微信 OAuth 绑定链路已核对：OAuth URL 必须由已登录 Web 用户生成；callback 按 OAuth state 中的 `user_id` 绑定 external account 并创建同一用户的 Web session；`/api/v1/auth/me` 返回当前用户 bindings；disabled binding 在企业微信 external account 解析时被拒绝。
- 已修复企业微信任务静默失败问题：入站消息能够落库并创建 turn 时，如计划创建或 controller 早期阶段失败，会将 turn 和 inbound 标记为 failed，记录 `agent.turn_failed` 与 `agent.turn_failure_feedback`，并向企业微信发送失败反馈，避免用户侧长时间无响应。
- 已修复企微任务 `搜索最新港股消息并分析` 的空响应根因：主 Agent 先生成结构化 `PlanSpec`，后端完成权限、预算和 capability scope 校验后，由子 Agent 显式调用工具并回传 observation；模型空回复不再由后端拼接本地分析，转为失败状态和统一反馈链路处理。
- 已强化搜索浏览闭环：`web.search` 保留用户或模型传入的查询语义，优先尝试 DuckDuckGo HTML，遇到 202 challenge、空解析或拦截页时自动降级到 Bing 普通网页搜索、Google News RSS 与 Bing News RSS；RSS 解析使用 XML 结构化解析和 HTML 文本清洗；搜索结果不再按后端财经词表过滤，最终结论由模型基于工具证据生成，执行治理内容保留在 Web 详情和审计中。
- 已修正企业微信最终回复形态：最终文本以用户问题的结论、事实依据和分析过程为主体，不再拼接状态锚点、预算、质量、成本、运行观测、证据引用和企微动作组件；这些执行层面数据保留在 Web 进度页、审计和任务详情中。Web 发起任务投递到企业微信时仍保留简短详情链接，便于跳转查看完整执行记录。
- 主 Agent 模型规划已接入 `AgentConversationService` 主路径：接收用户消息后先由主 Agent 生成结构化 `PlanSpec`，后端只做 JSON 结构、capability 注册、权限、预算和风险确认校验，再将授权范围下发给子 Agent 执行。规划阶段本身不占用工具 capability，提示词、schema 和回复约束已集中到 `internal/service/agent_main_prompts.go`，便于后续统一调整。
- LLM 客户端已支持 OpenAI-compatible 双协议智能路由：默认优先 `/chat/completions`，遇到协议不兼容时自动尝试 `/responses`，成功后在客户端实例内记忆可用协议；HTTP 429/5xx 和瞬时网络错误具备指数退避重试，非协议类 5xx 不再误触发协议切换。当前 `.env` 真实模型验证使用 `/v1/chat/completions` 完成。
- 已参考 `../references/openai_go` 与 `../references/opencode` 的长请求、SSE 超时、用户中断和 thinking 展示处理方式，将本项目非流式模型单次思考等待窗口调整为 180 秒；模型思考超时归类为 `llm_thinking_timeout`，不会在 HTTP 层重复提交同一长请求，并通过企业微信反馈 payload/template 显式暴露为“模型思考阶段超时”。
- 子 Agent 工具循环上限已提高到 50 次；达到上限时不再无限调用工具，而是进入强制收敛阶段，要求模型基于已有 observation 输出最终回答或明确说明证据不足。
- 已清理旧的任务规格、证据评分和质量门禁主路径规则：`TaskSpec` 保留兼容结构，不再从用户文本推断领域、搜索方向或结论；证据评分只检查标题、URL、来源、摘要和发布时间等结构完整性；最终回答质量门禁不再按涨跌、市场方向或低质量词表替模型判断。
- 已修复企微重复发送失败任务无响应问题：自然语言 `retry` 命中旧失败计划时，不再只记录重试请求并结束 turn，而是重新进入主 Agent 规划与执行闭环；直接回复为空时转入失败反馈链路；生产镜像已打包 `configs/agent_wechat_feedback.zh-CN.json`，保障模型短反馈为空时仍有配置化模板兜底。
- 已强化企微阶段失败反馈：企业微信短反馈集中提示词要求失败消息必须说明失败阶段和用户可理解原因；配置化兜底模板同步展示阶段、原因和详情地址；主 Agent 规划阶段在 PlanSpec 生成前失败时会补建 failed plan 与 failed step，Web 详情页可直接查看规划阶段错误、审计和进度地址。
- 已修正企业微信进度与最终报告文本 fallback 的通知上下文，避免后台执行上下文取消后影响失败原因和详情地址投递。
- 已增强子 Agent 模型空响应处理：runner 遇到上游模型空内容时不再立即失败，而是追加收敛提示并有限重试；重试事件记录为 `agent.llm_empty_response_retry`，Web 详情可看到空响应后的重试过程。后端仍不拼接业务结论，最终回答继续由模型基于工具观察生成。

### 3.3 Web 进度与治理展示

- Web 任务工作台已展示多数 Agent 运行、企业微信、审批、回放、恢复和真实交互自动化摘要。
- Web 任务工作台已展示 `wechat_web_progress_link` 摘要，包括进度地址、投递通道、模板状态、fallback 状态、浏览器目标和检查项。
- Web 进度页已支持计划 ID 或调度任务 ID 维度的进度查询、轮询、步骤、证据和审批状态展示。
- 前端已有任务创建表单，并通过 `createAgentTask` 以 `channel=web` 发起任务。
- 顶部主入口已扩展为“订阅 / 推荐 / 助理”，助理与订阅、推荐共用三页横向滑动模型，并同时挂载三个 pane。
- 助理页已复用现有 Agent 后端工作台；执行进度位于发起任务下方，评测基线不再展示给普通用户，最近任务不再展示开发治理摘要。
- 已修正 Agent 详情查看交互：助理首页保留发起任务和最近任务列表；从最近任务点击“查看”后进入 `/agent/plans/:id` 独立详情页。详情页已重构为单一流水线：主 Agent 接收任务、理解任务、判断复杂度、生成提示词和子 Agent 上下文、启动子 Agent、汇总结果、质量门禁、最终交付。主 Agent 和子 Agent 节点均默认收起明细，展开后可查看任务包、上下文预算、上下文快照、工具调用、观察、产物、错误和重试记录。

### 3.4 质量与验证

- 后端已有 `agent_progress_service_test.go` 对 `WeChatWebProgressLink` 的返回字段进行断言。
- 最近一轮全量验证已覆盖 Go 测试、Go vet、前端测试、类型检查和前端构建。
- 已补充企微任务失败闭环回归测试，覆盖 plan 创建失败时的 turn 状态、inbound 状态、失败审计和企微 fallback；已补充 Agent JSONB 空数组回归测试，防止空引用列表写成无效 JSON。
- 已补充本轮闭环回归测试：覆盖计划 scope 下发到上下文构建、`web.search` 计划预取、模型空响应降级、来源名称计划识别、controller scope 对齐、企微入站到计划完成的完整路径。
- 已补充搜索浏览强化回归测试：覆盖查询归一化、Bing 普通网页解析、RSS 新闻解析、DuckDuckGo challenge 后外部搜索 fallback、搜索任务优先使用相关 web 证据、用户可见降级回复不泄露内部字段，以及真实 repository 运行记录更新字段。
- 已补充任务规格、证据评分和质量门禁回归测试：覆盖兼容结构不再做领域推断、证据结构完整性评分、搜索结果不过滤模型查询语义、模型空回复进入失败链路，以及权限拒绝类回答不被质量门禁覆盖。
- 已补充真实 LLM 主闭环契约测试：`TestAgentConversationServiceRealLLMFullFlowContracts` 使用 `.env` 的 `LLM_*` 配置，覆盖历史聊天检索、真实外部 `web.search`、定时任务确认三个闭环；默认跳过，仅在 `RUN_REAL_LLM_TESTS=1` 时访问外部模型。本轮已用真实模型真实流程通过该测试。
- 当前新增治理文件将部分逻辑从大文件中抽离，包括：
  - `internal/service/agent_conversation_entry.go`
  - `internal/service/agent_conversation_session.go`
  - `internal/service/agent_button_control.go`
  - `internal/service/agent_multiturn_flow.go`
  - `internal/service/agent_turn_pipeline.go`
  - `internal/service/agent_plan_feedback.go`
  - `internal/service/agent_turn_result.go`
  - `internal/service/agent_controller_run.go`
  - `internal/service/agent_conversation_utils.go`
  - `internal/service/agent_conversation_contracts.go`
  - `internal/service/agent_real_interaction_automation_governance.go`
  - `internal/service/agent_wechat_web_progress_link_governance.go`
  - `internal/service/agent_dual_end_run_loop_governance.go`
  - `internal/service/agent_dual_end_task_closure_governance.go`
  - `internal/service/agent_governance_checks.go`
  - `internal/service/agent_workflow_metadata_builders.go`
  - `internal/service/agent_workflow_foundation_builders.go`
  - `internal/service/agent_workflow_wechat_builders.go`
  - `internal/service/agent_workflow_release_ops_builders.go`
  - `internal/service/agent_workflow_ops_handling_builders.go`
  - `internal/service/agent_session_snapshot_recorders.go`

## 4. 当前未完成缺口

| 优先级 | 缺口 | 当前判断 |
| --- | --- | --- |
| P1 | Web 浏览器进度地址权限校验与企业微信身份绑定仍需继续强化 | 进度与计划接口用户归属校验已有测试；OAuth / external account 绑定链路已有服务测试；当前轮处理助理顶部入口、三页滑动和 Agent 工作台用户化整理 |
| P1 | Agent 能力注册、上下文记忆、计划执行和评测体系仍需按设计持续补齐 | 主 Agent 模型 PlanSpec、子 Agent 工具执行和真实 LLM 三类闭环已跑通；更完整的长期记忆、评测基线和恢复策略仍需继续补齐 |
| P1 | 模型驱动执行仍需继续增强线上稳定性 | 旧硬编码意图、领域、搜索过滤和本地结论 fallback 已从主路径清理；后续重点是补充更多真实任务评测、模型异常解释、外部搜索质量和多轮迭代充分性评估 |
| P1 | 大文件职责边界仍不理想 | `agent_main.go` 已由约 3035 行降至 205 行；仍需要继续拆分 `agent_session_service.go`、`agent_workflow_governance.go`、`AgentPlanView.vue` |

## 5. 架构质量核对

当前关键大文件规模：

| 文件 | 行数 | 判断 |
| --- | ---: | --- |
| `internal/service/agent_main.go` | 205 | 已完成主闭环结构拆分；当前仅保留服务装配、构造、runner 初始化和 capability executor 组装 |
| `internal/service/agent_session_service.go` | 4446 | 仍明显过大；已迁出任务列表聚合响应 DTO、任务摘要 DTO、转换函数、任务摘要状态 helper、基础治理审计快照 recorder、发布执行/日报闭环 recorder、发布窗口/外部监控 recorder、生产发布/上线交接 recorder、运行态参数/反馈闭环 recorder、放量阶段/运维处置 recorder 和审批执行/工单 SLA recorder，后续应继续拆分剩余审计 recorder、进度构造和服务编排 |
| `internal/service/agent_workflow_governance.go` | 739 | 已明显低于此前 5000 行级别；本轮已迁出所有 `buildAgent*` 纯 builder，剩余内容主要为 admission、质量摘要、通用 helper 和 plan/domain 转换辅助 |
| `web/src/views/AgentPlanView.vue` | 3680 | 仍明显过大；本轮已迁出两个企业微信摘要组件，后续应继续拆分 composable、摘要面板组件和任务卡片组件 |

结论：这些文件达到数千行不能简单视为正常的企业级设计结果。当前实现虽然有业务闭环推进价值，但从企业级代码质量角度看，必须持续进行模块化拆分、职责收敛和测试保护。后续新增能力不得继续扩大上述文件，除非是短期兼容性必要改动；优先使用独立 service 文件、独立前端组件或 composable。

## 6. 当前活动文档执行状态

上一活动文档 `docs/nowdoit/agent-wechat-web-progress-link-delivery-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-wechat-web-progress-link-delivery-plan-implemented-2026-06-25.md`。

上一活动文档 `docs/nowdoit/agent-wechat-final-result-report-delivery-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-wechat-final-result-report-delivery-plan-implemented-2026-06-25.md`。

已归档活动文档：`docs/nowdoit/archive/agent-web-progress-permission-binding-governance-plan-implemented-2026-06-26.md`

上一轮完成项：

1. 已完成：梳理任务进度 URL、企业微信模板发送摘要和任务聚合结果。
2. 已完成：新增 Web 进度地址投递摘要 builder。
3. 已完成：`ListTasks` 接入地址投递摘要并写入审计快照。
4. 已完成：服务层测试补充地址投递字段断言。
5. 已完成：前端 API 类型和 Agent 任务工作台展示地址投递摘要。
6. 已验证：`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。
7. 已完成：企业微信真实模板消息或文本 fallback 中实际投递进度地址。
8. 已验证：`go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。
9. 已完成：本轮完成后归档活动文档并创建下一轮活动文档。

当前轮完成项：

1. 已完成：梳理最终结果汇报路径，包括主完成回复、失败反馈和最终报告摘要 builder。
2. 已完成：新增企业微信最终结果汇报 helper。
3. 已完成：最终结果采用模板卡片入口加完整文本结果的组合投递，模板失败时文本仍可发送。
4. 已完成：最终结果真实发送结果写入 `wechat_work.reply_sent` 和 `agent.turn_failure_feedback` 审计 metadata。
5. 已完成：`wechat_final_report` 聚合摘要暴露 `delivery_status`、`template_status`、`text_status` 和 `progress_url`。
6. 已完成：Web 任务工作台展示企业微信最终汇报真实投递状态和进度地址链接。
7. 已验证：`go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。

当前权限治理阶段性完成项：

1. 已完成：确认 `GetProgress` 和 `GetPlanDetail` 使用当前认证用户 ID 访问数据层。
2. 已完成：补充未登录访问进度拒绝测试。
3. 已完成：补充跨用户访问计划进度拒绝测试。
4. 已完成：补充跨用户访问计划详情拒绝测试。
5. 已完成：补充跨用户访问调度任务进度拒绝测试。
6. 已完成：梳理企业微信 OAuth / external account / Web session 与 Web 进度页访问关系。
7. 已完成：补充 OAuth 与绑定一致性测试，覆盖未登录生成 OAuth URL 拒绝、callback 按 state 用户绑定、当前用户 bindings 返回、disabled binding 解析拒绝。
8. 已验证：`go test ./internal/service -run 'TestAuthService(WeChatWorkOAuth|ResolveExternalAccount|Me)'`。
9. 本轮完整验证已通过：`go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。

## 7. 下一轮实施顺序

1. 提交并推送 OAuth / external account 绑定测试和文档同步结果。
2. 拆分前端 Agent 工作台中进度地址和最终汇报摘要展示逻辑，避免继续扩大 `AgentPlanView.vue`。
3. 归档当前活动文档，并按主设计新建下一轮活动文档。
4. 继续治理后端大文件职责边界，优先迁出 Agent session 聚合 DTO、审计快照 builder 和 workflow governance 子模块。

## 7.1 本轮文件变更核对

OAuth 绑定治理阶段未新增生产文件，文件数量未扩张。变更集中在：

1. `internal/service/auth_service_test.go`：新增 OAuth / external account 绑定语义测试，并扩展测试 fake repository 支持多用户断言。
2. `docs/nowdoit/agent-web-progress-permission-binding-governance-plan.md`：同步本轮活动文档状态。
3. `docs/implementation.md`：同步主进度台账。
4. `docs/agent-plan.md`：同步 Agent 设计对照状态。

前端摘要拆分小轮已完成，新增两个前端小组件：

1. `web/src/components/agent/AgentWeChatFinalReportSummary.vue`
2. `web/src/components/agent/AgentWeChatWebProgressLinkSummary.vue`

已迁出 `AgentPlanView.vue` 中企业微信最终汇报和 Web 进度地址摘要展示逻辑，未改变后端接口、字段语义和现有页面行为。`AgentPlanView.vue` 从 3707 行降至 3680 行，两个新增组件各 33 行，文件数量增加与职责拆分相匹配，不属于冗余扩张。

本小轮验证已通过：

1. `npm --prefix web run type-check`
2. `npm --prefix web run build`
3. `npm --prefix web run test`

当前活动文档已归档到 `docs/nowdoit/archive/agent-web-progress-permission-binding-governance-plan-implemented-2026-06-26.md`。

已归档活动文档：`docs/nowdoit/archive/agent-session-service-aggregation-modularization-plan-implemented-2026-06-26.md`

新一轮目标：

1. 梳理 `agent_session_service.go` 中任务聚合响应类型和 builder 可迁移边界。
2. 优先迁出低风险、无副作用的响应 DTO 或纯函数 builder。
3. 保持 `ListTasks` 的 JSON 字段、审计事件和前端 API 语义不变。
4. 验证通过后同步主文档和设计文档并提交推送。

后端任务聚合 DTO 拆分阶段性结果：

1. 已新增 `internal/service/agent_task_list_responses.go`，承接 `AgentTaskListResult` 以及任务列表聚合所需的 SLA、成本、告警和趋势响应 DTO。
2. 已从 `internal/service/agent_session_service.go` 迁出 201 行纯类型定义；该文件从 6255 行降至 6054 行。
3. 未改变 `ListTasks` 查询流程、审计事件、JSON 字段或前端 API 类型。
4. 已验证：`go test ./...`、`go vet ./...`。

任务摘要 DTO 与转换函数拆分阶段性结果：

1. 已将 `AgentTaskSummaryResponse` 和 `AgentTaskReportResponse` 迁入 `internal/service/agent_task_list_responses.go`。
2. 已将 `agentTaskSummaryFromPlan` 和 `agentTaskSummaryFromScheduledTask` 迁入同一文件，保留原有转换语义。
3. `agent_session_service.go` 从 6054 行继续降至 5975 行；`agent_task_list_responses.go` 从 202 行增至 287 行。
4. 已验证：`go test ./...`、`go vet ./...`。

任务摘要状态 helper 拆分阶段性结果：

1. 已将 `scheduledTaskBudgetStatus`、`scheduledTaskHandoffStatus`、`scheduledTaskObservabilitySummary` 和 `planLatestProgress` 迁入 `internal/service/agent_task_list_responses.go`。
2. helper 仍保持 package 内部可见，`agent_workflow_governance.go` 对 `scheduledTaskHandoffStatus` 的调用不变。
3. `agent_session_service.go` 从 5975 行继续降至 5936 行；`agent_task_list_responses.go` 从 287 行增至 326 行。
4. 已验证：`go test ./...`、`go vet ./...`。

已归档活动文档：`docs/nowdoit/archive/agent-workflow-governance-builder-modularization-plan-implemented-2026-06-26.md`

新一轮目标：

1. 梳理 `agent_workflow_governance.go` 中低耦合 builder 群组。
2. 优先迁出输入输出均为 domain 或 response DTO、无 repository 访问、无审计写入副作用的纯函数。
3. 保持 `ListTasks` 聚合结果、JSON 字段和审计语义不变。
4. 验证通过后同步主文档和设计文档并提交推送。

Workflow governance metadata builder 拆分阶段性结果：

1. 已新增 `internal/service/agent_workflow_metadata_builders.go`，承接 capability policy、handoff、runtime observability、recovery、quality、deployment acceptance 和 cost summary metadata builder。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 13 个纯函数，不改变 metadata 字段名、调用方或任务聚合顺序。
3. `agent_workflow_governance.go` 从 4626 行降至 4296 行；新增文件为 338 行。
4. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 基础聚合 builder 拆分阶段性结果：

1. 已新增 `internal/service/agent_workflow_foundation_builders.go`，承接成本、告警、趋势、部署验证和生产演练相关基础聚合 builder。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 9 个纯函数，不改变聚合摘要 JSON 字段、状态取值或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 4296 行降至 3893 行；新增文件为 412 行。
4. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 企业微信组件 builder 拆分阶段性结果：

1. 已新增 `internal/service/agent_workflow_wechat_builders.go`，承接企业微信组件、callback readiness、原生动作定义、payload 构造和按钮 helper。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 7 个纯函数，不改变企业微信按钮 key、fallback 文案、payload 字段、状态取值或 `ListTasks` 企业微信相关调用顺序。
3. `agent_workflow_governance.go` 从 3893 行降至 3717 行；新增文件为 183 行。
4. 本轮 workflow governance builder 拆分累计新增 3 个小文件，合计承接 29 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

已归档活动文档：`docs/nowdoit/archive/agent-workflow-governance-builder-modularization-plan-implemented-2026-06-26.md`

新活动文档：`docs/nowdoit/agent-workflow-governance-release-ops-builder-modularization-plan.md`

新一轮目标：

1. 梳理 `agent_workflow_governance.go` 中发布、运维、灰度、监控、日报和按钮闭环相关 builder 群组。
2. 优先迁出输入输出为响应 DTO、无 repository 访问、无审计写入副作用的纯聚合函数。
3. 保持 `ListTasks` 聚合结果、JSON 字段、状态取值和审计语义不变。
4. 验证通过后同步主文档和设计文档并提交推送。

Workflow governance release/ops 基础 builder 拆分阶段性结果：

1. 已新增 `internal/service/agent_workflow_release_ops_builders.go`，承接发布、运维、灰度、告警通道、上线演练和企业微信原生按钮联调相关基础 builder。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 10 个纯函数，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 3717 行降至 3381 行；新增文件为 345 行。
4. 本轮 workflow governance builder 拆分累计新增 4 个小文件，合计承接 39 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 发布执行与日报闭环 builder 拆分阶段性结果：

1. 已将 10 个发布执行、审批、日报、监控和按钮回调闭环 builder 追加迁入 `internal/service/agent_workflow_release_ops_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 3381 行降至 3057 行；`agent_workflow_release_ops_builders.go` 从 345 行增至 669 行。
4. 当前 workflow governance builder 拆分累计新增 4 个小文件，合计承接 49 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 发布窗口与外部监控 builder 拆分阶段性结果：

1. 已将 10 个发布窗口、外部监控、按钮直控、灰度扩展和企业微信验收相关 builder 追加迁入 `internal/service/agent_workflow_release_ops_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 3057 行降至 2750 行；`agent_workflow_release_ops_builders.go` 从 669 行增至 976 行。
4. 当前 workflow governance builder 拆分累计新增 4 个小文件，合计承接 59 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 生产发布与运行闭环 builder 拆分阶段性结果：

1. 已将 10 个灰度评审、企业微信验收复核、生产发布、外部监控配置、写放量策略和上线交接相关 builder 追加迁入 `internal/service/agent_workflow_release_ops_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 2750 行降至 2442 行；`agent_workflow_release_ops_builders.go` 从 976 行增至 1284 行。
4. 当前 workflow governance builder 拆分累计新增 4 个小文件，合计承接 69 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 运行态与反馈闭环 builder 拆分阶段性结果：

1. 已将 10 个写放量策略、最终报告、上线运行概览、运行参数、监控回读、放量推荐、企业微信用户反馈、运行闭环、运营面板和异常自动汇报相关 builder 追加迁入 `internal/service/agent_workflow_release_ops_builders.go`。
2. 已同步迁入 `latestAgentWeChatFinalReportAudit` 辅助函数，保持企业微信最终汇报真实审计回读语义不变。
3. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
4. `agent_workflow_governance.go` 从 2442 行降至 2127 行；`agent_workflow_release_ops_builders.go` 从 1284 行增至 1599 行。
5. 当前 workflow governance builder 拆分累计新增 4 个小文件，合计承接 79 个低风险纯函数；release/ops 承接文件累计承接 50 个纯函数和 1 个审计读取 helper，文件数量增加与职责拆分相匹配，不属于冗余扩张。
6. 已验证：`go test ./...`、`go vet ./...`。

已归档活动文档：`docs/nowdoit/archive/agent-workflow-governance-release-ops-builder-modularization-plan-implemented-2026-06-26.md`

新活动文档：`docs/nowdoit/agent-workflow-governance-ops-handling-builder-modularization-plan.md`

新一轮目标：

1. 梳理 `agent_workflow_governance.go` 中剩余运维处置、灰度阶段、反馈工单、证据交互和双端进度相关 builder 群组。
2. 优先迁出输入输出为响应 DTO、无 repository 访问、无审计写入副作用的纯聚合函数。
3. 新增独立 `internal/service/agent_workflow_ops_handling_builders.go`，避免继续扩大 release/ops 承接文件。
4. 验证通过后同步主文档和设计文档并提交推送。

Workflow governance 运维处置基础 builder 拆分阶段性结果：

1. 已新增 `internal/service/agent_workflow_ops_handling_builders.go`，承接写放量阶段、企业微信反馈循环、运营闭环、运营面板交互、告警去重升级、写阶段记录、反馈工单、运营处理、运营动作定义和告警升级策略相关 10 个纯 builder。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 2127 行降至 1832 行；`agent_workflow_ops_handling_builders.go` 新增为 302 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 89 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 审批执行与 SLA builder 拆分阶段性结果：

1. 已将写阶段审批、反馈工单生命周期、运营动作闭环、API 执行、告警回执、审批按钮、反馈 SLA、运营执行记录和企业微信审批回调相关 10 个纯 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 1832 行降至 1520 行；`agent_workflow_ops_handling_builders.go` 从 302 行增至 614 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 99 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go vet ./...`、`go test ./...`。其中 `go test ./...` 在当前沙箱内因 `httptest` 本地监听端口权限失败，提升权限后通过。

Workflow governance 证据闭环与双端进度基础 builder 拆分阶段性结果：

1. 已将反馈 SLA 报表、告警自动恢复、运营证据、统一进度组件、证据详情页、回调重放工具、恢复策略配置、双端进度证据、企业微信进度卡片和 Web 证据交互相关 10 个纯 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 1520 行降至 1224 行；`agent_workflow_ops_handling_builders.go` 从 614 行增至 911 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 109 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 权限审批、模板发送与真实交互 builder 拆分阶段性结果：

1. 已将回调重放权限、恢复策略审计、双端交互、企业微信模板渲染、Web 证据路由、回调重放审批、恢复策略持久化、双端交互发布、企业微信模板发送、Web 证据详情、回调重放执行、恢复策略版本和双端真实交互相关 13 个纯 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 1224 行降至 827 行；`agent_workflow_ops_handling_builders.go` 从 911 行增至 1308 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 122 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Workflow governance 剩余 SLA 与任务报表 builder 收尾拆分结果：

1. 已将 `buildAgentSLASummary` 和 `buildAgentTaskReport` 迁入既有 `internal/service/agent_workflow_foundation_builders.go`，承接职责与基础聚合统计一致。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、统计口径或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 827 行降至 739 行；`agent_workflow_foundation_builders.go` 从 412 行增至 500 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 124 个低风险纯函数；本收尾小轮未新增文件，文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

已归档活动文档：`docs/nowdoit/archive/agent-workflow-governance-ops-handling-builder-modularization-plan-implemented-2026-06-26.md`

新活动文档：`docs/nowdoit/agent-session-service-snapshot-recorder-modularization-plan.md`

新一轮目标：

1. 梳理 `agent_session_service.go` 中连续的 `recordAgent*Snapshot` 方法群组。
2. 优先迁出只写审计快照、不改变任务聚合业务语义的 recorder。
3. 新增独立 `internal/service/agent_session_snapshot_recorders.go`，降低 `agent_session_service.go` 的职责密度。
4. 验证通过后同步主文档和设计文档并提交推送。

Agent session 基础治理快照 recorder 拆分阶段性结果：

1. 已新增 `internal/service/agent_session_snapshot_recorders.go`，承接 `recordAgentAlertPolicyDecision`、`recordAgentProductionDrillSnapshot`、`recordAgentWriteSandboxSnapshot`、`recordAgentE2EAcceptanceSnapshot`、`recordAgentRealIntegrationSnapshot`、`recordAgentOpsAcceptanceSnapshot`、`recordAgentWriteGraySnapshot`、`recordAgentAlertChannelSnapshot`、`recordAgentLaunchDrillRecord` 和 `recordAgentWeChatNativeIntegrationSnapshot`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 5936 行降至 5731 行；新增 recorder 文件为 211 行。
4. 本小轮新增文件数量与审计快照职责拆分相匹配，不属于冗余扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 发布执行与日报闭环 recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteReplaySnapshot`、`recordAgentLaunchApprovalSnapshot`、`recordAgentDailyReportSnapshot`、`recordAgentPreprodAcceptanceSnapshot`、`recordAgentButtonLoopSnapshot`、`recordAgentWriteExecuteSnapshot`、`recordAgentDailyPersistSnapshot`、`recordAgentPostLaunchMonitorSnapshot`、`recordAgentReleaseApprovalSnapshot` 和 `recordAgentButtonCallbackSnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 5731 行降至 5518 行；`agent_session_snapshot_recorders.go` 从 211 行增至 424 行。
4. 当前 snapshot recorder 拆分累计承接 20 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 发布窗口与外部监控 recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteAuditReviewSnapshot`、`recordAgentDailySendSnapshot`、`recordAgentMonitorAlertDrillSnapshot`、`recordAgentButtonDirectControlSnapshot`、`recordAgentWeChatE2EAcceptanceSnapshot`、`recordAgentReleaseWindowReadinessSnapshot`、`recordAgentWriteGrayExpansionSnapshot`、`recordAgentExternalMonitorIntegrationSnapshot`、`recordAgentReleaseWindowExecutionSnapshot` 和 `recordAgentExternalMonitorRuntimeSnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 5518 行降至 5314 行；`agent_session_snapshot_recorders.go` 从 424 行增至 628 行。
4. 当前 snapshot recorder 拆分累计承接 30 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 生产发布与上线交接 recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteGrayReviewSnapshot`、`recordAgentWeChatAcceptanceReviewSnapshot`、`recordAgentOperationsDailyClosureSnapshot`、`recordAgentProductionReleaseSnapshot`、`recordAgentExternalMonitorConfigSnapshot`、`recordAgentWriteRampSnapshot`、`recordAgentWeChatSignoffSnapshot`、`recordAgentOperationsHandoffSnapshot`、`recordAgentProductionExecutionSnapshot` 和 `recordAgentMonitorIntegrationSnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 5314 行降至 5088 行；`agent_session_snapshot_recorders.go` 从 628 行增至 854 行。
4. 当前 snapshot recorder 拆分累计承接 40 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 运行态参数与反馈闭环 recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteRampPolicySnapshot`、`recordAgentWeChatFinalReportSnapshot`、`recordAgentLaunchRuntimeOverviewSnapshot`、`recordAgentRuntimeParametersSnapshot`、`recordAgentMonitorReadbackSnapshot`、`recordAgentWriteRampRecommendationSnapshot`、`recordAgentWeChatUserFeedbackSnapshot`、`recordAgentOperationsRuntimeClosureSnapshot`、`recordAgentOpsPanelConfigSnapshot` 和 `recordAgentMonitorAutoReportSnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 5088 行降至 4863 行；`agent_session_snapshot_recorders.go` 从 854 行增至 1079 行。
4. 当前 snapshot recorder 拆分累计承接 50 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 放量阶段与运维处置 recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteRampStageSnapshot`、`recordAgentWeChatFeedbackLoopSnapshot`、`recordAgentOperationsClosedLoopSnapshot`、`recordAgentOpsDashboardInteractionSnapshot`、`recordAgentAlertDedupeEscalationSnapshot`、`recordAgentWriteStageRecordSnapshot`、`recordAgentWeChatFeedbackTicketSnapshot`、`recordAgentOperationsHandlingSnapshot`、`recordAgentOpsActionDefinitionSnapshot` 和 `recordAgentAlertEscalationPolicySnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 4863 行降至 4651 行；`agent_session_snapshot_recorders.go` 从 1079 行增至 1291 行。
4. 当前 snapshot recorder 拆分累计承接 60 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

Agent session 审批执行与工单 SLA recorder 拆分阶段性结果：

1. 已将 `recordAgentWriteStageApprovalSnapshot`、`recordAgentFeedbackTicketLifecycleSnapshot`、`recordAgentOperationsActionClosureSnapshot`、`recordAgentOpsAPIExecutionSnapshot`、`recordAgentAlertEscalationReceiptSnapshot`、`recordAgentWriteApprovalButtonSnapshot`、`recordAgentFeedbackTicketSLASnapshot`、`recordAgentOperationsExecutionSnapshot`、`recordAgentOpsExecutionRecordSnapshot` 和 `recordAgentWeChatApprovalCallbackSnapshot` 迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 调用顺序。
3. `agent_session_service.go` 从 4651 行降至 4446 行；`agent_session_snapshot_recorders.go` 从 1291 行增至 1496 行。
4. 当前 snapshot recorder 拆分累计承接 70 个审计快照 recorder；文件数量未继续扩张。
5. 已验证：`go test ./...`、`go vet ./...`。

当前优先级切换为最小完整闭环交付。`docs/nowdoit/agent-session-service-snapshot-recorder-modularization-plan.md` 已写入的 4.8 recorder 拆分计划暂不归档、不删除，待闭环交付后继续执行。

新活动文档：`docs/nowdoit/agent-minimal-closed-loop-delivery-plan.md`

最小闭环本轮目标：

1. 保留企业微信发起任务已有闭环能力。
2. 补齐 Web 发起任务完成后向用户已绑定企业微信账号发送最终报告。
3. 最终报告包含 Web 浏览器可打开的进度地址。
4. 写入 Web 任务最终报告投递审计。
5. 补充服务层测试验证 Web 发起、计划生成、进度地址、企业微信最终汇报和审计证据。

最小闭环实施结果：

1. 已新增 `internal/service/agent_conversation_web_final_report_delivery.go`，承接 Web 任务完成后的企业微信最终报告投递职责。
2. 已扩展 `AgentConversationRepository`，允许 `AgentConversationService` 查询用户外部账号并选择启用的企业微信绑定作为 Web 任务最终报告目标。
3. `ReceiveWebAgentTask` 完成后会在存在启用企业微信绑定时复用既有 `sendWeChatWorkFinalReportDelivery`，向企业微信发送最终报告，报告包含 Web 浏览器可打开的计划进度地址。
4. 已修正 Web 任务未发生企业微信发送时误写空 `wechat_work.reply_sent` 的审计行为；普通完成改写为 `agent.turn_completed`，真实企业微信投递仍写 `wechat_work.reply_sent`。
5. 已扩展 `TestAgentConversationServiceReceivesWebAgentTask`，覆盖 Web 发起、计划生成、进度地址、企业微信模板与文本投递、审计 metadata。
6. 已验证：`go test ./...`、`go vet ./...`。
7. 后续可恢复 `docs/nowdoit/agent-session-service-snapshot-recorder-modularization-plan.md` 中已写入但暂停的 4.8 recorder 拆分计划，或继续补强闭环的生产级异常路径。

当前新活动文档：`docs/nowdoit/agent-web-assistant-entry-plan.md`

助理 Web 入口与工作台整理目标：

1. 已完成：顶部主入口从“订阅 / 推荐”扩展为“订阅 / 推荐 / 助理”。
2. 已完成：三个页面使用同一 `FeedPager` 横向滑动模型，并同时挂载内容，避免滑动到目标页后才加载。
3. 已完成：`/agent`、`/agent/plans/:id` 和 `/agent/plans/:id/evidence/:recordKey` 归入助理顶部页签。
4. 已完成：助理页复用现有 Agent 后端接入，不新增后端接口。
5. 已完成：用户界面中最近任务不展示开发治理摘要；任务行只保留用户可理解的任务信息。
6. 已完成：评测基线从用户界面移除，评测仍作为开发者验证能力保留在后端。
7. 已完成：执行进度移动到发起任务下方，减少用户查找成本。
8. 已验证：`npm --prefix web run type-check`、`npm --prefix web run build`、`npm --prefix web run test`、`go test ./...`、`go vet ./...`。
9. 已部署：通过 `systemctl restart messagefeed-dev.service` 重建并启动生产 Cloudflare 模式；`https://localhost:8443/healthz` 和 `https://aroen.eu.cc/healthz` 均返回正常。
10. 已补充修复：订阅和推荐隐藏 pane 启用首次后台预加载，助理 pane 挂载后加载自身任务数据；顶部三段页签修正盒模型，保证文字位于对应选项框内部。
11. 当前补充修复：企业微信消息 `搜索最新港股消息并分析` 已确认回调到达并创建 `agent_turns`，失败点为 `agent_plan_steps.artifact_refs_json` 空数组写入成空字符串导致 PostgreSQL JSONB 解析失败；已修正 Agent 相关 JSONB 字符串数组克隆逻辑，并补齐 `processTurn` 早期失败反馈闭环。
12. 当前补充修复：主页顶部“订阅 / 推荐 / 助理”三栏移动端宽度不再压缩到无法覆盖三项；横向滑动锁定手势起始页，一次左滑或右滑只提交到相邻一个选项。
13. 当前补充修复：继续修复同一企微搜索任务的 `llm response is empty` 根因。计划器可识别来源名称查询；上下文构建使用计划批准 scope；controller run 在计划创建后对齐真实 scope；`web.search` 预取写入 executor run、observation 和 artifact；模型空响应时基于已记录证据生成降级结果。验证命令：`go test ./...`、`go vet ./...`。
14. 当前补充修复：强化搜索浏览能力。`web.search` 已增加任务型查询清洗、DuckDuckGo challenge/空结果后的 Bing 普通网页搜索、Google News RSS 与 Bing News RSS 备用源、RSS XML 结构化解析和摘要清洗；模型空响应降级回复改为仅展示用户可读事实和分析，隐藏内部能力、证据引用和治理观测；真实 repository 已持久化 controller run 对齐后的 task packet、capability scope 和 context budget。验证命令：`go test ./...`、`go vet ./...`。
15. 当前补充修复：企业微信回复改为答案优先。最终回复只保留结论、事实依据和分析过程，不再发送状态锚点、预算、质量、成本、运行观测、证据引用和动作组件；搜索任务降级回复优先选取外部 `web.search` 相关证据并过滤无关订阅源条目；`web.search` 增加 Bing 普通网页搜索 fallback，以覆盖非 RSS 外部网页。验证命令：`go test ./...`、`go vet ./...`。
16. 当前补充修复：先落地通用 `TaskSpec`、证据评分过滤和质量门禁三项根源能力。用户消息先转为任务类型、领域、时效和证据要求；搜索结果和 fallback 证据统一经过相关性、来源、时效和低质量内容评分；最终回答前校验证据数量和结论方向，证据不足或结论不被证据支持时给出用户可读降级回复。新增文件：`internal/agent/task_spec.go`、`internal/agent/evidence_score.go`、`internal/agent/answer_quality.go` 及对应测试。验证命令：`go test ./...`、`go vet ./...`。
17. 当前补充修复：Web 端 Agent 详细过程从首页底部内嵌改为独立详情页。`/agent` 只显示发起任务和最近任务；`/agent/plans/:id`、按 `scheduled_task_id` 进入的详情路由只显示执行过程详情，并提供返回助理入口。验证命令：`npm --prefix web run type-check`、`npm --prefix web run test`、`npm --prefix web run build`。
18. 当前补充修复：Web 端 Agent 详情页进一步改为单一流水线，不再平铺旧的进度、阶段、步骤、调度记录、确认记录和事件板块。流水线按真实执行顺序合并为主 Agent 接收任务、理解任务、复杂度/权限/预算判断、提示词与子 Agent 上下文、子 Agent 执行、结果汇总、质量门禁、最终交付；子 Agent 明细默认收起，可展开查看步骤、observation、artifact 和重试操作。验证命令：`npm --prefix web run type-check`、`npm --prefix web run test`、`npm --prefix web run build`。
19. 当前补充修复：流水线详情页补齐主 Agent 节点的可展开明细，修正“生成提示词并合成子 Agent 上下文”显示实时连接“已关闭”导致信息不足的问题。该节点现在显示 controller run 合成状态，展开后展示任务包、上下文预算、上下文快照、controller observation 和 artifact；接收、理解、复杂度、结果汇总、质量门禁和交付节点也提供默认收起的详情。验证命令：`npm --prefix web run type-check`、`npm --prefix web run test`、`npm --prefix web run build`。
20. 当前补充修复：企业微信异步任务在入站消息、会话和 turn 创建成功后立即发送接收确认；后台随后继续执行既有计划生成、子 Agent 执行、质量门禁和最终回复流程；接收确认写入 `wechat_work.task_accepted_feedback` 审计，且不包含计划 ID、预算、质量评分、权限、动作组件等内部执行信息。该阶段的固定话术已在后续第 23 项中替换为模型/配置模板生成。验证命令：`go test ./internal/service -run 'TestAgentConversationServiceQueuesTurnAndProcessesAsync|TestAgentConversationServiceSendsWeChatProgressNotificationWithAudit'`、`go test ./internal/service`。
21. 当前补充修复：Agent 主闭环服务完成结构拆分。`agent_main.go` 从约 3035 行降至 205 行，仅保留服务装配、构造、runner 初始化和 capability executor 组装；新增或承接文件包括 `agent_conversation_entry.go`、`agent_conversation_session.go`、`agent_button_control.go`、`agent_multiturn_flow.go`、`agent_turn_pipeline.go`、`agent_plan_feedback.go`、`agent_turn_result.go`、`agent_controller_run.go`、`agent_conversation_utils.go`、`agent_conversation_contracts.go`。验证命令：`go test ./internal/service -run TestAgentConversationServiceUsesFallbackReplyWithoutLLM`；`go test ./internal/service` 仍受主 Agent 模型规划未接入影响失败，失败集中在搜索、历史和定时 scope 相关旧用例。
22. 当前补充修复：主 Agent 模型规划正式接入 service 主路径。`createPlanForTurn` 由旧 fallback `Build()` 改为主 Agent 模型生成 `PlanSpec` 后调用 `BuildFromSpec()`；提示词集中在 `agent_main_prompts.go`；规划结果、原始响应摘要和校验结果写入 controller trace 与 plan metadata。非高风险且工具 schema 带 `confirmed` 参数的变更能力允许进入工具级确认检查点，高风险能力仍保留计划/对话框复核确认。LLM 客户端支持 `/chat/completions` 与 `/responses` 双协议智能路由，默认优先 chat completions，并对 429/5xx 做有限重试。子 Agent 工具循环上限调整为 50，耗尽后基于已有 observation 强制收敛回答。验证命令：`go test ./internal/llm ./internal/agent ./internal/service -count=1`、`RUN_REAL_LLM_TESTS=1 go test ./internal/service -run TestAgentConversationServiceRealLLMFullFlowContracts -count=1 -timeout=8m -v`。
23. 当前补充修复：企业微信用户可见回复不再由 Go 流程代码硬编码。后端只传 `stage`、`status`、`timed_out`、`error_type`、`error`、`progress_url`、`approval_url`、plan/step 摘要等结构化事实；主 Agent/模型按集中提示词 `agent_wechat_feedback_prompts.go` 生成自然短回复；模型不可用时才读取外部配置模板 `configs/agent_wechat_feedback.zh-CN.json` 兜底。后台执行超时默认提升为 10 分钟，企微发送使用独立 15 秒通知上下文，避免执行超时吞掉失败说明；Web Agent 详情页容器改为居中显示。验证命令：`go test ./internal/llm ./internal/agent ./internal/service -count=1`、`npm --prefix web run build`。

## 8. 最小验证命令

当前阶段每轮代码实现后至少运行：

```text
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```

涉及真实企业微信发送链路时，还必须补充：

```text
查询 agent.wechat_web_progress_link_snapshot 审计事件
查询 wechat_work.reply_sent 或 wechat_work.reply_failed 审计事件
核对企业微信消息是否包含 Web 浏览器可打开的进度地址
核对模板不可用时文本 fallback 是否包含同一地址
```

## 9. 当前禁止事项

- 不得删除用户文件或通过 git 操作导致文件消失，除非用户明确要求。
- 模型不得直接写数据库。
- controller 不得绕过 executor 直接调用业务变更 capability。
- executor 不得获得超出本次任务的 capability scope。
- 密钥、token、Webhook URL、数据库 DSN 不得进入模型上下文或 context trace。
- 企业微信进度地址投递不得绕过 Web 登录、权限、审批和审计边界。
- 后续迭代不得以继续扩大大文件作为常态实现方式。
