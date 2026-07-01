# Agent 记忆 Embedding 异步化与 Trace 观测完善方案

## 1. 背景与目标

当前长期记忆 RAG 索引层已经具备事实索引、全文召回、向量召回、关系扩展和来源投影能力。近期验证表明，`Qwen/Qwen3-Embedding-8B` 的 4096 维 embedding 已可写入 `agent_fact_embeddings`，`semantic` 与 `hybrid` 召回均可返回向量命中。

后续需要解决的问题不再是单点向量召回可用性，而是长期运行时的稳定性、延迟控制、成本控制和问题定位效率。核心目标如下：

1. 将 embedding、记忆巩固和历史 backfill 从用户回复主链路中剥离，降低用户端等待时间。
2. 从“上下文溢出后被动 embedding”升级为“按主题和价值主动形成记忆块并异步 embedding”。
3. 建立接近 LangSmith、Cozeloop 类系统的内部 trace 能力，覆盖 planner、context projection、recall、embedding、tool execution 和 final answer。
4. 保留现有 `agent_runs`、`agent_run_context_traces`、`agent_observations`、`agent_artifacts`、`agent_plans` 等内部事实表，不引入过早复杂外部平台依赖。
5. 使 RAG 召回质量、耗时、降级原因和 embedding 覆盖率可被明确观测。

## 2. 当前状态

### 2.1 已具备能力

| 能力 | 当前状态 |
| --- | --- |
| fact index | 已有 `agent_fact_archive_index` |
| embedding 存储 | 已有 `agent_fact_embeddings` |
| pgvector | 已启用，当前 embedding 字段为 `vector(4096)` |
| hybrid recall | 已支持 fulltext、vector、relation 合并 |
| recall diagnostics | preview 已返回 query embedding 状态、维度、候选数等 |
| progress 页面 | 已有 plan/progress 相关页面与 `progress_url` |
| 内部 trace 表 | 已有 run、context trace、observation、artifact、audit 等事实表 |

### 2.2 主要不足

1. embedding 仍存在同步触发点。当前 recall 命中事实后会调用惰性 embedding，可能增加召回请求耗时。
2. 尚无完整 embedding worker。`agent_fact_index_jobs` 已具备 job 概念，但还没有后台化、可重试、可观测的 embedding 队列。
3. 上下文裁剪与记忆巩固尚未形成统一策略。当前更偏事实索引，不够强调主题级记忆单元。
4. Trace 数据分散在多个表中，尚未形成“单次用户请求 waterfall”视图。
5. Recall trace 尚不完整，缺少每次召回的 fulltext、embedding、vector、relation、rerank 分段耗时和降级原因持久化。
6. embedding 覆盖率、stale embedding、失败重试等指标还没有进入 readyz 或管理接口。

### 2.3 已部署可观测性现状

当前项目已经具备较完整的通用可观测性基础，但 RAG 与 embedding 的专项观测尚未闭环。经代码与部署配置核实，现状如下：

| 层级 | 已有实现 | 位置 | 状态 | 主要缺口 |
| --- | --- | --- | --- | --- |
| 结构化日志 | `slog` JSON 输出，固定携带 `service`、`service_version`、`environment`、`node_id`、`deployment_mode` | `internal/observability/observability.go` | 已实现 | RAG/embedding worker 日志字段尚未统一 |
| request id | 继承或生成 `X-Request-ID`，写入 `context.Context` 与响应头 | `internal/observability/context.go`、`internal/handler/middleware.go` | 已实现 | 异步 job 需要继承触发请求或业务关联 id |
| HTTP access log | 访问日志携带 `request_id`、`trace_id`、`span_id`、method、path、status、duration | `internal/handler/middleware.go` | 已实现 | Agent 内部分段耗时未进入访问日志摘要 |
| OpenTelemetry trace | 支持 OTLP gRPC exporter、采样比例、span error 状态 | `internal/observability/observability.go` | 已实现 | Docker Compose 中 API 默认关闭 trace |
| Prometheus metrics | HTTP、DB、Feed、外部 HTTP、通知、企业微信、Agent turn、LLM 请求与 token | `internal/metrics/metrics.go` | 已实现 | 缺少 planner、subagent、tool、approval、memory、recall、embedding、queue 专项指标 |
| DB span/metrics | repository 层统一记录 DB span 与查询耗时 | `internal/repository/observability.go` | 已实现 | 新增表与查询需要继续使用统一封装 |
| LLM span/metrics | LLM chat 记录 span、请求耗时、请求数、token，并通过 `otelhttp` 传播外部 HTTP span | `internal/llm/openai_compatible.go` | 已实现 | embedding client 尚未记录专用 span、metrics、外部 HTTP 指标 |
| Embedding client | 支持 OpenAI-compatible `/embeddings`，有重试、维度一致性校验、限速间隔 | `internal/llm/openai_embedding.go` | 已实现 | 缺少 span、metrics、请求体大小、响应维度、失败类型观测 |
| 事实惰性 embedding | recall 命中 fact 后最多同步补齐 8 条 embedding | `internal/service/agent_fact_retrieval.go`、`internal/service/agent_fact_embedding_service.go` | 已实现 | 应改为投递后台 job，并保留同步路径只用于显式管理任务 |
| readiness | `/readyz` 检查 database、migration、pgvector、agent fact index | `internal/handler/router.go` | 已实现 | 需加入 embedding 覆盖率、pending/failed job、worker 新鲜度 |
| runtime node | `/api/runtime/node` 返回节点、部署模式、公开地址等 | `internal/runtime/node.go` | 已实现 | 多 worker 时需展示 worker id 与职责 |
| 管理配置状态 | 设置页可展示 trace 是否启用、采样、Prometheus、Grafana 地址 | `internal/service/admin_config_service.go`、`web/src/views/SettingsView.vue` | 已实现 | 尚未展示 Agent trace、RAG、embedding、worker 子系统健康度 |
| Agent 内部 trace | `agent_runs`、`agent_run_context_traces`、`agent_observations`、`agent_artifacts`、`agent_audit_logs` 等 | domain/repository/service 多处 | 已实现 | 缺少统一 `agent_trace_events`、recall trace 与 embedding trace 独立事实表 |
| Progress 页面 | 可展示 controller run、context trace、ContextBundle、历史召回块、运行观测摘要 | `web/src/views/AgentPlanView.vue` | 已实现 | 缺少主 Agent/子 Agent/工具/审批/RAG/worker 统一 waterfall 与延迟拆分 |
| Prometheus | 抓取 `api:60001/metrics`、`api-dev:60001/metrics`、collector metrics | `ops/observability/prometheus/prometheus.yml` | 已部署 | 指标尚未覆盖全 Agent 行为与 RAG/embedding 专项 |
| Loki/Promtail | Docker 日志采集，解析 JSON 字段并提取 `trace_id` 标签 | `ops/observability/loki/loki.yml`、`ops/observability/promtail/promtail.yml` | 已部署 | 异步 job 日志需要标准字段以便检索 |
| Tempo/OTel Collector | OTLP receiver、batch processor、trace 写入 Tempo | `ops/observability/tempo/tempo.yml`、`ops/observability/otel-collector/otel-collector.yml` | 已部署 | API trace 默认关闭，生产启用策略需明确 |
| Grafana datasource | Prometheus、Loki、Tempo 已配置，Loki `trace_id` 可跳转 Tempo | `ops/observability/grafana/provisioning/datasources/datasources.yml` | 已部署 | Agent waterfall 相关 dashboard 尚未添加 |
| Grafana dashboard | 已有 HTTP、DB、Feed、外部 HTTP、通知、Agent、LLM 等概览 | `ops/observability/grafana/dashboards/messagefeed-overview.json` | 已部署 | 需新增 planner、subagent、tool、approval、recall、embedding、queue、覆盖率面板 |

