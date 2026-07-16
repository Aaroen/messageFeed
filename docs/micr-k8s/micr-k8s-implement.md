## messageFeed Kubernetes 实施文档

**定位**：实施细节、操作步骤、验收口径  
**更新日期**：2026-07-06  
**上位约束**：`micr-k8s-plan.md`

本文档只展开 `micr-k8s-plan.md` 已确定的技术方案，不新增新的架构路线。若本文件与 `micr-k8s-plan.md` 冲突，以 `micr-k8s-plan.md` 为准。

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
- [ ] 确认第一轮不改业务模型、不拆仓库、不直接引入 gRPC/Eino/Nginx Ingress。
- [ ] 确认第一轮重构目标是运行边界，不是业务微服务边界。

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

### 第五部分：完成单镜像多角色容器化

- [ ] 后端继续使用一个镜像承载多角色：`messagefeed-api:<git-sha>`。
- [ ] 前端继续使用独立镜像：`messagefeed-web:<git-sha>`。
- [ ] 禁止生产部署使用 `latest`。
- [ ] 镜像 tag 使用 Git SHA 或 SemVer + Git SHA。
- [ ] 本地验证不同 `APP_ROLE` 的容器启动行为。
- [ ] 确认容器健康检查路径与 K8s 探针一致。

### 第六部分：搭建 WSL 内 K3s single-server 基线

- [ ] 通过 SSH 进入 WSL 后或直接在环境内安装或核实 K3s single-server。
- [ ] 确认 `kubectl get nodes` 中 WSL 节点为 `Ready`。
- [ ] 确认 Helm 可在 SSH 会话内操作 WSL 内 K3s。
- [ ] 规划 WSL 内 StorageClass、数据卷、命名空间和 Secret 管理方式。
- [ ] 部署或接入 WSL 内 PostgreSQL/pgvector。
- [ ] 明确数据库备份落点和恢复验证方式。

### 第七部分：编写 Helm Chart 与 `values-wsl.yaml`

- [ ] 创建 `deploy/helm/messagefeed` 目录结构。
- [ ] 编写 API Deployment / Service。
- [ ] 编写 Web Deployment / Service。
- [ ] 编写 Caddy gateway Deployment / Service。
- [ ] 编写 cloudflared Deployment。
- [ ] 编写 source-worker Deployment。
- [ ] 编写 notification-worker Deployment。
- [ ] 编写 agent-scheduler-worker Deployment。
- [ ] 编写 embedding-worker Deployment。
- [ ] 编写 migrate Job。
- [ ] 编写 ConfigMap 与 Secret 引用。
- [ ] 编写 `values-wsl.yaml`，只描述 WSL 当前阶段所需副本数、资源限制和入口配置。

### 第八部分：部署 WSL 内第一版 K8s 应用

- [ ] 创建 messageFeed 命名空间。
- [ ] 创建数据库 Secret、认证 Secret、LLM/Embedding Secret、企业微信 Secret、Cloudflare Tunnel Secret。
- [ ] 先运行 migrate Job。
- [ ] 部署 PostgreSQL/pgvector 或确认外部 PostgreSQL 连接可用。
- [ ] 部署 API/Web/Gateway。
- [ ] 部署各 worker。
- [ ] 部署 cloudflared。
- [ ] 验证 `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node`。
- [ ] 验证外部 Cloudflare Tunnel 访问链路。

### 第九部分：完成 WSL 内应用级高可用演练

- [ ] 删除单个 API Pod，确认 Service 自动切换。
- [ ] 删除单个 Web Pod，确认页面访问恢复。
- [ ] 删除单个 Gateway Pod，确认入口恢复。
- [ ] 删除单个 cloudflared Pod，确认不出现长期 `1033`。
- [ ] 滚动发布 API，确认旧 Pod 在新 Pod ready 前继续接流量。
- [ ] 人为制造 readiness 失败，确认失败 Pod 不进入 Service endpoints。
- [ ] 验证 worker 多副本不会重复 claim 同一任务。
- [ ] 明确当前阶段不能覆盖 Windows 关机、WSL 停止和本机断网。

### 第十部分：加入 CI/CD 初版

