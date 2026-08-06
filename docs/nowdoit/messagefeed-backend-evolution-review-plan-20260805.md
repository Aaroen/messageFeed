---
type: technical-note
status: in_progress
p0_status: implemented
p1_status: implemented
p2_status: planned
project: messageFeed
language: go
framework:
  - gin
  - gorm
  - rabbitmq
  - kubernetes
topic:
  - backend
  - reliability
  - middleware
  - deployment
tags:
  - go/backend
  - messagefeed/nowdoit
  - middleware/redis
  - middleware/rabbitmq
  - kubernetes
created: 2026-08-05
updated: 2026-08-06
---

# messageFeed 后端演进与面试专项落地方案

来源：[[4. Go后端开发定制复习]] 的“后续复习具体项目展开”与本轮项目实现核查。

## 问题

最后的实际问题不是“把 Redis、RabbitMQ 和 K3s 都加入项目”，而是把已有 `messageFeed` 经验整理为可验证的后端工程闭环：

```text
现有项目事实 -> 故障与扩展边界 -> 最小实验 -> 验收证据 -> 面试表达
```

需要同时保持两个边界：

1. 主项目已经使用的内容可以作为项目经历回答。
2. Redis、RabbitMQ、双节点 K3s 等专项实践，在完成验证前只能表述为学习、实验或拟议方案。

## 当前事实基线

截至 2026-08-05，项目当前形态如下：

| 项目 | 当前事实 | 直接影响 |
| --- | --- | --- |
| 后端 | 同一 Go 二进制通过 `APP_ROLE` 运行 API、五类 worker 和 migrate | 已有进程级角色拆分，仍不是独立代码库和独立数据边界的微服务 |
| 异步任务 | PostgreSQL 任务表、`FOR UPDATE SKIP LOCKED`、定时轮询 | 当前规模足够，暂不需要 MQ |
| Redis/MQ | 主项目未部署 Redis、RabbitMQ 或 Kafka | 只能通过专项实验补齐理论与实践证据 |
| Agent | Web 请求同步执行，企业微信请求在 API goroutine 中异步执行 | Pod 重启可能中断任务；取消和会话锁依赖单 Pod 内存 |
| K3s | 单节点；API/Web/Gateway 各 2 副本，五类 worker 各 1 副本 | 具备 Pod 级冗余，不具备节点级高可用 |
| PostgreSQL | 单副本、节点本地 RWO 卷 | 数据库和节点是整体可用性的主要单点 |
| 运行数据 | 数据库约 61 MB、活跃连接 7 个、队列压力较低 | 当前优先级是可靠性和可解释性，不是容量扩展 |

已经发现的具体边界：

1. P0 实施前 `item_events` 存在长期 `pending` 记录且没有对应消费角色；现已接入 `item-event-worker`，历史数据仍需按业务语义处理。
2. 抓取 worker 的全局锁覆盖入队、领取和执行整个批次，增加副本后仍会被串行化。
3. 各队列没有统一的租约续期、过期重入和故障恢复协议。
4. Agent 的 `sessionLocks`、活动进程索引和取消信号位于 API 进程内。
5. `ai_analysis_jobs` 只有仓储和状态模型，没有明确的生产与消费闭环；暂不为其增加 speculative worker。

## 方案结论

推荐采用以下顺序：

```text
P0：修复 PostgreSQL 任务可靠性
  -> P1：把 Agent 执行改为持久化任务 + 独立 worker
  -> P2：主服务节点水平拓展 + GPU 节点保底推理
```

主项目继续使用 PostgreSQL 作为事实来源和任务存储。本阶段只实施双节点定向部署、资源隔离和安全控制，不增加其他专项范围。

## P0：任务可靠性闭环

### 目标

让 worker 在正常完成、超时、进程重启、重复领取和旧 worker 恢复五种情况下都能得到可解释结果。

### 当前实现

截至 2026-08-05，P0 已完成以下代码闭环：

1. `source_fetch_jobs`、`notification_jobs`、`ai_analysis_jobs`、`agent_scheduled_tasks`、`agent_fact_index_jobs` 和 `item_events` 均支持租约字段；领取使用 `FOR UPDATE SKIP LOCKED`，领取前回收过期任务。
2. 任务达到 `max_attempts` 后进入失败终态；未达到上限时按重试时间重新入队，并清除旧 worker 的锁和租约。
3. 任务完成更新支持 owner 条件，旧 worker 在租约失效后不能覆盖新 worker 的状态。
4. Source Worker 的分布式锁仅覆盖到期来源扫描和入队，领取及抓取执行位于锁外。
5. `item_events` 已接入独立 `item-event-worker` 角色、告警规则处理器、启动计划、Helm Deployment、RBAC、NetworkPolicy 和指标抓取。
6. 统一提供队列深度、最老任务年龄、领取耗时、重试、租约回收和死信指标。

