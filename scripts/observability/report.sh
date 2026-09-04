#!/usr/bin/env bash

set -euo pipefail

report_script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
report_project_root="$(cd "${report_script_dir}/../.." && pwd)"
report_name="${FOLLOWINGFEED_REPORT_NAME:-baseline}"
report_compare_to="${FOLLOWINGFEED_REPORT_COMPARE_TO:-}"
report_root="${FOLLOWINGFEED_REPORT_ROOT:-${report_project_root}/docs/performance-records}"
report_timestamp="$(date '+%Y%m%d-%H%M%S')"
report_slug="$(printf '%s' "${report_name}" | tr -cs '[:alnum:]_-' '-' | sed 's/^-//; s/-$//')"

if [[ -z "${report_slug}" ]]; then
  report_slug="baseline"
fi
if [[ -n "${report_compare_to}" && ! -f "${report_compare_to}" ]]; then
  echo "对比文件不存在：${report_compare_to}" >&2
  exit 1
fi

report_dir="${report_root}/${report_timestamp}-${report_slug}"
report_result="${report_dir}/result.json"
report_markdown="${report_dir}/report.md"
mkdir -p "${report_dir}"

echo "报告目录：${report_dir}"
FOLLOWINGFEED_BASELINE_RESULT_FILE="${report_result}" \
  "${report_script_dir}/baseline.sh" | tee "${report_dir}/baseline-output.txt"

if [[ -n "${report_compare_to}" ]]; then
  cp "${report_compare_to}" "${report_dir}/raw/comparison-result.json"
fi

json_value() {
  local report_json="$1"
  local report_path="$2"
  jq -r "${report_path} // empty" "${report_json}"
}

format_value() {
  local report_value="$1"
  local report_unit="$2"
  if [[ -z "${report_value}" || "${report_value}" == "null" ]]; then
    printf 'N/A'
    return
  fi
  case "${report_unit}" in
    count)
      printf '%.0f' "${report_value}"
      ;;
    *)
      printf '%.2f %s' "${report_value}" "${report_unit}"
      ;;
  esac
}

metric_change() {
  local report_before="$1"
  local report_after="$2"
  local report_unit="$3"
  if [[ -z "${report_before}" || -z "${report_after}" || "${report_before}" == "null" || "${report_after}" == "null" ]]; then
    printf 'N/A'
    return
  fi
  if [[ "${report_unit}" == "%" ]]; then
    awk -v before="${report_before}" -v after="${report_after}" \
      'BEGIN { printf "%+.2f pp", after - before }'
    return
  fi
  awk -v before="${report_before}" -v after="${report_after}" '
    BEGIN {
      if (before == 0) {
        if (after == 0) print "0.00%"; else print "N/A"
      } else {
        printf "%+.2f%%", (after - before) / before * 100
      }
    }'
}

metric_judgment() {
  local report_before="$1"
  local report_after="$2"
  local report_direction="$3"
  local report_unit="$4"
  if [[ -z "${report_before}" || -z "${report_after}" || "${report_before}" == "null" || "${report_after}" == "null" ]]; then
    printf '数据不足'
    return
  fi
  if [[ "${report_direction}" == "context" ]]; then
    printf '负载上下文'
    return
  fi
  awk -v before="${report_before}" -v after="${report_after}" \
    -v direction="${report_direction}" -v unit="${report_unit}" '
    BEGIN {
      difference = after - before
      if (unit == "%") {
        tolerance = 0.5
      } else if (before == 0) {
        tolerance = 0
      } else {
        tolerance = (before < 0 ? -before : before) * 0.03
      }
      if (difference < 0) absolute = -difference; else absolute = difference
      if (absolute <= tolerance) {
        print "基本持平"
      } else if ((direction == "higher" && difference > 0) ||
                 (direction == "lower" && difference < 0)) {
        print "改善"
      } else {
        print "退化"
      }
    }'
}

