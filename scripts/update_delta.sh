#!/bin/zsh
set -euo pipefail

cd /Users/temirlankultumanov/go/src/myprojects/freedom_bitrix

COOLDOWN_SECONDS=$((2 * 60 * 60))
STATE_FILE="/tmp/freedom_bitrix_last_delta_run_at"

DOCKER_BIN="/opt/homebrew/bin/docker"
if [[ ! -x "$DOCKER_BIN" ]]; then
  DOCKER_BIN="/usr/local/bin/docker"
fi

if [[ ! -x "$DOCKER_BIN" ]]; then
  echo "docker binary not found" >&2
  exit 127
fi

now_epoch=$(date +%s)
last_epoch=0
if [[ -f "$STATE_FILE" ]]; then
  last_epoch=$(cat "$STATE_FILE" 2>/dev/null || echo 0)
fi

if [[ "$last_epoch" =~ ^[0-9]+$ ]]; then
  elapsed=$((now_epoch - last_epoch))
else
  elapsed=$COOLDOWN_SECONDS
fi

if (( elapsed < COOLDOWN_SECONDS )); then
  echo "skip delta sync: cooldown active (${elapsed}s elapsed, need ${COOLDOWN_SECONDS}s)" >&2
  exit 0
fi

# Ensure required services are up.
"$DOCKER_BIN" compose --env-file .env.docker up -d postgres api

# Run incremental sync.
"$DOCKER_BIN" compose --env-file .env.docker run --rm api delta

echo "$now_epoch" > "$STATE_FILE"
