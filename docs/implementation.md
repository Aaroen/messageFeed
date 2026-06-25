# messageFeed 实施进度台账

**最后更新**：2026-06-26

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
| 工作区 | 本轮文档记录按 ops handling 第一实施单元迁出后的验证结果更新；提交推送后以 `git status -sb` 为准 |
| 当前活动文档 | `docs/nowdoit/agent-workflow-governance-ops-handling-builder-modularization-plan.md` |
| 最近本轮验证 | `go test ./...`、`go vet ./...` 已通过 |
| 最近核对提交 | 以 `git log -1 --oneline` 为准；本文档作为实现进度台账，不替代 Git 提交记录 |

## 3. 已完成能力

### 3.1 Agent 基础运行时

- 已具备 Agent session、turn、run、plan、approval、scheduled task、eval、recovery、audit 等基础对象和服务面。
- Web 侧已有 Agent 任务入口、任务工作台、计划进度页、审批页、证据与回放相关视图。
- 后端已形成任务聚合接口，用于向 Web 工作台提供任务、SLA、成本、告警、部署验证、企业微信联动、进度证据、恢复策略和评测摘要。
- 已具备任务进度 URL 字段，并在 Web 发起任务后可跳转进度页。

### 3.2 企业微信接入与交互

- 已具备企业微信自建应用 callback、OAuth、文本消息发送和模板卡片发送基础能力。
- 已具备企业微信审批按钮、回放、恢复、模板卡片、灰度、签收、最终报告、反馈闭环等多项治理摘要。
- 已完成真实交互自动化摘要 `real_interaction_automation` 的后端和前端展示。
- 已完成 `wechat_web_progress_link` 后端聚合摘要和审计快照，字段覆盖进度地址、地址来源、投递通道、模板状态、fallback 状态、浏览器目标和审计引用。
- 已接入企业微信进度通知真实投递：模板卡片优先，文本 fallback 保底，卡片和 fallback 均包含 Web 浏览器进度地址。
- `wechat_web_progress_link` 聚合摘要已读取真实 `agent.plan_progress_notification` 或 `agent.plan_started_feedback` 审计事件中的进度地址、模板状态和 fallback 状态。
- 企业微信 OAuth 绑定链路已核对：OAuth URL 必须由已登录 Web 用户生成；callback 按 OAuth state 中的 `user_id` 绑定 external account 并创建同一用户的 Web session；`/api/v1/auth/me` 返回当前用户 bindings；disabled binding 在企业微信 external account 解析时被拒绝。

### 3.3 Web 进度与治理展示

- Web 任务工作台已展示多数 Agent 运行、企业微信、审批、回放、恢复和真实交互自动化摘要。
- Web 任务工作台已展示 `wechat_web_progress_link` 摘要，包括进度地址、投递通道、模板状态、fallback 状态、浏览器目标和检查项。
- Web 进度页已支持计划 ID 或调度任务 ID 维度的进度查询、轮询、步骤、证据和审批状态展示。
- 前端已有任务创建表单，并通过 `createAgentTask` 以 `channel=web` 发起任务。

### 3.4 质量与验证

- 后端已有 `agent_progress_service_test.go` 对 `WeChatWebProgressLink` 的返回字段进行断言。
- 最近一轮全量验证已覆盖 Go 测试、Go vet、前端测试、类型检查和前端构建。
- 当前新增治理文件将部分逻辑从大文件中抽离，包括：
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

## 4. 当前未完成缺口

| 优先级 | 缺口 | 当前判断 |
| --- | --- | --- |
| P1 | Web 浏览器进度地址权限校验与企业微信身份绑定仍需继续强化 | 进度与计划接口用户归属校验已有测试；OAuth / external account 绑定链路已有服务测试；仍需完成前端进度摘要拆分和入口体验治理 |
| P1 | Agent 能力注册、上下文记忆、计划执行和评测体系仍需按设计持续补齐 | 已有较多基础对象，但未能证明全部设计均已完整实现 |
| P1 | 大文件职责边界仍不理想 | 需要继续拆分 `agent_session_service.go`、`agent_workflow_governance.go`、`AgentPlanView.vue` |

## 5. 架构质量核对

当前关键大文件规模：

| 文件 | 行数 | 判断 |
| --- | ---: | --- |
| `internal/service/agent_session_service.go` | 5936 | 仍明显过大；本轮已迁出任务列表聚合响应 DTO、任务摘要 DTO、转换函数和任务摘要状态 helper，后续应继续拆分聚合 builder、审计 recorder 和服务编排 |
| `internal/service/agent_workflow_governance.go` | 1832 | 仍偏大；已迁出 metadata builder、基础聚合 builder、企业微信组件 builder、release/ops 基础 builder、发布执行/日报闭环 builder、发布窗口/外部监控 builder、生产发布 builder、运行态反馈闭环 builder 和运维处置基础 builder 群组，后续继续拆分剩余审批执行、证据交互和双端进度等治理摘要 builder |
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
