# Agent 实时进度推送与企微过程可见性计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent Web/企微入口、统一进度快照、定时任务 worker、企微最终汇报、scope 校验、Web 控制面、eval 安全基线和步骤重试已具备的基础上，补齐更细粒度的实时进度推送能力，使用户在 Web 端可以近实时看到 Agent 工作事件，在企微端也能收到关键阶段的过程通知。

## 2. 实施范围

1. Web 实时进度：
   - 新增 Agent progress event stream API，优先采用 SSE。
   - 支持按 `plan_id`、`turn_id`、`run_id`、`scheduled_task_id` 订阅。
   - 事件负载复用现有 progress snapshot 的 `event_cursor` 和 `recent_events` 语义，避免引入第二套进度模型。
2. 前端接入：
   - `/agent` 页面在支持 EventSource 的浏览器中使用实时流更新进度。
   - 保留现有 polling 作为降级路径。
   - 展示连接状态、最近事件游标和断线后的自动恢复状态。
3. 企微过程通知：
   - 在 plan 创建、审批等待、执行开始、步骤失败、步骤重试、调度完成等关键状态写入可发送的过程摘要。
   - 对企微入口任务，在不刷屏的前提下发送关键过程通知；Web 入口默认不外发企微过程通知。
   - 通知内容必须引用 plan/run/task id，且不得包含敏感配置、原始 token 或未脱敏上下文。
4. 可靠性与安全：
   - SSE 需要鉴权，且只能读取当前用户自己的 plan/run/task。
   - 企微过程通知需要基于现有 target user/ref，禁止向错误目标通道发送。
   - 失败发送写入 audit，不阻断 Agent 主流程。
5. 测试：
   - 覆盖 SSE 查询参数校验、事件编码、用户隔离、前端降级逻辑和企微过程通知审计。

## 3. 非目标

- 本轮不实现复杂多租户消息总线。
- 本轮不引入 WebSocket，除非 SSE 无法满足当前需求。
- 本轮不实现完整通知偏好设置页面。
- 本轮不把 eval 趋势报表做成长期指标看板。

## 4. 验收标准

- Web 端可通过实时流接收 Agent 进度事件，并在断线时降级为 polling。
- 企微入口任务在关键阶段能收到过程通知和最终汇报。
- SSE 和企微过程通知均具备用户隔离与目标通道保护。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进评测指标趋势、生产级 capability 扩展、任务重试 UI 完整化和 Agent 长程任务恢复。

## 6. 实施结果

1. Web 实时进度：
   - 新增 `GET /api/v1/agent/progress/stream` SSE 端点。
   - 支持 `plan_id`、`turn_id`、`run_id`、`scheduled_task_id` 查询参数。
   - SSE 事件复用现有 progress snapshot，发送 `progress`、`heartbeat` 和 `error` 事件，并使用 `event_cursor` 作为事件 id。
   - 初始快照在建立连接后立即发送，后续按游标变化推送。
2. 前端接入：
   - `/agent` 页面新增 EventSource 连接管理。
   - 支持实时连接、连接中、轮询降级、关闭等状态展示。
   - EventSource 不可用或连接失败时保留并启用既有 polling 降级路径。
   - 页面展示实时事件游标和连接错误摘要。
3. 企微过程通知：
   - 企微入口异步任务在计划开始、审批等待、失败等关键阶段发送过程通知。
   - Web 入口默认不外发企微过程通知。
   - 通知内容仅包含 plan id、状态、进度摘要、进度地址和脱敏失败摘要。
   - 发送结果写入 `agent.plan_started_feedback` 或 `agent.plan_progress_notification` audit，失败不阻断主流程。
4. 测试覆盖：
   - 覆盖 SSE 查询参数校验、SSE 响应格式、事件字段换行清理。
   - 覆盖前端 stream URL 构造。
   - 覆盖企微过程通知发送、审计记录和 Web 入口外发保护。

## 7. 验证记录

- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run test` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。
