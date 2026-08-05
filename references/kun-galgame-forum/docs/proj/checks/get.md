# GET API 字段对齐检查

> 目的: 记录全部 GET 端点 (查询类) 以及 FE↔BE 字段对齐审计状态。
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套文档: [post.md](./post.md) · [put.md](./put.md) · [delete.md](./delete.md)
>
> **本文件目前是 inventory 阶段** — 仅列出全部 GET 端点供后续逐项审计，状态列暂为「待审计」。

## 图例 (状态列取值)

- 无问题 — 已审计，FE/BE 对齐无问题
- 已修复 — 已审计，**发现错位并修复**
- 已跳过 — 已审计，设计上有意保持当前行为 (详见备注)
- 待审计 — 本轮 inventory 阶段默认

## 鉴权图例 (中间件分组，对应「鉴权」列取值)

| 标记 | 中间件 | 含义 |
|---|---|---|
| 公开 | (无) | 完全公开，任何来源可访问 |
| 可选 | `optAuth` | 鉴权可选；带 token 时附加自己的 pending/隐藏内容，匿名时只看公共部分 |
| 登录 | `userAuth` / `authed` | 必须登录 (`Auth` 中间件)，401 if missing |
| 版主 | `galgameAdmin` (`role >= 2`) | 仅 admin/moderator |
| 管理 | `admin` (`role >= 3`) | 仅 admin |

## 后端 handler 类型

- **kungal-native** — 直接调用 kungal Go 业务 handler，可能附加 wiki 调用 + 本地数据装配
- **proxy** — `GalgameWikiHandler.ProxyGet`，透传到 wiki service 的同名/经 `path_mapper` 改写的端点
- **wiki+local** — handler 同时调 wiki API 并合并本地数据 (如 `GetGalgameLinks`, `GetGalgameHistory`)

## 统计

- 全部 GET 端点 (展开 for-loop 后): **98**
  - 静态注册行: 92 (原 95，K-PR 审计删了 `/galgame/check` + `/galgame/:gid/contributors` + `/galgame-resource/:id/recommend`)
  - 实体 revision 循环 (`tag` / `official` / `engine` × 2 routes): 6
