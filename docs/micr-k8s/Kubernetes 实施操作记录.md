## messageFeed Kubernetes 实施操作记录

**项目路径**：`/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`  
**关联文档**：`docs/micr-k8s/micr-k8s-plan.md`、`docs/micr-k8s/micr-k8s-implement.md`  
**记录创建日期**：2026-07-06  
**当前状态**：第一部分环境基线、K3s 安装、K3s 动态网络持久化与 Helm 安装已完成；第二部分只读代码职责核查已执行并回填；第三部分 all-in-one 过渡部署、Cloudflare Tunnel、观测栈与固定本地监控入口已完成；第四部分已建立 `deploy/helm/messagefeed` Helm Chart，并于 2026-07-16 使用 `--take-ownership` 完成现有 K3s 资源接管；第八部分环境与资产治理已于 2026-07-18 完成。当前 Helm release `messagefeed` revision 3 为 `deployed`，5 个 PVC 均为 `Bound`、对应 PV 均为 `Retain`，`local-path-retain` 为唯一默认 StorageClass；固定镜像、Grafana Secret 和 PostgreSQL 完整恢复演练均已通过，尚未实施 `APP_ROLE` 多运行角色拆分。

### 记录原则

1. 所有实施、核查、命令执行、配置变更和验证结果均追加记录。
2. 涉及敏感信息时只记录来源、字段名和校验结论，不记录明文 Secret、token、密码或私钥。
3. 所有路径记录优先使用相对路径；必要时记录项目绝对路径以确认 WSL 执行基线。
4. 不记录未经执行的命令为已完成事项，计划与实际执行结果分开维护。
5. 不执行文件删除、资源删除、回滚等可能导致文件或资源消失的操作，除非获得明确指令。

### 已阅读文档

| 时间 | 文档 | 结论 |
| --- | --- | --- |
| 2026-07-06 | `docs/micr-k8s/micr-k8s-plan.md` | 当前方案为 WSL 内 K3s single-server 基线，先进行单体多运行角色、Kubernetes 分布式部署与入口高可用，再考虑业务微服务拆分。 |
| 2026-07-06 | `docs/micr-k8s/micr-k8s-implement.md` | 实施顺序为固定 WSL 执行入口与项目基线、核实现有代码职责、`APP_ROLE` 角色化、启动装配拆分、镜像与 Helm、K3s、部署与高可用演练、CI/CD 和后续服务器扩展。 |

### 当前约束摘要

1. 当前阶段采用 WSL 内 K3s single-server 长期运行方案，不使用 K3d 作为主线。
2. 统一执行入口为 Linux/WSL 环境；若不在 Linux 内，则通过 `ssh aroen@127.0.0.1` 进入 WSL。
3. 项目目录为 `/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`。
4. 第一阶段不直接拆业务微服务，不引入 Nginx Ingress，不把数据库复杂高可用作为主目标。
5. 第一阶段目标是运行边界清晰化，包括 `api`、`source-worker`、`notification-worker`、`agent-scheduler-worker`、`embedding-worker`、`migrate`。
6. 当前阶段不承诺 Windows 关机、WSL 停止或本机断网后持续在线。
7. Cloudflare Tunnel、Caddy gateway、API、Web 后续应纳入 K3s 管理，并通过多副本和探针降低单 Pod 故障影响。

### 操作记录

| 时间 | 类型 | 操作 | 结果 |
| --- | --- | --- | --- |
| 2026-07-06 | 文档核查 | 列出 `docs/micr-k8s` 下文档 | 确认包含 `micr-k8s-plan.md` 与 `micr-k8s-implement.md`。 |
| 2026-07-06 | 文档阅读 | 阅读 `micr-k8s-plan.md` | 已提取总体技术方案、K3s 基线、入口、数据库、CI/CD、无感升级与后续微服务边界。 |
| 2026-07-06 | 文档阅读 | 阅读 `micr-k8s-implement.md` | 已提取十三部分实施顺序、K3s 安装核查、Helm、应用部署、高可用演练与验收标准。 |
| 2026-07-06 | 记录准备 | 创建本操作记录文档 | 用于后续持续追加记录；未执行安装、部署或代码改造。 |
| 2026-07-06 | 计划写入 | 写入第二部分“核实现有代码职责与改造切入点”的待执行步骤与命令 | 仅记录将要执行的只读核查命令；尚未执行代码改造、业务部署或 TODO 勾选。 |
| 2026-07-06 | 只读核查 | 执行第二部分 C1-C6 代码职责、配置、worker claim、边界与测试基线核查 | 已回填 C1-C6；未修改业务代码、部署资源或 `docs/micr-k8s/micr-k8s-implement.md` 勾选状态。 |
| 2026-07-06 | 数据备份 | 执行 Docker Compose PostgreSQL 逻辑备份 | 已生成 custom dump、sha256 与非敏感 metadata；备份可由 `pg_restore -l` 解析。 |
| 2026-07-06 | 方案修订 | 修订第三部分前置过渡部署方案 | 由“只部署后端单 Pod”调整为“同步部署后端 all-in-one、Web、Caddy gateway、Cloudflare Tunnel 与观测组件”；后端仍保持单副本。 |
| 2026-07-07 | 执行前核查 | 核查第三部分前置过渡部署流程 | 已修正初始 tracing 顺序、观测组件 PVC 写权限和 port-forward 验收说明；未执行 K8s 资源创建。 |
| 2026-07-07 | 数据修复与备份 | 清理 Docker PostgreSQL 中 `items(source_id, normalized_url)` 重复数据并重建唯一索引 | 已删除 5 条重复 `items` 记录；唯一索引 `uq_items_source_normalized_url` 已 `REINDEX`；重复组为 0；已生成新备份 `messagefeed-postgres-docker-20260707-201556.dump`。 |
| 2026-07-16 | Helm Chart 实现 | 建立 all-in-one 阶段 Helm Chart，并补充 `tini`、迁移、Retain StorageClass、核心服务和可选观测栈模板 | Chart lint、template、服务端 dry-run 与镜像构建验证通过；代码提交 `5b77e43` 已推送至 `origin/master`。 |
| 2026-07-16 | 数据保护 | 在 Helm 接管前生成 PostgreSQL custom dump，并将现有 PV 回收策略改为 `Retain` | 备份大小 7.6 MiB，SHA-256 与 `pg_restore -l` 校验通过；5 个现有 PV 均保持 `Bound` 且回收策略为 `Retain`。 |
| 2026-07-16 | Helm 接管 | 使用 `--take-ownership` 接管现有 all-in-one、PostgreSQL、Web、gateway、cloudflared 与观测组件 | revision 1 因 StatefulSet 不可变字段失败；修正 `volumeClaimTemplates` 与持久化组件更新策略后，revision 2 成功部署。 |
| 2026-07-16 | 接管验收 | 验证数据库、PVC/PV、内部服务端点、Cloudflare Tunnel 与公网健康检查 | PostgreSQL `schema_migrations=37,false`、公共表 55；全部 11 个 Pod Ready，核心服务 HTTP 200，公网 `/healthz` HTTP 200。 |
| 2026-07-17 | 当前状态复核 | 只读复核 Helm release、PVC/PV 与 StorageClass 状态，并同步实施文档 | Helm release 仍为 revision 2/deployed；5 个 PVC 为 `Bound`、5 个 PV 为 `Retain`；`local-path` 与 `local-path-retain` 同时为默认类，已记录为待修正项；本次未修改集群资源。 |
| 2026-07-18 | 环境与资产治理 | 完成唯一默认 StorageClass、固定镜像、Grafana Secret、API `tini` 和 PostgreSQL 恢复演练 | Helm revision 3/deployed；全部主要 Pod Ready；公网健康检查通过；隔离恢复库的数据、迁移、扩展、索引和约束核验通过。 |

## 第一部分：固定 WSL 执行入口与项目基线

**状态**：基线核查、K3s 安装、K3s 动态网络持久化、Helm 安装与运行信息记录已完成；Docker 组数据库已包含 `aroen`，当前 Codex shell 仍需新组会话才可直接访问 Docker。当前可进入第二部分前的用户核实与 TODO 勾选阶段。  
**反馈时间**：2026-07-06 20:14 CST  
**执行性质**：基线核查、K3s 安装记录、运行时网络问题复盘、K3s 动态网络持久化执行与验收、Helm/Docker 状态核查、运行方式与敏感配置来源记录。当前尚未执行业务部署、代码修改、业务镜像构建、项目级 Kubernetes 资源创建、文件删除或回滚操作。  
**记录方式**：本章节同时记录计划目标、核查命令、实际反馈与判定；确认通过后再到 `docs/micr-k8s/micr-k8s-implement.md` 勾选对应 TODO。  
**执行边界**：第一部分仅处理执行入口、项目基线、K3s/Helm 工具链、K3s 网络可用性与安装验收；不创建项目级 Kubernetes namespace、Secret、ConfigMap 或 Workload，不改业务代码，不构建业务镜像，不部署 PostgreSQL、gateway、cloudflared、API、Web 或 worker。

### 目标

建立后续实施的可重复执行基线，确认当前 shell、项目路径、工具链、运行资产、敏感配置来源和阶段边界。

### A1. 确认当前环境是否为 Linux/WSL

核查命令：

```bash
uname -a
cat /etc/os-release
pwd
```

反馈：

1. 当前内核为 WSL2：`Linux Aroen 6.6.87.2-microsoft-standard-WSL2 ... x86_64 GNU/Linux`。
2. 当前发行版为 Ubuntu 24.04.3 LTS。
3. 当前路径为 `/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`。

判定：

1. 已位于 Linux/WSL 环境。
2. 无需通过额外 SSH 再进入 WSL。

### A2. 进入项目目录并确认路径

核查命令：

```bash
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
pwd
```

反馈：

1. `pwd` 输出为 `/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`。

判定：

1. 项目路径与实施文档一致。

### A3. 记录 Git 基线

核查命令：

```bash
git rev-parse --show-toplevel
git rev-parse --short HEAD
git status --short
```

反馈：

1. Git 仓库根目录为 `/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`。
2. 当前短提交号为 `0703de0`。
3. `git status --short` 本次无输出。

判定：

1. 当前项目工作区在本次核查时未显示未提交变更。
2. 未执行回退、删除或清理操作。

### A4. 核实项目关键资产

核查命令：

```bash
ls -ld Dockerfile docker-compose.yml migrations deploy/caddy ops/observability
```

反馈：

1. `Dockerfile` 存在。
2. `docker-compose.yml` 存在。
3. `migrations` 存在。
4. `deploy/caddy` 存在。
5. `ops/observability` 存在。

判定：

1. 第一部分要求的关键资产齐全。
2. 当前无缺失项需要在第一部分修复。

### A5. 核实基础命令版本

核查命令：

```bash
go version
docker version
kubectl version --client
helm version
git --version
```

反馈：

1. `go`：`go version go1.26.1 linux/amd64`。
2. `docker`：客户端已安装，版本为 Docker Engine Community 29.5.3；普通用户访问 Docker socket 时权限不足。
3. `kubectl`：当前未安装，命令不存在。
4. `helm`：当前未安装，命令不存在。
5. `git`：`git version 2.43.0`。

判定：

1. 需要安装 K3s 以提供 Kubernetes 与 `kubectl` 能力。
2. 需要单独安装 Helm。
3. Docker 普通用户权限不足不阻塞 K3s 安装，因为 K3s 默认使用 containerd；但后续本地镜像构建可能需要单独处理 Docker 用户权限。

### A6. 核实 Docker 当前可用性

核查命令：

```bash
docker info
sudo docker info --format 'Server Version: {{.ServerVersion}}\nStorage Driver: {{.Driver}}'
```

反馈：

1. 使用 sudo 只读核查 Docker daemon 可访问。
2. Docker Server Version：29.5.3。
3. Storage Driver：overlayfs。
4. 普通用户直接执行 Docker 命令时提示无法访问 `/var/run/docker.sock`。

判定：

1. Docker daemon 本身可用。
2. 当前用户 Docker socket 权限不足，记录为后续镜像构建前置风险。
3. 本节未修改用户组或 Docker 配置。

### A7. 执行 K3s 与 Helm 安装教程

**执行人**：用户  
**目标**：在当前 WSL2 Ubuntu 24.04.3 环境安装 K3s single-server，并安装 Helm。  
**原则**：先安装 K3s，再安装 Helm；K3s 会提供 `kubectl` 能力，当前阶段不需要先单独安装上游 `kubectl`。  
**参考资料**：K3s 官方 Quick Start（https://docs.k3s.io/quick-start）、K3s 安装配置文档（https://docs.k3s.io/installation/configuration）、Helm 官方安装文档（https://helm.sh/docs/intro/install/）、Kubernetes 官方 kubectl 安装文档（https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/）。

#### 当前状态

**反馈时间**：2026-07-06 20:14 CST  
**当前结论**：K3s 已安装，K3s 动态 IP 与路由持久化方案已执行并验收通过；Helm 已安装并可访问当前集群；Docker 组数据库已包含 `aroen`，但当前 Codex shell 的有效组尚未刷新，直接 `docker ps` 仍提示权限不足，使用 `sg docker` 可访问 Docker。`coredns`、`local-path-provisioner`、`metrics-server` 均为 `1/1 Running`。  
**处理顺序**：第一部分工具链验收已完成；后续进入第二部分前，由用户根据核实结果更新 `docs/micr-k8s/micr-k8s-implement.md` 的对应 TODO。

已确认状态：

1. `k3s --version` 输出 `v1.36.2+k3s1`。
2. `kubectl version --client` 可用。
3. `kubectl config current-context` 为 `default`。
4. 初次 `kubectl get nodes -o wide` 中节点 `aroen` 为 `Ready`，但 Internal IP 为 `198.18.0.1`；该地址不是预期的 WSL 业务网卡地址。
5. 修复前 `kubectl get pods -A` 中 `coredns` 为 `0/1 Running`，`local-path-provisioner` 为 `Error`，`metrics-server` 为 `CrashLoopBackOff`。
6. 修复后 `kubectl get pods -n kube-system -o wide` 中：
   - `coredns` 为 `1/1 Running`。
   - `local-path-provisioner` 为 `1/1 Running`。
   - `metrics-server` 为 `1/1 Running`。
7. `kubectl get apiservice v1beta1.metrics.k8s.io` 显示 `Available=True`。
8. `kubectl top nodes` 可返回节点指标。
9. `local-path` StorageClass 已存在。
10. `helm version --short` 输出 `v3.21.2+g1259634`，`helm list -A` 可访问当前集群且当前为空列表。
11. `docker` 组数据库中已包含 `aroen`；当前 Codex shell 的 `id` 输出尚未包含 `docker`，直接 `docker ps` 仍提示 Docker socket 权限不足，`sg docker -c "docker ps"` 可列出容器。
12. `k3s-wsl-config.service`、`k3s-wsl-routes.service`、`k3s-wsl-reconcile.timer` 已启用并通过验收。

问题判断：

1. K3s 核心 Pod 日志显示访问 Kubernetes Service IP 超时：`Get "https://10.43.0.1:443/...": dial tcp 10.43.0.1:443: i/o timeout`。
2. `kubectl get endpoints kubernetes -o wide` 显示 apiserver endpoint 为 `198.18.0.1:6443`。
3. 当前 WSL 网络接口中存在 `eth2` 地址 `192.168.3.40/24`，但 K3s 自动选择了 `198.18.0.1` 作为节点 Internal IP。
4. Helm APT 失败不是包名错误，而是当前网络下 `baltocdn.com` 的 APT InRelease 校验失败：`Clearsigned file isn't valid, got 'NOSPLIT'`。
5. Docker 权限问题是会话未刷新用户组；`docker` 组已经包含 `aroen`，但当前 shell 的有效组列表尚未包含 `docker`。
6. CoreDNS 日志中的 `No files matching import glob pattern: /etc/coredns/custom/*.override` 与 `*.server` 是默认扩展配置为空时的提示；在 `coredns` 已为 `1/1 Running` 的情况下，不作为故障处理。
7. metrics-server 的关键故障来自 WSL 策略路由：`10.43.0.1` 被 `10.0.0.0/7 via 198.18.0.2 dev eth0` 覆盖，同时 table `128` 将当时的 WSL 主机地址 `192.168.3.40` 指向 `169.254.73.152`，导致 Pod/CNI 源地址访问 apiserver 超时。
8. 单纯把 `node-ip` 固定为 `192.168.3.40` 只能解决当前会话。WSL 重启或网络恢复后，`eth2` 的 IPv4 地址可能变化，因此最终方案应动态读取当前 `eth2` 地址并生成 K3s 配置与路由。

#### B1. 安装前确认

在项目目录执行：

```bash
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
ps -p 1 -o comm=
systemctl is-system-running || true
systemctl --failed --no-pager || true
command -v k3s || true
command -v kubectl || true
command -v helm || true
ss -ltn | awk '$4 ~ /:(6443|10250|8472|51820|51821)$/ { print $0 }'
```

预期：

1. PID 1 为 `systemd`。
2. `k3s`、`kubectl`、`helm` 当前不存在或待安装。
3. `6443` 等 K3s 常见端口没有被已有服务占用。

说明：

1. 本次核查中 `systemctl is-system-running` 为 `degraded`，失败单元为 `csc.service` 与 `messagefeed-dev.service`。这不是 K3s 安装的必然阻塞项，但若 K3s 安装后启动异常，需要优先查看 systemd 日志。

#### B2. 安装 K3s single-server

推荐命令：

```bash
curl -sfL https://get.k3s.io | sudo env INSTALL_K3S_CHANNEL=stable INSTALL_K3S_EXEC="server --write-kubeconfig-mode 644 --disable traefik" sh -
```

参数说明：

1. `INSTALL_K3S_CHANNEL=stable`：使用 K3s stable channel。
2. `server`：安装 single-server control-plane。
3. `--write-kubeconfig-mode 644`：允许当前用户读取 kubeconfig，便于后续执行 `kubectl`。
4. `--disable traefik`：当前方案使用 Caddy gateway，不以 Traefik Ingress 作为第一阶段入口。

暂不建议：

1. 暂不启用多 server 高可用。
2. 暂不加入 agent 节点。
3. 暂不引入 Nginx Ingress。
4. 暂不修改项目代码或 Helm chart。

#### B3. 验证 K3s 服务

执行：

```bash
sudo systemctl status k3s --no-pager
sudo journalctl -u k3s -n 80 --no-pager
sudo k3s kubectl get nodes -o wide
sudo k3s kubectl get pods -A
sudo k3s kubectl get storageclass
```

通过标准：

1. `k3s` systemd 服务为 `active`。
2. `sudo k3s kubectl get nodes -o wide` 中节点状态为 `Ready`。
3. `kube-system` 组件处于 `Running` 或 `Completed`，无持续 `CrashLoopBackOff`。
4. 存在默认 StorageClass，通常为 `local-path`。

#### B3.1 K3s 网络失败原因复盘

本次 K3s 安装后出现三类现象：

1. `coredns` 曾经 `0/1 Running`，日志出现 `plugin/ready: Plugins not ready: "kubernetes"`。
2. `local-path-provisioner` 曾经 `Error`。
3. `metrics-server` 曾经 `CrashLoopBackOff`，日志出现 `dial tcp 10.43.0.1:443: i/o timeout`。

根因链路：

1. K3s 初次启动时自动选择了 `198.18.0.1` 作为节点 Internal IP，而不是 WSL 正常业务网卡 `eth2` 的 IPv4 地址。
2. 因为 apiserver endpoint 初始指向 `198.18.0.1:6443`，核心 Pod 访问 Kubernetes Service `10.43.0.1:443` 时无法稳定连接 apiserver。
3. 修正 K3s 节点 IP 后，`kubernetes` endpoint 已变为当时的 `192.168.3.40:6443`，但 metrics-server 仍然无法通过 `10.43.0.1:443` 访问 apiserver。
4. 后续排查发现，WSL 路由中存在 `10.0.0.0/7 via 198.18.0.2 dev eth0`，覆盖了 K3s 默认 Service CIDR `10.43.0.0/16`。
5. 同时 WSL 策略路由 table `128` 将当时的 WSL 主机地址 `192.168.3.40` 指向 `169.254.73.152`，导致来自 CNI/Pod 源地址的连接被错误导出，而不是回到本机 apiserver。
6. 因此，metrics-server 的异常不是 metrics-server 自身配置错误，而是 Pod 到 Kubernetes Service IP、再 DNAT 到 apiserver endpoint 的网络路径错误。

非故障项：

1. CoreDNS 日志中的 `No files matching import glob pattern: /etc/coredns/custom/*.override` 与 `No files matching import glob pattern: /etc/coredns/custom/*.server` 是 K3s 默认 CoreDNS 配置预留自定义扩展目录时的提示。
2. 在 `coredns` 已经 `1/1 Running` 的情况下，上述 CoreDNS 提示不需要修复。

#### B3.2 动态持久化 K3s 网络修复最终方案

**执行时间**：2026-07-06 20:05-20:14 CST  
**执行状态**：已执行并验收通过。  
**最终结论**：K3s 节点 IP、Kubernetes Service CIDR 路由和 WSL table `128` 本地路由已改为 systemd 动态维护。K3s 重启后会自动读取当前 `eth2` IPv4；网络恢复或 IP 变化后由 timer 触发条件核查。

结论：

1. IP 可以自动获取更新，不应继续把 `192.168.3.40` 等某一次 WSL 会话中的地址写死到 K3s 配置中。
2. WSL 完整启动或 K3s 服务重启时，由 `k3s-wsl-config.service` 在 `k3s.service` 启动前动态读取当前 WSL 主网卡 IPv4，并重写 `/etc/rancher/k3s/config.yaml`。
3. K3s 启动后，由 `k3s-wsl-routes.service` 等待 `cni0` 出现，并动态恢复 `10.43.0.0/16` 到 `cni0` 的路由，以及 table `128` 中当前 WSL 主机 IP 的本地路由。
4. 同一 WSL 会话内发生网络恢复或 IP 变化时，由 `k3s-wsl-reconcile.timer` 周期性触发条件核查：若当前 `eth2` IPv4 与 Kubernetes Service endpoint 不一致，则重启 K3s；若一致，则只重新应用路由。
5. 该方案是动态 IP 方案，不是静态固定 IP 方案；但当前环境已确认主网卡名称为 `eth2`，若后续 WSL 网卡名称变化，需要更新 `/etc/default/k3s-wsl-network` 中的 `K3S_WSL_IFACE`。

目标：

1. 不固定写死 `192.168.3.40`。
2. 每次 WSL 启动、K3s 重启或网络恢复核查时，自动读取当前 WSL 主网卡 IPv4。
3. 在 K3s 启动前写入动态 `node-ip` 与 `advertise-address`。
4. 在 K3s 启动后等待 `cni0` 出现，并自动恢复 K3s Service CIDR 与 WSL table `128` 的本地路由。
5. 通过条件核查避免无条件频繁重启 K3s；只有 endpoint IP 与当前 WSL IP 不一致时才重启 K3s。
6. 不创建额外 `.sh` 脚本文件，使用 systemd unit 内联命令维护。

##### 1. 创建统一网络参数文件

```bash
sudo tee /etc/default/k3s-wsl-network >/dev/null <<'EOF'
K3S_WSL_IFACE=eth2
K3S_SERVICE_CIDR=10.43.0.0/16
EOF
```

说明：

1. `K3S_WSL_IFACE=eth2` 表示优先使用 WSL 当前业务网卡。
2. 若后续 WSL 网卡名称变化，只需要修改该文件。
3. `K3S_SERVICE_CIDR=10.43.0.0/16` 与当前 K3s 默认 Service CIDR 一致。

##### 2. 创建 K3s 动态配置生成服务

```bash
sudo tee /etc/systemd/system/k3s-wsl-config.service >/dev/null <<'EOF'
[Unit]
Description=Generate dynamic K3s config for WSL network
After=network-online.target
Wants=network-online.target
Before=k3s.service

[Service]
Type=oneshot
EnvironmentFile=-/etc/default/k3s-wsl-network
ExecStart=/bin/sh -ec 'IFACE="$${K3S_WSL_IFACE:-eth2}"; HOST_IP="$$(/usr/sbin/ip -4 -o addr show dev "$$IFACE" scope global | /usr/bin/tr -s " " | /usr/bin/cut -d " " -f 4 | /usr/bin/cut -d / -f 1 | /usr/bin/head -n 1)"; test -n "$$HOST_IP"; /usr/bin/install -d -m 0755 /etc/rancher/k3s; { /usr/bin/echo "write-kubeconfig-mode: 0644"; /usr/bin/echo "node-ip: $$HOST_IP"; /usr/bin/echo "advertise-address: $$HOST_IP"; /usr/bin/echo "flannel-iface: $$IFACE"; /usr/bin/echo "disable:"; /usr/bin/echo "  - traefik"; } > /etc/rancher/k3s/config.yaml'

[Install]
WantedBy=multi-user.target
EOF
```

说明：

