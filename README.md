# messageFeed

messageFeed 是一个面向个人及小规模用户的信息聚合与可控 AI 助理系统。

系统以 RSS、Atom 和 JSON Feed 为主要信息入口，提供订阅管理、内容同步、时间线阅读、推荐 Feed、阅读状态管理和来源导入；同时通过 Web 与企业微信接入 AI Agent，为信息查询、内容处理、定时任务和受控操作提供统一执行入口。

> 当前状态：项目处于持续开发阶段，信息聚合、Web 阅读、账户体系、企业微信 Agent、长期记忆和可观测性主链路已经建立；金融行情、更多通知通道和推荐算法仍待完善。

## 已实现能力

### 信息源与 Feed

- RSS、Atom、JSON Feed 解析、抓取和条目去重
- 订阅源创建、编辑、手动抓取和后台周期同步
- 推荐源目录、目录搜索和来源健康检查
- Feed URL、批量 URL 和 OPML 导入
- 按发布时间排列的订阅时间线
- 包含候选来源的推荐 Feed 原型
- 已读、收藏、隐藏和阅读历史状态
- 用户级 Feed 展示模式持久化

### Web 应用

- Vue 3、TypeScript、Vite 和 Pinia 构建的响应式界面
- 订阅、推荐、收藏和阅读历史视图
- 条目详情、来源详情及订阅管理
- 移动端滑动、返回、下拉刷新和阅读位置恢复
- 登录、邀请码注册、资料、安全和会话管理
- Owner 用户的邀请、用户和运行配置管理

### AI Agent

- Web 与企业微信双入口的自然语言任务
- 主 Agent 规划、能力路由和分步执行
- Feed 查询、来源查询、网页读取、内容摘要和定时任务等工具
- 写操作权限策略、一次性审批和执行审计
- 任务停止、步骤重试、计划替换和失败恢复
- Web 实时进度、执行证据、运行记录和最终报告
- 企业微信周期进度反馈、最终结果和按钮控制
- 完整会话 transcript、短期上下文和历史查询
- 长期事实记忆、混合召回、向量索引及异步 Embedding 任务

### 工程与运维

- PostgreSQL、GORM、pgvector 和版本化数据库迁移
- 后台来源同步、通知、定时任务和 Embedding Worker
- 健康检查、就绪检查、任务锁和通知幂等
- Prometheus 指标、Loki 日志和 Tempo 链路追踪
- Grafana 预置数据源与 messageFeed Dashboard
- Docker 多阶段构建、Docker Compose 和 Caddy HTTPS 统一入口
- Cloudflare Tunnel 配置及 K3s 部署材料
- Go 单元测试、集成测试、竞态检测和统一验收命令

## 系统结构

```text
Browser / WeChat Work
          |
          v
    Caddy / Cloudflare
          |
    +-----+-----+
    |           |
 Vue Web     Gin API
                |
       +--------+---------+
       |        |         |
   Feed/Item   Agent    Workers
       |        |         |
       +--------+---------+
                |
       PostgreSQL + pgvector
                |
 Prometheus / Loki / Tempo / Grafana
```

后端采用 Handler、Service、Repository 和 Domain 分层。业务数据、Agent 会话、计划、审计、记忆及追踪记录统一持久化至 PostgreSQL；后台 Worker 当前与 API 运行于同一进程，并通过任务锁保留多节点扩展边界。

## 快速启动

### 环境要求

- Docker 与 Docker Compose
- GNU Make
- 本地开发时需要 Go 1.26.1
- 独立构建 Web 时需要 Node.js 24

### Docker Compose

创建本地环境配置：

```bash
cp .env.example .env
```

至少设置 Owner 登录密码：

```dotenv
AUTH_OWNER_USERNAME=aroen
AUTH_OWNER_PASSWORD=<strong-password>
```

启动完整服务：

```bash
make compose-up
```

默认访问地址：

- Web 与统一 API：`https://localhost:8443`
- API 直连：`http://localhost:60001`
- 健康检查：`https://localhost:8443/healthz`
- Grafana：`http://localhost:3000`
- Prometheus：`http://localhost:9090`

本地 HTTPS 使用 Caddy 证书，首次访问时可能需要由浏览器确认本地证书。

常用操作：

```bash
make compose-ps
make compose-logs
make compose-down
```

开发态热更新入口：

```bash
make compose-dev
make compose-dev-watch
```

## 可选集成配置

AI Agent 需要配置 OpenAI 或 OpenAI-compatible 模型：

```dotenv
LLM_PROVIDER=openai
LLM_API_KEY=<api-key>
LLM_MODEL=<model>
```

长期记忆的向量召回需要额外配置 Embedding 模型，并确保维度与数据库迁移定义一致：

```dotenv
EMBEDDING_PROVIDER=<provider>
EMBEDDING_API_KEY=<api-key>
EMBEDDING_BASE_URL=<openai-compatible-base-url>
EMBEDDING_MODEL=<embedding-model>
EMBEDDING_DIMENSION=4096
```

企业微信自建应用接入需要同时设置以下变量：

```dotenv
WECHAT_WORK_CORP_ID=
WECHAT_WORK_AGENT_ID=
WECHAT_WORK_SECRET=
WECHAT_WORK_CALLBACK_TOKEN=
WECHAT_WORK_ENCODING_AES_KEY=
```

完整配置及约束见 [`.env.example`](.env.example)。

## 本地验证

```bash
make verify
```

该命令依次执行格式检查、静态检查、Go 测试和后端构建。其他验证命令包括：

```bash
make test-race
make test-cover
make deps-verify

cd web
npm ci
npm test
npm run build
```

## API 与项目文档

- [OpenAPI 定义](api/openapi.yaml)
- [需求说明](docs/requirements.md)
- [架构说明](docs/architecture.md)
- [实施记录](docs/implementation.md)
- [Agent 当前计划](docs/nowdoit/agent-plan.md)
- [K3s 实施材料](docs/micr-k8s/micr-k8s-implement.md)

运行时基础端点：

- `GET /healthz`：进程存活检查
- `GET /readyz`：数据库、迁移、pgvector 和 Agent 索引检查
- `GET /metrics`：Prometheus 指标
- `GET /api/runtime/node`：当前节点信息

## 当前边界

- 推荐 Feed 仍属于原型实现，尚未形成完整的个性化排序与反馈训练闭环。
- 金融行情采集、指标计算、告警解释和面向用户的管理界面尚未形成完整闭环。
- ntfy 等企业微信以外的通知通道尚未正式接入主链路。
- 当前部署重点为本地单节点与 Cloudflare Tunnel；多节点模式保留了任务锁和幂等边界，但仍需生产级验证。
- AI 与 Embedding 能力依赖外部模型服务；未配置时不影响基础 Feed 服务启动。

## 后续方向

1. 完善推荐质量、兴趣反馈和候选来源治理。
2. 建立金融标的、行情快照、规则告警与 AI 解读闭环。
3. 扩充通知通道并完善用户级通知偏好。
4. 完成 Agent 主从执行重构、长期记忆质量评测和成本治理。
5. 加强生产部署、安全策略、备份恢复与多节点验收。
6. 持续校准 OpenAPI、运行配置和用户界面之间的契约。
