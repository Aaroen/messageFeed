## messageFeed 微服务化与 Kubernetes 新技术方案

**定位**：当前项目情况与新的技术方案
**更新日期**：2026-07-18
**实施细节文档**：`micr-k8s-implement.md`

本文档只描述当前项目情况、目标技术架构和关键技术决策。具体部署、演练、CI/CD 操作步骤、服务器扩容脚本和故障验证流程，统一放入同级实施文档。

## 当前实施状态（2026-07-18）

当前已经完成 Kubernetes 基线、第 9 节应用运行边界拆分和第 10 节安全资源治理：

1. WSL 内 K3s single-server、动态网络维护和 Helm 工具链已完成。
2. `deploy/helm/messagefeed` Chart 已建立，现有 PostgreSQL、API、Web、Caddy gateway、cloudflared 和观测栈已由 Helm release `messagefeed` 管理。
3. Helm release 当前为 revision 16、状态 `deployed`；API 与四类 worker 均为 1 个 Ready 副本，独立 migrate Job 为 Complete。
4. PostgreSQL 完整恢复演练已通过，5 个现有 PV 均为 `Retain`，PVC/PV 绑定关系保持不变。
5. `local-path-retain` 已是唯一默认 StorageClass；Chart `0.3.0` 已建立独立运行身份、默认拒绝 NetworkPolicy、ResourceQuota、LimitRange、PDB 和调度约束。

当前尚未完成：

1. Web、Gateway、cloudflared 多副本及入口故障演练。
2. 数据库 expand/contract 兼容迁移与 CI/CD 发布回滚闭环。
3. 真实微服务拆分。

环境与资产治理状态：

1. `local-path=false`、`local-path-retain=true`，新 PVC 使用唯一默认类，现有 PVC/PV 不迁移。
2. API、四类 worker 和 migrate 使用 `messagefeed-api:role9-20260718-8a454cb690ec`，业务 Pod PID 1 均为 `tini`；cloudflared 固定为 `2026.6.1`。
3. PostgreSQL 恢复库的数据、迁移、pgvector、索引和约束核验通过，公网健康检查通过。
4. 六个应用角色使用独立零权限 ServiceAccount，19 条 NetworkPolicy 按角色放行，资源配额和 14 个 PDB 已通过故障验收。

上述状态是当前事实；后文的入口高可用、CI/CD 和真实微服务拆分仍属于后续方案。

## 1. 当前项目情况

`messageFeed` 当前不是微服务架构，而是模块化单体。

当前运行形态：

```text
同一 messagefeed 二进制
  -> APP_ROLE=api：HTTP API
  -> APP_ROLE=source-worker：RSS/Feed 抓取
  -> APP_ROLE=notification-worker：通知发送
  -> APP_ROLE=agent-scheduler-worker：Agent 定时任务
  -> APP_ROLE=embedding-worker：Embedding
  -> APP_ROLE=migrate：数据库迁移
```

当前已有基础：

| 能力 | 当前情况 |
| --- | --- |
| 后端架构 | Go 单二进制，内部按 `handler -> service -> repository` 分层 |
| 前端 | Vue 3 + Vite 独立前端，当前由静态服务承载 |
| 数据库 | PostgreSQL + pgvector |
| 后台任务 | 抓取、通知、Agent 定时任务、Embedding 由独立运行角色启动，共用一个后端镜像 |
| 容器化 | 已有 `Dockerfile`、`docker-compose.yml` 和多角色 Helm Chart；当前 K3s 由 Helm 管理 |
| 当前入口 | Cloudflare Tunnel + Caddy gateway |
| 可观测性 | 已有 Prometheus、Loki、Tempo、OpenTelemetry、Grafana 设计基础 |
| 健康检查 | 已有 `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node` |
| 分布式基础 | 已有 PostgreSQL 任务锁、job claim、通知幂等、Agent trace/memory/embedding 表 |

当前主要问题：

1. 仍为单二进制多运行角色，尚未形成独立业务代码和数据边界。
2. API、worker 和 migrate 已有独立生命周期、安全身份、网络与资源边界，但仍共用单二进制和数据库。
3. Cloudflare Tunnel 当前存在入口单点或弱高可用风险，偶发 `1033`、`502/504` 时难以定位。
4. 已建立并接管多角色 Helm Chart，API、四类 worker 和独立 migrate Job 已完成。
5. CI/CD、入口多副本和数据库兼容回滚策略还没有形成完整发布闭环。
6. 真正业务微服务边界尚未成熟，直接拆服务会引入认证、接口、数据一致性和链路追踪复杂度。

