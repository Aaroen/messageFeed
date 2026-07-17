## messageFeed Kubernetes 实施文档

**定位**：实施细节、操作步骤、验收口径
**更新日期**：2026-07-17
**上位约束**：`micr-k8s-plan.md`

本文档只展开 `micr-k8s-plan.md` 已确定的技术方案，不新增新的架构路线。若本文件与 `micr-k8s-plan.md` 冲突，以 `micr-k8s-plan.md` 为准。

## 当前实施状态（2026-07-17）

已完成：

1. WSL 内 K3s single-server、动态网络维护、Helm 工具链和基础运行环境核验。
2. all-in-one 阶段 Helm Chart、`values.yaml` 与 `values-k3s.yaml`。
3. PostgreSQL/pgvector、API、Web、Caddy gateway、cloudflared 和观测栈的 Helm 接管。
4. PostgreSQL 备份校验、5 个 PV 设置为 `Retain`、数据库和公网健康检查验收。
5. 当前 Helm release `messagefeed` revision 2 为 `deployed`；all-in-one API 仍保持单副本。

当前边界：

1. 当前 API 仍由单体进程同时承担 HTTP API、source sync、notification、agent scheduled task 和 embedding worker。
2. 当前数据库迁移由 API Pod 的 init container 执行，尚未改为独立 migrate Job。
3. 当前尚未部署独立 worker、独立 ServiceAccount、最小 RBAC、NetworkPolicy、PDB、HPA 或 CI/CD 发布闭环。
4. `Dockerfile` 已加入 `tini`，但集群仍使用旧的 `messagefeed-api:allinone-0703de0` 镜像，尚未实际启用该改动。
5. 当前集群中 `local-path` 与 `local-path-retain` 均被标记为默认 StorageClass；该冲突需在创建新 PVC 前修正，现有 PVC/PV 不应因此迁移。

后续实施门槛：先完成 StorageClass 默认类校正、Secret 与镜像版本治理，再实施 `APP_ROLE` 和独立迁移 Job；在这些条件完成前不进入真实业务微服务拆分。

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
- [ ] 梳理当前 worker 的任务锁、job claim、幂等、重试和失败记录机制。
- [x] 确认第一轮不改业务模型、不拆仓库、不直接引入 gRPC/Eino/Nginx Ingress。
- [x] 确认第一轮重构目标是运行边界，不是业务微服务边界。

### 第三部分：完成 `APP_ROLE` 启动角色化

- [ ] 新增并校验 `APP_ROLE=api`。
- [ ] 新增并校验 `APP_ROLE=source-worker`。
- [ ] 新增并校验 `APP_ROLE=notification-worker`。
- [ ] 新增并校验 `APP_ROLE=agent-scheduler-worker`。
- [ ] 新增并校验 `APP_ROLE=embedding-worker`。
- [ ] 新增并校验 `APP_ROLE=migrate`。
- [ ] 保留 `APP_ROLE=all` 仅用于本地兼容或过渡。
- [ ] 在 `DEPLOYMENT_MODE=cluster` 下禁止默认使用 `APP_ROLE=all`。
- [ ] 验证 `api` 角色只启动 HTTP，不启动 worker。
- [ ] 验证 worker 角色不监听 HTTP，只执行对应后台职责。

### 第四部分：拆出启动装配层

- [ ] 新增或整理 `internal/bootstrap`。
- [ ] 将配置加载、日志、Tracing、数据库、Repository、Service、Router、Worker 装配拆开。
- [ ] 将数据库迁移与 API/worker 启动解耦。
- [ ] 保证每个角色拥有清晰生命周期和优雅退出逻辑。
- [ ] 保证每个角色拥有可区分的 `APP_NODE_ID`、日志字段和指标标签。
- [ ] 为角色启动行为增加最小测试或命令级验收脚本。

### 第五部分：完成 all-in-one 镜像与容器化基线

