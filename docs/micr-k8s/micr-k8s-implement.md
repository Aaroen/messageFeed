## messageFeed Kubernetes 实施文档

**定位**：实施细节、操作步骤、验收口径
**更新日期**：2026-07-18
**上位约束**：`micr-k8s-plan.md`

本文档只展开 `micr-k8s-plan.md` 已确定的技术方案，不新增新的架构路线。若本文件与 `micr-k8s-plan.md` 冲突，以 `micr-k8s-plan.md` 为准。

## 当前实施状态（2026-07-18）

已完成：

1. WSL 内 K3s single-server、动态网络维护、Helm 工具链和基础运行环境核验。
2. all-in-one 阶段 Helm Chart、`values.yaml` 与 `values-k3s.yaml`。
3. PostgreSQL/pgvector、API、Web、Caddy gateway、cloudflared 和观测栈的 Helm 接管。
4. PostgreSQL 备份恢复演练、5 个 PV 设置为 `Retain`、数据库和公网健康检查验收。
5. 环境与资产治理已完成，`local-path-retain` 为唯一默认 StorageClass，5 个现有 PV 均为 `Retain`。
6. `APP_ROLE`、`internal/bootstrap`、四类独立 worker Deployment/Service 和独立 migrate Job 已完成并通过验收。
7. 当前 Helm release `messagefeed` revision 7 为 `deployed`，API 与四类 worker 各为 1 个 Ready 副本。

当前边界：

1. 当前仍为单二进制、多运行角色架构，尚未拆分独立业务代码仓库或数据库边界。
2. API、worker 和 migrate 尚未配置独立 ServiceAccount、最小 RBAC、NetworkPolicy、PDB、ResourceQuota 或 LimitRange。
3. API 与 worker 已验证独立扩缩容和 Helm rollback，但生产声明值仍为单副本；尚未形成完整多副本高可用基线。
4. 当前未建立 CI/CD 发布闭环，镜像仍由本地构建并导入 K3s containerd。
5. 集群使用 `messagefeed-api:role9-20260718-8a454cb690ec`；5 个角色均以 `tini` 为 PID 1，现有 PVC/PV 不迁移。

后续实施门槛：第 9 节已通过，下一阶段实施第 10 节 ServiceAccount、最小 RBAC、NetworkPolicy 与资源治理；第 10～12 节完成前不进入真实业务微服务拆分。

## 顶部步骤 TODO

本 TODO 是整体实施顺序导航。每一部分都以前一部分完成验收为前提，避免在启动边界、部署基线和回滚能力尚未稳定前提前进入微服务重构。

### 第一部分：固定 WSL 执行入口与项目基线

- [x] 若操作环境位于Linux内这直接进行，否则统一所有操作入口为 `ssh aroen@127.0.0.1`。
- [x] 进入项目目录：`/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`。
- [x] 核实当前源码、`Dockerfile`、`docker-compose.yml`、`migrations`、`deploy/caddy`、`ops/observability` 都在 WSL 项目内。
- [x] 核实 WSL 内基础命令：`go`、`docker`、`kubectl`、`helm`、`git`。
- [x] 记录当前运行方式、端口、Cloudflare Tunnel token 来源、数据库数据目录和 `.env` 敏感配置来源。
- [x] 明确当前阶段边界：WSL 长期运行，但不承诺 Windows 关机、WSL 停止或本机断网后服务持续在线。

### 第二部分：核实现有代码职责与改造切入点

- [x] 核实 `cmd/api/main.go` 当前同时启动 HTTP API、source sync、notification、agent scheduled task、embedding worker。
- [x] 核实数据库连接池、健康检查、指标、日志、OpenTelemetry、企业微信、LLM、Embedding 配置读取方式。
- [x] 梳理当前 worker 的任务锁、job claim、幂等、重试和失败记录机制。
- [x] 确认第一轮不改业务模型、不拆仓库、不直接引入 gRPC/Eino/Nginx Ingress。
- [x] 确认第一轮重构目标是运行边界，不是业务微服务边界。

### 第三部分：完成 `APP_ROLE` 启动角色化

