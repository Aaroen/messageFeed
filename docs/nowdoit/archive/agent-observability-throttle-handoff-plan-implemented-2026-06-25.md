# Agent 生产观测吞吐限制与人工接管计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企微入口、实时进度、审批控制、多轮编排、恢复治理、结果质量评分、跨设备一致展示和部署验收 metadata 的基础上，推进生产运行观测、任务吞吐限制、人工接管流程和用户级策略配置，使 Agent 在持续运行场景中具备更明确的容量边界、异常处置路径和用户可配置治理能力。

## 2. 实施范围

1. 生产运行观测：
   - 为 Agent plan、run、scheduled task、eval 和通知投递建立更完整的观测摘要。
   - 在 Web 任务详情或任务工作台中展示最近失败原因、恢复次数、质量评分和通知状态。
   - 将关键运行事件写入 audit metadata，便于追踪任务从入口到最终汇报的完整链路。
2. 任务吞吐限制：
   - 为用户级 Agent 任务建立并发与排队限制。
   - 对 Web 入口、企微入口和定时任务入口统一限制策略。
   - 当达到限制时返回可读的等待、降级或拒绝原因。
3. 人工接管流程：
   - 为失败、低质量评分、权限不确定和预算超限任务提供人工接管状态。
   - Web 可查看需要接管的任务，并可标记接管、恢复、取消或重新排队。
   - 企微通知中体现接管状态和下一步动作。
4. 用户级策略配置：
   - 为通知偏好之外的 Agent 策略增加用户级配置入口。
   - 覆盖最大并发数、自动恢复偏好、低质量阈值、失败通知和人工接管触发条件。
   - 保持默认策略保守，不改变已有用户的基本任务入口行为。

## 3. 非目标

- 本轮不引入外部队列系统。
- 本轮不进行真实生产部署。
- 本轮不开放跨用户任务共享。
- 本轮不实现任意写操作 capability。

## 4. 验收标准

- Web/企微/定时任务入口共享同一用户级吞吐限制判断。
- 达到限制、需要接管或自动恢复失败时，Web 与企微均能展示明确原因和下一步动作。
- Agent 任务详情能展示运行观测摘要，包括失败、恢复、质量和通知相关信息。
- 用户级策略配置能读取、保存并影响后续 Agent 任务调度或提示。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更细粒度的能力策略、长期运行报表、Agent 任务 SLA 统计和多节点部署一致性。

## 6. 本轮实施清单

1. 用户级策略配置：
   - 在现有 Agent 通知偏好配置中增加最大并发数、最大排队数、自动恢复开关、低质量接管阈值和接管触发条件。
   - 增加迁移、domain、repository、service、handler、Web API 和设置页表单字段。
2. 统一吞吐限制：
   - 实现共享的用户级 Agent 策略判断 helper。
   - Web 任务入口、企微消息入口和定时 worker 入口复用同一判断结果。
   - 达到限制时输出明确原因、下一步动作和策略 metadata。
3. 生产运行观测：
   - 为 plan 生成 `runtime_observability` metadata，汇总失败、恢复、质量、通知和吞吐状态。
   - 在 progress phase、Web 任务详情和企微文本中展示观测摘要。
4. 人工接管：
   - 为失败、低质量、权限不确定和预算超限 plan 生成 `handoff` metadata。
   - Web 任务详情展示接管状态、触发原因和建议动作。
   - 企微进度与最终汇报体现接管状态。
5. 验证：
   - 补充策略默认值、策略更新、入口限流、观测 metadata、接管 metadata 和 progress phase 测试。
   - 完整执行 Go 与前端验证命令。

## 7. 实施结果

1. 用户级策略配置：
   - 扩展 `agent_notification_preferences`，新增最大并发数、最大排队数、自动恢复开关、低质量接管阈值和接管触发条件。
   - 同步 domain、repository、service、handler、Web API 和设置页表单。
2. 统一吞吐限制：
   - 新增共享 admission 判断 helper，统计 active plan、queued plan、running scheduled task 和 queued scheduled task。
   - Web 任务入口和企微消息入口在创建任务前执行用户级策略判断。
   - 定时 worker 在领取任务后、执行前执行同一策略判断，受限任务释放锁并重新排队。
3. 生产运行观测：
   - plan 终态写入 `runtime_observability` metadata，包含状态、失败步骤、最近失败、恢复次数、质量分和吞吐状态。
   - 任务列表、progress phase 和文本摘要展示运行观测摘要。
4. 人工接管：
   - plan 终态写入 `handoff` metadata，覆盖失败、低质量、权限和预算触发条件。
   - Web 计划详情、任务列表和企微最终汇报展示接管状态与下一步动作。
5. 测试：
   - 补充策略默认值与更新、Web 入口限流、定时 worker 限流、运行观测和人工接管 progress phase 测试。

## 8. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