- [x] 后端继续使用一个 all-in-one 镜像：`messagefeed-api:allinone-0703de0`。
- [x] 前端继续使用独立镜像：`messagefeed-web:allinone-0703de0`。
- [ ] 禁止生产部署使用 `latest`。
- [ ] 镜像 tag 使用 Git SHA 或 SemVer + Git SHA。
- [ ] 构建并部署包含 `tini` 的新后端镜像。
- [x] 确认当前容器健康检查路径与 K8s 探针一致。

### 第六部分：搭建 WSL 内 K3s single-server 基线

- [x] 通过 SSH 进入 WSL 后或直接在环境内安装或核实 K3s single-server。
- [x] 确认 `kubectl get nodes` 中 WSL 节点为 `Ready`。
- [x] 确认 Helm 可在 SSH 会话内操作 WSL 内 K3s。
- [x] 建立 WSL 内 StorageClass、数据卷、命名空间和 Secret 管理基线；默认 StorageClass 唯一性仍待修正。
- [x] 部署或接入 WSL 内 PostgreSQL/pgvector。
- [x] 明确数据库备份落点和归档校验方式；完整恢复演练仍待执行。

### 第七部分：编写并接管 all-in-one Helm Chart

- [x] 创建 `deploy/helm/messagefeed` 目录结构。
- [x] 编写并接管 API、Web、Caddy gateway 和 cloudflared Deployment/Service。
- [x] 编写并接管 PostgreSQL、Prometheus、Loki、Tempo、OTel Collector、Grafana 和 Promtail。
- [ ] 编写 source-worker、notification-worker、agent-scheduler-worker 和 embedding-worker Deployment。
- [ ] 编写独立 migrate Job；当前仍由 API init container 执行迁移。
- [x] 编写 ConfigMap 与既有 Secret 引用。
- [x] 使用 `values.yaml` 与 `values-k3s.yaml` 描述 all-in-one WSL/K3s 环境。
- [x] 建立 `local-path-retain` StorageClass 模板；现有默认类冲突仍待修正。

### 第八部分：环境与资产治理

- [x] 完成 all-in-one Helm 部署、namespace、Secret 引用、PVC/PV 和公网健康检查基线。
- [ ] 修正 `local-path` 与 `local-path-retain` 双默认 StorageClass。
- [ ] 固定 cloudflared 镜像版本或 digest，完成默认凭据和 Secret 治理。
- [ ] 完成 PostgreSQL 备份恢复演练。

### 第九部分：应用运行边界拆分

- [ ] 梳理 worker 任务锁、claim、幂等、重试和失败记录。
- [ ] 实现 `APP_ROLE` 和启动装配层。
- [ ] 构建并部署包含 `tini` 的新镜像。
- [ ] 验证 API、各类 worker 和 migrate 可独立启动、停止和观测。

### 第十部分：Kubernetes 安全与资源治理

- [ ] 为 API、worker 和 migrate 配置独立 ServiceAccount 与最小 RBAC。
- [ ] 增加 NetworkPolicy、资源请求/限制、PDB、ResourceQuota 和 LimitRange。
- [ ] 验证网络访问、权限边界、资源边界和故障预算。

### 第十一部分：迁移、高可用与回滚

- [ ] 将 API init container 迁移改为独立 migrate Job。
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

当前 all-in-one 过渡阶段实际使用 `messagefeed-api:allinone-0703de0`，尚未启用 `APP_ROLE` 角色化。后续构建新镜像时应使用不可变 tag，并验证 `tini` 已作为容器 PID 1 的父进程生效。

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
  -> all-in-one API / Web / Caddy gateway / cloudflared
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

当前遗留项：

1. `local-path` 与 `local-path-retain` 同时被标记为默认 StorageClass，创建新 PVC 前必须恢复唯一默认类。
2. 现有 PVC/PV 不迁移；5 个现有 PV 已为 `Retain`，完整恢复演练仍待执行。

## 7. Helm Chart 与 Workload 设计

