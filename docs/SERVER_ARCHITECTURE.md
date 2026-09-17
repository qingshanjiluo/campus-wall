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

## 4. 里程碑（执行结果）

- **M1 契约对齐** ✅：10 项缺口全修（vote/link/shape/batch/by-uid/romance…），
  本地 `e2e_live.ps1 -BaseUrl :5000` **38/38** 全绿
- **M2 图片+安全** ✅：上传内容校验+Pillow 压缩+EXIF 剥离、Flask-Limiter 全端点
  （register 8/h、login 12/min、发帖 20/min、私信 30/min…，`RATELIMIT_ENABLED=1` 生产开）、
  CORS/JWT 强制经 env；429 中文 JSON 实测生效
- **M3 审核** ✅：敏感词三级（block/review/approved）+ posts.status 零迁移 ALTER +
  可见性闸（作者/管理员例外）+ `/api/admin/review` + admin「帖子审核」Tab + 作者徽标；
  E2E +6 步
- **M4 私信** ✅：dm_messages 双索引表 + 4 端点（聚合 threads 单 SQL）+ messages.html
  双栏聊天（移动适配/未读/深链/8s 轮询）+ profile 私信按钮 + 通知联动；E2E +5 步 → **54/54**
- **M5 UI** ✅：深色模式（localStorage + prefers-color-scheme + 导航开关，全页覆盖）、
  页面路由矩阵 11/11、静态资源统一到 frontend/static（uploads 同盘）、JS 门 0 失败
- **M6 部署包** ✅：deploy/（Dockerfile/compose 四服务/nginx cache 分级/backup.py 实跑/
  deploy.sh/systemd/手册 docs/DEPLOYMENT-SERVER.md）；本机 Docker daemon 不可用，
  compose YAML/脚本语法与 backup 链路均本地验证，容器整备冒烟留待目标服务器执行
- **M7 收尾** ✅：README/DEPLOYMENT/LAUNCH_CHECKLIST 重定位为服务器主线，Cloudflare 轨
  标注演示+契约蓝本；seed 已是 lucide 图标与前端一致
- **R5** ✅：多图画廊 images[]（upload≤9 张/预览/渲染）、trade 状态机前端（越权 403）、
  like 服务端真计数、金币明细接入孤儿 `/api/shop/transactions`、重复 JS 函数单例化、
  lucide 本地自托管锁版（vendor）、P2-10 批量修复（checkin 文案/游离标签/搜索注入/admin 改名）；
  E2E → **57/57**
- **R6** ✅（新增四功能，见下）：
  ① 角色关系图 `/world` —— 多用户共同新建/串联维护角色节点与关系，/api/world/* 蓝图 +
  力导向 SVG 可视化（拖拽/点选/关系高亮/双向虚线）+ seed 示例；② 爆料台 `/expose` ——
  `post_type=expose` 恒匿名 + 先审后发（pending 人工审核门），连管理员都只见匿名名；
  ③ 公开论坛 `/forum` —— 子站=版块矩阵 + 跨版块最新热议聚合；④ 广告位保留 ——
  `site_config` 键值表 + `/api/site/config` + admin 广告位配置页 + common.js 注入 ad-slot 占位。
  另：`/api/recommend/posts` 支持 `station_id`（兴趣子站热门筛选真正生效）；E2E → **66/66**，
  严格页面矩阵（真实公开路由全 200）、cold-boot 冷库 66/66 复验

剩余人工项（需外部资源）：真实服务器 `docker compose up -d --build` →
对线上域名跑 66 步 E2E → 改 admin 密码 → 浏览器 25 页双主题走查 → 公告。

## 5. 风险与对策
- 旧 templates 版仍在工作目录：保留但文档声明冻结，避免误改双份。
- SQLite 并发：WAL + gunicorn `--preload` + 每请求短连接；写重时切 Postgres（URL 一改即可）。
- Windows 本地开发差异：路径/编码全用 pathlib + utf-8；E2E 用 pwsh 跑。
