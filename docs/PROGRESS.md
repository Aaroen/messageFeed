# messageFeed 项目进度跟踪

本文档记录 messageFeed 项目各阶段的实施进度。

**最后更新**：2026-06-13

---

## 📊 实施进度总览

| 阶段 | 名称 | 状态 | 完成度 | 开始日期 | 完成日期 |
|------|------|------|--------|----------|----------|
| 阶段一 | 基础设施搭建 | ✅ 完成 | 100% | 2026-06-12 | 2026-06-13 |
| 阶段二 | 订阅源与 Feed 闭环 | 🚧 进行中 | 0% | 2026-06-13 | - |
| 阶段三 | 源目录与导入 | ⏸️ 未开始 | 0% | - | - |
| 阶段四 | 自动化、兴趣规则与推荐 | ⏸️ 未开始 | 0% | - | - |
| 阶段五 | AI 摘要与通知 | ⏸️ 未开始 | 0% | - | - |
| 阶段六 | 自然语言设置控制 | ⏸️ 未开始 | 0% | - | - |
| 阶段七 | 金融市场监控 | ⏸️ 未开始 | 0% | - | - |

### 图例说明

- ✅ 已完成
- 🚧 进行中
- ⏸️ 未开始
- ❌ 已取消
- ⚠️ 有问题需要解决

---

## 阶段一：基础设施搭建 ✅

**状态**：✅ 已完成  
**完成时间**：2026-06-13  
**完成度**：100%  
**负责人**：Claude Opus 4.8 + 用户

### 实施进度清单

