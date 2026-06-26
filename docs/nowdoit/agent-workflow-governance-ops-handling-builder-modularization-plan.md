# Agent Workflow Governance Ops Handling Builder 模块化治理计划

**创建日期**：2026-06-26

## 1. 本轮目标

在 release/ops builder 已迁出 50 个纯函数后，继续治理 `internal/service/agent_workflow_governance.go`。本轮聚焦剩余运维处置、灰度阶段、反馈工单、告警升级、写审批、证据交互和双端进度相关纯 builder，目标是在不改变 `ListTasks` 聚合结果、JSON 字段、状态取值和审计语义的前提下，进一步降低治理主文件职责密度。

## 2. 当前基线

- `internal/service/agent_workflow_governance.go`：2127 行，仍偏大。
- `internal/service/agent_workflow_release_ops_builders.go`：1599 行，已承接 release/ops 相关 50 个纯 builder 和 1 个审计读取 helper。
- 最近后端验证已通过：
  - `go test ./...`
  - `go vet ./...`

## 3. 实施范围

1. 梳理 `agent_workflow_governance.go` 中剩余运维处置、灰度阶段、反馈工单、证据交互和双端进度相关 builder 群组。
2. 优先迁出输入输出均为 response DTO、domain 对象或基础值、无 repository 访问、无审计写入副作用的纯函数。
3. 新增职责明确的小文件承接迁出的 builder，避免继续扩大既有 release/ops 承接文件。
4. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 4. 第一实施单元：运维处置基础 Builder 迁出

本小轮优先迁出运行阶段之后的运维处置基础 builder。这组函数主要组合既有响应 DTO，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentWriteRampStage`
2. `buildAgentWeChatFeedbackLoop`
3. `buildAgentOperationsClosedLoop`
4. `buildAgentOpsDashboardInteraction`
5. `buildAgentAlertDedupeEscalation`
6. `buildAgentWriteStageRecord`
7. `buildAgentWeChatFeedbackTicket`
8. `buildAgentOperationsHandling`
9. `buildAgentOpsActionDefinition`
10. `buildAgentAlertEscalationPolicy`

拟新增文件：

1. `internal/service/agent_workflow_ops_handling_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中相关 builder 的调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。
4. 不删除历史能力或归档文件。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

实施结果：

1. 已新增 `internal/service/agent_workflow_ops_handling_builders.go`，承接 10 个运维处置基础 builder。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 中相关 builder 的调用顺序。
3. `agent_workflow_governance.go` 从 2127 行降至 1832 行；新增文件为 302 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 89 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 4.2 第二实施单元：审批执行与 SLA Builder 迁出

本小轮继续迁出写阶段审批、反馈工单生命周期、运营动作闭环、API 执行、告警回执、审批按钮、反馈 SLA、运营执行记录和企业微信审批回调相关纯 builder。这组函数仍以响应 DTO 聚合为主，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentWriteStageApproval`
2. `buildAgentFeedbackTicketLifecycle`
3. `buildAgentOperationsActionClosure`
4. `buildAgentOpsAPIExecution`
5. `buildAgentAlertEscalationReceipt`
6. `buildAgentWriteApprovalButton`
7. `buildAgentFeedbackTicketSLA`
8. `buildAgentOperationsExecution`
9. `buildAgentOpsExecutionRecord`
10. `buildAgentWeChatApprovalCallback`

拟承接文件：

1. `internal/service/agent_workflow_ops_handling_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中相关 builder 的调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`

实施结果：

1. 已将 4.2 列出的 10 个审批执行与 SLA 相关 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 中相关 builder 的调用顺序。
3. `agent_workflow_governance.go` 从 1832 行降至 1520 行；`agent_workflow_ops_handling_builders.go` 从 302 行增至 614 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 99 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已通过：
   - `go vet ./...`
   - `go test ./...`。当前沙箱禁止 `httptest` 本地监听端口，普通沙箱执行失败于 `socket: operation not permitted`；提升权限后同一命令通过。

## 4.3 第三实施单元：证据闭环与双端进度基础 Builder 迁出

本小轮继续迁出反馈 SLA 报表、告警自动恢复、运营证据、统一进度组件、证据详情页、回调重放工具、恢复策略配置、双端进度证据、企业微信进度卡片和 Web 证据交互相关纯 builder。这组函数仍以响应 DTO 聚合为主，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentFeedbackSLAReport`
2. `buildAgentAlertAutoRecovery`
3. `buildAgentOperationsEvidence`
4. `buildAgentUnifiedProgressComponent`
5. `buildAgentEvidenceDetailPage`
6. `buildAgentCallbackReplayTool`
7. `buildAgentRecoveryPolicyConfig`
8. `buildAgentDualEndProgressEvidence`
9. `buildAgentWeChatProgressCard`
10. `buildAgentWebEvidenceInteraction`