1. 该服务在 K3s 启动前运行。
2. 它动态读取 `eth2` 当前 IPv4，并写入 `/etc/rancher/k3s/config.yaml`。
3. 该方式避免把 `node-ip` 与 `advertise-address` 固定为某一次 WSL 会话中的地址。

##### 3. 增加 K3s 对动态配置服务的依赖

```bash
sudo mkdir -p /etc/systemd/system/k3s.service.d
sudo tee /etc/systemd/system/k3s.service.d/10-wsl-network.conf >/dev/null <<'EOF'
[Unit]
Requires=k3s-wsl-config.service
After=k3s-wsl-config.service
Wants=k3s-wsl-routes.service
EOF
```

说明：

1. 该 drop-in 让 `k3s.service` 在启动前先执行动态配置生成。
2. 该 drop-in 同时让 `k3s.service` 启动时拉起 `k3s-wsl-routes.service`，由路由服务在 K3s 启动后恢复 WSL 路由。
3. 不直接改写 `/etc/systemd/system/k3s.service` 主文件，降低后续 K3s 升级冲突。

##### 4. 创建 K3s WSL 路由恢复服务

```bash
sudo tee /etc/systemd/system/k3s-wsl-routes.service >/dev/null <<'EOF'
[Unit]
Description=Restore WSL-specific routes for K3s Service CIDR
After=k3s.service
Requires=k3s.service

[Service]
Type=oneshot
EnvironmentFile=-/etc/default/k3s-wsl-network
ExecStartPre=/bin/sh -ec 'for i in $$(seq 1 90); do /usr/sbin/ip link show cni0 >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1'
ExecStart=/bin/sh -ec 'IFACE="$${K3S_WSL_IFACE:-eth2}"; HOST_IP="$$(/usr/sbin/ip -4 -o addr show dev "$$IFACE" scope global | /usr/bin/tr -s " " | /usr/bin/cut -d " " -f 4 | /usr/bin/cut -d / -f 1 | /usr/bin/head -n 1)"; CNI_IP="$$(/usr/sbin/ip -4 -o addr show dev cni0 scope global | /usr/bin/tr -s " " | /usr/bin/cut -d " " -f 4 | /usr/bin/cut -d / -f 1 | /usr/bin/head -n 1)"; SERVICE_CIDR="$${K3S_SERVICE_CIDR:-10.43.0.0/16}"; test -n "$$HOST_IP"; test -n "$$CNI_IP"; /usr/sbin/ip route replace "$$SERVICE_CIDR" dev cni0 src "$$CNI_IP"; /usr/sbin/ip route replace local "$$HOST_IP/32" dev lo table 128'

[Install]
WantedBy=multi-user.target
EOF
```

说明：

1. 该服务在 K3s 启动后运行。
2. 它等待 `cni0` 出现，再动态读取 `cni0` 地址作为 Service CIDR 路由源地址。
3. 它动态读取 WSL 主网卡 IPv4，并将该主机地址加入 table `128` 的本地路由。
4. 该服务不再周期性重启 metrics-server，避免网络核查定时器造成不必要的组件滚动重启。

##### 5. 创建网络恢复自动核查服务与定时器

```bash
sudo tee /etc/systemd/system/k3s-wsl-reconcile.service >/dev/null <<'EOF'
[Unit]
Description=Reconcile WSL network changes for K3s
After=network-online.target k3s.service
Wants=network-online.target

[Service]
Type=oneshot
EnvironmentFile=-/etc/default/k3s-wsl-network
ExecStart=/bin/sh -ec 'systemctl is-active --quiet k3s.service || exit 0; IFACE="$${K3S_WSL_IFACE:-eth2}"; HOST_IP="$$(/usr/sbin/ip -4 -o addr show dev "$$IFACE" scope global | /usr/bin/tr -s " " | /usr/bin/cut -d " " -f 4 | /usr/bin/cut -d / -f 1 | /usr/bin/head -n 1)"; test -n "$$HOST_IP"; /usr/bin/systemctl start k3s-wsl-routes.service || true; EP_IP="$$(/usr/local/bin/k3s kubectl get endpoints kubernetes -o jsonpath="{.subsets[0].addresses[0].ip}" 2>/dev/null || true)"; test -n "$$EP_IP" || exit 0; if [ "$$EP_IP" != "$$HOST_IP" ]; then /usr/bin/systemctl restart k3s.service; /usr/bin/systemctl start k3s-wsl-routes.service; fi'
EOF

sudo tee /etc/systemd/system/k3s-wsl-reconcile.timer >/dev/null <<'EOF'
[Unit]
Description=Periodically reconcile WSL network changes for K3s

[Timer]
OnBootSec=2min
OnUnitActiveSec=2min
AccuracySec=30s
Unit=k3s-wsl-reconcile.service
Persistent=false

[Install]
WantedBy=timers.target
EOF
```

说明：

1. `k3s-wsl-reconcile.timer` 不是无条件重启 K3s 的定时任务。
2. `k3s-wsl-reconcile.service` 每次运行时会读取当前 `eth2` IPv4，并读取 Kubernetes Service `kubernetes` 的 endpoint IP。
3. 两者一致时，仅执行 `k3s-wsl-routes.service` 以恢复可能丢失的路由。
4. 两者不一致时，说明当前 WSL IP 已变化而 K3s endpoint 仍是旧地址；此时重启 `k3s.service`，由前置的 `k3s-wsl-config.service` 重新生成动态配置。
5. 如果 apiserver 尚未就绪、endpoint 暂时读取不到，该服务直接退出，等待下一次 timer 触发，避免启动阶段误判导致连续重启。
6. 重启 K3s 会造成短暂 Kubernetes API 中断；该中断只在 IP 变化时发生，符合当前 WSL 单节点实验环境的恢复边界。

