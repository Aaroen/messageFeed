# Agent 评测、安全对抗与步骤重试计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent Web/企微入口、统一进度、调度 worker、企微汇报、scope 校验和 Web 控制面已具备的基础上，建立可持续验证 Agent 行为的评测与安全基线，并补齐计划步骤失败后的最小重试策略，使后续能力扩展可以被自动回归验证。

## 2. 实施范围

1. 评测自动运行：
   - 基于已有 `agent_eval_cases`、`agent_eval_runs`、`agent_eval_results` 增加可调用 service。
   - 支持创建一组内置最小 eval cases：企微入口、Web 入口、历史查询、联网查询、调度任务、scope 越权拒绝。
   - 运行后记录 pass/fail/skip、score、actual、evidence refs。
2. 安全对抗样例：
   - 增加 prompt injection、越权 capability、敏感配置泄露、错误目标通道发送等基础用例。
   - 断言系统拒绝或降级，并在 eval result 中保留证据引用。
3. 计划步骤重试策略：
   - 对 executor observation failed 的 plan step 增加最小 retry metadata。
   - 支持按 failure_strategy 判断是否可重试，当前轮实现 service 层可调用重试入口，不要求复杂 UI。
4. Web 与查询面：
   - 新增 eval run 查询 API，返回最近 eval run 与结果。
   - Web 端提供最小 eval 状态视图，便于查看安全回归结果。
5. 测试：
   - 覆盖 eval case 创建、eval run 记录、安全用例判定、step retry 状态流转和前端类型检查。

## 3. 非目标

- 本轮不实现大规模离线评测平台。
- 本轮不引入外部评测服务。
- 本轮不实现复杂多 agent 协作评分。
- 本轮不把 eval UI 做成完整报表系统。

## 4. 验收标准

- 可通过 service/API 创建并运行最小 Agent eval 集。
- scope 越权、敏感信息泄露、错误外发目标等安全用例能产生结构化结果。
- plan step failed 后有可追踪的 retry metadata 和最小重试入口。
- Web 可查看最近 eval run 与结果摘要。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更细粒度实时进度推送、评测指标趋势、任务重试 UI 和更多生产级 capability。

## 6. 实施结果

1. 评测自动运行：
   - 新增内置 Agent eval case 初始化与运行入口。
   - 覆盖企微入口、Web 入口、历史查询、联网查询、调度任务、scope 越权拒绝，以及 prompt injection、越权 capability、敏感配置泄露、错误目标通道发送等安全用例。
   - eval run 记录 `case_count`、`passed_count`、`failed_count`、`metrics`、`completed_at`，eval result 记录 `status`、`score`、`actual`、`failure_reason`、`evidence_refs`。
2. API 与 Web：
   - 新增 `POST /api/v1/agent/eval-runs`、`GET /api/v1/agent/eval-runs`、`GET /api/v1/agent/eval-runs/:id`。
   - Web `/agent` 增加最小评测基线视图，可运行评测并查看最近一次结果摘要和证据引用。
3. 计划步骤重试：
   - 新增 `agent_plan_steps` retry metadata 迁移，包含 `retry_count`、`max_retries`、`last_retry_at`、`retry_reason`、`retry_metadata_json`。
   - 新增 `POST /api/v1/agent/plans/:plan_id/steps/:step_id/retry`。
   - failed step 在 `failure_strategy` 允许且未超过次数时可回到 approved 状态，并写入 `agent.plan_step_retry_queued` audit。
   - Web 步骤列表对可重试 failed step 展示最小重试按钮。

## 7. 验证记录

- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run test` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。
