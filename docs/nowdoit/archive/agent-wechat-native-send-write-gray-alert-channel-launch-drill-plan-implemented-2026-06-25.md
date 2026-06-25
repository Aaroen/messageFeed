# Agent 企微原生发送写能力灰度告警通道与上线演练计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 已具备 Web/企业微信任务入口、实时进度、任务工作台、部署演练、长期趋势、告警策略、企业微信动作 fallback、原生按钮 schema、真实联调准备、写操作最小权限策略和上线前运维验收摘要的基础上，推进企业微信原生按钮消息发送适配、受控写能力灰度、生产级告警通知通道和上线演练记录固化，使 Agent 进一步具备生产上线所需的交互、扩权、告警和演练闭环。

## 2. 实施范围

1. 企业微信原生按钮发送适配：
   - 将现有原生按钮 schema 转换为可发送的企业微信消息 payload。
   - 覆盖查看进度、审批、重试、恢复、取消和查看最终报告。
   - 保留文本 fallback，确保不支持按钮时仍可操作。
2. 受控写能力灰度：
   - 为 `agent.schedule_message` 与 `agent.schedule_task` 建立灰度策略摘要。
   - 默认仍要求审批、预算、权限和审计闭环。
   - 输出灰度状态、允许用户范围、回滚条件和审计信息。
3. 生产级告警通知通道：
   - 为任务失败、通知失败、配额超限、低质量评分和接管待处理建立通知通道摘要。
   - 覆盖 Web、企业微信文本、企业微信按钮 fallback 和 audit。
   - 保持不接入外部告警平台。
4. 上线演练记录固化：
   - 将上线演练从即时摘要推进为可回溯记录。
   - 覆盖演练批次、触发者、检查项、结果、风险、阻断项和下一步动作。

## 3. 非目标

- 本轮不直接执行真实生产发布。
- 本轮不开放任意写操作 capability。
- 本轮不移除文本 fallback。
- 本轮不接入外部 PagerDuty、短信或电话告警系统。

## 4. 验收标准

- Web 或 service 层可以查看企业微信原生发送 payload 摘要。
- 受控写能力灰度策略默认受审批、预算、权限和审计约束。
- 生产级告警通知通道摘要覆盖 Web、企业微信和 audit。
- 上线演练记录具备批次、检查项、结果、风险、阻断项和下一步动作。
- `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check`、`npm --prefix web run build` 通过。

## 5. 本轮实施清单

1. 后端任务工作台响应补充企业微信原生发送 payload 摘要、写能力灰度策略摘要、生产级告警通知通道摘要和上线演练记录摘要。
2. 企业微信原生发送 payload 由现有按钮 schema 生成，覆盖查看进度、审批、重试、恢复、取消和查看最终报告，并保留文本 fallback。
3. 写能力灰度策略以 `agent.schedule_message` 与 `agent.schedule_task` 为候选，默认要求审批、预算、权限和审计闭环，并输出回滚条件。
4. 告警通知通道摘要覆盖 Web、企业微信文本、企业微信按钮 fallback 和 audit，不接入外部告警平台。
5. 上线演练记录摘要输出批次、触发者、检查项、结果、风险、阻断项和下一步动作，并写入 audit。
6. Web API 类型和任务工作台展示新增摘要，补充测试并执行完整验证。

## 6. 实施结果

1. 后端任务工作台响应已补充 `wechat_native_payload`、`write_gray`、`alert_channel`、`launch_drill` 四类结构化摘要。
2. 企业微信原生发送 payload 已由原生按钮 schema 生成，包含 `template_card` 类型、按钮列表、payload 和文本 fallback。
3. 写能力灰度策略已覆盖 `agent.schedule_message` 与 `agent.schedule_task` 候选，默认要求审批、预算和审计，并输出回滚触发条件。
4. 生产级告警通知通道摘要已覆盖 Web 任务工作台、企业微信文本、企业微信按钮 fallback、企业微信原生 payload 和 audit。
5. 上线演练记录已输出批次、触发者、检查项、结果、风险、阻断项和下一步动作。
6. 写能力灰度策略写入 `agent.write_gray_policy_snapshot` audit，告警通道写入 `agent.alert_channel_snapshot` audit，上线演练写入 `agent.launch_drill_record` audit。
7. Web API 类型和任务工作台已展示企业微信原生发送、写能力灰度、告警通道和上线演练摘要。

## 7. 验证记录

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `npm --prefix web run test`：通过。
- `npm --prefix web run type-check`：通过。
- `npm --prefix web run build`：通过。

## 8. 后续衔接

本轮完成后，继续推进真实企业微信按钮消息联调、灰度写操作执行回放、上线审批流和生产运行日报。
