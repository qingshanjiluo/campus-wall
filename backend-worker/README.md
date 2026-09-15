# backend-worker —— 校园墙 Cloudflare Python Worker

`entry.py`（WorkerEntrypoint，按请求设置 `context.env` 并调用 `bootstrap.ensure_ready()`）
→ `router` → 各 `routes_*.py`；数据层 `models*.py` 只依赖 `db.py` 的
`query / execute / execute_script / IntegrityError` 契约。

测试：`python tools/run_native_tests.py`（默认跑 `d1` + `kv` 两个模式，全部本地内存，零网络）。

## Storage backend: KV (launch) / D1 (upgrade path)

`src/db.py` 每次调用按 env 绑定选择后端，**业务 SQL 与路由层完全无感**：

1. `env.DB`（D1）存在 → D1 模式（生产当前状态；代码路径与最初版本一致）。
2. 否则 `env.KV` 存在 → **KV 模式**：`src/_kv.py` 用 Workers KV 做载体，
   每个分片存一个完整的嵌入式 SQLite 库。
3. 都没有 → 走 D1 分支报错（旧行为）。

> 背景：D1 免费层 50k 行读/日曾被两个后台请求打满，且部署 token 缺 D1:Edit
> （REST 建库/改库失败；wrangler 部署链路自身可用）。KV 无行读计费，
> 因此保留 KV 后端作为可随时切换的对照方案（当前以实验 worker 形态部署）。

### KV 模式工作方式

- 分片键 `db-shard:<name>`，值为 JSON `{"gen": int, "db": base64(整库 SQL 转储)}`；
  **每个分片都含全部 27 张表的 schema**，只有本分片拥有的表带数据 → 分片自洽。
- 分片归属（`_kv.TABLE_SHARDS`）：

  | 分片 | 拥有的表 |
  |---|---|
  | content | posts, comments, likes |
  | users | users, checkins, notifications, follows, station_members, coin_transactions, shop_orders, favorites, user_settings, trade_posts, romance_profiles, romance_links, romance_tasks |
  | stations | stations |
  | uploads | uploads |
  | global | identity_groups, site_announcements, shop_items, post_versions, reports, gossip, gossip_comments, kanban_messages, admin_log |

- 语句路由：先剥离字符串字面量/注释，再按词边界扫描引用的表（`station_id` 不会
  误匹配 `stations`）。单分片语句直接在拥有者分片执行；跨分片语句以第一个相关
  分片为底，把其它分片"被引用且其拥有"的表整表导入底库（scratch merge，含
  sqlite_sequence 单调对齐），写操作再把各表回写拥有者分片并持久化（gen+1）。
- 请求级缓存：`entry.py` 每次 fetch 都 `await bootstrap.ensure_ready()` →
  `db.begin_request()` 轮换 epoch → 本请求首次使用某分片时才从 KV 下载
  （raw 未变则复用已解析连接），写操作即时 write-through 到 KV。
- 引擎（`src/_engine.py`）：优先 stdlib `sqlite3`——Cloudflare 官方 stdlib 文档
  确认 Python Workers 携带完整标准库（sqlite3 不在排除清单）。任务最初设想的
  `puresql` 包在 PyPI 上不存在（2026-08 验证）；如需纯 Python 兜底，将任何
  sqlite3 兼容模块放到 `vendor/puresql/` 即自动接管（或用 `CW_SQL_ENGINE=puresql` 强制）。

### 切换方法

- 生产（D1）：`wrangler.toml` 当前只绑 `[[d1_databases]]` → 即 D1 模式。
- KV 实验：`wrangler.kv.experiment.toml`（worker 名 `campus-wall-api-kv`，只绑
  `[[kv_namespaces]]`，无 DB）→ `wrangler deploy -c wrangler.kv.experiment.toml`。
  CI：workflow_dispatch 勾选 `deploy_kv_experiment` 即在建好
  `campuswall-kv-experiment` 命名空间后自动部署（主 D1 部署照常先行）。
- 两者同时绑定时 D1 优先。KV 首请求自动建 5 个分片并播种。

### KV → D1 数据迁移（升级路径）

```
python tools/export_kv_to_sql.py --store-json snapshot.json -o migrations/9001_data_from_kv.sql
npx wrangler d1 execute campus-wall-db --remote --file=migrations/9001_data_from_kv.sql
```

`snapshot.json` = `{"db-shard:users": "<payload 字符串>", ...}`（`wrangler kv key get` 逐键导出，
或测试里 `KVShim.snapshot()`）。脚本为 27 表的 DELETE+INSERT + AUTOINCREMENT 序列对齐，可重复执行。

### 已知限制（KV 模式）

- KV 单值上限 5 MiB：uploads 分片以 base64 SQL 文本存图，超过约 2–3 MB 的图片
  （或大量图片累积）会让 `put` 失败 → save_upload 报"存储失败"。生产用 D1 无此问题。
- 无跨分片原子性/外键：KV 最终一致；两分片持久化之间宕机可能撕裂关联计数
  （如 posts.likes_count vs likes 行）。gen 仅为信息量，无 CAS 冲突检测；
  多 isolate 并发写同一分片为 last-write-wins。冷 isolate 播种竞态可能双写（D1 同理）。
- scratch merge 会把被引用表的副本随基础分片一并持久化 → 分片 blob 有一定膨胀
  （读路径总是从拥有者分片重新导入，副本永不产生脏读）。
- KV 模式下 schema 只增不改：新增迁移需让 `bootstrap.SCHEMA_SQL` 更新后清一次
  `db-shard:*`（或临时清空分片缓存），DDL 广播路径才生效。

### 读预算优化（两模式共用，响应形状不变）

- `admin/stats`：28 条查询 → 1 条 UNION ALL 总量 + 3 条 GROUP BY 趋势，另加
  每 isolate 60s TTL 记忆（`models_ext.reset_stats_cache()` 供测试）。
- `bootstrap.ensure_ready`：冷 isolate 从"28 条 CREATE 重放 + COUNT" → **1 条**
  `SELECT 1 FROM users LIMIT 1` 探针，探到行即跳过全部重放；热请求 0 条。
- 子站详情：membership 与 role 两次同行扫描 → 1 条（`get_station_membership`）。
- 子站列表：每站 2 条成员查询 → 1 条批量 `IN`（`get_station_memberships`）。
- 子站统计：days+1 条 COUNT → 1 条 GROUP BY。
