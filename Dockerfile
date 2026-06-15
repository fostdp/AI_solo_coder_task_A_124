# syntax=docker/dockerfile:1.6
# Stage 1: Go Builder - 多阶段构建，静态编译
FROM golang:1.22-alpine3.19 AS builder

LABEL maintainer="lingqu-dou-gate-team"
LABEL stage="builder"

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOTOOLCHAIN=auto

WORKDIR /src

# 缓存go.mod依赖层
COPY backend/go.mod backend/go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download 2>/dev/null || true

# 复制源码
COPY backend/ .

# 静态编译：-w -s 去除调试信息，减少二进制大小
RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out/config && \
    go build \
      -trimpath \
      -ldflags="-w -s -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.gitHash=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.version=1.0.0" \
      -tags "netgo,osusergo,static_build" \
      -o /out/ship-lock-scheduler \
      ./cmd/main.go && \
    cp -r config/* /out/config/ 2>/dev/null || true

# 验证静态链接
RUN ldd /out/ship-lock-scheduler 2>&1 | grep -q "not a dynamic executable" && echo "OK: static binary"

# Stage 2: 最小运行时镜像 (Alpine + ca-certs)
FROM alpine:3.19 AS runtime

LABEL org.opencontainers.image.title="Lingqu DouGate Scheduler"
LABEL org.opencontainers.image.description="灵渠陡门船舶调度与水力学仿真服务"
LABEL org.opencontainers.image.source="lingqu-dou-gate"

# 安装tzdata和tzdata，CA证书用于MQTT TLS
RUN apk add --no-cache \
        ca-certificates \
        tzdata \
        curl \
    && update-ca-certificates \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && apk del tzdata

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    CONFIG_DIR=/app/config

WORKDIR /app

# 从builder拷贝二进制和配置
COPY --from=builder /out/ship-lock-scheduler /app/ship-lock-scheduler
COPY --from=builder /out/config/ /app/config/

# 非root用户运行，增加安全性
RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080 6060

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/api/gates || exit 1

ENTRYPOINT ["/app/ship-lock-scheduler"]
