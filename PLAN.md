# CampusWall 上线方案（PLAN）

> 制定时间：2026-09-15 | 制定依据：PROJECT_MAP.md 探测事实 + backend-worker 原生测试台 80/80 全绿
> 本方案为上线唯一执行路线，按里程碑推进，全部自主执行。

---

## 一、决策

### 1.0 R2 修订：数据层 KV 化（权限现实驱动，2026-09-16）

CI 权限探测结论（token = sifangzhiji@qq.com 账户 API token）：
- ✅ Workers Scripts 部署（probe worker 真实上线又删除）；✅ KV 建/写/删；✅ Pages 部署；workers.dev 子域 = `sifangzhiji`
- ❌ **D1 读写（code 10000 Authentication error，token 未含 D1 scope）**；❌ R2（账户未开通）

决策：**上线构建 = Python Worker + KV**。db.py 双后端：KV 分片快照内嵌 SQL 引擎执行（业务代码零改动）；
D1 路径原样保留，token 勾选 `Account | D1 | Read+Edit` 后即按 `docs/LAUNCH_CHECKLIST.md` §4.1 升级回 D1，
数据经 `tools/export_kv_to_sql.py` 迁移。

### 1.1 上线架构（选定）

```
用户浏览器
  └─ Cloudflare Pages  (campus-wall-673.pages.dev，静态前端 frontend/)
       ├─ /pages/*.html + /static/*          （直出静态资源）
       └─ /api/* + /static/uploads/*  ──Pages Functions 反代──▶ Cloudflare Python Worker
                                                                     └─ D1 (campus-wall-db) + 内嵌 uploads
```

理由：
1. **零成本可部署**：账号已有（CI Secrets 持有 CLOUDFLARE_API_TOKEN），无需 VPS/PythonAnywhere 凭据（本机没有）。
2. **Worker 代码已验证**：native 测试台 80/80 全绿，bootstrap（建表+播种）设计已落地。
3. **Pages 前端已验证**：CI 曾成功部署（campus-wall-673），只差 API_BASE 接通。
4. 本地 Windows miniflare 对 Python Workers 有压测崩溃问题，**以线上真实 workerd 为最终验证环境**。

### 1.2 角色分配（三套代码）

| 代码 | 角色 | 处置 |
|------|------|------|
| `backend-worker/` | **线上后端（SSOT）** | 纳入 git，CI 部署，密钥走 `wrangler secret` |
| `campus-wall/frontend/` | **线上前端（SSOT）** | 保持 Pages 部署，functions 反代 → Worker |
| `campus-wall/app/`（Flask） | 本地开发参考/文档来源 | 不动，不上线 |
| 根级 `app.py` 原型 | 废弃 | 保留目录不再演进 |

---

## 二、里程碑

### M1 后端入库与 CI（本地凭据无关）
- [ ] `git add backend-worker`（排除 .venv*、.wrangler、__pycache__、smoke 临时文件）
- [ ] 新增 `.github/workflows/deploy-cloudflare-worker.yml`：
  - 触发：push 到 master 且 `backend-worker/**` 变更 / 手动 dispatch
  - 步骤：checkout → npx wrangler（npm ci）→ `wrangler d1 execute --remote`（幂等 IF NOT EXISTS）→ `wrangler deploy` → `wrangler secret put JWT_SECRET`（来自 repo secret `JWT_SECRET_PROD`）
  - 注意：wrangler 自动识别 Python worker（本地已证实 npx wrangler 4.122 可跑）
- [ ] `gh secret set JWT_SECRET_PROD`（随机 64 hex）
- [ ] 移除 wrangler.toml 明文 JWT_SECRET dev 值（保留 vars 其他项）

### M2 Worker 上线验证
- [ ] 触发部署，拿 workers.dev URL
- [ ] 线上 bootstrap 自举验证：首个 /api/stations 应 200（自动建表+播种；若远端 D1 不存在则 CI 先 d1 create + 回填 database_id）
- [ ] 冒烟：注册/登录/发帖/签到/商城/后台 全链路线上跑一遍

### M3 前端接通与修复
- [ ] 等子代理 frontend-audit.md → 按 P0/P1 修复前端
- [ ] Pages 项目设置 `API_BASE` = Worker URL：
  - 方案 A（优先）：CI 部署时调 CF API `PATCH /pages/projects/.../deployments` 设 production env
  - 方案 B（兜底）：functions/api/[[path]].ts 内置默认 Worker URL（构建期常量）
- [ ] 验证 Pages Functions 确实执行（此前一次部署 /api 回落 HTML，需查明原因：wrangler 版本/目录结构）
- [ ] 重新部署 Pages

### M4 线上端到端 + 收尾
- [ ] 浏览器级验证清单（用 HTTP 模拟 + 关键页面内容断言）：
  首页数据流、子站加入/发帖、帖子点赞评论、签到、商城购买、后台看板、上传头像
- [ ] 更新 DEPLOYMENT.md / README（Worker 路线为正式方案）
- [ ] 上线检查清单 GO/NO-GO（docs/LAUNCH_CHECKLIST.md）
- [ ] git 提交 + CI 全绿

---

## 三、风险与对策

| 风险 | 概率 | 对策 |
|------|------|------|
| D1 database_id 指向不存在库 | 中 | CI 里 `wrangler d1 info` 失败则 create + sed 回填 |
| Pages 项目 subdomain 混乱（campus-wall vs -673） | 中 | 明确部署到 `campus-wall-673`（我们已拥有），CI projectName 改正 |
| Pages Functions 不执行（前次部署 /api 回落 HTML） | 中 | 验证 wrangler pages functions 打包；不行则改 Worker 直出 assets（备胎：worker 加 assets binding，单应用架构） |
| Python Worker 线上仍不稳定 | 低 | 端点级探活 + 回退 Flask（暂无凭据，不推荐）；优先排查请求内全局态 `context.env` 并发覆盖问题（见附录） |
| workers.dev 被墙导致国内用户不可达 | 高（长期） | 上线后绑定自有域名可解；文档记录 |

### 附录：`context.env` 全局态并发风险（已知设计债）
entry.py 每请求 `context.env = self.env`，Python 在 isolate 内单线程事件循环，赋值幂等（同一 env 对象），**可接受**；但 `context.current_user` 若有跨请求残留需在 review 中确认每请求重置（fetch 入口已重置）。记录跟踪。

---

## 四、不做清单（明确排除，防止范围蔓延）

- Flask 版镜像部署（无凭据、非选定架构）
- 邮件找回密码 SMTP（Worker 无凭据；保持 dev-token 回显或在 UI 隐藏入口并给提示）
- R2 图床（未开通；uploads 走 D1 base64，5MB 上限，够 MVP）
- Live2D 看板娘资源优化（有 /api/kanban/message 文案轮播，足够）