拟承接文件：

1. `internal/service/agent_workflow_ops_handling_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中相关 builder 的调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`

实施结果：

1. 已将 4.3 列出的 10 个证据闭环与双端进度基础 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 中相关 builder 的调用顺序。
3. `agent_workflow_governance.go` 从 1520 行降至 1224 行；`agent_workflow_ops_handling_builders.go` 从 614 行增至 911 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 109 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 4.4 第四实施单元：权限审批、模板发送与真实交互 Builder 迁出

本小轮继续迁出回调重放权限、恢复策略审计、双端交互、企业微信模板渲染、Web 证据路由、回调重放审批、恢复策略持久化、双端交互发布、企业微信模板发送、Web 证据详情、回调重放执行、恢复策略版本和双端真实交互相关纯 builder。这组函数仍以响应 DTO 聚合为主，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentCallbackReplayPermission`
2. `buildAgentRecoveryPolicyAudit`
3. `buildAgentDualEndInteraction`
4. `buildAgentWeChatTemplateRender`
5. `buildAgentWebEvidenceRoute`
6. `buildAgentCallbackReplayApproval`
7. `buildAgentRecoveryPolicyPersist`
8. `buildAgentDualEndInteractionLaunch`
9. `buildAgentWeChatTemplateSend`
10. `buildAgentWebEvidenceDetailView`
11. `buildAgentCallbackReplayExecution`
12. `buildAgentRecoveryPolicyVersion`
13. `buildAgentDualEndRealInteraction`

拟承接文件：

1. `internal/service/agent_workflow_ops_handling_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中相关 builder 的调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`

实施结果：

1. 已将 4.4 列出的 13 个权限审批、模板发送与真实交互相关 builder 追加迁入 `internal/service/agent_workflow_ops_handling_builders.go`。
2. 已从 `internal/service/agent_workflow_governance.go` 移除同一函数块，不改变聚合摘要 JSON 字段、状态取值、summary 文案或 `ListTasks` 中相关 builder 的调用顺序。
3. `agent_workflow_governance.go` 从 1224 行降至 827 行；`agent_workflow_ops_handling_builders.go` 从 911 行增至 1308 行。
4. 当前 workflow governance builder 拆分累计新增 5 个小文件，合计承接 122 个低风险纯函数；文件数量增加与职责拆分相匹配，不属于冗余扩张。
5. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 5. 非目标

- 本轮不改变任务聚合 API 返回字段。
- 本轮不改变企业微信消息投递、审批、回放和恢复行为。
- 本轮不重写 workflow governance 的整体执行顺序。
- 本轮不处理 `agent_session_service.go` 和前端 `AgentPlanView.vue` 的拆分。

## 6. 验收标准

- `agent_workflow_governance.go` 行数有可度量下降。
- 新文件职责单一，迁移后调用方无需改变业务语义。
- `go test ./...` 和 `go vet ./...` 通过。
- 主进度文档和 Agent 设计文档同步记录实际变更。
