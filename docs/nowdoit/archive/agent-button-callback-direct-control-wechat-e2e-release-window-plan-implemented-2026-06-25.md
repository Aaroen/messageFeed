# Agent 按钮回调直接控制企微端到端验收与发布窗口计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企业微信任务入口、实时进度、企业微信按钮回调动作映射、发布审批执行摘要、写能力审计复盘、日报定时发送摘要和监控告警实测摘要的基础上，推进企业微信按钮回调直接控制执行、企业微信端到端人工验收和生产发布窗口准备，使用户可以从企业微信按钮直接触发查看进度、审批、重试、恢复、取消和最终报告等任务控制动作。

## 2. 实施范围

1. 按钮回调直接控制：
   - 将 `view_progress`、`approval`、`retry_plan`、`recover_plan`、`cancel_scheduled_task` 和 `view_final_report` 从动作映射推进到直接执行或明确的可执行控制入口。
   - 覆盖成功、失败、重复回调和无关联计划场景。
2. 企业微信端到端人工验收：
   - 输出从企微发起任务、查看实时进度、点击按钮控制、接收最终报告的验收摘要。
   - 覆盖文本 fallback 与 Web 端同步查看。
3. 生产发布窗口准备：
   - 输出发布窗口前置检查摘要。
   - 覆盖配置冻结、迁移状态、worker、告警、回滚、审批人、通知和审计。
4. 写能力灰度扩容准备：
   - 对 `agent.schedule_message` 与 `agent.schedule_task` 的现有灰度执行结果进行扩容前复核。
   - 保持默认拒绝其他写 capability。
5. 监控外部对接准备：
   - 明确后续接入外部监控平台所需的指标、告警事件和通知通道映射。

## 3. 非目标

- 本轮不开放未列入白名单的写 capability。
- 本轮不移除企业微信文本 fallback。
- 本轮不强制完成真实生产发布。
- 本轮不把外部监控平台设为运行时强依赖。

## 4. 验收标准

- 企业微信按钮回调可直接触发或明确进入对应任务控制动作，并写入审计记录。
- 企业微信端到端验收摘要覆盖任务发起、进度查看、按钮控制、最终报告和 Web 同步查看。
- 发布窗口准备摘要覆盖配置、迁移、worker、告警、回滚、审批人、通知和审计。
- 写能力灰度扩容准备仍保持最小权限和默认拒绝策略。
- 外部监控对接准备可在 Web 或 service 层查看。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进真实生产发布窗口执行、外部监控平台接入、写能力灰度扩容和企业微信用户验收复盘。

## 6. 本轮实施清单

1. 企业微信按钮回调从动作语义映射推进到直接控制执行或控制请求落库，覆盖 `view_progress`、`approval`、`retry_plan`、`recover_plan`、`cancel_scheduled_task` 和 `view_final_report`。
2. 按钮直接控制覆盖成功、失败、重复回调和无关联计划场景，并写入审计事件。
3. 后端任务工作台响应补充按钮直接控制摘要、企业微信端到端验收摘要、生产发布窗口准备摘要、写能力灰度扩容准备摘要和外部监控对接准备摘要。
4. 企业微信端到端验收摘要覆盖企微发起任务、实时进度查看、按钮控制、最终报告、文本 fallback 和 Web 同步查看。
5. 生产发布窗口准备摘要覆盖配置冻结、迁移状态、worker、告警、回滚、审批人、通知和审计。
6. 写能力灰度扩容准备摘要继续限制在 `agent.schedule_message` 与 `agent.schedule_task`，并保持默认拒绝其他写 capability。
7. 外部监控对接准备摘要覆盖指标、告警事件和通知通道映射。
8. Web API 类型和任务工作台展示新增摘要，补充服务层测试并执行完整验证。

## 7. 实施结果

1. 企业微信按钮回调已从动作映射推进到直接控制处理，`approval` 可批准待审批计划，`retry_plan` 可将失败步骤重新排队，`recover_plan` 可恢复可恢复计划，`cancel_scheduled_task` 可取消关联定时任务，`view_progress` 与 `view_final_report` 直接返回进度或报告入口。
2. 按钮直接控制统一写入 `agent.button_direct_control` 审计事件，审计 metadata 记录动作、处理器、计划、定时任务、控制类型、是否产生状态变更和原始 provider message。
3. 服务层测试已覆盖查看进度、重试失败计划和取消关联定时任务三类企业微信按钮回调路径。
4. 后端任务工作台响应已新增 `button_direct_control`、`wechat_e2e`、`release_window`、`write_gray_expansion` 和 `external_monitor` 五类摘要。
5. 企业微信端到端验收摘要已覆盖企微任务入口、进度查看、按钮控制、最终报告、文本 fallback 和 Web 同步查看。
6. 生产发布窗口准备摘要已覆盖配置冻结、迁移、worker、告警、回滚、审批人、通知和审计。
7. 写能力灰度扩容准备摘要继续限制 `agent.schedule_message` 与 `agent.schedule_task`，并保持默认拒绝或审批策略。
8. 外部监控对接准备摘要已输出指标、告警事件和通知通道映射，并保持外部监控非强依赖。
9. Web API 类型和任务工作台已展示新增摘要，服务层测试已覆盖新增字段和审计快照。

## 8. 验证记录

1. `go test ./internal/service`：通过。
2. `go test ./...`：通过。
3. `go vet ./...`：通过。
4. `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
5. `npm --prefix web run type-check`：通过。
6. `npm --prefix web run build`：通过。
