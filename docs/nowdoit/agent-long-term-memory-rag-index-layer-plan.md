# 长期记忆 RAG 索引层建立方案

## 1. 目标与边界

本方案用于补充当前长期记忆系统的索引层建设。核心目标是让超过短期上下文 token 预算、未被自动选入本轮 prompt 的历史事实，仍可以通过结构化索引、全文索引、语义向量索引和关系扩展被召回，并在召回后回表读取原始事实。

需要保持以下边界：

1. 原始事实层不复制。`agent_transcript_entries`、`agent_observations`、`agent_artifacts`、`agent_plans`、`agent_plan_steps`、`agent_run_context_traces` 仍是唯一可信事实来源。
2. `agent_fact_archive_index` 是统一事实索引目录，保存派生摘要、关键词、实体、关系引用、全文检索向量和召回元数据。
3. 向量 embedding 是索引层的派生能力，不应替代原始事实，也不应直接成为模型最终引用证据。
4. 最终进入 ContextBundle 的内容必须区分 `index_hit`、`source_fact` 和 `projection`。模型可引用证据必须来自回表后的 `source_fact`。
5. 用户画像长期记忆仍由 `agent_memory_candidates` 和 `agent_memory_blocks` 管理。RAG 索引层负责事实召回，不直接把历史片段提升为稳定用户偏好。

## 2. 参考项目结论

### 2.1 GraphRAG

`../references/graphrag` 的核心启发是：非结构文本应经过可配置的 indexing pipeline 转换为结构化索引数据，必要时抽取实体、关系和社区摘要。GraphRAG 明确提示 indexing 可能成本较高，因此应小批量开始，并把 chunking、vector store schema 和抽取流程配置化。

对本项目的落地结论：

1. 第一阶段不引入重型图数据库，先使用 `relation_refs_json` 表达轻量事实关系。
2. 对话、观察、计划和 artifact 应按语义单位进入索引，而不是简单按固定长度硬切。
3. 长文本 artifact 可以分 chunk，但每个 chunk 必须保留父级 `canonical_ref`、来源、时间、会话和任务背景。
4. 后续可以在 `relation_refs_json` 之外新增 `agent_fact_relations`，形成更强的 GraphRAG 查询能力。

### 2.2 Haystack

`../references/haystack` 的 hybrid pipeline 示例采用 BM25 retriever、text embedder、embedding retriever、DocumentJoiner 和 similarity ranker。RAG pipeline 示例将 retriever 输出的 documents 同时传递给 prompt builder 和 answer builder，并保留 document metadata。

对本项目的落地结论：

1. 检索链路应显式拆分为结构化过滤、全文召回、向量召回、合并去重、重排和证据投影。
2. 召回结果需要保留 metadata，包括 `canonical_ref`、`fact_type`、`fact_id`、`session_id`、`turn_id`、命中来源和分数。
3. 混合召回不应只依赖向量相似度。错误码、路径、URL、函数名、提交号和精确用户表述更适合全文或结构化检索。

### 2.3 LlamaIndex

`../references/llama_index` 强调 data connector、indices、retrievers、query engine 和 rerank 模块的分层。其 `VectorStoreIndex.from_documents` 和持久化 reload 思路说明：索引是围绕原始数据建立的查询结构，索引可以重建，原始数据不能被索引替代。

对本项目的落地结论：

1. 当前系统应建立内部 `FactIndexer`、`FactRetriever`、`FactSourceResolver` 和 `ContextProjector` 分层。
2. 初始化旧数据时应只 backfill `agent_fact_archive_index` 和 embedding 索引，不改写原始对话和运行事实。
3. 查询入口应接受明确的 recall plan，而不是让模型直接访问数据库。

### 2.4 LangChainGo 与 pgvector_go

`../references/langchaingo/vectorstores` 定义了 `AddDocuments` 和 `SimilaritySearch` 形式的向量存储接口。`pgvector` 实现会创建 vector extension、collection table、embedding table，并通过 advisory lock 避免并发建表问题。`../references/pgvector_go` 提供 `Vector` 类型，用于在 Go 中以 `[]float32` 方式读写 PostgreSQL vector 字段。

