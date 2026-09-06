#!/usr/bin/env bash
set -euo pipefail

# 仅创建随机命名的一次性依赖，绝不使用开发/生产数据库。
test_mysql="followingfeed-e2e-mysql-$$"
test_redis="followingfeed-e2e-redis-$$"
cleanup() {
  docker rm -f "$test_mysql" "$test_redis" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --rm --name "$test_mysql" \
  -e MYSQL_DATABASE=followingfeed_e2e -e MYSQL_USER=followingfeed \
  -e MYSQL_PASSWORD=e2e-only-password -e MYSQL_ROOT_PASSWORD=e2e-only-root \
  -p 127.0.0.1::3306 mysql:8.0 >/dev/null
docker run -d --rm --name "$test_redis" -p 127.0.0.1::6379 redis:7.4.7-alpine \
  redis-server --save '' --appendonly no >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$test_mysql" mysqladmin ping -h 127.0.0.1 -ufollowingfeed -pe2e-only-password --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$test_mysql" mysqladmin ping -h 127.0.0.1 -ufollowingfeed -pe2e-only-password --silent >/dev/null 2>&1
docker exec "$test_redis" redis-cli ping >/dev/null
mysql_address="$(docker port "$test_mysql" 3306/tcp)"
redis_address="$(docker port "$test_redis" 6379/tcp)"

FOLLOWINGFEED_ENV=development \
FOLLOWINGFEED_CORS_ALLOWED_ORIGINS=http://localhost:3000 \
FOLLOWINGFEED_CONFIG_FILE=/nonexistent/followingfeed-e2e.yaml \
FOLLOWINGFEED_MYSQL_DSN="followingfeed:e2e-only-password@tcp(127.0.0.1:${mysql_address##*:})/followingfeed_e2e?charset=utf8mb4&parseTime=true&loc=UTC" \
FOLLOWINGFEED_REDIS_ADDR="$redis_address" \
FOLLOWINGFEED_REDIS_DB=0 FOLLOWINGFEED_REDIS_TLS=false \
FOLLOWINGFEED_REDIS_USERNAME='' FOLLOWINGFEED_REDIS_PASSWORD='' \
FOLLOWINGFEED_JWT_ACCESS_TOKEN_KEY=e2e-access-key-longer-than-32-characters \
FOLLOWINGFEED_JWT_REFRESH_TOKEN_KEY=e2e-refresh-key-longer-than-32-characters \
FOLLOWINGFEED_AUTH_SIGNUP_ENABLED=true FOLLOWINGFEED_AUTH_PUBLISHING_ENABLED=true \
FOLLOWINGFEED_AUTH_RATE_THRESHOLD=1000 FOLLOWINGFEED_RATE_LIMIT_THRESHOLD=1000 \
  go test -tags=e2e . -count=1