**状态**：all-in-one Chart 已完成并用于现有资源接管；独立 worker、独立 migrate Job 和 `APP_ROLE` 模板尚未实现。

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
3. all-in-one API 保持单副本，直到 `APP_ROLE` 和任务幂等验证完成。
4. 当前 `values-k3s.yaml` 仍将 cloudflared 镜像 tag 覆盖为 `latest`，后续必须固定为版本或 digest。
5. 数据库迁移当前由 API init container 执行，独立 migrate Job 属于后续目标。

Workload 边界：

| Workload | 当前状态 | 目标状态 |
| --- | --- | --- |
| API | all-in-one 单副本，同时启动 HTTP 和 worker | `APP_ROLE=api`，只提供 HTTP |
| source/notification/agent/embedding worker | 未独立部署 | 各自独立 Deployment |
| migrate | API init container | 独立 Job |
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

**状态**：all-in-one Helm 部署、数据库接管、观测栈、入口和基础健康检查已完成；资产治理仍有待办。

当前基线：

| 项目 | 状态 |
| --- | --- |
| Helm release | `messagefeed` revision 2，`deployed` |
| PostgreSQL | 迁移状态 `37,false`，pgvector 可用 |
| PVC/PV | 5 个 PVC 为 `Bound`，5 个 PV 为 `Retain` |
| 外部入口 | Cloudflare -> cloudflared -> Caddy -> Web/API，公网 `/healthz` 已通过 |
| 镜像 | all-in-one 仍使用旧 tag；cloudflared 当前配置仍覆盖为 `latest` |
| StorageClass | `local-path` 与 `local-path-retain` 同时为默认类 |

实施内容：

1. 将 `local-path` 默认注解设为 `false`，确认 `local-path-retain` 为唯一默认类；不迁移现有 PVC/PV。
2. 固定 cloudflared 镜像版本或 digest，构建并部署包含 `tini` 的新 API 镜像。
3. 将 Grafana 管理凭据和其他敏感配置统一纳入 Secret 管理。
4. 完成 PostgreSQL 备份恢复演练，并记录恢复后的迁移状态和核心数据校验结果。

完成判定：

1. 新 PVC 不再依赖错误的默认 StorageClass。
2. 生产镜像不使用 `latest`，敏感配置不使用默认值。
3. 备份可以恢复，恢复后的应用健康检查和数据核验通过。

## 9. 应用运行边界拆分

**状态**：尚未完成。当前 API 仍是 all-in-one 单体进程，同时启动 HTTP API 和后台 worker。

实施内容：

1. 梳理 source、notification、agent scheduler 和 embedding worker 的任务锁、claim、幂等、重试和失败记录。
2. 实现 `APP_ROLE` 和启动装配层；详细角色定义见第 3、4 节。
3. 让 API、各类 worker 和 migrate 具备独立启动、停止、日志、指标和优雅退出能力。
4. 构建并部署包含 `tini` 的新镜像，验证容器 PID 1 能回收孤儿进程。

完成判定：

1. `APP_ROLE=api` 只启动 HTTP，不启动 worker。
2. 各 worker 只执行自身职责，不监听业务 HTTP。
3. 多 worker 并发不会重复处理、重复通知或丢失任务。
4. API 与 worker 可独立扩缩容和回滚。

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

**状态**：尚未完成。当前只有探针、Helm 接管和单副本基础，不代表已实现无感升级。

实施顺序：

1. 将 API init container 迁移改为独立 migrate Job，明确数据库锁、失败处理和向后兼容策略。
2. 完成 API、Web、Gateway、cloudflared 的多副本配置和 RollingUpdate。
3. 有状态单实例组件继续使用 `Recreate`，避免 RWO PVC 被新旧 Pod 并发写入。
4. worker 初期保持单副本，完成幂等验证后再扩容。
5. 完成单 Pod 删除、readiness 失败、滚动发布和 Helm rollback 演练。

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
