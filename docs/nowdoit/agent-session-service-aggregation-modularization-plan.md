# Agent Session 聚合职责模块化治理计划

**创建日期**：2026-06-26

## 1. 本轮目标

在完成 Web 进度权限绑定和前端摘要拆分后，继续治理后端大文件职责边界。本轮聚焦 `internal/service/agent_session_service.go` 中任务列表聚合、响应 DTO 和摘要 builder 混杂问题，目标是在不改变 API 响应语义的前提下，将可独立维护的任务聚合响应类型或构建逻辑迁出到小文件。

## 2. 实施范围

1. 梳理 `AgentTaskListResult`、`AgentTaskSummaryResponse`、任务聚合 summary builder 与 `ListTasks` 的职责边界。
2. 优先迁出低风险、无副作用的响应 DTO 或纯函数 builder。
3. 保持 `ListTasks` 对 repository、审计记录和前端 API 的行为不变。
4. 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。

## 3. 当前基线

- `internal/service/agent_session_service.go`：6255 行，仍明显过大。
- `internal/service/agent_workflow_governance.go`：4626 行，后续继续治理。
- `web/src/views/AgentPlanView.vue`：3680 行，已迁出两个企业微信摘要组件，但仍需继续拆分。
- 最近完整验证矩阵已通过：
  - `go test ./...`
  - `go vet ./...`
  - `npm --prefix web run test`
  - `npm --prefix web run type-check`
  - `npm --prefix web run build`

## 4. 本轮实施清单

1. [ ] 梳理 `agent_session_service.go` 中任务聚合响应类型和 builder 可迁移边界。
2. [ ] 新增职责明确的小文件，优先迁出任务列表响应 DTO 或纯函数摘要逻辑。
3. [ ] 运行 Go 测试和 vet；如 API 类型或前端展示受影响，则补跑前端验证。
4. [ ] 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。
5. [ ] 记录实施结果，归档本文档并创建下一轮活动文档。

## 5. 非目标

- 本轮不重写 `ListTasks` 的数据查询流程。
- 本轮不改变 JSON 字段、前端 API 类型和现有审计事件名称。
- 本轮不把后端聚合逻辑迁入前端。
- 本轮不删除历史能力或归档文件。

## 6. 验收标准

- `agent_session_service.go` 行数有可度量下降。
- 新文件职责单一，命名能反映 DTO 或 builder 边界。
- `go test ./...` 和 `go vet ./...` 通过。
- 主进度文档和 Agent 设计文档同步记录实际变更。