### 2.4 当前观测性结论

1. 通用观测链路已经成型：日志、metrics、trace provider、Prometheus、Loki、Tempo、OTel Collector、Grafana 均存在可复用实现。
2. API 进程已可在启用 trace 后把 HTTP、repository、service、LLM、通知等 span 输出到 Tempo；日志中也具备 `trace_id` 与 `span_id` 关联字段。
3. 当前部署配置中 API 默认 `OBSERVABILITY_TRACE_ENABLED=false`，因此 Tempo 链路处于“已部署但默认不采集 API trace”的状态。开发环境可继续默认关闭，联调、预发和生产建议按采样比例启用。
4. RAG 当前可通过 preview diagnostics 判断 query embedding 状态、向量维度、候选数和错误原因，但这些信息尚未统一写入持久化 trace，也未进入 Prometheus 和 Grafana。
5. embedding client 当前是功能型实现，未沿用 LLM chat client 的 span/metrics 模式，因此无法直接回答“embedding 失败率、延迟分布、批大小、维度、重试次数、上游错误类型”等运行问题。
6. progress 页面已具备展示 context trace 和历史召回内容的基础，但尚不是统一 waterfall 视图，无法按一次请求顺序展示 planner、context projection、subagent dispatch、tool execution、approval、recall、final answer、notification 与 async jobs。

### 2.5 后续可观测性设计原则

后续每新增一个 Agent 行为或运行阶段，不应只完成业务表和接口，还必须同步补齐观测面。该原则覆盖主 Agent 规划、上下文投影、历史召回、下发子 Agent、工具执行、审批、通知、异步任务、RAG、memory、embedding、评测和恢复流程。

1. OpenTelemetry span：覆盖耗时较高、可能失败、存在降级路径的步骤。
2. Prometheus metrics：覆盖吞吐、耗时、失败率、队列深度、覆盖率和降级次数。
3. 持久化 trace 表：保存业务诊断所需字段，避免仅依赖外部 trace retention。
4. 结构化日志字段：异步 worker 与模型调用必须携带可检索字段。
5. readiness/admin status：暴露可用性、积压和最近错误。
6. Progress/Grafana UI：分别服务单次任务排障和整体运行趋势分析。

观测设计应采用“全链路主干 + 专项子域”的结构。全链路主干负责回答一次用户请求从进入系统到完成回复的完整路径；专项子域负责回答某类能力内部为何成功、失败或降级。例如 RAG/memory 是专项子域之一，而不是唯一需要观测的对象。

全链路主干至少包含：

| 阶段 | 必须回答的问题 |
| --- | --- |
| inbound | 请求从哪个入口进入，request id、trace id、用户与会话关联是否完整 |
| transcript | 用户消息与 assistant 回复是否落库，耗时和失败原因是什么 |
| context projection | 哪些短期上下文、历史块、计划、artifact 被纳入或裁剪 |
| planner | 主 Agent 是否生成有效计划，是否需要历史召回、工具、审批或子 Agent |
| subagent dispatch | 子 Agent 是否被创建、下发、开始执行、完成或失败 |
| tool execution | 工具是否被调用，输入摘要、输出摘要、artifact、错误和重试是什么 |
| approval/governance | 是否触发权限、预算、人工审批、安全边界或恢复策略 |
| recall/memory | 是否执行历史召回，命中了什么，是否降级，是否投递记忆巩固任务 |
| final answer | final LLM 是否成功，token、耗时、模型路由和输出状态是什么 |
| notification | Web/企业微信通知是否发送成功，失败是否可重试 |
| async workers | 后台 job 是否被 claim、执行、重试、积压或失败 |

## 3. 总体目标架构

建议采用以下长期架构：

