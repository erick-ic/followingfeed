#!/usr/bin/env bash

set -euo pipefail

baseline_api_url="${FOLLOWINGFEED_BASELINE_API_URL:-http://127.0.0.1:8080}"
baseline_prometheus_url="${FOLLOWINGFEED_BASELINE_PROMETHEUS_URL:-http://127.0.0.1:9090}"
baseline_metrics_url="${FOLLOWINGFEED_BASELINE_METRICS_URL:-http://127.0.0.1:8081/metrics}"
baseline_requests="${FOLLOWINGFEED_BASELINE_REQUESTS:-50}"
baseline_concurrency="${FOLLOWINGFEED_BASELINE_CONCURRENCY:-5}"
baseline_duration="${FOLLOWINGFEED_BASELINE_DURATION_SECONDS:-0}"
baseline_rounds="${FOLLOWINGFEED_BASELINE_ROUNDS:-3}"
baseline_pause="${FOLLOWINGFEED_BASELINE_PAUSE_SECONDS:-2}"
baseline_cold_wait="${FOLLOWINGFEED_BASELINE_COLD_WAIT_SECONDS:-11}"
baseline_scrape_wait="${FOLLOWINGFEED_BASELINE_SCRAPE_WAIT_SECONDS:-20}"
baseline_rate_window="${FOLLOWINGFEED_BASELINE_RATE_WINDOW:-2m}"
baseline_result_file="${FOLLOWINGFEED_BASELINE_RESULT_FILE:-}"
baseline_tmp_dir="$(mktemp -d /tmp/followingfeed-observability-baseline.XXXXXX)"

cleanup_baseline() {
  rm -rf -- "${baseline_tmp_dir}"
}
trap cleanup_baseline EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "缺少命令: $1" >&2
    exit 1
  fi
}

for baseline_command in curl jq ab awk; do
  require_command "${baseline_command}"
done

if ! [[ "${baseline_requests}" =~ ^[1-9][0-9]*$ ]]; then
  echo "FOLLOWINGFEED_BASELINE_REQUESTS 必须是正整数" >&2
  exit 1
fi
if ! [[ "${baseline_concurrency}" =~ ^[1-9][0-9]*$ ]]; then
  echo "FOLLOWINGFEED_BASELINE_CONCURRENCY 必须是正整数" >&2
  exit 1
fi
for baseline_integer in baseline_duration baseline_rounds baseline_pause baseline_cold_wait; do
  if ! [[ "${!baseline_integer}" =~ ^[0-9]+$ ]] ||
    [[ "${baseline_integer}" == "baseline_rounds" && "${!baseline_integer}" == "0" ]]; then
    echo "${baseline_integer} 必须是正整数或允许为零的等待秒数" >&2
    exit 1
  fi
done
if ((baseline_duration == 0 && baseline_concurrency > baseline_requests)); then
  echo "按请求数执行时，并发数不能大于请求数" >&2
  exit 1
fi

curl -fsS --max-time 5 "${baseline_api_url}/health/ready" >/dev/null
curl -fsS --max-time 5 "${baseline_prometheus_url}/-/ready" >/dev/null

cached_endpoint="${baseline_api_url}/api/v1/pub/list?page=1&pageSize=10"
database_endpoint="${baseline_api_url}/api/v1/pub/list?page=2&pageSize=10"

# 等待列表缓存自然过期，单独记录一次冷缓存首请求；该结果不参与热缓存中位数。
echo "等待首页缓存过期（${baseline_cold_wait}s）..."
sleep "${baseline_cold_wait}"
cold_result="$(curl -sS --max-time 5 -o "${baseline_tmp_dir}/cold.json" \
  -w '%{http_code} %{time_total}' "${cached_endpoint}")"
read -r cold_status cold_seconds <<<"${cold_result}"
if [[ "${cold_status}" != "200" ]]; then
  echo "冷缓存请求失败，HTTP 状态码：${cold_status}" >&2
  exit 1
fi
cold_ms="$(awk -v seconds="${cold_seconds}" 'BEGIN { printf "%.3f", seconds * 1000 }')"

