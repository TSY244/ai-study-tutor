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

if ! grep -q '^ANTHROPIC_BASE_URL=' "$deploy_dir/.env"; then
  printf '\nANTHROPIC_BASE_URL=https://api.anthropic.com\n' >> "$deploy_dir/.env"
fi

if ! grep -q '^URL_ALLOWLIST=' "$deploy_dir/.env"; then
  printf '\nURL_ALLOWLIST=youtube.com,bilibili.com\n' >> "$deploy_dir/.env"
fi
if ! grep -q '^URL_API_DNS_VALIDATION=' "$deploy_dir/.env"; then
  printf 'URL_API_DNS_VALIDATION=false\n' >> "$deploy_dir/.env"
fi

# Compose resolves relative bind-mount paths from deploy/; honor an operator's
# CLAUDE_GATEWAY_CONFIG_DIR override rather than always creating the default.
gateway_config_value=$(sed -n 's/^[[:space:]]*CLAUDE_GATEWAY_CONFIG_DIR[[:space:]]*=[[:space:]]*//p' "$deploy_dir/.env")
gateway_config_value=$(printf '%s' "$gateway_config_value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
if [ -z "$gateway_config_value" ]; then
  gateway_config_value="../data/claude-gateway"
fi
case "$gateway_config_value" in
  /*) gateway_config_dir="$gateway_config_value" ;;
  *) gateway_config_dir="$deploy_dir/$gateway_config_value" ;;
esac
mkdir -p "$gateway_config_dir"
chmod 700 "$gateway_config_dir"

# 站点 Cookie 配置目录（B 站 bilibili.txt）：API 读写、Worker 只读；
# 同样支持 COOKIES_CONFIG_DIR 覆盖。
cookies_config_value=$(sed -n 's/^[[:space:]]*COOKIES_CONFIG_DIR[[:space:]]*=[[:space:]]*//p' "$deploy_dir/.env")
cookies_config_value=$(printf '%s' "$cookies_config_value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
if [ -z "$cookies_config_value" ]; then
  cookies_config_value="../data/cookies"
fi
case "$cookies_config_value" in
  /*) cookies_config_dir="$cookies_config_value" ;;
  *) cookies_config_dir="$deploy_dir/$cookies_config_value" ;;
esac
mkdir -p "$cookies_config_dir"
chmod 700 "$cookies_config_dir"

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