```text
用户请求
  -> Transcript 写入
  -> 短期上下文构建
  -> Planner 判断是否需要历史
  -> 必要时执行 Hybrid Recall
  -> LLM 生成回复
  -> 返回用户

后台异步链路：
  Transcript/Turn/Plan/Artifact 事件
  -> TopicTracker
  -> MemoryConsolidationPolicy
  -> MemoryChunkBuilder
  -> FactIndexBuilder
  -> EmbeddingJobQueue
  -> EmbeddingWorker
  -> agent_fact_embeddings
  -> RecallTrace/Eval 样本记录
```

该架构将用户响应链路与记忆处理链路拆开：

1. 用户链路只做必要的上下文读取、轻量 recall 和回复生成。
2. 记忆巩固、主题聚合、embedding、失败重试和统计分析由后台 worker 处理。
3. 上下文溢出只作为兜底触发条件，不作为主要 embedding 策略。

## 4. Embedding 设计

### 4.1 Embedding 时机

建议保留三类触发时机，但权重不同：

| 时机 | 作用 | 是否阻塞用户请求 |
| --- | --- | --- |
| 主题记忆块完成 | 主路径。将稳定主题、偏好、事实、决策主动沉淀 | 否 |
| 上下文溢出 | 兜底路径。处理即将被裁剪出短期窗口的高价值内容 | 否 |
| 召回命中未向量化事实 | 兜底补齐。当前同步惰性 embedding 应改为投递 job | 否 |

不建议继续依赖“上下文达到限制后每次请求都触发 embedding”。原因是该方式触发较晚，且长会话中容易重复进入压缩路径，影响延迟和成本控制。

### 4.2 主题记忆块

建议新增 `agent_memory_topics` 和 `agent_memory_chunks`。

`agent_memory_topics`：

```text
id
user_id
session_id
topic_key
title
summary
keywords_json
intent
status                  active / closed
message_count
token_estimate
start_turn_id
end_turn_id
last_message_at
created_at
updated_at
```

`agent_memory_chunks`：

```text
id
user_id
session_id
topic_id
title
summary
content
memory_kind             preference / fact / decision / task / casual / unknown
importance
source_refs_json
relation_refs_json
start_turn_id
end_turn_id
content_hash
embedding_status        pending / ready / failed / archived
consolidation_reason    high_value / topic_switch / size_limit / context_overflow / idle
metadata_json
created_at
updated_at
```

`agent_fact_archive_index` 需要支持新的 fact type：

```text
fact_type = memory_chunk
canonical_ref = memory_chunk:{id}
fact_id = agent_memory_chunks.id
```

### 4.3 后端判断策略

第一阶段采用规则优先、模型辅助的策略，避免每条消息都调用模型判断主题。

#### 4.3.1 主题延续判断

输入：

1. active topic 的摘要、关键词、intent。
2. 最近 3 到 5 条 transcript。
3. 当前新消息。
4. 时间间隔。

判断信号：

| 信号 | 判断依据 |
| --- | --- |
| 时间间隔 | 超过 30 分钟倾向新主题 |
| 关键词重叠 | overlap 高于 0.35 倾向同主题，低于 0.10 倾向新主题 |
| intent 变化 | 从问答变为任务、从配置变为闲聊等倾向新主题 |
| 高价值表达 | “记住、偏好、以后、决定、配置、路径、错误码、部署、方案”等倾向立即巩固 |
| embedding similarity | 可选增强项，不作为第一阶段硬依赖 |

伪代码：

```go
func ClassifyTopic(active TopicState, msg TranscriptEntry) TopicDecision {
    if active.Empty() {
        return NewTopic
    }
    if msg.CreatedAt.Sub(active.LastMessageAt) > 30*time.Minute {
        return NewTopic
    }
    if hasHighValueMemorySignal(msg.Content) {
        return SameTopicWithImmediateConsolidation
    }

    overlap := keywordOverlap(active.Keywords, extractKeywords(msg.Content))
    intentChanged := inferIntent(msg.Content) != active.Intent

    if overlap >= 0.35 && !intentChanged {
        return SameTopic
    }
    if overlap <= 0.10 || intentChanged {
        return NewTopic
    }
    return Uncertain
}
```

#### 4.3.2 Memory consolidation 判断

触发条件：

```text
should_consolidate =
  high_value_signal
  OR topic_switch
  OR topic_message_count >= 6
  OR topic_token_estimate >= 1000
  OR context_budget_usage >= 80%
  OR omitted_units_count > 0
  OR session_idle >= 30m
```

不同触发原因对应不同动作：

| 触发原因 | 动作 |
| --- | --- |
| high_value_signal | 立即形成小 chunk，优先 embedding |
| topic_switch | 关闭旧 topic，形成主题 chunk |
| topic_size_exceeded | 对当前 topic 做阶段性 chunk |
| context_overflow | 仅对被裁剪且高价值内容形成 chunk |
| session_idle | 后台整理 active topic |

### 4.4 Embedding Job

建议新增或扩展 `agent_fact_index_jobs`，让 `embed_fact_index` 成为真正可执行的后台任务。

job scope 示例：

```json
{
  "user_id": 1,
  "session_id": 50,
  "canonical_refs": ["memory_chunk:12", "transcript:184"],
  "reason": "topic_switch",
  "embedding_model": "Qwen/Qwen3-Embedding-8B"
}
```

Worker 逻辑：

```text
扫描 pending embed_fact_index job
  -> 加锁 claim
  -> 加载 canonical_refs 对应 facts/chunks
  -> 过滤已 ready 且 content_hash 未变化的记录
  -> 批量调用 embedding
  -> upsert agent_fact_embeddings
  -> 更新 embedding_json / embedding_status
  -> 成功则 job=succeeded
  -> 失败则记录 error_message、retry_count、next_retry_at
```

### 4.5 用户端延迟策略

用户请求主链路不应执行大批量 embedding。建议策略：

