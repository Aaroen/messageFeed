# Agent 工作流治理模块化质量计划

**创建日期**：2026-06-25

## 1. 本轮目标

在 Agent 双端任务闭环摘要持续扩展后，优先治理后端 `agent_workflow_governance.go` 和前端 `AgentPlanView.vue` 的单文件职责膨胀问题，降低继续推进企业微信真实试运行、Web 证据操作 API、回放结果查询和恢复策略自动化执行时的维护风险。

## 2. 实施范围

1. 后端工作流治理拆分：
   - 将企业微信、Web 证据、回放、恢复和双端闭环相关 builder 从宽文件中拆分到边界明确的治理文件。
   - 保持现有对外响应结构、JSON 字段和测试行为不变。
2. 后端公共判定收敛：
   - 识别重复的状态判定、审计事件检查和 checks 汇总逻辑。
   - 提取小型 helper，避免继续复制相似代码。
3. 前端任务工作台结构治理：
   - 将重复摘要展示逻辑收敛为本地可复用数据结构或渲染循环。
   - 保持当前页面功能、文案和路由行为不变。
4. 验证与风险控制：
   - 每个拆分小步后执行对应验证。
   - 避免改变接口契约和用户可见行为。

## 2.1 本轮实施清单

1. 统计并确认当前后端治理文件中各业务域函数分布。
2. 拆分企业微信模板、Web 证据、回放、恢复和双端闭环 builder 到独立后端文件。
3. 提取审计事件和 checks 相关公共 helper。
4. 运行 `go test ./internal/service` 和 `go test ./...` 验证后端拆分无行为回归。
5. 梳理 `AgentPlanView.vue` 中摘要展示重复结构，完成低风险收敛。
6. 运行 `npm --prefix web run type-check`、`npm --prefix web run test` 和 `npm --prefix web run build` 验证前端无行为回归。
7. 记录实施结果和验证记录，归档本文档并创建下一轮真实链路自动化执行计划。

## 3. 非目标

- 本轮不新增新的 Agent 能力字段。
- 本轮不改变后端 API 响应字段、JSON 名称或语义。
- 本轮不改变 Web 页面路由、交互入口或用户可见业务文案。
- 本轮不删除既有能力实现。

## 4. 验收标准

- `agent_workflow_governance.go` 不再继续承载本轮相关双端闭环 builder 的全部实现。
- 拆分后文件边界能够按企业微信、Web 证据、回放、恢复和双端闭环进行定位。
- 后端服务层和全量 Go 测试通过。
- 前端类型检查、测试和构建通过。
- Web 任务工作台仍可展示既有治理摘要和本轮新增闭环摘要。

## 5. 后续衔接

本轮完成后，继续推进企业微信模板试运行指标落库、证据页面实际操作 API、回放执行结果查询 API 和恢复策略自动化执行。

## 6. 实施结果

1. 后端已将双端任务闭环推进 builder 拆分至 `internal/service/agent_dual_end_task_closure_governance.go`。
2. 后端已将双端运行闭环 builder 拆分至 `internal/service/agent_dual_end_run_loop_governance.go`。
3. 后端已新增 `internal/service/agent_governance_checks.go`，用于收敛 `AgentDeploymentCheckResponse` 的通用构造逻辑。
4. `internal/service/agent_workflow_governance.go` 已从 4921 行降至 4588 行。
5. 前端 `AgentPlanView.vue` 已将本轮新增五个闭环摘要展示块收敛为 `taskClosureSummaryItems` 统一渲染数组，减少重复模板结构。
6. 本轮保持后端 API 字段、JSON 名称、服务层行为和 Web 可见业务文案不变。

## 7. 验证记录

1. `go test ./internal/service`：通过。
2. `go test ./...`：通过。
3. `go vet ./...`：通过。
4. `npm --prefix web run test`：通过，3 个测试文件、9 个测试用例通过。
5. `npm --prefix web run type-check`：通过。
6. `npm --prefix web run build`：通过。

## 8. 归档结论

本轮已完成后端双端闭环 builder 的第一阶段模块化、公共 checks helper 提取和前端新增摘要渲染收敛。后续仍应继续拆分更早期的企业微信、Web 证据、回放和恢复治理函数，但当前结构已能支撑下一轮推进真实链路自动化执行。
