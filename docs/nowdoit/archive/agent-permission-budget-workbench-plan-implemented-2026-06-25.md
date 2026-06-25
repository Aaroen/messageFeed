# Agent 权限预算治理与工作台计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企微任务入口、实时进度、审批控制、多轮编排、结果复用和长期记忆治理的基础上，推进更细粒度的 capability 权限策略、任务预算治理和 Web Agent 工作台，使复杂任务在执行前可解释、执行中可控、执行后可审计。

## 2. 实施范围

1. capability 权限策略：
   - 为 capability 增加更细粒度的风险、数据域、外部访问、可调度和可复用标记。
   - 区分只读本地查询、长期记忆召回、联网读取、定时任务创建和通知发送。
   - Web 与企微回复中明确说明被拒绝、需确认或可直接执行的能力边界。
2. 任务预算治理：
   - 为 plan/run 增加上下文预算、工具调用预算、联网预算和回复预算的可见摘要。
   - 对超预算任务提供降级策略，例如减少来源、缩短时间范围或转为需要确认。
   - 在 audit 与 progress event 中记录预算命中、降级和拒绝原因。
3. 复杂任务拆解质量：
   - 对 planner 输出增加步骤质量检查，包括目标覆盖、证据要求、风险说明和失败策略。
   - 为 eval 增加复杂任务拆解案例，覆盖多来源总结、历史追问、联网检索和定时汇报。
   - 将失败案例写入 eval trend，便于后续迭代。
4. Web Agent 工作台：
   - 在 Web 端提供更集中的任务工作台视图，展示 active、waiting approval、failed、completed 和 scheduled task。
   - 显示每个任务的权限状态、预算状态、最近进度和下一步动作。
   - 支持从工作台进入计划详情、重试失败步骤、恢复执行或基于结果创建新任务。
5. 企微协同：
   - 企微过程通知补充权限和预算摘要。
   - 任务完成汇报保留结果复用、证据引用和下一步建议。

## 3. 非目标

- 本轮不引入外部工作流引擎。
- 本轮不开放任意写操作 capability。
- 本轮不实现跨用户任务共享。
- 本轮不重构现有数据库主键或认证体系。

## 4. 验收标准

- capability 决策能够体现风险、数据域、外部访问和是否需要确认。
- plan/progress 能展示预算摘要、预算命中和降级原因。
- eval 覆盖复杂任务拆解质量，失败结果进入趋势统计。
- Web 工作台能按任务状态集中展示 Agent 任务，并能进入详情或触发已有控制操作。
- 企微通知和最终汇报包含权限、预算、证据引用和下一步建议。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更完整的任务工作流恢复、结果质量评分、跨设备通知一致性和生产部署验收。

## 6. 实施结果

本轮已完成以下实现：

1. capability 权限策略：
   - `Capability` 增加 `DataDomain` 与 `Reusable` 标记，覆盖本地订阅、长期记忆、外部 Web、外部仓库、定时任务、通知和派生内容等数据域。
   - planner 为 plan 写入 `permission_governance`，包含能力风险、数据域、外部访问、状态变更、可调度、可复用、决策和确认要求。
   - 每个 plan step 的 `RetryMetadata.permission` 记录该步骤的 capability 权限摘要。
2. 任务预算治理：
   - planner 为 plan 写入 `budget_governance`，记录上下文字符预算、工具调用预算、联网调用预算、回复 token 预算、预算状态和降级策略。
   - plan 创建后写入 `agent.plan_governance_recorded` audit，包含权限、预算、质量检查和下一步动作，可进入 progress recent events。
   - progress phase 新增 `permission` 和 `budget` 阶段，展示权限摘要和预算摘要。
3. 复杂任务拆解质量：
   - planner 写入 `planner_quality`，检查目标覆盖、证据要求、风险说明和失败策略。
   - 内置 eval 增加复杂多来源总结、预算降级和权限确认案例。
   - eval 结果继续进入 eval run 与 trend 统计。
4. Web Agent 工作台：
   - `AgentTaskSummaryResponse` 增加 `permission_status`、`budget_status`、`latest_progress` 和 `next_action`。
   - Web 最近任务列表展示权限状态、预算状态、最近进度和下一步动作，并保留进入详情能力。
   - 计划详情页展示权限治理和预算治理摘要。
5. 企微协同：
   - 审批提示、过程通知和最终汇报补充权限摘要和预算摘要。
   - 最终汇报补充 evidence refs 和下一步建议。
6. 测试覆盖：
   - 新增 planner 权限、预算和质量 metadata 测试。
   - 更新 progress/task summary 测试，覆盖权限和预算阶段及工作台字段。
   - 更新企微过程通知测试，覆盖权限和预算文本。
   - 更新 eval 测试以覆盖新增内置案例。

## 7. 验证记录

以下命令已通过：

```bash
go test ./...
go vet ./...
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
```
