# Agent 工作流恢复质量评分与部署验收计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企微任务入口、实时进度、审批控制、多轮编排、结果复用、长期记忆治理、权限预算治理和工作台可见性的基础上，推进更完整的任务工作流恢复、结果质量评分、跨设备通知一致性和生产部署验收，使 Agent 在中断、失败、重试、恢复和最终汇报场景中具备更稳定的闭环能力。

## 2. 实施范围

1. 工作流恢复：
   - 对 executing plan、executing step、queued scheduled task 和 interrupted run 建立更明确的恢复策略。
   - 在恢复时区分可重试失败、可恢复中断、需要重新审批和不可恢复状态。
   - 将恢复动作、恢复原因、恢复结果写入 audit，并在 progress recent events 中可见。
2. 结果质量评分：
   - 为 plan result、step output、artifact refs 和 final report 增加质量评分摘要。
   - 评分维度包括证据完整性、来源新鲜度、目标覆盖、风险提示和用户可读性。
   - eval 增加结果质量案例，并将质量评分进入 eval trend。
3. 跨设备通知一致性：
   - Web 与企微展示同一 progress snapshot 的关键状态、下一步动作、权限预算摘要和 evidence refs。
   - 企微最终汇报与 Web 详情中的结果摘要保持字段一致。
   - 通知偏好继续约束过程通知、失败通知、恢复通知和最终汇报。
4. 生产部署验收：
   - 梳理 Agent 关键路径的健康检查、迁移依赖和 worker 启动前置条件。
   - 为 Web 任务、企微任务、定时任务、恢复任务和 eval 建立部署验收检查项。
   - 保持现有 Docker 与数据库配置不做破坏性调整。

## 3. 非目标

- 本轮不引入外部队列或新工作流引擎。
- 本轮不实现跨用户共享任务或共享记忆。
- 本轮不开放任意写操作 capability。
- 本轮不进行生产环境真实部署。

## 4. 验收标准

- 可恢复任务能明确给出恢复策略、恢复结果和 audit 记录。
- plan result 或 final report 能展示质量评分摘要和证据完整性。
- Web 与企微对同一任务展示一致的关键状态、下一步动作、权限预算摘要和 evidence refs。
- 部署验收检查项覆盖 Web 入口、企微入口、定时 worker、恢复控制和 eval。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进生产运行观测、任务吞吐限制、人工接管流程和更细粒度的用户级策略配置。

## 6. 本轮实施清单

1. 恢复治理：
   - 为 plan 和 scheduled task 恢复动作写入 `recovery_strategy`、`recovery_reason`、`recovery_result`、`requires_reapproval` 和恢复对象数量。
   - 将恢复策略纳入 audit metadata，使 recent events 可展示同一恢复事实。
2. 结果质量评分：
   - 在 plan 完成或失败后生成 `result_quality` metadata。
   - 评分覆盖证据完整性、来源新鲜度、目标覆盖、风险提示和可读性，并保留 evidence refs。
3. 跨设备一致性：
   - 扩展 progress phase 和文本摘要，使 Web 与企微复用权限、预算、质量、证据和下一步字段。
   - Web 计划详情展示质量评分、恢复策略和部署验收检查项。
4. 部署验收：
   - 为 Web 入口、企微入口、定时 worker、恢复控制、eval、健康检查和迁移依赖建立 `deployment_acceptance` metadata。
5. 评测与验证：
   - 增加结果质量、恢复策略和跨设备一致性 eval case。
   - 补充恢复 metadata、质量 metadata、progress phase 的单元测试。

## 7. 实施结果

1. 恢复治理：
   - `RecoverPlan` 写入 `recovery` plan metadata，并在 audit metadata 中记录 `recovery_strategy`、`recovery_reason`、`recovery_result`、`requires_reapproval` 和 `recovered_steps`。
   - `RecoverScheduledTask` 在 audit metadata 中记录任务恢复策略、恢复原因、恢复结果、原锁信息和原错误状态。
2. 结果质量评分：
   - plan 完成或失败后写入 `result_quality` metadata。
   - 评分包含 `score`、`evidence_completeness`、`freshness`、`goal_coverage`、`risk_notice`、`readability` 和 `evidence_refs`。
3. 跨设备一致性：
   - `AgentProgressTextSummary` 在包含 plan 的快照中输出权限、预算、质量和证据引用。
   - 企微最终汇报增加质量评分和部署验收摘要。
   - Web 计划详情增加结果质量、恢复策略、部署验收和验收检查项展示。
4. 生产部署验收：
   - plan metadata 增加 `deployment_acceptance`，覆盖 Web 入口、企微入口、定时 worker、恢复控制、eval、healthz 和 migrations。
   - progress phase 增加 `quality`、`deployment_acceptance`，并在存在恢复记录时展示 `recovery`。
5. 评测：
   - 内置 eval case 增加 `result_quality_summary`、`workflow_recovery_strategy`、`cross_device_progress_consistency`。

## 8. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
