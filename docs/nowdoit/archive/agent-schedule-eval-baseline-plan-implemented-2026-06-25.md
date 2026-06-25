# Agent 调度与评测基线实施计划

**创建日期**：2026-06-25

## 1. 本轮目标

本轮在既有 Agent plan、approval、run 和 capability audit 基础上，补齐调度任务与评测安全的数据基线，使后续 `agent.schedule_task`、到点创建 controller run、企业微信/Web 进度展示和安全评测可以依赖稳定持久化模型。

## 2. 实施范围

1. 新增 `agent_scheduled_tasks` 迁移，保存 Agent 定时契约：
   - `user_id`、`session_id`、`turn_id`、`plan_id`、`source_run_id`
   - 目标、执行窗口、计划触发时间、投递时间、新鲜度策略、允许能力、模型策略、失败策略
   - 状态、尝试次数、最近错误、下一次运行时间、完成时间
2. 新增 `agent_eval_cases`、`agent_eval_runs`、`agent_eval_results` 迁移，保存评测用例、运行批次和单项结果。
3. 新增 domain 模型，表达调度任务和评测实体的状态枚举与结构体。
4. 新增 repository 基础方法：
   - 创建、查询、领取到期调度任务、更新调度任务状态。
   - 创建评测用例、创建评测运行、写入评测结果、查询运行详情。
5. 新增 service 薄层，封装状态流转与输入规整，供后续 capability 和 worker 复用。
6. 增加聚焦单元测试，覆盖状态默认值、到期任务领取、评测结果落库和运行统计。

## 3. 非目标

- 本轮不把 `agent.schedule_message` 完整迁移为 `agent.schedule_task`。
- 本轮不实现真实定时 worker 到点创建 controller run。
- 本轮不实现完整 Web 可视化进度面。
- 本轮不实现评测 CLI 或批量执行器。
- 本轮不引入外部浏览器自动化。

## 4. 验收标准

- 数据库迁移包含 `agent_scheduled_tasks`、`agent_eval_cases`、`agent_eval_runs`、`agent_eval_results` 及必要索引和约束。
- domain、repository、service 均有可编译实现。
- repository 或 service 测试覆盖本轮新增主要状态流转。
- `go test ./...` 通过。
- `npm --prefix web run type-check` 通过，确认前端未被破坏。

## 5. 后续衔接

本轮完成后，下一轮应基于 `agent_scheduled_tasks` 实现 `agent.schedule_task` capability、审批后写入定时契约、到点创建 controller run，并通过 Web/企业微信暴露调度任务执行进度。

## 6. 完成记录

**完成日期**：2026-06-25

已完成：

- 新增 `migrations/000025_add_agent_scheduled_tasks_eval.up.sql` 和 `down.sql`。
- 新增 `internal/domain/agent_schedule_eval.go`，定义调度任务与评测状态枚举和实体。
- 新增 `internal/repository/agent_schedule_eval_repository.go`，实现调度任务创建、查询、领取、更新，以及评测用例、运行和结果持久化。
- 新增 `internal/service/agent_schedule_eval_service.go`，封装调度任务创建、领取、完成、失败重试，以及评测用例、运行和结果记录。
- 新增正式测试文件覆盖迁移结构、仓储规整逻辑和服务状态流转。

验证命令：

```text
go test ./...
go vet ./...
npm --prefix web run type-check
```

验证结果均通过。
