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

1. [ ] 梳理 `agent_workflow_governance.go` 中可迁移 builder 群组。
2. [ ] 新增独立治理 builder 文件，迁出低风险纯函数。
3. [ ] 运行 `go test ./...` 和 `go vet ./...`。
4. [ ] 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。
5. [ ] 记录实施结果，归档本文档并创建下一轮活动文档。

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
