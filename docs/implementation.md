# messageFeed 实施进度台账

**最后更新**：2026-06-25

本文件是当前实现进度的主台账。每轮迭代除更新 `docs/nowdoit` 活动文档外，必须同步更新本文档以及相关设计文档，例如 `docs/agent-plan.md`。历史已完成活动文档保留在 `docs/nowdoit/archive/`。

## 1. 当前目标

最终目标保持为统一的 `messageFeed AI Agent`：

1. 用户可以从企业微信或 Web 发起 Agent 任务。
2. Web 端可以查看 Agent 实时进度、计划步骤、执行细节、证据、审批、回放和恢复状态。
3. 企业微信侧应向用户投递可在 Web 浏览器打开的进度地址。
4. 任务完成后，系统通过企业微信向用户汇报结果。
5. Agent 运行时、能力注册、策略审批、上下文记忆、调度、评测、审计和通知必须保持可追溯。
6. 代码结构必须维持清晰职责边界；大文件继续拆分治理，避免把新能力堆叠到既有长文件中。

## 2. 当前仓库状态

| 项目 | 当前状态 |
| --- | --- |
| 分支 | `master` |
| 工作区 | 最近核对为干净，且已与 `origin/master` 同步 |
| 当前活动文档 | `docs/nowdoit/agent-wechat-final-result-report-delivery-plan.md` |
| 最近全量验证 | `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 已在上一轮通过 |
| 最近提交 | `ac20538`、`97ba0f7`、`bd9fcc1`、`d629dc5`、`4bd3063` |

## 3. 已完成能力

### 3.1 Agent 基础运行时

- 已具备 Agent session、turn、run、plan、approval、scheduled task、eval、recovery、audit 等基础对象和服务面。
- Web 侧已有 Agent 任务入口、任务工作台、计划进度页、审批页、证据与回放相关视图。
- 后端已形成任务聚合接口，用于向 Web 工作台提供任务、SLA、成本、告警、部署验证、企业微信联动、进度证据、恢复策略和评测摘要。
- 已具备任务进度 URL 字段，并在 Web 发起任务后可跳转进度页。

### 3.2 企业微信接入与交互

- 已具备企业微信自建应用 callback、OAuth、文本消息发送和模板卡片发送基础能力。
- 已具备企业微信审批按钮、回放、恢复、模板卡片、灰度、签收、最终报告、反馈闭环等多项治理摘要。
- 已完成真实交互自动化摘要 `real_interaction_automation` 的后端和前端展示。
- 已完成 `wechat_web_progress_link` 后端聚合摘要和审计快照，字段覆盖进度地址、地址来源、投递通道、模板状态、fallback 状态、浏览器目标和审计引用。
- 已接入企业微信进度通知真实投递：模板卡片优先，文本 fallback 保底，卡片和 fallback 均包含 Web 浏览器进度地址。
- `wechat_web_progress_link` 聚合摘要已读取真实 `agent.plan_progress_notification` 或 `agent.plan_started_feedback` 审计事件中的进度地址、模板状态和 fallback 状态。

### 3.3 Web 进度与治理展示

- Web 任务工作台已展示多数 Agent 运行、企业微信、审批、回放、恢复和真实交互自动化摘要。
- Web 任务工作台已展示 `wechat_web_progress_link` 摘要，包括进度地址、投递通道、模板状态、fallback 状态、浏览器目标和检查项。
- Web 进度页已支持计划 ID 或调度任务 ID 维度的进度查询、轮询、步骤、证据和审批状态展示。
- 前端已有任务创建表单，并通过 `createAgentTask` 以 `channel=web` 发起任务。

### 3.4 质量与验证

- 后端已有 `agent_progress_service_test.go` 对 `WeChatWebProgressLink` 的返回字段进行断言。
- 最近一轮全量验证已覆盖 Go 测试、Go vet、前端测试、类型检查和前端构建。
- 当前新增治理文件将部分逻辑从大文件中抽离，包括：
  - `internal/service/agent_real_interaction_automation_governance.go`
  - `internal/service/agent_wechat_web_progress_link_governance.go`
  - `internal/service/agent_dual_end_run_loop_governance.go`
  - `internal/service/agent_dual_end_task_closure_governance.go`
  - `internal/service/agent_governance_checks.go`

## 4. 当前未完成缺口

| 优先级 | 缺口 | 当前判断 |
| --- | --- | --- |
| P0 | 任务完成后的企业微信最终结果汇报需要继续核实真实发送路径 | 已有最终报告摘要，但仍需以真实发送证据闭环 |
| P1 | Web 浏览器进度地址权限校验与企业微信身份绑定仍需继续强化 | 已有 OAuth 与审批基础，需与进度地址投递闭环结合 |
| P1 | Agent 能力注册、上下文记忆、计划执行和评测体系仍需按设计持续补齐 | 已有较多基础对象，但未能证明全部设计均已完整实现 |
| P1 | 大文件职责边界仍不理想 | 需要继续拆分 `agent_session_service.go`、`agent_workflow_governance.go`、`AgentPlanView.vue` |

## 5. 架构质量核对

当前关键大文件规模：

| 文件 | 行数 | 判断 |
| --- | ---: | --- |
| `internal/service/agent_session_service.go` | 6247 | 明显过大，承担聚合、响应结构、审计快照和部分运行逻辑，后续应拆分响应 DTO、聚合 builder、审计 recorder 和服务编排 |
| `internal/service/agent_workflow_governance.go` | 4588 | 明显过大，虽然近期已有拆分，但仍不符合长期可维护目标 |
| `web/src/views/AgentPlanView.vue` | 3680 | 明显过大，后续应拆分 composable、摘要面板组件和任务卡片组件 |

结论：这些文件达到数千行不能简单视为正常的企业级设计结果。当前实现虽然有业务闭环推进价值，但从企业级代码质量角度看，必须持续进行模块化拆分、职责收敛和测试保护。后续新增能力不得继续扩大上述文件，除非是短期兼容性必要改动；优先使用独立 service 文件、独立前端组件或 composable。

## 6. 当前活动文档执行状态

上一活动文档 `docs/nowdoit/agent-wechat-web-progress-link-delivery-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-wechat-web-progress-link-delivery-plan-implemented-2026-06-25.md`。

当前活动文档：`docs/nowdoit/agent-wechat-final-result-report-delivery-plan.md`

上一轮完成项：

1. 已完成：梳理任务进度 URL、企业微信模板发送摘要和任务聚合结果。
2. 已完成：新增 Web 进度地址投递摘要 builder。
3. 已完成：`ListTasks` 接入地址投递摘要并写入审计快照。
4. 已完成：服务层测试补充地址投递字段断言。
5. 已完成：前端 API 类型和 Agent 任务工作台展示地址投递摘要。
6. 已验证：`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。
7. 已完成：企业微信真实模板消息或文本 fallback 中实际投递进度地址。
8. 已验证：`go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build`。
9. 已完成：本轮完成后归档活动文档并创建下一轮活动文档。

