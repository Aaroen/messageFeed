# Agent 准实时进度更新实施计划

**创建日期**：2026-06-25

## 1. 本轮目标

在统一 `AgentProgressSnapshot` 数据面基础上，推进 Web 与企业微信的准实时进度体验：Web 端使用受控轮询和状态变更节流，企业微信可在关键阶段向用户发送进度摘要，executor observation 逐步细化为可展示事件。

## 2. 实施范围

1. Web 轮询策略：
   - 根据 progress status 动态调整轮询间隔。
   - 终态停止轮询。
   - 刷新失败时保留上一份快照并显示非阻塞错误。
2. Progress snapshot 增量字段：
   - 增加 `version` 或 `event_cursor`，便于前端判断是否发生变化。
   - 增加 `updated_at`，作为轮询节流和后续 SSE 的基础。
3. 企业微信进度摘要：
   - 基于 `AgentProgressSnapshot` 生成短文本摘要。
   - 在计划等待确认、开始执行、失败、完成等关键阶段复用同一摘要逻辑。
4. Observation 事件化：
   - executor 写入 observation 时补充稳定事件标题、状态、摘要和引用。
   - progress recent events 优先使用 observation / artifact 的可展示字段。
5. 测试：
   - 覆盖轮询间隔计算、快照版本字段、企微进度摘要和 observation 事件映射。

## 3. 非目标

- 本轮不实现 SSE 或 WebSocket。
- 本轮不改变现有审批状态机。
- 本轮不新增外部通知通道。

## 4. 验收标准

- Web 根据任务状态动态轮询，终态停止。
- progress snapshot 暴露可用于增量判断的版本或 cursor 字段。
- 企业微信关键消息可以复用 progress 摘要文本。
- recent events 对 observation / artifact 展示更清晰。
- `go test ./...`、`go vet ./...`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，再进入可选 SSE/WebSocket、后台 scheduler 常驻 worker、Web 发起 Agent 任务入口和企业微信最终结果主动汇报增强。

## 6. 完成记录

**完成日期**：2026-06-25

已完成：

- `AgentProgressSnapshot` 新增 `version`、`event_cursor` 和 `updated_at`。
- progress recent events 对 observation / artifact 增加稳定标题、摘要兜底和引用字段。
- 新增 `AgentProgressTextSummary`，企业微信计划确认、开始执行和完成/失败消息复用同一进度摘要。
- Web `AgentPlanView` 支持按状态动态轮询，终态停止轮询。
- Web 静默刷新失败时保留已有快照，并显示非阻塞刷新提示。
- 新增 `web/src/utils/agentProgress.ts` 和单元测试，覆盖轮询间隔与终态判断。
- 补充后端测试覆盖 snapshot 增量字段、observation 事件映射和企微进度摘要。

验证命令：

```text
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```

验证结果均通过。