- [ ] PR 阶段执行 `go test`、`go vet`、后端构建。
- [ ] PR 阶段执行前端 install、type-check、build。
- [ ] 构建后端镜像并打 Git SHA tag。
- [ ] 构建前端镜像并打 Git SHA tag。
- [ ] 推送镜像到镜像仓库。
- [ ] 手动触发部署到 WSL 内 K3s。
- [ ] 部署前运行 migrate Job。
- [ ] 部署后执行 smoke test。
- [ ] 失败时支持 Helm rollback 或镜像 tag 回退。

### 第十一部分：准备后续服务器 Agent 扩展

- [ ] 明确实验室服务器只作为受限 worker 节点，不默认作为公网入口。
- [ ] 明确低配服务器作为后续持续在线兜底节点，是否承载 control-plane 和 PostgreSQL 需要单独迁移方案。
- [ ] 准备 Tailscale 或 WireGuard 等安全网络方案。
- [ ] 准备 K3s agent join 脚本。
- [ ] 设计 WSL、实验室服务器、低配服务器的节点 label。
- [ ] 设计实验室服务器 taint/toleration 和安全限制。
- [ ] 准备后续 `values-lab.yaml`、`values-vps.yaml` 或多节点 values。
- [ ] 验证服务器 agent 加入不影响 WSL 当前主线运行。

### 第十二部分：达到微服务重构前置条件

- [ ] WSL 内 K3s 部署稳定。
- [ ] `APP_ROLE` 多角色运行稳定。
- [ ] API/Web/Gateway/cloudflared 滚动发布稳定。
- [ ] worker 幂等、任务锁、job claim 稳定。
- [ ] migrate Job 与应用发布顺序稳定。
- [ ] CI/CD 初版可构建、部署、回滚。
- [ ] 外部访问、健康检查、日志和指标可用于定位问题。
- [ ] 至少完成一轮备份与恢复演练。
- [ ] 明确第一个可拆服务边界，优先从 `notification-service` 或 `feed-worker-service` 开始。

### 第十三部分：开始微服务重构

- [ ] 保留旧 `APP_ROLE` Deployment 作为回滚基线。
- [ ] 为第一个服务定义接口契约、数据访问边界和失败重试策略。
- [ ] 新服务先作为独立镜像和独立 Deployment 部署。
- [ ] 新旧实现短期并存，通过配置或流量策略切换。
- [ ] 新服务验证通过后，缩容旧角色 Deployment。
- [ ] 保留回滚路径，确认数据结构和任务状态兼容旧实现。
- [ ] 第一轮微服务重构完成后，再进入下一个服务。

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

### 3.3 启动装配拆分

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

### 3.4 角色启动行为

| 角色 | 启动 HTTP | 启动 worker |
| --- | --- | --- |
| `api` | 是 | 否 |
| `source-worker` | 否 | 仅 source sync |
| `notification-worker` | 否 | 仅 notification |
| `agent-scheduler-worker` | 否 | 仅 agent scheduled task |
| `embedding-worker` | 否 | 仅 embedding |
| `all` | 是 | 全部 |
| `migrate` | 否 | 仅迁移 |

### 3.5 验收

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

## 4. 镜像设计

### 4.1 后端镜像

第一阶段仍使用一个后端镜像：

```text
messagefeed-api:<git-sha>
```

虽然名字叫 `api`，但它通过 `APP_ROLE` 启动不同后端角色。

原因：

1. 减少第一阶段镜像数量。
2. 避免拆多个二进制。
3. 后续真实拆服务时，再把某个角色替换成独立镜像。

### 4.2 前端镜像

前端独立镜像：

```text
messagefeed-web:<git-sha>
```

### 4.3 镜像 tag

禁止生产使用：

```text
latest
```

推荐：

```text
<git-sha>
<semver>-<git-sha>
```

## 5. Helm Chart 设计

目录：

```text
deploy/helm/messagefeed/
  Chart.yaml
  values.yaml
  values-local.yaml
  values-staging.yaml
  values-prod.yaml
  templates/
    configmap.yaml
    secret.example.yaml
    api-deployment.yaml
    api-service.yaml
    web-deployment.yaml
    web-service.yaml
    gateway-deployment.yaml
    gateway-service.yaml
    cloudflared-deployment.yaml
    source-worker-deployment.yaml
    notification-worker-deployment.yaml
    agent-scheduler-worker-deployment.yaml
    embedding-worker-deployment.yaml
    migrate-job.yaml
```

### 5.1 values 结构

建议结构：

