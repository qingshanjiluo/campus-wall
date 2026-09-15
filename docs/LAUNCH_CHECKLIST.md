# CampusWall 上线检查清单（Launch Checklist）

> 状态图例：`[x]` 已验证 · `[~]` 进行中 · `[ ]` 待办
> 更新时间：见 git log（本文件随里程碑推进更新）

## 0. 架构总览（实际落地形态）

```
浏览器
  └─ https://campus-wall-673.pages.dev          Cloudflare Pages（静态前端 + Functions）
       ├─ /pages/*.html 等 21 页静态资源
       ├─ _redirects：/ → index，/post/:id → post.html …（干净路由）
       └─ functions/api/[[path]].ts  反代  ── API_BASE ──┐
                                                          ▼
                       https://campus-wall-api.sifangzhiji.workers.dev   Cloudflare Python Worker
                         ├─ src/entry.py：103 条正则路由
                         ├─ 数据层 src/db.py：双后端
                         │    • KV（当前上线）：分片 SQL 快照（puresql/sqlite3 引擎）
                         │    • D1（升级路径）：token 获得 D1:Edit 后切回，migrations 保留
                         └─ uploads：图片 base64 存 KV uploads 分片
```

- CI 全部走 GitHub Actions（本机无 wrangler 登录态）：
  - `deploy-cloudflare-worker.yml`：建/对齐 KV → 注入 JWT secret → 部署 Worker
  - `deploy-cloudflare-pages.yml`：push `campus-wall/frontend/**` → Pages Direct Upload
- 密钥：`JWT_SECRET` 仅存在于 Worker secrets；`.dev.vars` 本地开发（已 gitignore）

## 1. 后端正确性（本地可证明的全部）

- [x] 原生测试套件（sqlite3 驱动 D1-shim，真 handler 链路）：**96/96 断言全绿**
  - 覆盖：注册/登录/JWT、子站 CRUD+成员+转让、发帖/编辑/删除/置顶、点赞、评论（含楼中楼）、
    签到/等级/商店/道具使用、交易、恋爱档案（含 hobbies 数组回归）、树洞、举报、
    管理后台（stats/users/posts 审核）、通知、搜索、推荐、上传（base64 往返）
  - 新增（本轮审计修复）：投票一人一票全链路、link 帖 scheme 过滤、trade 价格/成色校验、
    admin today_* 字段、station_posts 匿名脱敏
- [x] Windows miniflare 不稳定问题定性为平台问题，以原生套件为业务权威证明
- [~] KV 后端移植 + 双模式（D1Shim / KVShim）套件全绿 —— 子代理进行中

## 2. 前端契约（docs/frontend-audit.md 逐项）

- [x] P0-1 waterfall `{posts,total}` 解包
- [x] P0-2 投票/链接帖整链（后端补齐 + 契约对齐，前端零改动即工作）
- [x] P1 toast 不可见（`.toast.show` 体系）
- [x] P1 存储型 XSS：escHtml 全局加引号转义 + `safeUrl`；avatar/cover/price/condition 点位收敛
- [x] P1 弹窗误删 `#loginModal`（5 处 `closest` 定位）
- [x] P1 admin 仪表盘 today_posts/today_users undefined
- [x] P1 /create 页图片选择器不匹配
- [x] P1 移动端 toggleNavMenu 未挂 window
- [x] P1 romance hobbies 数组二次保存 500（后端 json.dumps）
- [x] P2 getStationIcon lucide 直通 / profile EXP 读 exp / only_owner_posts 回填 / 上传限制对齐 5MB
- [ ] P2 重复函数收敛（openModal/showToast/escHtml 多处声明）——低风险重构，上线后做
- [ ] P2 后端已有但前端未接：`PUT /api/trade/:id/status`、identity 认证、interests 推荐调权、transactions 页
- [ ] P2 lucide 固定版本 + 本地兜底
- [ ] 线上浏览器冒烟（真实数据下 21 页走查）——Pages 部署完成后

## 3. 部署与运维

- [x] Workers 部署权限验证（probe worker 成功上线又删除）
- [x] KV 建/写/删权限验证
- [x] Pages 生产 = 项目 `campus-wall`（域名 `campus-wall-673.pages.dev`，CI 一直正确）
- [x] CI：workers.dev 子域自动保障；KV namespace create-or-get；secret 注入
- [ ] Worker 首次部署成功（KV 模式）→ 线上 `/api/stations` 返回 seed 数据
- [ ] Pages 设置 `API_BASE`（CI curl PATCH env_vars 或手动一次性）→ `/api` 不再 501
- [ ] 线上 E2E：注册→发帖→投票→上传图片→建子站→签到→兑换→交易→后台
- [ ] `wrangler tail` 无异常；错误率观察
- [ ] 删除临时 debug workflows（debug-cf-token.yml / debug-cf-pages.yml）

## 4. 已知限制与升级路径（上线时如实声明)

1. **D1 未启用**：当前 API token 无 `Account | D1 | Read+Edit` 权限，数据层运行在 KV 分片快照上
   （写多 shard 同步最终一致；单校 demo 规模内可靠）。拿到权限后升级：
   - 给 token 勾选 D1 权限 → wrangler.toml 恢复 `[[d1_databases]]`（文件内有标记）
   - CI 跑 `d1 execute --file migrations/0001_init.sql` → `tools/export_kv_to_sql.py` 生成数据迁移 SQL → 灌入
   - `d1 execute --remote` 验证行数一致后重新部署 Worker（DB binding 优先于 KV）
2. **workers.dev 在中国大陆连通性受限**：正式对外需绑定自定义域名（CF 面板 Worker & Pages 各绑一个，
   10 分钟操作；不影响功能验证与 demo）。
3. **KV 一致性**：读己之写有毫秒~秒级延迟窗口；论坛类交互（点赞/投票）UI 以本地状态回填兜底。
4. uploads 单文件 ≤5MB；全站无 CDN 级图片处理（缩放/水印未做）。
5. 匿名/审核为轻量规则（无敏感词库、无验证码、无速率限制——Worker CPU/配额内安全，但需上线后观察）。

## 5. 演示账号（seed）

| 账号 | 密码 | 角色 |
|------|------|------|
| admin | admin123 | 管理员 |
| xiaohua | 123456 | 普通用户 |

种子数据：8 用户 / 15 子站 / 25 帖子 / 16 评论 + 商店/签到/交易样例。
上线后如需清场：删除 KV `db-shard:*` 键即可自动重建（生产环境慎用）。