- [x] 新增并校验 `APP_ROLE=api`。
- [x] 新增并校验 `APP_ROLE=source-worker`。
- [x] 新增并校验 `APP_ROLE=notification-worker`。
- [x] 新增并校验 `APP_ROLE=agent-scheduler-worker`。
- [x] 新增并校验 `APP_ROLE=embedding-worker`。
- [x] 新增并校验 `APP_ROLE=migrate`。
- [x] 保留 `APP_ROLE=all` 仅用于本地兼容或过渡。
- [x] 在 `DEPLOYMENT_MODE=cluster` 下禁止默认使用 `APP_ROLE=all`。
- [x] 验证 `api` 角色只启动 HTTP，不启动 worker。
- [x] 验证 worker 角色不监听业务 HTTP，只执行对应后台职责并暴露独立运维端点。

### 第四部分：拆出启动装配层

- [x] 新增或整理 `internal/bootstrap`。
- [x] 将配置加载、日志、Tracing、数据库、Repository、Service、Router、Worker 装配拆开。
- [x] 将数据库迁移与 API/worker 启动解耦。
- [x] 保证每个角色拥有清晰生命周期和优雅退出逻辑。
- [x] 保证每个角色拥有可区分的 `APP_NODE_ID`、日志字段和指标标签。
- [x] 为角色启动行为增加单元测试和命令级验收。

### 第五部分：完成 all-in-one 镜像与容器化基线

- [x] 后端继续使用一个多角色镜像：`messagefeed-api:role9-20260718-8a454cb690ec`。
- [x] 前端继续使用独立镜像：`messagefeed-web:allinone-0703de0`。
- [x] 禁止生产部署使用 `latest`。
- [x] 后端镜像 tag 使用 Git SHA。
- [x] 构建并部署包含 `tini` 的新后端镜像。
- [x] 确认当前容器健康检查路径与 K8s 探针一致。

### 第六部分：搭建 WSL 内 K3s single-server 基线

- [x] 通过 SSH 进入 WSL 后或直接在环境内安装或核实 K3s single-server。
- [x] 确认 `kubectl get nodes` 中 WSL 节点为 `Ready`。
- [x] 确认 Helm 可在 SSH 会话内操作 WSL 内 K3s。
- [x] 建立 WSL 内 StorageClass、数据卷、命名空间和 Secret 管理基线，并确认唯一默认 StorageClass。
- [x] 部署或接入 WSL 内 PostgreSQL/pgvector。
- [x] 明确数据库备份落点和归档校验方式，并完成完整恢复演练。

### 第七部分：编写并接管 all-in-one Helm Chart

- [x] 创建 `deploy/helm/messagefeed` 目录结构。
- [x] 编写并接管 API、Web、Caddy gateway 和 cloudflared Deployment/Service。
- [x] 编写并接管 PostgreSQL、Prometheus、Loki、Tempo、OTel Collector、Grafana 和 Promtail。
- [x] 编写 source-worker、notification-worker、agent-scheduler-worker 和 embedding-worker Deployment/Service。
- [x] 编写独立 migrate Job，并移除 API init container 迁移职责。
- [x] 编写 ConfigMap 与既有 Secret 引用。
- [x] 使用 `values.yaml` 与 `values-k3s.yaml` 描述多角色 WSL/K3s 环境。
- [x] 建立 `local-path-retain` StorageClass 模板，并将其设为唯一默认类。

### 第八部分：环境与资产治理

- [x] 完成 all-in-one Helm 部署、namespace、Secret 引用、PVC/PV 和公网健康检查基线。
- [x] 修正 `local-path` 与 `local-path-retain` 双默认 StorageClass。
- [x] 固定 cloudflared 镜像版本，完成默认凭据和 Secret 治理。
- [x] 完成 PostgreSQL 备份恢复演练。

### 第九部分：应用运行边界拆分

- [x] 梳理 worker 任务锁、claim、幂等、重试和失败记录。
- [x] 实现 `APP_ROLE` 和启动装配层。
- [x] 构建并部署包含 `tini` 的新镜像。
- [x] 验证 API、各类 worker 和 migrate 可独立启动、停止、扩缩容和观测。

### 第十部分：Kubernetes 安全与资源治理

- [ ] 为 API、worker 和 migrate 配置独立 ServiceAccount 与最小 RBAC。
- [ ] 增加 NetworkPolicy、资源请求/限制、PDB、ResourceQuota 和 LimitRange。
- [ ] 验证网络访问、权限边界、资源边界和故障预算。

