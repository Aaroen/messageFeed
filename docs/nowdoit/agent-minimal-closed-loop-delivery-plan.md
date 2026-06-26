# Agent Minimal Closed Loop Delivery Plan

## 1. 本轮目标

本轮先交付一个可验证的最小 Agent 闭环，后续复杂能力增强延后。闭环定义为：用户从 Web 或企业微信发起任务后，系统创建任务、生成计划和进度地址、执行至少一个可验证能力、Web 可查看进度，任务完成后通过企业微信发送最终结果，并写入审计证据。

## 2. 当前依据

1. 企业微信入口已经可以接收消息并进入 `AgentConversationService.ReceiveWeChatWorkAppMessage`。
2. Web 入口已经可以通过 `/api/v1/agent/tasks` 进入 `AgentConversationService.ReceiveWebAgentTask`。
3. 后端已有计划、run、step、observation、audit、progress URL 和进度查询能力。
4. 企业微信发起任务已具备最终报告发送能力。
5. 当前主要缺口是 Web 发起任务完成后不会主动向用户已绑定的企业微信账号发送最终报告。

## 3. 实施范围

1. 在 `AgentConversationRepository` 增加读取用户外部账号的能力边界，用于查找当前用户已绑定且启用的企业微信账号。
2. Web 发起任务完成后，如果存在可用企业微信绑定，则复用已有最终报告投递能力发送企业微信结果。
3. 写入 Web 任务最终报告投递审计，记录是否找到企业微信绑定、发送状态、进度地址和投递模式。
4. 保持企业微信发起任务现有行为不变。
5. 补充服务层测试，覆盖 Web 发起任务完成后向企业微信发送最终报告，并校验进度地址和审计记录。

## 4. 非目标

- 本轮不扩展完整能力注册体系。
- 本轮不引入 WebSocket。
- 本轮不重构前端大组件。
- 本轮不继续推进 recorder 4.8 迁移。
- 本轮不要求无绑定 Web 用户也强制发送企业微信消息。

## 5. 验收标准

1. Web 发起任务可返回 `plan` 和 `progress_url`。
2. Web 发起任务完成后，如用户已有启用的企业微信绑定，应发送最终报告到企业微信。
3. 企业微信最终报告应包含 Web 浏览器可打开的进度地址。
4. 审计日志应记录 Web 任务最终报告投递结果。
5. `go test ./...` 和 `go vet ./...` 通过。
6. `docs/implementation.md` 和 `docs/agent-plan.md` 同步记录本轮实际结果。

## 6. 实施结果

1. 已新增 `internal/service/agent_conversation_web_final_report_delivery.go`，承接 Web 发起任务完成后的企业微信最终报告投递逻辑。
2. 已在 `ReceiveWebAgentTask` 完成处理后调用 Web 最终报告投递 helper；主流程只增加调用点，保持投递职责在独立文件内。
3. 已扩展 `AgentConversationRepository` 能力边界，读取用户外部账号并选择启用的企业微信绑定作为最终报告目标。
4. 已保持企业微信发起任务既有最终报告投递路径不变。
5. 已修正无企业微信发送时的完成审计事件，避免 Web 任务误写空的 `wechat_work.reply_sent`；只有真实企业微信投递才写该事件。
6. 已扩展 `TestAgentConversationServiceReceivesWebAgentTask`，覆盖 Web 发起任务、计划生成、进度地址、企业微信最终报告模板与文本投递、审计 metadata。
7. 已通过：
   - `go test ./...`
   - `go vet ./...`