- 已审计：**§0 + §1 + §2 + §3 + §4 + §5 完成**（第一组），**§6 + §7 + §8 + §9 + §10 + §11 完成**（第二组），**§12 + §13 + §14 + §15 + §16 + §17 完成**（第三组）
- 已修复 (本轮 K-PR 审计):
  - 第一组 (§0–§5)：
    - `/admin/setting/register` FE 状态反转
    - `/ranking/{galgame,topic,user}` FE 泛型 (UI metadata → API DTO)；`/ranking/user` BE 补 `SortField`；`/ranking/galgame` BE 补 `effective_banner_hash/_url`
    - `/section`, `/category` FE 缺失类型 (`SectionTopicList` 补上，`CategorySection` → `CategorySectionStats`)
    - `/search` reply/comment/user FE 字段守护 (optional + v-if)
    - `/topic`, `/resource` `isPollTopic` BE hardcode false → 批量 `FindTopicIDsWithPoll`
    - `/topic/:tid/poll/topic` FE 泛型 (单 → 数组)
    - `/topic/:tid` 死代码 `'banned'` 字符串检查
    - `/unmoe` FE `result: string | number` → `string`
    - `/galgame/:gid/resource/all` BE `isLiked` hardcode false → 批量 `FindLikedSet`
    - `/galgame/:gid/comment/all` + `/comment/thread/:rootId` BE 缺 `IsLiked` + FE 类型 `number → string`
  - 第二组 (§6–§11)：
    - `/galgame-resource`, `/galgame-rating/all`, `/galgame-rating/:id` 路由组从 `api` 移到 `optAuth` (否则 `optionalUID` 恒 0，`isLiked` 永远 false)
    - `/galgame-official/:name` BE `OfficialDetail` 缺 `original`：wiki PR4/K-PR6 sub-change 加字段后 BE 漏接，FE 编辑模态打开时永远空白
    - `/galgame-tag/:name`, `/galgame-official/:name`, `/galgame-engine/:name` query rename helper 升级为多键 map (新增 `sortField`→`sort_field`、`sortOrder`→`sort_order`)，否则 wiki 静默 fallback 到默认排序
    - `/galgame-rating/:id` BE 缺 `galgameSeries`：FE Review JSON-LD 的 `isPartOf` 永远缺失；改为 wiki detail 拿 `series_id` 后再拉 `/series/:id` 的最小 brief
    - `/galgame-rating/all` FE 类型 `GalgameRatingCard` 缺 `short_summary` (BE 一直在返)
    - `/website`, `/website-category/:name`, `/website-tag/:name` BE 缺 SFW 过滤：FE Container.vue 宣称"默认仅显示 SFW 的网站"但 BE 全返 → 加 `age_limit='all'` scope，与 wiki content_limit 协议对齐
    - `/website/:domain/comment` 子节点孤儿处理：父评论作者被封后，原代码会把子评论提升到顶层（无 TargetUser 的悬挂回复），改为丢弃（与 galgame comment service 对齐）
  - 第三组 (§12–§17)：
    - `/message/admin` `created` 字段从未被赋值（DTO `time.Time` 漏 set）→ 显示零值"约 2024 年前"；改为 string 字段对齐 repo 行 `CreatedAt`，同步修 `/message` 同款漏赋值
    - `/user/:id` FE `useKunFetch<UserInfo \| 'banned'>` 死代码移除（BE 返封禁用户的 `{id, name, status:1}` 对象而非字符串），改用 `status !== 0` 判断；顺带清理 Ref 包装的冗余 query
    - `/user/:id/ratings` BE `UserRatingItem` + repo 加 `ShortSummary` 字段（FE GalgameRatingCard 在 K-PR6 已加 `short_summary`，user 子端点漏接）
    - `/doc/category` + `/doc/tag` FE 类型外层 key 从 `{categories,...}/{tags,...}` 改为 `{items, total}` 对齐 BE 实际 wire（FE consumer 早就读 `.items`）
    - `pages/message/notice.vue`、`components/update/{History,Todo}.vue` 用未声明类型 `MessageList`/`UpdateHistoryList`/`UpdateTodoList`（TS any）→ 在 shared/types 声明
    - `/toolset/:id` `commentPreview` BE `CommentDetailItem` 嵌入原始 model 输出 `user_id/toolset_id/parent_id` snake_case → 改为显式 camelCase 字段（与 ToolsetCommentItem 一致）
    - `/user/:id/galgames` BE `UserGalgameCard` + repo brief 透传 `releaseDate/releaseDateTBA`（wiki U1 后字段漂移，user 子端点漏接）
    - `/toolset/:id/practicality` FE `mine: number` → `number \| null`；`/message/nav/*` FE `lastMessageTime` 加 null + `ChatMessage.receiverId` 收窄为 `number`
    - `/user/status` BE 用户无 state 行时不再 404，lazy 用 0 默认值（新注册用户 Nav 不再丢萌萌点/签到状态）

---

## 0. 认证 / 身份

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /auth/me` | 登录 | kungal-native (proxies OAuth `/auth/me`) | 待审计 | 返回当前登录态用户的 profile 投影；FE 一般用持久化 store 而非每次拉 |

## 1. 首页 / 动态 / 搜索 / Feed

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /home` | 公开 | kungal-native + wiki batch | 待审计 | NSFW SFW filter via `IsSFW(c)` → `GetBatchPublic`；over-fetch 2× |
| `GET /activity` | 公开 | kungal-native + wiki batch | 待审计 | 按 `type` 过滤的单类活动流；isSFW filters galgame name 注入 |
| `GET /activity/timeline` | 公开 | kungal-native + wiki batch | 待审计 | 全类型时间线 |
| `GET /search` | 公开 | kungal-native (Meilisearch via wiki for galgame) | 待审计 | type=galgame 时**强制 `content_limit=all`** (产品决策：搜索可发现 NSFW) |
| `GET /rss/topic` | 公开 | kungal-native | 待审计 | SQL 强制 `is_nsfw=false` |
| `GET /rss/galgame` | 公开 | kungal-native + wiki batch | 待审计 | 强制 `GetBatchPublic(..., true)` SFW |
| `GET /unmoe` | 公开 | kungal-native | 待审计 | 站内违规记录列表 |
| `GET /admin/setting/register` | 公开 | kungal-native | 待审计 | 公开读：当前注册策略 (是否开放、是否需邀请码) |

