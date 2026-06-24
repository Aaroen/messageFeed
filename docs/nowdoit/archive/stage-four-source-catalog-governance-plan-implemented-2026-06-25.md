# 阶段四源目录治理执行计划

**最后更新**：2026-06-25

**归档状态**：已完成后端、API、迁移、许可治理、热度排序、健康检查与验证；Web 源目录 UI 筛选和展示改造按用户明确要求排除。

**实现提交**：

- `4c9b370 extend source catalog governance fields`
- `6ddc5c7 add source catalog health checks`

**验证结果**：

- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。
- Compose 重建和迁移完成，`api` 容器为 healthy。
- `GET /api/v1/source-catalogs?language=en&license_status=allowed&health_status=healthy&limit=3` 返回 200。
- 匿名 `POST /api/v1/source-catalogs/check` 返回 401，符合受保护入口预期。

## 目标

完成 `docs/implementation.md` 中阶段四收尾事项，补齐推荐源目录的健康检查、许可状态、热度、最近校验时间和更多过滤维度，使源目录从静态候选列表升级为可治理、可筛选、可审计的数据面。

## P1 实施范围

1. 数据模型补齐：
   - 为 `source_catalog_entries` 明确许可状态、许可说明、热度和校验元数据。
   - 复用现有 `health_status`、`last_checked_at` 和 `last_check_error` 字段，避免重复建模。
   - 保持 `source_origin`、`source_key` 和 `normalized_url` 的唯一性约束语义。
2. 健康检查流程：
   - 对源目录候选 feed 执行受控 HTTP 校验。
   - 记录可达性、HTTP 状态、内容类型、错误摘要和校验时间。
   - 将结果映射到 `healthy`、`degraded`、`unreachable`、`unknown`。
3. 许可状态治理：
   - 为候选源记录 `unknown`、`allowed`、`restricted`、`blocked` 等许可状态。
   - 推荐源导入和展示默认不绕过许可状态。
   - 对来源出处与许可状态保留可追溯字段。
4. 热度与排序：
   - 为源目录增加热度字段或派生排序指标。
   - 源目录默认排序兼顾健康状态、热度、分类和名称。
5. API 能力：
   - 扩展 `GET /api/v1/source-catalogs` 与 `/search` 查询参数。
   - 支持按语言、国家、健康状态、许可状态、分类、订阅状态和关键词过滤。
   - 响应包含许可状态、热度、最近校验时间和健康摘要。
6. Web 能力：
   - 订阅管理页源目录区域展示健康、许可、热度和最近校验时间。
   - 增加语言、国家、健康状态、许可状态等过滤控件。
   - 保持已有导入、订阅、启用和停用流程不回退。

## 非目标

- 不新增外部源目录供应商。
- 不批量抓取网页正文或实现主动采集快照。
- 不实现生产级调度系统。
- 不引入复杂评分模型。
- 不改变已有用户订阅源的启用、停用和抓取语义。

## 验收标准

- 数据迁移可正向执行，回滚脚本完整。
- 源目录列表 API 可按语言、国家、健康状态、许可状态和关键词过滤。
- 源目录响应包含健康状态、许可状态、热度和最近校验时间。
- 健康检查流程能更新 `health_status`、`last_checked_at` 和错误摘要。
- Web 源目录筛选与展示字段和 API 保持一致。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `npm --prefix web run type-check` 通过。
- `npm --prefix web run build` 通过。

## 实施顺序

1. 梳理现有 `source_catalog_entries` 迁移、仓储、服务、Handler、OpenAPI 和 Web 源目录展示。
2. 新增迁移，补齐许可状态、许可说明、热度和必要索引。
3. 扩展 domain、repository、service 和 handler 的过滤参数、排序和响应字段。
4. 实现源目录健康检查服务，优先采用短超时 HEAD/GET 校验并限制响应读取规模。
5. 增加健康检查入口或受控刷新路径，并补齐权限约束。
6. 更新 Web 订阅管理页源目录筛选、列表状态展示和导入前许可提示。
7. 更新 OpenAPI 契约和聚焦测试。
8. 执行后端、前端和 Compose 冒烟验证。
9. 将本计划归档，并根据主文档生成下一实施包计划。

## 风险与约束

- 健康检查必须设置超时、重定向限制和响应大小限制，避免源目录校验阻塞服务。
- 许可状态默认保守，未知许可不得被展示为可自由使用。
- 热度字段应保持可解释，不把不可追溯的外部排名写成确定事实。
- 过滤参数需要与 OpenAPI、前端控件和仓储查询保持一致。
- 不删除既有源目录数据，不物理删除用户订阅源。

## 后续衔接

本计划完成后，下一实施包应回到主文档中优先级最高的未完成项：Agent 计划与审批，或继续处理阶段三遗留的统一错误类型抽象。
