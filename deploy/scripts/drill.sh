#!/bin/sh
set -eu

if [ "${RUN_DEPLOYMENT_DRILLS:-}" != "1" ]; then
  echo "destructive deployment drills require RUN_DEPLOYMENT_DRILLS=1" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1 || \
  ! python3 -c 'import sys; raise SystemExit(sys.version_info < (3, 11))'; then
  echo "deployment drill requires Python 3.11+ for Compose boundary verification" >&2
  exit 2
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
deploy_dir="$repo_root/deploy"
project=${COMPOSE_PROJECT_NAME:-ai-study-tutor-drill}
port=${APP_PORT:-18080}
tmp_dir=$(mktemp -d)
backup_dir="$tmp_dir/backups"
mkdir -p "$backup_dir"

export AI_PROVIDER=fake
export APP_PORT="$port"
export APP_VERSION=drill
export BACKUP_DIR="$backup_dir"

compose() {
  docker compose --project-name "$project" --env-file "$deploy_dir/.env" -f "$deploy_dir/compose.yaml" "$@"
}

cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

base_url="http://127.0.0.1:$port"

compose down -v --remove-orphans >/dev/null 2>&1 || true
# Profiles are not included in the default `up --build`; build the shared
# Tools image explicitly so backup/restore exercise the current binaries.
compose --profile tools build backup
compose up -d --build --wait
curl -fsS "$base_url/healthz" >/dev/null
curl -fsS "$base_url/readyz" >/dev/null

# Fake Provider validates API enqueue -> Worker execution -> persisted result.
job_json=$(curl -fsS -X POST "$base_url/api/v1/system/claude-tests" -H 'Idempotency-Key: 018f9b3a-7e70-7c8a-af06-3be81b41a3e9')
job_id=$(printf '%s' "$job_json" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
test -n "$job_id"
for _ in $(seq 1 30); do
  job=$(curl -fsS "$base_url/api/v1/jobs/$job_id")
  printf '%s' "$job" | grep -q '"status":"succeeded"' && break
  sleep 1
done
printf '%s' "$job" | grep -q '"status":"succeeded"'

# Data must survive service restart, and a hard-killed Worker must restart.
compose --profile tools run --rm seed
curl -fsS "$base_url/api/v1/courses?limit=1" | grep -q '"lesson_count":2'
compose restart api worker
curl -fsS "$base_url/api/v1/courses?limit=1" | grep -q '"lesson_count":2'
worker_container=$(compose ps -q worker)
test -n "$worker_container"
# Signal PID 1 from inside the container: unlike `compose kill`, this is an
# unexpected process death and therefore exercises Docker's restart policy.
docker exec "$worker_container" sh -c 'kill -KILL 1' || true
for _ in $(seq 1 30); do
  compose ps --status running -q worker | grep -q . && break
  sleep 1
done
compose ps --status running -q worker | grep -q .

# Simulate a crashed maintenance owner and verify the only write-capable repair tool clears it.
compose exec -T db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "UPDATE system_state SET maintenance_mode=true, phase='\''draining'\'', maintenance_owner='\''drill-dead'\'', maintenance_lease_expires_at=now()-interval '\''1 second'\'' WHERE singleton=true"' >/dev/null
compose --profile manual run --rm maintenance-repair
compose --profile tools run --rm doctor | grep -q 'doctor 巡检正常'

compose --profile tools run --rm backup
backup_name=$(find "$backup_dir" -maxdepth 1 -type d -name 'backup-*' -exec basename {} \; | head -n 1)
test -n "$backup_name"

# Restore into Compose's newly created app_data/pg_data volumes.
compose down -v
compose up -d db --wait
RESTORE_BACKUP="$backup_name" RESTORE_CONFIRM=RESTORE compose --profile manual run --rm restore
compose up -d --wait
curl -fsS "$base_url/api/v1/courses?limit=1" | grep -q '"lesson_count":2'
BASE_URL="$base_url" "$deploy_dir/scripts/release-smoke.sh"
COMPOSE_PROJECT_NAME="$project" "$deploy_dir/scripts/verify-secrets.sh"

echo "deployment drill passed: startup, fake connectivity, persistence, worker kill, maintenance repair, backup/restore, SSE, secret boundary"