对本项目的落地结论：

1. 第一版向量能力建议使用 PostgreSQL + pgvector，减少外部组件复杂度。
2. 不建议把真实向量长期保存在 `agent_fact_archive_index.embedding_json`。该字段可继续保存模型、维度、hash、状态等轻量元数据；真实 vector 应放入独立 embedding 表。
3. embedding 表应使用 `canonical_ref` 关联事实索引，并用 `content_hash` 做幂等和去重。

### 2.5 Bleve

`../references/bleve` 支持全文索引，也支持向量检索和 hybrid scoring，但向量能力依赖 FAISS、Go build tag 和本地索引文件管理。

对本项目的落地结论：

1. 当前项目已有 PostgreSQL GIN `tsvector`，第一阶段不需要引入 Bleve。
2. 如果未来需要更复杂的中文分词、离线本地搜索或多字段相关性调优，可以把 Bleve 作为独立全文索引层评估。

## 3. 当前数据库基础

当前已经具备的事实与记忆相关结构如下：

| 层级 | 表 | 职责 |
| --- | --- | --- |
| 原始事实层 | `agent_transcript_entries` | 保存用户、assistant、tool 等 transcript entry，是历史对话事实的主要来源 |
| 原始事实层 | `agent_observations` | 保存工具执行、输入摘要、输出摘要和错误信息 |
| 原始事实层 | `agent_artifacts` | 保存 Agent 生成或引用的 artifact 元数据与内容定位 |
| 原始事实层 | `agent_plans` | 保存 Agent 计划 |
| 原始事实层 | `agent_plan_steps` | 保存计划步骤和执行状态 |
| 原始事实层 | `agent_run_context_traces` | 保存运行上下文追踪 |
| 事实索引层 | `agent_fact_archive_index` | 保存 canonical ref、摘要、关键词、实体、全文向量、关系引用和召回状态 |
| 用户画像记忆层 | `agent_memory_candidates` | 保存候选长期记忆，等待应用、确认或拒绝 |
| 用户画像记忆层 | `agent_memory_blocks` | 保存已确认或自动应用的稳定长期记忆 |
| 审计层 | `agent_memory_events` | 保存候选生成、应用、拒绝、撤销、使用等事件 |

`agent_fact_archive_index` 当前字段已经覆盖索引目录所需的主体结构：

```text
canonical_ref
fact_type
fact_id
user_id
session_id
turn_id
memory_kind
topics_json
keywords_json
entities_json
summary_for_index
contextual_text
full_text_vector
embedding_json
importance
confidence
source_refs_json
relation_refs_json
index_status
risk_level
access_count
last_accessed_at
metadata_json
```

当前不足是：

1. 旧历史事实尚未完成 backfill，因此 `agent_fact_archive_index` 可能只包含新写入后的数据。
2. `embedding_json` 尚未形成真实向量检索能力。
3. 查询仓储当前以结构化过滤和 `ILIKE` 为主，尚未使用 `full_text_vector @@ query`、pgvector similarity 和重排。
4. 关系扩展目前只保存 `relation_refs_json`，尚未形成独立关系查询能力。

## 4. 索引层目标架构

建议采用以下分层：

```text
原始事实层
  -> FactIndexBuilder
  -> agent_fact_archive_index
  -> agent_fact_embeddings
  -> HybridFactRetriever
  -> FactSourceResolver
  -> ContextProjector
  -> ContextBundle
```

各层职责：

1. `FactIndexBuilder`：从原始事实生成 `canonical_ref`、摘要、关键词、实体、上下文化文本、关系引用和索引元数据。
2. `agent_fact_archive_index`：统一事实索引目录，支持结构化过滤和全文检索。
3. `agent_fact_embeddings`：保存真实向量，支持语义召回。
4. `HybridFactRetriever`：执行结构化过滤、全文召回、向量召回、关系扩展、合并去重和重排。
5. `FactSourceResolver`：根据 `canonical_ref` 和 `fact_type/fact_id` 回表读取原始事实。
6. `ContextProjector`：在 token 预算内把回表事实压缩为本轮可见证据。

