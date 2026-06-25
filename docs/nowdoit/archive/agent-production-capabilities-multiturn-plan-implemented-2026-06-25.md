# Agent 生产级能力与多轮任务编排计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 入口、计划审批、执行进度、SSE 推送、企微过程通知、恢复重试、证据引用、eval 趋势、通知偏好和汇报治理已具备的基础上，扩展更可用的生产级 capability，并增强多轮任务编排能力，使用户能够在 Web 或企业微信中围绕同一目标持续补充信息、观察执行过程并获得结构化结果。

## 2. 实施范围

1. 生产级 capability 扩展：
   - 增强订阅源查询能力，支持按来源、时间范围、关键词和已读状态组合过滤。
   - 增强历史对话检索能力，输出命中条目、时间范围、召回原因和 evidence refs。
   - 增强内容总结能力，支持多来源摘要、关键结论、风险提示和引用列表。
   - 增强受限联网查询输出结构，保留 URL、标题、抓取时间和摘要证据。
2. 多轮任务编排：
   - 将用户后续消息与 active plan/session 关联，支持对未完成任务追加约束、补充输入或要求停止。
   - 对 awaiting approval、executing、failed、completed 等状态定义多轮响应策略。
   - Web 与企微入口均能识别“继续/修改/停止/重试”类用户意图。
3. 任务状态治理：
   - 对长程 plan 增加更明确的 active/stale 判定。
   - 对已完成任务支持基于历史结果的追问，而不是误创建重复任务。
   - 对停止请求写入 audit，并在 progress event 中可见。
4. Web 与企微呈现：
   - Web 展示多轮任务上下文，包括原始目标、追加约束和最近用户指令。
   - 企微过程通知和最终汇报体现多轮变更摘要。
5. 测试：
   - 覆盖 capability 输出结构、多轮状态流转、停止请求、追问复用历史结果和 Web 类型检查。

## 3. 非目标

- 本轮不实现多 agent 协作。
- 本轮不引入外部队列或复杂工作流引擎。
- 本轮不实现跨用户任务共享。
- 本轮不开放任意写操作 capability。

## 4. 验收标准

- 关键查询/总结/联网 capability 输出结构化结果和证据引用。
- Web 与企微后续消息可关联到 active task，并能追加约束、停止或追问。
- 停止、追加输入和追问行为写入 audit，并在 progress event 中可见。
- 最终汇报包含多轮变更摘要和证据引用，且继续执行通知偏好与敏感字段过滤。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进长期记忆治理、复杂任务拆解质量评测、任务结果复用和更细粒度权限策略。

## 6. 实施结果

本轮已完成以下实现：

1. 生产级 capability 扩展：
   - `feed.query_recent_items` 与 `source.query_latest_items` 支持基于用户消息解析来源、时间范围、关键词和已读状态，并在 executor 层对候选条目进行组合过滤。
   - 订阅条目输出补充发布时间或抓取时间、已读状态、URL 和 `item:<id>` evidence ref。
   - `conversation.query_history` 输出补充召回原因和 `agent_transcript_entry:<id>` evidence refs。
   - `web.search`、`web.fetch_page`、`web.extract_page` 输出补充 URL、抓取时间、HTTP 信息和 URL evidence refs。
   - `content.summarize_text` 增加正式执行分支，输出关键结论、风险提示、引用列表和 evidence refs。
2. 多轮任务编排：
   - Web 与企业微信入口在创建 turn 后、创建新 plan 前识别后续消息意图。
   - 支持 `stop`、`append_constraints`、`retry`、`followup_question` 与 `new_task` 分类。
   - active plan 支持追加约束和停止；failed plan 支持记录重试请求；completed plan 支持追问复用历史结果。
   - 多轮上下文写入 `agent_plans.metadata_json.multi_turn`，保留原始目标、最近指令、追加约束、追问记录、重试请求和停止原因。
3. 任务状态治理：
   - 增加 active/stale 判定，默认 24 小时内任务可关联，failed plan 保留 72 小时重试窗口。
   - 停止请求将 plan 标记为 `failed`，错误信息为 `stopped by user`，并写入 `agent.plan_stopped` audit。
   - 执行结果绑定阶段会读取最新 plan 状态，避免已停止任务被旧执行结果覆盖为 completed。
4. Web 与企微呈现：
   - Web 计划详情页展示多轮上下文，包括原始目标、最近指令、追加约束、追问记录和停止原因。
   - 企微直接回复追加、停止、重试和追问处理结果，并附带计划进度 URL。
5. 测试覆盖：
   - 新增多轮追加约束、停止请求、completed plan 追问复用测试。
   - 更新 capability 上下文测试，使其验证结构化 evidence ref 输出。

## 7. 验证记录

以下命令已通过：

```bash
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```
