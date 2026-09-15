# CampusWall 服务器部署手册（正式方案）

> 架构：**Nginx(80/443) → Gunicorn(4w×2t) → Flask → SQLite(WAL)**，前端为
> `campus-wall/frontend` 静态 21 页，由 Nginx/Flask 双路径托管。全部资产在
> `campus-wall/deploy/`，一条 compose 命令完成编排。Cloudflare Worker/Pages
> 轨道已降级为**演示环境**（代码保留，见文末）。

---

## 0. 服务器要求

| 项 | 最低 | 说明 |
|---|---|---|
| CPU/内存 | 1c / 1G | 2c2G 更稳（gunicorn×4 worker） |
| 磁盘 | 10G | 含上传件 + 每日备份（滚动 7 份） |
| 系统 | Ubuntu 22.04+ / Debian 12 | 其他发行版自行替换包管理 |
| 软件 | Docker + Compose v2 | 无 Docker 见 §6 systemd 路径 |
| 域名(可选) | A 记录指向服务器 | 用于 HTTPS 证书签发 |

```bash
# 安装 Docker（官方脚本）
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker
```

## 1. 拉取代码

```bash
sudo mkdir -p /opt && cd /opt
git clone https://github.com/qingshanjiluo/campus-wall.git campuswall-src
cd campuswall-src/campus-wall/deploy
```

## 2. 配置密钥（必做，跳过=事故）

```bash
cp .env.example .env
# 生成并填入 JWT_SECRET / SECRET_KEY（各一次）：
openssl rand -hex 32
```

- `SITE_BASE_URL`：填 `http://服务器IP` 或 `https://你的域名`（找回密码邮件用）。
- `CORS_ORIGINS`：起步可不设（默认 `*`），上域名后收紧为具体源。
- 限流默认在生产**开启**（`RATELIMIT_ENABLED=1` 已写死在 compose）：
  login 12/min、register 8/h、发帖 20/min、上传 10–15/min。

## 3. 启动

```bash
docker compose up -d --build
docker compose ps            # app 应为 healthy（约 20s）
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1/api/stations   # 期望 200
```

首次启动自动建表 + 灌入演示种子（admin/admin123 等 8 用户、15 子站、25 帖）。
**上线前务必删除或改密 admin 演示账号**：

```bash
docker compose exec app python -c "
from app import create_app; from app.models import verify_password, update_user, get_user_by_username
app=create_app()
with app.app_context():
    u=get_user_by_username('admin'); update_user(u['id'], password='<新密码>')
print('admin 密码已更新')"
```

## 4. HTTPS（有域名时）

```bash
# 4.1 先仅用 80 端口签证书
docker compose run --rm certbot certonly --webroot -w /var/www/certbot \
  -d campus.example.edu --email you@example.com --agree-tos --no-eff-email

# 4.2 nginx 增加 443 server（编辑 deploy/nginx/campuswall.conf 末尾追加：）
```
```nginx
server {
    listen 443 ssl http2;
    server_name campus.example.edu;
    ssl_certificate     /etc/letsencrypt/live/campus.example.edu/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/campus.example.edu/privkey.pem;
    # 其余 location 与 80 段一致，可抽成 include /etc/nginx/snippets/campuswall.conf
}
```
```bash
# 4.3 证书续期常驻（已在 compose 的 certbot 服务里，profiles=["tls"]）
docker compose --profile tls up -d certbot
```
建议将 80 段末尾改为对非 ACME 路径 `return 301 https://$host$request_uri;`，
并把 `.env` 的 `SITE_BASE_URL`/`CORS_ORIGINS` 改为 https 域名后 `docker compose up -d`。

## 5. 日常运维

```bash
# 更新发版（推代码后，在服务器执行）
bash deploy/scripts/deploy.sh

# 日志
docker compose logs -f app --tail=100
docker compose logs nginx

# 手动备份 / 查看备份
docker compose exec backup python /scripts/backup.py
docker compose exec backup ls -lh /backups

# 恢复备份（停 app → 解开 → 放回 volume → 起）
docker compose stop app
docker compose run --rm -v campuswall_campus_data:/data busybox sh -c 'rm -f /data/campushub.db*'
# 从 tar 包取出 campushub.db 放入 volume 挂载路径后：
docker compose up -d app

# 数据库体检（WAL 完整性）
docker compose exec app python -c "
import sqlite3; c=sqlite3.connect('/data/campushub.db')
print(c.execute('PRAGMA integrity_check').fetchone())"
```

## 6. 无 Docker 备选（systemd 直跑）

```bash
sudo useradd -r -m campuswall
sudo mkdir -p /srv/campuswall && sudo chown campuswall /srv/campuswall
# 以 campuswall 用户：clone 代码到 /srv/campuswall/campuswall，建 venv 装依赖
python -m venv /srv/campuswall/venv
/srv/campuswall/venv/bin/pip install -r /srv/campuswall/campuswall/campus-wall/requirements.txt
sudo cp deploy/systemd/campuswall.service /etc/systemd/system/
sudo cp deploy/.env.example /etc/campuswall.env && sudo nano /etc/campuswall.env  # 填密钥
sudo systemctl enable --now campuswall
# Nginx 站点配置同 §3（把 proxy_pass 指到 127.0.0.1:8000），uploads 直出指向
# /srv/campuswall/campuswall/campus-wall/frontend/static/uploads
```

## 7. 验证清单（上线 GO 门）

```bash
# 全功能 E2E（38 步，对线上域名跑）
powershell -ExecutionPolicy Bypass -File tools/e2e_live.ps1 -BaseUrl https://你的域名
# 期望输出：live E2E: pass=38 fail=0
```

人工抽查：
- [ ] `/` 首页瀑布流渲染、`/post/1` 详情、`/zzz` 出 404 页
- [ ] 注册→发帖(带图)→投票→评论→点赞→通知 全链路
- [ ] 匿名帖在详情/列表/子站页均显示"匿名用户"（作者本人登录可见）
- [ ] admin 后台登录、数据统计、用户管理
- [ ] 找回密码：邮件可达或开发 token 不外泄（生产 SMTP 配好）

## 8. 常见故障

| 症状 | 处置 |
|---|---|
| app 不 healthy | `docker compose logs app`：多半是 .env 缺密钥（compose 会直接拒启）|
| 502 | app 还在 seed/迁移，等 30s；持续 502 看 SQLite 磁盘满 |
| 图片 404 | volume 名带项目前缀（`campuswall_campus_uploads`），确认 nginx 与 app 挂同一 volume |
| database is locked | 并发写超 SQLite 极限：降 gunicorn worker 或迁 PostgreSQL（代码 `query_db/execute_db` 单点替换） |
| 429 频发 | 用户正常操作被限：上调 `deploy/` 中 limit 值（auth.py 各装饰器）并重建 |

## 9. Cloudflare 轨道（演示，非主线）

`backend-worker/`（Python Worker + D1）与 `campus-wall/frontend/_worker.js`（Pages 代理）
保留作架构参考与免费演示环境，**不再投入新功能**；新特性一律进 Flask 主线
（契约差异以 `backend-worker/src` 为准的修复已同步完毕）。
