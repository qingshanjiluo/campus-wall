# 老图迁移到 image_service：kungal 侧清单与迁移（2026-06-15）

> 本仓自有工程笔记（**非** infra 镜像）。背景：OAuth 侧正在跑**用户头像**迁移（老头像 → image_service）。
> 本文盘清 **kungal 自己的数据里还有哪些老图**需要迁移，并给出统计 SQL 与迁移命令。

## 0. 结论：头像在 kungal 这边零工作

`users.avatar` 早在 **migration 007** 就删了——kungal **不持久化头像**，每次从 OAuth 实时取。
OAuth 把头像迁完，kungal 自动拿到新 URL。**头像不需要、也无法在 kungal 侧迁。**

kungal 真正的「老图」是另一类：**用户内容里嵌的老图床图片**。

## 1. 老 vs 新 的判别

| | 形态 | 例子 |
|---|---|---|
| **老（需迁移）** | 路径式、非内容寻址，老图床主机 | `https://image.kungal.com/topic/user_38351/茅羽耶-1761790962059.webp` |
| **新（image_service）** | 内容寻址，两级分片 | `https://image.kungal.iloveren.link/{aa}/{bb}/{hash}.webp` |

判别子：内容里出现 **`image.kungal.com`** = 老图（老图床早已停写，新上传一律走 `image.kungal.iloveren.link`）。

## 2. 清单（**live prod kungalgame 库实测**，infra 2026-06-15 确认；与缓存快照一致——老图床早停写、数量稳定）

**需要迁移的：**

| 来源（content 列） | 含老图行数 | 已在 image_service |
|---|---|---|
| `topic.content` | 1,298 | 26 |
| `topic_reply.content` | 488 | 32 |
| `chat_message.content`（裸 URL，部分已 404） | 29 | 12 |
| `galgame_comment.content` | 1 | 8 |
| **合计** | **1,816 行** | — |

**实际工作量 = 5,440 个去重老图文件**（同图常被多处引用，所以文件数 < 引用数）。
可行性 infra 已确认：老图床走 Cloudflare（非本集群入口，无 hairpin 问题），dokploy-network 内随机抽 60 张 100% 返回 200；kungal client 配额 10,000/天·10GB/天，~5,440 张(~270MB)一轮跑完绰绰有余。

**确认不用迁的：**
- **头像**：见 §0。`chat_room.avatar` 1148 行全空；`doc_category.icon` 全空。
- `doc_article.banner`（29 行）：都是仓库静态资源 `/content/.../banner.avif`，非上传图。
- `galgame_website.icon`：基本是**外站 favicon**（bgm.tv / ggbases…），非我方图。
- galgame 封面/截图：早已 image_service（`image_hash`）。
- `sticker.kungal.com`（topic 156 + reply 409）：表情贴独立服务，语义上不算老图床上传——**是否纳入由产品定**。
- 第三方外链（raw.githubusercontent / postimg / ymgal…）：非我方图，迁不了（热链，可能失效，另一回事）。

> ⚠️ 部分 `image.kungal.com` 图**已经 404**（chat 样本里直接带「404错误」文字）——老图床在退场，**迁移要趁早**，且要容忍取不到原图。

## 3. 在 live prod 重跑的统计 SQL