## 2. 话题 / 回复 / 投票

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /topic` | 可选 | kungal-native | 待审计 | 话题列表；`isNSFW := !utils.IsSFW(c)` (list_repo SQL filter) |
| `GET /topic/:tid` | 可选 | kungal-native | 待审计 | 话题详情；embed `bestAnswer` 给 FE JSON-LD；BE **不**做 SFW gate (由 FE 拦截) |
| `GET /topic/:tid/reply` | 可选 | kungal-native | 待审计 | 回复列表；`GetReplies` 含 `isBestAnswer` flag |
| `GET /topic/:tid/reply/detail` | 可选 | kungal-native | 待审计 | 单回复完整 (含 replyContentHtml 渲染) |
| `GET /topic/:tid/poll/topic` | 可选 | kungal-native | 待审计 | 话题的投票题目 + 选项 |
| `GET /topic/:tid/poll/log` | 可选 | kungal-native | 待审计 | 当前用户的投票记录 |
| `GET /resource` | 公开 | kungal-native | 待审计 | 资源 section (g-seeking/g-other/t-help) 话题；同 `/topic` SFW 规则 |

## 3. Section / Category / 排行

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /section` | 公开 | kungal-native | 待审计 | section 元数据 (g-galgame / t-tech 等) |
| `GET /category` | 公开 | kungal-native | 待审计 | category → section 树 |
| `GET /ranking/galgame` | 公开 | kungal-native + wiki batch | 待审计 | galgame 排行；`isSFW` → `GetBatchPublic` |
| `GET /ranking/topic` | 公开 | kungal-native | 待审计 | topic 排行；SQL `t.is_nsfw=false` when SFW |
| `GET /ranking/user` | 公开 | kungal-native | 待审计 | 用户排行；NSFW 无关 |

## 4. Galgame (核心列表 / 详情)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame` | 公开 | kungal-native + wiki batch | 待审计 | 主列表；SFW filter via `GetBatchPublic(isSFW)`；含 type/language/platform/provider 本地 filter |
| `GET /galgame/:gid` | 可选 | kungal-native + wiki direct | 待审计 | 详情；带 token 时 wiki 可返 owner 自己的 pending；BE **不** gate NSFW |
| `GET /galgame/mine` | 登录 (`userAuth`) | proxy | 待审计 | `/galgame/mine` 透传 wiki；带 decline_reason |
| `GET /galgame/search/wizard` | 登录 (`userAuth`) | kungal-native (proxy + token) | 待审计 | "发布向导"搜索；自动加 `?include_pending=true` |
| ~~`GET /galgame/check`~~ | — | — | 已移除 | VNDB-ID 精确查重已被 name-based search 替代，FE 无调用方；K-PR 审计中已从 router.go 删除注册。 |
| `GET /galgame/:gid/resource/all` | 可选 | kungal-native | 待审计 | 该 galgame 全部资源；FE 详情页用 |
| `GET /galgame/:gid/comment/all` | 可选 | kungal-native | 待审计 | 该 galgame 评论 (含 root + 前 3 reply 预览) |
| `GET /galgame/:gid/comment/thread/:rootId` | 可选 | kungal-native | 待审计 | 单条 root 评论的完整子树 |
| `GET /galgame/:gid/history/all` | 可选 | wiki+local | 待审计 | revision 历史 (kungal-mapped camelCase) |
| `GET /galgame/:gid/pr/all` | 可选 | wiki+local | 待审计 | PR 列表 (kungal-mapped) |
| `GET /galgame/:gid/link/all` | 可选 | wiki+local | 待审计 | 链接列表 (kungal-mapped) |

## 5. Galgame Wiki 透传 (revisions / PR / links / aliases / contributors)