### 第十一部分：迁移、高可用与回滚

- [x] 将 API init container 迁移改为独立 migrate Job。
- [ ] 完成 API、Web、Gateway、cloudflared 多副本和滚动发布演练。
- [ ] 验证 readiness、单 Pod 故障、worker 幂等和 Helm rollback。
- [ ] 明确 WSL 关闭、Windows 关机和本机断网不属于当前可用性承诺。

### 第十二部分：CI/CD 闭环

- [ ] 执行后端、前端和 Helm 自动校验。
- [ ] 构建并推送 Git SHA 或 SemVer + Git SHA 镜像。
- [ ] 完成 K3s 部署、smoke test、发布观察和 rollback。

### 第十三部分：微服务拆分

- [ ] 第八至十二部分全部完成并通过验收。
- [ ] 定义第一个服务的接口、数据边界、重试和回滚策略。
- [ ] 优先拆分 `notification-worker`，每次只迁移一个服务。

### 第十四部分：多节点扩展与阶段验收

- [ ] 确定 Tailscale/WireGuard 等安全网络方案。
- [ ] 准备 K3s agent join、节点 label、taint/toleration 和多节点 values。
- [ ] 验证新节点加入不影响 WSL 主线；多节点、数据库 HA 和 HPA 不作为当前微服务化强制前置。

## 0. 当前连接与执行基线

当前阶段先按本机 WSL 长期运行方式推进。所有本机项目操作默认通过 SSH 进入 WSL 后执行：

```bash
ssh aroen@127.0.0.1
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
```

默认基线：

```text
Windows 宿主机
  -> SSH 连接到 WSL
  -> WSL 内运行 K3s single-server
  -> WSL 内运行 PostgreSQL/pgvector
  -> WSL 内运行 gateway/cloudflared/api/web/worker Pods
```

当前阶段不承诺 Windows 关机、WSL 停止或本机断网后的持续在线。后续持续在线能力通过实验室服务器和低配常驻服务器扩展实现。

## 1. 实施边界

本轮实施目标：

1. 让当前单体 Go 程序支持多运行角色。
2. 使用 Kubernetes 管理 API、Web、worker、gateway、cloudflared。
3. 先在 WSL 内完成 K3s single-server 长期运行。
4. 通过 SSH 连接方式统一本机操作口径，使后续迁移到真实 Linux 服务器时步骤一致。
5. 后续再支持实验室服务器和低配服务器作为 K3s agent 节点加入并承载副本。
6. 稳定后再逐步拆真实业务微服务。

本轮不做：

1. 不直接拆多个业务微服务。
2. 不直接引入 Nginx Ingress。
3. 不把数据库复杂高可用作为第一轮主目标。
4. 不把 Argo CD/Flux 作为第一轮必选项。
5. 不允许 API 多副本继续默认启动全部 worker。
6. 不在当前阶段承诺 WSL 关闭后服务仍持续在线。

## 2. 实施总览

实施顺序：

```text
SSH 进入 WSL
  -> 核实项目与依赖
  -> 代码启动角色化
  -> 镜像与 Helm chart
  -> WSL 内 K3s single-server
  -> WSL 内 PostgreSQL/pgvector
  -> WSL 内 Gateway/API/Web 多副本
  -> WSL 内 Worker 分角色运行
  -> WSL 内 Cloudflare Tunnel 多副本入口
  -> CI/CD 初版
  -> 后续服务器 agent 加入
  -> 后续跨节点副本分布
```

成功后的运行形态：

```text
Cloudflare
  -> cloudflared replicas
  -> Caddy gateway replicas
  -> api replicas / web replicas

workers:
  source-worker
  notification-worker
  agent-scheduler-worker
  embedding-worker

storage:
  PostgreSQL + pgvector
```

## 3. 代码启动角色改造

### 3.1 新增配置

新增环境变量：

```text
APP_ROLE=all
APP_ROLE=api
APP_ROLE=source-worker
APP_ROLE=notification-worker
APP_ROLE=agent-scheduler-worker
APP_ROLE=embedding-worker
APP_ROLE=migrate
```

默认建议：