```sql
-- 每表含老图的行数 + 已在新图床的行数
SELECT 'topic'           AS tbl, count(*) FILTER (WHERE content LIKE '%image.kungal.com%')          AS rows_old,
                                  count(*) FILTER (WHERE content LIKE '%image.kungal.iloveren.link%') AS rows_new FROM topic
UNION ALL SELECT 'topic_reply',     count(*) FILTER (WHERE content LIKE '%image.kungal.com%'),          count(*) FILTER (WHERE content LIKE '%image.kungal.iloveren.link%') FROM topic_reply
UNION ALL SELECT 'chat_message',     count(*) FILTER (WHERE content LIKE '%image.kungal.com%'),          count(*) FILTER (WHERE content LIKE '%image.kungal.iloveren.link%') FROM chat_message
UNION ALL SELECT 'galgame_comment',  count(*) FILTER (WHERE content LIKE '%image.kungal.com%'),          count(*) FILTER (WHERE content LIKE '%image.kungal.iloveren.link%') FROM galgame_comment;

-- 去重老图文件数（迁移工作量 = 这个数；同图多引用只传一次）
SELECT count(DISTINCT url) AS distinct_old_files FROM (
  SELECT (regexp_matches(content, 'https://image\.kungal\.com/[^\s)"''>\]]+', 'g'))[1] AS url FROM topic
  UNION ALL SELECT (regexp_matches(content, 'https://image\.kungal\.com/[^\s)"''>\]]+', 'g'))[1] FROM topic_reply
  UNION ALL SELECT (regexp_matches(content, 'https://image\.kungal\.com/[^\s)"''>\]]+', 'g'))[1] FROM chat_message
  UNION ALL SELECT (regexp_matches(content, 'https://image\.kungal\.com/[^\s)"''>\]]+', 'g'))[1] FROM galgame_comment
) s;
```

## 4. 迁移命令：`cmd/backfill-content-images`

按现有 `backfill-friend-link-banners` 同一套路实现（config → image client → DB → dry-run → 抓原图 → 传 image_service → 改写 DB）。

逐个去重老 URL：HTTP 抓 `image.kungal.com` 原图 → 用 `topic` preset 传 image_service → 缓存新 URL → 把每行 content 里的旧 URL 全替换成新 URL。**跨表共享缓存**（同图只传一次）。抓不到 / 404 的 URL **记日志并跳过**（该行保留旧 URL，重跑会再试）。**只写 `content` 列，绝不动 `updated`**（否则会把老帖顶到「最近更新」）。顺序执行，~5,440 张约 **1 小时**。

**审计轨迹（就是 job 日志）**：每张 `已重托管 old→new`、每个取不到的 `失败/404` URL、每行 `已改写 table id 替换处数`，跑完一条总结 + 失败 URL 列表。导出运行日志即留底（可据此核对 / 回滚 / 跟进死图）。

```bash
docker compose -f docker-compose.prod.yml --profile jobs run --rm tools \
  backfill-content-images                     # dry-run（默认）：纯统计去重老图文件数 + 各表行数（不联网、不写库）
  backfill-content-images -dry-run=false -limit=20   # 先拿 20 行/表 试水（真抓真传真改）
  backfill-content-images -dry-run=false      # 全量：重托管 + 改写（~1h）
  backfill-content-images -base=http://<内网镜像>     # 老图床若从 job 容器访问不到，改抓取来源（infra 实测可达，一般不用）
```

**安全性**：`-dry-run` 默认 **TRUE**——只 scan + 报工作量，**不联网、不写库**。幂等——改写后 content 不再匹配 `%image.kungal.com%`，重跑自动跳过（只重试上次的死图）。只改 `content`，不顶帖。

**分级流程**（同头像迁移）：
1. 部署带本命令的 forum **tools 镜像**（`fb052342` 已提交，但还没进已部署镜像）。
2. **dry-run** 确认去重文件数 ≈ 5,440。
3. **`-limit=20` 小批试水**（真改 20 行/表），抽查改写结果与新图能打开。
4. **全量** `-dry-run=false`，导出日志留底。
5. 跑完核对：死图列表人工跟进；`SELECT count(*) … LIKE '%image.kungal.com%'` 应只剩死图行。

**注意**：① 老图床可达性 infra 已实测 OK（Cloudflare，非集群入口，无 hairpin）；若环境变化抓不到，用 `-base` 指内网镜像。② **趁早**——已有 404，老图床在退场。③ 表情贴 `sticker.kungal.com` **不在范围**（独立服务，是否纳入另议）。

## 5. 执行记录（2026-06-15 完成）

按上面分级流程在 prod 跑完（`docker compose --profile jobs run --rm tools backfill-content-images`）：

