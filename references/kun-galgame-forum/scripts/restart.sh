#!/bin/bash
set -euo pipefail

echo "=== 重启服务 ==="

pm2 restart kun-api 2>/dev/null || echo "kun-api 未运行，跳过"
pm2 restart kun-web 2>/dev/null || echo "kun-web 未运行，跳过"

echo "=== 服务已重启 ==="
pm2 ls
