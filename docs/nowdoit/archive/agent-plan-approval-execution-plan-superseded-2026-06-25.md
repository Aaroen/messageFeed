# Agent 原子闭环执行计划

**最后更新**：2026-06-25

## 目标

将 `docs/implementation.md` 中 Agent 相关未完成事项合并为一个闭环实施包，完成从用户输入、会话建模、上下文装配、计划生成、审批确认、能力执行、结果追溯、记忆召回、联网信息获取、定时触发到评测安全的最小可运行闭环。

本实施包把 Agent 视为强原子系统：每个用户目标必须落入同一个可追溯事务链路，不能把 planner、approval、executor、memory、capability 和 observation 拆成互不一致的局部功能。

## 原子闭环边界

一次完整 Agent 闭环必须包含以下阶段：

```text
入口消息
session / turn 归档
controller run
上下文预算与 context trace
结构化 plan
policy / approval
executor task
capability 调用
observation / artifact 回写
controller 汇总
用户响应或后续任务调度
评测与审计记录
```

闭环完成的最低标准不是单个接口可用，而是同一个用户目标能完整穿过上述链路，并且每个关键决策都能被查询、复核和重放分析。

## P1 实施范围

1. 入口与会话归档：
   - 企业微信、Web 和内部触发入口统一创建或复用 `agent_sessions`。
   - 每次用户输入写入 turn 和 transcript entry。
   - 重复回调必须保持幂等，不得产生重复 controller run。
2. Controller run：
   - `ControllerAgent` 负责理解目标、生成结构化 plan、决定是否需要审批、分派 executor task、汇总 observation。
   - controller 不直接执行业务写操作。
   - controller 输出必须包含状态、下一步动作、用户可见回复和审计摘要。
3. 上下文与追溯：
   - `ContextBudgetManager` 负责模型可见上下文、工具上下文和历史召回预算。
   - `ContextTraceStore` 保存模型请求摘要、模型响应摘要、工具 schema、工具调用、裁剪记录、observation 和 artifact 引用。
   - 敏感配置、token、Webhook URL、数据库 DSN 不得进入 trace。
4. 结构化计划：
   - 定义 plan、plan step、executor task、影响评估和确认策略。
   - 计划状态至少包含 `draft`、`awaiting_approval`、`approved`、`rejected`、`expired`、`executing`、`completed`、`failed`。
   - 每个 step 必须声明 capability scope、预期输入、预期输出和失败策略。
5. Policy 与审批：
   - `PolicyEngine` 输出 `allow`、`prompt`、`forbidden`。
   - 只读任务可自动执行；写操作、外部通知、调度任务、联网抓取和潜在高成本操作必须可触发确认。
   - 审批通过后只允许执行获批 plan step 和 capability scope。
   - 审批拒绝、过期或 scope 变化后必须停止执行并回写状态。
6. Capability registry：
   - 统一注册、查询和执行 capability。
   - capability 必须声明输入 schema、输出 schema、权限等级、是否写操作、是否外部访问、是否可调度。
   - executor 只能调用 registry 中的 capability，不能直接绕过 service 或 repository。
7. Executor run：
   - executor 接收明确 task packet 和 capability scope，只执行一个任务。
   - executor 结果写入 observation，产物写入 artifact。
   - executor 失败必须结构化回传给 controller，由 controller 决定重试、降级、请求用户输入或终止。
8. 记忆与历史查询：
   - 提供短期 session 窗口。
   - 提供 `conversation.query_history` capability，支持按关键词、时间、角色、turn 和 transcript entry 查询原文。
   - 记忆召回必须受预算约束，并记录召回依据。
9. 联网信息获取：
   - 提供 `web.search`、`web.fetch_page`、`web.extract_page` 最小 capability。
   - 提供 `repo.search`、`repo.inspect_remote`，`repo.clone_reference` 仅允许写入受控 `references/`。
   - 联网结果必须记录来源、时间、摘要和可追溯引用。
10. 定时与后续任务：
    - 将 `agent.schedule_message` 升级为 `agent.schedule_task`。
    - 保存目标、执行窗口、投递时间、新鲜度策略、允许能力、模型策略和失败策略。
    - 到点后创建 controller run，不另起一套执行逻辑。
11. 评测与安全：
    - 建立 `agent_eval_cases`、`agent_eval_runs`、`agent_eval_results`。
    - 覆盖企业微信入口、订阅管理、推荐画像、AI 源、主动采集、通知、上下文记忆和安全对抗。
    - 指标至少包含任务成功率、工具选择准确率、权限决策正确率、越权拦截率、事实引用完整率和召回准确率。

## 非目标

- 不让模型直接写数据库。
- 不让 controller 绕过 executor 执行业务变更。
- 不让 executor 获得超出当前 task 的 capability scope。
- 不把 `repo.clone_reference` 结果写入产品源码目录。
- 不在第一轮闭环中实现复杂多 Agent 协作；当前只有 controller 和一次性 executor。
- 不在第一轮闭环中实现生产级网页浏览器自动化，优先使用静态 HTTP、正文抽取和仓库元数据读取。

