# 校园墙 CampusWall

> 连接校园，分享青春 —— 面向高校学生的社区站：子站（表白墙/失物招领/二手交易…）、
> 瀑布流、投票/链接帖、匿名树洞、私信、恋爱匹配、签到积分商店、二手市场、
> 敏感词+人工审核、管理后台、深色模式。

| | |
|---|---|
| 🚀 **正式部署** | 普通服务器自托管（Nginx + Gunicorn + Flask + SQLite）——[部署手册](docs/DEPLOYMENT-SERVER.md) |
| 🧪 **演示环境** | https://campus-wall-673.pages.dev（Cloudflare 轨，仅作参考，非主线） |
| 👤 **演示账号** | 管理员 `admin / admin123` · 普通用户 `xiaohua / 123456` |
| ✅ **质量门** | 本地全栈 E2E **49/49** 绿 · 原生断言 205 绿 · 内联/外链 JS `node --check` 0 失败 |
| 📦 **仓库** | qingshanjiluo/campus-wall（master = 生产） |

## 架构（主线）

```
用户浏览器
   │ 80/443
┌──▼────────────────────────── Nginx（TLS·gzip·静态缓存·真实IP透传）──────────┐
│  /static/uploads/ → volume 直出(30d)   /api/ + 页面 → proxy_pass           │
└──┬─────────────────────────────────────────────────────────────────────────┘
┌──▼───────────── Gunicorn 4w×2t（--preload）────────────────────────────────┐
│  Flask app（campus-wall/app）17+1 蓝图 · JWT · bcrypt · 限流 · 敏感词审核   │
│  SQLite WAL（DATABASE_PATH=/data volume，可平滑换 PostgreSQL）              │
└────────────────────────────────────────────────────────────────────────────┘
  前端 = campus-wall/frontend 静态 22 页（Nginx 直出静态资源 + Flask 页路由）
```

- **一键上线**：`cd campus-wall/deploy && cp .env.example .env`（填密钥）→ `docker compose up -d --build`
- **每日备份**：compose 内 cron sidecar（sqlite3 在线快照 + uploads，滚动 7 份）
- **更新发布**：`bash deploy/scripts/deploy.sh`（拉码→重建→冒烟）

## 目录速览

```
campus-wall/app/          Flask 后端主线（routes/18 蓝图、models*.py、seed.py）
campus-wall/frontend/     静态前端 22 页（线上唯一前端版本）
campus-wall/deploy/       Dockerfile · compose · nginx · systemd · 备份/发布脚本 · .env.example
backend-worker/           Cloudflare Python Worker + D1（演示轨 & API 契约蓝本）
tools/e2e_live.ps1        49 步全功能 E2E（-BaseUrl 可指 Flask/Worker 任一后端）
docs/                     DEPLOYMENT-SERVER · SERVER_ARCHITECTURE · LAUNCH_CHECKLIST · frontend-audit
```

## 开发 & 验证

```bash
# 起本地全栈（自动建表+种子）
cd campus-wall && python run.py            # http://127.0.0.1:5000

# 全功能 E2E（GO 门）
powershell -ExecutionPolicy Bypass -File tools/e2e_live.ps1 -BaseUrl http://127.0.0.1:5000

# 后端契约蓝本原生断言（可选）
cd backend-worker && python tools/run_native_tests.py --mode all --budget

# 生产环境（Linux 服务器）
cd campus-wall/deploy && docker compose up -d --build
```

## 功能地图（后端均已 E2E 覆盖）

| 域 | 亮点 |
|---|---|
| 帖子 | text/image/**vote/link** 四类型；投票一人一票；编辑保留票数；编辑历史 |
| 审核 | 敏感词三级（block 拒/review 队列/直发）· admin 审核 Tab · 作者状态徽标 |
| 私信 | 会话列表/未读徽标/深链 `?with=uid` · 移动端单栏 · 发信即通知 |
| 社区 | 子站(私密/仅站长发贴) · 树洞马甲 · 二手 · 恋爱匹配 · 签到/任务/商店 |
| 管理 | 统计+趋势图 · 用户/子站/帖子/身份组/商城/公告/举报/操作日志 |
| 安全 | JWT+bcrypt · 接口限流(生产开) · 上传防伪+Pillow 压缩+EXIF 剥离 · 匿名脱敏 |

## 文档

- 🧭 正式部署：[docs/DEPLOYMENT-SERVER.md](docs/DEPLOYMENT-SERVER.md)
- 🏗 架构定案：[docs/SERVER_ARCHITECTURE.md](docs/SERVER_ARCHITECTURE.md)
- ✅ 上线清单：[docs/LAUNCH_CHECKLIST.md](docs/LAUNCH_CHECKLIST.md)
- 🗂 方案与决策：[PLAN.md](PLAN.md) · 结构考古：[PROJECT_MAP.md](PROJECT_MAP.md)
- 🧪 Cloudflare 演示轨说明：[campus-wall/DEPLOYMENT.md](campus-wall/DEPLOYMENT.md)（已降级为参考）

## 已知限制 / 演进路线

- SQLite 单写者：WAL 下支撑 ~千日活无虞；起量按 `query_db/execute_db` 单点切 PostgreSQL。
- 限流默认 memory storage：多机部署时 `RATELIMIT_STORAGE_URI` 指 redis 即可。
- Pages 演示轨依赖 `*.pages.dev`（大陆可达性波动），正式站走自有域名+服务器。
