# Agent Workflow Governance Builder 模块化治理计划

**创建日期**：2026-06-26

## 1. 本轮目标

在 `agent_session_service.go` 任务列表响应职责完成阶段性迁出后，继续治理 `internal/service/agent_workflow_governance.go`。本轮聚焦文件中大量纯 builder 函数和治理摘要响应构建函数混杂问题，目标是在不改变 `ListTasks` 聚合结果、JSON 字段和审计语义的前提下，将可独立维护的一组 workflow governance builder 迁出到小文件。

## 2. 实施范围

1. 梳理 `agent_workflow_governance.go` 中低耦合 builder 群组。
2. 优先选择输入输出均为 domain 或 response DTO、无 repository 访问、无审计写入副作用的纯函数。
3. 新增职责明确的小文件承接迁出的 builder。
4. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 3. 当前基线

- `internal/service/agent_workflow_governance.go`：4626 行，明显过大。
- `internal/service/agent_session_service.go`：5936 行，仍明显过大，但本轮不继续扩大。
- `internal/service/agent_task_list_responses.go`：326 行，承接任务列表响应 DTO 和摘要 helper。
- 最近后端验证已通过：
  - `go test ./...`
  - `go vet ./...`

## 4. 本轮实施清单

1. [x] 梳理 `agent_workflow_governance.go` 中可迁移 builder 群组。
2. [x] 新增独立治理 builder 文件，迁出低风险纯函数。
3. [x] 运行 `go test ./...` 和 `go vet ./...`。
4. [x] 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。
5. [x] 记录实施结果，归档本文档并创建下一轮活动文档。

## 4.1 第一实施单元：Metadata Builder 迁出

本小轮先迁出 workflow governance 文件顶部的 metadata builder 群组。这些函数输入输出明确，主要生成 `domain.AgentJSON`，不访问 repository，也不写审计事件。

拟迁出内容：

1. `buildAgentCapabilityPolicyMetadata`
2. `agentCapabilityPolicyDecision`
3. `stricterCapabilityDecision`
4. `buildAgentHandoffMetadata`
5. `buildAgentRuntimeObservabilityMetadata`
6. `buildAgentPlanRecoveryMetadata`
7. `agentPlanRecoveryStrategy`
8. `buildAgentScheduledTaskRecoveryMetadata`
9. `buildAgentResultQualityMetadata`
10. `buildAgentDeploymentAcceptanceMetadata`
11. `buildAgentCostSummaryMetadata`
12. `agentTextTokenEstimate`
13. `agentDeploymentCheck`

拟新增文件：

1. `internal/service/agent_workflow_metadata_builders.go`

实施约束：

1. 不改变 metadata 字段名和值语义。
2. 不改变调用方和任务聚合顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

实施结果：

1. 已新增 `internal/service/agent_workflow_wechat_builders.go`，承接企业微信组件、callback readiness、原生动作定义、payload 构造和按钮 helper。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 7 个纯函数，不改变企业微信按钮 key、fallback 文案、payload 字段、状态取值或 `ListTasks` 企业微信相关调用顺序。
3. `agent_workflow_governance.go` 从 3893 行降至 3717 行；新增文件为 183 行。
4. 已通过：
   - `go test ./...`
   - `go vet ./...`

本轮归档前结论：

1. 已完成 metadata builder、基础聚合 builder 和企业微信组件 builder 三个低风险函数群组迁出。
2. `agent_workflow_governance.go` 从本轮基线 4626 行降至 3717 行，累计减少 909 行。
3. 新增的 3 个 builder 文件均保持 package 内部可见 helper，不扩大导出 API，不改变任务聚合响应语义。

实施结果：

1. 已新增 `internal/service/agent_workflow_metadata_builders.go`，承接 metadata builder 群组。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 13 个纯函数，不改变 metadata 字段名、调用方或任务聚合顺序。
3. `agent_workflow_governance.go` 从 4626 行降至 4296 行；新增文件为 338 行。
4. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 4.2 第二实施单元：基础聚合 Builder 迁出

本小轮迁出 workflow governance 中成本、告警、趋势、部署验证和生产演练相关的纯聚合 builder。这组函数主要服务 `ListTasks` 的基础运行治理摘要，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentTaskCostSummary`
2. `buildAgentAlertSummary`
3. `buildAgentAlertPolicy`
4. `agentAlertReasonEnabled`
5. `agentAlertReasonSeverity`
6. `buildAgentCostTrend`
7. `buildAgentTrendSnapshot`
8. `buildAgentDeploymentVerification`
9. `buildAgentProductionDrill`

拟新增文件：

1. `internal/service/agent_workflow_foundation_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段和状态取值。
2. 不改变 `ListTasks` 中 builder 调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

实施结果：

1. 已新增 `internal/service/agent_workflow_foundation_builders.go`，承接成本、告警、趋势、部署验证和生产演练相关基础聚合 builder。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 9 个纯函数，不改变聚合摘要 JSON 字段、状态取值或 `ListTasks` 调用顺序。
3. `agent_workflow_governance.go` 从 4296 行降至 3893 行；新增文件为 412 行。
4. 已通过：
   - `go test ./...`
   - `go vet ./...`

## 4.3 第三实施单元：企业微信组件 Builder 迁出

本小轮迁出企业微信组件、callback readiness、原生动作定义和 payload 构造相关纯 builder。这组函数围绕企业微信交互摘要与按钮 payload，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentWeChatComponentSet`
2. `buildAgentWeChatCallbackReadiness`
3. `buildAgentWeChatNativeActions`
4. `buildAgentWeChatNativePayload`
5. `agentWeChatActionStyle`
6. `nativeButtonHasURL`
7. `nativeButtonExists`

拟新增文件：

1. `internal/service/agent_workflow_wechat_builders.go`

实施约束：

1. 不改变企业微信按钮 key、fallback 文案、payload 字段和状态取值。
2. 不改变 `ListTasks` 中企业微信相关 builder 的调用顺序。
3. helper 仍保持 package 内部可见，不扩大导出面。

验收方式：

1. `go test ./...`
2. `go vet ./...`
3. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 5. 非目标

- 本轮不改变任务聚合 API 返回字段。
- 本轮不改变企业微信消息投递、审批、回放和恢复行为。
- 本轮不重写 workflow governance 的整体执行顺序。
- 本轮不删除历史能力或归档文件。

## 6. 验收标准

- `agent_workflow_governance.go` 行数有可度量下降。
- 新文件职责单一，函数迁移后调用方无需改变业务语义。
- `go test ./...` 和 `go vet ./...` 通过。
- 主进度文档和 Agent 设计文档同步记录实际变更。