## 5. 数据库新增方案

### 5.1 保留 `agent_fact_archive_index`

`agent_fact_archive_index` 不应被替换。它承担事实目录职责，是所有召回模式的统一入口。

建议对现有表增加以下约束性使用约定：

1. `embedding_json` 只保存 embedding 元信息，不保存主向量。
2. `contextual_text` 用于全文和向量输入的文本基础，必须包含事实自身内容与必要上下文。
3. `source_refs_json` 指向原始事实定位。
4. `relation_refs_json` 指向相关事实、turn、session、plan、artifact 或 memory candidate。
5. `metadata_json` 记录索引策略版本、chunk 信息、语言、模型抽取状态和错误信息。

### 5.2 新增 `agent_fact_embeddings`

建议新增独立表保存向量：

```sql
CREATE TABLE IF NOT EXISTS agent_fact_embeddings (
    id BIGSERIAL PRIMARY KEY,
    canonical_ref VARCHAR(160) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    embedding_model VARCHAR(128) NOT NULL,
    embedding_dimension INTEGER NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    embedding vector(1536) NOT NULL,
    embedding_status VARCHAR(16) NOT NULL DEFAULT 'ready',
    error_message TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_fact_embeddings_ref_model_hash
        UNIQUE (canonical_ref, embedding_model, content_hash),
    CONSTRAINT chk_agent_fact_embeddings_status
        CHECK (embedding_status IN ('ready', 'pending', 'failed', 'archived'))
);
```

索引建议：

```sql
CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_user_model
    ON agent_fact_embeddings(user_id, embedding_model, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_ref
    ON agent_fact_embeddings(canonical_ref);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_vector_hnsw
    ON agent_fact_embeddings USING hnsw (embedding vector_cosine_ops);
```

说明：

1. `vector(1536)` 需要按实际 embedding 模型维度确认。如果使用其他维度，应在建表时固定维度，或按模型建立不同 embedding 表。
2. `content_hash` 使用 `contextual_text` 规范化后的 SHA-256，用于判断是否需要重新生成 embedding。
3. 不在 `agent_fact_archive_index` 中直接放 vector，可以避免目录表膨胀，并便于后续多模型 embedding 并存。
4. pgvector extension 的创建应在 migration 中使用 `CREATE EXTENSION IF NOT EXISTS vector`。如果担心并发 migration，可参考 LangChainGo pgvector 的 advisory lock 思路。

### 5.3 可选新增 `agent_fact_index_jobs`

建议新增索引任务表，用于 backfill、embedding 补齐、失败重试和重建审计：

```sql
CREATE TABLE IF NOT EXISTS agent_fact_index_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_type VARCHAR(32) NOT NULL,
    scope_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    cursor_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    total_count INTEGER NOT NULL DEFAULT 0,
    processed_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_fact_index_jobs_type
        CHECK (job_type IN ('backfill_fact_index', 'embed_fact_index', 'rebuild_fact_index')),
    CONSTRAINT chk_agent_fact_index_jobs_status
        CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled'))
);
```

该表不是第一阶段召回的必需条件，但有利于生产环境观察和断点续跑。

### 5.4 可选新增 `agent_fact_relations`

第一阶段可以继续使用 `relation_refs_json`。当关系查询复杂度上升时，再新增独立关系表：

```sql
CREATE TABLE IF NOT EXISTS agent_fact_relations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_ref VARCHAR(160) NOT NULL,
    to_ref VARCHAR(160) NOT NULL,
    relation_type VARCHAR(32) NOT NULL,
    weight NUMERIC(6,4) NOT NULL DEFAULT 1,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_fact_relations_edge UNIQUE (from_ref, to_ref, relation_type)
);
```

可支持的关系类型：

```text
same_turn
same_session
evidence_for
derived_from
plan_contains
artifact_from
observation_from
preference_evidence
semantic_neighbor
```

## 6. canonical_ref 与 chunk_ref

索引层必须使用稳定、可回表、可重建的引用格式。建议格式如下：

