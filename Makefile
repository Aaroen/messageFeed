# messageFeed Makefile
# 本文件定义项目的常用构建、测试、运行和验证命令。
# 使用方式：make <target>

# ==================== 变量定义 ====================

# Go 编译器和工具链
GO := go
GOFMT := gofmt
GOVET := $(GO) vet
GOTEST := $(GO) test
DOCKER_COMPOSE ?= docker compose

# 开发态统一入口配置。可在命令行覆盖，例如：
# PUBLIC_BASE_URL=https://100.x.x.x:8443 \
# GATEWAY_SITE_ADDRESS="https://localhost:8443, https://100.x.x.x:8443" \
# GATEWAY_DEFAULT_SNI=100.x.x.x make compose-dev
GATEWAY_HTTPS_PORT ?= 8443
PUBLIC_BASE_URL ?= https://localhost:$(GATEWAY_HTTPS_PORT)
GATEWAY_SITE_ADDRESS ?= https://localhost:$(GATEWAY_HTTPS_PORT)
GATEWAY_DEFAULT_SNI ?= localhost

# 项目二进制名称
BINARY_NAME := messagefeed
BINARY_PATH := ./$(BINARY_NAME)

# 主入口路径
MAIN_PACKAGE := ./cmd/api

# 测试覆盖率输出文件
COVERAGE_FILE := coverage.out

# Docker 镜像名称和标签
DOCKER_IMAGE := messagefeed
DOCKER_TAG := latest

# 数据库迁移配置
# 该连接串在 docker compose 网络内使用，供 migrate 服务连接 PostgreSQL。
MIGRATE_DATABASE_URL ?= postgres://messagefeed:devpassword@postgres:5432/messagefeed?sslmode=disable

# ==================== 默认目标 ====================

# 默认目标：显示帮助信息
.DEFAULT_GOAL := help

# ==================== 主要目标 ====================

.PHONY: help
help: ## 显示本帮助信息
	@echo "messageFeed 项目 Makefile 目标列表："
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

.PHONY: verify
verify: fmt vet test build ## 执行完整验收流程（格式、静态检查、测试、构建）
	@echo "✅ 所有验收步骤通过"

.PHONY: fmt
fmt: ## 检查代码格式是否符合 gofmt 规范
	@echo "检查代码格式..."
	@files=$$($(GOFMT) -l . 2>&1 | grep -v '^vendor/' || true); \
	if [ -n "$$files" ]; then \
		echo "❌ 以下文件格式不符合规范："; \
		echo "$$files"; \
		echo ""; \
		echo "执行以下命令修复："; \
		echo "  make fmt-fix"; \
		exit 1; \
	fi
	@echo "✅ 代码格式检查通过"

.PHONY: fmt-fix
fmt-fix: ## 自动修复代码格式
	@echo "自动格式化代码..."
	@$(GOFMT) -w .
	@echo "✅ 代码格式已修复"

.PHONY: vet
vet: ## 运行 go vet 静态检查
	@echo "运行静态检查..."
	@$(GOVET) ./...
	@echo "✅ 静态检查通过"

.PHONY: test
test: ## 运行所有测试
	@echo "运行测试..."
	@$(GOTEST) -v ./...
	@echo "✅ 所有测试通过"

.PHONY: test-race
test-race: ## 运行测试并启用竞态检测
	@echo "运行测试（竞态检测）..."
	@$(GOTEST) -race ./...
	@echo "✅ 竞态检测通过"

.PHONY: test-cover
test-cover: ## 运行测试并生成覆盖率报告
	@echo "运行测试（覆盖率）..."
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
	@$(GO) tool cover -func=$(COVERAGE_FILE)
	@echo ""
	@echo "生成 HTML 覆盖率报告："
	@$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "✅ 覆盖率报告已生成：coverage.html"

.PHONY: build
build: ## 编译项目二进制
	@echo "编译项目..."
	@$(GO) build -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "✅ 编译完成：$(BINARY_PATH)"

.PHONY: run
run: ## 本地运行服务（使用默认配置）
	@echo "启动服务..."
	@$(GO) run $(MAIN_PACKAGE)

.PHONY: clean
clean: ## 清理构建产物和临时文件
	@echo "清理构建产物..."
	@rm -f $(BINARY_PATH)
	@rm -f $(COVERAGE_FILE) coverage.html
	@rm -f *.log
	@echo "✅ 清理完成"

# ==================== Docker 目标 ====================

.PHONY: docker-build
docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✅ Docker 镜像构建完成：$(DOCKER_IMAGE):$(DOCKER_TAG)"

.PHONY: docker-run
docker-run: ## 运行 Docker 容器（需要先构建镜像）
	@echo "运行 Docker 容器..."
	@docker run --rm -p 60001:60001 \
		-e BIND_ADDR=0.0.0.0:60001 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: compose-up
