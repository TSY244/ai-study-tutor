#!/bin/sh
set -eu

base_url=${BASE_URL:-http://127.0.0.1:8080}
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

curl -fsS "$base_url/healthz" > "$tmp_dir/health.json"
curl -fsS "$base_url/readyz" > "$tmp_dir/ready.json"
curl -fsS "$base_url/api/v1/courses?limit=1" > "$tmp_dir/courses.json"
curl -fsS "$base_url/route-that-does-not-exist" > "$tmp_dir/fallback.html"
grep -q '<div id="root"></div>' "$tmp_dir/fallback.html"

set +e
curl -sS -N -D "$tmp_dir/sse.headers" --max-time 2 -o /dev/null \
  "$base_url/api/v1/jobs/stream"
sse_status=$?
set -e
if [ "$sse_status" -ne 0 ] && [ "$sse_status" -ne 28 ]; then
  echo "SSE request failed with exit code $sse_status" >&2
  exit "$sse_status"
fi
grep -qi '^content-type: text/event-stream' "$tmp_dir/sse.headers"

asset_path=$(sed -n 's/.*src="\(\/assets\/[^\"]*\.js\)".*/\1/p' "$tmp_dir/fallback.html" | head -n 1)
test -n "$asset_path"
curl -fsSI "$base_url$asset_path" > "$tmp_dir/asset.headers"
grep -qi '^cache-control: public, max-age=31536000, immutable' "$tmp_dir/asset.headers"

echo "release smoke passed: REST, SSE, SPA fallback, immutable assets"