report_metrics=(
  '冷缓存首请求耗时|.cold_cache.latency_ms|lower|ms'
  '热缓存路径失败总数|.apache_bench.cached_path.failed_requests|lower|count'
  '热缓存路径吞吐中位数|.apache_bench.cached_path.requests_per_second|higher|req/s'
  '热缓存路径平均耗时中位数|.apache_bench.cached_path.mean_ms|lower|ms'
  '热缓存路径 P90 中位数|.apache_bench.cached_path.p90_ms|lower|ms'
  '热缓存路径 P95 中位数|.apache_bench.cached_path.p95_ms|lower|ms'
  '热缓存路径 P99 中位数|.apache_bench.cached_path.p99_ms|lower|ms'
  '浅分页数据库路径失败总数|.apache_bench.database_path.failed_requests|lower|count'
  '浅分页数据库路径吞吐中位数|.apache_bench.database_path.requests_per_second|higher|req/s'
  '浅分页数据库路径平均耗时中位数|.apache_bench.database_path.mean_ms|lower|ms'
  '浅分页数据库路径 P90 中位数|.apache_bench.database_path.p90_ms|lower|ms'
  '浅分页数据库路径 P95 中位数|.apache_bench.database_path.p95_ms|lower|ms'
  '浅分页数据库路径 P99 中位数|.apache_bench.database_path.p99_ms|lower|ms'
  '深分页数据库路径失败总数|.apache_bench.deep_database_path.failed_requests|lower|count'
  '深分页数据库路径吞吐中位数|.apache_bench.deep_database_path.requests_per_second|higher|req/s'
  '深分页数据库路径平均耗时中位数|.apache_bench.deep_database_path.mean_ms|lower|ms'
  '深分页数据库路径 P90 中位数|.apache_bench.deep_database_path.p90_ms|lower|ms'
  '深分页数据库路径 P95 中位数|.apache_bench.deep_database_path.p95_ms|lower|ms'
  '深分页数据库路径 P99 中位数|.apache_bench.deep_database_path.p99_ms|lower|ms'
  'HTTP 请求速率|.prometheus.http.request_rate_rps|context|req/s'
  'HTTP 5xx 比例|.prometheus.http.error_5xx_percent|lower|%'
  'HTTP P50|.prometheus.http.p50_ms|lower|ms'
  'HTTP P95|.prometheus.http.p95_ms|lower|ms'
  'HTTP P99|.prometheus.http.p99_ms|lower|ms'
  'HTTP 进行中请求|.prometheus.http.requests_in_flight|context|count'
  '公开首页缓存命中率|.prometheus.cache_hit_ratio_percent.article_published_first_page|higher|%'
  '公开总数缓存命中率|.prometheus.cache_hit_ratio_percent.article_published_count|higher|%'
  'MySQL SELECT P95|.prometheus.database.select_p95_ms|lower|ms'
)

generated_at="$(json_value "${report_result}" '.generated_at')"
git_commit="$(json_value "${report_result}" '.git_commit')"
git_dirty="$(json_value "${report_result}" '.environment.git_dirty')"
system_info="$(json_value "${report_result}" '.environment.system')"
dataset_total="$(json_value "${report_result}" '.environment.dataset_total')"
request_count="$(json_value "${report_result}" '.parameters.requests_per_path')"
concurrency="$(json_value "${report_result}" '.parameters.concurrency')"
duration_seconds="$(json_value "${report_result}" '.parameters.duration_seconds')"
rounds="$(json_value "${report_result}" '.parameters.rounds')"
deep_page="$(json_value "${report_result}" '.parameters.deep_page')"
cold_wait="$(json_value "${report_result}" '.parameters.cold_wait_seconds')"

