# Agent schedule_task 能力与调度执行闭环计划

**创建日期**：2026-06-25

## 1. 本轮目标

基于已完成的 `agent_scheduled_tasks` 数据基线，将旧的 `agent.schedule_message` 升级为更通用的 `agent.schedule_task` capability，并建立到点后创建 controller `AgentRun` 的最小执行闭环。

## 2. 实施范围

1. Capability registry：
   - 注册 `agent.schedule_task`。
   - 保留 `agent.schedule_message` 的兼容处理，但新系统提示词和 planner 优先使用 `agent.schedule_task`。
2. Tool executor：
   - 将用户确认后的定时任务写入 `agent_scheduled_tasks`。
   - 定时任务 payload 保存目标、允许能力、模型策略、新鲜度策略和失败策略。
   - 对未确认任务返回 `requires_confirmation` observation。
3. Planner 与 policy：
   - 让提醒、定时汇报、定时检索、定时总结类目标选择 `agent.schedule_task`。
   - 确认策略继续走 `prompt`，审批通过后只允许执行获批 scope。
4. Scheduler service：
   - 领取到期 `agent_scheduled_tasks`。
   - 为到期任务创建 controller run 所需 task packet。
   - 当前轮仅实现 service 层最小闭环，不强制接入后台常驻 worker。
5. 进度可见性：
   - 在 observation / audit 中写入调度任务 ID、状态和下一步。
   - 为后续 Web/企微实时进度展示保留稳定字段。
6. 测试：
   - 覆盖 registry、planner、tool executor、service 领取到期任务和兼容旧工具的行为。

## 3. 非目标

- 本轮不实现完整 Web 实时进度 UI。
- 本轮不实现 WebSocket 或 SSE。
- 本轮不实现生产级后台调度守护进程。
- 本轮不移除 `agent.schedule_message` 兼容入口。

## 4. 验收标准

- `agent.schedule_task` 出现在 capability registry 和模型工具提示中。
- 定时任务类用户目标生成包含 `agent.schedule_task` 的 plan。
- 未确认时不会写入调度任务；确认后写入 `agent_scheduled_tasks`。
- 调度 service 能领取到期任务并生成 controller run task packet。
- `go test ./...`、`go vet ./...`、`npm --prefix web run type-check` 通过。

## 5. 后续衔接

本轮完成后，下一轮推进 Web/企业微信 Agent 实时进度数据面：查询 scheduled task、run、plan step、observation、artifact 的统一进度视图，并为 Web 轮询或 SSE 做接口准备。

## 6. 完成记录

**完成日期**：2026-06-25

已完成：

- `AgentCapabilityRegistry` 新增 `agent.schedule_task`，保留 `agent.schedule_message` 兼容入口。
- planner 对提醒、定时、日报、周报等目标选择 `agent.schedule_task`，并按策略进入审批。
- TurnRunner 工具提示和工具白名单已优先使用 `agent.schedule_task`。
- executor 新增 `agent.schedule_task` 执行路径，确认后写入 `agent_scheduled_tasks`。
- 旧 `agent.schedule_message` 在存在 scheduled task store 时复用新写入路径，否则保留旧 notification job 兜底。
- `AgentScheduleEvalService` 新增 controller run task packet 构造方法。
- 补充 registry、planner、executor、service 和旧兼容路径测试。

验证命令：

```text
go test ./...
go vet ./...
npm --prefix web run type-check
```

验证结果均通过。