## 2. 总体技术方案

新的技术方案采用“三步走”：

```text
第一步：单体代码多运行角色
第二步：Kubernetes 分布式部署与高可用入口
第三步：稳定后再拆业务微服务
```

第一阶段不直接拆成多个业务微服务，而是先把当前单二进制拆成多个运行角色。以下角色已在当前集群落地：

```text
api
source-worker
notification-worker
agent-scheduler-worker
embedding-worker
migrate
```

采用原因：

1. 保留现有业务代码和数据模型，降低重构风险。
2. 先解决部署、扩容、入口高可用、发布回滚和任务隔离问题。
3. 让 API、worker、Web、Tunnel、gateway 能独立扩缩容和独立观测。
4. 为后续微服务拆分提前形成清晰运行边界。

当前已落地基线：

```text
Windows
  -> WSL 内 K3s single-server
  -> Helm release messagefeed
  -> PostgreSQL/pgvector
  -> API / 四类 worker / Web / Caddy gateway / cloudflared
  -> Prometheus / Loki / Tempo / OTel Collector / Grafana / Promtail
```

下一阶段目标：完成迁移兼容策略、入口高可用和 CI/CD 闭环，再进入真实业务微服务拆分。

统一连接方式：

```text
ssh aroen@127.0.0.1
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
```

采用原因：

1. 当前项目源码、Dockerfile、docker-compose、Caddy、观测配置和迁移文件都已经位于 WSL 项目目录内。
2. 通过 SSH 进入 WSL 后，后续命令、脚本和 CI/CD 行为可以与真实 Linux 服务器保持一致。
3. 先在 WSL 内完成 K3s、Helm、角色拆分、Tunnel 和发布闭环，可以降低直接跨机器调度带来的复杂度。
4. 后续服务器扩容时，只需要把已验证的 K3s agent 加入、节点标签、亲和性和副本策略扩展到新节点。

## 3. 目标技术架构

目标架构（完成角色化后）：

```text
用户 / 企业微信
  -> Cloudflare
  -> Cloudflare Tunnel
  -> cloudflared Pods
  -> Caddy gateway
  -> web Service / api Service

后台任务：
  source-worker
  notification-worker
  agent-scheduler-worker
  embedding-worker
  -> PostgreSQL/pgvector

观测：
  Prometheus
  Loki
  Tempo
  OpenTelemetry Collector
  Grafana
```

当前实际运行形态已与该多角色架构一致；API 与四类 worker 默认各 1 副本，migrate 由独立 Helm Job 执行。

核心组件职责：

| 组件 | 角色 |
| --- | --- |
| `cloudflared` | 保持 Cloudflare Tunnel 出站连接，作为外部入口连接器 |
| `Caddy gateway` | 集群内部 HTTP 路由，分发到 Web/API |
| `web` | 静态前端服务 |
| `api` | HTTP API、企业微信 callback、认证、用户请求主链路 |
| `source-worker` | Feed 抓取、解析、入库、重试 |
| `notification-worker` | 通知任务发送、delivery 记录 |
| `agent-scheduler-worker` | Agent 定时任务、任务恢复、最终报告 |
| `embedding-worker` | 记忆和事实 embedding job |
| `migrate` | 数据库迁移 Job |
| `PostgreSQL/pgvector` | 主数据、任务、Agent trace/memory/embedding 存储 |

## 4. Kubernetes 方案

当前阶段采用本机 WSL 内 K3s 长期运行方案。

| 场景 | 方案 |
| --- | --- |
| 当前落地基线 | WSL 内运行 K3s single-server 集群 |
| 本机一次性演练 | 可选 K3d，但不作为长期运行基线 |
| 后续服务器加入 | 作为 K3s agent 节点加入 |
| 资源编排 | Helm |

当前 WSL 基线：

```text
Windows
  -> WSL
  -> K3s server / control-plane
  -> PostgreSQL/pgvector
  -> Caddy gateway
  -> cloudflared
  -> API / worker / web Pods
  -> Prometheus / Loki / Tempo / OTel Collector / Grafana / Promtail
```

当前已部署 `source-worker`、`notification-worker`、`agent-scheduler-worker`、`embedding-worker` 和 `migrate` Job。

采用原因：