#### 项目骨架 ✅
- [x] 初始化 Go 模块（go.mod, go.sum）
- [x] 创建目录结构（cmd/api, internal/*, docs, migrations）
- [x] 配置 .gitignore

#### 配置系统 ✅
- [x] 实现 internal/config 模块
- [x] 环境变量解析（DATABASE_URL, BIND_ADDR, PUBLIC_BASE_URL）
- [x] 支持 APP_NODE_ID, DEPLOYMENT_MODE, LOG_LEVEL
- [x] 配置校验和默认值
- [x] 配置模块单元测试

#### HTTP 服务 ✅
- [x] 实现基础 HTTP 服务器（net/http）
- [x] 实现 GET / （服务信息）
- [x] 实现 GET /healthz（存活检查）
- [x] 实现 GET /readyz（就绪检查，含数据库）
- [x] 实现 GET /metrics（Prometheus 指标）
- [x] 实现 GET /api/runtime/node（节点信息）
- [x] 请求日志中间件

#### 数据库集成 ✅
- [x] 创建 internal/db 模块
- [x] 实现连接池配置（MaxOpenConns, MaxIdleConns, ConnMaxLifetime）
- [x] 实现健康检查（Ping, CheckHealth）
- [x] 创建 migrations/000001_init_schema.up.sql
- [x] 创建 migrations/000001_init_schema.down.sql
- [x] users 表创建和默认用户插入
- [x] Docker Compose 自动执行迁移

#### 可观测性 ✅
- [x] 集成 log/slog 结构化日志
- [x] 创建 internal/metrics 模块
- [x] 定义 HTTP 请求指标（requests_total, request_duration_seconds）
- [x] 定义数据库连接池指标（database_connections）
- [x] 定义应用级指标（预留）
- [x] Prometheus 指标采集和暴露

#### 构建与部署 ✅
- [x] 创建 Makefile（fmt, vet, test, build, docker-build）
- [x] 创建 Dockerfile（多阶段构建，Alpine 基础镜像）
- [x] 创建 docker-compose.yml（PostgreSQL + API 服务）
- [x] 创建 .env.example（完整环境变量示例）
- [x] make verify 完整验收流程
- [x] make test 单元测试
- [x] make build 编译成功

#### 文档 ✅
- [x] 需求文档（docs/requirements.md）
- [x] 架构文档（docs/architecture.md）
- [x] 实施文档（docs/implementation.md）
- [x] 阶段一验收报告（PHASE_ONE_REPORT.md）
- [x] 前后端架构章节（第 12 章）

### 验收结果 ✅

**验收日期**：2026-06-13  
**验收状态**：全部通过

- [x] docker-compose up -d 一键启动
- [x] /healthz 返回成功
- [x] /readyz 包含数据库检查项且状态为 ready
- [x] /metrics 暴露 Prometheus 指标
- [x] /api/runtime/node 返回节点信息
- [x] make verify 全部通过
- [x] 数据库自动迁移成功
- [x] 容器健康检查正常
- [x] users 表存在且有默认用户

### 交付物

- ✅ 可运行的 Go API 服务（23MB 二进制）
- ✅ PostgreSQL 数据库容器
- ✅ 完整的构建和部署脚本
- ✅ 全套技术文档
- ✅ Docker 镜像（messagefeed:latest）

---

## 阶段二：订阅源与 Feed 闭环 🚧

**状态**：🚧 进行中  
**开始时间**：2026-06-13  
**预计完成**：TBD  
**完成度**：0%

### 实施进度清单

#### 数据库设计 ⏸️
- [ ] 创建 migrations/000002_add_sources_items.up.sql
- [ ] 创建 migrations/000002_add_sources_items.down.sql
- [ ] 定义 sources 表（订阅源）
  - [ ] id, user_id, name, type, url, normalized_url
  - [ ] fetch_interval, enabled, tags, weight
  - [ ] last_fetched_at, last_status, created_at, updated_at
- [ ] 定义 items 表（Feed 条目）
  - [ ] id, source_id, title, url, normalized_url
  - [ ] guid, summary, content_snippet, author
  - [ ] published_at, fetched_at, content_hash
- [ ] 定义 user_item_states 表（阅读状态）
  - [ ] id, user_id, item_id, is_read, is_favorited, is_hidden
  - [ ] read_at, favorited_at, hidden_at, updated_at
- [ ] 添加唯一约束（sources: user_id + normalized_url）
- [ ] 添加唯一约束（items: source_id + normalized_url）
- [ ] 添加索引（items: published_at, fetched_at, source_id）
- [ ] 执行迁移并验证

#### 领域模型 ⏸️
- [ ] 创建 internal/domain/source.go（订阅源实体）
- [ ] 创建 internal/domain/item.go（条目实体）
- [ ] 创建 internal/domain/user_item_state.go（阅读状态）
- [ ] 定义 SourceType 枚举（RSS, Atom, JSONFeed）
- [ ] 定义 FetchStatus 枚举（Success, Failed, Pending）
- [ ] 定义领域错误（ErrSourceNotFound, ErrItemNotFound）

#### Repository 层 ⏸️
- [ ] 创建 internal/repository/source_repository.go
  - [ ] Create(source) - 创建订阅源
  - [ ] Get(id) - 获取单个源
  - [ ] List(userID, filters) - 列表查询
  - [ ] Update(id, updates) - 更新
  - [ ] Delete(id) - 删除
  - [ ] UpdateLastFetched(id, status, time) - 更新抓取状态
- [ ] 创建 internal/repository/item_repository.go
  - [ ] Create(item) - 创建条目
  - [ ] BatchCreate(items) - 批量创建
  - [ ] Get(id) - 获取单个条目
  - [ ] List(filters, page, pageSize) - 列表（含分页、排序）
  - [ ] GetByNormalizedURL(sourceID, url) - URL 去重查询
- [ ] 创建 internal/repository/user_item_state_repository.go
  - [ ] MarkRead(userID, itemID) - 标记已读
  - [ ] Favorite(userID, itemID) - 收藏
  - [ ] Hide(userID, itemID) - 隐藏
  - [ ] GetState(userID, itemID) - 查询状态

#### Fetcher 模块 ⏸️
- [ ] 创建 internal/fetcher/fetcher.go
- [ ] 集成 gofeed 库（go get github.com/mmcdole/gofeed）
- [ ] 实现 HTTP 抓取
  - [ ] 设置超时（30秒）
  - [ ] 处理重定向（最多5次）
  - [ ] 限制内容大小（10MB）
  - [ ] User-Agent 设置
- [ ] 实现 URL 规范化
  - [ ] 移除查询参数中的追踪参数
  - [ ] 统一 scheme（http/https）
  - [ ] 移除 URL fragment
- [ ] 实现 Feed 解析
  - [ ] 解析 RSS 2.0
  - [ ] 解析 Atom 1.0
  - [ ] 解析 JSON Feed
- [ ] 错误处理
  - [ ] 异常编码处理（非 UTF-8）
  - [ ] 异常 MIME 类型
  - [ ] 空 feed 处理
  - [ ] 重复 GUID 处理

#### Service 层 ⏸️
- [ ] 创建 internal/service/source_service.go
  - [ ] CreateSource(name, url, type) - 创建订阅源
  - [ ] ListSources(userID, filters) - 列表查询
  - [ ] UpdateSource(id, updates) - 更新
  - [ ] DeleteSource(id) - 删除
  - [ ] TriggerFetch(id) - 手动触发抓取
- [ ] 创建 internal/service/feed_service.go
  - [ ] FetchAndStore(sourceID) - 抓取并存储
  - [ ] 去重逻辑（检查 normalized_url）
  - [ ] 批量入库优化
  - [ ] 错误记录和状态更新
- [ ] 创建 internal/service/timeline_service.go
  - [ ] GetTimeline(userID, page, pageSize) - 时间线查询
  - [ ] 排序逻辑（published_at desc, 兜底 fetched_at）
  - [ ] 过滤逻辑（来源、标签）
- [ ] 创建 internal/service/item_service.go
  - [ ] MarkRead(userID, itemID) - 标记已读
  - [ ] Favorite(userID, itemID) - 收藏
  - [ ] Hide(userID, itemID) - 隐藏

#### Handler 层 (后端 API) ⏸️
- [ ] 创建 internal/handler/source_handler.go
  - [ ] POST /api/v1/sources - 创建订阅源
  - [ ] GET /api/v1/sources - 获取列表
  - [ ] GET /api/v1/sources/:id - 获取详情
  - [ ] PATCH /api/v1/sources/:id - 更新
  - [ ] DELETE /api/v1/sources/:id - 删除
  - [ ] POST /api/v1/sources/:id/fetch - 手动抓取
- [ ] 创建 internal/handler/item_handler.go
  - [ ] GET /api/v1/items - 获取条目列表
  - [ ] GET /api/v1/items/:id - 获取详情
  - [ ] POST /api/v1/items/:id/mark-read - 标记已读
  - [ ] POST /api/v1/items/:id/favorite - 收藏
  - [ ] POST /api/v1/items/:id/hide - 隐藏
- [ ] 创建 internal/handler/feed_handler.go
  - [ ] GET /api/v1/feed/timeline - 时间线模式
- [ ] 统一响应格式（code, message, data, request_id）
- [ ] 统一错误处理

#### OpenAPI 文档 ⏸️
- [ ] 安装 swaggo/swag（go get -u github.com/swaggo/swag/cmd/swag）
- [ ] 添加 OpenAPI 注解到所有 handler
- [ ] 生成 OpenAPI 3.0 规范文件（swag init）
- [ ] 配置 Swagger UI 路由（GET /swagger/*）
- [ ] 验证 API 文档完整性

#### 前端初始化 ⏸️
- [ ] 创建 web/ 目录
- [ ] 初始化 Vue 3 + Vite 项目（npm create vite@latest）
- [ ] 安装依赖
  - [ ] npm install @arco-design/web-vue
  - [ ] npm install vue-router pinia
  - [ ] npm install axios
  - [ ] npm install -D typescript @types/node
- [ ] 配置 vite.config.ts（代理、别名）
- [ ] 配置 tsconfig.json
- [ ] 配置 Arco Design Vue（按需引入）
- [ ] 配置 Vue Router
- [ ] 配置 Pinia store
- [ ] 配置 Axios（baseURL, 拦截器）

#### 前端页面 ⏸️
- [ ] 创建主布局（Layout.vue）
  - [ ] 顶部导航
  - [ ] 侧边栏菜单
  - [ ] 内容区域
- [ ] 创建订阅源管理页（/sources）
  - [ ] SourceList 组件（列表展示）
  - [ ] SourceForm 组件（新增/编辑表单）
  - [ ] 删除确认对话框
  - [ ] 手动抓取按钮和状态提示
- [ ] 创建时间线页面（/timeline）
  - [ ] FeedTimeline 组件（瀑布流布局）
  - [ ] ItemCard 组件（卡片式展示）
  - [ ] 分页组件
  - [ ] 来源过滤器
  - [ ] 操作按钮组（已读/收藏/隐藏）
- [ ] 创建条目详情页（/items/:id）
  - [ ] 全文展示
  - [ ] 操作栏
  - [ ] 返回按钮

#### 前端状态管理 ⏸️
- [ ] 创建 sourceStore（订阅源状态）
  - [ ] sources 列表
  - [ ] fetchSources() action
  - [ ] createSource() action
  - [ ] updateSource() action
  - [ ] deleteSource() action
- [ ] 创建 feedStore（Feed 状态）
  - [ ] items 列表
  - [ ] currentPage
  - [ ] fetchTimeline() action
  - [ ] markRead() action
  - [ ] favorite() action
  - [ ] hide() action

#### 前后端集成 ⏸️
- [ ] 配置 Go API CORS（允许 localhost:5173）
- [ ] 前端 Axios 配置 baseURL（http://localhost:60001）
- [ ] 前后端联调测试
  - [ ] 创建订阅源流程
  - [ ] 手动抓取流程
  - [ ] 时间线浏览流程
  - [ ] 标记操作流程
- [ ] 错误处理优化
- [ ] 加载状态和提示优化

#### 测试 ⏸️
- [ ] Repository 层单元测试
- [ ] Service 层单元测试
- [ ] Handler 层单元测试
- [ ] Fetcher 模块单元测试
- [ ] 端到端集成测试

### 验收标准

#### 后端 API
- [ ] 可以通过 API 创建订阅源
- [ ] 可以手动触发抓取，返回抓取结果
- [ ] 重复抓取不会重复入库（验证去重逻辑）
- [ ] 可以按时间倒序查询条目
- [ ] 可以标记已读、收藏、隐藏
- [ ] API 文档在 Swagger UI 中可访问
- [ ] 所有 API 返回统一格式

#### Web 前端
- [ ] 可以在 Web 界面管理订阅源（增删改查）
- [ ] 可以查看时间线模式
- [ ] 可以在界面上执行标记操作
- [ ] 界面响应式，支持移动端浏览
- [ ] 错误提示友好

#### 整体
- [ ] 前后端完整联调通过
- [ ] 错误处理完善
- [ ] 日志记录完整
- [ ] make verify 通过

---

## 更新日志

### 2026-06-13
- ✅ 完成阶段一全部任务
- ✅ 通过阶段一完整验收
- ✅ 创建进度跟踪文档
- 🚧 开始阶段二规划
