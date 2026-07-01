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

### 5.3 新增 Recall Trace

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

### 5.4 新增 Embedding Trace

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

### 5.5 单次请求 Waterfall

前端或内部 API 可以按 `request_id` / `turn_id` 聚合：

```text
inbound_message
  -> transcript_write
  -> context_projection
  -> planner_call
  -> recall
      -> fulltext
      -> query_embedding
      -> vector_search
      -> relation_expand
      -> projection
  -> tool_calls
  -> final_llm_call
  -> transcript_reply_write
  -> async_jobs_enqueued
```

### 5.6 与 LangSmith / Cozeloop 的关系

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
  -> agent_runs
  -> agent_run_context_traces
  -> agent_recall_traces
  -> agent_embedding_traces
  -> agent_observations
  -> agent_artifacts
  -> agent_audit_logs
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

- [ ] 新增 `agent_memory_topics`、`agent_memory_chunks` migration。
- [ ] 新增 `memory_chunk` fact type。
- [ ] 实现规则版 `TopicTracker`。
- [ ] 实现 `MemoryConsolidationPolicy`。
- [ ] 实现 `MemoryChunkBuilder`。
- [ ] 将 memory chunk 写入 `agent_fact_archive_index`。
- [ ] 将惰性 embedding 改为投递 `embed_fact_index` job。
- [ ] 实现 embedding worker claim、执行、重试和状态更新。
- [ ] 新增 `agent_recall_traces`。
- [ ] 新增 `agent_embedding_traces`。
- [ ] 将 recall diagnostics 持久化到 recall trace。
- [ ] 在 progress detail 中展示 recall waterfall。
- [ ] 扩展 fact index stats 覆盖率指标。
- [ ] 增加真实模型端到端测试用例。
- [ ] 增加延迟基线和回归检查。

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

