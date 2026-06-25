# Agent Controller / Executor 落地执行计划

**最后更新**：2026-06-24

## 目标

将 Agent 运行时落地为“唯一 `ControllerAgent` + 多个一次性 `ExecutorAgentRun`”：

- `ControllerAgent` 负责理解用户输入、拆分任务、选择能力、创建 executor、汇总结果、追问、确认和最终回复。
- `ExecutorAgentRun` 即用即丢，只执行一个明确任务包，结束后销毁运行上下文，但完整持久化模型可见上下文、工具上下文、observation、artifact 和审计记录。
- 所有 Agent 复用同一个 `AgentCapabilityRegistry`，不同 executor 只获得本次任务授权的 `capability_scope`。

## P0 实施范围

P0 只实现可运行闭环，不实现复杂并行调度和外部 A2A/MCP：

1. 数据库支持 controller/executor run。
2. 支持 executor 上下文追溯。
3. 支持统一 capability registry。
4. 支持 controller 创建一个 executor 完成只读任务。
5. 支持企业微信入口收到消息后返回可追溯回复。

## 数据模型

新增或调整：

```text
agent_runs
- id
- parent_run_id
- session_id
- turn_id
- role: controller / executor
- status
- task_packet_json
- capability_scope_json
- model_key
- context_budget_json
- context_trace_ref
- result_ref
- trace_id
- started_at
- completed_at
- created_at
```

```text
agent_run_context_traces
- id
- run_id
- trace_kind
- prompt_version
- model_key
- content_json
- content_hash
- redaction_status
- token_estimate
- created_at
```

```text
agent_observations
- id
- run_id
- capability_key
- input_summary
- output_summary
- status
- error
- artifact_refs_json
- created_at
```

```text
agent_artifacts
- id
- run_id
- artifact_type
- content_ref
- summary
- source_refs_json
- content_hash
- created_at
```

## Go 模块落点

```text
internal/agent/controller
internal/agent/executor
internal/agent/run
internal/agent/capability
internal/agent/context
internal/agent/audit
```

建议接口：

```text
ControllerAgent.Handle(turn) -> ControllerResult
ExecutorAgent.Run(task, capabilityScope) -> ExecutorResult
RunManager.CreateControllerRun(...)
RunManager.CreateExecutorRun(parentRunID, task, capabilityScope)
ContextTraceStore.Save(...)
CapabilityRegistry.Execute(...)
```

## 第一阶段实现顺序

1. 增加迁移：`agent_runs`、`agent_run_context_traces`、`agent_observations`、`agent_artifacts`。
2. 增加 domain 对象：`AgentRun`、`ExecutorTask`、`ContextTrace`、`Observation`、`Artifact`。
3. 增加 repository：run、context trace、observation、artifact。
4. 实现 `RunManager`：创建 controller run、创建 executor run、状态流转。
5. 实现 `ContextTraceStore`：保存模型可见上下文投影视图和工具执行上下文。
6. 实现最小 `CapabilityRegistry`：注册、查询、执行。
7. 实现只读 executor：支持 `feed.query_recent_items`、`source.query_latest_items`、`content.summarize_text`。
8. 实现最小 controller：收到用户消息后创建一个 executor，并汇总 executor 结果。
9. 接入企业微信 turn worker。
10. 补充 Web 查询接口：按 run 查看 controller、executor、context trace、observation 和 artifact。

## 验收标准

- 一条企业微信消息会创建一个 controller `agent_runs` 记录。
- controller 至少创建一个 executor `agent_runs` 记录，且 `parent_run_id` 正确。
- executor 只获得本次任务授权的 `capability_scope`。
- executor 的模型可见上下文、工具调用、observation 和最终输出可以按 `run_id` 查询。
- 敏感配置不会进入 `agent_run_context_traces.content_json`。
- 只读任务可以正常回复用户。
- 失败任务会保留失败状态、错误摘要和上下文追溯记录。
- 重复企业微信回调不会重复创建 controller run。

## 后续扩展

1. executor 并发调度。
2. 联网信息获取 executor：`web.search`、`web.fetch_page`、`web.extract_page`、`repo.search`、`repo.inspect_remote`。
3. 写操作 executor：提醒、通知、订阅、AI 源写入。
4. `agent.schedule_task` 到点创建 controller run。
5. 外部 MCP/A2A 适配。