```text
本地开发默认：APP_ROLE=all
Kubernetes API：APP_ROLE=api
Kubernetes worker：按具体角色设置
```

### 3.2 配置校验规则

`APP_ROLE` 只允许上述枚举值。

生产环境禁止隐式使用 `all`。如果需要保留兼容，可以通过显式变量允许：

```text
ALLOW_ALL_ROLE_IN_CLUSTER=false
```

推荐规则：

```text
DEPLOYMENT_MODE=cluster 时，APP_ROLE=all 启动失败。
DEPLOYMENT_MODE=single_node 时，APP_ROLE=all 可用于本地兼容。
```

## 4. 启动装配层

**状态**：尚未完成。当前 `cmd/api/main.go` 仍集中负责配置、依赖构造、HTTP 服务和后台 worker 启动。

### 4.1 启动装配拆分

建议新增：

```text
internal/bootstrap/
  app.go
  config.go
  logger.go
  tracing.go
  database.go
  repositories.go
  services.go
  router.go
  workers.go
```

职责：

| 文件 | 职责 |
| --- | --- |
| `config.go` | 加载配置和角色校验 |
| `logger.go` | 初始化 slog |
| `tracing.go` | 初始化 OpenTelemetry |
| `database.go` | 打开 PostgreSQL、ping、连接池 |
| `repositories.go` | 构造 repository |
| `services.go` | 构造 service |
| `router.go` | 构造 Gin router |
| `workers.go` | 构造并启动 worker loop |
| `app.go` | 汇总启动依赖 |

### 4.2 角色启动行为

| 角色 | 启动 HTTP | 启动 worker |
| --- | --- | --- |
| `api` | 是 | 否 |
| `source-worker` | 否 | 仅 source sync |
| `notification-worker` | 否 | 仅 notification |
| `agent-scheduler-worker` | 否 | 仅 agent scheduled task |
| `embedding-worker` | 否 | 仅 embedding |
| `all` | 是 | 全部 |
| `migrate` | 否 | 仅迁移 |

### 4.3 验收

命令级验收：

```text
APP_ROLE=api go run ./cmd/api
APP_ROLE=source-worker go run ./cmd/api
APP_ROLE=notification-worker go run ./cmd/api
APP_ROLE=agent-scheduler-worker go run ./cmd/api
APP_ROLE=embedding-worker go run ./cmd/api
```

行为验收：

1. `api` 角色监听 HTTP。
2. `api` 角色日志中不出现 worker tick。
3. worker 角色不监听 HTTP 端口。
4. worker 可以正常 claim job。
5. 多 worker 并发不重复处理同一个 job。
6. `APP_NODE_ID` 能区分不同 Pod。

## 5. 单镜像多角色容器化

### 5.1 后端镜像

第一阶段仍使用一个后端镜像：

```text
messagefeed-api:<git-sha>
```

虽然名字叫 `api`，但它通过 `APP_ROLE` 启动不同后端角色。

当前多角色阶段实际使用 `messagefeed-api:role9-20260718-8a454cb690ec`；已验证 API、四类 worker 和 migrate 均由 `tini` 作为 PID 1 启动 `/app/messagefeed`。

原因：

1. 减少第一阶段镜像数量。
2. 避免拆多个二进制。
3. 后续真实拆服务时，再把某个角色替换成独立镜像。

### 5.2 前端镜像

前端独立镜像：

```text
messagefeed-web:<git-sha>
```

### 5.3 镜像 tag

禁止生产使用：

```text
latest
```

推荐：

```text
<git-sha>
<semver>-<git-sha>
```

## 6. WSL 内 K3s single-server 基线

**状态**：已完成。WSL 内 K3s、动态网络维护、Helm 工具链和基础组件验收已完成。

当前基线：

```text
Windows
  -> WSL
  -> K3s server / control-plane
  -> messagefeed namespace
  -> PostgreSQL/pgvector
  -> API / worker / Web / Caddy gateway / cloudflared
  -> Prometheus / Loki / Tempo / OTel Collector / Grafana / Promtail
```

核查命令：

```bash
kubectl get nodes -o wide
kubectl get pods -A
kubectl get storageclass
helm list -A
```

验收标准：