未完成项：数据库中的历史 `item_events` pending 记录尚未在当前环境执行业务清理；Redis、RabbitMQ 和双节点 K3s 仍属于后续专项实验，不计入 P0 已落地能力。

验证记录：`go test ./...`、`go vet ./...` 和 `helm lint deploy/helm/messagefeed -f deploy/helm/messagefeed/values-k3s.yaml` 已通过。`go test -race ./...` 仍会命中既有 Agent 异步测试替身的数据竞争，未将其归因于本次队列改动。

### 实施顺序

1. 盘点 `source_fetch_jobs`、`notification_jobs`、`agent_scheduled_tasks`、`agent_fact_index_jobs` 和 `item_events` 的生产者、消费者和终态。
2. 为领取任务统一定义 `locked_by`、`locked_at`、租约截止时间、尝试次数、最后错误和更新时间语义。
3. 领取前回收超时的 `running/processing` 任务；根据 `attempt_count` 决定重新排队或进入失败终态。
4. 业务副作用使用稳定业务键或 `event_id` 做幂等；不要把“只发送一次”当作进程级保证。
5. 将抓取全局锁缩小到“发现到期来源并入队”的短区间，领取和执行放到锁外，使多个 worker 可以真正并行。
6. 对 `item_events` 做功能决策：若告警链路继续保留，接入明确的 event worker；若功能暂不保留，停止继续生成未消费事件，不为未来功能预先增加 AI analysis worker。
7. 增加队列深度、最老任务年龄、领取耗时、重试次数、超时回收次数和死信数量指标。

## P1：Agent 执行角色化

### 目标

将 API 内存 goroutine 执行改为可恢复、可跨 Pod 领取的持久化任务。

### Agent 执行改造

API 接收请求时只完成以下事务：

```text
校验身份与权限
  -> 写入 inbound message / turn
  -> 写入 queued execution job
  -> 返回 202、turn_id 和 progress_url
```

独立 `agent-worker` 负责领取并执行任务。停止接口写入持久化的 `cancel_requested` 状态，worker 在模型调用、工具调用和步骤边界检查该状态。会话串行性使用数据库行锁或 advisory lock 保证，不使用单 Pod `sync.Mutex` 作为跨实例约束。

第一版不要求立刻拆仓库或引入内部 gRPC；同一代码库和镜像增加独立运行角色即可获得主要收益。

### P1 实施记录

截至 2026-08-06，P1 已在 `messageFeed` 子仓库完成：

1. API 生产模式将 Web 和企业微信 Agent turn 写入 `queued` 状态，返回 `202 Accepted`、`turn_id` 和进度 URL；默认构造行为仍保留内联/进程内异步模式，以兼容现有本地调用。
2. 复用 `agent_turns` 作为执行队列，新增尝试次数、最大尝试次数、worker owner、租约和持久化取消请求字段，并提供 `000039` 迁移及 Helm 内置迁移副本。
3. 新增 `agent-worker` 运行角色，使用 `FOR UPDATE SKIP LOCKED` 领取任务；租约过期任务可恢复，owner 条件更新阻止旧 worker 覆盖新状态。
4. 取消请求在 API 副本与 worker 之间通过数据库同步；执行中的 worker 轮询取消状态，模型/工具流程使用 context 取消，已取消任务不会因租约过期重新入队。
5. Worker 领取后持续续租，并复用 PostgreSQL `task_locks` 对会话加分布式租约锁；锁在执行期间自动续租，进程内互斥只作为同一 Pod 内的补充，锁竞争时任务重新入队。
6. Helm 增加 `agent-worker` Deployment、RBAC、PDB、NetworkPolicy、资源配置和 Prometheus 抓取目标，支持通过 `APP_ROLE=agent-worker` 独立水平扩展。

验证记录：`go test ./...`、`go vet ./...`、P1 Worker 的 `go test -race ./internal/service -run TestAgentWorkerProcessesQueuedTurnAndReleasesLease -count=1`、两个 Helm values 组合的 `helm lint`、迁移文件与 Helm 副本一致性校验及 `git diff --check` 均已通过。全仓库 `go test -race ./...` 仍会命中既有 Agent 异步测试替身的数据竞争，未将其归因于本次队列改动。

