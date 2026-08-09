#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose() {
  docker compose --env-file "$deploy_dir/.env" -f "$deploy_dir/compose.yaml" --profile tools --profile manual "$@"
}

compose_json=$(compose config --format json)
printf '%s\n' "$compose_json" | python3 "$deploy_dir/scripts/verify_compose_boundary.py"
