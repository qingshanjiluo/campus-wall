#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo "=== 启动服务 ==="

# Start Go API
echo "[1/2] 启动 Go API..."
pm2 start apps/api/build/server \
  --name kun-api \
  --cwd "$ROOT_DIR/apps/api" \
  --log-date-format "YYYY-MM-DD HH:mm:ss" \
  --max-memory-restart 512M

# Start Nuxt
echo "[2/2] 启动 Nuxt..."
pm2 start apps/web/.output/server/index.mjs \
  --name kun-web \
  --cwd "$ROOT_DIR/apps/web" \
  --log-date-format "YYYY-MM-DD HH:mm:ss" \
  --max-memory-restart 1G \
  --node-args="--max-old-space-size=1024"

pm2 save
echo "=== 服务已启动 ==="
pm2 ls
