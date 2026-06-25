# Agent worker 常驻装配与企微最终汇报计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Web 任务入口与 `AgentScheduledTaskWorkerService.RunDueOnce` 已完成的基础上，补齐后台调度 worker 的运行时装配和企业微信最终汇报闭环，使到期定时任务不只停留在可调用 service，而是可以由后台进程持续领取、执行、记录并向目标用户汇报。

## 2. 实施范围

1. 后台 worker 运行时装配：
   - 在 API 进程启动阶段构造 scheduled task worker service。
   - 新增后台循环，按节点 ID 生成 worker ID，周期性调用 `RunDueOnce`。
   - 支持空依赖时安全跳过，不影响无数据库或未配置企微的部署。
2. 企业微信最终汇报：
   - 基于 worker item 的 `ReportText` 和 scheduled task 的 `target_channel` / `target_ref` 决定是否发送企微消息。
   - 记录发送成功、失败或跳过的 audit log。
   - 不新增外部通知通道；本轮仅接入已有企业微信 sender。
3. Web 任务列表：
   - 新增最小任务列表 API，返回当前用户最近 Web/调度 Agent 任务、plan、turn、status、progress URL。
   - Web `/agent` 页面展示最近任务，支持进入进度页。
4. 进度衔接：
   - 确保 worker 创建的 run 和 scheduled task 状态能通过 `GET /api/v1/agent/progress?scheduled_task_id=...` 查询到。
   - 若发现关联字段不足，补充查询或回写路径。
5. 测试：
   - 覆盖 worker 循环单次 tick、企微汇报发送/跳过、任务列表 API、前端任务列表渲染所需类型。

## 3. 非目标

- 本轮不实现分布式调度锁以外的新队列系统。
- 本轮不实现 SSE/WebSocket。
- 本轮不新增复杂任务筛选、批量取消或重试 UI。
- 本轮不改变既有企业微信入站回调协议。

## 4. 验收标准

- API 进程启动后可周期性领取到期 `agent_scheduled_tasks`。
- 到期任务完成后，若目标通道为企业微信且 sender 可用，则向目标用户发送最终摘要。
- 汇报发送状态有 audit log 可追溯。
- Web `/agent` 可看到最近任务并进入对应 progress 页面。
- scheduled task progress 查询能展示 worker 创建的 controller run 与任务终态。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更严格的 capability scope 执行校验、计划步骤级重试/失败策略、Web 审批与任务控制面，以及 Agent 评测用例的自动运行。

## 6. 实施结果

**完成日期**：2026-06-25

- API 启动阶段新增 `AgentScheduledTaskWorkerService` 构造与后台循环装配。
- 新增 `runAgentScheduledTaskWorker`，按节点 ID 派生 worker ID，周期性调用 `RunDueOnce`，并记录 claimed、succeeded、failed、report sent/failed/skipped 指标日志。
- `AgentScheduledTaskWorkerService` 支持配置企微 sender；到期任务完成后按 `target_channel` / `target_ref` 发送最终摘要。
- scheduled task 汇报发送成功、失败或跳过均写入 `agent_audit_logs`，事件类型为 `agent.scheduled_task_report`。
- Web 新增 `GET /api/v1/agent/tasks` 最近任务列表 API，合并 plan 与 scheduled task 摘要。
- Web `/agent` 页面新增最近任务列表，支持进入 plan 进度页或通过 `scheduled_task_id` 查询调度任务进度。
- `AgentPlanView` 支持 `/agent?scheduled_task_id=...` 作为调度任务进度入口。
- 补充 worker 单次后台循环、企微汇报审计、任务列表 service/handler 测试。

## 7. 验证记录

- `go test ./...`
- `go vet ./...`
- `npm --prefix web run test`
- `npm --prefix web run type-check`
- `npm --prefix web run build`

以上命令均已通过。
