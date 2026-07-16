# messageFeed Dockerfile
# 本文件定义多阶段构建流程，生成最小化生产镜像。

# ==================== 构建阶段 ====================
# 使用官方 Go 镜像作为构建环境
FROM golang:1.26.1-alpine AS builder

# 安装构建依赖
# - ca-certificates: HTTPS 请求所需的根证书
# - git: go mod download 可能需要访问私有仓库
# - tzdata: 时区数据，确保容器内时间处理正确
RUN apk add --no-cache ca-certificates git tzdata

# 设置工作目录
WORKDIR /build

# 复制 go.mod 和 go.sum（如果存在）并下载依赖
# 该步骤利用 Docker 层缓存，只有依赖变化时才重新下载
COPY go.mod ./
# go.sum 可能不存在（当前无外部依赖），使用通配符避免构建失败
COPY go.su[m] ./
RUN go mod download

# 复制项目源代码
COPY . .

# 编译二进制文件
# - CGO_ENABLED=0: 禁用 CGO，生成静态链接二进制，便于在精简镜像中运行
# - GOOS=linux: 目标操作系统
# - GOARCH=amd64: 目标架构（可根据需要调整为 arm64）
# - -ldflags="-s -w": 去除调试信息和符号表，减小二进制体积
# - -trimpath: 移除文件系统路径信息，增强安全性
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o messagefeed ./cmd/api

# ==================== Web 构建阶段 ====================
FROM node:24-alpine AS web-builder

WORKDIR /build/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web ./
RUN npm run build

# ==================== Web 静态服务阶段 ====================
FROM caddy:2.10.2-alpine AS web

COPY deploy/caddy/Caddyfile.web /etc/caddy/Caddyfile
COPY --from=web-builder /build/web/dist /usr/share/caddy

# ==================== 运行阶段 ====================
# 使用最小化基础镜像
FROM alpine:3.19 AS api

# 安装运行时依赖
# - ca-certificates: HTTPS 请求所需
# - tzdata: 时区数据
# - tini: 作为容器 PID 1 转发信号并回收孤儿子进程
RUN apk add --no-cache ca-certificates tzdata tini

# 创建非 root 用户运行服务
# 使用固定 UID/GID 便于跨容器保持一致性
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /build/messagefeed /app/messagefeed
COPY configs /app/configs

# 切换到非 root 用户
USER appuser

# 暴露服务端口
# 注意：容器内服务应监听 0.0.0.0，通过 BIND_ADDR 环境变量控制
EXPOSE 60001

# 健康检查
# 每 30 秒检查一次 /healthz 端点，超时 5 秒，失败 3 次后标记为 unhealthy
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:60001/healthz || exit 1

# 设置默认环境变量
# 实际部署时应通过 docker-compose 或 Kubernetes ConfigMap/Secret 覆盖
ENV BIND_ADDR=0.0.0.0:60001 \
    PUBLIC_BASE_URL=http://localhost:60001 \
    APP_NODE_ID=docker-node \
    DEPLOYMENT_MODE=single_node \
    LOG_LEVEL=info

# 由 init 进程启动服务，避免孤儿子进程退出后形成僵尸进程
ENTRYPOINT ["/sbin/tini", "--", "/app/messagefeed"]