> 全部走 `GalgameWikiHandler.ProxyGet` → `path_mapper` → wiki service。响应 shape **不经 kungal 改写**，原样回传。

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame/:gid/revisions` | 公开 | proxy | 待审计 | 历史版本列表 (wiki 直出) |
| `GET /galgame/:gid/revisions/:rev` | 公开 | proxy | 待审计 | 单 revision 快照 |
| `GET /galgame/:gid/revisions/:rev/diff` | 公开 | proxy | 待审计 | 单 revision diff |
| `GET /galgame/:gid/prs` | 公开 | proxy | 待审计 | PR 列表 (与 `/pr/all` 区别：这条原样透传) |
| `GET /galgame/:gid/prs/:id` | 公开 | proxy | 待审计 | 单 PR 详情 |
| `GET /galgame/:gid/links` | 公开 | proxy | 待审计 | 链接 (原样透传) |
| `GET /galgame/:gid/aliases` | 公开 | proxy | 待审计 | 别名列表 |
| ~~`GET /galgame/:gid/contributors`~~ | — | — | 已移除 | FE 现从 detail 的 `contributor[]` 内嵌字段读取，独立列表无人调用；K-PR 审计中已从 router.go 删除注册。 |

## 6. Galgame 分类轴 (tag / official / engine / series)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame-tag` | 公开 | kungal-native (forward to wiki) | 无问题 | tag list；`isSFW` 时过滤 `category=sexual` |
| `GET /galgame-tag/search` | 公开 | kungal-native (Meilisearch via wiki) | 无问题 | tag 搜索；同上 sexual filter |
| `GET /galgame-tag/multi` | 公开 | kungal-native + wiki batch | 无问题 | 多 tag AND 搜 galgame |
| `GET /galgame-tag/:name` | 公开 | kungal-native + wiki batch | 已修复 | tag detail；K-PR6 修复 `sortField/sortOrder` → snake_case 透传 (helper 多键 map 化) |
| `GET /galgame-official` | 公开 | kungal-native (forward to wiki) | 无问题 | official list；元数据无 NSFW filter |
| `GET /galgame-official/search` | 公开 | kungal-native (Meilisearch via wiki) | 无问题 | official 搜索 |
| `GET /galgame-official/:name` | 公开 | kungal-native + wiki batch | 已修复 | official detail；K-PR6 补 `original` 字段 (wiki PR4 起新增，FE 编辑模态用) + sortField/Order 透传 |
| `GET /galgame-engine` | 公开 | kungal-native (forward to wiki) | 无问题 | engine list；元数据 |
| `GET /galgame-engine/:name` | 公开 | kungal-native + wiki batch | 已修复 | engine detail；K-PR6 修复 sortField/sortOrder 透传 (与 tag/official 同一 helper) |
| `GET /galgame-series` | 公开 | kungal-native (forward to wiki) | 无问题 | series list；每条预载前 5 个 galgame sample |
| `GET /galgame-series/search` | 公开 | proxy | 无问题 | series 搜索 (Meilisearch via wiki，原样透传) |
| `GET /galgame-series/:id` | 公开 | kungal-native + wiki batch | 无问题 | series detail；**已删除 revision panel** (K-PR series-revision design)；含关联 galgame 列表 |

## 7. Galgame 分类轴 — Revisions (proxy 透传，K-PR5)

> 由 `router.go` 的 for-loop 动态注册：`for ent ∈ {galgame-tag, galgame-official, galgame-engine}`。
> `galgame-series` **已被故意排除** — series 成员变更落在 galgame 侧 revision，per-series 历史为空/误导，前端面板已下线，路由也未注册。