| fact_type | canonical_ref |
| --- | --- |
| transcript | `transcript:{id}` |
| observation | `observation:{id}` |
| artifact | `artifact:{id}` |
| plan | `plan:{id}` |
| plan_step | `plan_step:{id}` |
| run_trace | `run_trace:{id}` |

长文本 chunk 使用派生引用：

```text
artifact:{id}#chunk:{n}
transcript:{id}#chunk:{n}
observation:{id}#chunk:{n}
```

chunk 规则：

1. `canonical_ref` 可以保存 chunk ref，但 `metadata_json.parent_ref` 必须保存父级原始事实 ref。
2. `fact_type/fact_id` 必须仍然指向父级原始事实，保证回表路径稳定。
3. `source_refs_json` 必须包含父级 ref、session、turn 和 chunk ref。
4. chunk ref 不需要使用复杂随机 ID。它是派生索引，不是原始事实主键，应保持可重建。

## 7. 语义单位截断与 chunk 策略

本项目不应使用固定 8、12、20 条动态窗口策略，也不应对 token 超限内容做机械硬截断。新的 token 方案应以语义单位整体截断为原则。

索引层 chunk 建议：

1. 普通对话：优先以单条 transcript entry 或同一 turn 内的 user/assistant pair 为语义单位。
2. 多轮追问：不需要单独特殊处理，现有上下文中连续 transcript 和 turn 关系已经能表达追问关系；索引层只需保留 `session_id`、`turn_id`、`relation_refs_json` 和相邻 turn 引用。
3. tool observation：通常按一次工具调用的输入摘要、输出摘要、错误信息作为一个语义单位。
4. plan 与 step：通常不切分，直接生成摘要化索引投影。
5. artifact 长文本：按 token 预算切 chunk，但必须在句子、段落、列表项或代码块边界处截断，允许少量 overlap。
6. 代码、错误日志和路径：优先保持完整块，不把函数名、错误码、路径从上下文中切开。

建议参数：

| 内容类型 | chunk 上限 | overlap | 截断边界 |
| --- | --- | --- | --- |
| transcript | 600-1200 tokens | 同 turn 相邻引用 | entry 或 turn |
| observation | 800-1500 tokens | 100-200 tokens | 段落、日志块 |
| artifact | 1200-2000 tokens | 150-300 tokens | 标题、段落、代码块 |
| plan/step | 不切分 | 无 | 单条记录 |

## 8. contextual_text 生成方案

仅 embedding 原文短句容易丢失上下文。例如“按这个方案继续”在脱离 turn、plan 和上轮目标后没有检索价值。因此 `contextual_text` 应包含事实本身与检索所需上下文。

建议模板：

```text
time: {created_at}
user_id: {user_id}
session_id: {session_id}
turn_id: {turn_id}
fact_type: {fact_type}
role/source: {role_or_source}
task_context: {plan_title_or_recent_goal}
summary: {summary_for_index}
content:
{normalized_content}
source_refs: {source_refs}
relations: {relation_refs}
```

生成要求：

1. 保留用户原始表达中的关键限定词、路径、文件名、命令、错误信息和业务对象。
2. 对较长内容生成摘要，但摘要不能覆盖原文定位。
3. 对高风险偏好、凭据、敏感身份信息等内容设置 `risk_level`，避免自动注入 prompt。
4. `metadata_json.indexer_version` 必须记录索引策略版本，方便后续全量重建。

## 9. 初始化与索引建立流程

初始化只应生成索引层，不应复制原始事实层，也不应把旧历史自动提升为稳定用户画像。

### 9.1 扫描范围

按以下顺序扫描：

1. `agent_transcript_entries`
2. `agent_observations`
3. `agent_artifacts`
4. `agent_plans`
5. `agent_plan_steps`
6. `agent_run_context_traces`

每类数据使用主键升序或创建时间升序游标，按 batch 处理。

### 9.2 单条事实索引流程

