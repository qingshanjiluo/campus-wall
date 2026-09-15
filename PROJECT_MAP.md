# 校园墙 CampusWall — 项目结构探测地图（PROJECT MAP）

> 探测时间：2026-09-15 | 探测方式：全仓库逐目录 + git 历史 + 线上可达性实测
> 用途：在动任何代码之前，固化"现状事实"，作为方案与开发的唯一依据。

---

## 0. 一句话结论

仓库里同时存在 **三套后端实现 + 两套前端 + 两个线上地址**，彼此没有单一事实来源（SSOT），且当前**线上 Pages 域名被一个无关的 Vue 应用占用**。这是项目"看起来 100% 完成却上不了线"的根本原因。

---

## 1. 顶层目录清单

| 目录 | 性质 | 是否纳入 git | 说明 |
|------|------|--------------|------|
| `app.py` / `database.py` / `templates/` / `static/` | 🔴 早期 Flask 原型（根级） | 是 | 单文件版，硬编码 secret，直连 `instance/campushub.db`。已被 `campus-wall/` 取代，**遗留物**。 |
| `campus-wall/` | 🟢 **主项目（Flask 应用工厂）** | 是 | 108 路由、21 模板、文档齐全（README/API/USAGE/DESIGN/GAP/REVIEW/ROADMAP/TODO）。 |
| `campus-wall/frontend/` | 🟢 纯静态前端（Pages 部署） | 是 | 20 个静态页 + Pages Functions API 反代 + `_redirects`。是真正面向线上的前端。 |
| `backend-worker/` | 🟡 Cloudflare Python Worker + D1（**未跟踪**） | **否（git status 里是 ??）** | Flask 的无服务器移植，103 路由，D1 迁移脚本 + 种子。**最新架构方向，但没进版本库、没配 CI。** |
| `references/` `参考/` `paper2gal_src/` `Live2D/` | ⚪ 参考素材/看板娘 | 混合 | 设计参考、kimi 原型、Live2D 资源、训练数据（yolov8n.pt/eng.traineddata 等）。非运行时依赖。 |
| `work/` `instance/` `__pycache__/` `.wrangler/` `.opencode-autopilot/` | ⚪ 缓存/运行时产物 | 否/部分 | Live2D 工作目录、SQLite 实例、构建缓存。 |
| `mimoclaw_workspace (2).tar.gz` | ⚪ 历史快照归档 | - | 17MB，内含更早版本的 campus-wall（`openclaw` 时代）。考古用。 |

---

## 2. 三套后端对照

| 维度 | 根级 `app.py` 原型 | `campus-wall/app/`（主项目） | `backend-worker/`（Cloudflare） |
|------|-------------------|------------------------------|--------------------------------|
| 运行形态 | Flask 单文件 | Flask 应用工厂 + 蓝图 | Cloudflare Python Worker (Pyodide/WASM) |
| 数据库 | sqlite 直连 | sqlite（标准库 sqlite3） | **D1**（异步 prepare/bind） |
| 路由数 | ~6 | **108**（含页面路由） | **103**（纯 API，无页面路由） |
| 密码哈希 | bcrypt | bcrypt | **PBKDF2-SHA256**（Pyodide 无 C 扩展） |
| JWT | PyJWT | PyJWT | 手写 HS256（hmac+base64url） |
| 功能覆盖 | 仅注册/登录/列表 | 全站（子站/帖子/交易/恋爱/树洞/商城/后台…） | 全站（与主项目对等移植） |
| 版本库 | 是 | 是 | **否（未 git add）** |
| 部署 CI | 无 | 无（文档写 PythonAnywhere/VPS） | **无**（wrangler 本地都跑不起来） |
| 密钥 | 硬编码 dev | 硬编码 dev fallback | wrangler.toml 明文 `dev-only-change-me` |

> 结论：**`campus-wall/app/` 与 `backend-worker/` 都是"完整"的**，二者功能对等，区别只是运行形态（服务器 vs Serverless）。根级原型可废弃。

---

## 3. 前端现状

- `campus-wall/app/templates/`：Jinja2 模板，由 Flask 直出（服务端渲染路线）。
- `campus-wall/frontend/`：纯静态 SPA-ish，`/api/*` 走 Pages Functions 反代到 `API_BASE`。**这是面向线上的前端。**
  - `pages/` 20 个 HTML，`static/js/` 已模块化（api/auth/common/icons/modal/utils/animations）。
  - `_redirects`：`/post/* → pages/post.html` 等动态路由重写；`/` → `pages/index.html`；兜底 404。
  - `functions/api/[[path]].ts`：读 `API_BASE` 环境变量反代；未设置时回退同源 `/api`。
  - `functions/static/uploads/[[path]].ts`：反代上传文件回传。
