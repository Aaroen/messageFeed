# Agent 企业微信模板联调证据交互回放审计与恢复灰度计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备企业微信模板发送适配、Web 证据详情入口、回调重放审批执行 API 和恢复策略版本摘要的基础上，推进企业微信模板发送联调状态、Web 证据页面交互细节、回放执行安全审计和恢复策略灰度发布，使双端真实交互继续接近可运行闭环。

## 2. 实施范围

1. 企业微信模板发送联调状态：
   - 汇总模板卡片发送路径、文本 fallback、发送失败降级、消息 ID 回读和审计证据。
2. Web 证据页面交互细节：
   - 补充证据详情入口中的筛选、展开、审计时间线、回放申请入口和权限提示。
3. 回放执行安全审计：
   - 补充回调重放执行中的幂等校验、审批校验、签名校验、执行结果和失败审计。
4. 恢复策略灰度发布：
   - 补充恢复策略版本的灰度阶段、发布比例、回滚条件、审批状态和审计证据。
5. 双端运行闭环总览：
   - 汇总企业微信模板联调、Web 证据交互、回放安全审计和恢复策略灰度状态。

## 2.1 本轮实施清单

1. 后端任务聚合结果新增五类运行闭环摘要：
   - 企业微信模板联调状态。
   - Web 证据交互细节。
   - 回放执行安全审计。
   - 恢复策略灰度发布。
   - 双端运行闭环总览。
2. 后端 builder 基于上一轮真实交互摘要继续派生本轮摘要：
   - 基于 `wechat_template_send` 派生发送路径、fallback、降级策略、消息 ID 回读和审计证据。
   - 基于 `web_evidence_detail_view` 派生筛选、展开、审计时间线、回放申请入口和权限提示。
   - 基于 `callback_replay_execution` 派生幂等、审批、签名、执行结果和失败审计。
   - 基于 `recovery_policy_version` 派生灰度阶段、发布比例、回滚条件、审批状态和审计证据。
   - 汇总企业微信模板联调、Web 证据交互、回放安全审计和恢复灰度状态。
3. `ListTasks` 接入本轮五类摘要，并写入对应审计快照事件。
4. 服务层测试补充本轮新增字段和审计事件断言。
5. 前端 API 类型和 Agent 任务工作台展示本轮新增摘要。
6. 完成完整验证矩阵：
   - `go test ./...`
   - `go vet ./...`
   - `npm --prefix web run test`
   - `npm --prefix web run type-check`
   - `npm --prefix web run build`
7. 将实施结果和验证记录追加到本文档，随后归档本文档并创建下一轮唯一活动文档。

## 3. 非目标

- 本轮不移除企业微信文本 fallback。
- 本轮不绕过认证、审批、权限、预算、审计和回滚约束。
- 本轮不执行不可回滚的真实生产操作。
- 本轮不开放未纳入策略白名单的写 capability。

## 4. 验收标准

- 企业微信模板联调状态可在 service 层和 Web 任务工作台查看。
- Web 证据交互细节覆盖筛选、展开、审计时间线、回放入口和权限提示。
- 回放执行安全审计覆盖幂等、审批、签名、执行结果和失败 fallback。
- 恢复策略灰度发布覆盖阶段、比例、回滚条件、审批状态和审计证据。
- 双端运行闭环总览纳入企业微信模板、Web 证据、回放审计和恢复灰度状态。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进真实环境企业微信模板发送试运行、证据页面用户操作、回放执行结果留痕和恢复策略灰度自动化。

## 6. 实施结果

1. 后端 `AgentTaskListResult` 新增五类运行闭环摘要：
   - `wechat_template_integration`
   - `web_evidence_interaction_detail`
   - `callback_replay_safety_audit`
   - `recovery_policy_gray_release`
   - `dual_end_run_loop`
2. 后端新增五类 response 结构，覆盖企业微信模板发送路径、Web 证据交互、回放安全审计、恢复策略灰度和双端运行闭环。
3. 后端新增五个 builder：
   - `buildAgentWeChatTemplateIntegration`
   - `buildAgentWebEvidenceInteractionDetail`
   - `buildAgentCallbackReplaySafetyAudit`
   - `buildAgentRecoveryPolicyGrayRelease`
   - `buildAgentDualEndRunLoop`
4. `ListTasks` 已基于上一轮真实交互摘要继续派生本轮五类运行闭环摘要。
5. 后端新增五个审计快照事件：
   - `agent.wechat_template_integration_snapshot`
   - `agent.web_evidence_interaction_detail_snapshot`
   - `agent.callback_replay_safety_audit_snapshot`
   - `agent.recovery_policy_gray_release_snapshot`
   - `agent.dual_end_run_loop_snapshot`
6. 服务层测试已补充本轮五类摘要字段和审计事件断言。
7. 前端 `web/src/api/agent.ts` 已新增本轮五类摘要类型，并接入 `AgentTaskListResult`。
8. 前端 `web/src/views/AgentPlanView.vue` 已新增本轮五类摘要的状态、摘要计算、数据赋值和任务工作台展示。

## 7. 验证记录

1. `gofmt -w internal/service/agent_session_service.go internal/service/agent_workflow_governance.go internal/service/agent_progress_service_test.go`：通过。
2. `go test ./internal/service`：通过。
3. `npm --prefix web run type-check`：通过。
4. `go test ./...`：通过。
5. `go vet ./...`：通过。
6. `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
7. `npm --prefix web run build`：通过。