1. query embedding 设置短超时，例如 2 到 5 秒。
2. hybrid 中 vector 失败时降级到 fulltext，并记录降级原因。
3. semantic 模式用于测试和明确语义检索场景，失败时返回明确错误。
4. 惰性 embedding 改为创建后台 job，不再同步调用模型。
5. memory chunk consolidation 只写 job，不阻塞回复。

## 5. Trace 与观测设计

### 5.1 目标

Trace 体系要回答以下问题：

1. 本次用户请求为什么慢。
2. Planner 是否判断需要历史召回。
3. Recall 是否执行 fulltext、vector、relation。
4. Query embedding 是否成功，耗时多少，维度多少。
5. Vector 候选是否被扫描、合并和裁剪。
6. 最终回答使用了哪些 source fact。
7. embedding 或工具失败是否导致降级。
8. 哪些步骤可以回放和评估。

### 5.2 现有 trace 基础

当前可复用表：

| 表 | 用途 |
| --- | --- |
| `agent_runs` | Agent 执行单元 |
| `agent_run_context_traces` | prompt、context projection、模型响应等 |
| `agent_observations` | 工具调用观察结果 |
| `agent_artifacts` | 工具产物 |
| `agent_plans` | 计划 |
| `agent_plan_steps` | 步骤 |
| `agent_audit_logs` | 治理与审计 |

### 5.3 全 Agent 行为观测矩阵

现有 `agent_runs`、`agent_plan_steps`、`agent_observations`、`agent_artifacts` 和 `agent_audit_logs` 已经为全 Agent 行为 trace 提供基础。后续完善时应先保证这些主干行为具有统一 trace 语义，再补 RAG/memory 专项表。

| 行为域 | 现有承载 | 需要补强的观测内容 |
| --- | --- | --- |
| 主 Agent 规划 | `agent_runs`、`agent_run_context_traces`、`agent_plans` | planner 输入摘要、输出解析状态、计划有效性、needs_history_recall、tool/subagent/approval 需求、模型路由和耗时 |
| 上下文投影 | `agent_run_context_traces` | budget profile、selected/skipped/protected/projected 单元数、裁剪原因、历史块来源、token 分布 |
| 子 Agent 下发 | `agent_runs`、`agent_plan_steps` | parent_run_id、child_run_id、dispatch reason、capability scope、task packet、开始/结束/失败时间 |
| 工具执行 | `agent_observations`、`agent_artifacts` | capability_key、tool name、输入摘要、输出摘要、artifact refs、重试次数、错误分类、幂等状态 |
| 审批与治理 | `agent_approvals`、`agent_audit_logs`、plan metadata | 权限边界、预算检查、人工审批、恢复策略、拒绝原因、风险级别 |
| LLM 调用 | `agent_runs`、LLM metrics/span | operation、provider、model、protocol、token、耗时、路由降级、响应解析状态 |
| RAG/历史召回 | 新增 `agent_recall_traces` | fulltext、query embedding、vector、relation、projection、fallback、final sources |
| memory/embedding | 新增 memory 表、`agent_embedding_traces` | topic/chunk 形成原因、job 生命周期、覆盖率、stale、失败重试 |
| 通知与回调 | notification jobs、WeChat callback metrics/span | channel、recipient type、provider errcode、重试、callback replay trace |
| 后台 worker | job 表、worker logs、metrics | worker_id、claim latency、duration、queue depth、success/failure/retry、last heartbeat |
| 运行恢复 | `agent_audit_logs`、plan metadata | failure classification、recovery action、retry decision、handoff 状态 |

### 5.4 新增 Agent Trace Event

为了避免每类行为都新增一张细粒度表，建议新增通用 `agent_trace_events` 作为全链路 waterfall 主表，用于承载跨行为域的结构化事件；RAG 与 embedding 仍保留专项表存放高维诊断字段。

```text
id
request_id
trace_id
span_id
user_id
session_id
turn_id
plan_id
run_id
parent_run_id
step_id
event_kind              inbound / transcript / context_projection / planner / subagent_dispatch / tool_execution / approval / recall / llm / notification / worker / recovery
event_name
status                  started / succeeded / failed / skipped / degraded
started_at
finished_at
duration_ms
model_key
capability_key
tool_name
job_id
artifact_refs_json
source_refs_json
input_summary
output_summary
error_code
error_message
metadata_json
created_at
```

写入原则：

1. `agent_trace_events` 记录事件边界、状态、耗时和关联关系，不保存大段原始 prompt、completion 或工具输出。
2. 大文本、结构化上下文和模型输入输出继续放在 `agent_run_context_traces`、`agent_observations`、`agent_artifacts` 等专用事实表。
3. RAG 召回和 embedding 由于诊断字段较多，继续使用 `agent_recall_traces` 与 `agent_embedding_traces`，并通过 `request_id`、`turn_id`、`run_id`、`trace_id` 与通用事件关联。

### 5.5 新增 Recall Trace

建议新增 `agent_recall_traces`：

```text
id
request_id
trace_id
user_id
session_id
turn_id
run_id
plan_id
mode
query_text
needs_history_recall
history_query_plan_json
fulltext_attempted
fulltext_count
fulltext_ms
embedding_attempted
embedding_model
embedding_dimension
embedding_ms
embedding_status
embedding_error
vector_attempted
vector_candidate_count
vector_ms
relation_attempted
relation_count
relation_ms
final_hit_count
final_sources_json
fallback_reason
total_ms
created_at
```

### 5.6 新增 Embedding Trace

建议新增 `agent_embedding_traces`：

```text
id
job_id
request_id
user_id
canonical_ref
embedding_model
embedding_dimension
input_chars
content_hash
status
duration_ms
error_message
retry_count
created_at
```

### 5.7 Prometheus 指标补齐

Agent 全链路指标与 RAG/embedding 专项指标均应按低基数原则补齐，不把 `user_id`、`session_id`、`canonical_ref`、`plan_id`、`run_id` 作为 label。需要新增的指标如下：

