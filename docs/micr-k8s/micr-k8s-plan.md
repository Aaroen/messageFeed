## messageFeed 微服务化与 Kubernetes 新技术方案

**定位**：当前项目情况与新的技术方案
**更新日期**：2026-08-06
**实施细节文档**：`micr-k8s-implement.md`
**最新演进基线**：`../nowdoit/messagefeed-backend-evolution-review-plan-20260805.md`

本文档只描述当前项目情况、目标技术架构和关键技术决策。具体部署、演练、CI/CD 操作步骤、服务器扩容脚本和故障验证流程，统一放入同级实施文档。

## 当前实施状态（2026-08-06）

当前已经完成 Kubernetes 基线、P0 任务可靠性闭环、P1 Agent 持久化任务队列与独立 worker、第 9 节应用运行边界拆分、第 10 节安全资源治理、第 11 节迁移高可用与回滚，以及第一个 notification 微服务的生产拆分：

1. WSL 内 K3s single-server、动态网络维护和 Helm 工具链已完成。
2. `deploy/helm/messagefeed` Chart 已建立，现有 PostgreSQL、API、Web、Caddy gateway、cloudflared 和观测栈已由 Helm release `messagefeed` 管理。
3. 当前单节点 Helm release 为 revision 36、状态 `deployed`；API、Web、Gateway 各为 2 个 Ready 副本，cloudflared 为 3 个 Ready 连接器，六类 worker 各为 1 个 Ready 副本，独立 migrate Job 为 Complete。
4. PostgreSQL 完整恢复演练已通过，5 个现有 PV 均为 `Retain`，PVC/PV 绑定关系保持不变。
5. Chart `0.4.0` 已建立独立运行身份、默认拒绝 NetworkPolicy、资源治理、迁移门禁和入口多副本策略；当前复核发现 `local-path` 与 `local-path-retain` 均带 default 标记，唯一默认类仍需在 110 加入前修正。

当前尚未完成：

1. GitHub Actions 的首次 GHCR/staging 实际运行、人工审批和持续发布观察闭环；工作流代码已建立，尚未注册自托管 Runner。
2. 第二个真实微服务拆分；`notification-service` 已完成第一轮生产拆分。
3. P2 双节点迁移：目标调整为实体 Linux `100.106.96.110` 承载持续在线的 K3s server、PostgreSQL 和热备用应用，本机 WSL `100.78.141.120` 迁移为 K3s agent 和主应用节点；当前仍是 WSL single-server，迁移尚未执行。
4. Cloudflare 主备入口：保留现有 WSL Tunnel，新增独立 `messageFeed_fallback` Tunnel，通过 Public Load Balancer 实现 WSL 主池、110 fallback 池；Load Balancing 订阅和健康检查尚未配置。
5. 110 故障容错；当前只保证 WSL 停止后的服务延续，不建设两 server etcd，不承诺 110 故障后的控制面或数据库高可用。

环境与资产治理状态：

1. `local-path=false`、`local-path-retain=true`；现有 PVC/PV 不迁移。当前集群查询仍显示两个 StorageClass 带 default 标记，110 加入前必须解析唯一默认类和多节点存储策略。
2. API、非通知 worker 和 migrate 使用 `messagefeed-api:split-20260806`，notification 使用 `messagefeed-notification:split-20260806-2`；业务 Pod PID 1 均为 `tini`；cloudflared 固定为 `2026.6.1`。
3. PostgreSQL 恢复库的数据、迁移、pgvector、索引和约束核验通过，公网健康检查通过。
4. API、六类 worker 和 migrate 使用独立零权限 ServiceAccount，19 条 NetworkPolicy 按角色放行，资源配额和 14 个 PDB 已通过故障验收。
5. 迁移使用 PostgreSQL advisory lock、expand/contract 门禁和失败即终止策略；生产库为 `39,false`。
6. 单 Pod 故障和节点 cordon 演练通过；WSL/Windows 故障接管仍处于 planned，只有完成 110 控制面、数据库、热备用和 Cloudflare Load Balancer 后才能进入验收。

上述状态是当前事实；首个微服务已完成，但仍共享代码模块和 PostgreSQL 数据边界。P2 已排除 `100.72.246.82`，当前只推进本机 WSL 与 `100.106.96.110` 的双节点 K3s 配置。

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
  -> APP_ROLE=item-event-worker：条目事件处理
  -> APP_ROLE=agent-worker：持久化 Agent turn 执行
  -> APP_ROLE=migrate：数据库迁移