## 7. 下一轮实施顺序

1. 梳理最终结果汇报路径，包括最终回复、失败反馈和调度任务 worker。
2. 实现企业微信最终结果汇报模板卡片优先和文本 fallback。
3. 将最终结果真实发送结果写入审计，并在任务聚合摘要中暴露。
4. 运行完整验证矩阵，提交并推送。
5. 继续推进 Web 浏览器进度页权限校验与大文件拆分治理。

## 8. 最小验证命令

当前阶段每轮代码实现后至少运行：

```text
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```

涉及真实企业微信发送链路时，还必须补充：

```text
查询 agent.wechat_web_progress_link_snapshot 审计事件
查询 wechat_work.reply_sent 或 wechat_work.reply_failed 审计事件
核对企业微信消息是否包含 Web 浏览器可打开的进度地址
核对模板不可用时文本 fallback 是否包含同一地址
```

## 9. 当前禁止事项

- 不得删除用户文件或通过 git 操作导致文件消失，除非用户明确要求。
- 模型不得直接写数据库。
- controller 不得绕过 executor 直接调用业务变更 capability。
- executor 不得获得超出本次任务的 capability scope。
- 密钥、token、Webhook URL、数据库 DSN 不得进入模型上下文或 context trace。
- 企业微信进度地址投递不得绕过 Web 登录、权限、审批和审计边界。
- 后续迭代不得以继续扩大大文件作为常态实现方式。
