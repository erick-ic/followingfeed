# 本地可观测性使用指南

上线前的性能验证重点关注 HTTP 请求性能、业务缓存命中率和数据库查询耗时；同时保留
`/health/ready` 就绪探针、数据库连接池饱和度和已恢复 panic 三项最低运行守护能力。

## 启动

```bash
docker compose up -d --build
make observability-up
```

打开 Grafana <http://localhost:3001>，进入 `FollowingFeed Overview` Dashboard。
本地基线脚本还需要 `curl`、`jq` 和 ApacheBench（`ab`）；macOS 通常自带 `ab`，
Debian/Ubuntu 可通过 `apache2-utils` 安装。

## 建立本地基线

```bash
make observability-baseline
```

脚本使用公开只读接口，不创建或修改业务数据：

- 等待 11 秒让列表缓存自然过期，单独记录一次冷缓存首请求；
- 第一页请求用于观察预热后的热点缓存；
- 第二页和最后一页分别观察浅分页、深分页数据库查询；
- 三条路径默认各执行 3 轮，每轮 50 次请求、并发 5，报告使用各轮中位数；
- 每轮之间默认等待 2 秒，避免落入同一个限流窗口；
- 等待 Prometheus 抓取后输出缓存命中率、数据库 SELECT P95 和 HTTP P95。

脚本会把运行时接口返回的实际文章总数写入 `result.json`，以报告中的
`environment.dataset_total` 为准。测试要求至少存在 11 篇公开文章，否则会直接失败；默认使用最后一页
作为深分页场景，也可通过 `FOLLOWINGFEED_BASELINE_DEEP_PAGE` 指定页码。对比优化前后结果时，文章数量
和内容规模必须保持一致。

缓存命中率通过测试前后的 Prometheus 计数器差值计算，只覆盖本次测试区间；运行期间不要同时手工请求
相关接口。可以通过环境变量调整轮数、请求量、并发和等待时间：

```bash
FOLLOWINGFEED_BASELINE_REQUESTS=50 \
FOLLOWINGFEED_BASELINE_CONCURRENCY=10 \
FOLLOWINGFEED_BASELINE_ROUNDS=5 \
make observability-baseline
```

默认的每 IP 限流仍是 1 秒 100 次。需要执行每轮 30 秒以上的容量测试时，只能在隔离的本地性能环境中
临时调高阈值并重建 API，测试结束后立即恢复默认值：

```bash
FOLLOWINGFEED_RATE_LIMIT_THRESHOLD=100000 \
docker compose up -d --no-deps --force-recreate api

FOLLOWINGFEED_BASELINE_DURATION_SECONDS=30 \
FOLLOWINGFEED_BASELINE_CONCURRENCY=20 \
FOLLOWINGFEED_BASELINE_ROUNDS=3 \
make observability-report NAME=capacity-baseline

docker compose up -d --no-deps --force-recreate api
```

持续时间大于 0 时 ApacheBench 按时间运行，并忽略每轮请求数上限。不要在生产环境关闭或调高限流来压测。

## 自动保存分析报告

首次优化前执行：

```bash
make observability-report NAME=before-cache-optimization
```

命令会完成只读压测、查询 Prometheus，并在
`.local/performance-records/<时间>-before-cache-optimization/` 中保存：

- `report.md`：可直接阅读的本次指标与自动检查结论；
- `result.json`：供后续报告自动对比的结构化数据；
- `baseline-output.txt`：本次终端输出；
- `raw/`：ApacheBench 和 Prometheus 原始响应，便于复核。

完成优化后，把优化前目录中的 `result.json` 传给 `COMPARE`：

```bash
make observability-report \
  NAME=after-cache-optimization \
  COMPARE=.local/performance-records/<优化前目录>/result.json
```

新报告会增加“优化前后对比”表，自动计算相对变化并标记改善、基本持平或退化。
轮数、持续时间、请求数和并发数等参数仍可通过相同环境变量调整。使用固定请求数的小规模回归示例：

```bash
FOLLOWINGFEED_BASELINE_REQUESTS=50 \
FOLLOWINGFEED_BASELINE_CONCURRENCY=5 \
make observability-report NAME=before-query
```

为了让对比有效，两次报告必须使用相同机器、数据集、轮数、持续时间、请求数、并发数、深分页页码和
统计窗口。脚本会记录 Git commit 和工作区是否存在未提交改动；正式对比应在 clean 工作区执行。
脚本默认自动运行三轮，并使用中位结果降低本机抖动影响。

## 如何判断优化结果

同一机器、相同数据量、相同请求数和并发下，分别记录改造前后结果：

| 目标       | 重点指标                                                    | 期望方向               |
| ---------- | ----------------------------------------------------------- | ---------------------- |
| 缓存优化   | `followingfeed_cache_operations_total` 计算的 hit ratio     | 命中率提高，错误率降低 |
| 查询优化   | `followingfeed_db_operation_duration_seconds` 的 P95/P99    | 延迟降低               |
| 接口优化   | `followingfeed_http_server_request_duration_seconds` 的 P95 | 延迟降低且 5xx 不增加  |
| 连接池调整 | `go_sql_in_use_connections / go_sql_max_open_connections`   | 饱和度和等待时间降低   |

不要只比较单次请求耗时。优先比较多轮 P95 中位数，并确认冷缓存请求、三条压测路径和 HTTP 5xx
守护指标均正常。HTTP 指标按公开列表路由过滤；缓存命中率使用测试区间计数器差值，避免被两分钟
Prometheus 查询窗口稀释。

## 数据口径说明

文章数量取决于执行时的本地数据库。基线结果不在本文档中固化，执行
`make observability-report NAME=<场景名称>` 后，应直接查看本次生成的 `report.md` 和
`result.json`。这些结果只用于同一机器、相同数据量和相同压测参数下的相对比较，不能作为
生产环境容量结论或 SLO；硬件、Docker 资源、数据量或并发参数变化后应重新建立基线。