```text
读取原始事实
  -> 生成 canonical_ref
  -> 判断是否需要 chunk
  -> 生成 summary_for_index
  -> 抽取 keywords/topics/entities
  -> 生成 contextual_text
  -> 生成 source_refs_json
  -> 生成 relation_refs_json
  -> 计算 importance/confidence/risk_level
  -> upsert agent_fact_archive_index
  -> 计算 content_hash
  -> 若 embedding 不存在或 hash 改变，则进入 embedding 任务
```

### 9.3 精确查找能力

对话内容都是文本，因此精确查找不能只依赖 embedding。建议同时建立三种能力：

1. 结构化字段：按 `user_id`、`session_id`、`turn_id`、`fact_type`、`memory_kind`、时间范围和风险等级过滤。
2. 全文索引：使用 `full_text_vector` 查询精确词、路径、错误码、函数名、URL、中文短语和用户明确表达。
3. 原文回表校验：命中索引后，通过 `canonical_ref` 回表读取原始事实，必要时在原文字段上做二次 `ILIKE` 或片段定位。

查询示例：

```sql
SELECT canonical_ref, fact_type, fact_id, ts_rank(full_text_vector, plainto_tsquery('simple', $2)) AS rank
FROM agent_fact_archive_index
WHERE user_id = $1
  AND index_status = 'ready'
  AND full_text_vector @@ plainto_tsquery('simple', $2)
ORDER BY rank DESC, importance DESC, updated_at DESC
LIMIT 50;
```

### 9.4 embedding 生成流程

embedding 应异步生成，失败不得影响原始事实和基础索引。

```text
选择待 embedding 的 index rows
  -> 读取 contextual_text
  -> 规范化文本
  -> 计算 content_hash
  -> 批量调用 embedding provider
  -> 写入 agent_fact_embeddings
  -> 更新 agent_fact_archive_index.embedding_json
```

`embedding_json` 建议内容：

```json
{
  "provider": "openai",
  "model": "text-embedding-3-small",
  "dimension": 1536,
  "content_hash": "sha256:...",
  "status": "ready",
  "embedded_at": "2026-06-30T00:00:00Z"
}
```

### 9.5 幂等与重建

1. `agent_fact_archive_index` 使用 `canonical_ref` upsert。
2. `agent_fact_embeddings` 使用 `(canonical_ref, embedding_model, content_hash)` 去重。
3. 索引策略升级时增加 `metadata_json.indexer_version`，通过 job 重建受影响索引。
4. 原始事实删除或归档时，索引行可标记为 `archived`，但不能删除用户未要求删除的原始数据。

## 10. RAG 混合召回方案

### 10.1 查询入口

主 Agent 不直接拼 SQL。模型应输出受控 recall plan，后端执行。

```json
{
  "mode": "hybrid",
  "query": "用户问到的目标问题",
  "user_id": 123,
  "session_id": 456,
  "time_range": {
    "after": "optional",
    "before": "optional"
  },
  "fact_types": ["transcript", "observation", "artifact"],
  "memory_kinds": ["preference", "task", "fact"],
  "limit": 20,
  "needs_source_fact": true
}
```

支持模式：

| mode | 用途 |
| --- | --- |
| `search` | 精确词、路径、错误码、URL、命令、文件名 |
| `semantic` | 近义表达、意图相似、概念性问题 |
| `hybrid` | 默认模式，同时使用全文和向量 |
| `time_range` | 查找指定时期上下文 |
| `earliest` | 查找最早来源、首次决定 |
| `latest` | 查找最近状态、最新偏好或最后执行结果 |

### 10.2 检索执行顺序

```text
recall_plan
  -> 权限与 user_id 校验
  -> 结构化过滤候选集合
  -> full_text_vector 召回
  -> pgvector 语义召回
  -> relation_refs 扩展
  -> 合并去重
  -> rerank
  -> canonical_ref 回表
  -> ContextBundle 投影
```

结构化过滤必须先于向量召回，至少包括：

1. `user_id`
2. `index_status = ready`
3. session 范围或跨 session 策略
4. fact_type 白名单
5. risk_level 与权限限制
6. 时间范围

### 10.3 向量查询示例

