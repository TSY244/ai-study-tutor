#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
deploy_dir="$repo_root/deploy"
secrets_dir="$deploy_dir/secrets"
data_dir="$repo_root/data"
backup_dir="$repo_root/backups"
report_dir="$repo_root/import-reports"

mkdir -p "$secrets_dir" "$data_dir" "$backup_dir" "$report_dir"
chmod 700 "$secrets_dir"
umask 077

if [ ! -f "$deploy_dir/.env" ]; then
  cp "$deploy_dir/.env.example" "$deploy_dir/.env"
fi

if [ "$(uname -s)" = "Linux" ]; then
  if ! grep -q '^TOOLS_UID=' "$deploy_dir/.env"; then
    printf '\nTOOLS_UID=%s\n' "$(id -u)" >> "$deploy_dir/.env"
  fi
  if ! grep -q '^TOOLS_GID=' "$deploy_dir/.env"; then
    printf 'TOOLS_GID=%s\n' "$(id -g)" >> "$deploy_dir/.env"
  fi
fi

if [ ! -f "$secrets_dir/postgres_password.txt" ]; then
  openssl rand -base64 32 > "$secrets_dir/postgres_password.txt"
fi

if [ ! -f "$secrets_dir/anthropic_api_key.txt" ]; then
  : > "$secrets_dir/anthropic_api_key.txt"
fi

chmod 600 "$deploy_dir/.env" "$secrets_dir/postgres_password.txt" "$secrets_dir/anthropic_api_key.txt"

# Bind-mounted Tools outputs must be writable without promoting one-shot
# maintenance containers to root. Docker Desktop normally accepts 10001; on
# Linux the deployer can override TOOLS_UID/TOOLS_GID in deploy/.env.
chmod 700 "$data_dir" "$backup_dir" "$report_dir"

echo "Deployment files initialized in $deploy_dir"
