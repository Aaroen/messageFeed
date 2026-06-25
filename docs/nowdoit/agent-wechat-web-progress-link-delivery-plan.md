# Agent 企业微信 Web 进度地址投递计划

**创建日期**：2026-06-25

**最近同步**：2026-06-25

## 1. 本轮目标

在真实交互自动化数据面和审计面完成后，推进企业微信向用户投递可在 Web 浏览器打开的 Agent 实时进度地址。用户从企业微信或 Web 发起任务后，应能通过企业微信消息中的 Web 地址查看实时进度、详细步骤、证据、回放和恢复状态；任务完成后继续通过企业微信汇报结果。

## 2. 实施范围

1. Web 进度地址生成：
   - 基于计划 ID、调度任务 ID 或会话上下文生成可打开的 Web 进度地址。
   - 地址生成逻辑保持相对路径配置，避免硬编码环境地址。
2. 企业微信消息投递：
   - 在企业微信任务反馈或模板消息中加入 Web 进度地址。
   - 保留文本 fallback，确保模板能力不可用时仍可收到地址。
3. 后端聚合摘要：
   - 在任务聚合结果中暴露 Web 进度地址投递状态。
   - 写入审计事件，记录地址生成、投递渠道、fallback 和下一步动作。
4. Web 工作台展示：
   - 显示企业微信 Web 进度地址投递状态。
   - 展示地址生成状态、投递状态、fallback 状态和审计引用。
5. 结构质量约束：
   - 新增逻辑放入职责明确的 service 文件。
   - 避免继续扩大 `agent_workflow_governance.go` 和 `AgentPlanView.vue` 的重复结构。

## 2.1 本轮实施清单

1. [x] 梳理现有任务进度 URL、企业微信 notifier 和任务聚合结果。
2. [x] 新增 Web 进度地址投递摘要 builder。
3. [x] `ListTasks` 接入地址投递摘要并写入审计快照。
4. [x] 服务层测试补充地址投递字段断言。
5. [x] 前端 API 类型和 Agent 任务工作台展示地址投递摘要。
6. [ ] 接入企业微信真实模板卡片或文本 fallback 的 Web 进度地址投递。
7. [ ] 完成验证矩阵：
   - `go test ./...`
   - `go vet ./...`
   - [x] `npm --prefix web run test`
   - [x] `npm --prefix web run type-check`
   - [x] `npm --prefix web run build`
8. [ ] 记录实施结果和验证记录，归档本文档并创建下一轮文档。

## 2.2 当前实现记录

已完成后端聚合摘要：

- 新增 `internal/service/agent_wechat_web_progress_link_governance.go`，通过任务进度 URL、企业微信模板发送摘要和真实交互自动化摘要构造 `AgentWeChatWebProgressLinkResponse`。
- `AgentTaskListResult` 已包含 `wechat_web_progress_link` 后端 JSON 字段。
- `ListTasks` 已构建 `wechatWebProgressLink` 并调用 `recordAgentWeChatWebProgressLinkSnapshot` 写入 `agent.wechat_web_progress_link_snapshot` 审计事件。
- `internal/service/agent_progress_service_test.go` 已断言进度地址、投递通道和浏览器目标。

已完成前端展示：

- `web/src/api/agent.ts` 已声明 `AgentWeChatWebProgressLink` 类型，并在 `AgentTaskListResult` 中暴露 `wechat_web_progress_link` 字段。
- `web/src/views/AgentPlanView.vue` 已读取 `wechat_web_progress_link`，并在任务工作台展示进度地址、投递通道、模板状态、fallback 状态、下一步和检查项。
- 进度地址在 Web 工作台中以链接形式展示，便于核对浏览器可打开地址。
- 已通过 `npm --prefix web run test`、`npm --prefix web run type-check` 和 `npm --prefix web run build`。

当前未完成项：

- 企业微信真实发送链路尚未保证模板卡片或文本 fallback 中包含可在 Web 浏览器打开的进度地址。
- 当前摘要不能替代真实投递证据，后续需要通过 `wechat_work.reply_sent` 或失败审计证明。

## 2.4 真实企业微信投递接入方案

本阶段将进度地址投递接入真实企业微信发送路径：

1. 扩展 Agent 对话服务的企业微信发送抽象，使其支持模板卡片发送。
2. 在 `sendPlanProgressNotification` 中优先发送模板卡片，卡片主动作指向 `agentPlanURL(plan.ID)`。
3. 模板卡片发送失败时保留文本 fallback，fallback 文本必须包含同一个 Web 进度地址。
4. 审计事件继续使用 `agent.plan_progress_notification` 或 `agent.plan_started_feedback`，但 metadata 需记录模板状态、fallback 状态、进度地址、企业微信消息 ID 和错误信息。
5. 测试需覆盖模板发送成功、模板失败后文本 fallback 成功，以及审计中包含进度地址。

本阶段不改变企业微信最终结果汇报逻辑；任务完成后的企业微信结果汇报将在模板进度地址真实投递完成后继续推进。

## 2.3 文档同步要求

本轮恢复任务前已同步更新：

- `docs/implementation.md`：记录当前总目标、已完成能力、缺口、架构风险和下一步。
- `docs/agent-plan.md`：追加当前实现对照、缺口和架构治理要求。
- 本活动文档：记录当前执行清单和后端已完成项。

## 3. 非目标

- 本轮不引入外部短链服务。
- 本轮不绕过 Web 登录、权限、审批、审计和回滚约束。
- 本轮不删除企业微信文本 fallback。
- 本轮不执行不可回滚的真实生产写操作。

## 4. 验收标准

- 后端能够生成并返回 Web 进度地址投递摘要。
- 企业微信投递状态覆盖地址生成、投递渠道、fallback 和审计引用。
- Web 任务工作台可查看地址投递状态。
- 完整验证矩阵通过。

## 5. 后续衔接

本轮完成后，继续推进企业微信真实模板消息适配、Web 浏览器进度页权限校验、实时刷新稳定性和任务完成后的企业微信结果汇报。

## 6. 架构约束

- 不继续扩大 `agent_workflow_governance.go` 的职责范围。
- 尽量不继续扩大 `agent_session_service.go`；短期兼容字段可保留，新增聚合逻辑应放入独立 governance 文件。
- 前端展示优先使用现有任务工作台模式补齐，后续需要拆分 `AgentPlanView.vue` 的大组件职责。
- 当前大文件规模不是理想终态，后续每轮应在实现功能的同时持续拆分职责。