```

当前已有基础：

| 能力 | 当前情况 |
| --- | --- |
| 后端架构 | Go 单二进制，内部按 `handler -> service -> repository` 分层 |
| 前端 | Vue 3 + Vite 独立前端，当前由静态服务承载 |
| 数据库 | PostgreSQL + pgvector |
| 后台任务 | 抓取、通知、Agent 定时任务、Embedding、条目事件和 Agent turn 由独立运行角色启动，共用一个后端镜像 |
| 容器化 | 已有 `Dockerfile`、`docker-compose.yml` 和多角色 Helm Chart；当前 K3s 由 Helm 管理 |
| 当前入口 | Cloudflare Tunnel + Caddy gateway |
| 可观测性 | 已有 Prometheus、Loki、Tempo、OpenTelemetry、Grafana 设计基础 |
| 健康检查 | 已有 `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node` |
| 分布式基础 | 已有 PostgreSQL 任务锁、`FOR UPDATE SKIP LOCKED`、租约续期、owner 条件更新、通知幂等、Agent trace/memory/embedding 表 |

当前主要问题：

1. 仍为单二进制多运行角色，尚未形成独立业务代码和数据边界。
2. API、worker 和 migrate 已有独立生命周期、安全身份、网络与资源边界，但仍共用单二进制和数据库。
3. Cloudflare Tunnel 已消除单 Pod 故障点，但全部连接器仍位于同一 WSL 节点，节点或宿主机故障会造成整体不可用。
4. 已建立并接管多角色 Helm Chart；当前仓库已包含 API、六类 worker 和独立 migrate Job，其中新增的 `item-event-worker` 与 `agent-worker` 尚未进入 P2 远程部署。
5. 数据库兼容回滚已形成手动闭环，CI/CD 自动发布闭环尚未建立。
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
  item-event-worker
  agent-worker
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
| `agent-scheduler-worker` | Agent 定时任务触发 |
| `embedding-worker` | 记忆和事实 embedding job |
| `item-event-worker` | 条目事件和告警规则处理 |
| `agent-worker` | 持久化 Agent turn 执行、租约续期和取消 |
| `migrate` | 数据库迁移 Job |
| `PostgreSQL/pgvector` | 主数据、任务、Agent trace/memory/embedding 存储 |

## 4. Kubernetes 方案

当前已落地的是本机 WSL 内 K3s single-server。根据“WSL 主用、WSL 停止后由 110 接管”的新增要求，目标拓扑调整为 110 持续运行控制面和有状态基础设施，WSL 作为主应用 agent。

| 场景 | 方案 |
| --- | --- |
| 当前落地基线 | WSL 内运行 K3s single-server 集群，5 个 local-path PV 均绑定 WSL |
| 本机一次性演练 | 可选 K3d，但不作为长期运行基线 |
| P2 目标控制面 | 110 运行单 K3s server，WSL 迁移为 K3s agent |
| P2 目标数据面 | PostgreSQL 和持续在线的本地持久卷迁移到 110 的 ext4 `/home` 文件系统 |
| 资源编排 | Helm |

P2 目标形态：

```text
实体 Linux 110
  -> K3s server / control-plane
  -> PostgreSQL/pgvector
  -> 110 热备用 API / Web / gateway / cloudflared

Windows / WSL
  -> K3s agent
  -> WSL 主用 API / Web / gateway / cloudflared
  -> 主要 worker 与计算任务