compose-up: ## 启动 Docker Compose 服务
	@echo "启动 Docker Compose 服务..."
	@$(DOCKER_COMPOSE) up -d --build
	@echo "✅ 服务已启动"
	@echo ""
	@echo "服务地址："
	@echo "  API: http://localhost:60001"
	@echo "  健康检查: http://localhost:60001/healthz"
	@echo "  Grafana: http://localhost:3000"
	@echo "  Prometheus: http://localhost:9090"
	@echo ""

.PHONY: compose-down
compose-down: ## 停止 Docker Compose 服务
	@echo "停止 Docker Compose 服务..."
	@$(DOCKER_COMPOSE) down
	@echo "✅ 服务已停止"

.PHONY: compose-logs
compose-logs: ## 查看 Docker Compose 日志
	@$(DOCKER_COMPOSE) logs -f

.PHONY: compose-ps
compose-ps: ## 查看 Docker Compose 服务状态
	@$(DOCKER_COMPOSE) ps

.PHONY: compose-dev
compose-dev: ## 启动开发态准部署入口（HTTPS 统一入口 + API + Vite）
	@echo "启动开发态准部署入口..."
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev up -d api-dev web-dev gateway-dev
	@echo "开发态服务已启动"
	@echo ""
	@echo "开发入口："
	@echo "  Web/API: $(PUBLIC_BASE_URL)"
	@echo "  健康检查: $(PUBLIC_BASE_URL)/healthz"
	@echo "  证书站点: $(GATEWAY_SITE_ADDRESS)"
	@echo ""

.PHONY: compose-dev-watch
compose-dev-watch: ## 监听开发态文件变化并自动同步/重载容器
	@echo "监听开发态文件变化..."
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev watch api-dev web-dev gateway-dev

.PHONY: compose-dev-logs
compose-dev-logs: ## 查看开发态服务日志
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev logs -f api-dev web-dev gateway-dev

.PHONY: compose-dev-reload-api
compose-dev-reload-api: ## 手动重载开发态 API 服务
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev restart api-dev

.PHONY: compose-dev-reload-web
compose-dev-reload-web: ## 手动重载开发态 Web 服务
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev restart web-dev

.PHONY: compose-dev-reload-gateway
compose-dev-reload-gateway: ## 手动重载开发态统一入口服务
	@GATEWAY_HTTPS_PORT="$(GATEWAY_HTTPS_PORT)" PUBLIC_BASE_URL="$(PUBLIC_BASE_URL)" GATEWAY_SITE_ADDRESS="$(GATEWAY_SITE_ADDRESS)" GATEWAY_DEFAULT_SNI="$(GATEWAY_DEFAULT_SNI)" $(DOCKER_COMPOSE) --profile dev restart gateway-dev

# ==================== 依赖管理 ====================

.PHONY: deps
deps: ## 下载并整理项目依赖
	@echo "整理项目依赖..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "✅ 依赖已更新"

.PHONY: deps-verify
deps-verify: ## 验证依赖完整性
	@echo "验证依赖完整性..."
	@$(GO) mod verify
	@echo "✅ 依赖验证通过"

# ==================== 数据库迁移 ====================

.PHONY: migrate-up
migrate-up: ## 执行数据库迁移（需要先启动 PostgreSQL）
	@echo "执行数据库迁移..."
	@$(DOCKER_COMPOSE) run --rm migrate -path /migrations -database "$(MIGRATE_DATABASE_URL)" up
	@echo "✅ 数据库迁移完成"

.PHONY: migrate-down
migrate-down: ## 回滚 1 个数据库迁移版本
	@echo "回滚数据库迁移..."
	@$(DOCKER_COMPOSE) run --rm migrate -path /migrations -database "$(MIGRATE_DATABASE_URL)" down 1
	@echo "✅ 数据库迁移已回滚 1 个版本"

.PHONY: migrate-version
migrate-version: ## 显示当前数据库迁移版本
	@$(DOCKER_COMPOSE) run --rm migrate -path /migrations -database "$(MIGRATE_DATABASE_URL)" version

# ==================== 代码生成 ====================

.PHONY: generate
generate: ## 运行 go generate
	@echo "运行代码生成..."
	@$(GO) generate ./...
	@echo "✅ 代码生成完成"

# ==================== 其他工具 ====================

.PHONY: lint
lint: ## 运行 golangci-lint（需要预先安装）
	@echo "运行 golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "✅ Lint 检查通过"; \
	else \
		echo "⚠️  golangci-lint 未安装"; \
		echo "安装方式：https://golangci-lint.run/usage/install/"; \
	fi

.PHONY: mod-graph
mod-graph: ## 显示依赖关系图
	@$(GO) mod graph

.PHONY: version
version: ## 显示 Go 版本信息
	@$(GO) version
