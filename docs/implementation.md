# messageFeed 剩余实施计划

**最后更新**：2026-06-24

已实现内容已归档到 `docs/nowdoit/archive/implementation-implemented-summary-2026-06-24.md`。本文件只保留后续未完成事项和当前落地顺序。

## 1. 当前剩余总览

| 优先级 | 模块 | 状态 | 目标 |
| --- | --- | --- | --- |
| P0 | Agent Controller / Executor 运行时 | 待实现 | 建立唯一主控 Agent 和一次性执行 AgentRun |
| P0 | Agent 上下文追溯 | 待实现 | executor 的完整模型可见上下文、工具上下文和输出可追溯 |
| P0 | 阶段二收尾 | 待完成 | Web 条目状态操作、筛选、分页、阅读偏好完整绑定 |
| P0 | 阶段三收尾 | 待完成 | 完整 Compose 环境端到端观测验收 |
| P1 | 阶段四收尾 | 待完成 | 源目录健康检查、许可状态、热度和更多过滤维度 |
| P1 | Agent 计划与审批 | 待实现 | 结构化 plan、plan step、影响评估、确认策略和通用执行器 |
| P1 | Agent 记忆与历史查询 | 待实现 | 企微短期窗口、历史原文查询、冷热归档索引和回忆预算 |
| P1 | 联网信息获取 | 待实现 | web/repo 信息获取 capability |
| P2 | 调度、通知、推荐、金融和工程化增强 | 待实现 | 在 Agent 运行时闭环上逐步扩展 |

## 2. 当前第一实施包

当前第一实施包以 `docs/nowdoit/agent-controller-executor-implementation-plan.md` 为准。

目标：

```text
ControllerAgent 唯一主控
ExecutorAgentRun 即用即丢
CapabilityRegistry 统一复用
ContextTraceStore 完整追溯
```

必须先完成：

1. 新增或调整 `agent_runs`，支持 `parent_run_id`、`role`、`capability_scope_json`、`context_trace_ref`。
2. 新增 `agent_run_context_traces`，保存模型实际可见上下文投影视图。
3. 新增 `agent_observations` 和 `agent_artifacts`。
4. 实现 `RunManager.CreateControllerRun` 和 `RunManager.CreateExecutorRun`。
5. 实现 `ContextTraceStore`。
6. 实现最小 `CapabilityRegistry`。
7. 让 controller 创建一个只读 executor 完成一次企业微信请求。
8. 提供按 `run_id` 查询 controller、executor、context trace、observation 和 artifact 的接口。

## 3. Agent 剩余实施步骤

### 3.1 运行时

- [ ] `ControllerAgent`：理解用户输入、生成 executor task、汇总 observation、决定继续或结束。
- [ ] `ExecutorAgentRun`：接收明确任务包和 capability scope，只执行一个任务。
- [ ] `RunLoop`：支持 controller run 与 executor run 的状态流转。
- [ ] `ContextFitEstimator`：估算 controller 与 executor 是否能在各自上下文预算内完成。
- [ ] `ContextBudgetManager`：管理模型可见投影视图、工具结果裁剪和工具调用对保护。
- [ ] `ContextTraceStore`：保存模型请求、模型响应、工具 schema、工具调用、observation 和裁剪记录。

### 3.2 能力与策略

- [ ] `AgentCapabilityRegistry`：注册、查询、检索、执行 capability。
- [ ] `AgentCapabilitySearch`：延迟能力发现。
- [ ] `PolicyEngine`：输出 `allow`、`prompt`、`forbidden`。
- [ ] `AgentPlanner`：生成结构化 plan、step、executor task 和影响评估。
- [ ] `AgentExecutor`：只能调用已注册 capability 和既有 service。
- [ ] `agent_approvals` 前后端确认链路：展示计划、批准、拒绝、过期和二次校验。

### 3.3 记忆与上下文

- [ ] 企微短期聊天窗口。
- [ ] `conversation.query_history` 按关键词、时间、角色、turn 和 transcript entry 查询原文。
- [ ] transcript 冷热归档索引。
- [ ] 召回预算和召回审计。
- [ ] 用户偏好、订阅和画像查询 capability。
- [ ] 清空数据库上下文、切换当前企微 session、新建 session 和删除 session 的 Web 管理入口持续完善。

### 3.4 联网信息获取