```

当前已部署 `source-worker`、`notification-worker`、`agent-scheduler-worker`、`embedding-worker`、`item-event-worker`、`agent-worker` 和 `migrate` Job。

采用原因：

1. 当前 WSL single-server 已完成应用角色化、Helm 接管和恢复演练，可以作为迁移输入，而不是重新设计应用部署。
2. 110 持续在线并具有大容量 ext4 `/home`，适合承载 control-plane、PostgreSQL 和热备用；WSL 继续作为主应用计算节点。
3. K3s 和 Helm 可以在同一集群内统一管理两个节点的运行角色、Secret/ConfigMap、镜像 tag 和 GPU 调度。
4. 站点标签与独立 Service selector 用于保持 WSL 主用、110 备用，不依赖 Service 的随机跨节点转发。

本阶段定位调整为：

```text
主业务请求正常进入 WSL；WSL 应用或节点停止后，Cloudflare 将新请求切换到 110 热备用。
```

采用原因：

1. K3s server 和 PostgreSQL 若继续留在 WSL，WSL 停止后 110 没有持续可用的控制面和数据库，无法形成完整接管。
2. 110 单 server 可以覆盖 WSL 故障，但仍是控制面和数据库单点；后续要求容忍 110 故障时，应增加两个 server 形成三节点 embedded-etcd，而不是建设无仲裁能力的两 server etcd。
3. 主用和备用入口必须使用独立 Tunnel UUID、独立 Tunnel token 和节点隔离的 Gateway Service，不能依赖同一 Tunnel 的跨主机 replicas。

节点职责：

```text
WSL 本机：K3s agent / 主应用流量节点
实体服务器 110：K3s server / PostgreSQL / 热备用入口 / GPU 计算节点
```

后续扩展原则：

1. 迁移先建立 110 控制面、PostgreSQL 恢复库和本地备用链路，再调整 WSL 节点角色；不得在恢复验证前破坏当前 WSL single-server 数据。
2. WSL 和 110 的 API、Web、gateway 使用站点标签和独立 Service selector 隔离，避免任一 Tunnel 经统一 Service 跨节点转发。
3. PostgreSQL 固定在 110；有状态数据不依赖 WSL local-path。训练任务使用 110 的 GPU 和明确的数据目录。
4. 110 单机故障不在当前承诺范围，`/data/disk_d` 同机备份也不替代加密异机备份。

### 4.1 P0/P1 可靠性与 Agent 执行决策

P0 统一任务表的领取语义：使用 `FOR UPDATE SKIP LOCKED`，持久化 `attempt_count`、`max_attempts`、`locked_by`、`locked_at`、`lease_until`、最后错误和更新时间；领取前回收过期租约，完成/失败更新必须带 owner 条件，达到上限后进入明确失败终态。Source worker 的全局锁只覆盖发现到期来源并入队，不能覆盖实际抓取执行。

P1 将 Agent turn 复用为持久化执行队列：API 事务写入 inbound message 和 `queued` turn 后返回 `202 Accepted`、`turn_id` 和进度 URL；独立 `agent-worker` 使用数据库领取、租约续期和取消状态执行。会话级 PostgreSQL `task_locks` 负责跨 Pod 串行性，进程内锁只作补充；租约失效、取消、锁竞争和 worker 崩溃均必须得到可恢复终态。

P0/P1 的验收重点是任务不丢失、不被旧 owner 覆盖、取消可跨 Pod 传播、重复领取不产生重复业务副作用，并通过队列深度、最老任务年龄、领取耗时、重试、租约回收和死信指标观测。

### 4.2 P2 双节点部署决策

P2 不建设两 server embedded-etcd，也不接入 `100.72.246.82`。本阶段迁移 PostgreSQL 和 K3s control-plane，以覆盖 WSL 应用、WSL 节点和 Windows 宿主机停止场景：

| 节点 | 地址 | 调度职责 |
| --- | --- | --- |
| 实体 K3s server | `100.106.96.110` | control-plane、PostgreSQL、持续在线存储、110 热备用入口、通用 CPU 与 2×RTX 4090 计算 |
| WSL K3s agent | `100.78.141.120` | 主用 API/Web/gateway/cloudflared、主要 worker 与本机计算 |

`100.72.246.82` 的 SSH 入口经核实是外部 Kubernetes 集群中的 Docker Pod，而不是可安装 K3s agent 的宿主机；本阶段不对其安装、加入、调度或跨集群训练。后续只有在取得其所属集群 API、namespace 和 RBAC 后，才单独评估 MultiKueue 等多集群作业派发。

110 先以新 K3s server 建立持续在线基线，K3s state、containerd 和 local-path 位于 `/home/aroen/messagefeed`；PostgreSQL 从 WSL 逻辑备份恢复到 110 并核验后，WSL 才迁移为 agent。应用按 `messagefeed.io/site=wsl` 与 `messagefeed.io/site=server110` 隔离，GPU 任务必须声明 `nvidia.com/gpu` limit，不与普通服务隐式共享设备。

安全基线：SSH 仅使用密钥认证；敏感值只引用 Secret；工作负载使用最小 RBAC、非 root、只读根文件系统、seccomp 和默认拒绝 NetworkPolicy；禁止未审计的 hostPath、Docker socket、主机网络和公网管理入口。ResourceQuota、LimitRange、临时存储限制、日志轮转和孤儿资源检查共同约束计算资源保留。

当前前置检查结果（2026-08-06）：实体节点为 Ubuntu 22.04、80 vCPU、251 GiB 内存和 2×RTX 4090 24 GiB，NVIDIA 驱动 570.211.01、NVIDIA Container Toolkit 1.13.5、Docker Engine 28.3.1、NVIDIA runtime 和 CDI 均可用。`/home` 为 ext4，容量约 14.4 TiB、可用约 10.6 TiB；`/data/disk_d` 为 NTFS/fuseblk，容量约 14.6 TiB、可用约 14.3 TiB。110 尚未安装 K3s 和 cloudflared，`/home/aroen/messagefeed` 与 `/data/disk_d/messagefeed` 尚未创建，P2 状态保持 planned。

## 5. 运行角色方案（已落地）

后端第一阶段保持一个镜像，通过 `APP_ROLE` 控制运行职责。

当前已实现 `APP_ROLE`；cluster 模式禁止隐式使用 `all`，API 和六类 worker 可独立扩缩容。

| `APP_ROLE` | 职责 | 形态 |
| --- | --- | --- |
| `api` | HTTP API、企业微信 callback、健康检查、指标 | Deployment |
| `source-worker` | Feed 抓取任务 | Deployment |
| `notification-worker` | 通知发送任务 | Deployment |
| `agent-scheduler-worker` | Agent 定时任务 | Deployment |
| `embedding-worker` | Embedding job | Deployment |
| `item-event-worker` | 条目事件和告警规则处理 | Deployment |
| `agent-worker` | 持久化 Agent turn 领取、执行、续租和取消 | Deployment |
| `migrate` | 数据库迁移 | Job |
| `all` | 本地兼容模式 | 仅开发或过渡期 |

采用原因：

1. API 可以多副本扩容，不会重复启动 worker。
2. worker 可以按任务类型独立扩缩容。
3. worker 故障不会直接影响 Web/API 主链路。
4. 后续拆业务服务时，运行边界已经提前成型。

## 6. 外部访问方案

第一阶段继续使用 Cloudflare Tunnel，不直接开放公网 NodePort、LoadBalancer 或服务器 80/443 入站端口。

当前 cloudflared 已纳入 Helm 管理并固定使用 HTTP/2。WSL 的 `hostNetwork` 约束下运行 2 副本 OrderedReady StatefulSet，并保留 1 个兼容 Deployment 连接器；三个连接器分别使用指标端口 2010、2011 和 2000。

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

P2 主备入口策略：

```text
用户 / 企业微信
  -> Cloudflare Public Load Balancer：aroen.eu.cc
  -> primary pool：现有 WSL Tunnel
       -> gateway-wsl -> api-wsl / web-wsl
  -> fallback pool：独立 messageFeed_fallback Tunnel
       -> gateway-110 -> api-110 / web-110
