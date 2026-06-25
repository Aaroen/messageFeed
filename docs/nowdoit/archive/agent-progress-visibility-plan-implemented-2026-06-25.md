# Agent 进度视图数据面实施计划

**创建日期**：2026-06-25

## 1. 本轮目标

建立 Web 与企业微信可共用的 Agent 进度数据面，使用户可以查看一个任务从入口消息、session、turn、controller run、plan、approval、executor run、observation、artifact 到 scheduled task 的实时或准实时状态。

## 2. 实施范围

1. Domain 与 service response：
   - 定义统一 `AgentProgressSnapshot`。
   - 聚合 session、turn、plan、plan step、run、context trace、observation、artifact、scheduled task 和 approval。
2. Repository 查询：
   - 按 `plan_id`、`turn_id`、`run_id` 或 `scheduled_task_id` 查询任务进度。
   - 保证只能查询当前用户的数据。
3. API：
   - 新增 `GET /api/v1/agent/progress`，支持 `plan_id`、`turn_id`、`run_id`、`scheduled_task_id` 查询参数。
   - 响应包含当前状态、阶段列表、最近事件、下一步动作和可见细节。
4. Web：
   - 新增或扩展 Agent plan 页面，展示统一进度快照。
   - 初期使用轮询，不引入 SSE/WebSocket。
5. 企业微信：
   - 审批提示和执行完成消息中保留进度地址。
   - 后续可基于同一 snapshot 生成文本进度摘要。
6. 测试：
   - 覆盖 service 聚合逻辑、handler 参数校验和 API 响应结构。

## 3. 非目标

- 本轮不实现真正实时推送。
- 本轮不实现复杂甘特图或流程图 UI。
- 本轮不新增外部通知通道。
- 本轮不改变 Agent 执行状态机。

## 4. 验收标准

- 后端可以通过统一 API 查询 Agent 任务进度快照。
- 快照至少包含 plan steps、runs、observations、approvals 和 scheduled task 状态。
- Web 页面能展示该快照的关键阶段和详细内容。
- 企业微信消息中的进度地址继续可用。
- `go test ./...`、`go vet ./...`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，下一轮推进准实时更新能力：Web 轮询节流、可选 SSE、企业微信阶段性进度推送，以及 executor observation 更细粒度事件化。

## 6. 完成记录

**完成日期**：2026-06-25

已完成：

- 新增统一 `AgentProgressSnapshot`、阶段、事件和 scheduled task 响应结构。
- `AgentSessionService.GetProgress` 支持按 `plan_id`、`turn_id`、`run_id`、`scheduled_task_id` 聚合进度。
- `AgentRepository.ListAgentScheduledTasksByRefs` 支持按 plan、turn、run 反查调度任务。
- 新增 `GET /api/v1/agent/progress`。
- Web `AgentPlanView` 改为使用 progress snapshot，并展示统一进度阶段、定时任务和最近事件。
- 补充 service 聚合测试和 handler 参数/响应测试。

验证命令：

```text
go test ./...
go vet ./...
npm --prefix web run type-check
npm --prefix web run build
```

验证结果均通过。