1. WSL K3s 节点为 `Ready`。
2. CoreDNS、local-path-provisioner 和 metrics-server 正常运行。
3. `kubectl` 与 `helm` 可以访问当前集群。
4. PostgreSQL 备份落点和归档校验方式已明确。
5. 当前阶段不承诺 Windows 关机、WSL 停止或本机断网后的持续在线。

当前约束：

1. `local-path-retain` 是唯一默认 StorageClass，新 PVC 默认使用 `Retain` 回收策略。
2. 现有 PVC/PV 不迁移；5 个现有 PVC 仍使用 `local-path`，对应 PV 均为 `Retain`。

## 7. Helm Chart 与 Workload 设计

**状态**：多角色 Chart 已完成并用于现有资源接管；API、四类 worker、独立 migrate Job 和 `APP_ROLE` 模板均已实现。

Chart 入口：

```text
deploy/helm/messagefeed/
  Chart.yaml
  values.yaml
  values-k3s.yaml
  values.schema.json
  files/migrations/
  files/observability/
  templates/
    api.yaml
    workers.yaml
    migrate.yaml
    web.yaml
    gateway.yaml
    cloudflared.yaml
    postgresql.yaml
    migrations-configmap.yaml
    storageclass.yaml
    observability-*.yaml
```

当前配置原则：

1. `values.yaml` 提供默认配置，`values-k3s.yaml` 提供 WSL/K3s 覆盖。
2. 既有数据库、应用、Caddy 和 Tunnel Secret 通过 `existingSecret` 引用，不在 values 中保存明文。
3. API 与四类 worker 的副本数可在 values 中独立设置；当前生产声明值保持各 1 副本。
4. `values-k3s.yaml` 将 cloudflared 固定为 `2026.6.1`，Chart schema 拒绝 `latest`。
5. 数据库迁移由独立 Helm pre-install/pre-upgrade Job 执行，API 不再包含 migration init container。

Workload 边界：

| Workload | 当前状态 | 目标状态 |
| --- | --- | --- |
| API | `messagefeed-api`，`APP_ROLE=api`，只提供 HTTP | 已落地 |
| source/notification/agent/embedding worker | 各自独立 Deployment/Service，仅提供 `9090` | 已落地 |
| migrate | 独立 Helm Job，`APP_ROLE=migrate` | 已落地 |
| Web/Gateway/Tunnel | 已由 Helm 管理，当前单副本 | 按高可用演练逐步扩容 |
| PostgreSQL/观测栈 | 已由 Helm 管理，PVC 保持原绑定 | 在备份和资源策略稳定后再扩展 |

Helm 验证命令：

```bash
helm lint deploy/helm/messagefeed -f deploy/helm/messagefeed/values-k3s.yaml

helm template messagefeed deploy/helm/messagefeed \
  --namespace messagefeed \
  -f deploy/helm/messagefeed/values-k3s.yaml

helm status messagefeed -n messagefeed
```

## 8. 环境与资产治理

**状态**：已完成。all-in-one Helm 部署、存储治理、镜像版本治理、Grafana Secret 治理和 PostgreSQL 恢复演练均通过验收。

当前基线：

| 项目 | 状态 |
| --- | --- |
| Helm release | `messagefeed` revision 7，`deployed` |
| PostgreSQL | 生产库与恢复库均为迁移状态 `37,false`，pgvector `0.8.4` 可用 |
| PVC/PV | 5 个 PVC 为 `Bound`，5 个 PV 为 `Retain` |
| 外部入口 | Cloudflare -> cloudflared -> Caddy -> Web/API，公网 `/healthz` 与 `/readyz` 均为 HTTP 200 |
| 镜像 | API 为 `messagefeed-api:role9-20260718-8a454cb690ec`；cloudflared 为 `2026.6.1` |
| Secret | Grafana 管理密码已随机化并由 `messagefeed-grafana-secret` 提供 |
| StorageClass | `local-path=false`，`local-path-retain=true`，默认类唯一 |

实施结果：