| 指标 | 类型 | labels | 用途 |
| --- | --- | --- | --- |
| `messagefeed_agent_trace_events_total` | counter | `event_kind`,`status` | 统计全 Agent 行为事件结果 |
| `messagefeed_agent_trace_event_duration_seconds` | histogram | `event_kind`,`status` | 统计规划、投影、子 Agent、工具、审批、通知等行为耗时 |
| `messagefeed_agent_planner_requests_total` | counter | `status`,`needs_history_recall`,`needs_approval` | 统计主 Agent planner 结果 |
| `messagefeed_agent_subagent_dispatches_total` | counter | `capability`,`status` | 统计子 Agent 下发与执行结果 |
| `messagefeed_agent_tool_executions_total` | counter | `capability`,`tool`,`status` | 统计工具调用结果 |
| `messagefeed_agent_tool_execution_duration_seconds` | histogram | `capability`,`tool`,`status` | 统计工具耗时 |
| `messagefeed_agent_approvals_total` | counter | `decision`,`risk_level` | 统计审批与治理决策 |
| `messagefeed_agent_async_jobs_total` | counter | `job_type`,`status` | 统计所有 Agent 后台 job，不限 embedding |
| `messagefeed_agent_async_queue_depth` | gauge | `job_type`,`status` | 统计所有 Agent job 积压 |
| `messagefeed_agent_memory_topics_total` | counter | `status`,`reason` | 统计 topic 创建、关闭、归档 |
| `messagefeed_agent_memory_chunks_total` | counter | `memory_kind`,`reason`,`status` | 统计 chunk 形成与状态 |
| `messagefeed_agent_recall_requests_total` | counter | `mode`,`status`,`fallback_reason` | 统计 recall 请求与降级 |
| `messagefeed_agent_recall_duration_seconds` | histogram | `mode`,`stage`,`status` | 统计 fulltext、query_embedding、vector、relation、projection、total 分段耗时 |
| `messagefeed_agent_recall_hits` | histogram | `mode`,`source` | 统计 fulltext/vector/relation/final 命中数 |
| `messagefeed_agent_embedding_requests_total` | counter | `provider`,`model`,`operation`,`status` | 统计 embedding API 调用结果 |
| `messagefeed_agent_embedding_duration_seconds` | histogram | `provider`,`model`,`operation`,`status` | 统计 query embedding 与 batch embedding 耗时 |
| `messagefeed_agent_embedding_batch_size` | histogram | `provider`,`model`,`operation` | 统计批量 embedding 输入数量 |
| `messagefeed_agent_embedding_input_chars` | histogram | `provider`,`model`,`operation` | 统计 embedding 输入规模 |
| `messagefeed_agent_embedding_jobs_total` | counter | `status`,`reason` | 统计 job claim、success、failed、retry、skipped |
| `messagefeed_agent_embedding_job_duration_seconds` | histogram | `status` | 统计 job 生命周期耗时 |
| `messagefeed_agent_embedding_queue_depth` | gauge | `status` | 统计 pending、running、failed、retryable 积压 |
| `messagefeed_agent_embedding_coverage_ratio` | gauge | `fact_type`,`embedding_model` | 统计 fact/chunk embedding 覆盖率 |
| `messagefeed_agent_memory_stale_embeddings` | gauge | `fact_type`,`embedding_model` | 统计 content hash 变化后的 stale embedding |

### 5.8 Span 命名与属性规范

后续 span 建议保持稳定命名，便于 Grafana Tempo 与日志检索：

| 操作 | span name | 关键属性 |
| --- | --- | --- |
| inbound 消息 | `service.agent.inbound` | `agent.provider`、`agent.msg_type`、`agent.session_id`、`agent.turn_id` |
| transcript 写入 | `service.agent.transcript.write` | `agent.turn_id`、`agent.message_role`、`agent.message_bytes` |
| 上下文投影 | `service.agent.context_projection` | `agent.run_id`、`context.profile`、`context.selected_units`、`context.skipped_units` |
| 主 Agent 规划 | `service.agent.planner` | `agent.run_id`、`llm.model`、`agent.needs_history_recall`、`agent.plan_valid` |
| 子 Agent 下发 | `service.agent.subagent.dispatch` | `agent.parent_run_id`、`agent.child_run_id`、`agent.capability_scope` |
| 计划步骤执行 | `service.agent.plan_step.execute` | `agent.plan_id`、`agent.step_id`、`agent.capability_key` |
| 工具执行 | `service.agent.tool.execute` | `agent.capability_key`、`tool.name`、`tool.status`、`tool.retry_count` |
| artifact 写入 | `service.agent.artifact.write` | `agent.run_id`、`artifact.type`、`artifact.ref_hash` |
| 审批决策 | `service.agent.approval.evaluate` | `agent.plan_id`、`approval.decision`、`approval.risk_level` |
| final answer | `service.agent.final_answer` | `agent.turn_id`、`llm.model`、`llm.output_tokens`、`agent.reply_status` |
| 通知发送 | `service.agent.notification.enqueue` | `notification.channel`、`notification.reason`、`job.status` |
| 主题判断 | `service.agent.memory.topic.classify` | `agent.user_id_hash`、`agent.session_id`、`memory.topic_id`、`memory.decision`、`memory.reason` |
| 记忆巩固判断 | `service.agent.memory.consolidation.evaluate` | `memory.topic_id`、`memory.trigger_reason`、`memory.token_estimate`、`memory.message_count` |
| chunk 构建 | `service.agent.memory.chunk.build` | `memory.chunk_id`、`memory.kind`、`memory.source_count`、`memory.importance` |
| fact index 写入 | `service.agent.fact_index.upsert` | `agent.fact_type`、`agent.canonical_ref_hash`、`agent.embedding_status` |
| recall 总入口 | `service.agent.recall` | `agent.recall.mode`、`agent.recall.needs_history`、`agent.recall.limit` |
| fulltext 召回 | `service.agent.recall.fulltext` | `agent.recall.hit_count`、`db.sql.table` |
| query embedding | `service.agent.recall.query_embedding` | `llm.provider`、`llm.model`、`embedding.dimension`、`embedding.status` |
| vector search | `service.agent.recall.vector_search` | `agent.recall.vector_candidates`、`embedding.model`、`embedding.dimension` |
| relation expansion | `service.agent.recall.relation_expand` | `agent.recall.relation_refs`、`agent.recall.hit_count` |
| source projection | `service.agent.recall.source_projection` | `agent.recall.source_count` |
| embedding job claim | `service.agent.embedding_job.claim` | `job.id`、`job.status`、`worker.id` |
| batch embedding | `service.agent.embedding.batch_embed` | `llm.provider`、`llm.model`、`embedding.batch_size`、`embedding.input_chars` |
| embedding upsert | `service.agent.embedding.upsert` | `agent.fact_type`、`embedding.dimension`、`embedding.status` |