1. 当前源码和运行资产都在 WSL 项目目录内，先在 WSL 内闭环最直接。
2. WSL 内 K3s 与后续 Linux 服务器 K3s 的运行模型一致，比只用 Docker Compose 更接近最终形态。
3. K3s 轻量，适合单机长期运行，也便于后续把实验室服务器和低配服务器作为 agent 节点接入。
4. Helm 能统一管理多角色、多环境、Secret/ConfigMap 和镜像 tag。

本阶段定位：

```text
先完成本机 WSL 长期运行和应用级高可用验证，不承诺本机断电、Windows 关机或 WSL 停止后的持续在线。
```

采用原因：

1. WSL 随 Windows 生命周期运行，本阶段目标是把项目先改造成可被 K8s 管理的形态。
2. 单机 K3s 可以验证 Pod 故障、自恢复、滚动发布、Service 负载均衡和 Tunnel 多副本。
3. 完成本机闭环后，再把同一套 Helm、镜像和角色配置扩展到实验室服务器和低配服务器。

后续扩展形态：

```text
WSL 本机：K3s server 或主力节点
实验室服务器：K3s agent / 高配备份计算节点 / 严格安全隔离
低配常驻服务器：K3s agent 或后续 control-plane 迁移目标 / 持续在线兜底节点
```

后续扩展原则：

1. 当前先不把 control-plane 迁移到远程服务器，避免在基础改造前引入跨机器网络和权限复杂度。
2. 实验室服务器后续作为受限 worker 节点接入，不默认作为公网入口。
3. 低配服务器后续用于常驻兜底时，可以作为 agent 节点接入；若后续要求 WSL 关机后服务持续运行，再单独规划 control-plane 和数据库迁移。
4. 后续节点调度仍通过节点标签、亲和性、污点/容忍和副本分层实现。

## 5. 运行角色方案（已落地）

后端第一阶段保持一个镜像，通过 `APP_ROLE` 控制运行职责。

当前已实现 `APP_ROLE`；cluster 模式禁止隐式使用 `all`，API 和四类 worker 可独立扩缩容。

| `APP_ROLE` | 职责 | 形态 |
| --- | --- | --- |
| `api` | HTTP API、企业微信 callback、健康检查、指标 | Deployment |
| `source-worker` | Feed 抓取任务 | Deployment |
| `notification-worker` | 通知发送任务 | Deployment |
| `agent-scheduler-worker` | Agent 定时任务 | Deployment |
| `embedding-worker` | Embedding job | Deployment |
| `migrate` | 数据库迁移 | Job |
| `all` | 本地兼容模式 | 仅开发或过渡期 |

采用原因：

1. API 可以多副本扩容，不会重复启动 worker。
2. worker 可以按任务类型独立扩缩容。
3. worker 故障不会直接影响 Web/API 主链路。
4. 后续拆业务服务时，运行边界已经提前成型。

## 6. 外部访问方案

第一阶段继续使用 Cloudflare Tunnel，不直接开放公网 NodePort、LoadBalancer 或服务器 80/443 入站端口。

当前 cloudflared 已纳入 Helm 管理并固定使用 HTTP/2，当前副本数仍为 1；多副本是后续高可用目标。

访问链路：

```text
用户 / 企业微信
  -> Cloudflare
  -> Cloudflare Tunnel
  -> cloudflared
  -> Caddy gateway / api Service
  -> web / api
```

采用原因：

1. 保留当前已有 Cloudflare Tunnel 访问方式，迁移成本低。
2. 服务器无需开放公网入站端口，安全边界更小。
3. `cloudflared` 可以在 Kubernetes 内多副本运行，提高入口可用性。
4. Caddy 已在当前项目中使用，第一阶段继续作为内部 gateway 更稳。

当前阶段入口策略：

```text
用户 / 企业微信
  -> Cloudflare
  -> Cloudflare Tunnel
  -> WSL 内 cloudflared Pod
  -> WSL 内 Caddy gateway
  -> WSL 内 web / api Service
```

采用原因：

1. 保留当前已有 Cloudflare Tunnel 访问方式，迁移成本低。
2. 本阶段不开放 Windows/WSL 公网入站端口，入口仍由 Tunnel 出站连接承载。
3. `cloudflared`、gateway、api、web 都先纳入 WSL 内 K3s 管理，便于验证滚动发布和多副本行为。
4. Caddy 已在当前项目中使用，第一阶段继续作为内部 gateway 更稳。

后续三节点入口演进：

