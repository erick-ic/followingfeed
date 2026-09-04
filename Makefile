# 这些目标不对应同名文件，每次调用时都应执行命令。
# 环境启停
.PHONY: up up-detached down demo
# 监控服务
.PHONY: observability-up observability-down
# 性能测试
.PHONY: observability-baseline observability-report
# 项目测试与验证
.PHONY: test test-integration test-e2e check-migrations verify
# 开发和迁移工具
.PHONY: generate fmt vet web-build migrate-up

# 构建镜像并以前台模式启动完整业务环境，适合观察实时日志。
up:
	docker compose up --build

# 构建镜像并在后台启动完整业务环境，命令执行后立即返回终端。
# 需要查看实时日志时运行：docker compose logs -f
up-detached:
	docker compose up -d --build

# 停止并删除 Compose 容器和网络，保留 MySQL、Grafana 命名数据卷。
down:
	docker compose down

# 完整演示环境的便捷入口，当前行为与 up 相同。
demo:
	docker compose up --build

# 后台启动可选的 Prometheus 和 Grafana；业务环境需已启动或会按依赖自动启动。
observability-up:
	docker compose --profile observability up -d prometheus grafana

# 停止监控组件，不影响 API、Web、MySQL 和 Redis。
observability-down:
	docker compose --profile observability stop grafana prometheus

# 对公开只读接口执行一次本地基线压测，并输出 Prometheus 指标摘要。
observability-baseline:
	./scripts/observability/baseline.sh

# 生成可保存的性能报告；NAME 指定报告名称，COMPARE 可指定优化前 result.json。
# 示例：make observability-report NAME=after COMPARE=docs/performance-records/before/result.json
observability-report:
	FOLLOWINGFEED_REPORT_NAME='$(NAME)' FOLLOWINGFEED_REPORT_COMPARE_TO='$(COMPARE)' \
		./scripts/observability/report.sh

# 运行不依赖外部 MySQL、Redis 的 Go 单元测试。
test:
	go test . ./cmd/... ./config/... ./internal/... ./ioc/... ./migrations/... ./pkg/...

# 在一次性 MySQL 8.0 容器中运行迁移校验与集成测试。
test-integration:
	./scripts/test-integration.sh

# 运行完整 HTTP 端到端测试，执行前需启动本地 MySQL 和 Redis。
test-e2e:
	go test -tags=e2e .

# 在一次性 MySQL 容器的空数据库中执行并验证全部迁移，不使用本地开发数据库。
check-migrations:
	./scripts/check-migrations.sh

# 提交前总检查：单元测试、迁移、Go Vet、前端构建、Lint、格式和 Compose 配置。
verify: test check-migrations vet web-build
	npm --prefix web run lint
	npm --prefix web run format:check
	docker compose config --quiet

# 根据 go:generate 声明重新生成 Wire 依赖注入代码和 GoMock 文件。
generate:
	go generate . ./internal/...

# 格式化全部 Go、前端、Markdown 和 YAML 文件；可能直接修改工作区文件。
fmt:
	gofmt -w $$(rg --files -g '*.go' -g '!web/**')
	npm --prefix web run format
	./web/node_modules/.bin/prettier --write README.md 'docs/*.md' docs/api.yaml \
		config/dev.example.yaml docker-compose.yaml

# 对 Go 包执行静态检查，不修改源码。
vet:
	go vet . ./cmd/... ./config/... ./internal/... ./ioc/... ./migrations/... ./pkg/...

# 构建 Next.js 生产产物，同时执行 TypeScript 类型检查。
web-build:
	cd web && npm run build

# 使用当前配置执行尚未应用的 SQL 迁移；不会启动 API。
migrate-up:
	go run ./cmd/migrate
