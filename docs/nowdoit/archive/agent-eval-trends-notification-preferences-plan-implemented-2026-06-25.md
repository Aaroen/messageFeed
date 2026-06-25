# Agent Eval 趋势、通知偏好与汇报治理计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 入口、计划审批、执行进度、SSE 推送、定时任务、恢复、重试和证据引用已具备的基础上，补齐持续评测趋势、用户通知偏好和最终汇报治理，使 Web 与企微两端能够更稳定地呈现 Agent 工作质量、通知范围和任务结果。

## 2. 实施范围

1. Eval 趋势：
   - 增加 eval run 列表的趋势统计，包括通过率、失败数、最近运行时间和失败类型摘要。
   - Web 展示最近 eval 趋势，支持从趋势项进入 eval run 明细。
   - 为 eval 结果补充更明确的 evidence refs 展示。
2. 通知偏好：
   - 设计最小用户级 Agent 通知偏好，包括过程通知、最终汇报、失败通知和恢复通知开关。
   - Web 提供偏好读取与保存入口。
   - 企微过程通知和最终汇报读取偏好，避免不必要的外发。
3. 汇报治理：
   - 统一 Web 与企微最终汇报摘要格式，包含任务状态、关键步骤、失败原因和证据引用。
   - 对敏感字段进行输出过滤，避免配置、token、secret 等信息进入最终汇报。
   - 为失败、取消、恢复后成功等场景提供明确汇报文案。
4. 进度事件增强：
   - 将恢复、批量重试、通知发送等 audit 事件纳入可查询进度细节。
   - Web 最近事件区区分系统事件、用户操作和能力输出。
5. 测试：
   - 覆盖 eval 趋势统计、通知偏好读写、偏好影响外发、汇报过滤和进度事件增强。

## 3. 非目标

- 本轮不实现复杂通知规则引擎。
- 本轮不接入新的外部消息队列。
- 本轮不实现多 agent 协作或跨用户共享评测报表。
- 本轮不替换现有 eval oracle，只增强趋势和展示。

## 4. 验收标准

- Web 可查看 eval 趋势摘要和最近 eval run 明细。
- 用户可配置 Agent 通知偏好，企微通知路径遵守该偏好。
- 最终汇报在 Web 与企微中具有一致摘要结构，并包含安全过滤后的证据引用。
- 恢复、批量重试和通知发送事件可在进度细节中被识别。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 后续衔接

本轮完成后，继续推进更丰富的生产级 capability、多轮任务编排、长期记忆治理和复杂任务拆解质量评测。

## 6. 实施结果

1. Eval 趋势：
   - `GET /api/v1/agent/eval-runs` 返回 `trend` 摘要，包括运行数、完成数、失败运行数、case 总数、通过数、失败结果数、通过率、最近运行时间和失败摘要。
   - Web Agent 页面展示 eval 趋势摘要，并保留最近 eval run 明细和 evidence refs 展示。
2. 通知偏好：
   - 新增 `agent_notification_preferences` 持久化表，默认过程通知、最终汇报、失败通知和恢复通知均开启。
   - 新增 `GET /api/v1/agent/notification-preferences` 和 `PATCH /api/v1/agent/notification-preferences`。
   - Web 设置页偏好区域新增 Agent 通知开关。
   - 企微过程通知、失败通知、最终回复和定时任务最终汇报读取用户偏好；未配置偏好时保持默认发送。
3. 汇报治理：
   - 新增统一汇报过滤 helper，对 `api_key`、`secret`、`token`、`password`、`database_url`、`dsn`、`Bearer` 等敏感片段进行 redaction。
   - 定时任务最终汇报和 Agent 最终回复在写入/外发前应用过滤。
   - 定时任务最终汇报继续包含 task/plan/turn/run 证据引用摘要。
4. 进度事件增强：
   - 新增 audit log 查询能力，将恢复、批量重试、通知发送、取消等 audit 记录纳入 progress recent events。
   - progress event 新增 `source` 字段，用于区分 `user_action`、`notification`、`capability` 和 `system`。
   - Web 最近事件区展示 audit 事件来源。
5. 测试：
   - 覆盖 eval 趋势统计、通知偏好默认值与更新、偏好禁用最终汇报、敏感字段过滤、audit 进度事件和 handler 路由。

## 7. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过，3 个测试文件、9 个测试通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。