| 路径 (展开) | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame-tag/:id/revisions` | 公开 | proxy | 无问题 | tag 修订列表 |
| `GET /galgame-tag/:id/revisions/:rev` | 公开 | proxy | 无问题 | tag 单修订快照 |
| `GET /galgame-official/:id/revisions` | 公开 | proxy | 无问题 | official 修订列表 |
| `GET /galgame-official/:id/revisions/:rev` | 公开 | proxy | 无问题 | official 单修订快照 |
| `GET /galgame-engine/:id/revisions` | 公开 | proxy | 无问题 | engine 修订列表 |
| `GET /galgame-engine/:id/revisions/:rev` | 公开 | proxy | 无问题 | engine 单修订快照 |

## 8. Galgame 资源 (galgame-resource)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame-resource` | 可选 | kungal-native + wiki batch | 已修复 | 资源列表；K-PR6 移到 `optAuth` 组 (此前 `api` 组导致 `isLiked` 永远 false)；`isSFW` → `GetBatchPublic`；`total` over-reports in SFW |
| `GET /galgame-resource/:id` | 可选 | kungal-native + wiki | 无问题 | 资源详情；BE **不** gate NSFW (由 FE 处理) |
| `GET /galgame-resource/:id/detail` | 可选 | kungal-native + wiki | 无问题 | 含完整下载链接 (会触发 download view 增量) |
| ~~`GET /galgame-resource/:id/recommend`~~ | — | — | 已移除 | FE 已改用页面端点内嵌的 `recommendations` 字段，独立端点无调用方；K-PR6 审计中已从 router.go + handler + service 删除注册。 |

## 9. Galgame 评分 (galgame-rating)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /galgame-rating/all` | 可选 | kungal-native + wiki batch | 已修复 | 评分列表；K-PR6 移到 `optAuth` 组；FE 类型补 `short_summary`；BE 一直在返但 FE 类型缺 |
| `GET /galgame-rating/:id` | 可选 | kungal-native + wiki | 已修复 | 单评分详情；K-PR6 移到 `optAuth` 组 + BE 补 `galgameSeries` brief (FE Review JSON-LD `isPartOf` 用) |

## 10. Galgame Submission Messages (wiki 消息流)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| ~~`GET /galgame/messages/mine`~~ | — | — | 已退役 | wiki 通知页 wave 169 撤除(claim_event feed + 本地消息取代) |
| ~~`GET /galgame/messages/read-state`~~ | — | — | 已退役 | wave 169 撤除;wiki_message_read_state 表留作历史 |
| ~~`GET /admin/galgame/messages`~~ | — | — | 已退役 | wave 169 撤除(上游 404;claim.review 面取代) |

## 11. 网站 (website)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /website` | 可选 | kungal-native | 已修复 | 网站列表；K-PR6 加 SFW filter (`age_limit='all'` scope) — 原 FE Container.vue 宣称"默认仅显示 SFW"但 BE 不过滤 |
| `GET /website/:domain` | 可选 | kungal-native | 无问题 | 网站详情；包含 comment 列表 |
| `GET /website/:domain/comment` | 可选 | kungal-native | 已修复 | 该网站评论列表；K-PR6 修复孤儿子评论 (父被封时丢弃 reply 而非提升到顶层) |
| `GET /website-category/:name` | 公开 | kungal-native | 已修复 | 网站分类下的所有网站；K-PR6 加 SFW filter |
| `GET /website-tag` | 公开 | kungal-native | 无问题 | 网站标签列表 |
| `GET /website-tag/:name` | 公开 | kungal-native | 已修复 | 单标签详情；K-PR6 加 SFW filter |

## 12. 文档 (doc)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /doc/article` | 公开 | kungal-native | 无问题 | 文档列表 (含分类/标签 facets) |
| `GET /doc/article/:slug` | 公开 | kungal-native | 无问题 | 文档详情；含 `tagIds[]` (K-PR 修复，便于 rewrite 重填) |
| `GET /doc/category` | 公开 | kungal-native | 已修复 | 文档分类列表；K-PR6 FE 外层 key `{categories,...}` → `{items, total}` 对齐 BE wire |
| `GET /doc/tag` | 公开 | kungal-native | 已修复 | 文档标签列表；K-PR6 同上 |

## 13. 工具集 (toolset)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /toolset` | 可选 | kungal-native | 无问题 | 工具列表 |
| `GET /toolset/:id` | 可选 | kungal-native | 已修复 | 工具详情；K-PR6 `CommentDetailItem` 从嵌入 model (snake_case) 改为显式 camelCase 字段，对齐 FE ToolsetComment 类型 |
| `GET /toolset/:id/practicality` | 可选 | kungal-native | 已修复 | 实用度评分分布；K-PR6 FE `mine: number \| null` 收紧（BE 用 `*int` 可空） |
| `GET /toolset/:id/comment/all` | 可选 | kungal-native | 无问题 | 工具评论列表 |
| `GET /toolset/:id/resource/detail` | 公开 | kungal-native | 无问题 | 工具关联资源详情 |

