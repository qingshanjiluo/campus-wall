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

| 地址 | 实测 | 判定 |
|------|------|------|
| `https://campus-wall.pages.dev/` | 200，返回 Vue SPA「校园万能墙」，引用 `/assets/index-*.js` | ❌ **被一个无关的 Vue 应用占用**，不是本项目前端。其 `/api/*` 也返回该 Vue 首页 HTML。 |
| `https://6416e446.campus-wall-673.pages.dev/` | 200，返回「校园墙 · 连接校园 分享青春」，是本项目静态前端 | ✅ 本项目前端 CI 部署在此（历史 run 31478357943） |
| 上述 `-673` 前端的 `/api/stations` | 返回首页 HTML（因未配 `API_BASE` 且同源无后端） | ⚠️ 前端已上线但**后端未接通** |

- **GitHub Actions**：`Deploy to Cloudflare Pages` 成功 3 次（最近 2026-08-11），用 `secrets.CLOUDFLARE_API_TOKEN` + `secrets.CLOUDFLARE_ACCOUNT_ID`，projectName=`campus-wall`，directory=`campus-wall/frontend`。CI 里有 token，本地 `wrangler whoami` 未登录（凭据只在 CI Secrets 中）。
- **Cloudflare 账号**：缓存 `account_id=664cc8aa94cb585def8d27ec174fa417`，account=`Sifangzhiji@qq.com's Account`。
- **D1**：wrangler.toml 声明 `database_id=95ad3424-e78c-4480-ac6b-d675d1f7c3c9`（名为 `campus-wall-db`）。**未在远端验证是否真已创建 / 是否已迁移**。

---

## 5. 部署链路缺口（为什么"上不了线"）

1. **前端域名被占**：`campus-wall.pages.dev` 已不是我们的站点；CI 实际落在 `campus-wall-673`。需要理清：要么改用自有域名，要么用 `-673`，要么重建 Pages 项目。
2. **后端没有线上实例**：
   - Flask 主项目：无 PythonAnywhere/VPS 凭据，无部署脚本被验证。
   - Worker：无 CI 工作流部署它；本地 `pywrangler dev` 因 uv 创建 `cpython-3.13.2-emscripten-wasm32-musl` venv 失败（ModuleNotFoundError: 'python'）跑不起来；远端 D1 未验证。
3. **前后端未联通**：即便前端上线，`API_BASE` 未指向任何可用后端 → 全站数据接口 404/回落 HTML。
4. **密钥裸奔**：JWT_SECRET 三处全是 dev 值，生产必须换成 `wrangler secret` / 环境变量。
5. **backend-worker 未进版本库**：CI 无法构建它，等于"只存在于这台机器上"。

---

## 6. 建议的单一事实来源（SSOT）路线（详见 PLAN.md）

- **后端**：以 `backend-worker/`（Cloudflare Worker + D1）为**上线主线**（Serverless、零常驻成本、CI 里 wrangler 原生可部署、与 Pages 同生态），先补齐"进库 + CI + D1 迁移种子 + 密钥"四件事。
- **前端**：以 `campus-wall/frontend/`（静态 Pages）为**唯一线上前端**，配 `API_BASE` 指向 Worker。
- **保留** `campus-wall/app/`（Flask）作为**本地开发/对照实现与文档来源**，不作为上线产物。
- **废弃** 根级 `app.py` 原型。