| 形态 | 方案 | 适用情况 |
| --- | --- | --- |
| WSL 单入口 | 只有 WSL 内 `cloudflared` 承载入口 | 当前阶段采用 |
| 严格安全扩展 | 低配常驻服务器承载公网入口，实验室服务器只作为内部 worker 节点 | 后续要求 WSL 关机后仍可访问，且实验室服务器安全要求最高时采用 |
| 优先级入口扩展 | WSL、实验室服务器、低配服务器分别运行 Tunnel 连接器，通过 Cloudflare 健康检查做优先级切换 | 后续明确要求外部流量也按 WSL > 实验室 > 低配服务器 切换时采用 |

当前阶段只实现 WSL 单入口。WSL 关闭、Windows 关机或本机网络中断时，服务不可用属于当前阶段已知边界；后续持续在线能力通过接入低配常驻服务器和实验室服务器解决。

暂不引入 Nginx Ingress 的原因：

1. Nginx 不是必须项。
2. Kubernetes Ingress/Gateway API 都需要额外 controller，不是自带网关。
3. 当前路由需求主要是 Web/API 分发，Caddy 已能满足。
4. 网关复杂化应等服务数量和路由策略复杂后再推进。

后续可演进方向：

```text
Cloudflare Tunnel
  -> Gateway API
  -> Traefik / Envoy Gateway / Caddy Gateway
  -> 多业务服务
```

## 7. Tunnel 稳定性方案

本节描述目标高可用方案。当前集群已完成单副本 cloudflared、gateway 和独立 API 的入口验收，尚未完成入口多副本故障演练。

当前 Tunnel 偶发 `1033` 或网关错误时，新的方案把入口链路从单点升级为多副本。

目标链路：

```text
多节点 cloudflared
  -> 多 gateway
  -> 多 api/web
```

技术决策：

| 问题 | 方案 |
| --- | --- |
| `cloudflared` 进程单点 | WSL 内 `cloudflared` 至少 2 副本 |
| gateway/API 单点 | WSL 内 Caddy gateway、api、web 均多副本 |
| 单 Pod 故障 | 通过 Deployment 自恢复、readiness 和 Service endpoint 切换处理 |
| WSL 关机 | 当前阶段不承诺持续在线，后续由低配常驻服务器和实验室服务器扩展解决 |
| QUIC/UDP 网络不稳 | 保留 `auto/quic/http2` 协议切换能力 |
| 1033/502 难定位 | 增加外部探测、内部探针、metrics 和日志关联 |

采用原因：

1. `1033` 通常表示 Cloudflare 找不到健康的 `cloudflared` 连接。
2. 多副本 `cloudflared` 可以降低单连接器故障导致的入口中断。
3. 多 gateway、多 api/web 可以降低内部 Pod 重启导致的 `502/504`。
4. 探针和指标用于证明修复有效，并定位故障属于 Cloudflare、Tunnel、gateway、API 还是数据库。

后续如果要实现外部流量优先级切换，采用独立 Tunnel 或 Cloudflare Load Balancer 的健康检查策略：

```text
primary：WSL Tunnel
secondary：实验室服务器 Tunnel
fallback：低配常驻服务器 Tunnel
```

该模式只在实验室服务器的安全策略允许出站 Tunnel 且允许 Cloudflare 健康检查后启用。否则后续保持低配服务器统一入口，实验室服务器只承载内部计算。

## 8. 数据库方案

数据库继续使用 PostgreSQL + pgvector。

当前阶段数据库主实例放在 WSL 内 K3s 中，或作为 WSL 内本地 PostgreSQL 服务被 K3s workload 访问。后续如果要求 WSL 关机后服务持续运行，再把 PostgreSQL/pgvector 迁移到低配常驻服务器或托管 PostgreSQL。

当前由独立 Helm pre-install/pre-upgrade migrate Job 执行迁移，API 不再包含 migration init container。现有 5 个 PV 已设置为 `Retain`，`local-path-retain` 已是唯一默认 StorageClass，完整恢复演练已通过。

第一阶段不把数据库复杂高可用作为主目标，而是优先保证：

1. 连接配置可在 Kubernetes Secret 中管理。
2. 迁移通过独立 Job 执行。
3. 备份和恢复策略明确。
4. 数据库迁移遵循向后兼容原则。
5. 数据库备份可以加密同步到 Windows 宿主机、实验室服务器或低配服务器，但备份文件不等同于可直接写入的主库。

采用原因：

