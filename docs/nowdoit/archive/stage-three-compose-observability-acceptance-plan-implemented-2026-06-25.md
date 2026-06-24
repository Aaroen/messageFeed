# 阶段三 Compose 观测验收执行计划

**最后更新**：2026-06-25

**执行状态**：已完成，2026-06-25 归档。

## 目标

完成 `docs/implementation.md` 中 P0 阶段三收尾事项，对完整 Compose 环境做一次端到端观测验收，确认运行入口、健康状态、日志、指标和 trace 关联能力达到当前阶段要求。

## P0 实施范围

1. 使用完整 Compose 环境启动服务：
   - PostgreSQL。
   - migrate。
   - API。
   - Web。
   - Gateway。
   - Prometheus。
   - Grafana。
   - Loki。
   - Promtail。
   - Tempo。
   - OpenTelemetry Collector。
2. 验证基础入口：
   - `https://localhost:8443/healthz`。
   - `https://localhost:8443/readyz`。
   - `https://localhost:8443/api/runtime/node`。
   - `https://localhost:8443/metrics`。
3. 验证数据库迁移状态：
   - `schema_migrations` 非 dirty。
   - 迁移版本与当前迁移文件一致。
4. 验证请求关联字段：
   - 响应包含 `request_id`。
   - API 日志包含同一 `request_id`。
   - 在启用 trace 时日志可关联 `trace_id`。
5. 验证指标链路：
   - API `/metrics` 可被访问。
   - Prometheus 容器运行。
   - Grafana 容器运行。
6. 验证日志链路：
   - API 输出结构化 JSON 日志。
   - Docker 日志驱动限制仍存在。
   - Loki 与 Promtail 容器运行。
7. 验证 trace 链路：
   - OpenTelemetry Collector 容器运行。
   - Tempo 容器运行。
   - 当前配置关闭 trace 时，文档化关闭状态；如启用则确认 trace 进入 collector/tempo。

## 非目标

- 不重构观测系统架构。
- 不引入新的日志、指标或 trace 产品。
- 不实现生产级告警规则。
- 不新增复杂压测。
- 不变更公网域名、Cloudflare Tunnel 或证书策略。

## 验收标准

- `docker compose ps` 显示完整观测相关容器处于运行状态。
- `/healthz` 返回 `ok`。
- `/readyz` 返回 `ready`，数据库和迁移检查均为 ready。
- `/api/runtime/node` 返回节点、部署模式和公开访问基址。
- `/metrics` 返回 Prometheus 文本指标。
- 任一 API 请求响应中的 `request_id` 可以在 API 容器日志中检索到。
- `schema_migrations` 当前版本等于最新迁移版本，且 `dirty=false`。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run type-check` 通过。

## 实施顺序

1. 核实当前 Compose 配置、日志限制、观测组件配置和运行状态。
2. 启动或重建完整 Compose 环境。
3. 执行健康检查、就绪检查、runtime node 和 metrics 验证。
4. 执行一次带 `request_id` 的 API 请求，并从 API 日志中检索该 request id。
5. 核实数据库迁移版本和 dirty 状态。
6. 核实 Prometheus、Grafana、Loki、Promtail、Tempo、OpenTelemetry Collector 容器状态。
7. 运行后端测试、静态检查和前端类型检查。
8. 将验证结果沉淀到主文档当前进度，并归档本计划。

## 后续衔接

本计划完成后，下一实施包应进入阶段四收尾：源目录健康检查、许可状态、热度和更多过滤维度。

## 执行结果

- `docker compose up -d --build` 已完成，API 与 Web 镜像重建，PostgreSQL、API、Web、Gateway、Prometheus、Grafana、Loki、Promtail、Tempo、OpenTelemetry Collector 均处于运行状态。
- `https://localhost:8443/healthz` 返回 `{"status":"ok"}`。
- `https://localhost:8443/readyz` 返回 `ready`，数据库与迁移检查均为 ready，迁移版本为 `21`。
- `schema_migrations` 为 `version=21`、`dirty=false`，与最新迁移 `000021` 一致。
- `https://localhost:8443/api/runtime/node` 返回 `node_id=docker-api-1`、`deployment_mode=single_node` 和公开访问基址。
- `https://localhost:8443/metrics` 返回 Prometheus 文本指标。
- `GET /api/v1/feed/timeline?limit=1` 响应和 `x-request-id` 均包含 `a8321a58619ec307f3b056365fae061e`，API 容器 JSON 日志中检索到同一 `request_id`。
- 当前 API 配置 `OBSERVABILITY_TRACE_ENABLED=false`，日志中 `trace_id` 与 `span_id` 为空；OpenTelemetry Collector 与 Tempo 容器保持运行，trace 链路处于配置关闭状态。
- Docker 日志驱动为 `json-file`，保留 `max-size=10m` 与 `max-file=3` 限制。
- Prometheus、Grafana、Loki、Tempo 端口健康检查通过；OpenTelemetry Collector 日志显示 OTLP gRPC/HTTP receiver 已启动。
- Web 入口 HTML 使用 `Cache-Control: no-cache`，静态资源使用 `public, max-age=31536000, immutable`；Chromium 渲染检查确认首页包含导航、推荐列表和条目内容。
- `go test ./...`、`go vet ./...`、`npm --prefix web run type-check` 和 `npm --prefix web test` 均通过。