1. 已将 `local-path` 默认注解设为 `false`，确认 `local-path-retain` 为唯一默认类；现有 PVC/PV 绑定关系未变化。
2. cloudflared 已固定为 `2026.6.1`，实际 digest 为 `sha256:6d91c121b803126f7a5344005d17a9324788fc09d305b6e2560ec6040a7ae283`；API 已切换至按 Git SHA 标记且包含 `tini` 的镜像。
3. Grafana 管理凭据已迁移至独立 Secret，随机密码长度为 48，持久化管理员密码已轮换，管理 API 验证为 HTTP 200。
4. 已生成并校验 `backups/k8s-adoption/messagefeed-restore-drill-20260718-144227.dump`，恢复至隔离数据库 `messagefeed_restore_drill_20260718`。
5. 恢复库为迁移状态 `37,false`、pgvector `0.8.4`、55 张 public 基础表；核心数据包括 4 个用户、145 个源、7933 条内容、8 条用户内容状态、47 条源目录和 28609 条审计记录，与备份前快照一致。
6. 恢复库重复内容组为 0，`uq_items_source_normalized_url` 的 unique/valid/ready 均为 true，未验证约束为 0；验收后已关闭新连接并保留该恢复库。

完成判定：

1. 新 PVC 不再依赖错误的默认 StorageClass。
2. 生产镜像不使用 `latest`，敏感配置不使用默认值。
3. 备份可以恢复，恢复后的应用健康检查和数据核验通过。

## 9. 应用运行边界拆分

**状态**：已完成（2026-07-18）。当前 release 已从 all-in-one 过渡为 API、四类 worker 和独立 migrate Job。

### 9.1 运行角色与启动边界

运行角色固定为：

```text
all
api
source-worker
notification-worker
agent-scheduler-worker
embedding-worker
migrate
```

约束：

1. `DEPLOYMENT_MODE=cluster` 下禁止 `APP_ROLE=all`；只有显式设置 `ALLOW_ALL_ROLE_IN_CLUSTER=true` 才允许兼容运行。
2. `api` 只构造业务 Router 并监听 `60001`，不启动任何 worker loop。
3. 四类 worker 只构造自身 service 和 loop，不监听业务端口 `60001`；统一在 `9090` 提供 `/healthz`、`/readyz` 和 `/metrics`。
4. `migrate` 只调用既有 `golang-migrate` CLI，迁移路径固定为相对路径 `migrations`，不启动 HTTP 或 worker。
5. `APP_NODE_ID` 使用 Pod 名称，日志基础字段包含 `app_role`，Prometheus target 使用同名 `app_role` 标签。

### 9.2 代码与镜像实现

1. `internal/bootstrap` 汇总角色计划、数据库与 service 装配、worker loop、运维端点和受控关闭；`cmd/api/main.go` 仅负责入口生命周期。
2. 配置层新增 `APP_ROLE`、`ALLOW_ALL_ROLE_IN_CLUSTER`、`WORKER_METRICS_ADDR` 和 `MIGRATIONS_PATH`，并校验 cluster 数据库、相对迁移路径及角色枚举。
3. source、notification、agent scheduler 和 embedding claim 复核结果如下：

| 队列 | 一致性实现 | 结果 |
| --- | --- | --- |
| source fetch | PostgreSQL `FOR UPDATE SKIP LOCKED` 事务 claim | 保留 attempt、lock、失败与重试字段 |
| notification | PostgreSQL `FOR UPDATE SKIP LOCKED` 事务 claim | 保留 dedupe key、delivery 与重试字段 |
| agent scheduled task | PostgreSQL `FOR UPDATE SKIP LOCKED` 事务 claim | 保留 locked_by、attempt 和失败状态 |
| embedding index | 原子 `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING` | pending 任务不会被两个 claimant 同时取得 |

4. Dockerfile 将 `migrate 4.19.1` 和 `migrations` 复制进同一后端镜像；运行阶段使用非 root `appuser`，入口为 `/sbin/tini -- /app/messagefeed`，并按角色选择健康检查端口。

### 9.3 Helm 拓扑

当前 Chart 渲染并部署以下资源：

| 工作负载 | `APP_ROLE` | 业务端口 | 运维端口 | 副本 |
| --- | --- | --- | --- | ---: |
| `messagefeed-api` | `api` | `60001` | `/metrics` 复用 API | 1 |
| `source-worker` | `source-worker` | 不监听 | `9090` | 1 |
| `notification-worker` | `notification-worker` | 不监听 | `9090` | 1 |
| `agent-scheduler-worker` | `agent-scheduler-worker` | 不监听 | `9090` | 1 |
| `embedding-worker` | `embedding-worker` | 不监听 | `9090` | 1 |
| `messagefeed-migrate` | `migrate` | 不监听 | 不监听 | Job |