```

采用原因：

1. 保留当前已有 Cloudflare Tunnel 访问方式，迁移成本低。
2. 本阶段不开放 Windows/WSL 公网入站端口，入口仍由 Tunnel 出站连接承载。
3. 两个 Tunnel 使用不同 UUID 和 token；同一 Tunnel 的跨主机 replicas 没有主备优先级，不能满足严格故障接管。
4. 两个 Gateway Service 使用站点标签隔离，禁止两个 Tunnel 同时指向当前统一 `gateway` Service。

入口演进：

| 形态 | 方案 | 适用情况 |
| --- | --- | --- |
| WSL 多连接器入口 | 同一 WSL 内 3 个 `cloudflared` 连接器承载入口 | 当前已完成，仅覆盖 Pod/进程故障 |
| WSL/110 主备入口 | 两个独立 Tunnel、两个 pool、`/readyz` HTTPS monitor 和 110 fallback pool | P2 目标 |
| 三 server 控制面 | 增加两个持续在线 server，形成奇数 embedded-etcd 集群 | 后续要求容忍 110 故障时采用 |

Cloudflare Load Balancing 是账户附加产品，启用前需确认计费资料和套餐。健康监控使用业务 `/readyz`，不能使用固定 `HTTP_STATUS=200` 代替；当前 `/readyz` 已检查进程、PostgreSQL、迁移、pgvector 和 Agent 数据结构。切换时间受套餐最小监控间隔、timeout、retry 与连续失败阈值影响，不声明逐请求零损失。

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

本节所述单节点内高可用方案已经完成。当前集群运行 3 个 cloudflared 连接器，gateway、API 和 Web 各 2 副本，并已完成滚动发布和逐 Pod 故障演练。

当前 Tunnel 偶发 `1033` 或网关错误时，新的方案把入口链路从单点升级为多副本。

目标链路：

```text
多个 cloudflared 连接器
  -> 多 gateway
  -> 多 api/web
