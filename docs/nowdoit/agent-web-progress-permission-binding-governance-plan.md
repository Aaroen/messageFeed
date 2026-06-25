# Agent Web 进度权限绑定与架构治理计划

**创建日期**：2026-06-25

## 1. 本轮目标

在企业微信进度地址投递和最终结果汇报闭环完成后，继续强化 Web 浏览器进度页的权限校验、企业微信身份绑定和代码结构治理。目标是确保用户从企业微信打开 Web 进度地址时，只能查看属于自己的 Agent 任务、证据、审批和结果，同时开始压缩 `AgentPlanView.vue` 与后端大文件的职责范围。

## 2. 实施范围

1. 权限与身份核对：
   - 核对 `/agent/plans/:id`、进度 API、证据 API 和审批 API 的 user_id 校验。
   - 核对企业微信 OAuth / external account / Web session 的绑定关系是否能支撑进度页访问。
   - 补齐缺失的权限测试。
2. Web 进度入口治理：
   - 明确企业微信中投递的 Web 进度地址在未登录、未绑定、绑定不一致时的行为。
   - 保证前端不通过任意 `user_id` 决定数据归属。
3. 架构治理：
   - 优先拆分 `AgentPlanView.vue` 中最终汇报、进度地址和任务摘要展示逻辑。
   - 后端新增逻辑继续放入小文件，不扩大 `agent_workflow_governance.go`。
4. 验证：
   - 后端权限测试、前端类型检查和构建必须通过。
   - 完成后同步主进度文档和设计文档。

## 3. 当前已完成前置能力

- Web 和企业微信均可触发 Agent 任务。
- Web 端可查看 Agent 进度、证据和审批信息。
- 企业微信已真实投递 Web 进度地址。
- 企业微信已在任务完成后发送最终结果汇报。
- Web 工作台已展示进度地址投递摘要和最终汇报真实状态。
- 最新完整验证矩阵已通过：
  - `go test ./...`
  - `go vet ./...`
  - `npm --prefix web run test`
  - `npm --prefix web run type-check`
  - `npm --prefix web run build`

## 4. 本轮实施清单

1. [x] 梳理 Agent 进度、证据、审批 API 的用户归属校验。
2. [x] 梳理企业微信 OAuth 和 external account 绑定对 Web 进度页访问的支持状态。
3. [x] 补齐权限测试，覆盖跨用户访问拒绝、未登录拒绝、OAuth state 归属绑定和 disabled binding 拒绝。
4. [ ] 拆分前端 Agent 工作台中进度地址和最终汇报摘要展示逻辑。
5. [x] 同步更新 `docs/implementation.md` 和 `docs/agent-plan.md`。
6. [x] 完成验证矩阵：
   - [x] `go test ./...`
   - [x] `go vet ./...`
   - [x] `npm --prefix web run test`
   - [x] `npm --prefix web run type-check`
   - [x] `npm --prefix web run build`
7. [ ] 记录实施结果和验证记录，归档本文档并创建下一轮文档。

## 4.1 阶段性实施结果

- 已确认 `AgentSessionService.GetProgress` 和 `GetPlanDetail` 使用 `auth.User.ID` 访问 repository。
- 已补充未登录访问进度拒绝测试。
- 已补充跨用户访问计划进度拒绝测试。
- 已补充跨用户访问计划详情拒绝测试。
- 已补充跨用户访问调度任务进度拒绝测试。
- 测试 fake repository 已增加可选 user scope 校验，能证明 service 将当前认证用户传入数据访问层。
- 已确认 `/api/v1/auth/wechat-work/oauth-url` 由 `requireAuth` 保护，OAuth URL 必须在 Web 登录态下生成。
- 已确认 OAuth callback 消费 state 后按 `state.UserID` 绑定企业微信 external account，并创建同一用户的 Web session。
- 已确认 `/api/v1/auth/me` 会返回当前登录用户的 external account bindings，可支撑前端展示绑定状态。
- 已确认企业微信消息入口解析 external account 时会拒绝 disabled binding。
- 已补充 `AuthService` 测试，覆盖未认证用户生成 OAuth URL 拒绝、OAuth callback 绑定 state 用户、当前用户 binding 返回和 disabled binding 解析拒绝。
- 绑定不一致场景的实际访问控制由两层共同承担：企业微信外部账号解析要求 active binding；Web 进度 URL 打开后的数据访问要求当前 Web session 用户与 Agent 任务 owner 一致。
- 最新定向验证已通过：`go test ./internal/service -run 'TestAuthService(WeChatWorkOAuth|ResolveExternalAccount|Me)'`。
- 本轮完整验证矩阵已通过：
  - `go test ./...`
  - `go vet ./...`
  - `npm --prefix web run test`
  - `npm --prefix web run type-check`
  - `npm --prefix web run build`

当前仍未完成：

- `AgentPlanView.vue` 摘要展示逻辑仍需拆分。

## 5. 非目标

- 本轮不重写完整登录系统。
- 本轮不引入新的外部身份提供方。
- 本轮不删除既有 Agent 工作台功能。
- 本轮不进行无测试保护的大规模重构。

## 6. 验收标准

- 进度页、证据页和审批页的用户归属校验有测试证明。
- 企业微信打开 Web 进度地址的身份绑定路径和未绑定行为有明确实现或缺口记录。
- Web 工作台摘要展示逻辑有可度量的拆分进展。
- 完整验证矩阵通过。