```sql
WITH vector_hits AS (
    SELECT
        e.canonical_ref,
        1 - (e.embedding <=> $2) AS vector_score
    FROM agent_fact_embeddings e
    JOIN agent_fact_archive_index i
      ON i.canonical_ref = e.canonical_ref
    WHERE e.user_id = $1
      AND i.user_id = $1
      AND i.index_status = 'ready'
      AND e.embedding_model = $3
      AND e.embedding_status = 'ready'
    ORDER BY e.embedding <=> $2
    LIMIT 80
)
SELECT i.*, v.vector_score
FROM vector_hits v
JOIN agent_fact_archive_index i
  ON i.canonical_ref = v.canonical_ref
ORDER BY v.vector_score DESC, i.importance DESC
LIMIT 20;
```

### 10.4 合并与评分

建议第一版使用规则化分数，后续再接专用 reranker。

```text
final_score =
  0.25 * structured_score +
  0.25 * fulltext_score +
  0.30 * vector_score +
  0.10 * importance_score +
  0.05 * recency_score +
  0.05 * relation_score
```

说明：

1. `structured_score` 来自 session、turn、fact_type、memory_kind、时间范围匹配程度。
2. `fulltext_score` 来自 `ts_rank` 或后续 BM25。
3. `vector_score` 来自 pgvector cosine similarity。
4. `importance_score` 来自索引行重要性。
5. `recency_score` 用于最新状态类问题，不应在所有问题中绝对优先。
6. `relation_score` 来自 same turn、same plan、same artifact、evidence refs 等关系扩展。

### 10.5 回表与证据投影

召回阶段只产生候选，不直接产生最终上下文。必须回表：

```text
index_hit:
  canonical_ref: transcript:100
  reason: fulltext + vector
  score: 0.82

source_fact:
  table: agent_transcript_entries
  id: 100
  role: user
  content: 原始文本
  created_at: ...

projection:
  text: 本轮 prompt 可见的压缩证据
  token_estimate: 180
```

投影原则：

1. 每条投影必须保留 `canonical_ref`。
2. 对短事实可直接投影原文。
3. 对长 artifact 只投影相关片段和定位信息。
4. 对低置信度或高风险事实，只作为候选证据，不自动转化为指令。
5. ContextBundle 应在 token 预算下按 final_score、任务相关性和证据多样性选入。

## 11. 用户画像长期记忆与 RAG 的关系

用户画像长期记忆已经由 `agent_memory_candidates` 和 `agent_memory_blocks` 表达。RAG 索引层与其关系如下：

1. `agent_fact_archive_index` 负责召回历史事实和证据。
2. `agent_memory_candidates` 负责把可能稳定的偏好、任务结论、决策或事实提出为候选。
3. `agent_memory_blocks` 只保存经过确认或策略允许自动应用的稳定记忆。
4. 历史事实 backfill 不应直接写入 `agent_memory_blocks`，否则会把普通上下文错误提升为用户画像。
5. 当 RAG 召回到多条证据支持某项稳定偏好时，可以生成 candidate，但仍需经过风险、置信度和用户确认策略。

## 12. 主 Agent 与子 Agent 的后续接入

主 Agent 应负责判断何时需要召回，而后端负责执行受控召回。子 Agent 不应直接读取数据库，应通过统一 memory/retrieval tool 获取受限 ContextBundle。

建议职责：

1. 主 Agent：根据用户问题、当前短期上下文和任务状态生成 recall plan，判断证据是否充分。
2. 子 Agent：接收主 Agent 分配的任务和裁剪后的 ContextBundle，只在需要时请求补充召回。
3. 后端 Retrieval Service：执行索引查询、向量查询、回表、重排和投影。
4. 审计层：记录 recall plan、命中、回表事实、投影结果和最终使用情况。

模型判断是否查看长期记忆的条件：

1. 用户询问“之前”“上次”“刚才”“历史”“已实现”“测试结果”等跨轮信息。
2. 当前问题依赖文件路径、配置、部署状态、数据库状态或上轮执行结果，但短期上下文不足。
3. 用户要求核实事实、对比进度、继续未完成步骤。
4. 当前任务涉及用户偏好、长期约束或此前确认过的工作方式。
5. 模型对关键事实不确定，且该事实可能存在于历史对话或执行记录中。

