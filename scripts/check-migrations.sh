#!/usr/bin/env bash

set -euo pipefail

check_container="followingfeed-migration-check-$$"
check_database="followingfeed_migration_check"
check_user="followingfeed"
check_password="followingfeed-migration-check"
check_root_password="followingfeed-migration-check-root"

cleanup() {
  docker rm -f "$check_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[migration-check] 启动临时 MySQL 8.0"
docker run --detach --rm \
  --name "$check_container" \
  --env "MYSQL_DATABASE=$check_database" \
  --env "MYSQL_USER=$check_user" \
  --env "MYSQL_PASSWORD=$check_password" \
  --env "MYSQL_ROOT_PASSWORD=$check_root_password" \
  --publish 127.0.0.1::3306 \
  mysql:8.0 >/dev/null

ready=false
for _ in $(seq 1 60); do
  if docker exec "$check_container" \
    mysqladmin ping -h 127.0.0.1 -u"$check_user" -p"$check_password" --silent >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done

if [[ "$ready" != "true" ]]; then
  echo "[migration-check] MySQL 未在 60 秒内就绪" >&2
  docker logs "$check_container" >&2
  exit 1
fi

port_address="$(docker port "$check_container" 3306/tcp)"
check_port="${port_address##*:}"

echo "[migration-check] 在空数据库执行并验证全部迁移"
MIGRATION_CHECK_DSN="$check_user:$check_password@tcp(127.0.0.1:$check_port)/$check_database?charset=utf8mb4&parseTime=true&loc=UTC" \
  go test -tags=migrationcheck ./migrations -run '^TestMigrationsAgainstFreshMySQL$' -count=1

echo "[migration-check] 检查通过，删除临时 MySQL"
