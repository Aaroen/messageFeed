# Agent 企业微信模板 Web 证据路由回放审批与恢复持久化计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企业微信任务入口、实时进度、企业微信按钮控制、双端进度证据摘要以及双端交互治理摘要的基础上，推进企业微信进度卡片真实模板渲染、Web 证据明细路由、回调重放权限审批流和恢复策略配置持久化，使进度展示、证据访问、回放操作和恢复策略从摘要治理推进到可路由、可审批、可持久化的交互闭环。

## 2. 实施范围

1. 企业微信进度卡片模板渲染摘要：
   - 将企业微信进度卡片推进为模板渲染摘要。
   - 覆盖模板键、渲染状态、阶段字段、按钮字段、fallback 文本和发送入口。
2. Web 证据明细路由摘要：
   - 将 Web 证据交互推进为路由摘要。
   - 覆盖路由名、路径参数、筛选参数、权限要求、回放入口和审计展示。
3. 回调重放权限审批流摘要：
   - 将回调重放权限推进为审批流摘要。
   - 覆盖审批键、申请入口、审批角色、审批状态、执行门禁和审计事件。
4. 恢复策略配置持久化摘要：
   - 将恢复策略变更审计推进为持久化摘要。
   - 覆盖配置键、当前版本、待发布版本、持久化状态、回滚版本和审计证据。
5. 双端交互落地总览：
   - 汇总企业微信模板渲染、Web 证据路由、回放审批流和恢复策略持久化状态。

## 2.1 本轮实施清单

1. 后端任务聚合响应新增五类落地摘要：
   - 企业微信进度卡片模板渲染摘要。
   - Web 证据明细路由摘要。
   - 回调重放审批流摘要。
   - 恢复策略持久化摘要。
   - 双端交互落地总览。
2. 后端治理 builder 基于上一轮五类治理摘要继续派生本轮摘要：
   - 从企业微信进度卡片派生模板键、渲染状态、阶段字段、按钮字段、fallback 文本和发送入口。
   - 从 Web 证据交互派生路由名、路径参数、筛选参数、权限要求、回放入口和审计展示。
   - 从回调重放权限派生审批键、申请入口、审批角色、审批状态、执行门禁和审计事件。
   - 从恢复策略审计派生配置键、当前版本、待发布版本、持久化状态、回滚版本和审计证据。
   - 汇总企业微信模板、Web 证据路由、回放审批和恢复策略持久化状态。
3. `ListTasks` 接入本轮五类摘要，并写入对应审计快照事件。
4. 服务层测试补充本轮五类摘要字段和审计事件断言。
5. 前端 API 类型和 Agent 任务工作台展示本轮五类摘要。
6. 完成局部验证和完整验证矩阵：
   - `go test ./internal/service`
   - `npm --prefix web run type-check`
   - `go test ./...`
   - `go vet ./...`
   - `npm --prefix web run test`
   - `npm --prefix web run build`
7. 将实施结果和验证记录追加到本文档，随后归档本文档并创建下一轮唯一活动文档。

## 3. 非目标

- 本轮不开放未进入白名单的写 capability。
- 本轮不绕过审批、预算、权限、审计和回滚约束。
- 本轮不执行不可回滚的真实生产变更。
- 本轮不移除企业微信文本 fallback。

## 4. 验收标准

- 企业微信进度卡片模板渲染摘要可在 service 层和 Web 任务工作台查看，并写入审计事件。
- Web 证据明细路由摘要覆盖路由名、路径参数、筛选参数、权限要求、回放入口和审计展示。
- 回调重放权限审批流摘要覆盖申请、审批、执行门禁和审计事件。
- 恢复策略配置持久化摘要覆盖版本、持久化、回滚和审计证据。
- 双端交互落地总览纳入企业微信模板、Web 证据路由、回放审批和恢复策略持久化状态。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进真实企业微信模板发送适配、Web 证据路由页面实现、回放审批执行 API 和恢复策略配置版本管理。

## 6. 实施结果

1. 后端 `AgentTaskListResult` 新增五类落地摘要：
   - `wechat_template_render`
   - `web_evidence_route`
   - `callback_replay_approval`
   - `recovery_policy_persist`
   - `dual_end_interaction_launch`
2. 后端新增五类 response 结构，覆盖模板键、路由参数、审批门禁、恢复策略版本和双端落地状态。
3. 后端新增五个 builder，并在 `ListTasks` 中基于上一轮治理摘要继续派生本轮落地摘要。
4. 后端新增五个审计快照事件：
   - `agent.wechat_template_render_snapshot`
   - `agent.web_evidence_route_snapshot`
   - `agent.callback_replay_approval_snapshot`
   - `agent.recovery_policy_persist_snapshot`
   - `agent.dual_end_interaction_launch_snapshot`
5. 服务层测试已补充本轮五类摘要字段和审计事件断言。
6. 前端 `web/src/api/agent.ts` 已新增本轮五类摘要类型，并接入 `AgentTaskListResult`。
7. 前端 `web/src/views/AgentPlanView.vue` 已新增本轮五类摘要的状态、摘要计算、数据赋值和任务工作台展示。

## 7. 验证记录

1. `gofmt -w internal/service/agent_session_service.go internal/service/agent_workflow_governance.go internal/service/agent_progress_service_test.go`：通过。
2. `go test ./internal/service`：通过。
3. `npm --prefix web run type-check`：通过。
4. `go test ./...`：通过。
5. `go vet ./...`：通过。
6. `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
7. `npm --prefix web run build`：通过。