## 13. 测试与验收

### 13.1 初始化验收

1. `agent_fact_archive_index` 行数应与可索引原始事实数量基本一致；长 artifact 允许多 chunk。
2. 每条索引行都能通过 `canonical_ref` 回表。
3. `source_refs_json` 至少包含父级事实 ref；有 session 和 turn 时必须包含对应引用。
4. 失败行记录 `index_status=failed` 和错误信息，不影响后续 batch。

### 13.2 精确查询验收

1. 用文件路径、错误码、函数名、URL 查询，应优先命中全文索引。
2. 用用户原话短语查询，应能回表到对应 transcript。
3. 用时间范围查询，应返回指定 session 或 turn 附近事实。

### 13.3 语义查询验收

1. 用近义表达查询，应命中含义相关但字面不同的历史事实。
2. 用“用户偏好是什么”查询，应优先返回 `agent_memory_blocks`，并能通过 evidence refs 回查事实索引。
3. 用“上次部署结果”查询，应召回相关 observation、transcript 和 artifact。

### 13.4 超 token 场景验收

1. 人为构造超过短期上下文预算的对话历史。
2. 确认超出预算的旧事实不会直接进入 prompt。
3. 用户追问旧事实时，系统通过索引召回并回表。
4. ContextBundle 只投影 top evidence，不对语义单位做硬截断。

### 13.5 安全与风险验收

1. 高风险候选不得自动进入稳定记忆。
2. 不同 user_id 的索引和 embedding 不能交叉召回。
3. 已归档或失败索引不得进入 prompt。
4. 召回审计可追踪 query、index_hit、source_fact 和 projection。

## 14. 实施步骤清单

后续实现时逐项勾选：

- [ ] 核实生产 PostgreSQL 是否已安装 pgvector extension，确认可用版本和向量维度上限。
- [x] 新增 migration：创建 `agent_fact_embeddings`，必要时创建 `agent_fact_index_jobs`。
- [x] 调整 `agent_fact_archive_index.embedding_json` 使用约定，只保存 embedding 元信息。
- [x] 新增 embedding provider 抽象，支持批量 embedding、维度校验、失败重试和速率限制。
- [x] 新增 `FactIndexBuilder`，统一从 transcript、observation、artifact、plan、step 生成索引行。
- [x] 新增 backfill service/job，仅初始化索引层，不复制原始事实，不直接写稳定记忆。
- [x] 新增 `HybridFactRetriever`，支持结构化过滤、全文检索、向量检索、关系扩展、合并去重和规则化 rerank。
- [x] 调整 `ResolveAgentFactSources`，确保 chunk ref 能回表到父级事实并定位片段。
- [x] 接入 ContextBundle 投影，显式区分 `index_hit`、`source_fact` 和 `projection`。
- [x] 将主 Agent recall plan 接入统一 retrieval service，子 Agent 只能通过受控工具请求补充上下文。
- [ ] 增加 Web/API 观测能力，展示 recall query、命中来源、分数、回表事实和投影内容。
- [ ] 完成初始化、精确查询、语义查询、超 token 场景、用户隔离和风险控制验收。
- [ ] 每完成一个可独立验证阶段后提交并推送。
- [ ] 全部实现完成后重新部署上线，并核实 readyz、migration version、索引行数和召回链路。

## 15. 第一阶段建议范围

第一阶段建议限定为 PostgreSQL 内混合检索：

1. 完成 `agent_fact_archive_index` 旧数据 backfill。
2. 接入 pgvector 和 `agent_fact_embeddings`。
3. 使用 `full_text_vector` 替代当前主要依赖 `ILIKE` 的查询。
4. 实现 hybrid recall 和回表证据链。
5. 暂不引入外部向量库、重型图数据库和复杂社区摘要。

这样可以在当前数据库和 Go 服务结构内完成闭环，同时保留后续向 GraphRAG、专用 reranker、Bleve 或外部向量库演进的空间。
