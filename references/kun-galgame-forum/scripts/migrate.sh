#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR/apps/api"

echo "=== 执行数据库迁移 ==="
go run ./cmd/migrate -dir up
echo "=== 迁移完成 ==="
