# 校园墙 · 生产部署指南（Cloudflare Tunnel）

本文档说明如何将校园墙部署到生产环境，并通过 **Cloudflare Tunnel** 暴露到公网，支持 100+ 用户。

## 架构总览

```
用户 ──HTTPS──> Cloudflare 边缘 ──Cloudflare Tunnel──> 你的服务器（gunicorn + Flask + SQLite）
```

- 无需公网 IP、无需开放端口、无需备案（若用 Cloudflare 域名）
- Flask 应用保持 SQLite 不变（100 人量级完全够用）
- gunicorn 作为 WSGI 服务器（多 worker 并发）

## 一、准备

### 1.1 服务器要求
- 任意 Linux 服务器（或 Windows Server，本指南以 Ubuntu 为例）
- Python 3.10+
- 至少 512MB 内存

### 1.2 域名
- 在 Cloudflare 添加域名（例：`campuswall.example.com`）
- Cloudflare 免费计划即可

## 二、安装应用

```bash
# 1. 拉取代码
git clone https://github.com/qingshanjiluo/campus-wall.git
cd campus-wall/campus-wall

# 2. 创建虚拟环境
python3 -m venv venv
source venv/bin/activate

# 3. 安装依赖
pip install -r requirements.txt

# 4. 设置环境变量（重要！）
export JWT_SECRET="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export SECRET_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export FLASK_ENV="production"
export SITE_BASE_URL="https://campuswall.example.com"
# SMTP（可选，用于找回密码发邮件；不配则开发模式回显token）
# export SMTP_HOST="smtp.example.com"
# export SMTP_PORT="587"
# export SMTP_USER="noreply@example.com"
# export SMTP_PASSWORD="your-password"
# export SMTP_TLS="1"

# 5. 初始化数据库（自动创建）
python run.py  # 首次运行会自动建表并写入种子数据，看到"Running"后 Ctrl+C
```

## 三、用 gunicorn 启动

```bash
# 前台测试
gunicorn -w 3 -b 127.0.0.1:8000 "run:app"

# 确认可用后，用 systemd 托管（见下）
```

### 3.1 systemd 服务（推荐）

创建 `/etc/systemd/system/campuswall.service`：

```ini
[Unit]
Description=CampusWall Web Service
After=network.target

[Service]
User=www-data
WorkingDirectory=/opt/campus-wall/campus-wall
Environment="JWT_SECRET=CHANGE_ME"
Environment="SECRET_KEY=CHANGE_ME"
Environment="FLASK_ENV=production"
Environment="SITE_BASE_URL=https://campuswall.example.com"
ExecStart=/opt/campus-wall/campus-wall/venv/bin/gunicorn -w 3 -b 127.0.0.1:8000 --timeout 60 "run:app"
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable campuswall
sudo systemctl start campuswall
sudo systemctl status campuswall
```

## 四、安装并配置 Cloudflare Tunnel

### 4.1 安装 cloudflared

```bash
# 下载二进制（x86_64 Linux）
sudo wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O /usr/local/bin/cloudflared
sudo chmod +x /usr/local/bin/cloudflared
```

### 4.2 登录并创建隧道

```bash
cloudflared tunnel login
# 浏览器授权后，会自动写入 ~/.cloudflared/cert.pem

# 创建隧道
cloudflared tunnel create campuswall
```

### 4.3 配置隧道

创建 `~/.cloudflared/config.yml`：

```yaml
tunnel: <TUNNEL_ID>          # cloudflared tunnel list 查看
credentials-file: /home/USER/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: campuswall.example.com
    service: http://127.0.0.1:8000
  - service: http_status:404
```

### 4.4 配置 DNS

```bash
cloudflared tunnel route dns campuswall campuswall.example.com
```

### 4.5 启动隧道服务

```bash
# 测试
cloudflared tunnel run campuswall

# 配置为系统服务（后台常驻）
sudo cloudflared service install
sudo systemctl start cloudflared
sudo systemctl enable cloudflared
```

## 五、验证

```bash
curl -I https://campuswall.example.com/
# 应返回 200
```

## 六、管理员账号

种子数据自带管理员：
- 用户名：`admin`
- 密码：`admin123`

⚠️ **首次登录后请立即修改密码**，并设置 `JWT_SECRET`/`SECRET_KEY` 环境变量。

## 七、数据备份

SQLite 数据库位于 `campus-wall/instance/campushub.db`。

```bash
# 每日备份（crontab）
0 3 * * * cp /opt/campus-wall/campus-wall/instance/campushub.db /backup/campushub-$(date +\%Y\%m\%d).db
```

## 八、安全建议

1. **必须设置强 `JWT_SECRET` 和 `SECRET_KEY`**，否则应用拒绝在生产环境启动
2. 通过 Cloudflare 控制台开启 HTTPS（自动）
3. 可开启 Cloudflare 的 Bot Fight Mode / WAF 保护
4. 定期备份数据库
5. 生产环境不要用 `debug=True`（run.py 已按 `FLASK_ENV` 自动判断）

## 九、扩容（超过 100 人时）

当用户量显著增长，可考虑：
1. 增加 gunicorn worker 数（`-w 5`）
2. 迁移 SQLite → PostgreSQL（改 `app/models.py` 的连接层）
3. 用 Cloudflare 缓存静态资源