```yaml
global:
  environment: staging
  deploymentMode: cluster
  publicBaseUrl: https://example.com

image:
  api:
    repository: registry.example.com/messagefeed-api
    tag: ""
  web:
    repository: registry.example.com/messagefeed-web
    tag: ""

api:
  replicas: 2

web:
  replicas: 2

gateway:
  replicas: 2

cloudflared:
  replicas: 2
  protocol: auto

workers:
  source:
    replicas: 1
  notification:
    replicas: 1
  agentScheduler:
    replicas: 1
  embedding:
    replicas: 1
```

### 5.2 ConfigMap 字段

ConfigMap 放非敏感配置：

```text
BIND_ADDR
PUBLIC_BASE_URL
APP_ROLE
DEPLOYMENT_MODE
ENVIRONMENT
LOG_LEVEL
OTEL_SERVICE_NAME
OTEL_EXPORTER_OTLP_ENDPOINT
OBSERVABILITY_TRACE_ENABLED
```

### 5.3 Secret 字段

Secret 放敏感配置：

```text
DATABASE_URL
AUTH_OWNER_PASSWORD
LLM_API_KEY
EMBEDDING_API_KEY
WECHAT_WORK_SECRET
WECHAT_WORK_CALLBACK_TOKEN
WECHAT_WORK_ENCODING_AES_KEY
CLOUDFLARED_TUNNEL_TOKEN
```

## 6. Kubernetes Workload 设计

### 6.1 API Deployment

关键环境变量：

```text
APP_ROLE=api
DEPLOYMENT_MODE=cluster
BIND_ADDR=0.0.0.0:60001
APP_NODE_ID=$(POD_NAME)
```

探针：

```text
startupProbe: /healthz
livenessProbe: /healthz
readinessProbe: /readyz
```

发布策略：

```text
maxUnavailable=0
maxSurge=1
```

### 6.2 Web Deployment

Web 只提供静态资源，不直接访问数据库。

副本建议：

```text
local: 1
staging: 1
prod: 2
```

### 6.3 Caddy Gateway Deployment

职责：

1. 接收 cloudflared 转发。
2. `/api/*` 转发到 API Service。
3. 其他路径转发到 Web Service。
4. `/healthz`、`/readyz` 按需要转发 API 或提供 gateway 自身检查。

第一阶段继续使用 Caddy，不引入 Nginx。

### 6.4 cloudflared Deployment

职责：

1. 与 Cloudflare 建立 Tunnel 出站连接。
2. 将外部 hostname 转发到 Caddy gateway Service。

建议：

```text
replicas=2
不使用 HPA
开启 --metrics 0.0.0.0:2000
使用 /ready 探针
协议默认 auto，网络不稳时改 http2
```

### 6.5 Worker Deployments

每类 worker 独立 Deployment：

```text
source-worker
notification-worker
agent-scheduler-worker
embedding-worker
```

共同规则：

1. 不暴露公网 Service。
2. 默认不监听 HTTP。
3. 使用独立 `APP_NODE_ID`。
4. 通过数据库 job claim / lock / 幂等保证任务不重复。

初始副本建议：

| worker | 初始副本 |
| --- | --- |
| source-worker | 1 |
| notification-worker | 1 |
| agent-scheduler-worker | 1 |
| embedding-worker | 1 |

稳定后再按队列积压扩容。

### 6.6 Migrate Job

迁移必须独立于 API 启动。

规则：

1. 发布前先运行迁移 Job。
2. 迁移失败则停止发布。
3. 生产迁移必须向后兼容。
4. 不在生产流水线自动执行破坏性 down migration。

## 7. WSL 内 K3s 长期运行

当前阶段使用 WSL 内 K3s single-server 作为长期运行基线，不使用 K3d 作为主线。

### 7.1 进入项目

所有操作先通过 SSH 进入 WSL：

```bash
ssh aroen@127.0.0.1
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
```

### 7.2 推荐形态

```text
WSL
  K3s server / control-plane
  PostgreSQL/pgvector
  messageFeed namespace
    api Deployment
    web Deployment
    gateway Deployment
    cloudflared Deployment
    source-worker Deployment
    notification-worker Deployment
    agent-scheduler-worker Deployment
    embedding-worker Deployment
    migrate Job
```

K3d 只作为一次性沙盒演练可选项，不作为当前长期运行基线。

### 7.3 环境核实

核实项：

```bash
pwd
git status --short
go version
docker version
kubectl version --client
helm version
```

