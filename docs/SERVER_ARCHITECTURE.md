# 服务器自托管架构与实施计划（R4 主线，取代 Cloudflare 方案）

> 决策日期：2026-09-16 · 依据用户指示「开发架构不要 cloudflare 部署，转为正常服务器完整部署，自主规划架构/功能/UI」。
> Cloudflare 轨道（backend-worker + Pages）**降级为线上演示/参考实现**，代码保留不删（其 API 契约是 Flask 对齐蓝本）。

## 1. 目标形态

一台普通 VPS（1c2g，Ubuntu 22.04+）跑起全站，`docker compose up -d` 一键部署；无 Docker 时提供 systemd+venv 备选路径。

```
用户 ─HTTPS─▶ Nginx :443/:80
               ├─ /            → 静态直出 campus-wall/frontend（21 页，同源 API，零反代）
               ├─ /static/     → 同上（js/css/img）
               ├─ /api/        → proxy_pass gunicorn :8000（4 sync worker，Flask app factory）
               └─ /uploads/    → 磁盘直出（上传图片，经 app 校验后落 data/uploads）
                     Flask (campus-wall/app) ── SQLite WAL（data/campus.db）
                                    └── DATABASE_URL 可切 Postgres（SQLAlchemy 层已在，保留迁移性）
```

**关键决策**：
- **前端用 `campus-wall/frontend/` 静态 21 页**（本会话已完成 XSS/契约/P0P1 修复 + JS 门禁），Flask 的 templates 服务端渲染版冻结；页面里 `API_BASE` 同源相对路径即可（api.js 已支持）。
- **后端以 Flask 主项目为生产主线**，端点/响应形状逐项对齐 `backend-worker`（Worker 即蓝本），差异清单来自 `tools/e2e_live.ps1`（首轮本地 27/38）。
- 数据**重新播种**（Flask seed.py 更新为与 Worker seed 同源数据：lucide 图标名、15 子站、bio/avatar、投票样例帖），旧 instance 库废弃。
- 上传落**磁盘** `data/uploads`（volume 持久化），不再用 D1-base64。

## 2. 功能完善（相对现状的增量）

| 域 | 内容 | 来源/新做 |
|---|---|---|
| 契约缺口 | 投票链路、link_url shaping、/api/growth、/api/trades、/api/search、/api/notifications、romance UPDATE 修复 | 从 Worker 移植 |
| 图片 | 上传持久化 + Pillow 压缩（长边 1600px、EXIF 剥离、webp 输出） | 新做 |
| 安全 | flask-limiter：登录 5/min、注册 3/h、发帖 10/min、上传 20/h；生产 CORS 收敛；安全响应头（Nginx） | 新做 |
| 审核 | 敏感词表（data/words.txt，发帖/评论/树洞命中→进 review 队列 status=pending）；管理后台审核页（通过/驳回/删除+举报队列合并） | 新做（reports_bp 已有底座） |
| 私信 | DM：conversations + messages 两张新表；GET /api/dm/conversations、POST /api/dm/<uid>/send、GET /api/dm/<uid>；通知联动 | 新做 |
| 前端配套 | 私信页 pages/messages.html + 聊天窗 UI；admin 页新增「审核」「私信」Tab；深色模式（variables.css 已有 token → 加 [data-theme=dark] + 开关持久化 localStorage） | 新做 |

## 3. 部署产物（campus-wall/deploy/）

- `Dockerfile`（python:3.12-slim，gunicorn 入口，healthcheck）
- `docker-compose.yml`（web + nginx + 每日 sqlite 备份 sidecar cron）
- `nginx.conf`（TLS 占位注释、gzip、静态缓存、限传 /api body 上限、安全头）
- `.env.example`（JWT_SECRET/SECRET_KEY/SMTP/CORS_ORIGINS/DATABASE_URL）
- `systemd/`（campuswall.service + gunicorn unit 备选）
- `scripts/deploy.sh`（git pull → build → up -d → 健康检查 → 跑 e2e）
- `scripts/backup.sh`（sqlite3 .backup + uploads tar，滚动 7 份）
- docs/DEPLOYMENT-SERVER.md（从零到 HTTPS 全流程，含 certbot）

## 4. 里程碑

- **M1 契约对齐**：10 项缺口全修，本地 `e2e_live.ps1 -BaseUrl :8000` 38/38 ✅
- **M2 图片+安全**：uploads 磁盘化+压缩、limiter、CORS 收敛、Nginx 头
- **M3 审核**：敏感词 + pending 队列 + admin UI Tab
- **M4 私信**：后端两张表三端点 + 前端 messages 页 + 通知
- **M5 UI**：深色模式、页面打磨、JS 门禁与爬检工具适配本地全栈
- **M6 部署包**：deploy/ 产物 + 文档 + 本地 docker compose 冒烟（若本机无 Docker 则以 compose config/语法校验 + gunicorn 裸跑等效验证并说明）
- **M7 收尾**：README/PLAN/LAUNCH_CHECKLIST 重定位（CF→服务器主线），归档 Cloudflare 章节

## 5. 风险与对策
- 旧 templates 版仍在工作目录：保留但文档声明冻结，避免误改双份。
- SQLite 并发：WAL + gunicorn `--preload` + 每请求短连接；写重时切 Postgres（URL 一改即可）。
- Windows 本地开发差异：路径/编码全用 pathlib + utf-8；E2E 用 pwsh 跑。