### P0/P1 验收

P0 队列可靠性：

- kill 一个已领取任务的 worker，任务能在租约过期后重新领取或进入明确失败状态。
- 同一任务重复投递时，数据库唯一约束或幂等检查不会产生重复业务结果。
- 两个 source worker 副本可以同时执行不同抓取任务，不再被全局执行锁串行化。

P1 Agent 角色化：

- API 重启后，已入库但未执行的 Agent 任务仍能被 worker 继续处理。
- 任意 API 副本收到停止请求后，执行中的 Agent worker 都能观察到取消状态。

## P2：双节点定向部署与资源安全

### 节点职责

| 节点 | 地址 | 仅允许承载的工作负载 |
| --- | --- | --- |
| 主服务节点 | 100.106.96.110 | API、Web、普通 CPU worker 及其水平副本 |
| GPU 推理节点 | 100.72.246.82 | GPU 密集型任务、本地 LLM 保底推理 worker |

两台机器通过节点标签与 taint 做硬隔离。主服务 Deployment 使用 nodeSelector 和拓扑分布约束固定到主服务节点；GPU worker 使用 GPU 节点 taint 容忍和 GPU 资源限制，禁止普通服务占用 GPU 节点。

### 主服务水平拓展

1. 在 100.106.96.110 部署主服务多副本，配置 readiness/liveness 探针、滚动更新和 PodDisruptionBudget。
2. 所有容器必须声明 CPU、内存 requests/limits；副本数、并发度和队列 worker 数量以节点可用容量为上限，不预留未使用的固定资源。
3. 使用 topology spread、反亲和和优雅终止，保证扩缩容及单 Pod 故障时服务仍可用。
4. 通过 API、队列深度、最老任务年龄、CPU/内存和重启次数指标确定扩容，空闲时缩回最低副本数。

### GPU 保底推理

1. 在 100.72.246.82 仅部署独立 agent-worker / 本地 LLM 推理进程，主服务通过持久化 Agent 任务队列投递保底任务。
2. GPU 使用 resources.limits.nvidia.com/gpu 明确上限；同时设置 CPU、内存和临时存储上限，禁止无界批处理、常驻调试进程和未声明的模型副本。
3. 保底推理只在上游模型不可用或超时达到阈值时触发；任务完成后释放上下文、临时文件和显存缓存，空闲 worker 缩容至零或停止调度。
4. GPU 节点不承载数据库、入口流量或普通 worker，避免推理资源被非目标任务保留。

### 两节点安全基线

1. SSH 仅允许密钥认证和最小管理来源，禁用密码登录；不在仓库、镜像或 Helm values 中保存私钥、模型凭据和数据库密码。
2. 使用最小 RBAC、独立 ServiceAccount、NetworkPolicy 和 Pod Security；容器以非 root 用户运行，启用 seccomp，根文件系统只读并禁止特权模式。
3. 不使用未审计的 hostPath、Docker socket 或主机网络；镜像固定版本并执行漏洞扫描，节点和 GPU 驱动保持安全更新。
4. 对 API、worker、模型服务分别限制出口地址和端口；管理面、监控面和推理面不直接暴露公网。
5. 设置 ResourceQuota、LimitRange、进程数限制和日志轮转；定期检查孤儿 Pod、Job、PVC、模型缓存和临时文件，发现闲置资源立即回收。
6. 部署前执行节点备份、变更记录和回滚演练；部署后审计登录、容器创建、GPU 使用和异常出网事件。

### P2 执行与验收

当前检查（2026-08-06）：主服务节点 Docker daemon 可用；GPU 节点已确认 RTX 4090 与 NVIDIA 驱动，但未发现可用 Docker/K3s 运行时，当前账号也没有免密 sudo。GPU 节点需先由管理员完成运行时和 NVIDIA Container Toolkit 配置，再继续部署。

1. 核对两台服务器的 SSH、K3s、GPU 驱动、磁盘和可用 CPU/内存，不满足安全基线则停止部署。
2. 先给节点打标签和 taint，再部署主服务与 GPU worker，确认调度结果没有跨节点漂移。
3. 验证主服务扩容、单 Pod 重建、节点维护和滚动回滚；验证 GPU 保底任务仅在触发条件下执行。
4. 观察资源 requests/limits、实际使用率、队列延迟、GPU 显存和闲置资源回收结果，确保没有无业务任务长期占用计算资源。
5. 保留部署命令、关键日志、指标截图和回滚结果；未完成的双节点验证不得表述为生产高可用能力。