如果 K3s 尚未安装，先安装 K3s single-server；如果已经安装，先确认当前 kubeconfig 指向 WSL 内 K3s。

### 7.4 集群验收

```bash
kubectl get nodes -o wide
kubectl get pods -A
kubectl get storageclass
```

验收标准：

1. WSL 内只有一个 K3s server 节点也可以接受。
2. 节点状态为 `Ready`。
3. `kubectl` 和 `helm` 在 SSH 会话内可直接操作集群。
4. 本阶段不要求跨物理节点高可用。

### 7.5 应用级高可用验证

本阶段只验证应用级高可用，不代表 Windows 关机、WSL 停止或本机断网后仍可用。

演练目标：

1. Helm chart 可安装。
2. API/Web/Gateway/cloudflared 多副本可运行。
3. 删除单个 Pod 不影响主链路。
4. API rolling update 时外部访问不中断。
5. `cloudflared` 单 Pod 重启不会长时间出现 `1033`。
6. worker 能按角色独立运行。

演练项：

```text
删除一个 api Pod
删除一个 web Pod
删除一个 gateway Pod
删除一个 cloudflared Pod
滚动重启 api Deployment
让一个 api Pod readiness 失败
观察外部 /healthz /readyz
```

验收标准：

1. 单个 API Pod 删除后，外部访问仍成功。
2. 单个 gateway Pod 删除后，外部访问仍成功。
3. 单个 cloudflared Pod 删除后，不出现长期 `1033`。
4. worker 日志显示各自只执行对应职责。
5. API Pod 不再输出 worker tick 日志。

## 8. 后续服务器 Agent 扩展

本章是后续扩展内容，不是当前第一落地阶段的前置条件。

### 8.1 前提

1. WSL 内 K3s single-server 已稳定运行。
2. Helm chart、镜像 tag、Secret、ConfigMap 已在 WSL 环境验证通过。
3. 实验室服务器和低配服务器能通过安全网络访问 WSL control-plane，或已决定将 control-plane 迁移到常驻服务器。
4. 新服务器能访问镜像仓库。
5. 新服务器能访问 PostgreSQL 所在位置。
6. 新服务器能出站访问 Cloudflare。

建议先用 Tailscale 或 WireGuard 打通内网。

### 8.2 一键加入脚本目标

脚本：

```text
scripts/cluster/join-node.sh
```

职责：

1. 安装 K3s agent。
2. 加入当前集群。
3. 设置节点名。
4. 设置节点 labels。
5. 检查节点 Ready。

### 8.3 节点标签

WSL：

```text
messagefeed/site=wsl
messagefeed/node-pool=primary
messagefeed/tunnel=true
messagefeed/gateway=true
messagefeed/worker=true
```

实验室服务器：

```text
messagefeed/site=lab
messagefeed/node-pool=restricted
messagefeed/worker=true
messagefeed/security=restricted
```

低配服务器：

```text
messagefeed/site=vps
messagefeed/node-pool=fallback
messagefeed/always-on=true
```

### 8.4 扩展后调度目标

```text
WSL:
  主力 api / web / worker / cloudflared

实验室服务器:
  高配备份 worker
  按安全策略决定是否承载 api
  默认不作为公网入口

低配服务器:
  后续持续在线兜底
  是否承载 control-plane 和 PostgreSQL 需要单独迁移方案
```

通过 nodeAffinity、taints/tolerations、anti-affinity 或 topology spread 控制关键副本分布。

## 9. 外部访问与 Tunnel 稳定性

### 9.1 访问链路

```text
用户 / 企业微信
  -> Cloudflare
  -> Tunnel
  -> cloudflared Pods
  -> Caddy gateway
  -> web/api
```

### 9.2 `1033` 定位

`1033` 通常表示 Cloudflare 找不到健康 `cloudflared` 连接。

排查顺序：

1. Cloudflare Tunnel 状态。
2. `cloudflared` Pod 状态。
3. `cloudflared /ready`。
4. `cloudflared` 日志。
5. 当前协议是 `auto/quic/http2`。
6. 服务器出站网络。

### 9.3 `502/504` 定位

`502/504` 更常见于内部 origin 访问失败。

排查顺序：

1. gateway Pod 状态。
2. gateway upstream 日志。
3. API/Web Service endpoints。
4. API `/readyz`。
5. 数据库连接。

### 9.4 修复有效性证明

外部探测：

```text
/
/healthz
/readyz
```

指标：

