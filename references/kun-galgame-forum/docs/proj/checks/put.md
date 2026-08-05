# PUT API 字段对齐检查

> 目的:记录全部 PUT 端点以及 FE↔BE 字段对齐审计状态。
>
> 路由源:`apps/api/internal/app/router.go`
>
> 配套文档:[post.md](./post.md) / [delete.md](./delete.md)

## 图例 (状态列取值)

- 无问题 — 已审计,FE/BE 对齐无问题
- 已修复 — 已审计,**发现错位并修复**
- 已跳过 — 已审计,设计上有意保持当前行为(详见备注)

## 统计

- 全部 PUT 端点: **52**
- 已审计: **52**(100%)
- 已修复: **26**

> **复核轮次**: 第一轮(post 拆分时)→ 15 项;第二轮(深度逐端点重读)→ 又找到 11 项。
> 第二轮重点核查"BE 重构后字段微调,FE 未跟上"以及"FE 比 BE 更严/更松"的隐性错位。

---

## 认证 / 用户(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/user/bio` | 已跳过 | OAuth 代理 → `PATCH /auth/me { bio }`,bio max=107 双端对齐 |
| `/user/username` | 已跳过 | OAuth 代理 → `PATCH /auth/me { name }`(handler 翻译 username→name),max=17 + `isValidName` regex `{1,17}` 对齐 |

## 消息(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/message/system/read` | 无问题 | 无 body,通用"全部已读" |
| `/message/admin/read` | 无问题 | 无 body,管理员消息已读 |

## 话题(13)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/topic/:tid` | 无问题 | 完整 update,字段全对齐 |
| `/topic/:tid/like` | 已修复 | 清死 body(BE 只用 path) |
| `/topic/:tid/dislike` | 已修复 | 清死 body |
| `/topic/:tid/upvote` | 已修复 | 清死 body |
| `/topic/:tid/favorite` | 已修复 | 清死 body |
| `/topic/:tid/hide` | 已修复 | 清死 body |
| `/topic/:tid/best-answer` | 无问题 | body 带 `topicId+replyId` BE 都用 |
| `/topic/:tid/reply` | 已修复 | BE service 加 content+targets 全空兜底 |
| `/topic/:tid/reply/like` | 无问题 | body 带 `replyId` |
| `/topic/:tid/reply/dislike` | 无问题 | body 带 `replyId` |
| `/topic/:tid/reply/pin` | 无问题 | body 带 `topicId+replyId` |
| `/topic/:tid/comment/like` | 已修复 | FE URL path 改用 `comment.topicId`(原来错填 `comment.id`) |
| `/topic/:tid/poll` | 已修复 | 字段对齐;BE service 修 `role <= 2` → `role < 2`(原 bug 让 admin 改不了别人投票) |

## 网站(2 公开)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/website/:domain/like` | 无问题 | BE 用 path 参数 |
| `/website/:domain/favorite` | 无问题 | BE 用 path 参数 |

## Galgame 核心(9)

| 路径 | 状态 | 备注 |
|---|---|---|
| ~~`/galgame/messages/read-state`~~ | 已退役 | wiki 消息读取游标 — wave 169 撤除(上游 feed 随 wiki 退役) |
| `/galgame/:gid/like` | 已修复 | 清死 body(BE 只用 path) |
| `/galgame/:gid/favorite` | 已修复 | 清死 body |
| `/galgame/:gid/comment` | 无问题 | `commentId+content` 对齐,content max=5000 |
| `/galgame/:gid/comment/like` | 已修复 | FE 字段 `galgameCommentId → commentId`(组件层 + schema 层) |
| `/galgame/:gid/resource` | 已修复 | FE `link.max 107→20` 对齐 BE(POST 创建也受益) |
| `/galgame/:gid/resource/like` | 无问题 | body 带 `galgameResourceId` |
| `/galgame/:gid/resource/valid` | 无问题 | body 带 `galgameResourceId` |
| `/galgame/:gid/resource/expired` | 无问题 | body 带 `galgameResourceId` |

## Galgame 评分(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/galgame-rating/:id` | 无问题 | snake_case 字段(`play_status` 等)双端对齐 |
| `/galgame-rating/:id/like` | 无问题 | body 带 `galgameRatingId` |
| `/galgame-rating/:id/comment` | 无问题 | content max=1314 双端对齐 |

## Galgame Wiki 代理(7)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/galgame/:gid` | 无问题 | wiki 写代理(PATCH draft 也走这里) |
| `/galgame/:gid/prs/:id/merge` | 无问题 | PR 合并,wiki 端鉴权 |
| `/galgame/:gid/prs/:id/decline` | 无问题 | PR 拒绝 |
| ~~`/galgame-tag`~~ | 已修复 | BE proxy 自动翻译 `tagId → tag_id` 给 wiki(见 `wiki_service.go renameTaxonomyIDField`) | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-official`~~ | 已修复 | BE proxy 翻译 `officialId → official_id` | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-engine`~~ | 已修复 | BE proxy 翻译 `engineId → engine_id` | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-series/:id`~~ | 无问题 | FE `Detail.vue:38` 手工把 `galgameIds → galgame_ids` | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)

## 工具集(4)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/toolset/:id` | 无问题 | 主信息更新对齐 |
| `/toolset/:id/practicality` | 已修复 | FE 删多余 `toolsetId`(BE DTO 只接 `rate`) |
| `/toolset/:id/comment` | 无问题 | `commentId+content` 对齐 |
| `/toolset/:id/resource` | 已修复 | BE 改返回 resource 而非 `OKMessage`;schema 加 `type` superRefine 按 s3/user 模式分支 |

