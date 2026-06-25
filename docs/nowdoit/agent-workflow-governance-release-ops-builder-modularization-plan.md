# Agent Workflow Governance Release/Ops Builder 模块化治理计划

**创建日期**：2026-06-26

## 1. 本轮目标

在 metadata、基础聚合和企业微信组件 builder 已迁出后，继续治理 `internal/service/agent_workflow_governance.go`。本轮聚焦发布、运维、灰度、监控、日报和按钮闭环相关纯 builder，目标是在不改变 `ListTasks` 聚合结果、JSON 字段、状态取值和审计语义的前提下，进一步降低治理主文件职责密度。

## 2. 实施范围

1. 梳理 `agent_workflow_governance.go` 中发布、运维、灰度、监控、日报和按钮闭环相关 builder 群组。
2. 优先迁出输入输出均为 response DTO、domain 对象或基础值、无 repository 访问、无审计写入副作用的纯函数。
3. 新增职责明确的小文件承接迁出的 builder。
4. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 3. 当前基线

- `internal/service/agent_workflow_governance.go`：3717 行，仍明显过大。
- `internal/service/agent_workflow_metadata_builders.go`：338 行。
- `internal/service/agent_workflow_foundation_builders.go`：412 行。
- `internal/service/agent_workflow_wechat_builders.go`：183 行。
- 最近后端验证已通过：
  - `go test ./...`
  - `go vet ./...`

## 4. 本轮实施清单

1. [ ] 梳理发布、运维、灰度、监控、日报和按钮闭环 builder 群组。
2. [ ] 新增独立 release/ops builder 文件，迁出低风险纯函数。
3. [ ] 运行 `go test ./...` 和 `go vet ./...`。
4. [ ] 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。
5. [ ] 记录实施结果，归档本文档并创建下一轮活动文档。

## 4.1 第一实施单元：发布与运维基础 Builder 迁出

本小轮优先迁出 `agent_workflow_governance.go` 前段的发布、运维和执行准备相关纯 builder。这组函数主要组合既有响应 DTO，不访问 repository，不写审计事件。

拟迁出内容：

1. `buildAgentLoadTestSummary`
2. `buildAgentWriteSandbox`
3. `buildAgentE2EAcceptance`
4. `buildAgentRealIntegrationReadiness`
5. `buildAgentWriteLeastPrivilege`
6. `buildAgentOpsAcceptance`
7. `buildAgentWriteGrayPolicy`
8. `buildAgentAlertChannel`
9. `buildAgentLaunchDrillRecord`
10. `buildAgentWeChatNativeIntegration`

拟新增文件：

1. `internal/service/agent_workflow_release_ops_builders.go`

实施约束：

1. 不改变聚合摘要 JSON 字段、状态取值和 summary 文案。
2. 不改变 `ListTasks` 中相关 builder 的调用顺序。
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