- [ ] `web.search`：搜索候选网页。
- [ ] `web.fetch_page`：抓取页面响应和元数据。
- [ ] `web.extract_page`：正文、标题、发布时间、作者、站点名和主要链接抽取。
- [ ] `repo.search`：搜索参考仓库候选。
- [ ] `repo.inspect_remote`：不克隆读取 README、目录树、license 和指定文件片段。
- [ ] `repo.clone_reference`：浅克隆到受控 `references/`，记录审计，不进入构建、测试、部署。
- [ ] `web.browse_page`：后置，仅用于静态 HTTP 与正文抽取不能满足任务的页面。

### 3.5 定时任务

- [ ] 将 `agent.schedule_message` 升级为 `agent.schedule_task`。
- [ ] 保存定时契约：目标、执行窗口、投递时间、新鲜度策略、允许能力、模型策略和失败策略。
- [ ] 到点后创建 controller `AgentRun`，不另起一套执行逻辑。
- [ ] 支持提前执行、准时投递、失败汇报和用户确认。

### 3.6 评测与安全

- [ ] `agent_eval_cases`、`agent_eval_runs`、`agent_eval_results`。
- [ ] 固定用例覆盖企业微信入口、订阅管理、推荐画像、AI 源、主动采集、通知、金融分析、上下文记忆和安全对抗。
- [ ] 安全用例覆盖 prompt injection、敏感信息泄露、未授权通知目标、默认永久删除和绕过访问限制。
- [ ] 输出任务成功率、工具选择准确率、权限决策正确率、越权拦截率、事实引用完整率和召回准确率。

## 4. 非 Agent 收尾事项

### 4.1 阶段二收尾

- [ ] Web 界面支持已读、收藏、隐藏和取消状态。
- [ ] Web 界面支持时间线筛选、分页加载和阅读模式偏好持久化。
- [ ] 独立 `/items/:id` 详情路由。
- [ ] `ActionBar` 状态操作组件。
- [ ] 完整 OpenAPI 契约、契约校验和 Swagger UI。
- [ ] 前后端联调验收和端到端测试。

### 4.2 阶段三收尾

- [ ] 使用完整 Compose 环境做一次端到端验收。
- [ ] 确认同一请求可通过 `request_id` 和 `trace_id` 串联日志、指标和 trace。
- [ ] 继续抽象统一错误类型，供 AI、通知、金融和自然语言控制模块复用。

### 4.3 阶段四收尾

- [ ] 源目录自动健康检查。
- [ ] 许可状态治理。
- [ ] 热度字段。
- [ ] 最近校验时间更新流程。
- [ ] 语言、国家、健康状态等过滤维度。

## 5. 后续阶段

### 阶段六：主动采集与内容理解

- [ ] `web_acquisition_tasks` 和 `web_snapshots`。
- [ ] `WebAcquisitionProvider`、`SearchProvider`、`PageExtractor`、`SnapshotStore`。
- [ ] 静态网页抓取和正文抽取。
- [ ] 网页变化监控。
- [ ] 搜索型采集、去重和来源评估。
- [ ] 采集结果与 `items`、AI 源和推荐候选池打通。

### 阶段七：推荐、摘要与通知

- [ ] 推荐画像与反馈闭环。
- [ ] 摘要、日报、周报和热点条目生成。
- [ ] 通知规则、通知渠道和冷却策略。
- [ ] 高优先级 AI 源条目或告警推送。

### 阶段八：金融与跨领域分析

- [ ] 行情源接入。
- [ ] 市场日历、行情快照和事件关联。
- [ ] 金融资讯与行情异动解释。
- [ ] 风险提示与通知规则。

### 阶段九：工程化增强

- [ ] E2E 测试。
- [ ] API 契约校验。
- [ ] 性能压测。
- [ ] 数据保留与归档策略。

### 阶段十：来源扩展与分布式升级验证

- [ ] 更多来源桥接。
- [ ] 多节点 API 验证。
- [ ] 共享任务锁验证。
- [ ] Redis 作为可选缓存、队列、限流或分布式锁实现。

## 6. 最小验收命令

```text
go test ./...
go vet ./...
npm --prefix web run type-check
docker compose ps
curl -sk https://localhost:8443/healthz
```

完整 Agent P0 完成后，还必须补充：

```text
查询 controller run
查询 executor run
查询 executor context trace
查询 observation
查询 artifact
验证企业微信重复回调幂等
验证敏感配置不进入 context trace
```

## 7. 当前禁止事项

- 模型不得直接写数据库。
- controller 不得绕过 executor 直接调用业务变更 capability。
- executor 不得获得超出本次任务的 capability scope。
- `repo.clone_reference` 不得写入产品源码目录，不得自动修改 `go.mod`、构建、测试或部署配置。
- 密钥、token、Webhook URL、数据库 DSN 不得进入模型上下文或 context trace。