敏感文本不应直接进入 span attribute。`canonical_ref`、query text、source refs 如需关联，应优先使用 hash、计数和 trace 表中的受控字段。

### 5.9 单次请求 Waterfall

前端或内部 API 可以按 `request_id` / `turn_id` 聚合：

```text
inbound_message
  -> transcript_write
  -> context_projection
  -> planner_call
      -> plan_validation
      -> governance_budget_permission_check
  -> recall
      -> fulltext
      -> query_embedding
      -> vector_search
      -> relation_expand
      -> projection
  -> subagent_dispatch
      -> child_run_start
      -> child_context_projection
      -> child_tool_calls
      -> child_artifacts
      -> child_run_finish
  -> tool_calls
      -> tool_input
      -> tool_execution
      -> observation
      -> artifact_write
  -> approval_or_recovery
  -> final_llm_call
  -> transcript_reply_write
  -> notification_enqueue_or_send
  -> async_jobs_enqueued
```

### 5.10 与 LangSmith / Cozeloop 的关系

第一阶段建议继续建设内部 trace。原因：

1. 项目已有 Agent 运行事实表，接入成本低。
2. 内部 trace 可保存业务字段，如 `user_id`、`canonical_ref`、`plan_id`、`capability_key`。
3. 对敏感数据脱敏和访问控制更可控。

后续可以增加导出层：

1. OpenTelemetry span 导出到 Tempo / Jaeger。
2. 将 prompt、completion、tool、retrieval 样本导出到 LangSmith 或 Cozeloop 做评测和对比。
3. 内部表仍作为权威运行事实。

## 6. 主链路与异步链路流程图

### 6.1 用户请求主链路

```text
用户请求
  -> 写 inbound message
  -> 写 user transcript
  -> 构建短期上下文
  -> Planner
       -> needs_history_recall?
            -> 否：跳过 recall
            -> 是：Hybrid Recall
                 -> fulltext
                 -> query embedding
                 -> vector search
                 -> relation expand
                 -> source projection
  -> LLM final answer
  -> 写 assistant transcript
  -> 返回用户
  -> 投递异步 memory/embedding jobs
```

### 6.2 主题记忆异步链路

```text
transcript event
  -> TopicTracker
  -> 更新 active topic
  -> ConsolidationPolicy
       -> 不触发：结束
       -> 触发：
            -> MemoryChunkBuilder
            -> 写 agent_memory_chunks
            -> 写 agent_fact_archive_index
            -> 创建 embed_fact_index job
```

### 6.3 Embedding Worker

```text
pending embed_fact_index job
  -> claim job
  -> load facts/chunks
  -> check content_hash/model/dimension
  -> batch embedding
  -> upsert agent_fact_embeddings
  -> update embedding status
  -> write embedding trace
  -> update job status
```

### 6.4 Trace 聚合

```text
request_id / turn_id
  -> agent_trace_events
  -> agent_runs
  -> agent_run_context_traces
  -> agent_plans / agent_plan_steps
  -> agent_observations
  -> agent_artifacts
  -> agent_approvals
  -> agent_audit_logs
  -> agent_recall_traces
  -> agent_embedding_traces
  -> progress detail / trace waterfall
```

## 7. 后端接口建议

### 7.1 管理接口

```text
GET  /api/v1/agent/fact-index/stats
POST /api/v1/agent/fact-index/backfill
POST /api/v1/agent/fact-index/embed
GET  /api/v1/agent/fact-index/jobs
GET  /api/v1/agent/fact-index/jobs/:id
```

### 7.2 Trace 接口

```text
GET /api/v1/agent/traces/requests/:request_id
GET /api/v1/agent/traces/turns/:turn_id
GET /api/v1/agent/traces/runs/:run_id
GET /api/v1/agent/traces/plans/:plan_id
GET /api/v1/agent/traces/recalls/:id
GET /api/v1/agent/traces/embedding-jobs/:id
```

### 7.3 指标字段

`fact-index/stats` 建议增加：

```text
embedding_coverage
transcript_embedding_coverage
memory_chunk_count
memory_chunk_embedding_coverage
pending_embedding_job_count
failed_embedding_job_count
stale_embedding_count
last_embedding_error
```

## 8. 验证方案

### 8.1 单元测试

1. 短对话不创建 memory chunk。
2. 高价值内容立即创建 memory chunk。
3. 主题切换关闭旧 topic 并创建 chunk。
4. 上下文压力触发 overflow chunk。
5. embedding job 跳过 content hash 未变化的 ready fact。
6. embedding 失败记录 failed 状态和 error_message。
7. recall trace 记录 fulltext、embedding、vector、relation 分段耗时。

### 8.2 集成测试

构造 session：

