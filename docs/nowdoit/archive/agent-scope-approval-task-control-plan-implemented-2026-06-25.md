# Agent scope 校验、Web 审批与任务控制面计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Web/企微任务入口、统一进度、定时任务 worker 和企微最终汇报已具备的基础上，增强 Agent 执行安全与控制面：确保 executor 调用严格受批准的 capability scope 约束，并让 Web 端具备审批、拒绝、取消和查看任务控制状态的最小闭环。

## 2. 实施范围

1. capability scope 二次校验：
   - executor 执行前校验当前 capability 是否包含在 plan step / approved scope 中。
   - scope 不匹配时拒绝执行，写入 observation、audit log 和 plan step error。
   - 覆盖 `agent.schedule_task`、联网能力、历史查询和只读订阅能力。
2. Web 审批控制面：
   - 在 Agent plan/progress 页面展示 pending approval。
   - 提供批准、拒绝入口，复用现有 approval service。
   - 审批操作后刷新 progress snapshot。
3. 任务取消与失败控制：
   - 为 scheduled task 提供最小取消 service/API。
   - 取消后状态变为 canceled，并在 progress 中体现。
   - 对 plan step 失败策略增加结构化展示字段。
4. 追溯与审计：
   - scope 拒绝、Web 审批、取消任务均写入 audit log。
   - progress recent events 中体现控制面事件。
5. 测试：
   - 覆盖 scope 拒绝、Web 批准/拒绝、scheduled task 取消、progress 事件衔接和前端类型检查。

## 3. 非目标

- 本轮不实现复杂多级权限模型。
- 本轮不实现批量任务管理。
- 本轮不新增 WebSocket/SSE。
- 本轮不重写现有 planner 策略，仅加强 executor 前置校验和控制面。

## 4. 验收标准

- 未获批准或超出 approved scope 的 capability 调用不会执行。
- Web 用户可以在进度页完成批准或拒绝，并看到状态变化。
- 用户可以取消 queued/running scheduled task，取消状态可查询。
- scope 拒绝、审批、取消均有 audit log 和 progress event。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进 Agent 评测用例自动运行、安全对抗样例、计划步骤级重试策略和更细粒度的实时进度推送。

## 6. 实施结果

**完成日期**：2026-06-25

- `TurnRunner` 新增动态 `AllowedToolKeys`，工具定义与工具执行阶段均受当前 plan `AllowedScopes` 约束。
- 超出 approved scope 的工具调用不会进入底层 executor，会生成 forbidden/failed observation，并记录 `agent.capability_scope_denied` audit log。
- planner 扩展历史查询、首条消息、偏好查询和调度确认类意图识别；确认类调度回复可进入已批准计划，避免重复审批。
- `agent.schedule_task` scope 兼容旧 `agent.schedule_message` 工具入口，保留兼容行为。
- 新增按 approval id 批准/拒绝 API：`POST /api/v1/agent/approval-records/:id/approve` 和 `/reject`。
- approval 决策会写入 `agent.approval_decided` audit log，并更新关联 plan 状态。
- 新增 scheduled task 取消 API：`POST /api/v1/agent/scheduled-tasks/:id/cancel`。
- scheduled task 取消后状态变为 `canceled`，写入 `agent.scheduled_task_canceled` audit log，并可通过 progress recent events 查询。
- Web `AgentPlanView` 在进度页展示 pending approval 的批准/拒绝按钮，以及 queued/running/input_required scheduled task 的取消按钮。
- 补充 scope 拒绝、approval by id、scheduled task cancel、progress event 和 handler 测试。

## 7. 验证记录

- `go test ./...`
- `go vet ./...`
- `npm --prefix web run test`
- `npm --prefix web run type-check`
- `npm --prefix web run build`

以上命令均已通过。