## 14. 用户主页 (user/:id/*)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /user/status` | 登录 (`userAuth`) | kungal-native | 已修复 | 当前用户的萌萌点/签到/未读数；K-PR6 用户无 state 行时不再 404，lazy 0 默认值（新用户 Nav 不再丢字段） |
| `GET /user/:id` | 公开 | kungal-native | 已修复 | 用户 profile；K-PR6 FE 移除 `'banned'` sentinel 死代码，改用 `status !== 0` 判断 |
| `GET /user/:id/floating` | 公开 | kungal-native | 无问题 | 浮动卡片 (hover user header 显示) |
| `GET /user/:id/galgames` | 公开 | kungal-native + wiki batch | 已修复 | 用户上传/参与的 galgame；K-PR6 BE `UserGalgameCard` 补 `releaseDate/releaseDateTBA`（wiki U1 字段漂移漏接） |
| `GET /user/:id/galgame-comments` | 公开 | kungal-native + wiki batch | 无问题 | 用户的 galgame 评论；按 galgame `contentLimit` 过滤 |
| `GET /user/:id/topics` | 公开 | kungal-native | 无问题 | 用户话题；SQL `topic.is_nsfw=false` when SFW |
| `GET /user/:id/replies` | 公开 | kungal-native | 无问题 | 用户回复；JOIN topic 过滤 NSFW topic 下的回复 |
| `GET /user/:id/comments` | 公开 | kungal-native | 无问题 | 用户评论；JOIN topic 过滤 |
| `GET /user/:id/resources` | 公开 | kungal-native + wiki batch | 无问题 | 用户上传的 galgame 资源 |
| `GET /user/:id/ratings` | 公开 | kungal-native + wiki batch | 已修复 | 用户的 galgame 评分；K-PR6 BE 补 `short_summary`（与 GalgameRatingCard 对齐） |

## 15. 消息中心 / 聊天

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /message` | 登录 (`authed`) | kungal-native | 已修复 | 个人通知列表；K-PR6 BE `MessageResponse.Created` 漏赋值修复 + DTO `time.Time → string` 对齐 repo row；FE 补声明 `MessageList` 类型 |
| `GET /message/admin` | 登录 (`authed`) | kungal-native | 已修复 | 系统广播；K-PR6 BE `SystemMessageResponse.Created` 漏赋值修复（之前序列化为零值导致 FE 显示"约 2024 年前"） |
| `GET /message/nav/system` | 登录 (`authed`) | kungal-native | 已修复 | nav-bar 概览；K-PR6 FE `lastMessageTime` 加 null 类型，避免 `formatTimeDifference("")` 渲染 NaN |
| `GET /message/nav/contact` | 登录 (`authed`) | kungal-native | 已修复 | 私信 nav；同上 lastMessageTime + ChatMessage.receiverId 收窄 |
| `GET /message/chat/history` | 登录 (`authed`) | kungal-native | 无问题 | 单对话历史消息 |

## 16. 排行 / 更新日志

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /update/history` | 公开 | kungal-native | 已修复 | GitHub commits feed 镜像；K-PR6 FE 补声明 `UpdateHistoryList` 类型（之前 TS any） |
| `GET /update/todo` | 公开 | kungal-native | 已修复 | 待办列表；K-PR6 FE 补声明 `UpdateTodoList` 类型 |

> 注：排行 (`/ranking/*`) 已在 §3 列出。

## 17. 管理后台 (admin)

