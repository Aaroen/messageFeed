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
5. [ ] 记录实施结果，归档本文档并创建下一轮活动文档。

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

1. 已新增 `internal/service/agent_workflow_metadata_builders.go`，承接 metadata builder 群组。
2. 已从 `internal/service/agent_workflow_governance.go` 迁出 13 个纯函数，不改变 metadata 字段名、调用方或任务聚合顺序。
3. `agent_workflow_governance.go` 从 4626 行降至 4296 行；新增文件为 338 行。
4. 已通过：
   - `go test ./...`
   - `go vet ./...`

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
