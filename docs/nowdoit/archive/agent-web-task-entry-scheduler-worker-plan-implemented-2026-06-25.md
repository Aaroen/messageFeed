# Agent Web 任务入口与调度 worker 实施计划

**创建日期**：2026-06-25

## 1. 本轮目标

在已有企业微信入口、`agent.schedule_task`、`agent_scheduled_tasks` 和统一进度快照基础上，补齐 Web 发起 Agent 任务入口和后台调度 worker 的最小闭环，使用户可以从 Web 创建任务，并让到点调度任务复用 controller run 链路。

## 2. 实施范围

1. Web Agent 任务入口 API：
   - 新增 `POST /api/v1/agent/tasks`。
   - 请求包含 `message`、可选 `session_id`、可选 `channel`。
   - 后端创建或复用 Web session / turn，并进入现有 controller -> plan -> approval / execution 链路。
2. Web 页面入口：
   - 在 Agent 相关页面提供最小任务输入框。
   - 提交后展示 plan/progress 地址并进入进度视图。
3. 调度 worker service：
   - 领取到期 `agent_scheduled_tasks`。
   - 为任务创建 controller run task packet。
   - 将任务状态从 queued -> running -> succeeded/failed 回写。
   - 当前轮优先实现 service 可调用闭环，不强制常驻 goroutine。
4. 企微最终汇报准备：
   - worker 完成后生成可用于企微投递的最终摘要文本。
   - 后续接入真实发送策略。
5. 测试：
   - 覆盖 Web task API 参数校验、service 创建任务、调度 worker 状态流转和进度快照衔接。

## 3. 非目标

- 本轮不实现 SSE/WebSocket。
- 本轮不实现复杂多用户任务队列 UI。
- 本轮不新增外部通知通道。
- 本轮不要求调度 worker 常驻运行。

## 4. 验收标准

- Web API 可以发起 Agent 任务并返回 plan/progress 可追踪结果。
- Web 页面可以提交任务并跳转或展示进度。
- 调度 worker service 能领取到期任务并回写状态。
- 调度执行结果可通过 progress snapshot 查询。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进后台常驻 worker 装配、企业微信最终结果主动推送、Web 任务列表和更完整的 capability scope 执行闭环。

## 6. 实施结果

**完成日期**：2026-06-25

- 新增 `POST /api/v1/agent/tasks`，Web 用户可提交 `message`、可选 `session_id`、可选 `channel` 发起 Agent 任务。
- `AgentConversationService` 新增 Web 入口，创建或复用 provider=`web` 的 external account 和 session，并复用现有 controller -> plan -> approval / execution 链路。
- API 返回 session、turn、plan、reply 和 progress URL，前端可据此进入进度页。
- `cmd/api` 中的 Agent conversation service 不再依赖企微回调配置才创建，Web 入口可独立使用。
- Web 新增 `/agent` 入口，`AgentPlanView` 顶部提供最小任务输入框，提交后跳转到 `/agent/plans/:id`。
- 新增 `AgentScheduledTaskWorkerService.RunDueOnce`，可领取到期 `agent_scheduled_tasks`，创建 controller run task packet，写入 controller run，并将任务状态回写为 succeeded/failed。
- scheduled task 更新路径补充 `source_run_id` 回写，便于进度快照通过 scheduled task 关联 controller run。
- worker 生成可用于后续企微投递的最终摘要文本。

## 7. 验证记录

- `go test ./...`
- `go vet ./...`
- `npm --prefix web run test`
- `npm --prefix web run type-check`
- `npm --prefix web run build`

以上命令均已通过。
