# Agent 生产级能力、任务恢复与重试 UI 计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent Web/企微入口、统一进度、SSE 实时推送、企微过程通知、定时任务 worker、最终汇报、scope 校验、Web 控制面、eval 安全基线和最小步骤重试已具备的基础上，扩展可用于生产任务的 capability，并补齐长程任务恢复和更完整的重试 UI，使用户能够更可靠地发起、观察、恢复和重试 Agent 任务。

## 2. 实施范围

1. 生产级 capability 扩展：
   - 梳理当前 P0 capability 的输入输出和权限边界。
   - 增强订阅源查询、历史检索、内容总结、联网查询和调度任务的结果结构。
   - 对每个 capability 输出统一 evidence refs，便于 Web 与企微展示细节。
2. 长程任务恢复：
   - 定义可恢复 run/plan 的状态判定。
   - 增加最小恢复 service/API，将中断的 executing plan 或 running scheduled task 回收为可继续/可重试状态。
   - 对恢复操作写入 audit，并在 progress event 中可见。
3. 重试 UI 完整化：
   - Web 步骤列表展示 retry metadata、上次失败原因和可重试性。
   - 对 failed plan 提供集中重试入口，按可重试步骤批量排队。
   - 对 exhausted retry 显示明确状态，避免用户重复提交无效操作。
4. 企微与 Web 细节呈现：
   - 企微最终汇报补充 evidence refs 摘要。
   - Web run/observation/artifact 区域展示更可读的输入、输出和证据引用。
5. 测试：
   - 覆盖 capability evidence refs、恢复状态流转、批量重试、Web 类型检查和进度事件。

## 3. 非目标

- 本轮不实现多 agent 协作。
- 本轮不引入外部任务队列或消息总线。
- 本轮不实现复杂权限配置页面。
- 本轮不把所有 capability 扩展到生产最终形态，只完成最小可靠增强。

## 4. 验收标准

- 关键 capability 均能输出结构化 evidence refs。
- 中断 plan/run/task 可通过最小 API 恢复或转入可重试状态。
- Web 可集中查看失败原因、retry metadata，并批量重试可重试步骤。
- 企微最终汇报包含可读的证据摘要且不泄露敏感配置。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进 eval 指标趋势、通知偏好设置、更多生产级 capability 和多轮任务编排。

## 6. 实施结果

1. capability evidence refs：
   - executor 成功记录会在 artifact source refs 和 observation artifact refs 中写入 `capability:*`、`agent_run:*`、`agent_turn:*`、`agent_artifact:*`、`agent_observation:*` 等引用。
   - executor 失败记录会在 failed observation 中写入 capability/run/turn/observation 证据引用。
   - 已补充 service 测试验证成功与失败路径的 evidence refs 持久化。
2. 长程任务恢复：
   - 新增计划恢复 service/API：`POST /api/v1/agent/plans/:plan_id/recover`。
   - 新增定时任务恢复 service/API：`POST /api/v1/agent/scheduled-tasks/:id/recover`。
   - 恢复操作会回收 executing step 或 running/input_required/failed scheduled task，写入 audit，并通过进度快照中的状态更新时间可见。
3. 批量重试：
   - 新增计划级批量重试 service/API：`POST /api/v1/agent/plans/:plan_id/retry`。
   - 批量重试复用单步骤重试规则，统计 queued、skipped、exhausted，并写入 `agent.plan_retry_queued` audit。
4. Web 控制面：
   - Web Agent 页面新增 failed plan 集中重试入口、executing plan 恢复入口和 scheduled task 恢复入口。
   - 步骤列表展示上次重试时间、重试原因、上次失败原因、重试次数耗尽状态和证据引用。
   - run/observation/artifact 区域展示 observation refs 与 artifact source refs。
5. 企微最终汇报：
   - 定时任务最终汇报追加 task/plan/turn/run 证据引用摘要，避免暴露敏感配置。

## 7. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
