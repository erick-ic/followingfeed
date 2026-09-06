# 构建阶段：使用完整 Go 工具链编译应用，编译产物不会把工具链带入最终镜像。
FROM golang:1.26.5-alpine AS builder

WORKDIR /src

# 先复制依赖清单并下载模块。业务源码变化时可继续复用 Docker 依赖缓存层。
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并生成两个静态 Linux 可执行文件：业务 API 和一次性迁移工具。
COPY . .
# CGO_ENABLED=0 使程序不依赖运行镜像中的 C 动态库；trimpath 和 -s -w 用于减小产物。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/followingfeed .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# 运行阶段：Distroless 镜像不包含编译工具和 Shell，减少镜像体积与攻击面。
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# 只从构建阶段复制运行所需的可执行文件。
COPY --from=builder /out/followingfeed /app/followingfeed
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/healthcheck /app/healthcheck

# Gin 使用发布模式，关闭开发调试输出。
ENV GIN_MODE=release

# 声明 API 默认监听端口；端口是否对宿主机开放仍由 Docker Compose 或部署平台决定。
EXPOSE 8080

# 使用 Distroless 内置的非 root 用户运行，避免容器进程获得 root 权限。
USER nonroot:nonroot

# 默认启动业务 API；Compose 的 migrate 服务会覆盖入口并执行 /app/migrate。
ENTRYPOINT ["/app/followingfeed"]
