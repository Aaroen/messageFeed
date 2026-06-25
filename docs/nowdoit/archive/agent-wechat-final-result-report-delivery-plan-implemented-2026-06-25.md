# Agent 企业微信最终结果汇报投递计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Web 进度地址真实投递完成后，继续补齐任务完成后的企业微信结果汇报闭环。用户从企业微信或 Web 发起 Agent 任务后，系统应在任务完成、失败或等待用户处理时，通过企业微信向用户汇报最终结果、状态、进度地址、关键证据和下一步动作。

## 2. 实施范围

1. 结果汇报发送路径：
   - 核实现有最终回复、计划完成回复、失败回复和调度任务汇报路径。
   - 将完成态结果汇报与进度通知区分，避免重复发送或缺失发送。
2. 企业微信投递内容：
   - 完成、失败、等待审批、等待补充信息均应包含状态、摘要、Web 进度地址和下一步。
   - 优先复用模板卡片能力；模板不可用时使用文本 fallback。
3. 审计与聚合：
   - 记录真实发送结果、消息 ID、fallback、错误和目标用户。
   - Web 任务工作台需要能区分“进度地址已投递”和“最终结果已汇报”。
4. 测试与验证：
   - 覆盖完成、失败、模板失败 fallback 和 Web 发起任务不误发企业微信的场景。
   - 完成完整验证矩阵。
5. 架构约束：
   - 结果汇报发送逻辑应放入职责明确的小文件或 helper。
   - 不继续扩大 `agent_workflow_governance.go`。
   - 对 `agent_conversation_service.go` 只做必要编排调用。

## 3. 当前已完成前置能力

- 企业微信进度通知已支持模板卡片优先、文本 fallback 保底。
- 进度通知审计已记录 `message_type`、`template_status`、`fallback_status`、错误字段和 `progress_url`。
- Web 工作台已展示 `wechat_web_progress_link`，并从真实审计读取进度地址投递状态。
- 完整验证矩阵已通过：
  - `go test ./...`
  - `go vet ./...`
  - `npm --prefix web run test`
  - `npm --prefix web run type-check`
  - `npm --prefix web run build`

## 4. 本轮实施清单

1. [x] 梳理最终结果汇报相关路径：最终回复、失败反馈和最终报告摘要 builder。
2. [x] 新增企业微信最终结果汇报 helper，区分进度通知与最终结果。
3. [x] 完成模板卡片加完整文本的最终结果投递；模板失败时文本仍可发送。
4. [x] 将最终结果汇报真实发送结果写入审计。
5. [x] 在任务聚合摘要中暴露最终结果汇报真实状态。
6. [x] 补充测试。
7. [x] 完成验证矩阵：
   - [x] `go test ./...`
   - [x] `go vet ./...`
   - [x] `npm --prefix web run test`
   - [x] `npm --prefix web run type-check`
   - [x] `npm --prefix web run build`
8. [ ] 记录实施结果和验证记录，归档本文档并创建下一轮文档。

## 4.1 实施结果

- 新增 `internal/service/agent_conversation_wechat_final_report_delivery.go`，将最终结果汇报封装为模板卡片入口加完整文本结果的组合投递。
- 主完成路径 `wechat_work.reply_sent` 已记录 `message_type`、`template_status`、`text_status`、错误字段和 `progress_url`。
- 失败反馈路径 `agent.turn_failure_feedback` 已记录同类最终汇报投递字段。
- `wechat_final_report` 聚合摘要已读取真实最终汇报审计，并暴露 `delivery_status`、`template_status`、`text_status` 和 `progress_url`。
- Web 任务工作台已展示企业微信最终汇报真实投递状态和进度地址链接。
- 已补充模板成功、模板失败文本 fallback、聚合摘要读取真实最终汇报审计的测试。

## 5. 非目标

- 本轮不引入新的外部通知服务。
- 本轮不绕过 Web 登录、企业微信身份绑定、审批和审计边界。
- 本轮不删除文本 fallback。
- 本轮不执行真实生产不可回滚操作。

## 6. 验收标准

- 企业微信用户可在任务完成后收到最终结果汇报。
- 最终结果汇报包含 Web 进度地址。
- 模板发送失败时文本 fallback 可用。
- 审计可证明发送结果、消息 ID、fallback 状态和目标用户。
- Web 任务工作台可核对最终结果汇报状态。
- 完整验证矩阵通过。