{
  echo "# FollowingFeed 监控分析报告：${report_name}"
  echo
  echo "## 测试信息"
  echo
  echo "- 生成时间：${generated_at}"
  echo "- Git Commit：\`${git_commit}\`"
  if [[ "${git_dirty}" == "true" ]]; then
    echo "- Git 工作区：**存在未提交改动，报告只能用于本次本机诊断**"
  else
    echo "- Git 工作区：clean"
  fi
  echo "- 本机环境：\`${system_info}\`"
  echo "- 公开文章数量：${dataset_total}"
  echo "- 每轮请求数上限：${request_count}（持续时间大于 0 时由 ApacheBench 忽略）"
  echo "- 并发数：${concurrency}"
  echo "- 每轮持续时间：${duration_seconds} 秒（0 表示按请求数执行）"
  echo "- 重复轮数：${rounds}（表中 AB 性能指标取各轮中位数，失败数为各轮合计）"
  echo "- 冷缓存等待：${cold_wait} 秒"
  echo "- 深分页页码：${deep_page}"
  echo
  echo "## 本次指标"
  echo
  echo "| 指标 | 本次结果 | 期望方向 |"
  echo "| --- | ---: | --- |"
  for report_metric in "${report_metrics[@]}"; do
    IFS='|' read -r metric_name metric_path metric_direction metric_unit <<<"${report_metric}"
    metric_value="$(json_value "${report_result}" "${metric_path}")"
    case "${metric_direction}" in
      higher) direction_text="越高越好" ;;
      lower) direction_text="越低越好" ;;
      context) direction_text="负载上下文" ;;
    esac
    echo "| ${metric_name} | $(format_value "${metric_value}" "${metric_unit}") | ${direction_text} |"
  done

  if [[ -n "${report_compare_to}" ]]; then
    echo
    echo "## 优化前后对比"
    echo
    echo "- 对比基线：\`${report_compare_to}\`"
    comparison_consistent=true
    for comparison_path in \
      '.schema_version' \
      '.environment.system' \
      '.environment.git_dirty' \
      '.environment.dataset_total' \
      '.parameters.requests_per_path' \
      '.parameters.concurrency' \
      '.parameters.duration_seconds' \
      '.parameters.rounds' \
      '.parameters.deep_page' \
      '.parameters.prometheus_rate_window'; do
      current_condition="$(json_value "${report_result}" "${comparison_path}")"
      previous_condition="$(json_value "${report_compare_to}" "${comparison_path}")"
      if [[ "${current_condition}" != "${previous_condition}" ]]; then
        comparison_consistent=false
        break
      fi
    done
    if [[ "${comparison_consistent}" == true ]]; then
      echo "- 对比条件检查：机器、数据量、请求参数和统计窗口一致。"
    else
      echo "- **对比条件警告：机器、数据量、请求参数或统计窗口不一致，变化结果仅供参考。**"
    fi
    echo
    echo "| 指标 | 优化前 | 本次 | 变化 | 判断 |"
    echo "| --- | ---: | ---: | ---: | --- |"
    for report_metric in "${report_metrics[@]}"; do
      IFS='|' read -r metric_name metric_path metric_direction metric_unit <<<"${report_metric}"
      current_value="$(json_value "${report_result}" "${metric_path}")"
      previous_value="$(json_value "${report_compare_to}" "${metric_path}")"
      if [[ "${comparison_consistent}" == true ]]; then
        judgment="$(metric_judgment "${previous_value}" "${current_value}" "${metric_direction}" "${metric_unit}")"
      else
        judgment="条件不一致"
      fi
      echo "| ${metric_name} | $(format_value "${previous_value}" "${metric_unit}") | $(format_value "${current_value}" "${metric_unit}") | $(metric_change "${previous_value}" "${current_value}" "${metric_unit}") | ${judgment} |"
    done
  fi

  echo
  echo "## 自动检查结论"
  echo
  cached_failed="$(json_value "${report_result}" '.apache_bench.cached_path.failed_requests')"
  database_failed="$(json_value "${report_result}" '.apache_bench.database_path.failed_requests')"
  deep_database_failed="$(json_value "${report_result}" '.apache_bench.deep_database_path.failed_requests')"
  cold_status="$(json_value "${report_result}" '.cold_cache.status_code')"
  http_5xx="$(json_value "${report_result}" '.prometheus.http.error_5xx_percent')"
  if awk -v cached="${cached_failed:-0}" -v database="${database_failed:-0}" \
    -v deep="${deep_database_failed:-0}" -v cold="${cold_status:-0}" -v errors="${http_5xx:-0}" \
    'BEGIN { exit !((cached == 0) && (database == 0) && (deep == 0) && (cold == 200) && (errors == 0)) }'; then
    echo "- 基础守护指标通过：冷缓存请求成功，多轮压测失败数为 0，HTTP 5xx 为 0。"
  else
    echo "- 基础守护指标未通过：请先检查冷缓存状态、失败请求或 HTTP 5xx，再评价性能变化。"
  fi
  echo "- 缓存命中率由压测前后计数器差值计算，只覆盖本次测试区间；运行期间不得混入其他流量。"
  if [[ "${git_dirty}" == "true" ]]; then
    echo "- **当前工作区存在未提交改动；用于正式前后对比前，请先提交并重新生成基线。**"
  fi
  echo "- 低于 3% 的相对变化或低于 0.5 个百分点的比例变化标记为“基本持平”，用于过滤本机正常波动。"
  echo "- 本报告只适合相同机器、数据量、请求数和并发参数下的优化前后比较，不代表生产容量或 SLO。"
  echo
  echo "## 原始数据"
  echo
  echo "- 结构化指标：\`result.json\`"
  echo "- 终端输出：\`baseline-output.txt\`"
  echo "- ApacheBench 与 Prometheus 原始响应：\`raw/\`"
} >"${report_markdown}"

echo
echo "监控报告已生成：${report_markdown}"
echo "结构化指标：${report_result}"