1. 写入早期偏好：“我的回答偏好是先给结论”。
2. 写入多轮无关对话，使早期内容不再进入短期上下文。
3. 触发主题巩固 worker。
4. 执行 embedding worker。
5. 提问：“我之前说过回答格式偏好吗？”

期望：

1. Planner 判断需要历史召回。
2. hybrid recall 命中 memory chunk 或 transcript fact。
3. 命中来源包含 `vector` 或 `fulltext+vector`。
4. projection 中含 `source_fact`。
5. final answer 正确回答偏好。
6. trace 中可见 recall 和 embedding 耗时。

### 8.3 延迟验证

需要采集：

```text
p50 / p95 用户请求耗时
planner_ms
recall_total_ms
query_embedding_ms
vector_search_ms
final_llm_ms
async_jobs_enqueued_count
```

验收标准建议：

1. 短对话不触发历史召回时，额外记忆处理耗时接近 0。
2. hybrid recall query embedding 超时不超过配置阈值。
3. embedding worker 失败不影响用户回复成功率。
4. progress 页面可定位主要耗时阶段。

## 9. 实施步骤清单

本清单后续实施时应逐项勾选。每完成一项功能实现，需要同步完成同一行的观测性更新；否则该项不视为完成。

| 状态 | 实施项 | 必须同步更新的观测性实现 |
| --- | --- | --- |
| [ ] | 建立全 Agent trace event 写入规范 | 新增 `agent_trace_events` migration、domain、repository；定义 event_kind/status 枚举；所有主链路事件统一关联 request_id、trace_id、turn_id、plan_id、run_id |
| [ ] | 补齐主 Agent planner trace | planner span 记录 plan_valid、needs_history_recall、needs_approval、needs_tool、needs_subagent；metrics 记录 planner 请求数、失败数、耗时；trace event 记录输入/输出摘要和解析错误 |
| [ ] | 补齐 context projection trace | 将 selected/skipped/protected/projected 单元数、裁剪原因、token 分布写入 trace event；progress detail 展示投影 waterfall 与预算变化 |
| [ ] | 补齐子 Agent 下发 trace | dispatch span 记录 parent_run_id、child_run_id、capability_scope；metrics 记录 dispatch 成功、失败、耗时；trace event 关联 plan step 与 child run |
| [ ] | 补齐工具执行 trace | tool span 记录 capability、tool、retry_count、status；metrics 记录工具调用次数、失败率、耗时；observation/artifact 与 trace event 建立稳定关联 |
| [ ] | 补齐审批、治理与恢复 trace | approval/governance/recovery 事件写入 `agent_trace_events` 与 audit log；metrics 记录审批决策、风险级别、恢复动作和失败分类 |
| [ ] | 补齐通知与回调 trace | notification enqueue/send/callback replay 事件统一关联 request_id、turn_id、job_id；Grafana 展示通知成功率、重试和 provider 错误 |
| [ ] | 新增 `agent_memory_topics`、`agent_memory_chunks` migration | `/readyz` 增加 memory 表结构检查；repository 新查询统一使用 `traceRepositoryOperation`；管理统计预留 topic/chunk count |
| [ ] | 新增 `memory_chunk` fact type | fact index stats 按 `fact_type` 输出 chunk 数量与覆盖率；日志字段增加 `fact_type`、`canonical_ref_hash` |
| [ ] | 实现规则版 `TopicTracker` | 新增 `service.agent.memory.topic.classify` span；新增 topic created/closed counter；trace 内容记录 decision、reason、message_count |
| [ ] | 实现 `MemoryConsolidationPolicy` | 新增 `service.agent.memory.consolidation.evaluate` span；新增 consolidation counter 与 duration；记录 trigger_reason、token_estimate、omitted_units_count |
| [ ] | 实现 `MemoryChunkBuilder` | 新增 chunk build span 与 chunk counter；写入 `agent_run_context_traces` 或新增 memory trace 记录 source refs、risk level、redaction status |
| [ ] | 将 memory chunk 写入 `agent_fact_archive_index` | 新增 fact index upsert span；DB 指标自动覆盖新增表操作；记录 indexer_version、content_hash、embedding_status |
| [ ] | 将惰性 embedding 改为投递 `embed_fact_index` job | recall trace 记录 `async_embedding_enqueued` 与 enqueued count；新增 job enqueue counter；召回日志记录降级与投递原因 |
| [ ] | 实现 embedding worker claim、执行、重试和状态更新 | 新增 job claim/batch/upsert span；新增 jobs total、duration、queue depth、batch size、input chars、coverage、stale gauges；worker 日志携带 `worker_id`、`job_id`、`attempt`、`status` |
| [ ] | 新增 `agent_recall_traces` | repository 查询使用 DB span；管理接口按 request/turn/run 查询；trace 表记录 fulltext、embedding、vector、relation、projection、fallback |
| [ ] | 新增 `agent_embedding_traces` | 记录 job_id、request_id、canonical_ref、model、dimension、duration、status、error；失败时截断 error_message 并保留 retry_count |
| [ ] | 将 recall diagnostics 持久化到 recall trace | `agentFactRetriever.Recall` 分段计时；hybrid 降级时记录 fallback_reason；semantic 失败时记录明确 error code |
| [ ] | 为 embedding client 增加 span 和 metrics | 参考 LLM chat client 增加 request counter、duration histogram、batch size、input chars、dimension attribute；外部 HTTP 可复用 `ExternalHTTPRequestsTotal` |
| [ ] | 在 progress detail 中展示 recall waterfall | Web 增加 request/turn trace 聚合视图；展示 stage、status、duration、hit_count、fallback_reason、source refs |
| [ ] | 扩展 fact index stats 覆盖率指标 | 管理接口与设置页展示 coverage、pending、failed、stale、last_error、last_success_at；Grafana 增加覆盖率与积压面板 |
| [ ] | 增加真实模型端到端测试用例 | 测试断言 recall trace、embedding trace、metrics 样本、readyz/status 均可观察；记录 query embedding 维度与命中来源 |
| [ ] | 增加延迟基线和回归检查 | 采集 p50/p95、planner、recall、query embedding、vector search、final LLM、async enqueue；形成回归阈值 |

