# Agent 能力策略 SLA 统计与多节点一致性计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备任务入口、实时进度、恢复治理、质量评分、部署验收、用户级吞吐限制、运行观测和人工接管基础上，推进更细粒度的能力策略、长期运行报表、任务 SLA 统计和多节点部署一致性，使 Agent 在生产级持续运行场景中具备可度量、可约束和可横向扩展的治理基础。

## 2. 实施范围

1. 能力策略：
   - 为 capability 增加更细粒度的用户级策略覆盖能力。
   - 支持按 capability key 配置允许、降级、需要确认或拒绝。
   - 将策略命中结果写入 plan metadata 和 audit。
2. SLA 统计：
   - 为 Agent plan、scheduled task、通知投递和 eval 建立基础 SLA 指标。
   - 统计成功率、失败率、平均耗时、超时数量、恢复次数和人工接管次数。
   - Web 展示近期 SLA 摘要。
3. 长期运行报表：
   - 为任务工作台增加按状态、入口、能力和接管状态聚合的摘要。
   - 为企微最终汇报保留可追踪的报表引用。
4. 多节点一致性：
   - 梳理单节点与多节点部署下的 worker claim、任务恢复、限流判断和通知幂等约束。
   - 为多节点部署补充一致性 metadata 和验收检查项。

## 3. 非目标

- 本轮不引入外部队列系统。
- 本轮不实现跨用户共享任务。
- 本轮不进行真实生产环境部署。
- 本轮不开放任意写操作 capability。

## 4. 验收标准

- 用户可配置 capability 级策略，并影响后续 plan 生成或审批提示。
- Web 可查看 Agent SLA 摘要，至少覆盖成功、失败、耗时、恢复和接管。
- 任务工作台能展示聚合运行报表摘要。
- 多节点一致性检查项进入部署验收 metadata 或 progress phase。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更完整的生产可观测图表、任务成本统计、按用户/能力的配额管理和多节点实际部署验证。

## 6. 本轮实施清单

1. 能力策略：
   - 在用户级 Agent 策略中增加 `capability_policy`，支持 capability key 到 `allow`、`degrade`、`confirm`、`reject` 的映射。
   - 支持精确 key 和 `prefix.*` 两类命中方式。
   - plan 创建后写入 `capability_policy` metadata，并将 `confirm/degrade/reject` 命中转化为审批或拒绝状态。
2. SLA 统计：
   - 在任务工作台响应中增加 SLA 摘要，覆盖计划成功/失败、调度成功/失败、平均耗时、恢复次数、接管次数和通知投递统计。
   - Web 任务工作台展示近期 SLA 摘要。
3. 长期运行报表：
   - 在任务工作台响应中增加按状态、入口、能力和接管状态聚合的报表摘要。
   - Web 展示聚合报表摘要。
4. 多节点一致性：
   - 扩展部署验收 metadata，增加 worker claim、恢复控制、限流判断、通知幂等和节点模式检查。
   - progress phase 展示多节点一致性摘要。
5. 验证：
   - 补充 capability 策略、SLA 摘要、报表聚合和多节点一致性测试。
   - 完整执行 Go 与前端验证命令。

## 7. 实施结果

1. 能力策略：
   - 用户级 Agent 策略新增 `capability_policy` JSON 映射，支持精确 capability key 和 `prefix.*` 规则。
   - 支持 `allow`、`degrade`、`confirm`、`reject`，并对中文或同义写法进行标准化。
   - plan 创建后写入 `capability_policy` metadata，命中 `confirm/degrade` 时进入审批，命中 `reject` 时拒绝计划。
   - 写入 `agent.capability_policy_applied` audit，并将策略命中信息纳入 capability audit metadata。
2. SLA 统计：
   - 任务工作台响应增加 `sla` 摘要，覆盖计划成功/失败、定时任务成功/失败、平均计划耗时、恢复次数、接管次数和通知投递成功/失败。
   - Web 最近任务区域展示 SLA 摘要。
3. 长期运行报表：
   - 任务工作台响应增加 `report` 聚合，覆盖状态、入口、能力和接管状态。
   - Web 最近任务区域展示聚合摘要。
4. 多节点一致性：
   - 部署验收 metadata 增加 worker claim、恢复控制、限流策略、通知幂等和部署模式一致性检查。
   - progress phase 增加 `cluster_consistency`。
5. 测试：
   - 增加 capability 策略默认值与更新、plan 策略应用、SLA/报表统计、多节点一致性 phase 断言。

## 8. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