1. PostgreSQL/pgvector 是当前系统主数据和 Agent 记忆/Embedding 的核心依赖。
2. 应用部署改造和数据库高可用不宜在第一阶段叠加。
3. 对个人项目和小型集群而言，先保证备份、恢复和兼容迁移比立即搭建数据库 HA 更重要。
4. 当前先以 WSL 长期运行为目标，数据库与应用部署在同一 K3s 环境内，能最小化跨机器网络和权限复杂度。

## 9. CI/CD 技术方案

第一阶段采用 GitHub Actions + 镜像仓库 + Helm。

目标链路：

```text
PR 校验
  -> 构建镜像
  -> 推送 Git SHA tag
  -> 部署 staging
  -> smoke test
  -> 人工审批
  -> 部署 production
  -> 观察指标
```

采用原因：

1. PR 阶段阻断测试、构建和类型错误。
2. Git SHA 镜像 tag 保证每次发布可追踪、可回滚。
3. staging 先验证镜像、配置、数据库迁移和基本链路。
4. production 保留人工审批，避免自动误发布。
5. Helm 可统一管理多角色 Deployment、Job、Secret、ConfigMap 和滚动升级。

当前阶段发布策略：

1. CI/CD 或手动发布均面向 WSL 内 K3s control-plane。
2. 本地操作统一通过 `ssh aroen@127.0.0.1` 进入 WSL 后执行。
3. 当前 Helm values 使用 `values.yaml` 与 `values-k3s.yaml`，描述 WSL 单节点长期运行的副本数、资源限制、Secret 引用和入口配置。
4. 后续接入服务器时，再新增 `values-lab.yaml`、`values-vps.yaml` 或多节点 values，不推翻当前 Helm 结构。

## 10. 无感升级方案

无感升级依赖：

1. 多副本。
2. readiness/liveness/startup 探针。
3. 滚动发布。
4. 向后兼容数据库迁移。
5. 可回滚镜像 tag。

当前多角色阶段已完成 Helm 接管和基础链路验收；API/source worker 双副本、worker 独立升级、独立迁移和 Helm rollback 已验证，Web/Gateway/cloudflared 仍为单副本。

技术决策：

| 事项 | 方案 |
| --- | --- |
| API/Web/Gateway 发布 | RollingUpdate |
| 最大不可用 | `maxUnavailable=0` |
| 最大增量 | `maxSurge=1` |
| API readiness | `/readyz` |
| API liveness | `/healthz` |
| 数据库迁移 | expand/contract |
| 应用回滚 | Helm rollback 或镜像 tag 回退 |

采用原因：

1. 新 Pod 未 ready 前旧 Pod 继续接流量。
2. readiness 失败的 Pod 不进入 Service endpoints。
3. 兼容式迁移保证应用版本可回滚。
4. worker 迁移时可避免新旧环境双跑导致重复通知或重复抓取。

当前阶段无感升级要求：

1. API 和 worker 已使用 RollingUpdate 并完成双副本演练；Web/Gateway/cloudflared 多副本仍是后续目标。
2. readiness 失败的 Pod 不进入 Service endpoints，旧 Pod 在新 Pod ready 前继续接流量。
3. worker 必须依赖数据库任务锁、job claim 和幂等机制，保证同一 WSL 集群内多副本不会重复处理同一任务。
4. Windows 关机、WSL 停止、本机断网属于当前阶段不可无感覆盖的故障，后续通过远程服务器扩展解决。

## 11. 后续微服务边界

业务微服务拆分放在 Kubernetes 多角色部署稳定之后。

推荐拆分方向：

| 服务 | 拆分优先级 | 原因 |
| --- | --- | --- |
| `notification-service` | 高 | 边界清晰，副作用可幂等 |
| `feed-worker-service` | 高 | 抓取链路适合独立扩缩容 |
| `embedding-service` | 中 | 模型调用成本高，适合独立限流 |
| `agent-worker-service` | 中 | 长任务多，需要独立治理 |
| `feed-api-service` | 中低 | 用户主链路，需更谨慎 |
| `auth-service` | 低 | 权限影响全系统，最后拆 |
| `market-service` | 新能力时独立建设 | 金融能力尚未落地，适合作为新服务设计 |

采用原因：

1. 先拆后台副作用清晰的服务，降低跨服务调用风险。
2. 用户主链路和认证链路稳定性要求更高，不宜过早拆。
3. 金融能力属于后续新增业务，可按微服务形态单独建设。
