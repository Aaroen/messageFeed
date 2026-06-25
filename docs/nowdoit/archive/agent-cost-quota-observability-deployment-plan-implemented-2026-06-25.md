# Agent 成本配额观测图表与部署验证计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备任务入口、实时进度、恢复治理、质量评分、用户级策略、capability 级策略、SLA 摘要、任务报表和多节点一致性 metadata 的基础上，推进更完整的生产可观测图表、任务成本统计、按用户和能力的配额管理，以及多节点实际部署验证前置检查，使 Agent 持续运行时具备成本边界、容量边界和可视化运营基础。

## 2. 实施范围

1. 成本统计：
   - 为 plan、run、capability observation 和 scheduled task 汇总基础成本摘要。
   - 统计工具调用数、外部访问数、估算 token、重试次数和通知投递次数。
   - 将成本摘要写入 plan metadata，并在 Web 详情展示。
2. 配额管理：
   - 在用户级策略中增加每日任务数、每日外部访问数和 capability 调用上限。
   - Web/企微/定时 worker 入口复用同一配额判断。
   - 达到配额时给出明确原因和下一步动作。
3. 可观测图表：
   - 在 Web 任务工作台增加 SLA、成本、状态分布和入口分布的轻量图表或结构化摘要。
   - 保持现有页面结构，不引入额外前端图表库。
4. 多节点部署验证：
   - 为多节点部署前置检查增加数据库迁移、worker claim、通知幂等、限流一致性和 healthz 检查项。
   - 将验证结果纳入部署验收 metadata 或 progress phase。

## 3. 非目标

- 本轮不引入外部队列系统。
- 本轮不执行真实生产部署。
- 本轮不实现跨用户共享任务。
- 本轮不开放任意写操作 capability。

## 4. 验收标准

- Agent 任务详情可展示成本摘要。
- 用户级配额能配置、保存，并影响 Web/企微/定时 worker 任务入口。
- Web 任务工作台可展示 SLA、成本和状态分布摘要。
- 多节点部署验证检查项进入 deployment metadata 或 progress phase。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进生产部署脚本化验收、运行告警、成本趋势留存和更细粒度的企业微信交互式进度组件。

## 6. 本轮实施清单

1. 成本统计：
   - 在 plan 终态写入 `cost_summary` metadata。
   - 统计工具调用数、外部访问数、估算 token、重试次数、通知投递次数和 scheduled task 关联数量。
   - Web 计划详情与任务工作台展示成本摘要。
2. 配额管理：
   - 在用户级 Agent 策略中增加每日任务数、每日外部访问数和每日 capability 调用数上限。
   - Web、企微和定时 worker 入口复用同一 admission 判断。
   - 达到配额时写入 audit metadata，并返回明确原因和下一步动作。
3. 可观测摘要：
   - 扩展任务工作台响应，增加成本汇总。
   - Web 任务工作台展示 SLA、成本、状态分布、入口分布和能力分布摘要。
4. 多节点部署验证：
   - 扩展 deployment acceptance metadata，增加数据库迁移、healthz、worker claim、通知幂等、限流一致性和配额一致性检查。
   - progress phase 展示部署验证摘要。
5. 验证：
   - 补充成本 metadata、配额判断、任务工作台成本摘要和部署验证 phase 测试。
   - 完整执行 Go 与前端验证命令。

## 7. 实施结果

1. 成本统计：
   - plan 终态写入 `cost_summary` metadata，覆盖工具调用、外部访问、估算 token、重试次数、通知次数和关联定时任务数。
   - 企微最终汇报、progress 文本摘要、progress phase 和 Web 计划详情展示成本摘要。
2. 配额管理：
   - 用户级 Agent 策略新增 `daily_task_quota`、`daily_external_call_quota` 和 `daily_capability_call_quota`。
   - admission 判断扩展每日任务数、外部访问数和 capability 调用数，Web、企微和定时 worker 入口复用同一判断结果。
   - 配额命中时写入 audit metadata，并返回明确原因和下一步动作。
3. 可观测摘要：
   - 任务工作台响应新增 `cost` 汇总，聚合工具调用、外部访问、估算 token、重试、通知和定时任务数量。
   - Web 最近任务区域展示 SLA、成本、状态分布、入口分布、能力分布和接管分布摘要。
4. 多节点部署验证：
   - deployment acceptance metadata 增加配额一致性、迁移就绪和 healthz 就绪检查项。
   - progress phase 保持展示部署验收与多节点一致性检查。
5. 测试：
   - 增加用户策略配额默认值与更新、Web 入口每日配额阻断、成本 metadata、成本 phase 和任务工作台成本聚合断言。

## 8. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