# 冷请求已经填充缓存，再读取一次保存数据规模，并确保热缓存测量处于稳定状态。
curl -fsS --max-time 5 "${cached_endpoint}" >"${baseline_tmp_dir}/warm.json"
dataset_total="$(jq -r '.data.total // 0' "${baseline_tmp_dir}/warm.json")"
total_pages="$(jq -r '.data.totalPages // 0' "${baseline_tmp_dir}/warm.json")"
if ((total_pages < 2)); then
  echo "公开文章不足 11 篇，无法区分首页缓存和数据库分页路径" >&2
  exit 1
fi
deep_page="${FOLLOWINGFEED_BASELINE_DEEP_PAGE:-${total_pages}}"
if ! [[ "${deep_page}" =~ ^[1-9][0-9]*$ ]] || ((deep_page > total_pages)); then
  echo "FOLLOWINGFEED_BASELINE_DEEP_PAGE 必须位于 1..${total_pages}" >&2
  exit 1
fi
deep_endpoint="${baseline_api_url}/api/v1/pub/list?page=${deep_page}&pageSize=10"

# 直接抓取计数器快照，报告中的缓存命中率只覆盖本次多轮压测，不再使用两分钟滑动窗口。
curl -fsS --max-time 5 "${baseline_metrics_url}" >"${baseline_tmp_dir}/metrics-before.prom"

run_ab_test() {
  local baseline_label="$1"
  local baseline_endpoint="$2"
  local baseline_output_file="$3"

  echo "== ${baseline_label} =="
  if ((baseline_duration > 0)); then
    ab -q -t "${baseline_duration}" -c "${baseline_concurrency}" "${baseline_endpoint}" \
      >"${baseline_output_file}"
  else
    ab -q -n "${baseline_requests}" -c "${baseline_concurrency}" "${baseline_endpoint}" \
      >"${baseline_output_file}"
  fi
  awk '/Failed requests:|Requests per second:|Time per request:|  50%|  90%|  95%|  99%/ {print}' \
    "${baseline_output_file}"
}

run_ab_series() {
  local baseline_label="$1"
  local baseline_slug="$2"
  local baseline_endpoint="$3"
  local baseline_round

  for ((baseline_round = 1; baseline_round <= baseline_rounds; baseline_round++)); do
    echo
    run_ab_test "${baseline_label}（第 ${baseline_round}/${baseline_rounds} 轮）" \
      "${baseline_endpoint}" "${baseline_tmp_dir}/${baseline_slug}-round-${baseline_round}.txt"
    # 每轮隔离到不同限流窗口，默认参数不会触发每秒 100 次的生产默认阈值。
    sleep "${baseline_pause}"
  done
}

run_ab_series "热缓存路径：公开文章第一页" cached "${cached_endpoint}"
run_ab_series "浅分页数据库路径：公开文章第二页" database "${database_endpoint}"
run_ab_series "深分页数据库路径：公开文章第 ${deep_page} 页" deep-database "${deep_endpoint}"

curl -fsS --max-time 5 "${baseline_metrics_url}" >"${baseline_tmp_dir}/metrics-after.prom"

echo
echo "等待 Prometheus 完成一次抓取（${baseline_scrape_wait}s）..."
sleep "${baseline_scrape_wait}"

prometheus_query() {
  curl -fsSG --max-time 5 "${baseline_prometheus_url}/api/v1/query" \
    --data-urlencode "query=$1"
}

cache_hit_query="100 * sum by (cache) (rate(followingfeed_cache_operations_total{cache=~\"article_published_(first_page|count)\",operation=\"read\",result=\"hit\"}[${baseline_rate_window}])) / clamp_min(sum by (cache) (rate(followingfeed_cache_operations_total{cache=~\"article_published_(first_page|count)\",operation=\"read\",result=~\"hit|miss\"}[${baseline_rate_window}])), 0.001)"
db_p95_query="(1000 * histogram_quantile(0.95, sum by (le, operation, table) (rate(followingfeed_db_operation_duration_seconds_bucket{operation=\"select\",table=\"publish_articles\"}[${baseline_rate_window}])))) and on(operation, table) (sum by (operation, table) (rate(followingfeed_db_operation_duration_seconds_count{operation=\"select\",table=\"publish_articles\"}[${baseline_rate_window}])) > 0)"
http_p95_query="1000 * histogram_quantile(0.95, sum by (le, route) (rate(followingfeed_http_server_request_duration_seconds_bucket{route=\"/api/v1/pub/list\"}[${baseline_rate_window}])))"
http_p50_query="1000 * histogram_quantile(0.50, sum by (le) (rate(followingfeed_http_server_request_duration_seconds_bucket{route=\"/api/v1/pub/list\"}[${baseline_rate_window}])))"
http_p99_query="1000 * histogram_quantile(0.99, sum by (le) (rate(followingfeed_http_server_request_duration_seconds_bucket{route=\"/api/v1/pub/list\"}[${baseline_rate_window}])))"
request_rate_query="sum(rate(followingfeed_http_server_requests_total[${baseline_rate_window}]))"
http_5xx_query="(100 * sum(rate(followingfeed_http_server_requests_total{status_code=~\"5..\"}[${baseline_rate_window}])) / clamp_min(sum(rate(followingfeed_http_server_requests_total[${baseline_rate_window}])), 0.001)) or vector(0)"
in_flight_query="followingfeed_http_server_requests_in_flight"