### 9.1 每一步提交前检查项

每个阶段提交前，需要执行以下检查：

1. 新增业务耗时路径是否有 span。
2. 新增异步 job 是否有 queue depth、成功数、失败数、重试数和耗时指标。
3. 新增表是否被 readiness、admin status 或 stats 接口覆盖。
4. 新增错误路径是否写入结构化日志，并带 `request_id`、`trace_id`、`job_id` 或业务关联 id。
5. 新增 Agent 行为是否写入 `agent_trace_events` 或现有专用事实表，并能通过 request/turn/plan/run 关联查询。
6. 新增 RAG/embedding 诊断字段是否能在 progress detail 或 trace API 中查询。
7. Grafana dashboard 是否需要新增或调整 panel。
8. 测试是否覆盖成功、失败、降级和异步不阻塞用户回复四类路径。

### 9.2 建议实施顺序

1. 先建立 `agent_trace_events` 通用事件模型和查询接口，使主 Agent、子 Agent、工具、审批、通知、worker 与 RAG 专项 trace 都能进入同一 waterfall。
2. 补齐 planner、context projection、subagent dispatch、tool execution、approval/recovery、notification 的 span/metrics/trace event。该阶段先覆盖全链路主干。
3. 补齐 embedding client 的 span/metrics，并将 query embedding 与 batch embedding 区分为不同 operation。
4. 新增 recall trace 表与持久化逻辑，把当前 diagnostics 从“接口返回”升级为“每次运行可追溯”。
5. 将惰性 embedding 改为 job enqueue，保留显式管理任务或测试工具中的同步 embedding 能力。
6. 实现 embedding worker，并把 queue depth、coverage、failed/stale 状态接入 readyz、admin status 和 Grafana。
7. 实现 memory topic 与 memory chunk，使 embedding 的主触发从“召回时补齐”转为“主题巩固后异步生成”。
8. 最后实现 progress waterfall 与端到端延迟回归检查，验证主链路规划、子 Agent、工具、召回、模型调用和后台任务均可定位。

## 10. 选型结论

建议采用：

```text
Semantic Chunking
+ Hybrid Retrieval
+ Memory Consolidation
+ Async Embedding Worker
+ Internal Trace Waterfall
+ Optional OpenTelemetry Export
```

不建议仅依赖上下文溢出触发 embedding。主题级记忆化更适合长期 Agent 系统，可以降低成本、减少重复 embedding、提升召回质量，并使用户端响应延迟更可控。

## 11. 观测性部署启用与验收

### 11.1 部署启用策略

当前 Compose 已部署 Prometheus、Loki、Promtail、Tempo、OTel Collector 和 Grafana。API 容器已配置 OTLP endpoint，但 trace 默认关闭。建议采用以下策略：

| 环境 | trace 策略 | 采样建议 | 说明 |
| --- | --- | --- | --- |
| 本地开发 | 默认关闭，排障时手动开启 | 1.0 | 避免本地日志和 trace 噪声过高 |
| 联调环境 | 默认开启 | 0.2 到 1.0 | 用于验证 Agent waterfall、RAG、embedding worker 和日志关联 |
| 生产环境 | 默认开启 | 0.05 到 0.2 | 对错误、慢请求和关键 Agent 行为可在代码侧强制保留业务 trace 表 |

生产环境即使降低 OTel 采样率，也应完整写入内部 `agent_trace_events`、`agent_recall_traces`、`agent_embedding_traces` 和 job 状态表。外部 trace 负责跨服务时序分析，内部 trace 负责业务回放、行为审计和召回质量诊断，两者职责不同。

### 11.2 验收方式

每次实现 Agent 行为、RAG、memory、embedding 或 worker 相关阶段后，需要完成以下验收：

1. `/metrics` 能查询到对应新增指标，且 label 基数保持可控。
2. `/readyz` 或管理状态接口能反映新增表、job 积压、失败数、覆盖率或最近错误。
3. 开启 trace 后，Grafana Tempo 能按日志中的 `trace_id` 查到请求 span。
4. Loki 日志能按 `request_id`、`trace_id`、`job_id` 或 `worker_id` 检索到异步处理过程。
5. Progress detail 能按 request/turn/plan/run 展示 planner、context projection、subagent、tool、approval、recall、final answer、notification 和 async jobs 的 waterfall。
6. Grafana dashboard 能看到 planner、子 Agent、工具、审批、通知、recall、embedding、queue depth、coverage、失败率和外部模型调用趋势。
7. planner 解析失败、工具失败、审批拒绝、通知失败、embedding 上游失败、vector search 失败、relation expansion 失败时，系统能记录明确错误、降级或恢复原因。

### 11.3 观测代码修改边界

后续实现时应遵守以下边界：

1. `internal/metrics/metrics.go` 只定义低基数指标，不加入用户、会话、canonical ref 等高基数字段。
2. `internal/observability` 继续作为通用 trace/logger/context 工具层，不放入 Agent 业务判断。
3. repository 层继续通过 `traceRepositoryOperation` 记录 DB span 与 query metrics。
4. service 层负责业务 span、业务 metrics、trace 表写入和降级原因。
5. handler 层只负责请求级日志、HTTP metrics、响应中的 trace id，不承载 Agent 内部分段统计。
6. worker 入口必须建立独立 span，并在日志中写入稳定 `worker_id`、`job_id`、`attempt`、`status`。
7. Web progress 负责单次任务排障；Grafana 负责整体趋势与告警；两者不应互相替代。