迁移 Job 使用 Helm `pre-install,pre-upgrade` hook；API 不再包含 migration init container。四个 worker Service 只发布 `9090`，Prometheus 配置新增四个独立 scrape target。

### 9.4 严格验收结果

代码与模板：

```text
go test ./...                         PASS
go test -race -count=1 ./internal/bootstrap ./internal/config ./cmd/api PASS
go vet ./...                          PASS
go build ./cmd/api                    PASS
helm lint                             PASS
helm template                         PASS
kubectl apply --dry-run=client        PASS
schema 反向校验（latest/副本 0/非法角色/非法迁移路径） PASS
```

集群发布：

```text
镜像：messagefeed-api:role9-20260718-8a454cb690ec
Helm：revision 7，STATUS=deployed
migrate Job：Complete，1/1
生产数据库：schema_migrations=37,false，pgvector=0.8.4，public 基础表=55
```

运行隔离：

1. API `/healthz`、`/readyz` 返回 200，API Pod 日志只有 API server 启动记录，没有 worker loop/tick 记录；API 的 `9090` 连接被拒绝。
2. 四个 worker 的 `/healthz`、`/readyz`、`/metrics` 均返回成功；四个 worker 的 `60001` 连接均被拒绝。
3. 五个 messagefeed Prometheus target 全部 `up`：API 使用 `api:60001`，worker 使用各自 Service 的 `9090`。
4. 五个业务 Pod 的 PID 1 均为 `/sbin/tini -- /app/messagefeed`，运行用户为 UID 1000。
5. `https://aroen.eu.cc/healthz` 和 `https://aroen.eu.cc/readyz` 均返回 HTTP 200；gateway 内部 `/healthz` 与 Web 首页访问成功。

并发 claim：

在隔离数据库 `messagefeed_role9_acceptance_20260718` 的四张真实队列表中各准备 40 条任务，两个并发 claimant 各处理 20 条。四类队列均得到 40 个不同 ID，重复 claim 行数为 0；source、notification、scheduler 三类任务的 `attempt_count=1`，embedding 队列的任务均由 pending 原子转为 running，未留下 queued/pending 任务；验收库随后设置 `ALLOW_CONNECTIONS=false`，生产库未写入测试任务。

扩缩容与回滚：

1. Helm revision 5 将 API 与 source worker 独立扩展为 2 副本，其他 worker 保持 1 副本，所有 messagefeed Prometheus target 仍为 `up`。
2. `helm rollback messagefeed 4 --wait --wait-for-jobs` 成功生成 revision 6，并恢复 API/source worker 各 1 副本。
3. 最终 revision 7 通过 `helm upgrade --atomic --wait --wait-for-jobs` 固化模板标签和声明值。

优雅退出：

对 source worker PID 1 发送 SIGTERM 后，容器重启计数由 0 增至 1；`--previous` 日志依次包含 `worker loop stopped`、`application role stopping` 和 `application role stopped`，无 error/panic，重启后 `/readyz` 恢复为 200。

**第 9 节判定**：API 与 worker 运行边界、独立迁移、日志/指标、claim 并发、容器 PID 1、独立扩缩容、rollback 和 SIGTERM 优雅退出均通过；下一阶段进入第 10 节 Kubernetes 安全与资源治理。

## 10. Kubernetes 安全与资源治理

**状态**：尚未完成。当前主要工作负载仍使用默认 ServiceAccount，尚未建立完整网络和资源治理边界。

实施内容：

1. 为 API、各类 worker 和 migrate 配置独立 ServiceAccount 与最小 RBAC。
2. 增加默认拒绝 NetworkPolicy，仅放行 DNS、PostgreSQL、OTel、gateway 和必要的外部访问。
3. 配置 CPU/内存 requests、limits、ResourceQuota、LimitRange 和 PDB。
4. 为有状态组件、worker 和入口组件补充节点选择、反亲和性或 topology spread 策略。

完成判定：