prometheus_query "${cache_hit_query}" >"${baseline_tmp_dir}/cache-hit.json"
prometheus_query "${db_p95_query}" >"${baseline_tmp_dir}/db-p95.json"
prometheus_query "${http_p95_query}" >"${baseline_tmp_dir}/http-p95.json"
prometheus_query "${http_p50_query}" >"${baseline_tmp_dir}/http-p50.json"
prometheus_query "${http_p99_query}" >"${baseline_tmp_dir}/http-p99.json"
prometheus_query "${request_rate_query}" >"${baseline_tmp_dir}/request-rate.json"
prometheus_query "${http_5xx_query}" >"${baseline_tmp_dir}/http-5xx.json"
prometheus_query "${in_flight_query}" >"${baseline_tmp_dir}/in-flight.json"

prometheus_counter_value() {
  local baseline_file="$1"
  local baseline_cache_name="$2"
  local baseline_result="$3"
  awk -v cache_name="${baseline_cache_name}" -v result_name="${baseline_result}" '
    index($0, "followingfeed_cache_operations_total{") == 1 &&
    index($0, "cache=\"" cache_name "\"") &&
    index($0, "operation=\"read\"") &&
    index($0, "result=\"" result_name "\"") { total += $NF }
    END { printf "%.17g", total + 0 }
  ' "${baseline_file}"
}

cache_interval_ratio() {
  local baseline_cache_name="$1"
  local before_hit after_hit before_miss after_miss
  before_hit="$(prometheus_counter_value "${baseline_tmp_dir}/metrics-before.prom" "${baseline_cache_name}" hit)"
  after_hit="$(prometheus_counter_value "${baseline_tmp_dir}/metrics-after.prom" "${baseline_cache_name}" hit)"
  before_miss="$(prometheus_counter_value "${baseline_tmp_dir}/metrics-before.prom" "${baseline_cache_name}" miss)"
  after_miss="$(prometheus_counter_value "${baseline_tmp_dir}/metrics-after.prom" "${baseline_cache_name}" miss)"
  awk -v hits="$(awk -v a="${after_hit}" -v b="${before_hit}" 'BEGIN { print a - b }')" \
    -v misses="$(awk -v a="${after_miss}" -v b="${before_miss}" 'BEGIN { print a - b }')" '
      BEGIN {
        denominator = hits + misses
        if (denominator <= 0) exit
        printf "%.8f", 100 * hits / denominator
      }
    '
}

cache_first_page_ratio="$(cache_interval_ratio article_published_first_page)"
cache_count_ratio="$(cache_interval_ratio article_published_count)"

echo
echo "== 本次多轮压测的业务缓存命中率（%） =="
printf 'article_published_count\t%s%%\n' "${cache_count_ratio:-暂无数据}"
printf 'article_published_first_page\t%s%%\n' "${cache_first_page_ratio:-暂无数据}"

echo
echo "== Prometheus 数据库 SELECT P95（ms） =="
jq -r '(.data.result | map(select(.value[1] != "NaN"))) as $result | if ($result | length) == 0 then "暂无数据" else $result[] | "\(.metric.table)\t\(.value[1] | tonumber | . * 100 | round / 100) ms" end' \
  "${baseline_tmp_dir}/db-p95.json"