| 路径 | 鉴权 | Handler 类型 | 状态 | 备注 |
|---|---|---|---|---|
| `GET /admin/overview/all` | 管理 (`admin`, role >= 3) | kungal-native | 无问题 | 后台数据总览 (user/topic/galgame/comment 增长) |
| `GET /admin/overview/stats` | 管理 (`admin`, role >= 3) | kungal-native | 无问题 | 后台统计数字快照 |
| ~~`GET /admin/galgame/messages`~~ | — | — | 已退役 | (重复列出,与 §10 一致)wave 169 撤除 |
| `GET /admin/setting/register` | 公开 | kungal-native | 无问题 | (重复列出，与 §1 一致) 公开读注册策略 |

---

## 审计维度提示 (用于后续审核)

按 post.md / put.md 同款检查项逐条核对：

1. **路径与方法**：FE `useKunFetch` / `kunFetch` 中 URL / method 与 router.go 一致
2. **响应 shape**：FE 类型 `T` 与 BE DTO 字段名 / 嵌套结构一致 (camelCase vs snake_case)
3. **NSFW 过滤**：是否根据 `IsSFW(c)` 走 `GetBatchPublic`／`content_limit=sfw`
4. **认证语义**：optAuth / authed / admin 是否符合调用方期望 (匿名爬虫能看到吗？)
5. **错误码**：BE `ErrNotFound` (404 + code=233) 时 FE 是否优雅降级为 `KunNull` 而非崩
6. **分页 contract**：`{items, total}` vs `[]` 形态；`total` 是否被 wiki SFW filter 影响 (8 个已知端点)
7. **字段漂移**：post-K-PR refactor 后旧字段名残留 (`released`/`banner_image_hash`/`status` on system_message 等) — 见 [seo-audit.md §10.4](../seo-audit.md#104-字段名漂移每次后端重构都要核对此节)
8. **proxy vs kungal-native**：proxy 端点的字段 shape 取决于 wiki 当前版本；wiki 文档 (`docs/galgame_wiki/*.md`) 改了字段后 FE 是否同步

---

## 后续审计建议顺序

1. **公开高流量入口**：`/home`, `/galgame`, `/topic`, `/galgame-resource`, `/galgame-rating/all` — 影响 SEO + 热路径性能
2. **详情页 + JSON-LD 依赖**：`/galgame/:gid`, `/topic/:tid`, `/galgame-rating/:id`, `/toolset/:id`, `/website/:domain` — 字段缺失会直接打破 JSON-LD
3. **用户主页系列**：`/user/:id/*` — 字段名一致性 + NSFW JOIN 正确性
4. **proxy 透传组**：`/galgame/:gid/{revisions,prs,links,aliases,contributors}` 等 — wiki 字段 shape 变化的主要受害区
5. **admin / message** — 低流量但容易因 schema 演进 silent break

---

## 路径来源行号速查 (开发者参考)

`apps/api/internal/app/router.go`：

| 起始行 | 结束行 | 内容 |
|---|---|---|
| 22 | 28 | 首页 / 用户基础 |
| 36 | 54 | 用户主页系列 |
| 57 | 59 | 排行 |
| 62 | 63 | section / category |
| 66 | 69 | 文档 |
| 72 | 74 | 网站分类/标签 |
| 77 | 78 | 更新日志 |
| 81 | 81 | admin 公开读注册策略 |
| 84 | 85 | 活动 |
| 88 | 89 | 评分 |
| 92 | 92 | 资源 section topics |
| 95 | 95 | 搜索 |
| 98 | 99 | RSS |
| 102 | 102 | unmoe |
| 105 | 105 | 工具集 resource 详情 |
| 108 | 129 | galgame core (含 proxy revisions/prs/links/aliases/contributors) |
| 133 | 145 | galgame 分类轴 + galgame-resource list |
| 152 | 154 | galgame-resource 详情 (optAuth) |
| 157 | 162 | topic + reply + poll (optAuth) |
| 165 | 171 | galgame 子端点 (optAuth) |
| 174 | 176 | website (optAuth) |
| 179 | 182 | toolset (optAuth) |
| 189 | 189 | `/auth/me` (authed) |
| 221 | 228 | message + chat (authed) |
| 262 | 263 | wiki message mine + read-state (authed) |
| 349 | 351 | tag/official/engine revision proxy (for-loop) |
| 380 | 381 | admin overview (role>=3) |
| 393 | 393 | admin galgame messages (role>=2) |