- **dry-run**：去重老图 5,440 张，与 infra live prod 实测一致。
- **`-limit=20` 试水**：重托管 96 张 / 改写 61 行 / 0 失败；抽查新 URL 200、剩余数恰好减 20。
- **全量**（13:18→15:18 UTC，约 2 小时，顺序）：**重托管 5,342 张、改写 1,747 行（5,370 处）、0 报错**。
- **死图 3 张**（404，已保留旧 URL，本就已坏，非回归）：
  - `image.kungal.com/kun`（残缺 URL，非真实图）
  - `image.kungal.com/topic/user_10050/Clobber1238-1722604366830.webp`（老图床上已删）
  - `image.kungal.com/topic/user_2/鲲-1780457802051.webp1`（文件名带错字 `1`）
- **收尾核对**：剩余含 `image.kungal.com` 的行 = topic 1 / reply 1 / chat 6 = **8 行**，与「跳过 8 行」吻合，且全部只引用上述 3 张死图（`NOT /kun AND NOT Clobber1238` 过滤后为 0）。新图 200 可访问。
- 表情贴 `sticker.kungal.com` 未动（按约定）。审计全量日志见 prod `/tmp/bcimg-full.log`。
- **死图清理（2026-06-15）**：3 张死图中,reply 3354 的 markdown 图 + 6 条 chat 的裸 URL 已删除（共 7 行,只删图不删消息/行）；topic 1701 的 `image.kungal.com` 是帖子里的**配置示例文本**（`IMAGE_BED_ENDPOINT = "…"`），非图片,**保留**。至此 content 内已无任何坏图引用。表情贴 `sticker.kungal.com` 仍在用,不动。

## 6. 收敛为域名无关引用 `/image/<hash>`（2026-06-16 完成）

§5 的 backfill 把图迁进了 image_service,但正文写的是**绝对 URL**（`https://image.kungal.iloveren.link/<aa>/<bb>/<hash>.webp`）——
等于把图床域名重新焊进了每条内容,**没解决"换域名"**。infra 在 `99ed2215` 同步的 image_service 契约要求内容存**域名无关 token `/image/<hash>`**,
渲染期解析、后端 302 兜底。本节记录把 forum 对齐到该契约的闭环。

**改动（commit `60c87b71`,forum 侧闭环）：**
- `markdown.renderImage`：`/image/<hash>` → `imageclient.MainURL(cdnBase,…)`,在 sanitize **之前**解析（覆盖 topic/reply/chat/comment 全部服务端渲染；
  chat 走 inline sanitizer 的 host 白名单,解析后落在 `image.kungal.iloveren.link` 上才过得了）。启动注入 `cfg.GalgameWiki.ImageCDNBase`。
- 上传 `UploadTopicImage` / `UploadMessageImage` 返回 `/image/<hash>`,**新内容**天生域名无关（编辑器原样插入,前端零改动）。
- web 全局 middleware `GET /image/<hash>` → 302 CDN,兜底编辑器预览 / RSS / 原文消费方。**用 middleware 而非 server route**:
  `public/image/`（kohaku.webp）目录冲突会让 Nitro 静默跳过同名 route。
- `cmd/rewrite-content-image-refs`（新,幂等,只动 content）：把已写入的绝对 URL 收敛为 token。`backfill-content-images` 也改为直接写 token。

**执行（prod,2026-06-16,顺序铁律 = 先部署 resolver+302,再改写）：**
- **部署确认**：web 302 实测 `/image/<hash>`→302→CDN ✓；kungal-api 镜像构建于 08:44 UTC（commit 08:38 UTC 之后,CI 同源）✓；
  topic 20 单行先改 token,API 实测 `contentMarkdown`=token、`contentHtml`=解析后的绝对 URL（block 路径端到端验证）✓。