## 数据模型

1. 复核并补齐既有模型：
   - `agent_sessions`
   - `agent_turns`
   - `agent_runs`
   - `agent_run_context_traces`
   - `agent_observations`
   - `agent_artifacts`
   - `agent_approvals`
2. 新增或扩展计划模型：
   - `agent_plans`
   - `agent_plan_steps`
   - `agent_plan_approvals`
   - `agent_scheduled_tasks`
3. 新增或扩展能力与评测模型：
   - `agent_capability_audit_logs`
   - `agent_eval_cases`
   - `agent_eval_runs`
   - `agent_eval_results`

所有模型必须保留 `session_id`、`turn_id`、`run_id` 或 `plan_id` 的可追溯关系，避免出现无法归因的执行记录。

## API 与查询面

1. Run 查询：
   - 查询 controller run。
   - 查询 executor run。
   - 查询 run 的 context trace、observation 和 artifact。
2. Plan 查询：
   - 查询当前 session 或 turn 的计划列表。
   - 查询计划详情、步骤、审批状态和执行状态。
   - 提交批准、拒绝和重新生成计划。
3. Capability 查询：
   - 查询已注册 capability。
   - 查询某个 plan step 可用 capability scope。
   - 查询 capability audit log。
4. Memory 查询：
   - 查询短期上下文窗口。
   - 查询历史 transcript entry。
   - 查询召回记录和裁剪记录。
5. Schedule 查询：
   - 创建、查询、提前执行、取消和查看定时任务。

## Web 与企业微信交互

1. 企业微信：
   - 用户消息进入 session / turn。
   - controller 能返回计划摘要、审批请求、执行结果和失败原因。
   - 重复回调不重复执行。
2. Web：
   - 提供必要的计划、审批、run trace、observation 和 artifact 可见数据面。
   - 不要求复杂编排界面，但必须支持用户理解计划影响并完成批准或拒绝。

## 实施顺序

1. 梳理现有 Agent session、turn、run、approval、context trace、handler、service 和 repository。
2. 修正或补齐当前 P0 Agent run 相关未提交实现，使其成为稳定基线。
3. 新增 plan、plan step、approval 关联和 schedule 相关迁移。
4. 实现 `AgentCapabilityRegistry` 和最小 capability 集合。
5. 实现 `ContextBudgetManager`、`ContextTraceStore` 和历史查询 capability。
6. 实现 `AgentPlanner` 和 `PolicyEngine`。
7. 实现 plan service 状态机和审批单调状态流转。
8. 实现 executor task 与 plan step 的绑定和 scope 二次校验。
9. 接入企业微信入口，形成一次完整 controller -> plan -> approval -> executor -> observation -> response 链路。
10. 接入 Web 查询和审批 API。
11. 实现联网信息获取最小 capability。
12. 实现定时任务最小闭环。
13. 建立评测用例、评测运行和安全对抗用例。
14. 执行后端、前端和 Compose 验收。
15. 将本计划归档，并根据主文档生成下一实施包计划。

## 长程任务提交策略

- 每完成一个可验证阶段必须提交并推送。
- 数据迁移和模型基线单独提交。
- capability registry 和 executor scope 校验单独提交。
- planner、policy 和 approval 状态机单独提交。
- 企业微信闭环单独提交。
- Web 查询和审批数据面单独提交。
- 联网 capability、定时任务和评测能力分别提交。

## 验收标准

- 同一个企业微信用户目标可完整生成 session、turn、controller run、plan、approval、executor run、observation、artifact 和用户响应。
- 审批前不会执行需要确认的写操作。
- 审批通过后 executor 只能调用获批 capability scope。
- 审批拒绝、过期或 scope 变化后不会继续执行。
- context trace 可查询，且不包含敏感配置。
- 历史查询 capability 能按关键词、时间、角色和 turn 返回原文引用。
- 联网 capability 返回结果带来源、时间和摘要。
- 定时任务到点后创建 controller run，并复用同一执行闭环。
- 评测用例可运行并输出核心指标。
- 企业微信重复回调具备幂等保护。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。
- Compose 环境中 `/healthz`、Agent 查询 API 和企业微信入口冒烟通过。

## 风险与约束

- Agent 闭环必须以结构化状态机为准，不能依赖自然语言计划文本判断执行权限。
- 审批状态必须单调，批准、拒绝、过期和取消之间不能被旧请求覆盖。
- capability scope 必须在计划生成和执行前二次校验。
- 记忆召回必须有预算和审计，避免把无关历史塞入模型上下文。
- 联网信息必须带来源引用，不能把外部内容写成无来源事实。
- 所有外部访问必须有超时、大小限制和错误摘要。
- 所有执行记录必须可归因到 session、turn、run、plan 或 scheduled task。

## 后续衔接

本闭环完成后，再按主文档拆分非核心增强：推荐画像、摘要日报、通知策略、金融分析、E2E 测试、性能压测和多节点验证。
