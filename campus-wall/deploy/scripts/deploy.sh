#!/usr/bin/env bash
# CampusWall 一键更新（服务器端执行）：拉代码 → 重建 → 滚动重启 → 冒烟检查
set -euo pipefail
cd "$(dirname "$0")/.."        # campus-wall/deploy

BRANCH="${BRANCH:-master}"
REPO_DIR="${REPO_DIR:-/opt/campuswall-src}"   # git 仓库所在路径

echo "==> 1/4 更新代码"
if [ -d "$REPO_DIR/.git" ]; then
  git -C "$REPO_DIR" fetch origin "$BRANCH"
  git -C "$REPO_DIR" reset --hard "origin/$BRANCH"
else
  echo "仓库不存在：$REPO_DIR（请先 git clone）" >&2; exit 1
fi

echo "==> 2/4 构建镜像"
docker compose build app

echo "==> 3/4 滚动重启（nginx 不动，app 秒级切换）"
docker compose up -d app
sleep 5

echo "==> 4/4 冒烟检查"
for i in 1 2 3 4 5; do
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1/api/stations || true)
  if [ "$code" = "200" ]; then echo "OK: /api/stations 200"; exit 0; fi
  echo "等待服务就绪…($i/5, http=$code)"; sleep 4
done
echo "!! 冒烟失败，最近日志：" >&2
docker compose logs --tail=40 app >&2
exit 1