## 管理员(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/admin/setting/register` | 无问题 | 无 body,切换注册开关 |
| `/admin/galgame/:gid/status` | 无问题 | wiki proxy,审核通过 / 拒绝 |

## 文档(Doc, admin)(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/doc/article` | 已修复 | camelCase 统一 + `banner.max 777→500` 对齐 BE,`description` 改 optionalString(1000)(原 FE min(1).max(777) 太紧 + 上限太低) |
| `/doc/category` | 已修复 | `slug.max 233→100`、`description.max 777→500`、`icon.max 128→200` 对齐 BE |
| `/doc/tag` | 已修复 | `slug.max 233→100`、`title.max 128→100`、`description.max 255→500` 对齐 BE |

## 网站(admin)(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/website/:domain` | 已修复 | BE 加 `Domain[]` + `CreateTime`,FE 字段 camelCase 对齐;BE 加 `description.min=10` + `icon` url validate(原 FE 严 BE 松) |
| `/website-category` | 已修复 | BE 加 name/label/description max 约束(原 BE 完全无 max) |
| `/website-tag` | 已修复 | FE `level` 补 `min(0)` 对齐 BE |

## 更新日志(admin)(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/update/history` | 已修复 | BE 加 version/content_* max 约束(原 BE 完全无 max) |
| `/update/todo` | 已修复 | BE 加 status `min=0,max=10` + content_* max 约束 |

---

## PUT 已修复问题清单(15 项)

| # | 端点 | 修复 |
|---|---|---|
| 1 | `/topic/:tid/like` | 清死 body |
| 2 | `/topic/:tid/dislike` | 清死 body |
| 3 | `/topic/:tid/upvote` | 清死 body |
| 4 | `/topic/:tid/favorite` | 清死 body |
| 5 | `/topic/:tid/hide` | 清死 body |
| 6 | `/topic/:tid/reply` | BE service 加 content+targets 全空兜底 |
| 7 | `/topic/:tid/comment/like` | URL path 用 `comment.topicId` |
| 8 | `/galgame/:gid/like` | 清死 body |
| 9 | `/galgame/:gid/favorite` | 清死 body |
| 10 | `/galgame/:gid/comment/like` | 字段名 `galgameCommentId → commentId` |
| 11 | ~~`/galgame-tag`~~ | BE proxy 自动 camel→snake | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| 12 | ~~`/galgame-official`~~ | 同上 | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| 13 | ~~`/galgame-engine`~~ | 同上 | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| 14 | `/toolset/:id/practicality` | 删多余 `toolsetId` 字段 |
| 15 | `/toolset/:id/resource` | 返回 resource 而非 OKMessage;schema 加 type superRefine |
| 16 | `/doc/article` | model + DTO JSON tag camelCase 统一 |
| 17 | `/website/:domain` | BE 加 Domain[]/CreateTime,FE 字段对齐 |

> 注:`/doc/article` 跟 `/website/:domain` 的修复同时涉及对应 POST,在 post.md 中只计一次。这里列出来是为了表明 PUT 端点也受益。

### 第二轮深度复核新发现(11 项)

| # | 端点 | 修复 |
|---|---|---|
| 18 | `/galgame/:gid/resource` | FE `link.max 107→20`(BE 始终 max=20,FE 5 倍偏差) |
| 19 | `/doc/category` | FE `slug.max 233→100`(用新建的 `shortSlugSchema`)、description 777→500、icon 128→200 |
| 20 | `/doc/tag` | FE `slug.max 233→100`、title 128→100、description 255→500 |
| 21 | `/doc/article` | FE `banner.max 777→500`;description 改 `optionalString(1000)`(移除 `min(1)` 配合 BE) |
| 22 | `/website-tag` | FE `level` 补 `min(0)` |
| 23 | `/website/:domain` | BE 加 `description.min=10` + `icon` url validate(对齐 FE,封堵直接 API 调用绕过) |
| 24 | `/website-category` | BE 加 name/label/description max 约束(原 BE 三个字段都无 max,直接 API 能写超长) |
| 25 | `/update/history` | BE 加 version/content_* max 约束(原 BE 完全无 max) |
| 26 | `/update/todo` | BE 加 status `min=0,max=10` + content_* max 约束 |
| 27 | `/topic/:tid/poll` | BE service `role <= 2` → `role < 2`(原 bug 让 role=2 的 admin 改不了别人投票) |
| 28 | `/galgame/:gid/comment/like` | FE schema `galgameCommentId → commentId`(组件早已对齐,schema 错位独立修)|

> 第二轮发现的特点:**BE 重构时部分 DTO 漏写 max/min 约束**(category/update-log/todo 等),让"FE 严 BE 松"的直接 API 调用能绕过校验。已统一在 BE 一侧加约束,FE 不动。

---

## 检查方法论(摘录)

每个 PUT 端点核对:

1. **路径参数**:`:tid` `:gid` `:id` 等 BE 是否实际使用,FE 是否传对
2. **请求体字段**:Go DTO `json:"..."` tag vs FE kunFetch `body` 字段名 + zod schema
3. **校验约束**:Go `validate:"..."` (min/max/oneof) vs FE zod 约束
4. **响应形状**:BE response DTO vs FE 期待的 TS 类型(`useKunFetch<T>` / `kunFetch<T>`)
5. **dead body**:FE 发了但 BE handler 完全忽略的字段
6. **proxy 翻译**:wiki 代理是否需要字段名 camel↔snake 转换

`go build ./...` + `pnpm typecheck` 全程通过。
