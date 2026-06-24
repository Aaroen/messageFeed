# Agent 计划与审批执行计划

**最后更新**：2026-06-25

## 目标

完成 `docs/implementation.md` 中 P1 `Agent 计划与审批` 实施包，使 Agent 在执行有影响的操作前能够形成结构化计划、拆分步骤、评估影响范围、触发确认策略，并通过统一执行器约束 capability 调用。

## P1 实施范围

1. 结构化计划模型：
   - 定义 plan、plan step、executor task、影响评估和确认策略的数据结构。
   - 为计划状态提供 `draft`、`awaiting_approval`、`approved`、`rejected`、`expired`、`executing`、`completed`、`failed` 等有限状态。
   - 保留 session、turn、run 和用户审批记录的可追溯关系。
2. 计划生成服务：
   - 在 controller run 中生成结构化计划草案。
   - 将用户目标、影响范围、预期写操作、所需 capability scope 和风险摘要写入计划。
   - 对只读任务与写操作任务采用不同确认策略。
3. 审批链路：
   - 复用现有 `agent_approvals` 语义，补齐计划批准、拒绝、过期和二次校验流程。
   - 审批通过后只允许执行批准计划中的 capability scope。
   - 审批拒绝或过期后不得继续执行对应计划。
4. 通用执行器约束：
   - `AgentExecutor` 只能调用已注册 capability 和既有 service。
   - executor task 必须绑定 plan step 和 capability scope。
   - 执行结果写回 observation，并可被 controller 汇总。
5. API 能力：
   - 增加计划查询、计划详情、审批提交和计划执行状态查询接口。
   - 响应包含计划步骤、状态、影响摘要、审批状态和关联 run。
6. 测试与验收：
   - 覆盖计划状态流转、审批拒绝、审批过期、scope 越权拦截和 executor 结果回写。
   - 保持既有 Agent session、turn、run 查询能力不回退。

## 非目标

- 不实现联网 capability。
- 不实现长期记忆和冷热归档。
- 不新增前端复杂编排界面，只补齐必要的审批可见数据面。
- 不允许模型直接写数据库。
- 不绕过现有 service、repository 和权限边界。

## 验收标准

- controller 能生成包含 step、impact、capability scope 和 confirmation policy 的计划。
- 需要确认的计划在批准前不会执行写操作。
- 拒绝或过期的计划不会创建 executor 执行。
- executor 只能执行获批 scope 内的 capability。
- observation、artifact 或错误能回写到关联 run 和 plan step。
- API 可查询计划、步骤、审批状态和关联 run。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。

## 实施顺序

1. 梳理现有 Agent session、turn、run、approval、context trace 和 handler 实现。
2. 新增或扩展计划、计划步骤和审批关联的数据模型与迁移。
3. 实现 plan service，负责创建计划、状态流转和审批约束。
4. 将 controller run 的输出接入结构化计划生成。
5. 实现 executor task 与 plan step 的绑定和 capability scope 校验。
6. 扩展 Handler 与 OpenAPI，提供计划查询、审批提交和状态查询。
7. 补齐聚焦测试和回归测试。
8. 执行后端、前端和 Compose 冒烟验证。
9. 将本计划归档，并根据主文档生成下一实施包计划。

## 风险与约束

- 审批状态必须具备单调性，避免批准后被旧请求覆盖。
- capability scope 必须在执行前二次校验，不能只依赖计划生成阶段。
- 计划文本不能成为唯一事实来源，执行器必须读取结构化字段。
- 敏感配置、token、数据库 DSN 不得进入计划、审批说明或模型上下文。
- 计划和执行结果需要可追溯，但不应记录过量模型上下文。

## 后续衔接

本计划完成后，下一实施包优先处理 `Agent 记忆与历史查询`，或回到阶段三遗留的统一错误类型抽象。