##### 6. 启用并执行服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable k3s-wsl-config.service
sudo systemctl enable k3s-wsl-routes.service
sudo systemctl enable --now k3s-wsl-reconcile.timer
sudo systemctl restart k3s
sudo systemctl restart k3s-wsl-routes.service
sudo systemctl start k3s-wsl-reconcile.service
```

执行中问题与修复：

1. 首次尝试使用包含复杂 `awk` 与 `printf "%s\n"` 的 unit 命令时，systemd 预处理导致 `awk` 中的 `/` 转义失效，日志出现 `awk: unterminated regexp`，`k3s-wsl-config.service` 启动失败。
2. 通过管道向 `sudo -S tee` 写入 unit 文件时，因 sudo 凭据缓存状态变化，曾将一行密码标记误写入 unit 文件首行。已用仅匹配该异常首行的 `sed -i` 命令移除，未删除 unit 文件。
3. 最终修复方式为改用 `tr`、`cut`、`head` 解析 IPv4，并使用 `echo` 写配置，避免 systemd `%` 转义与 shell/awk 多层转义冲突。
4. 修复后执行 `systemd-analyze verify` 通过，`systemctl daemon-reload` 后重新启动 K3s 成功。

##### 7. 验证动态持久化结果

```bash
systemctl status k3s-wsl-config.service --no-pager
systemctl status k3s-wsl-routes.service --no-pager
systemctl status k3s-wsl-reconcile.timer --no-pager
systemctl status k3s-wsl-reconcile.service --no-pager
systemctl list-timers --all | grep k3s-wsl || true
cat /etc/rancher/k3s/config.yaml
ip -brief addr show eth2
ip route get 10.43.0.1
CNI_IP="$(ip -4 -o addr show dev cni0 scope global | awk '{split($4,a,"/"); print a[1]; exit}')"
ip route get "$(ip -4 -o addr show dev eth2 scope global | awk '{split($4,a,"/"); print a[1]; exit}')" from "$CNI_IP"
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide
kubectl get apiservice v1beta1.metrics.k8s.io -o jsonpath='{.status.conditions[*].type} {.status.conditions[*].status} {.status.conditions[*].reason} {.status.conditions[*].message}{"\n"}'
kubectl top nodes
```

验收反馈：

1. `k3s.service` 为 `active (running)`，重启后 `/readyz` 可用。
2. `k3s-wsl-config.service` 执行成功，`/etc/rancher/k3s/config.yaml` 写入 `node-ip: 192.168.3.40`、`advertise-address: 192.168.3.40`、`flannel-iface: eth2`。
3. `k3s-wsl-routes.service` 执行成功，`ip route get 10.43.0.1` 输出 `dev cni0 src 10.42.0.1`。
4. `ip route get 192.168.3.40 from 10.42.0.1` 输出 `local 192.168.3.40 ... dev lo table 128`。
5. `k3s-wsl-reconcile.timer` 为 `active (waiting)`，并已按周期触发 `k3s-wsl-reconcile.service`。
6. `kubectl get nodes -o wide` 中节点 `aroen` 为 `Ready`，Internal IP 为 `192.168.3.40`。
7. `kubernetes` endpoint 为 `192.168.3.40:6443`。
8. `coredns`、`local-path-provisioner`、`metrics-server` 均为 `1/1 Running`。
9. `v1beta1.metrics.k8s.io` 为 `Available=True`，message 为 `all checks passed`。
10. `kubectl top nodes` 可返回节点指标。

##### 8. 重启后复核

WSL 重启后执行：

```bash
systemctl status k3s --no-pager
systemctl status k3s-wsl-config.service --no-pager
systemctl status k3s-wsl-routes.service --no-pager
systemctl status k3s-wsl-reconcile.timer --no-pager
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide
kubectl top nodes
```

通过标准：

1. K3s 正常启动。
2. K3s 节点 IP、Kubernetes Service CIDR 路由、table `128` 本地路由均随当前 WSL IP 动态更新。
3. metrics-server 不再因 `10.43.0.1:443` 超时而 CrashLoop。
4. `k3s-wsl-reconcile.timer` 会继续按周期进行网络恢复核查。

##### 9. 启动与网络恢复后的自动化边界

结论：

1. WSL 完整重启后，`k3s-wsl-config.service` 会在 K3s 启动前重新读取当前 `eth2` IPv4，并生成新的 K3s 配置。
2. K3s 启动后，`k3s-wsl-routes.service` 会重新读取当前 `cni0` 与 `eth2` IPv4，并恢复 Service CIDR 与 table `128` 路由。
3. 同一 WSL 会话内网络恢复但 IP 未变化时，`k3s-wsl-reconcile.timer` 会触发路由恢复，不需要重启 K3s。
4. 同一 WSL 会话内 `eth2` IPv4 已变化时，`k3s-wsl-reconcile.timer` 会发现当前 endpoint IP 与当前 `eth2` IP 不一致，然后重启 K3s，使 `kubernetes` endpoint 更新为新地址。
5. 该自动化方案以“条件重启”为边界，避免持续无条件重启 K3s；后续部署真实应用后，仍应把这类重启视为一次短暂控制面恢复事件。

如需立即手动触发一次自动核查，可执行：

```bash
sudo systemctl start k3s-wsl-reconcile.service
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide
kubectl top nodes
```

说明：

1. 手动触发与定时触发使用同一个 `k3s-wsl-reconcile.service`。
2. 若 endpoint IP 与当前 `eth2` IP 一致，该服务只恢复路由。
3. 若 endpoint IP 与当前 `eth2` IP 不一致，该服务会重启 K3s，并在重启后恢复路由。

#### B4. 配置当前用户 kubectl

执行：

```bash
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$USER:$USER" ~/.kube/config
chmod 600 ~/.kube/config
kubectl version --client
kubectl config current-context
kubectl get nodes -o wide
kubectl get pods -A
kubectl get storageclass
```

通过标准：

1. `kubectl version --client` 可输出客户端版本。
2. `kubectl get nodes -o wide` 不需要 sudo 即可返回节点。
3. 节点状态为 `Ready`。

#### B5. 安装 Helm

当前 Helm APT 源不可用，建议先禁用该源，避免后续 `apt-get update` 持续失败：

```bash
sudo sed -i.bak 's|^deb |# deb |' /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
```

然后改用 Helm 官方安装脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

复核：

```bash
helm version
helm list -A
```

通过标准：

1. `helm version` 可输出版本。
2. `helm list -A` 能访问当前 K3s 集群，允许为空列表。

执行反馈：

1. Helm 已安装，`helm version --short` 输出 `v3.21.2+g1259634`。
2. `helm list -A` 可访问当前 K3s 集群，当前无已安装 Helm release。

#### B6. 安装后统一验收

执行：

```bash
k3s --version
kubectl version --client
kubectl config current-context
kubectl get nodes -o wide
kubectl get pods -A
kubectl get storageclass
helm version
helm list -A
```

反馈模板：

```text
K3s 与 Helm 安装反馈：
1. k3s --version：
2. kubectl version --client：
3. kubectl current-context：
4. kubectl get nodes -o wide：
5. kube-system Pod 状态摘要：
6. StorageClass：
7. helm version：
8. helm list -A：
9. 是否出现错误：
```

通过标准：

1. K3s 已安装。
2. `kubectl` 可由当前用户直接使用。
3. WSL 节点状态为 `Ready`。
4. Helm 可访问集群。
5. 以上完成后，可回到第一部分 TODO 勾选 K3s 工具链相关项，并进入第二部分代码职责核查。

统一验收反馈：

1. `k3s --version` 为 `v1.36.2+k3s1`。
2. `kubectl version --client` 为 `v1.36.2+k3s1`，当前 context 为 `default`。
3. `kubectl get nodes -o wide` 中节点 `aroen` 为 `Ready`，Internal IP 为 `192.168.3.40`。
4. `kubectl get pods -n kube-system -o wide` 中 `coredns`、`local-path-provisioner`、`metrics-server` 均为 `1/1 Running`。
5. StorageClass `local-path` 已存在。
6. `helm version --short` 为 `v3.21.2+g1259634`。
7. `helm list -A` 可访问集群且当前为空列表。
8. `kubectl top nodes` 可返回节点指标。

#### B7. 可选事项：Docker 普通用户权限

当前现象：

1. Docker daemon 可通过 sudo 访问。
2. 当前普通用户直接访问 Docker socket 时权限不足。
3. `sudo usermod -aG docker "$USER"` 已执行，`docker` 组已经包含 `aroen`，但当前 shell 的有效组尚未刷新。

可任选一种方式刷新会话：

```bash
newgrp docker
docker ps
```

或从 Windows 侧重启 WSL 后重新进入：

```powershell
wsl --shutdown
```

重开 WSL 后复核：

```bash
id
docker ps
```

通过标准：

1. `id` 输出包含 `docker` 组。
2. 当前用户执行 `docker ps` 不再提示 `/var/run/docker.sock` 权限不足。

执行反馈：

1. `getent group docker` 显示 `docker:x:989:aroen`，说明用户已加入 docker 组数据库。
2. 当前 Codex shell 的 `id` 输出尚未包含 `docker`，因此直接执行 `docker ps` 仍提示 Docker socket 权限不足。
3. 使用 `sg docker -c "docker ps"` 可列出当前 Docker 容器，说明以 docker 组启动的新会话可访问 Docker daemon。
4. 当前运行态容器摘要：`messagefeed-postgres` 为 `Up` 且 `healthy`，`messagefeed-web` 为 `Up`，`messagefeed-api` 处于 `Restarting (1)`；该业务容器状态记录不属于 K3s 工具链故障。

### A8. 核实 kubectl、K3s 与 kubeconfig 状态

核查命令：

```bash
command -v k3s || true
command -v kubectl || true
ls -la ~/.kube 2>/dev/null || true
ls -la /etc/rancher 2>/dev/null || true
ls -la /var/lib/rancher 2>/dev/null || true
ss -ltn | awk '$4 ~ /:(6443|10250|8472|51820|51821)$/ { print $0 }'
```

反馈：

1. `k3s` 已安装，版本为 `v1.36.2+k3s1`。
2. `kubectl` 已由 K3s 提供，客户端版本为 `v1.36.2+k3s1`。
3. 当前 kube context 为 `default`。
4. `/etc/rancher/k3s/config.yaml` 已由 `k3s-wsl-config.service` 动态生成，当前 `node-ip` 与 `advertise-address` 为 `192.168.3.40`。
5. `k3s.service` 已启用并处于 `active`。
6. 当前监听端口包括 Kubernetes API `6443` 与 kubelet `10250`。
7. `kubectl get nodes -o wide` 中节点 `aroen` 为 `Ready`，Internal IP 为 `192.168.3.40`。
8. `kubectl get pods -n kube-system -o wide` 中 `coredns`、`local-path-provisioner`、`metrics-server` 均为 `1/1 Running`。
9. `kubectl top nodes` 可返回节点指标。

判定：

1. K3s single-server 已安装并验收通过。
2. K3s 动态 IP 与路由持久化已完成。
3. 当前阶段不需要另行安装上游 `kubectl`。

### A9. 核实 Helm 状态

核查命令：

```bash
command -v helm || true
helm version
```

反馈：

1. Helm 已安装，`helm version --short` 输出 `v3.21.2+g1259634`。
2. `helm list -A` 可访问当前 K3s 集群，当前为空列表。

判定：

1. Helm 已安装并验收通过。
2. 当前尚无 Helm release。

### A10. 记录当前运行方式、端口与配置来源

核查命令：

```bash
find . -maxdepth 2 -type f \( -name '.env' -o -name '.env.*' -o -name 'docker-compose.yml' -o -name 'Caddyfile' \) -print
find deploy -maxdepth 3 -type f 2>/dev/null | sort
find ops -maxdepth 3 -type f 2>/dev/null | sort
```

反馈：

1. 配置文件路径：`.env`、`.env.example`、`docker-compose.yml`。
2. Caddy 配置路径：`deploy/caddy/Caddyfile.dev`、`deploy/caddy/Caddyfile.prod`、`deploy/caddy/Caddyfile.web`。
3. 运维脚本路径：`deploy/bin/messageFeed-start`、`deploy/bin/messageFeed-status`、`deploy/bin/messageFeed-make`。
4. 可观测性配置路径：`ops/observability/prometheus/prometheus.yml`、`ops/observability/loki/loki.yml`、`ops/observability/tempo/tempo.yml`、`ops/observability/otel-collector/otel-collector.yml`、`ops/observability/promtail/promtail.yml`。
5. 当前 Kubernetes 运行方式：K3s single-server 仅运行控制面与 `kube-system` 组件；尚未创建项目级业务 namespace 或 Workload。
6. 当前 Docker Compose 运行方式：处于部分运行状态，`messagefeed-postgres` 为 `Up` 且 `healthy`，`messagefeed-web` 为 `Up`，`messagefeed-api` 为 `Restarting (1)`。
7. 当前实际监听端口：PostgreSQL `127.0.0.1:5432`，Kubernetes API `*:6443`，kubelet `*:10250`。
8. Docker Compose 配置中声明的主要端口：API `127.0.0.1:60001`、PostgreSQL `127.0.0.1:5432`、gateway HTTPS `127.0.0.1:${GATEWAY_HTTPS_PORT:-8443}`、Web dev `5173`、Web service `8080`、Prometheus `127.0.0.1:9090`、Grafana `127.0.0.1:3000`、Loki `127.0.0.1:3100`、Tempo `127.0.0.1:3200`、OTel `127.0.0.1:4317/4318`、Tempo OTLP 映射 `127.0.0.1:4319`。
9. Cloudflare Tunnel token 来源类型：`docker-compose.yml` 中 `cloudflared` 与 `cloudflared-dev` 通过 `${CLOUDFLARED_TUNNEL_TOKEN:-}` 注入；`deploy/bin/messageFeed-start` 与 `deploy/bin/messageFeed-make` 从环境变量读取 `CLOUDFLARED_TUNNEL_TOKEN`；当前 `.env` 中存在该变量；本记录不保存明文。
10. `/etc/messagefeed/messagefeed.env` 是 `deploy/bin/messageFeed-start` 与 `deploy/bin/messageFeed-status` 的可选运行时环境来源；本次核查结果为不存在或当前用户不可读。
11. 数据库数据目录：Docker volume `messagefeed-postgres-data` 挂载到容器内 `/var/lib/postgresql/data`，宿主机 Docker volume mountpoint 为 `/var/lib/docker/volumes/messagefeed-postgres-data/_data`。
12. `.env` 中本次识别的敏感或连接类变量名：`CLOUDFLARED_TUNNEL_TOKEN`、`EMBEDDING_API_KEY`、`LLM_API_KEY`、`WECHAT_WORK_CALLBACK_TOKEN`、`WECHAT_WORK_ENCODING_AES_KEY`、`WECHAT_WORK_SECRET` 均为 present；`WECHAT_WEBHOOK_URL` 为 empty。
13. `.env.example` 记录了 `AUTH_OWNER_PASSWORD`、`DATABASE_URL`、`LLM_API_KEY`、`EMBEDDING_API_KEY`、企业微信相关密钥、Webhook 等变量示例；本记录不保存示例之外的实际敏感值。

判定：

1. 当前运行资产与敏感配置来源已记录。
2. 已记录当前 Docker Compose 业务容器存在 `messagefeed-api` 重启状态；该状态不属于本次 K3s 工具链验收失败。
3. 未记录 token、API key、密码、私钥或回调密钥明文。

### 第一部分阶段结论

**反馈时间**：2026-07-06 20:14 CST  
**总体判定**：第一部分环境基线核查与工具链安装验收已完成。K3s single-server、K3s 动态网络持久化、Helm、kubectl、local-path StorageClass、metrics-server 均已通过当前核查。Docker 组数据库已包含 `aroen`，当前 Codex shell 仍需新组会话才可直接执行 `docker ps`，但 `sg docker` 已验证可访问 Docker。当前可由用户核实后更新 `docs/micr-k8s/micr-k8s-implement.md` 对应 TODO，并进入第二部分“核实现有代码职责与改造切入点”。

环境与项目基线：

1. 当前执行环境为 WSL2 Ubuntu 24.04.3 LTS。
2. 当前项目路径为 `/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed`，与实施文档一致。
3. Git 仓库根目录与项目路径一致；本次基线核查时短提交号为 `0703de0`，当时 `git status --short` 无输出。
4. `Dockerfile`、`docker-compose.yml`、`migrations`、`deploy/caddy`、`ops/observability` 等关键资产已确认存在。
5. `.env`、`.env.example`、`docker-compose.yml`、Caddy 配置、观测配置、Cloudflare Tunnel token 来源类型和主要端口已记录；未记录任何 Secret、token、密码或私钥明文。

安装与工具状态：

1. `go`、`git`、Docker client/server 已具备基础可用性。
2. Docker daemon 可通过 `sudo` 访问；`docker` 组数据库已包含 `aroen`；当前 Codex shell 有效组尚未刷新，`sg docker` 可访问 Docker。
3. K3s 已安装，版本为 `v1.36.2+k3s1`。
4. `kubectl` 已由 K3s 提供，客户端命令可用，当前 context 为 `default`。
5. K3s 节点 `aroen` 已达到 `Ready`。
6. 默认 StorageClass `local-path` 已存在。
7. Helm 已安装，版本为 `v3.21.2+g1259634`，`helm list -A` 可访问当前集群。

问题情况：

1. K3s 初次启动时自动选择 `198.18.0.1` 作为节点 Internal IP，不符合当前 WSL 业务网卡 `eth2` 的预期地址。
2. `coredns`、`local-path-provisioner`、`metrics-server` 曾分别出现未 Ready、Error 或 `CrashLoopBackOff`。
3. 核心网络故障表现为 Pod 访问 Kubernetes Service IP `10.43.0.1:443` 超时。
4. WSL 路由中 `10.0.0.0/7 via 198.18.0.2 dev eth0` 覆盖了 K3s 默认 Service CIDR `10.43.0.0/16`。
5. WSL policy routing table `128` 曾将当前 WSL 主机地址指向 `169.254.73.152`，导致 CNI/Pod 源地址访问 apiserver 的路径异常。
6. CoreDNS 日志中的 `/etc/coredns/custom/*.override` 与 `*.server` import glob warning 属于默认扩展目录为空时的提示；在 CoreDNS Ready 后不作为故障处理。
7. Helm APT 路径失败，表现包括 `Unable to locate package helm` 以及 `baltocdn.com` InRelease 校验异常 `Clearsigned file isn't valid, got 'NOSPLIT'`。
8. Docker 普通用户权限问题在系统组数据库层面已处理；当前 Codex shell 未刷新有效组，直接 `docker ps` 仍失败，`sg docker -c "docker ps"` 可访问 Docker。

修复情况：

1. 已通过运行时路由修复恢复 K3s 核心组件连通性。
2. 修复后 `coredns`、`local-path-provisioner`、`metrics-server` 均已达到 `Running` 且 Ready。
3. `v1beta1.metrics.k8s.io` 已显示 `Available=True`。
4. `kubectl top nodes` 已可返回节点指标。
5. K3s 动态 IP 与路由持久化方案已执行并验收通过，当前 `node-ip`、`advertise-address` 与 `kubernetes` endpoint 均为 `192.168.3.40`。
6. Helm 已安装并验收通过，当前无 Helm release。
7. Docker 组数据库已包含 `aroen`；当前 Codex shell 未刷新有效组，已用 `sg docker` 验证新组会话可访问 Docker。
8. 当前运行方式、端口、Cloudflare Tunnel token 来源、数据库数据目录和 `.env` 敏感配置来源已记录到 A10。

进入第二部分前的用户核实事项：

1. 用户核实 B3.2、B5、B6、B7 和 A10 记录是否与当前终端一致。
2. 用户在自己的新 shell 或 WSL 重启后执行 `docker ps`，确认普通用户 Docker 组会话已生效。
3. 用户更新 `docs/micr-k8s/micr-k8s-implement.md` 中对应 TODO。

### 第一部分暂不执行项

1. 不创建项目级 Kubernetes namespace、Secret、ConfigMap、Deployment、StatefulSet、Service、Job、CronJob 或其他业务 Workload。
2. 不修改业务代码、启动装配代码、配置读取逻辑或 Helm chart。
3. 不构建、推送或加载业务镜像。
4. 不部署 PostgreSQL、gateway、cloudflared、API、Web 或 worker。
5. 不进行 PostgreSQL 数据迁移、备份恢复、高可用切换或持久卷迁移。
6. 不引入 Nginx Ingress、Traefik 入口改造、多 server K3s 高可用、agent 节点或跨机器集群。
7. 不配置 CI/CD、镜像仓库、自动发布、滚动发布或回滚演练。
8. 不修改 `docs/micr-k8s/micr-k8s-implement.md` 的勾选状态；该清单由用户在核实完成后更新。
9. 不删除文件、系统服务、Kubernetes 资源或运行资产，除非后续获得明确指令。

## 第二部分：核实现有代码职责与改造切入点

**状态**：已执行并回填；等待用户核实后更新 `docs/micr-k8s/micr-k8s-implement.md` 第二部分 TODO。  
**建议**：第二部分先做只读代码核查，形成职责矩阵与改造切入点，再进入第三部分 `APP_ROLE` 启动角色化。不要直接开始改代码，否则容易把现有 HTTP、worker、迁移、观测和配置读取边界混在同一次变更中。  
**反馈时间**：2026-07-06 21:12 CST。  
**执行性质**：只读核查、职责梳理、改造边界确认。  
**记录方式**：本次由 Codex 执行只读核查并回填结果；确认通过后再由用户更新 `docs/micr-k8s/micr-k8s-implement.md` 中第二部分 TODO。  
**执行边界**：不修改业务代码，不新增 `APP_ROLE`，不改 Dockerfile、Compose、Helm 或 Kubernetes 资源，不读取或记录 `.env` 明文敏感值，不停止或重启任何服务。

### 目标

1. 核实 `cmd/api/main.go` 当前是否同时启动 HTTP API、source sync、notification、agent scheduled task、embedding worker。
2. 核实数据库连接池、健康检查、指标、日志、OpenTelemetry、企业微信、LLM、Embedding 的配置读取和装配路径。
3. 梳理当前 worker 的任务锁、job claim、幂等、重试和失败记录机制。
4. 明确第一轮不改业务模型、不拆仓库、不直接引入 gRPC/Eino/Nginx Ingress。
5. 确认第一轮重构目标是运行边界，而不是业务微服务边界。

### C1. 确认执行基线和工作区状态

待执行命令：

```bash
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed
pwd
git rev-parse --show-toplevel
git rev-parse --short HEAD
git status --short
go version
```

预期：

1. 当前路径仍为项目根目录。
2. Git 仓库根目录与项目根目录一致。
3. 记录当前短提交号，便于后续对比。
4. 若 `git status --short` 有输出，只记录文件路径和状态，不回退、不删除、不清理。

当前实施步骤反馈：

```text
C1 反馈：
1. 当前项目路径：/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed。
2. Git 仓库根目录：/home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed。
3. 当前短提交号：0703de0。
4. 工作区是否存在未提交变更：否，git status --short 无输出。
5. Go 版本：go version go1.26.1 linux/amd64。
6. 是否存在会影响第二部分只读核查的异常：未发现。
```

### C2. 核实 `cmd/api/main.go` 当前启动职责

待执行命令：

```bash
# 分段查看 main.go，避免一次输出过长
sed -n '1,220p' cmd/api/main.go
sed -n '220,560p' cmd/api/main.go

# 定位 HTTP、worker、后台 goroutine、数据库指标和关闭逻辑
rg -n 'ListenAndServe|Shutdown|go run|runSourceSyncWorker|runNotificationWorker|runAgentScheduledTaskWorker|runAgentEmbeddingWorker|collectDatabaseMetrics|signal.NotifyContext' cmd/api/main.go

# 定位 worker 循环、tick、claim、失败和重试记录
rg -n 'func run(SourceSync|Notification|AgentScheduledTask|AgentEmbedding)Worker|time.NewTicker|WorkerID|LockName|Claimed|Failed|Retry|RunOnce|RunDueOnce' cmd/api/main.go
```

需要核实的问题：

1. HTTP server 是否由当前 `cmd/api/main.go` 直接启动。
2. source sync worker 是否由当前进程后台 goroutine 启动。
3. notification worker 是否由当前进程后台 goroutine 启动。
4. agent scheduled task worker 是否由当前进程后台 goroutine 启动。
5. embedding worker 是否由当前进程后台 goroutine 启动。
6. 当前是否已经存在 `APP_ROLE` 或类似角色分流逻辑。
7. 当前优雅退出是否同时覆盖 HTTP server、worker context、数据库关闭和 tracing shutdown。

当前实施步骤反馈：

```text
C2 反馈：
1. HTTP API 是否由 cmd/api/main.go 启动：是，main.go 构造 http.Server 并在 goroutine 中执行 ListenAndServe。
2. source sync worker 是否同进程启动：是，在 DATABASE_URL 配置后通过 runSourceSyncWorker 后台 goroutine 启动。
3. notification worker 是否同进程启动：是，在 DATABASE_URL 配置后通过 runNotificationWorker 后台 goroutine 启动；实际 service 依赖企业微信 sender，未配置时函数直接返回。
4. agent scheduled task worker 是否同进程启动：是，在 DATABASE_URL 配置后通过 runAgentScheduledTaskWorker 后台 goroutine 启动。
5. embedding worker 是否同进程启动：是，在 DATABASE_URL 配置后通过 runAgentEmbeddingWorker 后台 goroutine 启动；实际 service 依赖 embedding client，未配置时函数直接返回。
6. 当前是否已有 APP_ROLE 分流：否。cmd、internal、Dockerfile、docker-compose.yml、deploy 中未检出 APP_ROLE 实现。
7. 优雅退出覆盖范围：收到 interrupt/SIGTERM 后会 cancel background context、关闭 HTTP server、关闭数据库连接池；OpenTelemetry shutdown 通过 defer 执行。worker loop 监听 background context；数据库指标采集 goroutine 当前未接收 context，只依赖进程退出结束。
8. 初步改造切入点：将 main.go 中配置加载、日志/tracing、数据库、业务 service、router、worker loop、HTTP server 生命周期拆成可复用装配函数，再由 APP_ROLE 决定是否启动 HTTP 与具体 worker。
```

### C3. 核实配置读取、数据库、健康检查、指标和 OpenTelemetry

待执行命令：

```bash
# 查看配置结构、环境变量读取和校验规则
sed -n '1,260p' internal/config/config.go
sed -n '260,540p' internal/config/config.go

# 查看数据库初始化封装
sed -n '1,220p' internal/db/db.go

# 查看路由、健康检查、ready 检查、运行节点信息和指标定义
sed -n '1,260p' internal/handler/router.go
sed -n '1,220p' internal/runtime/readiness.go
sed -n '1,220p' internal/runtime/node.go
sed -n '1,260p' internal/metrics/metrics.go

# 查看 OpenTelemetry 初始化
sed -n '1,180p' internal/observability/observability.go

# 汇总定位配置、健康检查、指标和观测相关关键字
rg -n 'healthz|readyz|metrics|runtime/node|DATABASE_URL|APP_NODE_ID|DEPLOYMENT_MODE|OTEL|Trace|InitTracing|LLM_|EMBEDDING_|WECHAT_WORK_' cmd internal
```

需要核实的问题：

1. `DATABASE_URL` 是否集中由 `internal/config/config.go` 读取。
2. 数据库连接池参数是否已经配置，是否适合后续 API/worker 分角色独立运行。
3. `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node` 是否已经存在并可作为 K8s 探针或观测入口。
4. 日志字段是否可以区分 `APP_NODE_ID` 或后续角色标识。
5. OpenTelemetry 是否由配置开关控制，是否可在不同角色中复用。
6. 企业微信、LLM、Embedding 配置是否集中读取，是否存在角色化后必须保留的初始化依赖。

当前实施步骤反馈：

```text
C3 反馈：
1. 配置读取集中位置：internal/config/config.go；当前从环境变量读取，不读取 YAML/TOML/JSON 配置文件。
2. 数据库连接池配置位置：internal/config/config.go 读取 DATABASE_URL、DATABASE_MAX_OPEN_CONNS、DATABASE_MAX_IDLE_CONNS、DATABASE_CONN_MAX_LIFETIME；internal/db/db.go 通过 sqlDB.SetMaxOpenConns、SetMaxIdleConns、SetConnMaxLifetime 应用。
3. 健康检查路径：/healthz、/readyz。
4. ready 检查依赖：process；若 database 非 nil，还检查数据库 ping、schema_migrations、pgvector、agent_fact_archive_index、agent_fact_embeddings、agent_trace_events、agent_recall_traces、agent_embedding_traces、agent_memory_topics、agent_memory_chunks、agent_fact_index_jobs。
5. metrics 暴露方式：/metrics 使用 prometheus promhttp.HandlerFor(metrics.Gatherer)；指标集中定义在 internal/metrics/metrics.go，并通过默认注册表注册。
6. OpenTelemetry 初始化位置：internal/observability/observability.go 的 InitTracing；TraceEnabled=false 时返回 no-op shutdown，启用时使用 OTLP gRPC exporter、service.name、service.version、service.instance.id 和 deployment.environment。
7. 企业微信配置读取位置：internal/config/config.go 读取 WECHAT_WORK_CORP_ID、WECHAT_WORK_AGENT_ID、WECHAT_WORK_SECRET、WECHAT_WORK_CALLBACK_TOKEN、WECHAT_WORK_ENCODING_AES_KEY；cmd/api/main.go 初始化 callback codec、sender 和 OAuth client。
8. LLM 配置读取位置：internal/config/config.go 读取 LLM_PROVIDER、LLM_API_KEY、LLM_BASE_URL、LLM_MODEL、LLM_CONFIG_SECRET；cmd/api/main.go 初始化 OpenAI-compatible client。
9. Embedding 配置读取位置：internal/config/config.go 读取 EMBEDDING_PROVIDER、EMBEDDING_API_KEY、EMBEDDING_BASE_URL、EMBEDDING_MODEL、EMBEDDING_DIMENSION；cmd/api/main.go 初始化 embedding client。
10. 对第三部分 APP_ROLE 改造的约束：api 角色仍需 HTTP、router、健康检查、metrics、企业微信 callback 和主链路 service；worker 角色应复用配置、日志、tracing、数据库与对应 service，但不启动 HTTP server。日志已有 node_id 和 deployment_mode，后续应补充 app_role。另需注意 AdminConfigService 当前在数据库打开前构造，传入 database=nil；第四部分拆装配时应核实构造顺序。
```

### C4. 梳理 worker 任务锁、claim、幂等、重试和失败记录机制

待执行命令：

```bash
# 汇总定位任务锁、claim、重试、失败和投递幂等相关代码
rg -n 'Claim|claim|locked_by|locked_at|lock|Lock|retry|Retry|failed|Failed|attempt|Attempt|idempot|delivery|Delivery|status' internal/domain internal/repository internal/service migrations

# 查看任务锁和 source fetch job 仓储
sed -n '1,240p' internal/repository/task_lock_repository.go
sed -n '1,320p' internal/repository/source_fetch_job_repository.go

# 查看 notification、agent schedule、embedding 相关仓储和服务
sed -n '1,320p' internal/repository/notification_repository.go
sed -n '1,320p' internal/repository/agent_schedule_eval_repository.go
sed -n '1,260p' internal/service/source_sync_service.go
sed -n '1,260p' internal/service/notification_worker_service.go
sed -n '1,260p' internal/service/agent_schedule_eval_service.go
sed -n '1,260p' internal/service/agent_embedding_worker_service.go

# 查看涉及任务锁、job、notification、embedding 的迁移
ls migrations/*task* migrations/*job* migrations/*notification* migrations/*embedding* 2>/dev/null || true
```

需要核实的问题：

1. source sync 是否使用任务锁或 job claim 避免并发重复抓取。
2. notification 是否有 delivery 记录、状态流转、失败重试和幂等约束。
3. agent scheduled task 是否有 claim、执行状态、失败记录和最终报告失败记录。
4. embedding worker 是否有待处理 job、claim 或可重复执行保护。
5. 多副本 worker 的安全前提是否已经满足，或者第三部分前需要标注风险。

当前实施步骤反馈：

```text
C4 反馈：
1. source-worker 锁或 claim 机制：SourceSyncService.RunOnce 可通过 TaskLockRepository.WithLock 使用 task_locks 全局 TTL 锁；source_fetch_jobs 使用 FOR UPDATE SKIP LOCKED 认领 due job，认领时设置 running、locked_by、locked_at 并递增 attempt_count。CreateJob 会避免同一 user/source 已有 queued/running job 时重复创建。
2. source-worker 重试与幂等：source_fetch_attempts 记录每次尝试，迁移中约束 uq_source_fetch_attempts_job_number 保证 job 内 attempt 编号唯一；失败后 attempt_count < max_attempts 时重排为 queued/retry，并按 attempt 分钟延迟；item_events 使用 dedupe_key 且迁移中有唯一约束。
3. notification-worker 幂等与失败记录机制：notification_jobs 有 dedupe_key 唯一约束；ClaimDueJobs 使用 FOR UPDATE SKIP LOCKED 认领 queued job 并递增 attempt_count；每次发送记录 notification_deliveries；发送失败时按 attempt_count^2 分钟延迟重排，达到 MaxAttempts 后标记 failed。
4. agent-scheduler-worker claim 与失败记录机制：agent_scheduled_tasks 使用 FOR UPDATE SKIP LOCKED 认领 due queued task，认领时设置 running、locked_by、locked_at 并递增 attempt_count；执行中创建 AgentRun、scheduled_task_controller_input trace、audit log，成功时 CompleteScheduledTask，失败时 FailScheduledTask 或记录报告失败。
5. embedding-worker claim 或幂等机制：agent_fact_index_jobs 的 pending embed job 使用 UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) 认领为 running；完成后记录 succeeded/failed、processed_count、failed_count、error_message 和 AgentTraceEvent。agent_fact_embeddings 对 canonical_ref、embedding_model、content_hash 有唯一约束，agent_fact_archive_index 对 canonical_ref 有唯一约束。
6. 当前多副本 worker 风险：source-worker 当前有全局 task_locks 串行化保护，安全但会限制横向扩展吞吐；agent-scheduler 的 defer admission 路径写入 NextRunAt，但 claim 条件核查 scheduled_at，后续多副本前应确认是否会造成频繁重新 claim；embedding failed job 当前不会自动重排，需确认是否接受人工/后续流程重试。
7. 第三部分前必须保留或补充的约束：保留数据库级 SKIP LOCKED claim、dedupe 唯一约束、attempt_count、locked_by、locked_at、LastError 和 delivery/trace/audit 记录；APP_ROLE 拆分后不能同时由 api 角色隐式启动 worker。
```

### C5. 确认第一轮改造边界

待执行命令：

```bash
# 核实当前是否已有 APP_ROLE 或部署模式相关实现
rg -n 'APP_ROLE|DEPLOYMENT_MODE|role|worker|api' cmd internal Dockerfile docker-compose.yml deploy docs/micr-k8s

# 核实是否已经存在 gRPC、Eino、Ingress 或业务微服务拆分入口
rg -n 'grpc|gRPC|eino|Eino|Ingress|nginx|microservice|notification-service|feed-worker-service' cmd internal deploy docs/micr-k8s Dockerfile docker-compose.yml

# 查看当前部署资产，不创建或修改任何文件
find deploy -maxdepth 3 -type f | sort
find ops -maxdepth 3 -type f | sort
```

需要核实的问题：

1. 第一轮是否仍应保持单仓库、单后端镜像、多 `APP_ROLE` 运行。
2. 是否确认不直接引入 gRPC/Eino/Nginx Ingress。
3. 是否确认第三部分只做启动角色化，不改业务模型。
4. 是否确认 API 与 worker 的边界是运行边界，不是业务微服务边界。

当前实施步骤反馈：

```text
C5 反馈：
1. 当前是否已有 APP_ROLE：否。APP_ROLE 仅出现在 docs/micr-k8s 规划文档中，未在 cmd、internal、Dockerfile、docker-compose.yml、deploy 中实现。
2. 当前是否已有 gRPC/Eino/Nginx Ingress 主线：未发现业务 gRPC/Eino/Nginx Ingress 主线；仅 OpenTelemetry 使用 OTLP gRPC exporter，文档中明确暂不引入 Nginx Ingress。
3. 是否确认第一轮不拆业务微服务：确认。当前后端仍是 ./cmd/api 单入口、Dockerfile 构建单个 messagefeed 二进制，第三部分应保持单仓库、单后端镜像、多运行角色。
4. 是否确认第三部分优先做运行边界：确认。当前问题是 api 与 worker 同进程启动，API 多副本会连带后台任务一起扩容；第三部分应优先用 APP_ROLE 切开运行边界。
5. 第三部分建议改造范围：新增 APP_ROLE 配置枚举与校验；按 api、source-worker、notification-worker、agent-scheduler-worker、embedding-worker、migrate、all 分流启动；DEPLOYMENT_MODE=cluster 下禁止隐式 all；保留 all 作为本地兼容；增加命令级或最小测试验证 api 不启动 worker、worker 不监听 HTTP。
```

### C6. 可选：只读测试基线

说明：本步骤用于确认现有代码在进入 `APP_ROLE` 改造前的测试基线。若当前环境缺少测试依赖、外部服务或执行时间过长，可先跳过，不影响第二部分职责梳理。

待执行命令：

```bash
# 核心启动与配置相关测试
go test ./cmd/api ./internal/config ./internal/runtime ./internal/handler

# worker、repository、service 相关测试；若耗时较长可分包执行
go test ./internal/repository ./internal/service
```

当前实施步骤反馈：

```text
C6 反馈：
1. 是否执行测试：是。
2. 通过的包：messagefeed/cmd/api、messagefeed/internal/config、messagefeed/internal/runtime、messagefeed/internal/handler、messagefeed/internal/repository、messagefeed/internal/service。
3. 失败的包和失败摘要：无失败；本次输出均为 ok，且结果来自缓存。
4. 是否阻塞进入第三部分：不阻塞。
```

### 第二部分通过标准

1. 已确认 `cmd/api/main.go` 当前实际启动的 HTTP 与 worker 职责。
2. 已确认配置读取、数据库、健康检查、指标、日志、OpenTelemetry、企业微信、LLM、Embedding 的装配路径。
3. 已形成 source、notification、agent scheduler、embedding worker 的任务锁、claim、幂等、重试和失败记录摘要。
4. 已确认第一轮不改业务模型、不拆仓库、不直接引入 gRPC/Eino/Nginx Ingress。
5. 已确认第三部分的直接目标是 `APP_ROLE` 运行角色化，而不是业务微服务拆分。
6. 已记录第三部分需要优先保护的回归点，例如 API 不启动 worker、worker 不监听 HTTP、`APP_ROLE=all` 仅作为本地兼容或过渡。

### 第二部分阶段结论

**反馈时间**：2026-07-06 21:12 CST  
**总体判定**：第二部分只读核查已完成。当前可以进入第三部分“完成 APP_ROLE 启动角色化”的实施准备；建议用户先核实本章节回填内容，再更新 `docs/micr-k8s/micr-k8s-implement.md` 第二部分 TODO。

结论：

1. `cmd/api/main.go` 当前是单入口装配层，负责配置加载、日志、OpenTelemetry、企业微信、LLM、Embedding、数据库、Repository、Service、Router、HTTP server、数据库指标采集和四类 worker loop。
2. 当前 API 与 worker 未分离；只要 DATABASE_URL 配置存在，source sync、notification、agent scheduled task、embedding worker 都会随 API 进程启动。
3. 当前未实现 `APP_ROLE`，也未实现角色化启动校验。
4. 已存在 `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node`，可作为后续 K8s 探针和观测入口基础。
5. 已存在数据库级 claim、锁、dedupe、attempt、delivery、trace、audit 机制，具备拆分运行角色的基础。
6. 第三部分应优先以最小范围完成 `APP_ROLE` 运行边界，不应同时进行业务模型拆分、仓库拆分、Helm chart 编写或 Kubernetes 部署。
7. 第三部分重点回归项为：`APP_ROLE=api` 不启动任何 worker；worker 角色不启动 HTTP server；`APP_ROLE=all` 仅用于本地兼容；`DEPLOYMENT_MODE=cluster` 下禁止隐式使用 `all`。

### 第二部分暂不执行项

1. 不修改 `cmd/api/main.go` 或任何 Go 源码。
2. 不新增 `APP_ROLE`、`internal/bootstrap`、Helm chart 或 Kubernetes manifest。
3. 不构建镜像，不启动或重启 Docker Compose 服务，不部署 Kubernetes Workload。
4. 不读取、输出或记录 `.env` 中的敏感值明文。
5. 不修改 `docs/micr-k8s/micr-k8s-implement.md` 的勾选状态；该清单仍由用户核实后更新。
6. 不删除文件、容器、镜像、volume、Kubernetes 资源或 systemd 服务。

## 第三部分前置过渡部署：当前生产链路同步启动

**状态**：数据库备份已执行；过渡部署方案已按用户要求修订为完整链路同步部署；等待用户执行并反馈部署步骤。  
**反馈时间**：2026-07-06 21:52 CST。  
**执行性质**：过渡型 Kubernetes 启动验证；先保证当前后端整体服务、Web、gateway、Cloudflare Tunnel 与观测组件均可由 K3s 管理。  
**记录方式**：本章节先写入将要执行的步骤和具体命令，由用户逐项执行并反馈结果；Codex 后续只根据反馈回填状态与问题处理。  
**实施定位**：临时跳过 `APP_ROLE`、Helm Chart 和 worker 拆分，直接把当前 Go 后端按现状作为单副本 all-in-one Deployment 启动，同时同步部署当前 Compose 生产链路中的 Web、Caddy gateway、cloudflared、Prometheus、Loki、Promtail、Tempo、OpenTelemetry Collector 和 Grafana。  
**关键限制**：当前代码未实现 `APP_ROLE`，后端 Pod 会同时承担 HTTP API、source sync、notification、agent scheduled task 和 embedding worker；因此 `replicas` 必须固定为 `1`，且不得同时启动独立 worker Pod。

### D-1. 数据库备份执行记录

执行结果：

```text
当前有效备份时间：2026-07-07 20:15:56 CST
备份源：Docker Compose 容器 messagefeed-postgres
备份源镜像：pgvector/pgvector:pg15
备份源数据卷：messagefeed-postgres-data
数据库：messagefeed
备份格式：pg_dump custom，参数为 -Fc --no-owner --no-privileges
备份文件：/mnt/disk_A/Notes/gogogo/Go_Pro/messageFeed/micr-k8s/backups/postgres/messagefeed-postgres-docker-20260707-201556.dump
元数据文件：/mnt/disk_A/Notes/gogogo/Go_Pro/messageFeed/micr-k8s/backups/postgres/messagefeed-postgres-docker-20260707-201556.meta.txt
SHA256 文件：/mnt/disk_A/Notes/gogogo/Go_Pro/messageFeed/micr-k8s/backups/postgres/messagefeed-postgres-docker-20260707-201556.dump.sha256
备份大小：6.4M
schema_migrations：37,false
public base table count：55
items(source_id, normalized_url) 重复组：0
唯一索引状态：uq_items_source_normalized_url 的 indisunique、indisvalid、indisready 均为 true
校验方式：sha256sum -c 通过；使用 PostgreSQL 容器内 pg_restore -l 解析 custom dump 目录，结果可读。
旧备份状态：2026-07-06 21:46:26 CST 的 messagefeed-postgres-docker-20260706-214625.dump 已因重复 items 数据导致恢复失败，不再作为 D6 恢复输入。
```

本次备份不包含明文数据库密码、Cloudflare token、LLM key、Embedding key 或企业微信 secret。后续 K8s PostgreSQL 默认应从 2026-07-07 新备份恢复，再运行 migrate Job 做版本校验。

### D0. 方案边界与执行前确认

本章节默认采用以下过渡形态：

```text
K3s namespace: messagefeed
PostgreSQL: pgvector/pgvector:pg15 StatefulSet，使用 local-path PVC
数据库初始化方式: 先恢复 D-1 备份，再运行 migrate/migrate:v4.19.1 Job 做 up 校验
后端: messagefeed-api:allinone-<git-sha> Deployment，replicas=1，strategy=Recreate
Web: messagefeed-web:allinone-<git-sha> Deployment，Service web:8080
Caddy gateway: caddy:2.10.2-alpine Deployment，Service gateway/gateway-dev:8443
Cloudflare Tunnel: cloudflare/cloudflared:latest Deployment，使用现有 tunnel token
Observability: Prometheus、Loki、Promtail、Tempo、OpenTelemetry Collector、Grafana
验收入口: kubectl port-forward svc/gateway 8443:8443；必要时直接 port-forward svc/api 60001:60001
```

执行前必须确认：

1. 本步骤会创建新的 K8s PostgreSQL PVC，不会自动复用 Docker Compose 的 `messagefeed-postgres-data` 数据卷。
2. 现有 Docker Compose 数据已经通过 D-1 备份；K8s PostgreSQL 默认从该备份恢复。
3. 本步骤同步部署 Web 静态服务、Caddy gateway、Cloudflare Tunnel 和可观测性组件。
4. 本步骤只用于过渡验证，不替代后续 `APP_ROLE`、Helm 和多角色 Deployment 改造。
5. 后端 Deployment 使用 `Recreate`，避免更新时短暂出现两个 all-in-one Pod 导致后台任务重复运行。
6. Web、gateway、cloudflared 和观测组件可以独立重启或扩容，但后端 all-in-one 在 `APP_ROLE` 完成前不得扩容。

### D1. 核查当前 K3s 与项目基线

待执行命令：

```bash
# 进入项目根目录
cd /home/aroen/projects/Amoney/_Astu/go/go_st/Go_Pro/messageFeed

# 确认当前路径和提交号
pwd
git rev-parse --short HEAD
git status --short

# 确认 kubectl 指向当前 K3s
kubectl config current-context
kubectl get nodes -o wide
kubectl get storageclass

# 确认 K3s 系统 Pod 已处于可接受状态
kubectl get pods -A -o wide

# 确认 Docker 可用于构建本地镜像
docker ps
docker version

# 若当前 shell 的 Docker 组权限尚未刷新，可临时改用 sudo 执行只读核查
sudo docker ps
sudo docker version
```

预期：

1. 当前路径为项目根目录。
2. `kubectl get nodes` 中 WSL 节点为 `Ready`。
3. 默认 StorageClass 存在，当前预期为 `local-path`。
4. Docker daemon 可用。
5. 若普通用户执行 `docker ps` 仍提示 permission denied，可在本次过渡部署中临时使用 `sudo docker ...`，后续再通过新 shell 验证 Docker 组权限。
6. 若 `git status --short` 有输出，只记录状态，不回退、不清理。

当前实施步骤反馈：

```text
D1 反馈：
1. 项目路径：
2. Git 短提交号：
3. 工作区状态：
4. kubectl context：
5. K3s node 状态：
6. StorageClass：
7. Docker 可用性：
8. 是否可以进入 D2：
```

### D2. 设置本次过渡部署变量并创建 namespace

待执行命令：

```bash
# 设置本次部署使用的 namespace、镜像 tag 和数据库备份文件
export NS=messagefeed
export GIT_SHA="$(git rev-parse --short HEAD)"
export API_IMAGE="messagefeed-api:allinone-${GIT_SHA}"
export WEB_IMAGE="messagefeed-web:allinone-${GIT_SHA}"
export IMAGE="${API_IMAGE}"
export DB_BACKUP_FILE="/mnt/disk_A/Notes/gogogo/Go_Pro/messageFeed/micr-k8s/backups/postgres/messagefeed-postgres-docker-20260707-201556.dump"

# 创建或更新 namespace；该命令不会删除已有资源
kubectl create namespace "${NS}" --dry-run=client -o yaml | kubectl apply -f -

# 查看 namespace 是否存在
kubectl get namespace "${NS}"

# 查看备份文件是否存在
ls -lh "${DB_BACKUP_FILE}"
```

预期：

1. `NS=messagefeed`。
2. `API_IMAGE=messagefeed-api:allinone-<git-sha>`。
3. `WEB_IMAGE=messagefeed-web:allinone-<git-sha>`。
4. `DB_BACKUP_FILE` 指向 D-1 已生成的 custom dump。
5. namespace 创建成功或显示 unchanged/configured。

当前实施步骤反馈：

```text
D2 反馈：
1. NS：
2. API_IMAGE：
3. WEB_IMAGE：
4. DB_BACKUP_FILE：
5. namespace 状态：
6. 是否可以进入 D3：
```

### D3. 构建后端与 Web 镜像并导入 K3s containerd

说明：Docker 构建出的镜像默认只存在于 Docker daemon 中；K3s 默认使用自己的 containerd，必须把后端与 Web 镜像都导入 K3s containerd 后 Pod 才能使用。

待执行命令：

```bash
# 确认变量仍存在；若是新终端，请重新执行 D2 的 export
echo "${NS}"
echo "${API_IMAGE}"
echo "${WEB_IMAGE}"

# 构建当前后端 api 阶段镜像
docker build --target api -t "${API_IMAGE}" .

# 构建当前 Web 静态服务镜像
docker build --target web -t "${WEB_IMAGE}" .

# 查看 Docker 中的镜像
docker image ls "${API_IMAGE}"
docker image ls "${WEB_IMAGE}"

# 将 Docker 镜像导入 K3s containerd
docker save "${API_IMAGE}" | sudo k3s ctr images import -
docker save "${WEB_IMAGE}" | sudo k3s ctr images import -

# 查看 K3s containerd 中是否已有这两个镜像
sudo k3s ctr images ls | rg "messagefeed-api.*allinone-${GIT_SHA}"
sudo k3s ctr images ls | rg "messagefeed-web.*allinone-${GIT_SHA}"
或
sudo k3s ctr images ls | grep "messagefeed-api.*allinone-${GIT_SHA}"
sudo k3s ctr images ls | grep "messagefeed-web.*allinone-${GIT_SHA}"
```

若当前 shell 仍无法直接访问 Docker socket，则将上方 `docker build`、`docker image ls` 和 `docker save` 临时改为 `sudo docker ...`。

若 `sudo k3s ctr images import -` 不支持标准输入，改用以下备用命令：

```bash
# 备用方式：先把镜像保存到 /tmp，再导入 K3s containerd
docker save "${API_IMAGE}" -o "/tmp/messagefeed-api-allinone-${GIT_SHA}.tar"
docker save "${WEB_IMAGE}" -o "/tmp/messagefeed-web-allinone-${GIT_SHA}.tar"
sudo k3s ctr images import "/tmp/messagefeed-api-allinone-${GIT_SHA}.tar"
sudo k3s ctr images import "/tmp/messagefeed-web-allinone-${GIT_SHA}.tar"
sudo k3s ctr images ls | rg "messagefeed-api.*allinone-${GIT_SHA}"
sudo k3s ctr images ls | rg "messagefeed-web.*allinone-${GIT_SHA}"
```

预期：

1. 后端 Docker 构建成功。
2. Web Docker 构建成功。
3. K3s containerd 可检索到 `messagefeed-api:allinone-<git-sha>`。
4. K3s containerd 可检索到 `messagefeed-web:allinone-<git-sha>`。
5. 不推送远程镜像仓库，不使用 `latest` 作为本次部署 tag。

当前实施步骤反馈：

```text
D3 反馈：
1. 后端 Docker build 是否成功：
2. Web Docker build 是否成功：
3. 后端镜像 tag：
4. Web 镜像 tag：
5. K3s containerd 是否已导入：
6. 是否使用备用导入方式：
7. 是否可以进入 D4：
```

### D4. 创建应用 ConfigMap 与 Secret

说明：

1. 不把敏感值写入本文档。
2. 命令会从当前 shell 环境和可选 `.env` 中读取变量。
3. `POSTGRES_PASSWORD` 本过渡命令要求使用 URL-safe 字符，建议仅包含字母、数字、点、下划线和短横线，避免 `DATABASE_URL` 需要额外 URL 编码。
4. 企业微信、LLM、Embedding 配置必须成组提供；若暂不启用，对应字段保持空值。
5. 本次过渡部署同步部署 OpenTelemetry Collector 和 Tempo；为避免后端早于观测组件启动时出现追踪初始化依赖问题，应用初始配置先关闭 tracing，待 D12 观测组件就绪后再开启并重启 all-in-one。
6. 本次过渡部署同步部署 Cloudflare Tunnel，因此 `CLOUDFLARED_TUNNEL_TOKEN` 必须来自当前 `.env` 或当前 shell 环境。

待执行命令：

```bash
# 确认变量仍存在；若是新终端，请重新执行 D2 的 export
echo "${NS}"

# 可选：若项目根目录存在 .env，则加载现有环境变量；不要输出变量值
set -a
[ -f .env ] && . ./.env
set +a

# 设置非敏感默认值
: "${PUBLIC_BASE_URL:=https://localhost:8443}"
: "${AUTH_OWNER_USERNAME:=aroen}"
: "${AUTH_SESSION_COOKIE_NAME:=messagefeed_session}"
: "${AUTH_SESSION_TTL:=604800}"
: "${AUTH_SESSION_COOKIE_SECURE:=false}"
: "${AUTH_OAUTH_STATE_TTL:=600}"
: "${AUTH_APPROVAL_TOKEN_TTL:=1800}"
: "${EMBEDDING_DIMENSION:=4096}"
: "${GATEWAY_SITE_ADDRESS:=https://localhost:8443}"
: "${GATEWAY_DEFAULT_SNI:=localhost}"
: "${CLOUDFLARED_PROTOCOL:=auto}"

# Cloudflare Tunnel 本步骤要求同步部署；token 为空时停止执行
test -n "${CLOUDFLARED_TUNNEL_TOKEN:-}" || { echo "CLOUDFLARED_TUNNEL_TOKEN is required for this transition deployment"; exit 1; }

# 交互录入敏感值；不要把值复制到本文档或聊天记录
read -rsp "POSTGRES_PASSWORD for K8s PostgreSQL: " POSTGRES_PASSWORD; echo
read -rsp "AUTH_OWNER_PASSWORD for messageFeed login: " AUTH_OWNER_PASSWORD; echo

# 生成应用连接 K8s PostgreSQL 的 DATABASE_URL
export DATABASE_URL="postgres://messagefeed:${POSTGRES_PASSWORD}@messagefeed-postgres:5432/messagefeed?sslmode=disable"

# 创建 PostgreSQL Secret
kubectl -n "${NS}" create secret generic messagefeed-postgres-secret \
  --from-env-file=<(printf 'POSTGRES_USER=%s\nPOSTGRES_PASSWORD=%s\nPOSTGRES_DB=%s\n' \
    "messagefeed" "${POSTGRES_PASSWORD}" "messagefeed") \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建应用敏感配置 Secret
kubectl -n "${NS}" create secret generic messagefeed-app-secret \
  --from-env-file=<(cat <<EOF
DATABASE_URL=${DATABASE_URL}
AUTH_OWNER_PASSWORD=${AUTH_OWNER_PASSWORD}
WECHAT_WORK_SECRET=${WECHAT_WORK_SECRET:-}
WECHAT_WORK_CALLBACK_TOKEN=${WECHAT_WORK_CALLBACK_TOKEN:-}
WECHAT_WORK_ENCODING_AES_KEY=${WECHAT_WORK_ENCODING_AES_KEY:-}
LLM_API_KEY=${LLM_API_KEY:-}
LLM_CONFIG_SECRET=${LLM_CONFIG_SECRET:-}
EMBEDDING_API_KEY=${EMBEDDING_API_KEY:-}
EOF
) \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建应用非敏感配置 ConfigMap
kubectl -n "${NS}" create configmap messagefeed-app-config \
  --from-env-file=<(cat <<EOF
BIND_ADDR=0.0.0.0:60001
PUBLIC_BASE_URL=${PUBLIC_BASE_URL}
DEPLOYMENT_MODE=single_node
ENVIRONMENT=wsl-k3s
LOG_LEVEL=info
OTEL_SERVICE_NAME=messagefeed-all-in-one
OTEL_SERVICE_VERSION=0.2.0
OBSERVABILITY_TRACE_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACES_SAMPLER_ARG=1.0
AUTH_OWNER_USERNAME=${AUTH_OWNER_USERNAME}
AUTH_SESSION_COOKIE_NAME=${AUTH_SESSION_COOKIE_NAME}
AUTH_SESSION_TTL=${AUTH_SESSION_TTL}
AUTH_SESSION_COOKIE_SECURE=${AUTH_SESSION_COOKIE_SECURE}
AUTH_OAUTH_STATE_TTL=${AUTH_OAUTH_STATE_TTL}
AUTH_APPROVAL_TOKEN_TTL=${AUTH_APPROVAL_TOKEN_TTL}
WECHAT_WORK_CORP_ID=${WECHAT_WORK_CORP_ID:-}
WECHAT_WORK_AGENT_ID=${WECHAT_WORK_AGENT_ID:-}
LLM_PROVIDER=${LLM_PROVIDER:-}
LLM_BASE_URL=${LLM_BASE_URL:-}
LLM_MODEL=${LLM_MODEL:-}
EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER:-}
EMBEDDING_BASE_URL=${EMBEDDING_BASE_URL:-}
EMBEDDING_MODEL=${EMBEDDING_MODEL:-}
EMBEDDING_DIMENSION=${EMBEDDING_DIMENSION}
TZ=UTC
EOF
) \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建 gateway 非敏感配置
kubectl -n "${NS}" create configmap messagefeed-gateway-config \
  --from-env-file=<(cat <<EOF
GATEWAY_SITE_ADDRESS=${GATEWAY_SITE_ADDRESS}
GATEWAY_DEFAULT_SNI=${GATEWAY_DEFAULT_SNI}
EOF
) \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建 Caddyfile ConfigMap；当前 Caddyfile.prod 依赖 K8s Service 名称 api、web、gateway、gateway-dev
kubectl -n "${NS}" create configmap messagefeed-caddy-config \
  --from-file=Caddyfile=deploy/caddy/Caddyfile.prod \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建 Caddy 与 cloudflared 共用证书 Secret；包含当前项目已有证书和 CA bundle
kubectl -n "${NS}" create secret generic messagefeed-caddy-certs \
  --from-file=deploy/caddy/certs/gateway-dev.crt \
  --from-file=deploy/caddy/certs/gateway-dev.key \
  --from-file=deploy/caddy/certs/cloudflared-ca-bundle.crt \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建 Cloudflare Tunnel Secret；不输出 token 明文
kubectl -n "${NS}" create secret generic messagefeed-cloudflared-secret \
  --from-env-file=<(cat <<EOF
CLOUDFLARED_TUNNEL_TOKEN=${CLOUDFLARED_TUNNEL_TOKEN}
CLOUDFLARED_PROTOCOL=${CLOUDFLARED_PROTOCOL}
EOF
) \
  --dry-run=client -o yaml | kubectl apply -f -

# 创建观测组件 ConfigMap
kubectl -n "${NS}" create configmap messagefeed-prometheus-config \
  --from-file=prometheus.yml=ops/observability/prometheus/prometheus.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-loki-config \
  --from-file=loki.yml=ops/observability/loki/loki.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-tempo-config \
  --from-file=tempo.yml=ops/observability/tempo/tempo.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-otel-collector-config \
  --from-file=otel-collector.yml=ops/observability/otel-collector/otel-collector.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-grafana-datasources \
  --from-file=datasources.yml=ops/observability/grafana/provisioning/datasources/datasources.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-grafana-dashboards-provider \
  --from-file=dashboards.yml=ops/observability/grafana/provisioning/dashboards/dashboards.yml \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap messagefeed-grafana-dashboards \
  --from-file=messagefeed-overview.json=ops/observability/grafana/dashboards/messagefeed-overview.json \
  --dry-run=client -o yaml | kubectl apply -f -

# 查看对象是否存在；不输出 Secret 明文
kubectl -n "${NS}" get secret messagefeed-postgres-secret messagefeed-app-secret
kubectl -n "${NS}" get secret messagefeed-caddy-certs messagefeed-cloudflared-secret
kubectl -n "${NS}" get configmap messagefeed-app-config messagefeed-gateway-config messagefeed-caddy-config
kubectl -n "${NS}" get configmap messagefeed-prometheus-config messagefeed-loki-config messagefeed-tempo-config messagefeed-otel-collector-config
kubectl -n "${NS}" get configmap messagefeed-grafana-datasources messagefeed-grafana-dashboards-provider messagefeed-grafana-dashboards

# 清理当前 shell 中的敏感变量，避免后续误输出
unset POSTGRES_PASSWORD AUTH_OWNER_PASSWORD DATABASE_URL
```

预期：

1. `messagefeed-postgres-secret` 创建成功。
2. `messagefeed-app-secret` 创建成功。
3. `messagefeed-app-config` 创建成功。
4. `messagefeed-cloudflared-secret` 创建成功。
5. Caddy 与观测组件 ConfigMap 创建成功。
6. 命令输出不包含敏感值明文。

当前实施步骤反馈：

```text
D4 反馈：
1. PostgreSQL Secret：
2. App Secret：
3. App ConfigMap：
4. Cloudflare Tunnel Secret：
5. Caddy ConfigMap/Secret：
6. 观测组件 ConfigMap：
7. 是否启用企业微信配置：
8. 是否启用 LLM 配置：
9. 是否启用 Embedding 配置：
10. 是否可以进入 D5：
```

### D5. 部署 K8s PostgreSQL/pgvector

待执行命令：

```bash
# 确认 namespace
echo "${NS}"

# 创建 PostgreSQL Service 与 StatefulSet
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: messagefeed-postgres
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: messagefeed-postgres
    app.kubernetes.io/part-of: messagefeed
spec:
  clusterIP: None
  ports:
    - name: postgres
      port: 5432
      targetPort: postgres
  selector:
    app.kubernetes.io/name: messagefeed-postgres
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: messagefeed-postgres
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: messagefeed-postgres
    app.kubernetes.io/part-of: messagefeed
spec:
  serviceName: messagefeed-postgres
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: messagefeed-postgres
  template:
    metadata:
      labels:
        app.kubernetes.io/name: messagefeed-postgres
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: postgres
          image: pgvector/pgvector:pg15
          imagePullPolicy: IfNotPresent
          ports:
            - name: postgres
              containerPort: 5432
          envFrom:
            - secretRef:
                name: messagefeed-postgres-secret
          env:
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
            - name: TZ
              value: UTC
            - name: PGTZ
              value: UTC
          readinessProbe:
            exec:
              command:
                - sh
                - -c
                - pg_isready -U "\${POSTGRES_USER}" -d "\${POSTGRES_DB}"
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
          livenessProbe:
            exec:
              command:
                - sh
                - -c
                - pg_isready -U "\${POSTGRES_USER}" -d "\${POSTGRES_DB}"
            initialDelaySeconds: 30
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          volumeMounts:
            - name: postgres-data
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: postgres-data
      spec:
        accessModes:
          - ReadWriteOnce
        storageClassName: local-path
        resources:
          requests:
            storage: 10Gi
EOF

# 等待 PostgreSQL StatefulSet 就绪
kubectl -n "${NS}" rollout status statefulset/messagefeed-postgres --timeout=240s

# 验证 PostgreSQL readiness
POSTGRES_POD="$(kubectl -n "${NS}" get pod -l app.kubernetes.io/name=messagefeed-postgres -o jsonpath='{.items[0].metadata.name}')"
kubectl -n "${NS}" exec "${POSTGRES_POD}" -- pg_isready -U messagefeed -d messagefeed

# 查看 PVC 与 Pod 状态
kubectl -n "${NS}" get pod,pvc,svc -l app.kubernetes.io/part-of=messagefeed -o wide
```

预期：

1. `messagefeed-postgres-0` 为 `Running`。
2. PostgreSQL readiness 成功。
3. PVC 绑定成功。
4. 此时只创建了新的 K8s PostgreSQL 数据目录，尚未恢复备份。

当前实施步骤反馈：

```text
D5 反馈：
1. PostgreSQL Pod 状态：
2. PVC 状态：
3. pg_isready 结果：
4. 是否可以进入 D6：
```

### D6. 恢复 D-1 数据库备份到 K8s PostgreSQL

说明：本步骤只面向新建空库执行。若目标 K8s 数据库已经存在业务表，则停止执行并先反馈，不要覆盖已有数据。

待执行命令：

```bash
# 确认 namespace 与备份文件
echo "${NS}"
echo "${DB_BACKUP_FILE}"

# 校验备份文件 sha256；sha256 文件使用相对文件名，因此进入备份目录执行
(cd "$(dirname "${DB_BACKUP_FILE}")" && sha256sum -c "$(basename "${DB_BACKUP_FILE}").sha256")

# 确认目标数据库为空；新库 public base table count 应为 0
POSTGRES_POD="$(kubectl -n "${NS}" get pod -l app.kubernetes.io/name=messagefeed-postgres -o jsonpath='{.items[0].metadata.name}')"
TARGET_TABLE_COUNT="$(kubectl -n "${NS}" exec "${POSTGRES_POD}" -- psql -U messagefeed -d messagefeed -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';")"
echo "target_public_base_table_count=${TARGET_TABLE_COUNT}"
test "${TARGET_TABLE_COUNT}" = "0" || { echo "target database is not empty; stop before restore"; exit 1; }

# 将 custom dump 流式恢复到 K8s PostgreSQL
kubectl -n "${NS}" exec -i "${POSTGRES_POD}" -- pg_restore \
  -U messagefeed \
  -d messagefeed \
  --no-owner \
  --no-privileges \
  --single-transaction \
  < "${DB_BACKUP_FILE}"

# 验证恢复后的迁移版本、pgvector 和表数量
kubectl -n "${NS}" exec "${POSTGRES_POD}" -- psql -U messagefeed -d messagefeed -Atc "SELECT version::text || ',' || dirty::text FROM schema_migrations LIMIT 1;"
kubectl -n "${NS}" exec "${POSTGRES_POD}" -- psql -U messagefeed -d messagefeed -Atc "SELECT extversion FROM pg_extension WHERE extname='vector';"
kubectl -n "${NS}" exec "${POSTGRES_POD}" -- psql -U messagefeed -d messagefeed -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';"
```

预期：

1. sha256 校验通过。
2. 恢复前目标库 public base table count 为 `0`。
3. `pg_restore` 成功完成。
4. `schema_migrations` 为 `37,false`。
5. pgvector extension 存在。
6. 恢复后 public base table count 与备份 metadata 一致，当前记录为 `55`。

当前实施步骤反馈：

```text
D6 反馈：
1. sha256 校验：
2. 恢复前目标表数量：
3. pg_restore 结果：
4. schema_migrations：
5. pgvector：
6. 恢复后表数量：
7. 是否可以进入 D7：
```

### D7. 挂载迁移文件并运行 migrate 校验 Job

待执行命令：

```bash
# 确认 namespace
echo "${NS}"

# 将 migrations 目录作为 ConfigMap 挂载给一次性迁移 Job
kubectl -n "${NS}" create configmap messagefeed-migrations \
  --from-file=migrations \
  --dry-run=client -o yaml | kubectl apply -f -

# 使用唯一 Job 名称，避免覆盖历史迁移记录对象
export MIGRATE_JOB="messagefeed-migrate-$(date +%Y%m%d%H%M%S)"

# 创建迁移校验 Job；若备份已是最新版本，migrate up 应为 no change 或直接完成
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${MIGRATE_JOB}
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: messagefeed-migrate
    app.kubernetes.io/part-of: messagefeed
spec:
  backoffLimit: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: messagefeed-migrate
        app.kubernetes.io/part-of: messagefeed
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: migrate/migrate:v4.19.1
          imagePullPolicy: IfNotPresent
          args:
            - -path
            - /migrations
            - -database
            - \$(DATABASE_URL)
            - up
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: messagefeed-app-secret
                  key: DATABASE_URL
          volumeMounts:
            - name: migrations
              mountPath: /migrations
              readOnly: true
      volumes:
        - name: migrations
          configMap:
            name: messagefeed-migrations
EOF

# 等待迁移校验完成
kubectl -n "${NS}" wait --for=condition=complete "job/${MIGRATE_JOB}" --timeout=300s
kubectl -n "${NS}" logs "job/${MIGRATE_JOB}" --tail=200
```

预期：

1. 迁移 Job `Complete`。
2. 日志不出现 dirty migration、连接失败或 pgvector extension 错误。
3. `/readyz` 后续可以检查到 migrations 和 pgvector。

当前实施步骤反馈：

```text
D7 反馈：
1. migrations ConfigMap：
2. MIGRATE_JOB：
3. Job 状态：
4. 迁移日志摘要：
5. 是否可以进入 D8：
```

### D8. 部署当前整体后端 all-in-one Pod

说明：当前代码尚未实现 `APP_ROLE`。本 Deployment 不依赖 `APP_ROLE` 控制职责，而是利用现有默认行为启动完整后端进程。为避免后台任务重复运行，副本数必须保持为 `1`。

待执行命令：

```bash
# 确认变量仍存在；若是新终端，请重新执行 D2 的 export
echo "${NS}"
echo "${API_IMAGE}"

# 创建 all-in-one Deployment 与 API Service；api 服务名用于 Caddyfile.prod 中 reverse_proxy api:60001
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: messagefeed-all-in-one
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: messagefeed-all-in-one
    app.kubernetes.io/component: all-in-one
    app.kubernetes.io/part-of: messagefeed
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: messagefeed-all-in-one
  template:
    metadata:
      labels:
        app.kubernetes.io/name: messagefeed-all-in-one
        app.kubernetes.io/component: all-in-one
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: api
          image: ${API_IMAGE}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 60001
          envFrom:
            - configMapRef:
                name: messagefeed-app-config
            - secretRef:
                name: messagefeed-app-secret
          env:
            - name: APP_NODE_ID
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
          startupProbe:
            httpGet:
              path: /healthz
              port: http
            periodSeconds: 2
            timeoutSeconds: 5
            failureThreshold: 30
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
---
apiVersion: v1
kind: Service
metadata:
  name: api
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: api
    app.kubernetes.io/part-of: messagefeed
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: messagefeed-all-in-one
  ports:
    - name: http
      port: 60001
      targetPort: http
---
apiVersion: v1
kind: Service
metadata:
  name: messagefeed-api
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: messagefeed-api
    app.kubernetes.io/part-of: messagefeed
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: messagefeed-all-in-one
  ports:
    - name: http
      port: 60001
      targetPort: http
EOF

kubectl -n "${NS}" rollout status deploy/messagefeed-all-in-one --timeout=240s
kubectl -n "${NS}" get deploy,rs,pod,svc,endpoints -o wide
kubectl -n "${NS}" logs deploy/messagefeed-all-in-one --tail=200
```

预期：

1. `messagefeed-all-in-one` Deployment `Available=True`。
2. 只存在 1 个 all-in-one Pod。
3. `api` 与 `messagefeed-api` Service 均有 endpoints。
4. 日志显示 HTTP server 启动，且没有配置校验错误、数据库连接错误或迁移未就绪错误。

当前实施步骤反馈：

```text
D8 反馈：
1. Deployment 状态：
2. Pod 名称与状态：
3. api Service endpoints：
4. messagefeed-api Service endpoints：
5. 启动日志摘要：
6. 是否可以进入 D9：
```

### D9. 部署 Web 静态服务

待执行命令：

```bash
# 确认变量仍存在；若是新终端，请重新执行 D2 的 export
echo "${NS}"
echo "${WEB_IMAGE}"

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: web
    app.kubernetes.io/component: web
    app.kubernetes.io/part-of: messagefeed
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: web
  template:
    metadata:
      labels:
        app.kubernetes.io/name: web
        app.kubernetes.io/component: web
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: web
          image: ${WEB_IMAGE}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
          readinessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 5
          livenessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 15
            periodSeconds: 30
            timeoutSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: web
    app.kubernetes.io/part-of: messagefeed
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: web
  ports:
    - name: http
      port: 8080
      targetPort: http
EOF

kubectl -n "${NS}" rollout status deploy/web --timeout=180s
kubectl -n "${NS}" get deploy,pod,svc,endpoints -l app.kubernetes.io/name=web -o wide
```

当前实施步骤反馈：

```text
D9 反馈：
1. Web Deployment 状态：
2. Web Pod 状态：
3. Web Service endpoints：
4. 是否可以进入 D10：
```

### D10. 部署 Caddy gateway

待执行命令：

```bash
# 创建 gateway Deployment 与 gateway/gateway-dev Service
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: gateway
    app.kubernetes.io/component: gateway
    app.kubernetes.io/part-of: messagefeed
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: gateway
  template:
    metadata:
      labels:
        app.kubernetes.io/name: gateway
        app.kubernetes.io/component: gateway
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: caddy
          image: caddy:2.10.2-alpine
          imagePullPolicy: IfNotPresent
          envFrom:
            - configMapRef:
                name: messagefeed-gateway-config
          ports:
            - name: https
              containerPort: 8443
          readinessProbe:
            tcpSocket:
              port: https
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 5
          livenessProbe:
            tcpSocket:
              port: https
            initialDelaySeconds: 20
            periodSeconds: 30
            timeoutSeconds: 5
          volumeMounts:
            - name: caddyfile
              mountPath: /etc/caddy/Caddyfile
              subPath: Caddyfile
              readOnly: true
            - name: caddy-certs
              mountPath: /etc/caddy/certs
              readOnly: true
            - name: caddy-data
              mountPath: /data
            - name: caddy-config
              mountPath: /config
      volumes:
        - name: caddyfile
          configMap:
            name: messagefeed-caddy-config
        - name: caddy-certs
          secret:
            secretName: messagefeed-caddy-certs
        - name: caddy-data
          emptyDir: {}
        - name: caddy-config
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: gateway
    app.kubernetes.io/part-of: messagefeed
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: gateway
  ports:
    - name: https
      port: 8443
      targetPort: https
---
apiVersion: v1
kind: Service
metadata:
  name: gateway-dev
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: gateway-dev
    app.kubernetes.io/part-of: messagefeed
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: gateway
  ports:
    - name: https
      port: 8443
      targetPort: https
EOF

kubectl -n "${NS}" rollout status deploy/gateway --timeout=180s
kubectl -n "${NS}" get deploy,pod,svc,endpoints -l app.kubernetes.io/name=gateway -o wide
kubectl -n "${NS}" get svc,endpoints gateway gateway-dev -o wide
```

当前实施步骤反馈：

```text
D10 反馈：
1. Gateway Deployment 状态：
2. Gateway Pod 状态：
3. gateway/gateway-dev endpoints：
4. 是否可以进入 D11：
```

### D11. 部署 Cloudflare Tunnel

待执行命令：

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloudflared
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: cloudflared
    app.kubernetes.io/component: tunnel
    app.kubernetes.io/part-of: messagefeed
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: cloudflared
  template:
    metadata:
      labels:
        app.kubernetes.io/name: cloudflared
        app.kubernetes.io/component: tunnel
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: cloudflared
          image: cloudflare/cloudflared:latest
          imagePullPolicy: IfNotPresent
          args:
            - tunnel
            - --no-autoupdate
            - --metrics
            - 0.0.0.0:2000
            - --protocol
            - \$(CLOUDFLARED_PROTOCOL)
            - run
            - --token
            - \$(CLOUDFLARED_TUNNEL_TOKEN)
          env:
            - name: SSL_CERT_FILE
              value: /etc/messagefeed-certs/cloudflared-ca-bundle.crt
            - name: CLOUDFLARED_TUNNEL_TOKEN
              valueFrom:
                secretKeyRef:
                  name: messagefeed-cloudflared-secret
                  key: CLOUDFLARED_TUNNEL_TOKEN
            - name: CLOUDFLARED_PROTOCOL
              valueFrom:
                secretKeyRef:
                  name: messagefeed-cloudflared-secret
                  key: CLOUDFLARED_PROTOCOL
          ports:
            - name: metrics
              containerPort: 2000
          readinessProbe:
            httpGet:
              path: /ready
              port: metrics
            initialDelaySeconds: 10
            periodSeconds: 10
            timeoutSeconds: 5
          volumeMounts:
            - name: caddy-certs
              mountPath: /etc/messagefeed-certs
              readOnly: true
      volumes:
        - name: caddy-certs
          secret:
            secretName: messagefeed-caddy-certs
EOF

kubectl -n "${NS}" rollout status deploy/cloudflared --timeout=240s
kubectl -n "${NS}" get deploy,pod -l app.kubernetes.io/name=cloudflared -o wide
kubectl -n "${NS}" logs deploy/cloudflared --tail=120
```

当前实施步骤反馈：

```text
D11 反馈：
1. Cloudflared Deployment 状态：已创建；当前 `READY 1/1`、`AVAILABLE 1`。
2. Cloudflared Pod 状态：当前 `Running` 且 Ready；曾出现 3 次重启。
3. Cloudflared 日志摘要：日志仍显示 `protocol=quic`，并持续出现 `failed to dial to edge with quic: timeout: no recent network activity`、`failed to accept QUIC stream`、`lookup region1.v2.argotunnel.com: i/o timeout` 等网络错误。
4. 当前排查结论：`messagefeed-cloudflared-secret` 中 `CLOUDFLARED_PROTOCOL=auto`，尚未固定为 `http2`；Pod 内 DNS 查询可成功解析 `region1.v2.argotunnel.com`，但临时 Pod 访问 `https://api.cloudflare.com/cdn-cgi/trace` 超时；主机侧访问同一 Cloudflare 目标成功。因此问题主要位于 Pod 出站路径和 cloudflared 协议选择，而不是主机整体网络不可用。
5. 是否可以进入 D12：可以继续部署或核查 D12，但 D13 外部链路验收前应先执行 D11.1。
```

### D11.1 Cloudflare Tunnel 网络排查与修复

本步骤用于修复 D11 暴露的 tunnel 不稳定问题。当前证据显示：

1. WSL 主机 DNS 与 HTTPS 出站正常，`api.cloudflare.com:443` 和 `region1.v2.argotunnel.com:7844` 主机侧 TCP 可达。
2. K3s/Flannel 已存在 `10.42.0.0/16` Pod 网段的 MASQUERADE 规则，且计数器已命中；`net.ipv4.ip_forward=1`，`bridge-nf-call-iptables=1`。因此当前问题不是缺少 Pod 出站 NAT。
3. Pod 到外部 `api.cloudflare.com:443` 在绕过 DNS 后可成功访问，说明 Pod TCP 443 出站路径可用。
4. Pod 到多个公共 DNS 的 UDP/TCP 53 均超时，包括 `1.1.1.1`、`8.8.8.8`、`223.5.5.5`、`119.29.29.29`；CoreDNS 日志同步出现 `read udp 10.42.0.2 -> ...:53: i/o timeout`。
5. Pod 可访问 WSL 本地 DNS stub `10.255.255.254:53`，并可成功解析 `api.cloudflare.com`。
6. cloudflared 当前协议 Secret 为 `auto`，实际运行日志选择了 `quic`；Pod 到 `region1.v2.argotunnel.com:7844` 超时。当前网络条件下应固定为 `http2`，走 TCP 443。

最终处理原则：

1. 不添加额外 Pod NAT 规则。
2. 不把 CoreDNS 上游永久写死为某个临时 IP；CoreDNS 保持 `forward . /etc/resolv.conf`。
3. 将 WSL 的 `/etc/resolv.conf` 维护为“本机可用 DNS stub”的动态入口，使 CoreDNS 继续通过标准文件发现上游。
4. cloudflared 在 WSL 本地 K3s 环境中使用声明式网络 profile 固定为 `http2`；真实生产节点若确认 UDP/7844 可用，可切回 `auto` 或 `quic`。

深层原因分析：

1. 当前 WSL 网络不是单一路由出口。主机存在 `eth0 198.18.0.1/30`、下一跳 `198.18.0.2`、策略路由表 `127/128`、`loopback0`、本地 DNS stub `10.255.255.254:53`，并存在本地代理相关进程。主机本机发起的 DNS/HTTPS 流量与 Pod 经 `cni0` 转发、Flannel MASQUERADE 后的流量不完全等价。
2. `/etc/wsl.conf` 中 `generateResolvConf=false`，当前 `/etc/resolv.conf` 手动列出多个公共 DNS。CoreDNS 默认 `forward . /etc/resolv.conf`，因此 CoreDNS 会从 Pod 网络直接访问公共 DNS，而不是使用 WSL 本地 DNS stub。
3. 实测 Pod 到 `1.1.1.1`、`8.8.8.8`、`223.5.5.5` 等公共 DNS 的 UDP/TCP 53 均超时；但 Pod 到 `10.255.255.254:53` 可解析成功。这说明 WSL/代理/虚拟网关层对从 Pod 转发出的 DNS 53 流量不放行或不稳定，而 WSL 本地 DNS stub 是当前可用的 DNS 出口。
4. `cloudflared` 的 `auto` 并不等同于“QUIC 失败后一定自动切换 HTTP/2”。当前日志显示它无法稳定完成协议/特性探测，并选择了 `Initial protocol quic`；随后连接失败时继续重试 QUIC。由于 Pod 到 `region1.v2.argotunnel.com:7844` 超时，而 Pod 到 `api.cloudflare.com:443` 成功，固定 `http2` 是当前环境下的确定性方案。
5. 因此可维护修复应把“WSL 当前可用 DNS stub 是什么”交给 systemd 动态发现，把“本地 WSL 网络不保证 UDP/7844”表达为环境 profile，而不是在每次故障时手动改 CoreDNS 或临时添加 iptables 规则。

#### D11.1.1 根治 DNS 入口：动态维护 `/etc/resolv.conf`

本步骤目标：CoreDNS 继续保留 `forward . /etc/resolv.conf`，由 WSL 侧 systemd 服务动态发现本机可用 DNS stub 并写入 `/etc/resolv.conf`。这样后续 WSL DNS stub 地址变化时，不需要改 CoreDNS ConfigMap。

待执行命令：

```bash
# 先确认 Pod 可访问 WSL 本地 DNS stub
kubectl -n "${NS}" run dns-stub-test-$(date +%s) --rm -i --restart=Never --image=busybox:1.36 \
  -- nslookup api.cloudflare.com 10.255.255.254

# 保留当前 /etc/resolv.conf 备份
sudo install -m 0644 /etc/resolv.conf "/etc/resolv.conf.before-k3s-wsl-dns-sync-$(date +%Y%m%d%H%M%S)"

# 创建动态同步服务：自动选择非 127.0.0.0/8 的本机 DNS 监听地址，例如当前的 10.255.255.254
sudo tee /etc/systemd/system/k3s-wsl-dns-sync.service >/dev/null <<'EOF'
[Unit]
Description=Synchronize WSL resolver for K3s CoreDNS
After=systemd-resolved.service network-online.target
Wants=network-online.target
Before=k3s.service

[Service]
Type=oneshot
ExecStart=/usr/bin/python3 -c "import re, subprocess; out=subprocess.check_output(['/usr/bin/ss','-H','-lun'], text=True); addrs=[re.sub(r':53$', '', line.split()[3]) for line in out.splitlines() if len(line.split()) > 3 and line.split()[3].endswith(':53')]; addrs=[a for a in addrs if not a.startswith('127.') and a not in ('0.0.0.0', '*')]; assert addrs, 'no non-loopback local DNS listener found'; dns=addrs[0]; open('/etc/resolv.conf', 'w').write('options timeout:1 attempts:2 rotate\nnameserver ' + dns + '\n'); print('K3s/CoreDNS resolver upstream: ' + dns)"

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable k3s-wsl-dns-sync.service
sudo systemctl restart k3s-wsl-dns-sync.service
sudo systemctl status k3s-wsl-dns-sync.service --no-pager || true

# 确保 K3s 启动前先同步 resolver
sudo install -d -m 0755 /etc/systemd/system/k3s.service.d
sudo tee /etc/systemd/system/k3s.service.d/05-wsl-dns-sync.conf >/dev/null <<'EOF'
[Unit]
Requires=k3s-wsl-dns-sync.service
After=k3s-wsl-dns-sync.service
EOF

# 周期性重跑，覆盖 WSL 网络恢复或 DNS stub 变化
sudo tee /etc/systemd/system/k3s-wsl-dns-sync.timer >/dev/null <<'EOF'
[Unit]
Description=Periodically synchronize WSL resolver for K3s CoreDNS

[Timer]
OnBootSec=30s
OnUnitActiveSec=2min
AccuracySec=15s
Unit=k3s-wsl-dns-sync.service

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now k3s-wsl-dns-sync.timer
sudo systemctl list-timers k3s-wsl-dns-sync.timer --no-pager

# 验证 /etc/resolv.conf 已变为本机 DNS stub，而不是公共 DNS 列表
sed -n '1,40p' /etc/resolv.conf

# 确保 CoreDNS 仍使用标准入口 /etc/resolv.conf；如不是，则改回
kubectl -n kube-system get cm coredns -o jsonpath='{.data.Corefile}'; echo
kubectl -n kube-system get cm coredns -o yaml \
  | sed 's#forward \\. 10.255.255.254#forward . /etc/resolv.conf#' \
  | kubectl apply -f -

# 重启 CoreDNS 使其重新读取 /etc/resolv.conf
kubectl -n kube-system rollout restart deploy/coredns
kubectl -n kube-system rollout status deploy/coredns --timeout=120s

# 验证 Pod 经 Cluster DNS 解析外部域名
kubectl -n "${NS}" run dns-test-$(date +%s) --rm -i --restart=Never --image=busybox:1.36 \
  -- nslookup api.cloudflare.com

kubectl -n "${NS}" run dns-argo-test-$(date +%s) --rm -i --restart=Never --image=busybox:1.36 \
  -- nslookup region1.v2.argotunnel.com
```

#### D11.1.2 自动选择 cloudflared 网络与协议 profile

本步骤目标：不依赖 `cloudflared --protocol auto` 的内部选择逻辑，也不假设普通 Pod 网络一定能出站。先在目标 namespace 内做 preflight 探测，自动选择 `podNetwork` 或 `hostNetwork`，再在选定网络模式下选择协议，并把结果写入 `messagefeed-network-profile` ConfigMap 和 `messagefeed-cloudflared-secret`。Deployment 只消费最终 profile。

当前 WSL 实测结论：

1. 普通 Pod 经 Cluster DNS 可解析 `region1.v2.argotunnel.com`。
2. 普通 Pod 访问 `api.cloudflare.com:443` 超时。
3. 抓包显示普通 Pod 的 SYN 包已从 `cni0` 进入、经 SNAT 变为 `192.168.3.40` 后从 `eth2` 发出，但无 SYN-ACK 返回。
4. 主机本机从同一 `eth2` 访问同一 Cloudflare IP 成功。
5. `hostNetwork` 临时 Pod 访问 `api.cloudflare.com:443` 成功，访问集群内 `https://gateway:8443/healthz` 成功。
6. 因此本机 WSL profile 应使用 `hostNetwork: true` 与 `dnsPolicy: ClusterFirstWithHostNet`，避免让 cloudflared 出站流量经过 WSL/Windows 不稳定的 Pod 转发路径。

hostNetwork profile 影响与边界：

1. 该 profile 只用于 `cloudflared`，不用于 API、Web、gateway、PostgreSQL 或 worker。
2. `hostNetwork: true` 会让 cloudflared 使用节点网络命名空间，网络隔离弱于普通 Pod。
3. hostNetwork Pod 通常不按普通 Pod 方式受 NetworkPolicy 约束，因此不能把它作为业务服务默认模式。
4. 容器监听端口会占用节点端口；当前 `--metrics 0.0.0.0:2000` 在单节点 WSL 中与多副本 hostNetwork 不兼容。
5. WSL/K3s 本地过渡部署建议 `cloudflared replicas=1`；生产 podNetwork profile 才考虑 `replicas=2`。
6. 必须设置 `dnsPolicy: ClusterFirstWithHostNet`，否则 hostNetwork Pod 可能无法稳定解析 `gateway` 等 Kubernetes Service。
7. 真实 Linux 节点或云服务器应优先使用 podNetwork；只有 preflight 证明普通 Pod 出站不可用时才切 hostNetwork。
8. 该例外不破坏 API/Web/Gateway/PostgreSQL 的 ClusterIP、PVC、Secret、ConfigMap、rollout 等 Kubernetes 常规能力，但会降低 cloudflared 这一项的网络可移植性。
9. hostNetwork profile 若使用固定 metrics 端口 `0.0.0.0:2000`，Deployment 策略应使用 `Recreate`，避免默认 RollingUpdate 在新旧 Pod 并存时产生节点端口冲突或旧副本迟迟不能终止。
10. 协议选择必须以 cloudflared precheck 实测为准：若 `UDP Connectivity ... PASS` 且 `TCP Connectivity ... FAIL`，应选择 `quic`；不能机械地因 WSL 环境而固定 `http2`。

待执行命令：

```bash
# 先确认当前协议值；应避免输出 token 明文
kubectl -n "${NS}" get secret messagefeed-cloudflared-secret \
  -o jsonpath='{.data.CLOUDFLARED_PROTOCOL}' | base64 -d; echo

# 先确认 DNS 和 TCP 443 基础链路；失败时不应继续重启 cloudflared
kubectl -n "${NS}" run cf-dns-test-$(date +%s) --rm -i --restart=Never --image=busybox:1.36 \
  -- nslookup region1.v2.argotunnel.com

kubectl -n "${NS}" run cf-https-test-$(date +%s) --rm -i --restart=Never --image=curlimages/curl:8.8.0 \
  -- curl -4fsS --connect-timeout 10 --max-time 25 https://api.cloudflare.com/cdn-cgi/trace

# 如果普通 Pod HTTPS 失败，则验证 hostNetwork 是否可用
kubectl -n "${NS}" run cf-hostnet-https-test-$(date +%s) --rm -i --restart=Never --image=curlimages/curl:8.8.0 \
  --overrides='{"spec":{"hostNetwork":true,"dnsPolicy":"ClusterFirstWithHostNet"}}' \
  -- curl -4fsS --connect-timeout 10 --max-time 25 https://api.cloudflare.com/cdn-cgi/trace

kubectl -n "${NS}" run cf-hostnet-gateway-test-$(date +%s) --rm -i --restart=Never --image=curlimages/curl:8.8.0 \
  --overrides='{"spec":{"hostNetwork":true,"dnsPolicy":"ClusterFirstWithHostNet"}}' \
  -- curl -kfsS --connect-timeout 10 --max-time 20 https://gateway:8443/healthz

# 当前 WSL 若普通 Pod HTTPS 失败且 hostNetwork HTTPS/gateway 成功，则设置：
DETECTED_NETWORK_MODE="hostNetwork"

# 使用 cloudflared 自身探测 quic；token 通过 Secret 文件挂载，不通过命令行或日志输出。
# 若 DETECTED_NETWORK_MODE=hostNetwork，则探测 Job 同样使用 hostNetwork。
PROBE_JOB="cloudflared-probe-quic-$(date +%Y%m%d%H%M%S)"

cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${PROBE_JOB}
  namespace: ${NS}
  labels:
    app.kubernetes.io/name: cloudflared-probe
    app.kubernetes.io/part-of: messagefeed
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 45
  ttlSecondsAfterFinished: 300
  template:
    spec:
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      restartPolicy: Never
      containers:
        - name: cloudflared-probe
          image: cloudflare/cloudflared:latest
          imagePullPolicy: IfNotPresent
          args:
            - tunnel
            - --no-autoupdate
            - --metrics
            - 0.0.0.0:2000
            - --protocol
            - quic
            - run
            - --token-file
            - /var/run/cloudflared-token/CLOUDFLARED_TUNNEL_TOKEN
          volumeMounts:
            - name: cloudflared-token
              mountPath: /var/run/cloudflared-token
              readOnly: true
      volumes:
        - name: cloudflared-token
          secret:
            secretName: messagefeed-cloudflared-secret
EOF

# 等待探测产生日志；Job 因 activeDeadlineSeconds 结束也可接受
sleep 50

PROBE_LOG="$(kubectl -n "${NS}" logs "job/${PROBE_JOB}" --tail=260 || true)"

if printf '%s\n' "${PROBE_LOG}" | grep -q 'precheck component="UDP Connectivity".*status=pass' \
  && printf '%s\n' "${PROBE_LOG}" | grep -q 'precheck component="TCP Connectivity".*status=fail'; then
  DETECTED_PROTOCOL="quic"
elif printf '%s\n' "${PROBE_LOG}" | grep -q 'Registered tunnel connection.*protocol=quic'; then
  DETECTED_PROTOCOL="quic"
else
  DETECTED_PROTOCOL="http2"
fi

echo "detected_cloudflared_protocol=${DETECTED_PROTOCOL}"

# 记录网络 profile，便于后续审计
kubectl -n "${NS}" create configmap messagefeed-network-profile \
  --from-literal=cloudflared_network_mode="${DETECTED_NETWORK_MODE}" \
  --from-literal=cloudflared_protocol="${DETECTED_PROTOCOL}" \
  --from-literal=detected_at="$(date -Is)" \
  --from-literal=profile_source="k8s-preflight" \
  --dry-run=client -o yaml | kubectl apply -f -

# 将最终协议写入 Secret；不读取、不输出 tunnel token
ENCODED_PROTOCOL="$(printf '%s' "${DETECTED_PROTOCOL}" | base64 -w0)"
kubectl -n "${NS}" patch secret messagefeed-cloudflared-secret \
  --type='json' \
  -p="[{\"op\":\"replace\",\"path\":\"/data/CLOUDFLARED_PROTOCOL\",\"value\":\"${ENCODED_PROTOCOL}\"}]"

# 若 DETECTED_NETWORK_MODE=hostNetwork，则将 cloudflared Deployment 改为 hostNetwork。
# 该操作只影响 cloudflared，不改变 API/Web/Gateway/PostgreSQL 的网络模式。
if [ "${DETECTED_NETWORK_MODE}" = "hostNetwork" ]; then
  kubectl -n "${NS}" patch deploy cloudflared --type='merge' -p='{"spec":{"template":{"spec":{"hostNetwork":true,"dnsPolicy":"ClusterFirstWithHostNet"}}}}'
  kubectl -n "${NS}" patch deploy cloudflared --type='json' -p='[
    {"op":"remove","path":"/spec/strategy/rollingUpdate"},
    {"op":"replace","path":"/spec/strategy/type","value":"Recreate"}
  ]'
fi

# 重启 cloudflared 使 Secret 生效
kubectl -n "${NS}" rollout restart deploy/cloudflared
kubectl -n "${NS}" rollout status deploy/cloudflared --timeout=240s

# 验证协议值和 profile 记录
kubectl -n "${NS}" get secret messagefeed-cloudflared-secret \
  -o jsonpath='{.data.CLOUDFLARED_PROTOCOL}' | base64 -d; echo
kubectl -n "${NS}" get cm messagefeed-network-profile -o yaml

# 验证日志中协议与错误情况；注意不要粘贴 token 明文
kubectl -n "${NS}" logs deploy/cloudflared --tail=160
```

#### D11.1.3 修复后验收

待执行命令：

```bash
# 查看所有主要工作负载
kubectl -n "${NS}" get deploy,statefulset,pod,svc,endpointslice -o wide

# 查看 CoreDNS 是否仍持续出现外部 DNS 超时
kubectl -n kube-system logs deploy/coredns --tail=120 | grep -E 'timeout|argotunnel|cloudflare|read udp' || true

# 查看 cloudflared 是否已稳定连接；日志中应优先看到 http2 连接，且不应持续出现 quic 超时
kubectl -n "${NS}" logs deploy/cloudflared --tail=160

# hostNetwork profile 下 cloudflared 已直接监听节点 2000 端口，不需要 port-forward
curl -fsS http://127.0.0.1:2000/ready
curl -fsS http://127.0.0.1:2000/metrics | head
```

若当前不是 hostNetwork profile，才另开终端执行 port-forward：

```bash
kubectl -n "${NS}" port-forward deploy/cloudflared 2000:2000
```

当前实施步骤反馈：

```text
D11.1 反馈：
1. Pod 到 `10.255.255.254` DNS stub 测试结果：成功，可解析 `api.cloudflare.com`。
2. `k3s-wsl-dns-sync.service` 是否启动成功：成功；oneshot 状态为 `inactive (dead)`，上一轮执行状态为 `status=0/SUCCESS`；服务已 enabled。
3. K3s 启动依赖是否已配置：已配置 `/etc/systemd/system/k3s.service.d/05-wsl-dns-sync.conf`，要求 K3s 在 `k3s-wsl-dns-sync.service` 后启动。
4. `k3s-wsl-dns-sync.timer` 是否已启用：已启用；最近一次触发成功，下一次计划约 2 分钟后执行。
5. `/etc/resolv.conf` 当前状态：已由动态同步服务写入 `nameserver 10.255.255.254`；变更前备份为 `/etc/resolv.conf.before-k3s-wsl-dns-sync-20260707213905`。
6. CoreDNS rollout 是否成功：成功。
7. Pod 经 Cluster DNS 解析 `api.cloudflare.com` 结果：成功，返回 Cloudflare 多个 IPv4 地址。
8. Pod 经 Cluster DNS 解析 `region1.v2.argotunnel.com` 结果：成功，返回 Cloudflare Tunnel region 多个 IPv4 地址。
9. cloudflared 协议自动 profile 是否已执行：尚未执行；D11.1.2 已写入自动探测与写入方案。
10. cloudflared 协议 Secret 是否已变更：
11. cloudflared rollout 是否成功：
12. Pod 内 HTTPS 访问 Cloudflare 结果：
13. CoreDNS 是否仍持续出现外部 DNS timeout：修复后最近 `deploy/coredns --tail=120` 未再匹配到 `timeout|argotunnel|cloudflare|read udp|plugin/errors`。
14. cloudflared 日志是否仍持续出现 quic/http2 连接错误：
15. 是否可以进入 D13 完整链路验收：
```

### D12. 部署观测组件

说明：本步骤同步部署 Prometheus、Loki、Promtail、Tempo、OpenTelemetry Collector 与 Grafana。Promtail 使用 Kubernetes 日志采集配置，不再使用 Docker socket 采集。

待执行命令：

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-data
  namespace: ${NS}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: loki-data
  namespace: ${NS}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tempo-data
  namespace: ${NS}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: grafana-data
  namespace: ${NS}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: prometheus
  template:
    metadata:
      labels:
        app.kubernetes.io/name: prometheus
        app.kubernetes.io/part-of: messagefeed
    spec:
      securityContext:
        fsGroup: 65534
      containers:
        - name: prometheus
          image: prom/prometheus:v3.8.1
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --storage.tsdb.retention.time=7d
            - --web.enable-lifecycle
          ports:
            - name: http
              containerPort: 9090
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus/prometheus.yml
              subPath: prometheus.yml
              readOnly: true
            - name: data
              mountPath: /prometheus
      volumes:
        - name: config
          configMap:
            name: messagefeed-prometheus-config
        - name: data
          persistentVolumeClaim:
            claimName: prometheus-data
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: ${NS}
spec:
  selector:
    app.kubernetes.io/name: prometheus
  ports:
    - name: http
      port: 9090
      targetPort: http
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: loki
  template:
    metadata:
      labels:
        app.kubernetes.io/name: loki
        app.kubernetes.io/part-of: messagefeed
    spec:
      securityContext:
        fsGroup: 10001
      containers:
        - name: loki
          image: grafana/loki:3.5.8
          args:
            - -config.file=/etc/loki/loki.yml
          ports:
            - name: http
              containerPort: 3100
            - name: grpc
              containerPort: 9096
          volumeMounts:
            - name: config
              mountPath: /etc/loki/loki.yml
              subPath: loki.yml
              readOnly: true
            - name: data
              mountPath: /loki
      volumes:
        - name: config
          configMap:
            name: messagefeed-loki-config
        - name: data
          persistentVolumeClaim:
            claimName: loki-data
---
apiVersion: v1
kind: Service
metadata:
  name: loki
  namespace: ${NS}
spec:
  selector:
    app.kubernetes.io/name: loki
  ports:
    - name: http
      port: 3100
      targetPort: http
    - name: grpc
      port: 9096
      targetPort: grpc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tempo
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: tempo
  template:
    metadata:
      labels:
        app.kubernetes.io/name: tempo
        app.kubernetes.io/part-of: messagefeed
    spec:
      securityContext:
        fsGroup: 10001
      containers:
        - name: tempo
          image: grafana/tempo:2.9.0
          args:
            - -config.file=/etc/tempo/tempo.yml
          ports:
            - name: http
              containerPort: 3200
            - name: otlp-grpc
              containerPort: 4317
            - name: otlp-http
              containerPort: 4318
          volumeMounts:
            - name: config
              mountPath: /etc/tempo/tempo.yml
              subPath: tempo.yml
              readOnly: true
            - name: data
              mountPath: /var/tempo
      volumes:
        - name: config
          configMap:
            name: messagefeed-tempo-config
        - name: data
          persistentVolumeClaim:
            claimName: tempo-data
---
apiVersion: v1
kind: Service
metadata:
  name: tempo
  namespace: ${NS}
spec:
  selector:
    app.kubernetes.io/name: tempo
  ports:
    - name: http
      port: 3200
      targetPort: http
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
    - name: otlp-http
      port: 4318
      targetPort: otlp-http
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: otel-collector
  template:
    metadata:
      labels:
        app.kubernetes.io/name: otel-collector
        app.kubernetes.io/part-of: messagefeed
    spec:
      containers:
        - name: otel-collector
          image: otel/opentelemetry-collector-contrib:0.142.0
          args:
            - --config=/etc/otel-collector.yml
          ports:
            - name: otlp-grpc
              containerPort: 4317
            - name: otlp-http
              containerPort: 4318
            - name: metrics
              containerPort: 8888
          volumeMounts:
            - name: config
              mountPath: /etc/otel-collector.yml
              subPath: otel-collector.yml
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: messagefeed-otel-collector-config
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector
  namespace: ${NS}
spec:
  selector:
    app.kubernetes.io/name: otel-collector
  ports:
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
    - name: otlp-http
      port: 4318
      targetPort: otlp-http
    - name: metrics
      port: 8888
      targetPort: metrics
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: grafana
  template:
    metadata:
      labels:
        app.kubernetes.io/name: grafana
        app.kubernetes.io/part-of: messagefeed
    spec:
      securityContext:
        fsGroup: 472
      containers:
        - name: grafana
          image: grafana/grafana:12.3.0
          env:
            - name: GF_SECURITY_ADMIN_USER
              value: admin
            - name: GF_SECURITY_ADMIN_PASSWORD
              value: admin
            - name: GF_AUTH_ANONYMOUS_ENABLED
              value: "true"
            - name: GF_AUTH_ANONYMOUS_ORG_ROLE
              value: Admin
            - name: GF_USERS_DEFAULT_THEME
              value: light
          ports:
            - name: http
              containerPort: 3000
          volumeMounts:
            - name: data
              mountPath: /var/lib/grafana
            - name: datasources
              mountPath: /etc/grafana/provisioning/datasources/datasources.yml
              subPath: datasources.yml
              readOnly: true
            - name: dashboards-provider
              mountPath: /etc/grafana/provisioning/dashboards/dashboards.yml
              subPath: dashboards.yml
              readOnly: true
            - name: dashboards
              mountPath: /var/lib/grafana/dashboards
              readOnly: true
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: grafana-data
        - name: datasources
          configMap:
            name: messagefeed-grafana-datasources
        - name: dashboards-provider
          configMap:
            name: messagefeed-grafana-dashboards-provider
        - name: dashboards
          configMap:
            name: messagefeed-grafana-dashboards
---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: ${NS}
spec:
  selector:
    app.kubernetes.io/name: grafana
  ports:
    - name: http
      port: 3000
      targetPort: http
EOF

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: promtail
  namespace: ${NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: messagefeed-promtail
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces", "nodes"]
    verbs: ["get", "watch", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: messagefeed-promtail
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: messagefeed-promtail
subjects:
  - kind: ServiceAccount
    name: promtail
    namespace: ${NS}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: messagefeed-promtail-config
  namespace: ${NS}
data:
  promtail.yml: |
    server:
      http_listen_port: 9080
      grpc_listen_port: 0
    positions:
      filename: /run/promtail/positions.yml
    clients:
      - url: http://loki:3100/loki/api/v1/push
    scrape_configs:
      - job_name: kubernetes-pods
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_namespace]
            target_label: namespace
          - source_labels: [__meta_kubernetes_pod_name]
            target_label: pod
          - source_labels: [__meta_kubernetes_pod_container_name]
            target_label: container
          - source_labels: [__meta_kubernetes_pod_uid]
            target_label: __path__
            replacement: /var/log/pods/*\$1/*.log
        pipeline_stages:
          - cri: {}
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: ${NS}
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: promtail
  template:
    metadata:
      labels:
        app.kubernetes.io/name: promtail
        app.kubernetes.io/part-of: messagefeed
    spec:
      serviceAccountName: promtail
      containers:
        - name: promtail
          image: grafana/promtail:3.5.8
          args:
            - -config.file=/etc/promtail/promtail.yml
          volumeMounts:
            - name: config
              mountPath: /etc/promtail/promtail.yml
              subPath: promtail.yml
              readOnly: true
            - name: varlogpods
              mountPath: /var/log/pods
              readOnly: true
            - name: positions
              mountPath: /run/promtail
      volumes:
        - name: config
          configMap:
            name: messagefeed-promtail-config
        - name: varlogpods
          hostPath:
            path: /var/log/pods
        - name: positions
          emptyDir: {}
EOF

kubectl -n "${NS}" rollout status deploy/prometheus --timeout=240s
kubectl -n "${NS}" rollout status deploy/loki --timeout=240s
kubectl -n "${NS}" rollout status deploy/tempo --timeout=240s
kubectl -n "${NS}" rollout status deploy/otel-collector --timeout=240s
kubectl -n "${NS}" rollout status deploy/grafana --timeout=240s
kubectl -n "${NS}" rollout status daemonset/promtail --timeout=240s
kubectl -n "${NS}" get pod,svc,pvc -o wide

# 观测组件就绪后开启应用 tracing，并通过 Recreate 策略重启 all-in-one
kubectl -n "${NS}" patch configmap messagefeed-app-config \
  --type merge \
  -p '{"data":{"OBSERVABILITY_TRACE_ENABLED":"true","OTEL_EXPORTER_OTLP_ENDPOINT":"otel-collector:4317"}}'
kubectl -n "${NS}" rollout restart deploy/messagefeed-all-in-one
kubectl -n "${NS}" rollout status deploy/messagefeed-all-in-one --timeout=240s
```

当前实施步骤反馈：

```text
D12 反馈：
1. Prometheus：
2. Loki：
3. Promtail：
4. Tempo：
5. OpenTelemetry Collector：
6. Grafana：
7. 应用 tracing 是否已开启并完成 all-in-one 重启：
8. 是否可以进入 D13：
```

### D13. 完整链路访问与观测验收

待执行命令：

```bash
# 终端 1：保持 gateway port-forward 运行
kubectl -n "${NS}" port-forward svc/gateway 8443:8443
```

另开一个终端执行：

```bash
# 经 gateway 验证 API 健康检查
curl -kfsS https://127.0.0.1:8443/healthz
curl -kfsS https://127.0.0.1:8443/readyz
curl -kfsS https://127.0.0.1:8443/api/runtime/node
curl -kfsS https://127.0.0.1:8443/metrics | head

# 经 gateway 验证 Web 首页
curl -kfsS https://127.0.0.1:8443/ | head
```

```bash
# 终端 2：保持 API Service port-forward 运行
kubectl -n "${NS}" port-forward svc/api 60001:60001
```

```bash
# 另开终端直接验证 API Service
curl -fsS http://127.0.0.1:60001/readyz
```

观测入口验收：

```bash
# 终端 3：保持 Prometheus port-forward 运行
kubectl -n "${NS}" port-forward svc/prometheus 9090:9090
```

```bash
# 另开终端验证 Prometheus readiness
curl -fsS http://127.0.0.1:9090/-/ready
```

```bash
# 终端 4：保持 Grafana port-forward 运行
kubectl -n "${NS}" port-forward svc/grafana 3000:3000
```

```bash
# 另开终端验证 Grafana health
curl -fsS http://127.0.0.1:3000/api/health
```

```bash
# hostNetwork profile 下 cloudflared 已直接监听节点 2000 端口，不需要 port-forward
curl -fsS http://127.0.0.1:2000/ready
curl -fsS http://127.0.0.1:2000/metrics | head
```

如后续 profile 为普通 podNetwork，再使用：

```bash
kubectl -n "${NS}" port-forward deploy/cloudflared 2000:2000
```

资源状态核查：

```bash
kubectl -n "${NS}" get deploy,statefulset,daemonset,pod,svc,pvc -o wide
kubectl -n "${NS}" get endpoints api web gateway gateway-dev prometheus loki tempo otel-collector grafana -o wide
kubectl -n "${NS}" get events --sort-by=.lastTimestamp | tail -n 80
```

当前实施步骤反馈：

```text
D13 反馈：
1. gateway /healthz：成功，返回 `{"status":"ok"}`。
2. gateway /readyz：成功，数据库、migrations、pgvector、agent observability 等检查均 ready；schema migrations version 37。
3. gateway Web 首页：成功，返回 Web HTML，包含 `messageFeed` 标题与静态资源引用。
4. direct api /readyz：用户新终端执行 `kubectl -n "${NS}" port-forward svc/api 60001:60001` 时因 `NS` 未设置或为空导致查到 default namespace，报 `services "api" not found`；实际 `messagefeed` namespace 中 `service/api` 与 `service/messagefeed-api` 均存在且 endpoints 指向 `10.42.0.63:60001`。后续新终端需先执行 `export NS=messagefeed` 或直接使用 `-n messagefeed`。
5. Prometheus：当前 Deployment/Service 已 Ready，`service/prometheus` 存在，Pod `prometheus-5dff76b769-2x5tb` 为 `1/1 Running`。
6. Grafana：当前 Deployment/Service 已 Ready，`service/grafana` 存在，Pod `grafana-5bd59c4bdf-7p82v` 为 `1/1 Running`。
7. cloudflared readiness 与 metrics：成功；当前 Pod `cloudflared-69c55fbb65-zllvl` 为 `1/1 Running`，profile 为 `hostNetwork + quic`，日志显示多条 `Registered tunnel connection ... protocol=quic`。hostNetwork profile 下无需 `kubectl port-forward deploy/cloudflared 2000:2000`，直接访问 `http://127.0.0.1:2000/ready` 返回 `{"status":200,"readyConnections":4,...}`，`http://127.0.0.1:2000/metrics` 返回 `build_info` 等 Prometheus 指标。`curl: (23) Failure writing output to destination` 由 `| head` 提前关闭输出管道导致，不表示 cloudflared metrics 异常。
8. 全部 Pod restart 次数：当前主要运行 Pod 均为 0 restart；历史失败 migrate Job `messagefeed-migrate-20260707122237-*` 仍保留为 Error 记录，成功 migrate Job `messagefeed-migrate-20260707123604-db6gx` 为 Completed。
9. 当前是否达到“完整过渡链路可用”：本地 gateway 链路、API ready、Web 首页、cloudflared 连接、观测组件资源均已通过当前核查；外部 Cloudflare 域名访问仍需用户侧浏览器或公网 curl 最终确认。
```

### D13.1. 长期打开本地监控访问入口

执行目的：

1. 不再依赖长期占用终端的 `kubectl port-forward svc/prometheus 9090:9090` 与 `kubectl port-forward svc/grafana 3000:3000`。
2. 通过 Kubernetes Service `NodePort` 固定暴露 Prometheus 与 Grafana 的本地访问端口。
3. Pod、Deployment、Service 仍由 Kubernetes 自动维护；终端关闭不会影响监控服务运行。

访问边界：

1. 本步骤只处理本地过渡环境的监控备用入口，不把 Prometheus/Grafana 暴露到 Cloudflare Tunnel。
2. `NodePort` 会监听在节点网络上，暴露范围通常大于 `kubectl port-forward` 的 `127.0.0.1` 临时通道；当前仅用于 WSL/K3s 本地环境。
3. Prometheus 端口固定为 `30909`，Grafana 端口固定为 `30300`，均在 Kubernetes 默认 NodePort 范围 `30000-32767` 内。
4. 如果后续改用 Helm Chart、Kustomize 或正式 YAML 管理监控组件，应把该 Service 类型和端口写入正式清单，否则重新 apply 原始 Service 时可能恢复为 `ClusterIP`。
5. 当前实测 `http://192.168.3.40:30909/` 在 WSL 内可访问，但 Windows 侧不可访问。因此 NodePort 不满足“Windows 固定本机访问”的目标，只能作为 WSL 内部备用入口。
6. 之前 Windows 能访问 `127.0.0.1:9090/3000`，是因为手动运行的 `kubectl port-forward` 绑定了 WSL loopback，WSL localhost forwarding 将其映射到 Windows 侧；这与 NodePort 不是同一条访问路径。

端口说明：

1. `9090` 与 `3000` 仍保留为 Service 的集群内端口，即 `svc/prometheus:9090` 与 `svc/grafana:3000` 不变。
2. `30909` 与 `30300` 是节点外部访问端口，即浏览器从 WSL/宿主机访问时使用的 NodePort。
3. 默认 Kubernetes 不允许 NodePort 直接使用 `9090` 或 `3000`，因为它们不在默认范围 `30000-32767` 内。
4. 若强行希望节点外部也使用 `9090/3000`，需要修改 K3s apiserver 的 `service-node-port-range`，或改用 hostNetwork、本机反向代理、gateway 路由等方案；这会扩大影响面，当前过渡部署不建议在此步骤修改集群全局端口范围。
5. WSL/K3s 下 NodePort 不一定能通过 `127.0.0.1` 访问；更稳定的方式是使用 Kubernetes Node 的 `INTERNAL-IP`。当前实测节点 `INTERNAL-IP=192.168.3.40` 可访问 `30909/30300`，而 `127.0.0.1:30909/30300` 失败。
6. 如果 `127.0.0.1:9090` 或 `127.0.0.1:3000` 仍可访问，通常说明仍有旧的 `kubectl port-forward` 进程存在；这不代表 NodePort 使用了原端口。

WSL 关闭后自动恢复说明：

1. K3s 已作为 systemd 服务安装，WSL 重新启动并进入发行版后，`k3s.service` 会自动启动。
2. PostgreSQL、API all-in-one、Web、gateway、cloudflared、Prometheus、Grafana 等 Kubernetes Workload 会由 Kubernetes 控制器按声明状态自动恢复。
3. PVC 数据、K3s containerd 镜像、Secret、ConfigMap、Service 规格会保留；NodePort 修改成功后也会随 Kubernetes 状态恢复。
4. `kubectl port-forward` 不会自动恢复，因为它只是当前终端进程；这也是 D13.1 改用 NodePort 的原因。
5. cloudflared 当前为 `hostNetwork + quic`，Pod 自动恢复后会重新建立 Tunnel；但如 Windows/WSL 网络、DNS 或 Cloudflare 出站链路异常，仍需按 D14 排查。
6. 验证自动恢复时，重启 WSL 后先执行 `systemctl is-active k3s`、`kubectl -n "${NS}" get pod -o wide`、`curl -fsS http://127.0.0.1:2000/ready` 和本节 NodePort 验收命令。

待执行命令：

```bash
# 固定命名空间，避免新终端默认访问 default namespace
export NS=messagefeed

# 将 Prometheus Service 改为 NodePort，并固定节点端口为 30909
kubectl -n "${NS}" patch svc prometheus --type='merge' -p='{
  "spec": {
    "type": "NodePort",
    "ports": [
      {
        "name": "http",
        "port": 9090,
        "targetPort": 9090,
        "protocol": "TCP",
        "nodePort": 30909
      }
    ]
  }
}'

# 将 Grafana Service 改为 NodePort，并固定节点端口为 30300
kubectl -n "${NS}" patch svc grafana --type='merge' -p='{
  "spec": {
    "type": "NodePort",
    "ports": [
      {
        "name": "http",
        "port": 3000,
        "targetPort": 3000,
        "protocol": "TCP",
        "nodePort": 30300
      }
    ]
  }
}'

# 确认 Service 类型和端口已变更为 NodePort
kubectl -n "${NS}" get svc prometheus grafana -o wide

# 获取当前 Kubernetes 节点 Internal-IP；WSL 重启后该地址可能变化
NODE_IP="$(kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')"
echo "NODE_IP=${NODE_IP}"

# 从本机通过 NodePort 验证 Prometheus readiness
curl -fsS "http://${NODE_IP}:30909/-/ready"

# 从本机通过 NodePort 验证 Grafana health
curl -fsS "http://${NODE_IP}:30300/api/health"
```

浏览器访问地址：

```text
Prometheus: http://<NODE_IP>:30909
Grafana:    http://<NODE_IP>:30300
```

如需确认旧的本地 port-forward 是否仍在运行：

```bash
# 查看是否仍有 kubectl 进程监听本机 9090/3000
ss -ltnp | grep -E ':9090|:3000' || true
```

当前实施步骤反馈：

```text
D13.1 反馈：
1. Prometheus Service 是否已改为 NodePort：是，`9090:30909/TCP`。
2. Grafana Service 是否已改为 NodePort：是，`3000:30300/TCP`。
3. Prometheus 本地长期入口是否可访问：`127.0.0.1:30909` 失败；节点 `INTERNAL-IP=192.168.3.40` 下 `http://192.168.3.40:30909/-/ready` 成功。
4. Grafana 本地长期入口是否可访问：`127.0.0.1:30300` 失败；节点 `INTERNAL-IP=192.168.3.40` 下 `http://192.168.3.40:30300/api/health` 成功。
5. Windows 侧是否可访问 NodePort：否，Windows 访问 `http://192.168.3.40:30909/` 失败；该方案不作为 Windows 固定本机访问方案。
6. 当前是否仍有旧 port-forward 进程：本机核查显示 `kubectl` 仍监听 `127.0.0.1:9090` 与 `127.0.0.1:3000`，因此原端口当前可访问并不代表 NodePort 使用了原端口。
```

### D13.2. 固定 Windows 本机回环监控入口

执行目的：

1. 固定 Windows 浏览器访问地址为 `http://127.0.0.1:9090` 与 `http://127.0.0.1:3000`。
2. 复用此前已验证可用的 `kubectl port-forward` 访问路径。
3. 使用 systemd 托管 port-forward 进程，使 WSL 重启后自动恢复，不再依赖手动保持终端窗口。

设计说明：

1. Kubernetes 仍负责维护 Prometheus/Grafana Pod 与 Service。
2. systemd 仅负责维护 WSL 本机到 Kubernetes Service 的 loopback 转发进程。
3. 该方案更符合当前目标：Windows 固定访问 `localhost`，并保留原端口 `9090/3000`。
4. 如果端口已被旧手动 `kubectl port-forward` 占用，systemd 服务会启动失败或反复重启；执行前应关闭旧 port-forward 终端，或先确认端口未被占用。

待执行命令：

```bash
# 固定命名空间
export NS=messagefeed

# 确认 kubectl 与 kubeconfig 路径
command -v kubectl
ls -l /home/aroen/.kube/config

# 查看是否仍有旧的手动 port-forward 占用 9090/3000
ss -ltnp | grep -E ':9090|:3000' || true
```

若上一步显示 `kubectl` 正在监听 `127.0.0.1:9090` 或 `127.0.0.1:3000`，先在对应终端按 `Ctrl+C` 关闭旧命令，再继续。

```bash
# 创建 Prometheus port-forward systemd 服务
sudo tee /etc/systemd/system/messagefeed-prometheus-port-forward.service >/dev/null <<'EOF'
[Unit]
Description=messageFeed Prometheus kubectl port-forward
Wants=k3s.service
After=k3s.service network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=aroen
Environment=KUBECONFIG=/home/aroen/.kube/config
ExecStartPre=/usr/local/bin/kubectl -n messagefeed wait --for=condition=available deployment/prometheus --timeout=180s
ExecStart=/usr/local/bin/kubectl -n messagefeed port-forward --address 127.0.0.1 svc/prometheus 9090:9090
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 创建 Grafana port-forward systemd 服务
sudo tee /etc/systemd/system/messagefeed-grafana-port-forward.service >/dev/null <<'EOF'
[Unit]
Description=messageFeed Grafana kubectl port-forward
Wants=k3s.service
After=k3s.service network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=aroen
Environment=KUBECONFIG=/home/aroen/.kube/config
ExecStartPre=/usr/local/bin/kubectl -n messagefeed wait --for=condition=available deployment/grafana --timeout=180s
ExecStart=/usr/local/bin/kubectl -n messagefeed port-forward --address 127.0.0.1 svc/grafana 3000:3000
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 重新加载 systemd，并启用自启动
sudo systemctl daemon-reload
sudo systemctl enable --now messagefeed-prometheus-port-forward.service
sudo systemctl enable --now messagefeed-grafana-port-forward.service

# 查看服务状态
systemctl status messagefeed-prometheus-port-forward.service --no-pager
systemctl status messagefeed-grafana-port-forward.service --no-pager

# WSL 内验证固定本机入口
curl -fsS http://127.0.0.1:9090/-/ready
curl -fsS http://127.0.0.1:3000/api/health
```

Windows 侧验证：

```powershell
# 在 Windows PowerShell 中执行
curl.exe -fsS http://127.0.0.1:9090/-/ready
curl.exe -fsS http://127.0.0.1:3000/api/health
```

WSL 重启恢复验证：

```text
1. 在 Windows 侧执行 `wsl --shutdown`。
2. 重新打开 Ubuntu/WSL。
3. 在 WSL 中执行以下命令。
```

```bash
export NS=messagefeed

systemctl is-active k3s
systemctl is-active messagefeed-prometheus-port-forward.service
systemctl is-active messagefeed-grafana-port-forward.service
kubectl -n "${NS}" get pod -o wide
curl -fsS http://127.0.0.1:9090/-/ready
curl -fsS http://127.0.0.1:3000/api/health
```

常见问题：

```bash
# 如果服务未启动，查看日志
journalctl -u messagefeed-prometheus-port-forward.service -n 80 --no-pager
journalctl -u messagefeed-grafana-port-forward.service -n 80 --no-pager

# 如果提示端口占用，查看占用者
ss -ltnp | grep -E ':9090|:3000' || true

# 关闭旧手动 port-forward 后，重启 systemd 托管入口
sudo systemctl restart messagefeed-prometheus-port-forward.service
sudo systemctl restart messagefeed-grafana-port-forward.service
```

当前实施步骤反馈：

```text
D13.2 反馈：
1. 是否已关闭旧手动 port-forward：是，旧 `kubectl` 进程 PID `1089176` 与 `1090064` 已终止，释放 `127.0.0.1:9090/3000`。
2. Prometheus port-forward systemd 服务是否已创建：是，`/etc/systemd/system/messagefeed-prometheus-port-forward.service` 已创建，状态 `enabled`、`active`。
3. Grafana port-forward systemd 服务是否已创建：是，`/etc/systemd/system/messagefeed-grafana-port-forward.service` 已创建，状态 `enabled`、`active`。
4. WSL 内 `127.0.0.1:9090/-/ready` 是否成功：成功，返回 `Prometheus Server is Ready.`。
5. WSL 内 `127.0.0.1:3000/api/health` 是否成功：成功，返回 Grafana health JSON，`database=ok`，`version=12.3.0`。
6. Windows 侧 `127.0.0.1:9090/-/ready` 是否成功：待用户在 Windows PowerShell 或浏览器中确认；当前 WSL 环境未找到可调用的 Windows `curl.exe`。
7. Windows 侧 `127.0.0.1:3000/api/health` 是否成功：待用户在 Windows PowerShell 或浏览器中确认；当前 WSL 环境未找到可调用的 Windows `curl.exe`。
8. WSL 重启后两个 systemd port-forward 服务是否自动恢复：待执行 `wsl --shutdown` 后复验。
```

### D14. 常见问题只读排查命令

待执行命令：

```bash
# 查看所有主要资源
kubectl -n "${NS}" get all,pvc,configmap,secret -o wide

# 查看后端 all-in-one
kubectl -n "${NS}" describe deploy/messagefeed-all-in-one
kubectl -n "${NS}" logs deploy/messagefeed-all-in-one --tail=300

# 查看 PostgreSQL
kubectl -n "${NS}" get statefulset,pod,pvc,svc -l app.kubernetes.io/name=messagefeed-postgres -o wide
kubectl -n "${NS}" logs statefulset/messagefeed-postgres --tail=200

# 查看 Web、gateway、cloudflared
kubectl -n "${NS}" describe deploy/web
kubectl -n "${NS}" describe deploy/gateway
kubectl -n "${NS}" describe deploy/cloudflared
kubectl -n "${NS}" logs deploy/gateway --tail=200
kubectl -n "${NS}" logs deploy/cloudflared --tail=200

# 查看观测组件
kubectl -n "${NS}" logs deploy/prometheus --tail=200
kubectl -n "${NS}" logs deploy/loki --tail=200
kubectl -n "${NS}" logs deploy/tempo --tail=200
kubectl -n "${NS}" logs deploy/otel-collector --tail=200
kubectl -n "${NS}" logs deploy/grafana --tail=200
kubectl -n "${NS}" logs daemonset/promtail --tail=200

# 查看最近事件和 endpoints
kubectl -n "${NS}" get events --sort-by=.lastTimestamp | tail -n 100
kubectl -n "${NS}" get endpoints -o wide
```

常见判断：

1. `ImagePullBackOff`：通常表示本地镜像没有导入 K3s containerd，回到 D3。
2. `CrashLoopBackOff` 且日志出现配置错误：回到 D4 核查 ConfigMap/Secret 是否完整。
3. `pg_restore` 失败：先确认 D-1 备份 sha256 校验通过，再确认目标 K8s 数据库为空。
4. `/readyz` 失败且提示 migrations：回到 D7 查看 migrate Job 日志。
5. gateway 访问 API 失败：确认 `api` Service endpoints 存在，且 Caddyfile.prod 中 `reverse_proxy api:60001` 可解析。
6. gateway 访问 Web 失败：确认 `web` Service endpoints 存在。
7. cloudflared 不 ready：确认 `messagefeed-cloudflared-secret` 存在且 token 非空，确认 Cloudflare Tunnel 远端 public hostname 仍指向 `https://gateway:8443` 或 `https://gateway-dev:8443`。
8. Prometheus 无 API target：确认 `api:60001` Service 可访问，确认 `/metrics` 返回成功。

当前实施步骤反馈：

```text
D14 反馈：
1. 是否触发排查：
2. 问题现象：
3. 关键日志：
4. 初步原因：
5. 处理结果：
```

### 第三部分前置过渡部署通过标准

1. `messagefeed` namespace 存在。
2. D-1 Docker Compose PostgreSQL 备份存在，sha256 校验通过，`pg_restore -l` 可解析。
3. `messagefeed-postgres-0` 为 `Running`，PVC 绑定成功。
4. K8s PostgreSQL 已从 D-1 备份恢复，`schema_migrations=37,false`，pgvector extension 存在。
5. migrate Job 完成，数据库 schema_migrations 不处于 dirty 状态。
6. `messagefeed-all-in-one` 只有 1 个 Pod，且处于 `Running` 和 Ready。
7. `api`、`web`、`gateway`、`gateway-dev` Service 均有 endpoints。
8. `cloudflared` Pod ready，日志中无持续 tunnel 鉴权或 origin 连接错误。
9. Prometheus、Loki、Promtail、Tempo、OpenTelemetry Collector、Grafana 均处于可用状态。
10. 经 gateway 访问 `/healthz`、`/readyz`、`/metrics`、`/api/runtime/node` 成功。
11. 经 gateway 访问 Web 首页成功。
12. 未部署独立 worker Pod，未把 all-in-one Deployment 扩容到 2 个或更多副本。

### 第三部分前置过渡部署暂不执行项

1. 不修改 Go 源码，不实现 `APP_ROLE`。
2. 不创建 Helm Chart，不写入 `deploy/helm` 文件。
3. 不把 all-in-one Deployment 扩容到多副本。
4. 不启动独立 source-worker、notification-worker、agent-scheduler-worker 或 embedding-worker Pod。
5. 不删除 Docker Compose 数据卷、K8s PVC、Secret、ConfigMap、Job、Pod 或 namespace；如需回退或清理，另行记录并确认后再执行。

## 第四部分：all-in-one Helm Chart 与现有 K3s 资源接管

**状态**：已完成。Helm release `messagefeed` revision 2 为 `deployed`，all-in-one 过渡部署已由 Helm 管理。  
**反馈时间**：2026-07-16 17:12 CST  
**执行性质**：Helm Chart 实现、接管前数据保护、StorageClass/PV 回收策略调整、现有 Kubernetes 资源所有权接管、故障修正与完整链路验收。  
**实施边界**：本部分继续保持后端 all-in-one 单副本，不实施 `APP_ROLE`，不扩容后台任务进程，不删除 PVC/PV、Secret、namespace 或数据库数据。现有 Secret 只按名称引用，不把 Secret 明文写入 Chart 或本文档。  
**历史边界说明**：第三部分“暂不创建 Helm Chart”是当时过渡部署阶段的执行边界；本部分由用户明确推进 Helm 化，因此不回改第三部分历史记录，后续状态以本部分为准。

### E1. 建立 all-in-one Helm Chart

实现目标：

1. 将当前 PostgreSQL、后端 all-in-one、Web、Caddy gateway、cloudflared 与观测栈纳入一个 Helm release。
2. 继续复用当前稳定 Service 名称，包括 `api`、`web`、`gateway`、`gateway-dev`、`messagefeed-postgres`、`prometheus`、`loki`、`tempo`、`otel-collector` 和 `grafana`。
3. 后端保持单副本和 `Recreate` 策略，避免后台任务重复执行。
4. 数据库迁移通过后端 Pod 的 init container 在业务进程启动前执行。
5. 现有敏感配置通过 `existingSecret` 引用，不写入 `values-k3s.yaml`。
6. 观测栈保持可选，并打包现有 Prometheus、Loki、Tempo、OpenTelemetry Collector 和 Grafana 配置。

新增 Chart 路径：

```text
deploy/helm/messagefeed/
  Chart.yaml
  values.yaml
  values-k3s.yaml
  values.schema.json
  files/
    migrations/
    observability/
  templates/
    api.yaml
    cloudflared.yaml
    config.yaml
    gateway.yaml
    migrations-configmap.yaml
    observability-config.yaml
    observability-storage.yaml
    observability-workloads.yaml
    postgresql.yaml
    promtail.yaml
    secrets.yaml
    storageclass.yaml
    web.yaml
```

关键实现结论：

1. Chart 版本为 `0.1.0`，应用版本为 `0.2.0`。
2. 74 个 SQL migration 文件已打包进 Chart。
3. `values-k3s.yaml` 继续使用当前镜像：
   - `messagefeed-api:allinone-0703de0`
   - `messagefeed-web:allinone-0703de0`
4. 当前四个敏感配置 Secret 继续由集群外部管理：
   - `messagefeed-app-secret`
   - `messagefeed-postgres-secret`
   - `messagefeed-caddy-certs`
   - `messagefeed-cloudflared-secret`
5. Chart 不创建或接管上述四个既有 Secret，避免 Helm 卸载或配置变更影响敏感资产。
6. `Dockerfile` 已加入 `tini`，镜像 Entrypoint 为 `/sbin/tini -- /app/messagefeed`；本次 K3s 接管仍使用旧镜像 `allinone-0703de0`，因此 `tini` 需要后续构建新镜像并通过 Helm 更新镜像标签后才会在集群实际生效。

验证命令：

```bash
helm lint deploy/helm/messagefeed \
  -f deploy/helm/messagefeed/values-k3s.yaml

helm template messagefeed deploy/helm/messagefeed \
  --namespace messagefeed \
  -f deploy/helm/messagefeed/values-k3s.yaml

helm upgrade --install messagefeed deploy/helm/messagefeed \
  --namespace messagefeed \
  --take-ownership \
  --dry-run=server \
  --hide-secret \
  --timeout 10m \
  -f deploy/helm/messagefeed/values-k3s.yaml
```

反馈：

1. `helm lint` 通过，仅提示 Chart icon 为推荐字段，不影响安装。
2. 完整栈服务端 dry-run 通过。
3. 完整接管配置在不生成 Secret 时渲染 37 个 Kubernetes 资源。
4. Web 端 9 项 Vitest 测试通过，`npm run build` 通过。
5. API 测试镜像 `messagefeed:helm-test` 构建成功，镜像检查确认 Entrypoint 为 `/sbin/tini -- /app/messagefeed`，运行用户为 `appuser`。
6. Go 全量测试存在既有失败：`TestAuthServiceDefaultOwnerMigrationPasswordHash` 的 bcrypt hash 校验失败；该失败与 Helm/Dockerfile 变更无直接调用关系，本次未修改业务测试和认证代码。

代码提交与推送：

```text
commit: 5b77e43
message: deploy: add Helm chart for all-in-one stack
branch: master
remote: origin/master
```

判定：

1. all-in-one 阶段 Helm Chart 已进入主分支。
2. Chart 不包含私钥、Tunnel token、数据库密码、登录密码或模型 API Key 明文。
3. `backups/` 已加入项目 `.gitignore`，接管备份保留在本机，不进入 Git。

### E2. 接管前数据库与存储基线核查

核查命令：

```bash
kubectl -n messagefeed get pod,deploy,statefulset,daemonset -o wide
kubectl -n messagefeed get pvc -o wide
kubectl get pv
kubectl get storageclass

POSTGRES_POD="$(kubectl -n messagefeed get pod \
  -l app.kubernetes.io/name=messagefeed-postgres \
  -o jsonpath='{.items[0].metadata.name}')"

kubectl -n messagefeed exec "${POSTGRES_POD}" -- \
  pg_isready -U messagefeed -d messagefeed

kubectl -n messagefeed exec "${POSTGRES_POD}" -- \
  psql -U messagefeed -d messagefeed -Atc \
  "SELECT version::text || ',' || dirty::text FROM schema_migrations LIMIT 1;"

kubectl -n messagefeed exec "${POSTGRES_POD}" -- \
  psql -U messagefeed -d messagefeed -Atc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';"
```

反馈：

1. 接管前全部 11 个主要 Pod 为 `Running` 和 Ready。
2. PostgreSQL 返回 `accepting connections`。
3. `schema_migrations=37,false`。
4. `public` schema 中 base table 数量为 55。
5. 5 个 PVC 均为 `Bound`：
   - `postgres-data-messagefeed-postgres-0`：10 Gi
   - `prometheus-data`：5 Gi
   - `loki-data`：5 Gi
   - `tempo-data`：5 Gi
   - `grafana-data`：5 Gi
6. 接管前 5 个对应 PV 的回收策略均为 `Delete`。
7. 接管前默认 StorageClass 为 `local-path`，其回收策略为 `Delete`。

判定：

1. Kubernetes 工作负载和数据库基线正常，可以进入数据备份和回收策略修改。
2. 在 PV 回收策略仍为 `Delete` 时不执行 Helm 接管或 Helm uninstall。

### E3. 生成并校验 Helm 接管前 PostgreSQL 备份

执行命令：

```bash
mkdir -p backups/k8s-adoption

TS="$(date +%Y%m%d-%H%M%S)"
BACKUP="backups/k8s-adoption/messagefeed-before-helm-${TS}.dump"
POSTGRES_POD="$(kubectl -n messagefeed get pod \
  -l app.kubernetes.io/name=messagefeed-postgres \
  -o jsonpath='{.items[0].metadata.name}')"

kubectl -n messagefeed exec "${POSTGRES_POD}" -- \
  pg_dump -U messagefeed -d messagefeed -Fc > "${BACKUP}"

sha256sum "${BACKUP}" > "${BACKUP}.sha256"

kubectl -n messagefeed exec -i "${POSTGRES_POD}" -- \
  pg_restore -l < "${BACKUP}" > /tmp/messagefeed-pg-restore-list.txt

sha256sum -c "${BACKUP}.sha256"
```

反馈：

1. 备份文件：`backups/k8s-adoption/messagefeed-before-helm-20260716-164141.dump`。
2. 文件大小：7.6 MiB。
3. SHA-256 校验通过。
4. 宿主机未安装 `pg_restore`，因此改为使用 PostgreSQL Pod 内的 `pg_restore -l` 读取归档；该操作只读取备份，不写入数据库。
5. 归档包含 685 个 TOC entries，目录输出 696 行。
6. 备份来源数据库版本和 `pg_dump` 版本均为 PostgreSQL 15.18。

判定：

1. 接管前逻辑备份可解析，具备后续恢复输入条件。
2. 备份和校验文件保留在本机项目 `backups/k8s-adoption/` 下，不提交到 Git。

### E4. 创建 Retain StorageClass 并修改现有 PV 回收策略

设计原则：

1. PVC 本身没有 `Retain/Delete` 回收策略，该字段位于 PV 和 StorageClass。
2. 已有 PVC 的 `storageClassName` 不可原位修改，因此现有 PVC 继续使用 `local-path`。
3. 新建默认 StorageClass `local-path-retain`，供后续新 PVC 使用。
4. 先修改现有 PV 为 `Retain`，再执行 Helm 接管。

执行命令：

```bash
helm template messagefeed ./deploy/helm/messagefeed \
  -n messagefeed \
  -f ./deploy/helm/messagefeed/values-k3s.yaml \
  --show-only templates/storageclass.yaml | kubectl apply -f -

kubectl patch storageclass local-path --type=merge \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'

kubectl -n messagefeed get pvc \
  -o jsonpath='{range .items[*]}{.spec.volumeName}{"\n"}{end}' | \
xargs -r -n1 kubectl patch pv --type=merge \
  -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'
```

执行结果：

```text
StorageClass:
local-path          reclaimPolicy=Delete  default=false
local-path-retain   reclaimPolicy=Retain  default=true

PersistentVolume:
pvc-07160768-bfbd-4331-86a6-bf4cf062b9fb  Retain  Bound  messagefeed/loki-data
pvc-71d8faac-a083-4dd3-8bf2-ac9d83227106  Retain  Bound  messagefeed/postgres-data-messagefeed-postgres-0
pvc-a90e111c-08b9-4a49-9bc8-93c2cb960d9c  Retain  Bound  messagefeed/prometheus-data
pvc-d1f44b89-7913-4011-975d-80cd711beff5  Retain  Bound  messagefeed/tempo-data
pvc-ff6ede65-22b7-494b-9304-a322cfb0bbec  Retain  Bound  messagefeed/grafana-data
```

补充保护：

1. `local-path-retain` 在 Chart 中包含 `helm.sh/resource-policy: keep`。
2. Prometheus、Loki、Tempo、Grafana 独立 PVC 模板包含 `helm.sh/resource-policy: keep`。
3. PostgreSQL PVC 由 StatefulSet `volumeClaimTemplates` 创建，不作为独立 Helm 资源直接删除；对应 PV 已额外改为 `Retain`。

判定：

1. 现有 PVC/PV 名称、UID、容量和绑定关系未变化。
2. 删除 PVC 后 PV 将进入 `Released` 而不是自动删除底层数据；重新绑定仍需人工处理。
3. `Retain` 不是数据库备份替代方案，因此保留 E3 逻辑备份。

### E5. 第一次 Helm 接管失败与原因修正

第一次实际执行命令：

```bash
helm upgrade --install messagefeed ./deploy/helm/messagefeed \
  -n messagefeed \
  -f ./deploy/helm/messagefeed/values-k3s.yaml \
  --take-ownership \
  --wait \
  --timeout 10m
```

本次首次接管明确未使用：

```text
--atomic
--force
--cleanup-on-fail
```

失败信息：

```text
Error: cannot patch "messagefeed-postgres" with kind StatefulSet:
StatefulSet.apps "messagefeed-postgres" is invalid:
spec: Forbidden: updates to statefulset spec for fields other than
'replicas', 'ordinals', 'template', 'updateStrategy',
'revisionHistoryLimit', 'persistentVolumeClaimRetentionPolicy'
and 'minReadySeconds' are forbidden
```

原因分析：

1. 初版 Chart 在 PostgreSQL `volumeClaimTemplates.metadata` 中新增了 `helm.sh/resource-policy: keep` 注解。
2. 对既有 StatefulSet 而言，`volumeClaimTemplates` 属于不可变字段，即使只新增 metadata 注解也不能原位更新。
3. 第一次安装在 PostgreSQL StatefulSet patch 阶段失败，Helm revision 1 状态为 `failed`。
4. 由于未使用 `--atomic`、`--force` 或资源删除操作，PostgreSQL PVC/PV 没有被删除或重建，数据库 Pod仍挂载原 PVC。

失败后的附带现象：

1. Helm 在 StatefulSet patch 失败前已经更新部分 Deployment Pod template。
2. Prometheus、Loki、Tempo 在默认 `RollingUpdate` 下短时间同时存在新旧 Pod，并并发挂载同一节点上的 RWO PVC。
3. Prometheus 新 Pod 日志显示 TSDB lock 冲突并退出。
4. Loki 新 Pod曾出现 `init compactor: failed to init delete store: timeout`，旧 Pod退出后恢复。
5. Prometheus 和 Loki 在该窗口各累计 5 次 restart；后续 30 秒稳定性复查中 restart 数不再增长。

Chart 修正：

1. 移除 PostgreSQL `volumeClaimTemplates` 中新增的 Helm keep 注解，保持不可变字段与现有 StatefulSet 一致。
2. 将以下有状态单写 Deployment 更新策略改为 `Recreate`：
   - Prometheus
   - Loki
   - Tempo
   - Grafana
3. OpenTelemetry Collector 保持无状态 Deployment 更新方式。
4. 修正后重新执行 `helm lint` 和服务端 upgrade dry-run，结果通过，dry-run 状态为 `pending-upgrade`、revision 2。

判定：

1. 第一次失败属于 Kubernetes 不可变字段与持久卷并发写入策略问题，不是数据库数据丢失。
2. 后续有状态单实例组件必须使用 `Recreate`，不能在同一 RWO 数据目录上并发启动新旧写实例。

### E6. 第二次 Helm 接管与 release 状态

修正后执行命令：

```bash
helm upgrade --install messagefeed ./deploy/helm/messagefeed \
  -n messagefeed \
  -f ./deploy/helm/messagefeed/values-k3s.yaml \
  --take-ownership \
  --wait \
  --timeout 10m
```

Helm history：

```text
REVISION  STATUS      DESCRIPTION
1         superseded  StatefulSet immutable field patch failed
2         deployed    Upgrade complete
```

最终 release：

```text
NAME: messagefeed
NAMESPACE: messagefeed
STATUS: deployed
REVISION: 2
CHART: messagefeed-0.1.0
APP VERSION: 0.2.0
```

Helm 管理边界：

1. Deployment、StatefulSet、DaemonSet、Service、应用 ConfigMap、观测 ConfigMap、独立观测 PVC 和 `local-path-retain` 已带 Helm ownership metadata。
2. PostgreSQL StatefulSet 已由 Helm 管理；其生成的 PostgreSQL PVC 不作为独立 Helm manifest 管理。
3. 四个既有敏感 Secret 没有 Helm ownership metadata，继续独立保留。
4. Prometheus/Grafana 既有 NodePort 规格在接管后仍保留：
   - Prometheus：`9090:30909/TCP`
   - Grafana：`3000:30300/TCP`
5. gateway 仍为 ClusterIP，宿主机 `127.0.0.1:8443` 没有固定监听；gateway 通过集群内 Service 和 Cloudflare Tunnel 对外提供访问。

判定：

1. 当前 all-in-one 过渡栈已由单一 Helm release 管理。
2. 后续升级可使用 `helm upgrade`；首次接管完成后，如已完成备份和 dry-run，后续常规升级可以评估使用 `--atomic`。
3. 不应执行 `helm uninstall messagefeed` 作为普通回退方式；如需卸载，必须先核查 Helm keep 资源、StatefulSet PVC 保留行为和现有 PV `Retain` 状态。

### E7. 接管后数据库、存储与服务验收

数据库验收：

```text
PostgreSQL readiness: accepting connections
schema_migrations: 37,false
public base tables: 55
pgvector: 0.8.4
API migrate init container: Completed, exit=0
```

API `/readyz` 检查：

1. process：ready。
2. database：ready。
3. migrations：version 37，ready。
4. pgvector：version 0.8.4，ready。
5. agent fact index：`rows=1803`、`embeddings=22`。
6. agent observability：trace、recall、embedding trace 与 memory 相关检查返回 ready。

存储验收：

1. 5 个 PVC 均保持原名称、容量和 PV 绑定，状态为 `Bound`。
2. PostgreSQL 继续使用 `postgres-data-messagefeed-postgres-0`，容量 10 Gi。
3. 5 个 PV 均为 `Retain` 和 `Bound`。
4. 接管验收时记录 `local-path-retain` 为唯一默认 StorageClass；2026-07-17 复核发现两个 StorageClass 当前均为默认类，详见 E9。

内部服务端点验收：

```text
http://api:60001/readyz             200
http://web:8080/                    200
http://prometheus:9090/-/ready      200
http://loki:3100/ready              200
http://tempo:3200/ready             200
http://otel-collector:8888/metrics  200
http://grafana:3000/api/health      200
https://gateway:8443/healthz        200，返回 {"status":"ok"}
```

Cloudflare Tunnel 与公网验收：

```text
cloudflared protocol: http2
cloudflared readyConnections: 3
https://aroen.eu.cc/healthz: HTTP 200
response: {"status":"ok"}
```

cloudflared 补充观察：

1. 启动日志显示 `Initial protocol http2`。
2. 已注册多条 `Registered tunnel connection ... protocol=http2`。
3. `connIndex=1` 仍偶发 `already connected to this server, trying another address`、retry 和 connection terminated。
4. `/ready` 仍返回 3 个 ready connections，公网健康检查持续 HTTP 200，因此本次不将该单连接索引重试判定为接管失败。

接管窗口内 API 日志：

1. PostgreSQL Pod因 Helm 更新 Pod template 重启时，API 短暂记录 `SQLSTATE 57P01` 和 DNS lookup timeout。
2. PostgreSQL恢复后，API `/readyz` 重新全部 Ready。
3. 接管完成后最近 90 秒日志未出现新的 error、panic、fatal 或 dirty migration。

最终 Pod 状态：

```text
cloudflared                 1/1 Running
gateway                     1/1 Running
grafana                     1/1 Running
loki                        1/1 Running
messagefeed-all-in-one      1/1 Running
messagefeed-postgres        1/1 Running
otel-collector              1/1 Running
prometheus                  1/1 Running
promtail                    1/1 Running
tempo                       1/1 Running
web                         1/1 Running
```

判定：

1. 接管前后数据库迁移版本和表数量一致，没有发现数据遗失证据。
2. PVC/PV 绑定关系未变化，PostgreSQL、Prometheus、Loki、Tempo 和 Grafana 均从原卷恢复。
3. 完整内部链路和公网入口均通过。

### E8. systemd port-forward 接管后恢复观察

背景：

1. Prometheus 与 Grafana 的固定 Windows 回环入口由 D13.2 systemd 服务维护。
2. Helm 接管替换 Prometheus/Grafana Pod 后，原 port-forward 进程仍短暂引用旧 Pod sandbox。

现象：

```text
failed to find sandbox ... in store: not found
error: lost connection to pod
```

恢复结果：

1. 两个 systemd unit 均配置 `Restart=always`。
2. 旧 Pod sandbox 消失后，服务自动失败并由 systemd 在 5 秒后重启。
3. 新进程重新绑定当前 Prometheus/Grafana Pod。
4. 恢复后验证：

```text
http://127.0.0.1:9090/-/ready  -> Prometheus Server is Ready.
http://127.0.0.1:3000/api/health -> database=ok, version=12.3.0
```

判定：

1. systemd 自动恢复机制有效，不需要人工长期维护 port-forward 终端。
2. Helm 替换目标 Pod 时可能出现一次连接失败；systemd 重启周期当前为 5 秒。
3. Windows 侧固定入口仍需在 Windows 浏览器或 PowerShell 中按 D13.2 最终确认。

### 第四部分通过标准

1. `deploy/helm/messagefeed` Chart 已提交并推送。
2. Helm lint 和服务端 dry-run 通过。
3. 接管前 PostgreSQL custom dump、SHA-256 和 `pg_restore -l` 校验通过。
4. 现有 5 个 PV 均为 `Retain`。
5. `local-path-retain` 应为唯一默认 StorageClass；2026-07-17 复核发现该条件尚未满足。
6. Helm release `messagefeed` revision 2 为 `deployed`。
7. PostgreSQL迁移版本为 `37,false`，公共表数量为 55。
8. 5 个 PVC 均保持 `Bound`，原 PV 绑定关系不变。
9. 后端 all-in-one 仍为单副本。
10. 全部主要 Pod Ready，内部服务端点均返回 HTTP 200。
11. cloudflared Ready，公网 `/healthz` 返回 HTTP 200。
12. Prometheus/Grafana systemd port-forward 可在 Pod 替换后自动恢复。

### E9. 2026-07-17 当前状态复核与文档一致性修正

本次性质：只读核查和文档同步，不修改 Kubernetes 资源，不删除 PVC/PV、Secret、Pod 或 namespace。

核查命令：

```bash
kubectl get storageclass
kubectl get storageclass -o yaml
kubectl -n messagefeed get pvc \
  -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,VOLUME:.spec.volumeName,STORAGECLASS:.spec.storageClassName
kubectl get pv \
  -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,RECLAIM:.spec.persistentVolumeReclaimPolicy
helm status messagefeed -n messagefeed --show-desc
```

当前结果：

```text
local-path          (default)  Delete  rancher.io/local-path
local-path-retain   (default)  Retain  rancher.io/local-path

PVC：5 个均为 Bound，现有 PVC 仍使用 local-path。
PV：5 个均为 Bound，回收策略均为 Retain。
Helm：messagefeed revision 2，状态 deployed。
```

判定：

1. 当前集群存在两个默认 StorageClass，不满足“唯一默认 StorageClass”要求。
2. 现有 PVC/PV 仍为 `Bound`，PV 回收策略仍为 `Retain`，本次未发现数据丢失证据。
3. E4/E7 中的命令和结果属于历史执行记录予以保留；当前状态以后续 E9 复核结果为准。
4. 后续应先将 `local-path` 的默认注解设为 `false`，再核验只有 `local-path-retain` 为默认类；完成前不创建依赖默认 StorageClass 的新 PVC。
5. `values-k3s.yaml` 当前仍将 cloudflared 镜像 tag 覆盖为 `latest`，后续应固定为经过验证的版本或 digest。

### E10. 2026-07-18 第八部分环境与资产治理

本次执行范围：StorageClass 唯一默认类核验、生产镜像版本固定、Grafana 管理凭据 Secret 化、包含 `tini` 的 API 镜像发布、PostgreSQL 完整恢复演练和发布后健康检查。现有 PVC/PV 未迁移、未重建。

StorageClass 与持久卷结果：

```text
local-path          default=false  reclaimPolicy=Delete
local-path-retain   default=true   reclaimPolicy=Retain

PVC：5 个均为 Bound，继续使用原 local-path StorageClass。
PV：5 个均为 Bound，回收策略均为 Retain。
```

镜像与 Secret 治理结果：

1. 构建并导入 `messagefeed-api:ff96bb37781e`，镜像入口为 `/sbin/tini -- /app/messagefeed`，运行用户为 UID/GID 1000。
2. Helm Chart 默认值与 K3s 覆盖值不再使用 `latest`；values schema 会拒绝 API、Web 或 cloudflared 的 `latest` tag。
3. cloudflared 固定为 `2026.6.1`，运行 digest 为 `sha256:6d91c121b803126f7a5344005d17a9324788fc09d305b6e2560ec6040a7ae283`。
4. 创建 `messagefeed-grafana-secret`，Grafana Deployment 通过 `secretKeyRef` 读取管理员账号和随机密码；持久化管理员密码已使用 Grafana CLI 轮换，管理 API 返回 HTTP 200。
5. 应用、PostgreSQL、Caddy 证书和 Cloudflare Tunnel 继续使用原有 `existingSecret`，values 文件未新增敏感明文。

PostgreSQL 恢复演练输入：

```text
备份文件：backups/k8s-adoption/messagefeed-restore-drill-20260718-144227.dump
备份格式：pg_dump custom，--no-owner --no-privileges
文件大小：8280530 bytes
SHA-256：b5efce9d2718c8e43c9ce2e151e1c4e6ee88affa05390bb08a53aeb4faf13277
归档目录：696 行，可由 pg_restore -l 解析
恢复目标：messagefeed_restore_drill_20260718
```

恢复结果：

```text
pg_restore --exit-on-error：成功
schema_migrations：37,false
pgvector：0.8.4
public base table count：55
users：4
sources：145
items：7933
user_item_states：8
source_catalog_entries：47
agent_audit_logs：28609
items(source_id, normalized_url) 重复组：0
uq_items_source_normalized_url：unique=true, valid=true, ready=true
未验证约束：0
恢复库大小：53 MB
```

恢复库核心计数与备份前快照完全一致。根据资产保留约束，恢复库未删除；已设置 `ALLOW_CONNECTIONS=false`，避免应用误连。Pod 内备份副本保留在 `/tmp/messagefeed-restore-drill-20260718.dump`，正式备份和校验文件保留在项目忽略目录 `backups/k8s-adoption/`。

发布与健康验收：

1. `helm lint`、`helm template` 和服务端 dry-run 通过；反向校验确认 `cloudflared.image.tag=latest` 被 schema 拒绝。
2. `helm upgrade --atomic --wait` 成功，release `messagefeed` revision 3 为 `deployed`。
3. API 为单副本且 PID 1 为 `tini`；cloudflared、Grafana 和全部主要工作负载均为 Ready。
4. 公网 `https://aroen.eu.cc/healthz` 与 `https://aroen.eu.cc/readyz` 均返回 HTTP 200。
5. 生产库仍为迁移状态 `37,false`，恢复演练未修改生产数据库名称或应用连接配置。

判定：第八部分“环境与资产治理”全部完成并通过验收，可以进入第九部分“应用运行边界拆分”。

### 后续事项

1. 实施 `APP_ROLE`：`api`、`source-worker`、`notification-worker`、`agent-scheduler-worker`、`embedding-worker` 和 `migrate`。
2. 将数据库迁移从 API init container 调整为独立 migrate Job。
3. 补充 ServiceAccount、最小 RBAC、NetworkPolicy、资源限制和故障预算。
4. 建立 CI lint、template、镜像构建、K3s smoke test、升级观察和回滚闭环。
5. `APP_ROLE` 和 worker 幂等验证完成前，`messagefeed-all-in-one` 必须保持单副本。


### E11. 2026-07-18 第九部分应用运行边界拆分

本次执行范围：将单二进制后端切换为 `APP_ROLE` 多运行角色，拆出 API、四类 worker 和独立 migrate Job；验证启动装配、端口隔离、日志与指标、claim 并发、优雅退出、独立扩缩容和 Helm rollback。未修改生产数据库业务数据，未迁移或重建现有 PVC/PV。

#### E11.1 代码实现与自动化验证

运行角色：

```text
all
api
source-worker
notification-worker
agent-scheduler-worker
embedding-worker
migrate
```

代码边界：

1. `cmd/api/main.go` 仅负责配置加载、日志/tracing、信号上下文和 `internal/bootstrap.Application` 生命周期。
2. `internal/bootstrap` 负责角色计划、数据库与 service 装配、worker loop、运维 HTTP 和受控关闭。
3. `DEPLOYMENT_MODE=cluster` 下 `APP_ROLE=all` 默认拒绝；迁移路径必须是相对路径 `migrations`。
4. worker 运维端点为 `9090/healthz`、`9090/readyz` 和 `9090/metrics`；业务端口 `60001` 不由 worker 监听。
5. Dockerfile 复用 `migrate/migrate:v4.19.1` CLI，运行用户为 UID/GID 1000，入口为 `/sbin/tini -- /app/messagefeed`。

自动化验证：

```text
go test ./...                         PASS
go test -race -count=1 ./internal/bootstrap ./internal/config ./cmd/api PASS
go vet ./...                          PASS
go build ./cmd/api                    PASS
helm lint                             PASS
helm template                         PASS
kubectl apply --dry-run=client        PASS
schema 反向校验：latest、副本数 0、非法角色、非法迁移路径均被拒绝
```

#### E11.2 镜像与 Helm 发布

```text
镜像：messagefeed-api:role9-20260718-8a454cb690ec
Helm Chart：messagefeed-0.2.0
Helm upgrade：--atomic --wait --wait-for-jobs
最终 release：messagefeed revision 7，STATUS=deployed
```

最终工作负载：

```text
messagefeed-api          APP_ROLE=api                    60001/TCP
source-worker            APP_ROLE=source-worker          9090/TCP
notification-worker      APP_ROLE=notification-worker    9090/TCP
agent-scheduler-worker   APP_ROLE=agent-scheduler-worker 9090/TCP
embedding-worker         APP_ROLE=embedding-worker       9090/TCP
messagefeed-migrate      APP_ROLE=migrate                Job Complete
```

迁移验收：

```text
Job messagefeed-migrate：Complete，1/1
日志：migration role starting -> migration role completed
生产 schema_migrations：37,false
pgvector：0.8.4
public 基础表：55
```

API init container 已移除，迁移不再与 API Pod 启动耦合。

#### E11.3 运行隔离与可观测性

1. API `/healthz`、`/readyz` 返回 200，日志包含 `app_role=api`，未出现 worker loop/tick；API 访问本地 `9090` 被拒绝。
2. 四个 worker 的 `/healthz`、`/readyz`、`/metrics` 均成功，访问本地 `60001` 均被拒绝。
3. API 与四个 worker 的 Prometheus target 全部 `up`，target 标签分别为 `api`、`source-worker`、`notification-worker`、`agent-scheduler-worker`、`embedding-worker`。
4. 五个业务 Pod 的 PID 1 均为 `/sbin/tini -- /app/messagefeed`；容器运行用户为 UID 1000。
5. `APP_NODE_ID` 使用 Pod 名称，日志的 `node_id` 在不同 Pod 间可区分。

端到端检查：

```text
https://aroen.eu.cc/healthz  -> HTTP 200
https://aroen.eu.cc/readyz   -> HTTP 200
gateway -> API /healthz      -> {"status":"ok"}
gateway -> Web               -> HTML 200
```

#### E11.4 claim 并发验收

为避免污染生产队列，创建隔离数据库 `messagefeed_role9_acceptance_20260718`，从生产库逻辑复制后在四张真实队列表中各准备 40 条验收任务。两条并发 claimant 使用与 repository 一致的 `FOR UPDATE SKIP LOCKED` 或原子更新语义，并在持锁期间引入短暂等待以形成真实竞争。

结果：

| 队列 | claimant A | claimant B | 总 ID | 重复 claim | 未 claim |
| --- | ---: | ---: | ---: | ---: | ---: |
| source | 20 | 20 | 40 | 0 | 0 |
| notification | 20 | 20 | 40 | 0 | 0 |
| scheduler | 20 | 20 | 40 | 0 | 0 |
| embedding | 20 | 20 | 40 | 0 | 0 |

source、notification、scheduler 三类任务的 `attempt_count` 均为 1；embedding 队列的 40 个任务均由 pending 原子转为 running。未发现重复处理证据。验收数据库已设置 `ALLOW_CONNECTIONS=false`，生产库未插入验收任务。

#### E11.5 优雅退出、扩缩容与回滚

优雅退出：

1. 对 source worker PID 1 发送 SIGTERM。
2. 容器重启计数从 0 增至 1，`--previous` 日志包含 `worker loop stopped`、`application role stopping` 和 `application role stopped`。
3. 日志未出现 error/panic，容器重启后 `/readyz` 恢复为 200。

扩缩容与回滚：

1. revision 5 将 API 与 source worker 独立扩展为 2 副本，其他 worker 保持 1 副本；五个 messagefeed target 仍为 `up`。
2. `helm rollback messagefeed 4 --wait --wait-for-jobs` 成功生成 revision 6，并恢复 API/source worker 各 1 副本。
3. 最终 revision 7 使用 `helm upgrade --atomic --wait --wait-for-jobs` 固化各角色 1 副本声明值和最新模板标签。

#### E11.6 第九部分判定

1. API、各类 worker 和 migrate 可独立启动、停止、日志记录、指标采集和扩缩容。
2. API 与 worker 业务端口隔离，worker 只提供运维端点。
3. 四类 claim 在并发竞争下未出现重复 ID、遗漏或异常 attempt 增长。
4. 独立 migrate Job、tini PID 1、SIGTERM 优雅退出、Prometheus target 和公网健康检查均通过。

判定：第九部分“应用运行边界拆分”完成，可以进入第十部分“Kubernetes 安全与资源治理”。

### E12. 下一节实施内容：Kubernetes 安全与资源治理

下一节仅处理 Kubernetes 权限、网络和资源边界，不改变本节已经验收的角色职责：

1. 为 API、四类 worker 和 migrate 配置独立 ServiceAccount，并以最小 RBAC 替代默认权限。
2. 建立 namespace 默认拒绝 NetworkPolicy，只放行 DNS、PostgreSQL、OTel、gateway 及角色所需外部依赖。
3. 校准 requests/limits，补充 ResourceQuota、LimitRange、PDB 和节点调度约束。
4. 对拒绝访问、资源不足、Pod 驱逐和滚动更新执行可观测故障验收。

### E13. 2026-07-18 第十部分 Kubernetes 安全与资源治理

本次执行范围：为 API、四类 worker 和 migrate 建立独立运行身份；在 `messagefeed` 命名空间启用默认拒绝网络边界；增加资源配额、容器默认边界、PDB 和节点调度约束；执行权限、网络、资源、驱逐、扩缩容和不可调度故障验收。未迁移或重建 PVC/PV，未修改生产数据库业务数据。

#### E13.1 ServiceAccount 与 RBAC

新增六个独立身份：

```text
messagefeed-api
messagefeed-source-worker
messagefeed-notification-worker
messagefeed-agent-scheduler-worker
messagefeed-embedding-worker
messagefeed-migrate
```

每个身份的 Role 均为 `rules=[]`，ServiceAccount 和 Pod 均设置 `automountServiceAccountToken=false`。运行态 `kubectl auth can-i` 验证读取 Pod、Secret、ConfigMap 和创建 Deployment 均返回 `no`；五个业务 Pod 内未挂载 token。Promtail 继续使用独立 ClusterRole，只能读取 Pod、Namespace 和 Node 元数据，读取 Secret 返回 `no`。

migrate 身份使用 `pre-install,pre-upgrade` hook，权重为 `-20`；迁移 Job 权重为 `-10`。Job 增加 `wait-for-postgres` initContainer，并将 `backoffLimit` 调整为 3。

#### E13.2 NetworkPolicy

最终部署 19 条 NetworkPolicy：

1. `messagefeed-default-deny` 默认拒绝所有 ingress/egress，`messagefeed-allow-dns` 只放行 CoreDNS TCP/UDP 53。
2. API、worker、migrate 只按角色访问 PostgreSQL；API/worker 可访问 OTel。
3. gateway 只访问 API/Web；API ingress 只接受 gateway/Prometheus；worker 9090 只接受 Prometheus。
4. Prometheus、Grafana、Loki、Tempo、OTel 和 Promtail 按实际观测调用关系互相放行。
5. 所有应用角色可访问外部 HTTPS 443；只有 API/source worker 可访问 HTTP 80；只有 API 可访问 Windows LLM 入口 15721。

默认拒绝探针结果：

```text
DNS                         allowed
PostgreSQL 5432             denied
API 60001                   denied
external 443                denied
ServiceAccount token        absent
```

角色拒绝结果：

```text
API -> Web 8080             denied
source worker -> API 60001 denied
Web -> PostgreSQL 5432     denied
source worker -> LLM 15721 denied
```

Windows LLM 访问路径经实测固定为：

```text
LLM_BASE_URL=http://198.18.0.1:15721/v1
LLM_MODEL=gpt-5.6-sol
NetworkPolicy=198.18.0.1/32:15721，仅选择 APP_ROLE=api
```

API Pod 请求 `/health` 返回 HTTP 200。最小 completion 请求已到达 Windows 代理，代理返回 HTTP 503，错误为当前分组下 `gpt-5.6-sol` 无可用渠道；网络连接与角色策略均已验证，模型渠道恢复属于外部依赖事项。

cloudflared 继续使用 `hostNetwork=true`。标准 NetworkPolicy 不保证隔离 hostNetwork 流量，该例外已显式记录，后续多节点阶段评估主机防火墙或支持 host policy 的 CNI。

#### E13.3 资源、PDB 与调度

ResourceQuota：

```text
pods=32
requests.cpu=4
requests.memory=6Gi
limits.cpu=24
limits.memory=20Gi
persistentvolumeclaims=10
requests.storage=50Gi
```

LimitRange：默认 request 为 `50m/64Mi`，默认 limit 为 `500m/512Mi`；单容器最小值为 `5m/16Mi`，最大值为 `2 CPU/2Gi`。全部运行容器已有显式 requests/limits。

API、四类 worker、gateway、Web、cloudflared、PostgreSQL 和五个观测组件共 14 个 PDB，统一 `minAvailable=1`。所有工作负载使用节点 `aroen`，并配置 `ScheduleAnyway` topology spread 和权重 50 的 preferred pod anti-affinity。

#### E13.4 故障验收

1. LimitRange 服务端 dry-run 拒绝 3 CPU 单容器，错误明确指出最大值为 2 CPU。
2. ResourceQuota 服务端 dry-run 拒绝额外 4 CPU requests，错误包含 used、requested 和 limited。
3. API 单副本的 eviction dry-run 返回 `TooManyRequests`；API/source 临时扩展到 `2/2` 后 PDB `disruptionsAllowed=1`，eviction dry-run 返回 201。
4. 双副本期间公网 `/readyz` 保持 HTTP 200，随后 API/source 恢复 `1/1`。
5. 使用不存在节点标签的探针保持 Pending，事件为 `FailedScheduling`，原因是 node affinity/selector 不匹配。
6. 所有短生命周期验收 Pod 均已清理。

#### E13.5 发布故障与修正

1. revision 8 首次升级因 migrate Job 早于普通 ServiceAccount 创建而失败，atomic 自动生成 revision 9 回滚到 revision 7。
2. revision 10 暴露初始 quota 未覆盖滚动峰值，PostgreSQL 重建一度被 admission 拒绝；将预算修正为 CPU 24、内存 20Gi 后数据库和应用恢复。
3. revision 11 的 migrate Pod 在新网络规则传播窗口立即连接数据库并失败，atomic 自动生成 revision 12 回滚；加入数据库就绪 initContainer 和 `backoffLimit=3` 后 revision 13 成功。
4. revision 14 完成角色化外部 egress；revision 15 验证 `198.18.0.2` 只建立 TCP 而不返回 HTTP；revision 16 固定为可返回 HTTP 的 `198.18.0.1`。

上述失败均由 `--atomic --wait --wait-for-jobs` 控制，最终未遗留 pending release，数据库仍为 `schema_migrations=37,false`。

#### E13.6 最终判定

```text
Helm Chart：messagefeed-0.3.0
Helm release：revision 16，STATUS=deployed
运行 Pod：15 个，全部 Ready
migrate Job：Complete，1/1
NetworkPolicy：19
PDB：14
Prometheus target：7 个，全部 up
公网首页、/healthz、/readyz：HTTP 200
PostgreSQL：schema_migrations=37,false，pgvector=0.8.4，public 表=55
```

判定：第十部分“Kubernetes 安全与资源治理”完成，可以进入第十一部分“迁移、高可用与回滚”。

### E14. 下一节实施内容：迁移、高可用与回滚

1. 为迁移补充并发锁、失败恢复和 expand/contract 数据库兼容规范。
2. 将 API、Web、Gateway 和 cloudflared 扩展为多副本，校准 RollingUpdate 与 PDB。
3. 验证新 Pod 未 Ready 时旧 Pod 持续服务、单 Pod 故障、入口故障和实际节点维护边界。
4. 验证应用镜像回滚与数据库状态兼容，明确不可回滚迁移的阻断条件。
5. 保持 WSL 关闭、Windows 关机和本机断网不属于当前单节点可用性承诺。
