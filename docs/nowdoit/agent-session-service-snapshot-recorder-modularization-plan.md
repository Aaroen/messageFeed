# Agent Session Service Snapshot Recorder 模块化治理计划

**创建日期**：2026-06-26

## 1. 本轮目标

在 `agent_workflow_governance.go` 已完成纯 builder 拆分后，继续治理 `internal/service/agent_session_service.go`。当前该文件仍为 5936 行，主要混合了 session 编排、任务聚合、审计快照记录、进度构造和响应转换等职责。本轮优先迁出无业务分支、只负责写入审计快照的 `recordAgent*Snapshot` 方法，降低主服务文件职责密度。

## 2. 当前基线

- `internal/service/agent_session_service.go`：5936 行，仍明显过大。
- `internal/service/agent_workflow_governance.go`：739 行，已不再承接 `buildAgent*` 纯 builder。
- `internal/service/agent_workflow_ops_handling_builders.go`：1308 行。
- `internal/service/agent_workflow_foundation_builders.go`：500 行。
- 最近后端验证已通过：
  - `go test ./...`
  - `go vet ./...`

## 3. 实施范围

1. 梳理 `agent_session_service.go` 中连续的 `recordAgent*Snapshot` 方法群组。
2. 优先迁出只调用 `s.repository.CreateAgentAuditLog`、不改变业务状态、不参与事务编排的审计快照 recorder。
3. 新增职责明确的小文件承接迁出的 recorder。
4. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 4. 第一实施单元：基础治理快照 Recorder 迁出

本小轮优先迁出 `ListTasks` 前段基础治理摘要相关 recorder。这组方法只把既有响应 DTO 序列化为审计 metadata，不改变任务聚合调用顺序。

拟迁出内容：

1. `recordAgentAlertPolicyDecision`
2. `recordAgentProductionDrillSnapshot`
3. `recordAgentWriteSandboxSnapshot`
4. `recordAgentE2EAcceptanceSnapshot`
5. `recordAgentRealIntegrationSnapshot`
6. `recordAgentOpsAcceptanceSnapshot`
7. `recordAgentWriteGraySnapshot`
8. `recordAgentAlertChannelSnapshot`
9. `recordAgentLaunchDrillRecord`
10. `recordAgentWeChatNativeIntegrationSnapshot`

拟新增文件：

1. `internal/service/agent_session_snapshot_recorders.go`

实施约束：

1. 不改变审计事件类型、metadata 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中 recorder 调用顺序。
3. 方法仍保持 `AgentSessionService` receiver 和 package 内部可见。
4. 不删除历史能力或归档文件。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

实施结果：

1. 已新增 `internal/service/agent_session_snapshot_recorders.go`，承接第一批 10 个基础治理审计快照 recorder。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 中 recorder 调用顺序。
3. `agent_session_service.go` 从 5936 行降至 5731 行；新增 recorder 文件为 211 行。
4. 本小轮新增文件数量与审计快照职责拆分相匹配，不属于冗余扩张。
5. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 4.2 第二实施单元：发布执行与日报闭环 Recorder 迁出

本小轮继续迁出发布执行、审批、日报、预生产验收、按钮回调和监控相关 recorder。这组方法只把既有响应 DTO 序列化为审计 metadata，不改变任务聚合调用顺序。

拟迁出内容：

1. `recordAgentWriteReplaySnapshot`
2. `recordAgentLaunchApprovalSnapshot`
3. `recordAgentDailyReportSnapshot`
4. `recordAgentPreprodAcceptanceSnapshot`
5. `recordAgentButtonLoopSnapshot`
6. `recordAgentWriteExecuteSnapshot`
7. `recordAgentDailyPersistSnapshot`
8. `recordAgentPostLaunchMonitorSnapshot`
9. `recordAgentReleaseApprovalSnapshot`
10. `recordAgentButtonCallbackSnapshot`

拟承接文件：

1. `internal/service/agent_session_snapshot_recorders.go`

实施约束：

1. 不改变审计事件类型、metadata 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中 recorder 调用顺序。
3. 方法仍保持 `AgentSessionService` receiver 和 package 内部可见。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`

实施结果：

1. 已将 4.2 列出的 10 个发布执行与日报闭环 recorder 追加迁入 `internal/service/agent_session_snapshot_recorders.go`。
2. 已从 `internal/service/agent_session_service.go` 移除同一方法块，不改变审计事件类型、metadata 字段、状态取值、summary 文案或 `ListTasks` 中 recorder 调用顺序。
3. `agent_session_service.go` 从 5731 行降至 5518 行；`agent_session_snapshot_recorders.go` 从 211 行增至 424 行。
4. 当前 snapshot recorder 拆分累计承接 20 个审计快照 recorder；文件数量未继续扩张。
5. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 5. 非目标

- 本轮不改变任务聚合 API 返回字段。
- 本轮不改变 Agent 执行、审批、回放、恢复或企业微信投递行为。
- 本轮不重写 `ListTasks` 的整体编排。
- 本轮不处理前端 `AgentPlanView.vue` 拆分。

## 6. 验收标准

- `agent_session_service.go` 行数有可度量下降。
- 新文件职责单一，迁移后调用方无需改变业务语义。
- `go test ./...` 和 `go vet ./...` 通过。
- 主进度文档和 Agent 设计文档同步记录实际变更。
