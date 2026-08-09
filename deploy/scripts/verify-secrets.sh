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

api_block=$(service_block api)
worker_block=$(service_block worker)
web_block=$(service_block web)

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

# Claude runtime: API and Worker share the same persistent HOME, settings file,
# Base URL, and app_data volume. The host Claude directory is never mounted.
for block in "$api_block" "$worker_block"; do
  printf '%s\n' "$block" | grep -q 'HOME: /var/lib/ai-study-tutor'
  printf '%s\n' "$block" | grep -q 'CLAUDE_CODE_SETTINGS_FILE: /var/lib/ai-study-tutor/.claude/settings.json'
  printf '%s\n' "$block" | grep -q 'source: app_data'
  printf '%s\n' "$block" | grep -q 'target: /var/lib/ai-study-tutor'
done

api_base_url=$(printf '%s\n' "$api_block" | sed -n 's/^      ANTHROPIC_BASE_URL: //p')
worker_base_url=$(printf '%s\n' "$worker_block" | sed -n 's/^      ANTHROPIC_BASE_URL: //p')
test -n "$api_base_url"
test "$api_base_url" = "$worker_base_url"

if compose config | grep -qiE 'CLAUDE_CODE_SETTINGS_DIR|/claude-code'; then
  echo "secret boundary failure: host Claude Code settings mount remains" >&2
  exit 1
fi

# Claude multi-key gateway: the config directory is mounted only into api and
# worker, only api is write-enabled, and web stays free of any Claude config.
printf '%s\n' "$worker_block" | grep -q 'CLAUDE_GATEWAY_CONFIG_FILE: /config/claude-gateway/keys.json'
printf '%s\n' "$worker_block" | grep -q 'target: /config/claude-gateway'
if printf '%s\n' "$worker_block" | grep -q 'CLAUDE_GATEWAY_WRITE_ENABLED'; then
  echo "secret boundary failure: worker sets CLAUDE_GATEWAY_WRITE_ENABLED" >&2
  exit 1
fi
printf '%s\n' "$api_block" | grep -q 'CLAUDE_GATEWAY_CONFIG_FILE: /config/claude-gateway/keys.json'
printf '%s\n' "$api_block" | grep -q 'CLAUDE_GATEWAY_WRITE_ENABLED: "true"'
printf '%s\n' "$api_block" | grep -q 'target: /config/claude-gateway'
for service in migrate web seed doctor backup restore maintenance-repair legacy-import db; do
  if service_block "$service" | grep -qiE 'CLAUDE_GATEWAY_CONFIG_FILE|CLAUDE_GATEWAY_WRITE_ENABLED|target: /config/claude-gateway'; then
    echo "secret boundary failure: $service has Claude gateway config or write access" >&2
    exit 1
  fi
done

# URL imports use one explicit allowlist in API and Worker. The API has no
# egress, so it defers DNS validation; Worker must still validate before fetch.
if ! printf '%s\n' "$api_block" | grep -q 'URL_ALLOWLIST:'; then
  echo "URL policy failure: api is missing URL_ALLOWLIST" >&2
  exit 1
fi
if ! printf '%s\n' "$worker_block" | grep -q 'URL_ALLOWLIST:'; then
  echo "URL policy failure: worker is missing URL_ALLOWLIST" >&2
  exit 1
fi
if ! printf '%s\n' "$api_block" | grep -q 'URL_API_DNS_VALIDATION: \"false\"'; then
  echo "URL policy failure: no-egress api must defer DNS validation to worker" >&2
  exit 1
fi

if printf '%s\n' "$web_block" | grep -qiE '^[[:space:]]+(CLAUDE|ANTHROPIC)|anthropic_api_key|source: .*claude|target: .*claude'; then
  echo "secret boundary failure: web has Claude gateway config, env, or secret" >&2
  exit 1
fi

echo "secret boundary verified: only worker mounts anthropic_api_key"
echo "gateway boundary verified: config mounted into api (write-enabled) and worker; shared runtime Claude settings reach api and worker; web is clean"
