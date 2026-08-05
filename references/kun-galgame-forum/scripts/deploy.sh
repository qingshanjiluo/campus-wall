#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo "=== 部署开始 ==="

echo "[1/4] 拉取最新代码..."
git pull --ff-only

echo "[2/4] 安装前端依赖..."
pnpm install --frozen-lockfile

echo "[3/4] 构建 Go 后端..."
cd apps/api
go mod tidy
go build -o ./build/server ./cmd/server
cd "$ROOT_DIR"

echo "[4/4] 构建 Nuxt 前端..."
pnpm run build:web

echo "=== 部署完成，请运行 pnpm run prod:restart 重启服务 ==="