- **改写**：`rewrite-content-image-refs -dry-run=false` → **改写 1884 行（5608 处）、0 报错**（topic 1321 / reply 519 / chat 35 / comment 9；topic 20 已先行,合计 1885）。
- **端态核对**：4 表残留绝对 `image.kungal.iloveren.link` = **0**；token 行 = topic 1322 / reply 519 / chat 35 / comment 9 = **1885**。
  block（topic 20）+ inline（topic 97 的 reply,= chat 同 `RenderInline` 路径）均实测渲染为 CDN URL、无 token 泄漏、图未被 sanitizer 剥。
- **老图床确认**：4 表里真实 `image.kungal.com` 图片 = **0**（`![](…)`/`<img>` 语法包裹的为 0 行）；仅剩 topic 1701 的配置示例文本,保留。

至此内容里**不含任何硬编码图床域名**。换 CDN/域名 = 改三处配置（Go `KUN_IMAGE_PUBLIC_BASE_URL` + web `NUXT_IMAGE_CDN_BASE` + chat 白名单 `KUNGAL_MESSAGE_IMAGE_HOSTS`),
**零内容重写**。旧图床 `image.kungal.com` 可由 infra 安排下线。moyu（patch 仓）是否同样写了绝对 URL 需 patch 侧自查,不在本仓范围。

## 7. 补全:第一次漏扫的 4 个含旧图列（2026-06-16）

§5 的迁移只覆盖 topic/reply/chat/comment 四个 content 列。**全库 string 列扫描**（115 列 / 38 表,
`information_schema` + `LIKE '%image.kungal.com%'`）发现 `image.kungal.com` 真实旧图还残留在 **4 个被漏的列**:
`message.content`（通知/系统广播快照,2189 真图）、`topic_reply_target.content`（引用回复快照,268）、
`galgame_toolset.description`（13）、`doc_article.content_markdown`(8)。

**改动:**
- `backfill-content-images` targets 加这 4 列;`cron.RunReferencePing` 扫描同步加（→ 共 8 列,否则只在这些列出现的图仍会被 GC）。
- **migration 027**:`message.content` `varchar(233)` → `text`。旧路径式 URL 短（~58 字符），token `/image/<64hex>` 71 字符,
  在接近 233 上限的通知上改写会溢出（SQLSTATE 22001,约一半 message 行失败）。`text` 与其余 content 列对齐;新通知仍由 notifier 的 233 截断约束。
  （prod 已先手动 `ALTER`,027 入库后 migrate 为幂等 no-op + 记录。）

**执行（prod）:** 重跑 backfill → **重托管 884 张、改写 2099 行、跳过 245、失败/404 80**。失败/跳过基本都是 **被 233 截断的残缺 URL**
（如 `…鲲-1707721662801.we`，本就加载不了）+ 3 张已知死图。`image-refping` 复跑 = **distinct 5917 / updated 5917、0 NotFound**
（较上轮 +398,且 topic 3618 那 3 张软删图已被 infra 复活 → 0 NotFound）。

**端态:** 4 个补迁列里**真实可加载的 `image.kungal.com` 图 = 0**;残留全是死图 / 截断残缺 URL（本就坏）+ topic 1701 配置文本。
全库再无任何**可加载**的旧图床图片。

**migration 027 入库（2026-06-16）:** CI 构建 migrate 镜像后 `--profile jobs run --rm migrate` 跑通——027 对已是 text 的列为 no-op、记录入 `_migrations`,prod schema 与迁移表一致。

**残留清理（2026-06-16）:** `message.content` 里 387 条坏图引用（1 张死图 Clobber 全 markdown + 386 条 233 截断的残缺 URL）用 `regexp_replace` 删除
（删图不删消息正文,dry-run 实测删后该列 image.kungal.com 归零、正文完好）→ `UPDATE 387`。**全库最终 string 扫描:`image.kungal.com` 仅剩 2 处 —
topic 1701 与 doc_article 28,均为讲 `IMAGE_BED_ENDPOINT` 配置的示例文本（非图）,保留。** 旧图床自此无任何可加载图片被引用,可安全下线。