1. Pod 只能访问其职责所需的 Kubernetes API 和网络目标。
2. 缺少权限或网络放行时，失败行为可观测且不会扩大影响范围。
3. 资源不足、节点维护和 Pod 驱逐时具备明确的恢复边界。

## 11. 迁移、高可用与回滚

**状态**：部分完成。独立 migrate Job、API/worker RollingUpdate、独立扩缩容和 Helm rollback 已验证；入口多副本、PDB、故障注入和数据库兼容回滚闭环尚未完成。

实施顺序：

1. 独立 migrate Job 已落地；下一步补充数据库迁移锁、失败处理和 expand/contract 兼容策略。
2. 完成 API、Web、Gateway、cloudflared 的多副本配置和 RollingUpdate。
3. 有状态单实例组件继续使用 `Recreate`，避免 RWO PVC 被新旧 Pod 并发写入。
4. worker 默认保持单副本，已完成 source worker 双副本与四类 claim 并发验证。
5. 已完成 SIGTERM、独立扩缩容和 Helm rollback；仍需补充 readiness 失败、节点维护和入口故障演练。

完成判定：

1. 新 Pod ready 前旧 Pod 持续接收流量。
2. 单 Pod 故障不会造成长时间外部不可用。
3. 发布失败可以回到上一稳定镜像和兼容数据库状态。
4. WSL 关闭、Windows 关机和本机断网不纳入当前无感升级承诺。

## 12. CI/CD 闭环

**状态**：尚未实现。当前仅完成手动 Helm lint、template、镜像构建和部署验收。

目标流程：

```text
PR 校验
  -> 后端测试、vet、build
  -> 前端 install、type-check、build
  -> Helm lint/template
  -> 构建 Git SHA 或 SemVer + Git SHA 镜像
  -> 推送镜像仓库
  -> 部署 K3s staging
  -> smoke test
  -> 人工确认
  -> Helm upgrade
  -> 发布后观察和 rollback
```

完成判定：

1. 生产镜像和 Chart 版本可追踪，不使用 `latest`。
2. 独立 migrate Job 成功后才允许发布应用。
3. smoke test 覆盖首页、`/healthz`、`/readyz`、登录、核心 API 和外部入口。
4. 发布失败可通过 `helm rollback` 或镜像 tag 回退。
5. CI/CD 日志记录镜像、Chart、迁移和回滚版本。

## 13. 微服务拆分

**状态**：尚未开始。第 8～12 节全部通过前，不进入真实业务微服务拆分。

拆分顺序：

1. `notification-worker` -> `notification-service`
2. `source-worker` -> `feed-worker-service`
3. `embedding-worker` -> `embedding-service`
4. `agent-scheduler-worker` -> `agent-worker-service`
5. API 中的 Feed 能力 -> `feed-api-service`
6. 认证能力 -> `auth-service`
7. 后续新增金融能力 -> `market-service`

单服务迁移方式：

```text
旧：
  messagefeed-api + APP_ROLE=notification-worker

新：
  messagefeed-notification
  独立 Kubernetes Deployment
```

每次只迁移一个服务：

1. 定义接口、数据访问边界、重试策略、指标和失败处理。
2. 保留旧角色 Deployment 作为回滚基线。
3. 新旧实现短期并存，通过配置或流量策略切换。
4. 新服务稳定后再缩容旧角色。
5. 验证数据结构、任务状态和回滚路径兼容后，再进入下一项。

## 14. 多节点扩展与阶段验收

**状态**：尚未开始，且不是当前微服务化的强制前置条件。

扩展内容：

1. 使用 Tailscale 或 WireGuard 等安全网络接入实验室服务器和低配常驻服务器。
2. 准备 K3s agent join、节点 label、taint/toleration、亲和性和多节点 values。
3. 实验室服务器默认只承载受限 worker，不作为公网入口。
4. 低配常驻服务器用于持续在线兜底；是否承载 control-plane、PostgreSQL 和入口需单独评估。

阶段验收：

1. 第 8～12 节的必要任务全部通过。
2. 新节点加入不影响 WSL 当前 Helm release 和数据卷。
3. 多节点扩展、数据库高可用和 HPA 作为后续增强，不阻塞第一个微服务拆分。
4. 完成第 13 节首个服务验证后，再决定是否推进多节点和数据库 HA。