```

技术决策：

| 问题 | 方案 |
| --- | --- |
| `cloudflared` 进程单点 | WSL 内 2 副本有序 StatefulSet + 1 个兼容 Deployment |
| gateway/API 单点 | WSL 内 Caddy gateway、api、web 均多副本 |
| 单 Pod 故障 | 通过 Deployment 自恢复、readiness 和 Service endpoint 切换处理 |
| WSL 关机 | 由独立 `messageFeed_fallback` Tunnel 和 110 热备用接管新请求 |
| QUIC/UDP 网络不稳 | 保留 `auto/quic/http2` 协议切换能力 |
| 1033/502 难定位 | 增加外部探测、内部探针、metrics 和日志关联 |

采用原因：

1. `1033` 通常表示 Cloudflare 找不到健康的 `cloudflared` 连接。
2. 多副本 `cloudflared` 可以降低单连接器故障导致的入口中断。
3. 多 gateway、多 api/web 可以降低内部 Pod 重启导致的 `502/504`。
4. 探针和指标用于证明修复有效，并定位故障属于 Cloudflare、Tunnel、gateway、API 还是数据库。

P2 使用 Cloudflare Public Load Balancer 实现外部流量优先级切换：

```text
hostname：aroen.eu.cc
primary pool：<WSL_TUNNEL_UUID>.cfargotunnel.com
  Host：origin-wsl.aroen.eu.cc
fallback pool：<FALLBACK_TUNNEL_UUID>.cfargotunnel.com
  Host：origin-110.aroen.eu.cc
