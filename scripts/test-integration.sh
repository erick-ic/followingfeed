#!/usr/bin/env bash
set -euo pipefail

test_container="followingfeed-integration-$$"
test_database="followingfeed_integration"
test_user="followingfeed"
test_password="followingfeed-integration"
test_root_password="followingfeed-integration-root"

cleanup() {
  docker rm -f "$test_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "Starting disposable MySQL 8.0 container..."
docker run --detach --rm \
  --name "$test_container" \
  --env MYSQL_DATABASE="$test_database" \
  --env MYSQL_USER="$test_user" \
  --env MYSQL_PASSWORD="$test_password" \
  --env MYSQL_ROOT_PASSWORD="$test_root_password" \
  --publish 127.0.0.1::3306 \
  mysql:8.0 >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$test_container" mysqladmin ping \
    --host=127.0.0.1 \
    --user="$test_user" \
    --password="$test_password" \
    --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker exec "$test_container" mysqladmin ping \
  --host=127.0.0.1 \
  --user="$test_user" \
  --password="$test_password" \
  --silent >/dev/null 2>&1; then
  echo "MySQL did not become ready within 60 seconds" >&2
  exit 1
fi

port_address="$(docker port "$test_container" 3306/tcp)"
test_port="${port_address##*:}"
test_dsn="$test_user:$test_password@tcp(127.0.0.1:$test_port)/$test_database?charset=utf8mb4&parseTime=true&loc=UTC"

echo "Applying and validating migrations..."
MIGRATION_CHECK_DSN="$test_dsn" \
  go test -tags=migrationcheck ./migrations \
  -run '^TestMigrationsAgainstFreshMySQL$' -count=1

echo "Running MySQL integration tests..."
FOLLOWINGFEED_TEST_MYSQL_DSN="$test_dsn" \
FOLLOWINGFEED_CONFIG_FILE="/nonexistent/followingfeed-integration.yaml" \
FOLLOWINGFEED_MYSQL_DSN="$test_dsn" \
FOLLOWINGFEED_REDIS_ADDR="127.0.0.1:6379" \
FOLLOWINGFEED_JWT_ACCESS_TOKEN_KEY="integration-access-token-key-at-least-32-characters" \
FOLLOWINGFEED_JWT_REFRESH_TOKEN_KEY="integration-refresh-token-key-at-least-32-characters" \
  go test -tags=integration ./internal/repository/... ./ioc/... -count=1
