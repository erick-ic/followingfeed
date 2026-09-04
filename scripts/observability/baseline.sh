#!/usr/bin/env bash

set -euo pipefail

baseline_api_url="${FOLLOWINGFEED_BASELINE_API_URL:-http://127.0.0.1:8080}"
baseline_prometheus_url="${FOLLOWINGFEED_BASELINE_PROMETHEUS_URL:-http://127.0.0.1:9090}"
baseline_requests="${FOLLOWINGFEED_BASELINE_REQUESTS:-50}"
baseline_concurrency="${FOLLOWINGFEED_BASELINE_CONCURRENCY:-5}"
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
if ((baseline_concurrency > baseline_requests)); then
  echo "并发数不能大于请求数" >&2
  exit 1
fi

curl -fsS --max-time 5 "${baseline_api_url}/health/ready" >/dev/null
curl -fsS --max-time 5 "${baseline_prometheus_url}/-/ready" >/dev/null

cached_endpoint="${baseline_api_url}/api/v1/pub/list?page=1&pageSize=10"
database_endpoint="${baseline_api_url}/api/v1/pub/list?page=2&pageSize=10"

# 先预热首页缓存，基线测量阶段只观察稳定状态下的缓存命中。
curl -fsS --max-time 5 "${cached_endpoint}" >"${baseline_tmp_dir}/warm.json"

run_ab_test() {
  local baseline_label="$1"
  local baseline_endpoint="$2"
  local baseline_output_file="$3"

  echo "== ${baseline_label} =="
  ab -q -n "${baseline_requests}" -c "${baseline_concurrency}" "${baseline_endpoint}" \
    >"${baseline_output_file}"
  awk '/Failed requests:|Requests per second:|Time per request:|  50%|  90%|  95%|  99%/ {print}' \
    "${baseline_output_file}"
}

run_ab_test "缓存路径：公开文章第一页" "${cached_endpoint}" \
  "${baseline_tmp_dir}/cached.txt"

# 避免两组快速请求落入限流器的同一秒窗口。
sleep 2

echo
run_ab_test "数据库路径：公开文章第二页" "${database_endpoint}" \
  "${baseline_tmp_dir}/database.txt"

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
http_p50_query="1000 * histogram_quantile(0.50, sum by (le) (rate(followingfeed_http_server_request_duration_seconds_bucket[${baseline_rate_window}])))"
http_p99_query="1000 * histogram_quantile(0.99, sum by (le) (rate(followingfeed_http_server_request_duration_seconds_bucket[${baseline_rate_window}])))"
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

echo
echo "== Prometheus 业务缓存命中率（%） =="
jq -r 'if (.data.result | length) == 0 then "暂无数据" else .data.result[] | "\(.metric.cache)\t\(.value[1] | tonumber | . * 100 | round / 100)%" end' \
  "${baseline_tmp_dir}/cache-hit.json"

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

  ab_value() {
    local baseline_file="$1"
    local baseline_pattern="$2"
    awk "${baseline_pattern}" "${baseline_file}"
  }

  cached_failed="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "Failed" && $2 == "requests:" { print $3; exit }')"
  cached_rps="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "Requests" && $2 == "per" { print $4; exit }')"
  cached_mean="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "Time" && $2 == "per" { print $4; exit }')"
  cached_p50="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "50%" { print $2; exit }')"
  cached_p90="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "90%" { print $2; exit }')"
  cached_p95="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "95%" { print $2; exit }')"
  cached_p99="$(ab_value "${baseline_tmp_dir}/cached.txt" '$1 == "99%" { print $2; exit }')"
  database_failed="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "Failed" && $2 == "requests:" { print $3; exit }')"
  database_rps="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "Requests" && $2 == "per" { print $4; exit }')"
  database_mean="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "Time" && $2 == "per" { print $4; exit }')"
  database_p50="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "50%" { print $2; exit }')"
  database_p90="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "90%" { print $2; exit }')"
  database_p95="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "95%" { print $2; exit }')"
  database_p99="$(ab_value "${baseline_tmp_dir}/database.txt" '$1 == "99%" { print $2; exit }')"

  prometheus_scalar() {
    jq -r '.data.result[0].value[1] // empty' "$1"
  }
  cache_ratio() {
    local baseline_cache_name="$1"
    jq -r --arg cache "${baseline_cache_name}" \
      '[.data.result[] | select(.metric.cache == $cache)][0].value[1] // empty' \
      "${baseline_tmp_dir}/cache-hit.json"
  }
  dataset_total="$(jq -r '.data.total // 0' "${baseline_tmp_dir}/warm.json")"
  git_commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
  generated_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  system_info="$(uname -srm)"

  jq -n \
    --arg generated_at "${generated_at}" \
    --arg git_commit "${git_commit}" \
    --arg system "${system_info}" \
    --arg api_url "${baseline_api_url}" \
    --arg prometheus_url "${baseline_prometheus_url}" \
    --argjson dataset_total "${dataset_total}" \
    --argjson requests "${baseline_requests}" \
    --argjson concurrency "${baseline_concurrency}" \
    --arg rate_window "${baseline_rate_window}" \
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
    --arg http_request_rate "$(prometheus_scalar "${baseline_tmp_dir}/request-rate.json")" \
    --arg http_5xx "$(prometheus_scalar "${baseline_tmp_dir}/http-5xx.json")" \
    --arg http_p50 "$(prometheus_scalar "${baseline_tmp_dir}/http-p50.json")" \
    --arg http_p95 "$(prometheus_scalar "${baseline_tmp_dir}/http-p95.json")" \
    --arg http_p99 "$(prometheus_scalar "${baseline_tmp_dir}/http-p99.json")" \
    --arg in_flight "$(prometheus_scalar "${baseline_tmp_dir}/in-flight.json")" \
    --arg db_select_p95 "$(prometheus_scalar "${baseline_tmp_dir}/db-p95.json")" \
    --arg cache_first_page "$(cache_ratio article_published_first_page)" \
    --arg cache_count "$(cache_ratio article_published_count)" \
    'def number_or_null:
      if . == null or . == "" or . == "NaN" then
        null
      else
        try tonumber catch null
      end;
    {
      schema_version: 1,
      generated_at: $generated_at,
      git_commit: $git_commit,
      environment: {
        system: $system,
        api_url: $api_url,
        prometheus_url: $prometheus_url,
        dataset_total: $dataset_total
      },
      parameters: {
        requests_per_path: $requests,
        concurrency: $concurrency,
        prometheus_rate_window: $rate_window
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