monitor：HTTPS /readyz，expected 200
traffic steering：Off
```

`messageFeed_fallback` 的 Tunnel token 只注入 110 的 `messagefeed-cloudflared-standby-secret`，不写入仓库、values、命令历史或本文档。WSL 继续使用现有 `messagefeed-cloudflared-secret`。通过 Dashboard 配置 Load Balancer 时不需要 Global API Key；只有 API 自动化才使用最小权限的 Load Balancer API token。

## 8. 数据库方案

数据库继续使用 PostgreSQL + pgvector。

当前数据库主实例仍位于 WSL K3s local-path PV。P2 必须先生成逻辑备份并恢复到 110 的 ext4 `/home/aroen/messagefeed/postgres`，核验迁移版本、pgvector、索引、约束和业务数据后，再让 WSL 主应用与 110 热备用共同连接 110 PostgreSQL。否则 WSL 停止时备用 API 不具备完整服务能力。

当前由独立 Helm pre-install/pre-upgrade migrate Job 执行迁移，API 不再包含 migration init container。现有 5 个 PV 已设置为 `Retain`，完整恢复演练已通过；当前集群查询显示两个 StorageClass 带 default 标记，唯一默认类修正列为 110 加入前置任务。

第一阶段不把数据库复杂高可用作为主目标，而是优先保证：

1. 连接配置可在 Kubernetes Secret 中管理。
2. 迁移通过独立 Job 执行。
3. 备份和恢复策略明确。
4. 数据库迁移遵循向后兼容原则。
5. PostgreSQL custom dump、K3s etcd snapshot、非敏感 manifests 和 SHA-256 校验文件写入 `/data/disk_d/messagefeed/backups`；NTFS 盘不承载实时数据库、K3s datastore 或 containerd overlayfs。

采用原因：

1. PostgreSQL/pgvector 是当前系统主数据和 Agent 记忆/Embedding 的核心依赖。
2. 应用部署改造和数据库高可用不宜在第一阶段叠加。
3. 对个人项目和小型集群而言，先保证备份、恢复和兼容迁移比立即搭建数据库 HA 更重要。
4. 110 PostgreSQL 仍是单实例，只覆盖 WSL 故障，不覆盖 110 故障；`disk_d` 与服务同机，仍需保留一份加密异机备份。

当前迁移治理已落地：迁移进程在版本读取和 SQL 执行前获取 PostgreSQL advisory lock；从版本 38 起，文件必须标记 `_expand_` 或 `_contract_`。常规发布只允许 expand，并拒绝破坏性 SQL；contract 必须在旧应用停止读取旧结构后显式启用。迁移失败通过 Job `PodFailurePolicy` 立即阻断发布，dirty schema 不自动 force。

## 9. CI/CD 技术方案

第一阶段采用 GitHub Actions + 镜像仓库 + Helm。

当前实现状态：工作流文件 `.github/workflows/ci.yml` 已建立，包含 PR/push 校验、API/notification 独立镜像构建与 GHCR 推送，以及受保护 staging 手动部署任务。由于尚未注册 `messagefeed-staging` 自托管 Runner，staging 首次流水线尚未执行；生产本次仍通过本地构建、导入 K3s containerd 和 Helm 原子发布完成。

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

当前多角色阶段已完成无感升级基线：API、Web、Gateway 各 2 副本；cloudflared 为 3 个连接器；六类 worker 生产默认各 1 副本，并由数据库 claim/幂等保护短暂双跑；独立迁移和 Helm rollback 已验证。notification-service 已完成独立镜像滚动切换。

技术决策：

| 事项 | 方案 |
| --- | --- |
| API/Web/Gateway 发布 | RollingUpdate，`minReadySeconds=10`，`preStop=10s` |
| 最大不可用 | `maxUnavailable=0` |
| 最大增量 | `maxSurge=1` |
| API readiness | `/readyz` |
| API liveness | `/healthz` |
| 数据库迁移 | expand/contract |
| 应用回滚 | Helm rollback 或镜像 tag 回退 |
| PostgreSQL 更新 | StatefulSet `OnDelete`，人工确认后重建 |
| cloudflared 更新 | OrderedReady StatefulSet；兼容 Deployment 使用 Recreate |

采用原因：

1. 新 Pod 未 ready 前旧 Pod 继续接流量。
2. readiness 失败的 Pod 不进入 Service endpoints。
3. 兼容式迁移保证应用版本可回滚。
4. worker 迁移时可避免新旧环境双跑导致重复通知或重复抓取。

当前阶段无感升级要求：

1. API、Web、Gateway 和 cloudflared 已完成多副本滚动与单 Pod 故障演练。
2. readiness 失败的 Pod 不进入 Service endpoints，旧 Pod 在新 Pod ready 前继续接流量。
3. worker 必须依赖数据库任务锁、job claim 和幂等机制，保证同一 WSL 集群内多副本不会重复处理同一任务。
4. Windows 关机、WSL 停止、本机断网属于当前阶段不可无感覆盖的故障，后续通过远程服务器扩展解决。

## 11. 后续微服务边界

业务微服务拆分放在 Kubernetes 多角色部署稳定之后；当前已完成第一个 notification-service，第二个服务仍需等待 CI/CD staging 闭环稳定。

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

### 11.1 notification-service 已实施边界

1. `cmd/notification` 是独立 Go 入口，强制使用 `APP_ROLE=notification-worker`；复用现有 bootstrap、notification service、repository、租约、重试和幂等实现。
2. Dockerfile 的 `notification` target 生成独立镜像；Helm 仅为 notification Deployment 覆盖镜像，其他 worker 保持 API 多角色镜像。
3. 当前仍共享 `notification_jobs`、`notification_deliveries` 和 PostgreSQL，因此本阶段是独立运行服务，不是独立数据库服务。
4. 生产切换后 notification Pod `/healthz`、`/readyz`、`/metrics` 和 Prometheus target 均正常；空队列统计 NULL 扫描问题已修复。
5. 后续拆分第二个服务前，必须完成 GHCR、staging Runner、smoke test、人工审批、观察和 rollback 自动闭环。