- ⚠️ 两套前端**并行维护**，功能有漂移（如 admin.html 主项目 18KB / frontend 28KB）。需明确以 `frontend/` 为线上唯一版本。

---

## 4. 线上可达性实测（关键！）

> ⚠️ 本表为 2026-09-15 首次探测记录；**R3 修正（09-16）**：下表第 1 行"被占用"与第 3 行"回落 HTML"的判断已被后续事实推翻——
> `campus-wall-673.pages.dev` 就是项目 `campus-wall` 的正式域名（CF 随机后缀），当前生产为 **_worker.js 高级控制**（干净路由 + /api 反代 + 404 兜底），
> `API_BASE` 已生效指向 `campus-wall-api.sifangzhiji.workers.dev`（D1 数据在线，但免费额度 row-read 已耗尽，见 docs/LAUNCH_CHECKLIST.md）。
> 当前权威架构以 **PLAN.md §1.0/§1.1 + docs/LAUNCH_CHECKLIST.md §0** 为准。

| 地址 | 实测（首次探测时） | 判定 |
|------|------|------|
| `https://campus-wall.pages.dev/` | 200，返回 Vue SPA「校园万能墙」，引用 `/assets/index-*.js` | ❌ ~~被无关 Vue 应用占用~~（实为另一旧项目缓存/后续已变化，现由本项目 CI 管理） |
| `https://6416e446.campus-wall-673.pages.dev/` | 200，返回「校园墙 · 连接校园 分享青春」 | ✅ 本项目前端 CI 部署（历次 run） |
| `-673` 前端的 `/api/stations` | ~~返回首页 HTML~~ | ✅ 现已返回 Worker JSON（_worker.js 反代 + API_BASE） |

- **GitHub Actions**：`Deploy to Cloudflare Pages` 成功 3 次（最近 2026-08-11），用 `secrets.CLOUDFLARE_API_TOKEN` + `secrets.CLOUDFLARE_ACCOUNT_ID`，projectName=`campus-wall`，directory=`campus-wall/frontend`。CI 里有 token，本地 `wrangler whoami` 未登录（凭据只在 CI Secrets 中）。
- **Cloudflare 账号**：缓存 `account_id=664cc8aa94cb585def8d27ec174fa417`，account=`Sifangzhiji@qq.com's Account`。
- **D1**：wrangler.toml 声明 `database_id=95ad3424-e78c-4480-ac6b-d675d1f7c3c9`（名为 `campus-wall-db`）。**未在远端验证是否真已创建 / 是否已迁移**。

---

## 5. 部署链路缺口（为什么"上不了线"）—— 2026-09-16 状态：全部已闭合/降级为运维项

1. ~~前端域名被占~~ → 已澄清：`campus-wall-673.pages.dev` 即本项目 Pages 正式域名，CI 正常。
2. ~~后端没有线上实例~~ → 已有 `campus-wall-api.sifangzhiji.workers.dev`（D1 绑定工作，wrangler 通道可用）；新代码经 CI 重复部署中。
3. ~~前后端未联通~~ → `_worker.js` 反代 + `API_BASE` 已通。
4. 密钥：`JWT_SECRET` 已走 secret 注入（CI `wrangler secret put`）。
5. ~~backend-worker 未进版本库~~ → 已入库，CI 双工作流（worker + pages）。
6. 【新风险】D1 免费层 row-read 50k/天：正在做查询合并降载 + KV 试验轨兜底（见 PLAN §1.0）。

---

## 6. 建议的单一事实来源（SSOT）路线（详见 PLAN.md）

- **后端**：以 `backend-worker/`（Cloudflare Worker + D1）为**上线主线**（Serverless、零常驻成本、CI 里 wrangler 原生可部署、与 Pages 同生态），先补齐"进库 + CI + D1 迁移种子 + 密钥"四件事。
- **前端**：以 `campus-wall/frontend/`（静态 Pages）为**唯一线上前端**，配 `API_BASE` 指向 Worker。
- **保留** `campus-wall/app/`（Flask）作为**本地开发/对照实现与文档来源**，不作为上线产物。
- **废弃** 根级 `app.py` 原型。
