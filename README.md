# 校园墙 CampusWall

> 连接校园，分享青春 —— 面向高校学生的社区站：子站（表白墙/失物招领/二手交易…）、瀑布流、匿名树洞、恋爱匹配、签到积分商店、二手市场、管理后台。

| | |
|---|---|
| 🌐 **线上地址** | https://campus-wall-673.pages.dev |
| 🔌 **API（线上）** | https://campus-wall-673.pages.dev/api/（同源反代）· https://campus-wall-api.sifangzhiji.workers.dev/api/（直连） |
| 👤 **演示账号** | 管理员 `admin / admin123` · 普通用户 `xiaohua / 123456` |
| 📦 **仓库** | qingshanjiluo/campus-wall（master = 生产） |

## 架构

```
Cloudflare Pages (campus-wall-673.pages.dev)          Cloudflare Python Worker
  campus-wall/frontend  ·  Direct Upload/高级控制        campus-wall-api.sifangzhiji…      Cloudflare D1
  _worker.js: /api·/static/uploads 反代 + 干净路由 + 404   backend-worker/ · Pyodide           campus-wall-db
                                                              └── env.DB 自动迁移 + 种子（幂等）
```

- **前端** `campus-wall/frontend/`：纯静态 20 页（无框架），发布即 CI 自动部署。
- **后端** `backend-worker/`：Cloudflare **Python Worker**（stdlib+sqlite 同构层，本地 97 断言直跑），数据落 **D1**；`JWT_SECRET` 走 wrangler secret。
- **发布**：push `master` 即自动部署（见下表），零人工。
- 仓库内 Flask 版（`campus-wall/app/`）为对照实现/文档来源，非上线产物。

## 目录速览

```
campus-wall/frontend/    静态前端 + _worker.js（反代/路由）     → Pages
backend-worker/          Python Worker（src/ 路由与模型、migrations/、seed/） → Workers + D1
tools/                   e2e_live.ps1（线上 38 步冒烟）等
docs/                    LAUNCH_CHECKLIST.md · frontend-audit.md
PLAN.md / PROJECT_MAP.md 方案与结构考古  ·  campus-wall/DEPLOYMENT.md 部署手册
```

## 开发 & 验证

```bash
# 后端改动后必跑（97 断言，零网络，~30s）
cd backend-worker && python tools/run_native_tests.py

# 前端本地预览（任意静态服务器 + 指 API 到线上）
python -m http.server 8000 -d campus-wall/frontend

# 部署后线上端到端
powershell -ExecutionPolicy Bypass -File tools/e2e_live.ps1
```

| 变更路径 | 自动触发 |
|---|---|
| `campus-wall/frontend/**` | Deploy to Cloudflare Pages |
| `backend-worker/**` | Deploy CampusWall Worker（D1 迁移兜底 + secret） |

## 文档

- 部署与运维：[campus-wall/DEPLOYMENT.md](campus-wall/DEPLOYMENT.md)
- 上线检查清单：[docs/LAUNCH_CHECKLIST.md](docs/LAUNCH_CHECKLIST.md)
- 方案（含决策记录）：[PLAN.md](PLAN.md) · 结构考古：[PROJECT_MAP.md](PROJECT_MAP.md)
- 前端审计：[docs/frontend-audit.md](docs/frontend-audit.md)

## 已知限制（MVP）

- `*.workers.dev`/`*.pages.dev` 在中国大陆可达性波动 → 上线后绑自有域名（DEPLOYMENT §三）。
- D1 免费层 row-read 50k/天：已做查询预算化；重负载路径备有 KV 试验轨。
- 无限流/验证码；上传图片存 D1（≤5MB/张），起量后切 R2（开关已备）。
