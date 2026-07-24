#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose() {
  docker compose --env-file "$deploy_dir/.env" -f "$deploy_dir/compose.yaml" --profile tools --profile manual "$@"
}

service_block() {
  compose config | awk -v service="$1" '
    $0 == "  " service ":" { inside = 1; next }
    inside && $0 ~ /^  [A-Za-z0-9_-]+:$/ { exit }
    inside { print }
  '
}

worker_block=$(service_block worker)
printf '%s\n' "$worker_block" | grep -q 'anthropic_api_key'

for service in api migrate web seed doctor backup restore maintenance-repair legacy-import db; do
  if service_block "$service" | grep -q 'anthropic_api_key'; then
    echo "secret boundary failure: $service mounts anthropic_api_key" >&2
    exit 1
  fi
done

if compose config | grep -q '^ *ANTHROPIC_API_KEY:'; then
  echo "secret boundary failure: plain ANTHROPIC_API_KEY environment variable found" >&2
  exit 1
fi

echo "secret boundary verified: only worker mounts anthropic_api_key"