echo
echo "== Prometheus 公开列表 HTTP P95（ms） =="
jq -r '(.data.result | map(select(.value[1] != "NaN"))) as $result | if ($result | length) == 0 then "暂无数据" else $result[] | "\(.metric.route)\t\(.value[1] | tonumber | . * 100 | round / 100) ms" end' \
  "${baseline_tmp_dir}/http-p95.json"

if [[ -n "${baseline_result_file}" ]]; then
  baseline_output_dir="$(dirname "${baseline_result_file}")"
  mkdir -p "${baseline_output_dir}/raw"
  cp "${baseline_tmp_dir}"/*.txt "${baseline_output_dir}/raw/"
  cp "${baseline_tmp_dir}"/*.json "${baseline_output_dir}/raw/"
  cp "${baseline_tmp_dir}"/*.prom "${baseline_output_dir}/raw/"

  ab_series_value() {
    local baseline_slug="$1"
    local baseline_pattern="$2"
    local baseline_aggregation="${3:-median}"
    local baseline_file
    for baseline_file in "${baseline_tmp_dir}/${baseline_slug}-round-"*.txt; do
      awk "${baseline_pattern}" "${baseline_file}"
    done | sort -n | awk -v aggregation="${baseline_aggregation}" '
      { values[NR] = $1; total += $1 }
      END {
        if (NR == 0) {
          exit
        } else if (aggregation == "sum") {
          printf "%.8f", total
        } else if (NR % 2 == 1) {
          printf "%.8f", values[(NR + 1) / 2]
        } else {
          printf "%.8f", (values[NR / 2] + values[NR / 2 + 1]) / 2
        }
      }
    '
  }

  collect_ab_series() {
    local baseline_slug="$1"
    local baseline_prefix="$2"
    printf -v "${baseline_prefix}_failed" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "Failed" && $2 == "requests:" { print $3; exit }' sum)"
    printf -v "${baseline_prefix}_rps" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "Requests" && $2 == "per" { print $4; exit }')"
    printf -v "${baseline_prefix}_mean" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "Time" && $2 == "per" { print $4; exit }')"
    printf -v "${baseline_prefix}_p50" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "50%" { print $2; exit }')"
    printf -v "${baseline_prefix}_p90" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "90%" { print $2; exit }')"
    printf -v "${baseline_prefix}_p95" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "95%" { print $2; exit }')"
    printf -v "${baseline_prefix}_p99" '%s' "$(ab_series_value "${baseline_slug}" '$1 == "99%" { print $2; exit }')"
  }

  collect_ab_series cached cached
  collect_ab_series database database
  collect_ab_series deep-database deep_database

  prometheus_scalar() {
    jq -r '.data.result[0].value[1] // empty' "$1"
  }
  git_commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
  git_dirty=false
  if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
    git_dirty=true
  fi
  generated_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  system_info="$(uname -srm)"

  jq -n \
    --arg generated_at "${generated_at}" \
    --arg git_commit "${git_commit}" \
    --argjson git_dirty "${git_dirty}" \
    --arg system "${system_info}" \
    --arg api_url "${baseline_api_url}" \
    --arg prometheus_url "${baseline_prometheus_url}" \
    --argjson dataset_total "${dataset_total}" \
    --argjson requests "${baseline_requests}" \
    --argjson concurrency "${baseline_concurrency}" \
    --argjson duration_seconds "${baseline_duration}" \
    --argjson rounds "${baseline_rounds}" \
    --argjson pause_seconds "${baseline_pause}" \
    --argjson cold_wait_seconds "${baseline_cold_wait}" \
    --argjson deep_page "${deep_page}" \
    --arg rate_window "${baseline_rate_window}" \
    --arg cold_status "${cold_status}" \
    --arg cold_ms "${cold_ms}" \
    --arg cached_failed "${cached_failed}" \
    --arg cached_rps "${cached_rps}" \
    --arg cached_mean "${cached_mean}" \
    --arg cached_p50 "${cached_p50}" \
    --arg cached_p90 "${cached_p90}" \
    --arg cached_p95 "${cached_p95}" \
    --arg cached_p99 "${cached_p99}" \
    --arg database_failed "${database_failed}" \
    --arg database_rps "${database_rps}" \
    --arg database_mean "${database_mean}" \
    --arg database_p50 "${database_p50}" \
    --arg database_p90 "${database_p90}" \
    --arg database_p95 "${database_p95}" \
    --arg database_p99 "${database_p99}" \
    --arg deep_database_failed "${deep_database_failed}" \
    --arg deep_database_rps "${deep_database_rps}" \
    --arg deep_database_mean "${deep_database_mean}" \
    --arg deep_database_p50 "${deep_database_p50}" \
    --arg deep_database_p90 "${deep_database_p90}" \
    --arg deep_database_p95 "${deep_database_p95}" \
    --arg deep_database_p99 "${deep_database_p99}" \
    --arg http_request_rate "$(prometheus_scalar "${baseline_tmp_dir}/request-rate.json")" \
    --arg http_5xx "$(prometheus_scalar "${baseline_tmp_dir}/http-5xx.json")" \
    --arg http_p50 "$(prometheus_scalar "${baseline_tmp_dir}/http-p50.json")" \
    --arg http_p95 "$(prometheus_scalar "${baseline_tmp_dir}/http-p95.json")" \
    --arg http_p99 "$(prometheus_scalar "${baseline_tmp_dir}/http-p99.json")" \
    --arg in_flight "$(prometheus_scalar "${baseline_tmp_dir}/in-flight.json")" \
    --arg db_select_p95 "$(prometheus_scalar "${baseline_tmp_dir}/db-p95.json")" \
    --arg cache_first_page "${cache_first_page_ratio}" \
    --arg cache_count "${cache_count_ratio}" \
    'def number_or_null:
      if . == null or . == "" or . == "NaN" then
        null
      else
        try tonumber catch null
      end;
    {
      schema_version: 2,
      generated_at: $generated_at,
      git_commit: $git_commit,
      environment: {
        system: $system,
        git_dirty: $git_dirty,
        api_url: $api_url,
        prometheus_url: $prometheus_url,
        dataset_total: $dataset_total
      },
      parameters: {
        requests_per_path: $requests,
        concurrency: $concurrency,
        duration_seconds: $duration_seconds,
        rounds: $rounds,
        pause_seconds: $pause_seconds,
        cold_wait_seconds: $cold_wait_seconds,
        deep_page: $deep_page,
        prometheus_rate_window: $rate_window
      },
      cold_cache: {
        status_code: ($cold_status | number_or_null),
        latency_ms: ($cold_ms | number_or_null)
      },
      apache_bench: {
        cached_path: {
          failed_requests: ($cached_failed | number_or_null),
          requests_per_second: ($cached_rps | number_or_null),
          mean_ms: ($cached_mean | number_or_null),
          p50_ms: ($cached_p50 | number_or_null),
          p90_ms: ($cached_p90 | number_or_null),
          p95_ms: ($cached_p95 | number_or_null),
          p99_ms: ($cached_p99 | number_or_null)
        },
        database_path: {
          failed_requests: ($database_failed | number_or_null),
          requests_per_second: ($database_rps | number_or_null),
          mean_ms: ($database_mean | number_or_null),
          p50_ms: ($database_p50 | number_or_null),
          p90_ms: ($database_p90 | number_or_null),
          p95_ms: ($database_p95 | number_or_null),
          p99_ms: ($database_p99 | number_or_null)
        },
        deep_database_path: {
          failed_requests: ($deep_database_failed | number_or_null),
          requests_per_second: ($deep_database_rps | number_or_null),
          mean_ms: ($deep_database_mean | number_or_null),
          p50_ms: ($deep_database_p50 | number_or_null),
          p90_ms: ($deep_database_p90 | number_or_null),
          p95_ms: ($deep_database_p95 | number_or_null),
          p99_ms: ($deep_database_p99 | number_or_null)
        }
      },
      prometheus: {
        http: {
          request_rate_rps: ($http_request_rate | number_or_null),
          error_5xx_percent: ($http_5xx | number_or_null),
          p50_ms: ($http_p50 | number_or_null),
          p95_ms: ($http_p95 | number_or_null),
          p99_ms: ($http_p99 | number_or_null),
          requests_in_flight: ($in_flight | number_or_null)
        },
        cache_hit_ratio_percent: {
          article_published_first_page: ($cache_first_page | number_or_null),
          article_published_count: ($cache_count | number_or_null)
        },
        database: { select_p95_ms: ($db_select_p95 | number_or_null) }
      }
    }' >"${baseline_result_file}"

  echo
  echo "结构化结果已保存：${baseline_result_file}"
fi
