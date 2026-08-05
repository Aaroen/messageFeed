---
type: technical-note
status: in_progress
p0_status: implemented
p1_status: implemented
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
  -> P2：隔离环境完成双节点 K3s 实验
  -> P3：单独完成 Redis cache-aside 实验
  -> P4：单独完成 RabbitMQ 可靠投递实验
  -> 只有出现明确指标压力后，才接入主项目
```

主项目当前继续使用 PostgreSQL 作为事实来源和任务存储。Redis 先作为缓存或跨实例限流实验，RabbitMQ 先作为可靠投递实验；不同时把两者接入主链路，也不以中间件数量作为微服务化指标。

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

## P2：隔离环境 K3s 双节点实验

### 范围

使用本地虚拟机或其他隔离环境完成一台 server 加一台 agent 的 K3s 实验。该实验用于补齐部署与排障证据，不表述为生产级控制面高可用；真正的 K3s 控制面高可用通常需要三个 server 节点和可靠存储。

### 实验清单

1. 节点加入、token 管理、节点标签和集群 DNS。
2. API Deployment、Service、滚动更新、资源限制、readiness/liveness 探针。
3. ConfigMap、Secret、最小 RBAC、NetworkPolicy 和日志查看。
4. Pod 重建、节点 cordon/drain、镜像拉取失败、PVC 不可用和滚动发布失败。
5. 移除固定 hostname 约束，验证 topology spread 和 anti-affinity 在两个节点上生效。

主项目的 PostgreSQL 仍应优先迁移到支持 pgvector 的托管实例；若自建，则需要 CloudNativePG 或等价方案、跨节点存储、WAL 归档、备份和恢复演练。单节点本地卷不能作为数据库高可用证据。

## P3：Redis cache-aside 专项实验

### 设计

只选择 Feed 列表或条目详情作为缓存场景：

```text
读取：Redis 命中 -> 返回
      未命中 -> PostgreSQL 查询 -> 写入带抖动 TTL 的 Redis -> 返回

写入：先提交 PostgreSQL -> 删除对应缓存
      删除失败时数据库仍是事实来源，业务允许回源
```

建议使用稳定 key，例如 `feed:item:{user_id}:{item_id}:v1`，并记录命中率、回源耗时和缓存错误。实验不把 Session、权限事实或唯一任务状态放入 Redis。

### 必须验证

- 缓存穿透：不存在的 ID 是否有短期负缓存或限流。
- 缓存击穿：热点 key 同时过期时是否限制回源并发。
- 缓存雪崩：批量 TTL 是否有随机抖动，Redis 不可用时是否降级到 PostgreSQL。
- 热 key、大 key、缓存删除失败和旧值回填竞态。
- RDB/AOF、主从、Sentinel 和 Cluster 的差异只按实验范围记录，不扩展为未验证的生产经验。

### 主项目接入门槛

只有当 API 多副本导致本地缓存命中率不稳定，或全局限流无法由 Cloudflare/Caddy 解决时，才考虑引入 Redis。分布式锁和任务事实仍优先由 PostgreSQL 约束、行锁或租约承载。

## P4：RabbitMQ 可靠投递专项实验

### 最小拓扑

```text
producer -> durable exchange -> durable queue -> consumer
                                      |
                                      +-> retry queue -> DLQ
```

实验要求：

1. Producer 开启 publisher confirm，区分“Broker 已接收”和“业务已完成”。
2. Consumer 使用手动 ack，业务成功后再 ack。
3. 瞬时错误有限重试，永久错误进入失败终态，毒消息进入 DLQ，禁止无限 requeue。
4. 在“数据库事务已提交、ack 尚未完成”时杀掉 consumer，验证重复投递。
5. 使用稳定 `event_id` 和数据库唯一约束验证幂等消费。
6. 记录生产速率、消费速率、最老消息年龄、失败率、重试数和下游容量。

当前项目只有在需要跨服务事件广播、消费者独立扩缩容、持续积压或事件回放时，才有理由接入 RabbitMQ。若只是降低当前 PostgreSQL 轮询次数，先优化 claim、租约和指标，不用 MQ 代替故障模型。

## 复习与面试表达

### 30 秒结论版

> 我的主项目目前没有直接部署 Redis 或 RabbitMQ。项目规模较小，抓取、通知和 Embedding 任务使用 PostgreSQL 任务表、状态机和 `SKIP LOCKED` 由独立 worker 消费，数据库同时承担事实存储和任务状态。这样可以减少基础设施复杂度，但边界是任务回收、独立扩缩容和事件广播能力仍需要继续完善。若出现持续积压、需要多个服务订阅同一事件或需要回放，我会先补事务 Outbox 和幂等消费，再评估 RabbitMQ；Redis 则优先用于 cache-aside 或跨实例限流，而不是替代数据库事实。

### 90 秒展开版结构

```text
当前方案
  -> 为什么适合当前规模
  -> 任务 claim、超时、重试和幂等怎么保证
  -> 当前真实缺口是什么
  -> 哪个指标触发 Redis/MQ 演进
  -> 如何灰度、观测和回滚
```

回答 K3s 时必须说明：当前是 WSL2 单节点 K3s，已经完成 Helm、多角色 Deployment、探针、NetworkPolicy、资源治理和 Pod 级故障验证；尚未具备多节点控制面、跨节点存储和数据库高可用生产经验。

## 执行清单

- [ ] 盘点并处理长期 `item_events` pending 记录，先确认业务语义，不直接删除数据。
- [x] 统一任务租约、超时回收、重试和幂等字段及指标。
- [x] 缩小 source worker 全局锁范围，接入 item-event-worker 消费角色。
- [x] 设计并实现持久化 Agent execution job 和 `agent-worker` 角色。
- [ ] 完成隔离环境双节点 K3s 实验并保留命令、日志和故障现象。
- [ ] 在独立实验环境完成 Redis cache-aside 和故障降级验证。
- [ ] 在独立实验环境完成 RabbitMQ confirm、ack、重试、DLQ 和幂等验证。
- [ ] 将主项目事实、实验结果和理论知识分别记录，未完成项不写成已落地能力。

## 验收标准

1. `go test ./...`、`go vet ./...` 和必要的 `go test -race ./...` 通过。
2. 每一个任务队列都能说明生产者、消费者、状态转换、租约、重试、幂等和恢复路径。
3. Redis 实验能展示命中、回源、穿透、击穿、雪崩和不可用降级。
4. RabbitMQ 实验能展示 confirm、手动 ack、重复投递、有限重试和 DLQ。
5. K3s 实验能展示节点加入、服务发现、Pod 恢复、滚动发布和持久化边界。
6. 面试回答始终区分“主项目实际使用”“专项实验完成”和“理论方案”。

## 相关

- [[4. Go后端开发定制复习]]
- [[What‘sThis]]
- [[Q&A]]
- [[micr-k8s/micr-k8s-plan]]
- [[micr-k8s/micr-k8s-implement]]
