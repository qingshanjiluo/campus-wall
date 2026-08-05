#!/bin/bash
set -euo pipefail

echo "=== 停止服务 ==="

pm2 delete kun-api 2>/dev/null || true
pm2 delete kun-web 2>/dev/null || true
pm2 save

echo "=== 服务已停止 ==="
