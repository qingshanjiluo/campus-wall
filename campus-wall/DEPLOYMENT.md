# 校园墙 · 生产部署指南（Cloudflare Pages + Python Worker）

> **当前生产架构（2026-09 起，本文件为权威）**
>
> ```
> 浏览器
>   │ HTTPS
>   ▼
> Cloudflare Pages  campus-wall-673.pages.dev        ← 前端（Direct Upload / 高级控制）
>   │  _worker.js：/api/*、/static/uploads/* 反代 + 干净路由 + 404 兜底
>   ▼
> Cloudflare Python Worker  campus-wall-api.sifangzhiji.workers.dev   ← 后端 API
>   │  env.DB
>   ▼
> Cloudflare D1  campus-wall-db（SQLite 兼容，自动迁移 + 种子）
> ```
>
> 两条 GitHub Actions 链路自动完成全部部署（推 master 即发布）。

## 一、日常发布（自动）

| 触发 | 工作流 | 内容 |
|------|--------|------|
| push 涉及 `campus-wall/frontend/**` | `Deploy to Cloudflare Pages` | Direct Upload 整站（含 `_worker.js`），绑定 `campus-wall` 项目 production |
| push 涉及 `backend-worker/**` | `Deploy CampusWall Worker` | ①（可选）D1 建库/迁移 ②`wrangler deploy` Python Worker ③`wrangler secret put JWT_SECRET` |

预览环境：每个 PR 分支的 frontend 改动会生成 Pages preview 部署（同项目非 production）。

## 二、首次搭建 / 重建（手动一次性）

前提：有 Cloudflare 账号 + API Token（可 Workers Scripts、Pages 读写），仓库 Settings→Secrets 配置：

- `CLOUDFLARE_API_TOKEN`、`CLOUDFLARE_ACCOUNT_ID`
- `JWT_SECRET_PROD`（≥32 字节随机值：`python -c "import secrets;print(secrets.token_hex(32))"`）

```bash
# 1. Pages 项目（Direct Upload，创建后 production 分支设为 master）
npx wrangler pages project create campus-wall --production-branch master

# 2. D1 数据库（记下 uuid，写入 backend-worker/wrangler.toml 的 database_id）
npx wrangler d1 create campus-wall-db

# 3. 首次部署（此后全自动）
npx wrangler deploy --config backend-worker/wrangler.toml
JWT=… npx wrangler secret put JWT_SECRET --config backend-worker/wrangler.toml
npx wrangler pages deploy campus-wall/frontend --project-name campus-wall
```

D1 表结构与种子数据无需手工迁移：Worker 冷启动 `bootstrap.ensure_ready()` 自动执行（幂等）。

Pages 生产环境变量 `API_BASE` = Worker URL（当前以 `secret_text` 形式配置在
Pages → Settings → Environment variables；CI 会尝试同步为 `text`，失败仅告警不阻断）。

## 三、域名（推荐上线后 48h 内完成）

`*.workers.dev` 与 `*.pages.dev` 在中国大陆可达性不稳定，正式对外建议绑定自有域名：

1. Pages 项目 → Custom domains → 绑 `campus.example.com`（NS 托管到 CF 时自动签发证书）
2. Workers 域同理（或复用同一 Pages 项目反代：`API_BASE` 换成自定义 Worker 域）
3. 绑域后 `API_BASE` 若变：CI 下次部署会自动 PATCH；手工改亦可（Pages Settings）

## 四、验证清单（部署后必做）

```bash
# 前端 JS 语法门禁（21 页内联 + 公共模块 + _worker.js；已挂 Pages CI 防回归）
python tools/check_frontend_js.py

# 本地全量回归（97 断言）
cd backend-worker && python tools/run_native_tests.py

# 线上端到端（注册→发帖→投票→签到→商店→交易→表白→管理后台→上传，38 步）
powershell -ExecutionPolicy Bypass -File tools/e2e_live.ps1
```

人工抽查：首页/瀑布流/子站页/帖子详情/评论/点赞、`/404-garbage` 返回 404 页、移动端汉堡菜单、投票与链接帖渲染。

## 五、数据与备份

- **导出 D1**：`npx wrangler d1 export campus-wall-db --remote --output backup.sql`（建议每周 cron 一次，或发布前）
- **重置**：删除 D1 库 → 重新部署 Worker（冷启动自动重建 + 种子）
- **上传件**：base64 持久化于 D1 `uploads` 表（≤5MB，png/jpg/gif/webp，读回带一年 cache-control），导出 D1 即含图片；R2 开通后把 `uploads.py` 的 `STORAGE_BACKEND` 改为 `'r2'` + 绑定 `UPLOADS` 即可无缝切换（接口已双轨）
- 默认管理员 `admin/admin123`（种子）——**上线后立刻改密**；测试账号 `xiaohua/123456`

## 六、已知限制（MVP 范围）

1. **D1 免费层 row-read 50k/天**：重聚合查询已合并降载（admin 统计、趋势 GROUP BY、冷启动单条探测）；若再触顶：升级 D1 Pro 或切 KV 试验轨（`backend-worker/wrangler.kv.experiment.toml`，worker workflow 的 `deploy_kv_experiment` 开关）。
2. **workers.dev 大陆可达性**：绑自有域名解决（见 §三）。
3. 无限流/验证码/敏感词；HTTPS 依赖 CF 边缘。
4. 上传存 D1 单行 ≤1MB base64 列限制内可用，但大图多时 D1 体积增长快——正式起量后切 R2（代码已备好开关）。

## 附录 A · 备选方案：自托管 Flask + Cloudflare Tunnel（已冻结，仅存档参考）

Flask 版（`campus-wall/app/`，PythonAnywhere 对照实现 `zzhx.pythonanywhere.com`）不是上线主线。若需完全自托管：

```bash
git clone https://github.com/qingshanjiluo/campus-wall.git && cd campus-wall/campus-wall
python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt
export JWT_SECRET="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export SECRET_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')" FLASK_ENV=production
gunicorn -w 3 -b 127.0.0.1:8000 "run:app"     # 首次运行自动建表+种子（instance/campushub.db）
```

Tunnel：`cloudflared tunnel create campuswall` → `config.yml` 的 ingress 指向
`http://127.0.0.1:8000` → `tunnel route dns` → `cloudflared service install`。
备份：`crontab` 每日 `cp instance/campushub.db /backup/…`。安全：强 JWT_SECRET、开 Bot Fight Mode、禁 debug。
