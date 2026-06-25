# Agent 长期记忆治理与结果复用计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企微任务入口、执行进度、审批控制、恢复重试、证据引用、通知偏好、生产级 capability 和多轮任务编排的基础上，推进长期记忆治理与任务结果复用能力，使用户围绕历史任务继续追问、复用结论或派生新任务时，系统能够明确区分可复用结果、过期结果、敏感结果和需要重新执行的任务。

## 2. 实施范围

1. 长期记忆治理：
   - 为 agent transcript、recall event、plan result 和 artifact 建立更明确的可复用元数据。
   - 区分短期会话上下文、长期历史记忆、任务结果记忆和外部证据。
   - 对敏感字段、失败原因、审批原因和通知内容继续沿用脱敏策略。
2. 任务结果复用：
   - completed plan 的追问优先复用历史结果、artifact refs 和 evidence refs。
   - 支持用户基于历史 plan 派生新任务，并在 metadata 中记录 parent plan。
   - 对过期结果给出重新执行建议，而不是直接复用旧结论。
3. 结果新鲜度治理：
   - 为不同 capability 输出增加 freshness hint。
   - Web 与企微回复中体现结果时间、来源时间和是否建议刷新。
   - 对联网结果、订阅源结果和历史对话结果采用不同 stale 判定。
4. 权限与审计：
   - 对结果复用、派生任务、刷新请求和记忆召回写入 audit。
   - 对跨 session 复用进行同用户边界校验。
   - 保持本轮不开放任意写操作 capability。
5. Web 呈现：
   - 在计划详情页展示结果复用来源、parent plan、freshness 状态和 evidence refs 摘要。
   - 在任务入口中区分“继续追问”和“基于此创建新任务”的后端语义。

## 3. 非目标

- 本轮不实现跨用户共享记忆。
- 本轮不引入向量数据库或外部检索服务。
- 本轮不实现多 agent 协作。
- 本轮不开放任意文件写入、订阅修改或权限提升 capability。

## 4. 验收标准

- completed plan 追问可复用历史结果，并能说明复用来源、证据引用和新鲜度。
- 基于历史 plan 创建新任务时，metadata 记录 parent plan 和派生原因。
- stale 结果不会被当作新事实直接回复，系统会提示重新执行或刷新。
- 结果复用、派生、刷新和记忆召回行为写入 audit，并可在 progress recent events 中观察。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进复杂任务拆解质量评测、细粒度 capability 权限策略、任务预算治理和更完整的 Web 端 Agent 工作台。

## 6. 实施结果

本轮已完成以下实现：

1. 长期记忆治理：
   - completed plan 追问写入 `metadata.multi_turn.result_reuse`，记录来源 plan、来源 session、来源 turn、记忆类型、证据引用和复用时间。
   - `metadata.memory_governance` 区分短期会话上下文、长期历史记忆、任务结果记忆和外部证据，并保留脱敏策略说明。
   - 历史对话召回事件写入 `memory_scope=long_term_conversation`、`reusable=true` 和 transcript evidence refs。
2. 任务结果复用：
   - completed plan 追问优先复用历史结果，并在企微/Web 回复中说明来源 plan、结果新鲜度和 evidence refs。
   - stale completed plan 不再直接复述旧结论，而是提示用户刷新或重新创建任务。
   - 支持“基于刚才/上一个结果创建任务”类派生请求，新 plan 的 metadata 写入 `parent_plan` 和 `result_reuse`。
3. 结果新鲜度治理：
   - 对联网、订阅源、历史对话和普通任务结果设置不同 freshness 策略。
   - capability 输出补充 freshness hint：联网结果 6 小时后建议刷新，订阅源结果 12 小时后建议刷新，历史对话结果 30 天内可作为同用户会话记忆引用。
   - Web/企微追问回复体现 freshness status、freshness hint 和是否建议刷新。
4. 权限与审计：
   - 结果复用写入 `agent.plan_result_reused` audit。
   - stale 结果命中写入 `agent.plan_result_stale` audit。
   - 派生任务写入 `agent.plan_derived` audit。
   - 继续沿用同用户、同 session 的 plan 查询边界，不开放任意写操作 capability。
5. Web 呈现：
   - 计划详情页展示 parent plan、结果新鲜度、freshness hint 和复用证据引用。
   - 多轮上下文区域同时展示追问记录、追加约束和复用来源。
6. 测试覆盖：
   - 新增 completed plan 结果复用测试。
   - 新增 stale completed plan 不直接复用旧事实测试。
   - 新增派生任务 parent plan metadata 测试。

## 7. 验证记录

以下命令已通过：

```bash
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```