```text
1033 次数
502/504 次数
cloudflared restart 次数
gateway upstream error 次数
api readyz 失败次数
```

验收建议：

```text
连续 14 天无 1033 或显著降低
外部 /healthz 成功率 >= 99.9%
外部 /readyz 成功率 >= 99.5%
单 Pod 故障不造成长时间不可用
```

## 10. CI/CD 实施

### 10.1 PR Workflow

检查：

```text
go test -count=1 ./...
go vet ./...
go build ./cmd/api
npm --prefix web ci
npm --prefix web run test
npm --prefix web run type-check
npm --prefix web run build
docker build
```

### 10.2 Release Workflow

流程：

```text
构建 api 镜像
构建 web 镜像
使用 Git SHA 打 tag
推送镜像
部署 staging
运行 smoke test
人工审批 production
部署 production
发布后观察
```

### 10.3 Smoke Test

```text
GET /
GET /healthz
GET /readyz
登录接口
订阅列表接口
Agent 创建任务接口
企业微信 callback 验证
```

### 10.4 回滚

应用回滚：

```text
helm rollback messagefeed <revision>
```

镜像回滚：

```text
切回上一个 Git SHA tag
```

worker 回滚：

```text
缩容 K8s worker 到 0
恢复旧 worker
确认 claim/lock/幂等状态
```

## 11. 无感升级控制

### 11.1 API/Web/Gateway

策略：

```text
RollingUpdate
maxUnavailable=0
maxSurge=1
readiness 必须通过后才接流量
```

### 11.2 Worker

策略：

1. 初期单副本。
2. 确认幂等后再多副本。
3. 避免旧环境和新环境双跑。
4. 发布时观察队列 claim、失败重试、重复通知、重复抓取。

### 11.3 数据库迁移

采用 expand/contract：

```text
expand:
  新增兼容结构

deploy:
  发布兼容新旧结构的代码

backfill:
  后台补数据

contract:
  下一轮再删除旧结构
```

## 12. 后续微服务拆分实施思路

拆分前置条件：

1. Kubernetes 多角色部署稳定。
2. CI/CD 稳定。
3. 入口和监控稳定。
4. 数据边界明确。
5. 接口契约明确。

推荐拆分顺序：

1. `notification-service`
2. `feed-worker-service`
3. `embedding-service`
4. `agent-worker-service`
5. `feed-api-service`
6. `auth-service`
7. `market-service`

拆分方式：

```text
旧：
  messagefeed-api 镜像 + APP_ROLE=notification-worker

新：
  messagefeed-notification 镜像

Kubernetes Deployment 名称和运行边界尽量保持不变。
```

这样可以把“部署骨架”保留下来，只替换内部实现，减少返工。

## 13. 第一轮任务清单

1. 固定本机操作入口：`ssh aroen@127.0.0.1`。
2. 核实 WSL 项目目录、依赖、Docker、kubectl、Helm。
3. `APP_ROLE` 配置与校验。
4. `internal/bootstrap` 启动装配层。
5. API 与 worker 分离启动。
6. 后端镜像支持多角色。
7. Helm chart 初版。
8. `values-wsl.yaml`。
9. WSL 内 K3s single-server 部署。
10. WSL 内 PostgreSQL/pgvector 部署或接入。
11. migrate Job。
12. Caddy gateway 多副本部署。
13. API/Web 多副本部署。
14. worker 独立 Deployment。
15. cloudflared 多副本部署。
16. WSL 内应用级高可用演练脚本。
17. PR CI workflow。
18. Release workflow。
19. 外部健康探测脚本。
20. 后续服务器 join 脚本。

## 14. 验收总标准

第一轮完成后，应满足：

1. `APP_ROLE=api` 不启动 worker。
2. worker 角色不监听 HTTP。
3. WSL 内 K3s single-server 可稳定运行。
4. API/Web/Gateway/cloudflared 可多副本运行。
5. 删除单个 API/Web/Gateway/cloudflared Pod 不导致长时间不可用。
6. 外部 `/healthz`、`/readyz` 可持续探测。
7. worker 可独立运行并正确 claim job。
8. 发布失败可回滚到上一镜像 tag。
9. 所有本机部署、排查和发布操作都可以通过 `ssh aroen@127.0.0.1` 进入 WSL 后完成。
10. 新服务器可通过脚本加入集群并承载副本作为后续扩展验收，不作为第一轮阻塞项。
