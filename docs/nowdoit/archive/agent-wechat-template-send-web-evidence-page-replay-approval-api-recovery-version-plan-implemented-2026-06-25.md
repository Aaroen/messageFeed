# Agent 企业微信模板发送 Web 证据页面回放审批 API 与恢复版本计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备企业微信模板渲染摘要、Web 证据路由摘要、回调重放审批摘要和恢复策略持久化摘要的基础上，推进真实企业微信模板发送适配、Web 证据详情页面、回调重放审批执行 API 和恢复策略版本管理，使双端交互从可观测摘要进一步进入可操作闭环。

## 2. 实施范围

1. 企业微信模板真实发送适配：
   - 在保留文本 fallback 的前提下，补充企业微信模板消息发送结构。
   - 覆盖模板消息类型、标题、阶段字段、按钮字段、fallback 文本、发送结果和审计记录。
2. Web 证据详情页面：
   - 基于现有 Agent 任务工作台和计划详情，补充证据详情页面或等价详情视图。
   - 覆盖计划参数、证据记录、筛选条件、审计事件、回放入口和权限提示。
3. 回调重放审批执行 API：
   - 在现有审批能力基础上补充回调重放申请和执行入口。
   - 覆盖申请、审批状态、执行门禁、幂等键、审计事件和失败 fallback。
4. 恢复策略版本管理：
   - 在现有恢复策略摘要基础上补充版本化配置模型或等价持久化路径。
   - 覆盖当前版本、待发布版本、回滚版本、发布状态和审计证据。
5. 双端真实交互总览：
   - 汇总企业微信模板发送、Web 证据详情、回放审批执行和恢复策略版本状态。

## 2.1 本轮实施清单

1. 后端补充企业微信模板发送适配摘要：
   - 在任务聚合结果中新增模板发送状态、消息类型、标题、字段清单、按钮清单、fallback、发送入口、发送结果和审计事件。
   - 保持现有企业微信文本 fallback，不移除原有 `SendText` 路径。
2. 后端补充 Web 证据详情视图摘要：
   - 在任务聚合结果中新增证据详情视图入口、计划参数、记录来源、筛选参数、审计事件、回放入口和权限提示。
3. 后端补充回调重放审批执行摘要和 API：
   - 新增回调重放审批申请与执行接口的服务层返回结构。
   - 路由接入受保护 API，执行结果必须包含审批状态、执行门禁、幂等键、审计事件和 fallback。
4. 后端补充恢复策略版本管理摘要：
   - 在任务聚合结果中新增恢复策略版本状态、当前版本、待发布版本、回滚版本、发布状态、配置来源和审计事件。
5. 后端补充双端真实交互总览：
   - 汇总企业微信模板发送、Web 证据详情、回调重放审批执行和恢复策略版本管理状态。
6. 服务层和 handler 测试覆盖新增摘要与 API 路由。
7. 前端同步 API 类型，并在 Agent 任务工作台展示本轮新增摘要。
8. 完成完整验证矩阵：
   - `go test ./...`
   - `go vet ./...`
   - `npm --prefix web run test`
   - `npm --prefix web run type-check`
   - `npm --prefix web run build`
9. 将实施结果与验证记录追加到本文档，归档本文档，并创建下一轮唯一活动文档。

## 3. 非目标

- 本轮不移除企业微信文本 fallback。
- 本轮不绕过既有认证、审批、权限、预算、审计和回滚约束。
- 本轮不开放未纳入策略白名单的写 capability。
- 本轮不执行真实生产不可回滚操作。

## 4. 验收标准

- 企业微信模板发送适配具备明确结构、发送入口、fallback 和审计证据。
- Web 端可查看证据详情的页面或等价视图，并包含审计和回放入口信息。
- 回调重放 API 具备申请、审批状态、执行门禁、幂等和审计约束。
- 恢复策略版本管理具备当前版本、待发布版本、回滚版本和发布状态。
- 双端真实交互总览可在 service 层和 Web 任务工作台查看。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进企业微信模板发送联调、Web 证据页面交互细节、回放执行安全审计和恢复策略灰度发布。

## 6. 实施结果

1. 企业微信模板发送适配：
   - `internal/notifier/wechat_work_app.go` 新增 `SendTemplateCard` 发送方法。
   - 新增企业微信 `template_card` 请求结构，保留原有 `SendText` 文本 fallback 路径。
   - 任务聚合新增 `wechat_template_send` 摘要，覆盖消息类型、标题、阶段字段、按钮字段、fallback、发送入口、发送结果和审计事件。
2. Web 证据详情视图：
   - 前端新增 `/agent/plans/:id/evidence/:recordKey` 路由，复用现有 Agent 视图作为证据详情入口。
   - 任务聚合新增 `web_evidence_detail_view` 摘要，覆盖路由、计划参数、记录参数、记录来源、筛选参数、审计事件、回放入口和权限提示。
3. 回调重放审批执行 API：
   - 后端新增受保护接口 `POST /api/v1/agent/callback-replay/requests`。
   - 后端新增受保护接口 `POST /api/v1/agent/callback-replay/execute`。
   - 服务层新增审批申请和执行门禁逻辑，执行接口在未批准时返回 blocked，并写入审计事件。
   - 任务聚合新增 `callback_replay_execution` 摘要，覆盖申请入口、执行入口、审批状态、执行门禁、幂等键、审计事件和失败 fallback。
4. 恢复策略版本管理：
   - 任务聚合新增 `recovery_policy_version` 摘要，覆盖策略键、当前版本、待发布版本、回滚版本、发布状态、配置来源和审计事件。
5. 双端真实交互总览：
   - 任务聚合新增 `dual_end_real_interaction` 摘要，汇总企业微信模板发送、Web 证据详情、回调重放执行和恢复策略版本状态。
6. 前端 `web/src/api/agent.ts` 已同步新增类型和回调重放 API 调用函数。
7. 前端 `web/src/views/AgentPlanView.vue` 已新增本轮五类摘要展示。
8. 测试覆盖：
   - notifier 测试覆盖企业微信模板卡片请求体。
   - handler 测试覆盖回调重放执行路由。
   - service 测试覆盖新增摘要、审计事件、回调重放申请与执行门禁。

## 7. 验证记录

1. `gofmt -w internal/service/agent_session_service.go internal/service/agent_workflow_governance.go internal/service/agent_progress_service_test.go internal/handler/agent_session_handler.go internal/handler/agent_progress_handler_test.go internal/notifier/wechat_work_app.go internal/notifier/wechat_work_app_test.go`：通过。
2. `go test ./internal/service ./internal/handler ./internal/notifier`：通过。
3. `npm --prefix web run type-check`：通过。
4. `go test ./...`：通过。
5. `go vet ./...`：通过。
6. `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
7. `npm --prefix web run build`：通过。
