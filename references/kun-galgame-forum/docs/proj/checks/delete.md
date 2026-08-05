# DELETE API 字段对齐检查

> 目的:记录全部 DELETE 端点以及 FE↔BE 字段对齐审计状态。
>
> 路由源:`apps/api/internal/app/router.go`
>
> 配套文档:[post.md](./post.md) / [put.md](./put.md)

## 图例 (状态列取值)

- 无问题 — 已审计,FE/BE 对齐无问题
- 已修复 — 已审计,**发现错位并修复**
- 已跳过 — 已审计,设计上有意保持当前行为(详见备注)

## 统计

- 全部 DELETE 端点: **27**
- 已审计: **27**(100%)
- 已修复: **2**

> DELETE 通常没有 request body,只用 path 或 query param,所以错位率天然较低。
> 主要风险是 FE 传错 query 名 / 多传无用字段 / BE 用 path 但 FE 用 query。

---

## 话题(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/topic/:tid/reply` | 无问题 | BE 用 query `replyId`,FE 同名传入 |
| `/topic/:tid/comment` | 无问题 | BE 用 query `commentId`,FE 同名传入 |
| `/topic/:tid/poll` | 无问题 | BE 用 query `poll_id`(snake_case),FE 用 snake_case |

## 消息(1)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/message/:id` | 已修复 | 清掉 FE 多余的 `?messageId=` query(BE 只用 path `:id`) |

## 网站(1 公开)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/website/:domain/comment` | 已修复 | 删除前端死的 `updateCommentSchema`(BE 没有 PUT 端点) |

## Galgame 核心(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/galgame/:gid` | 无问题 | 删除草稿(status IN (3,4) + 拥有者),wiki 端双重校验 |
| `/galgame/:gid/comment` | 无问题 | BE 用 query `commentId`,FE 同名 |
| `/galgame/:gid/resource` | 无问题 | BE 用 query `galgameResourceId`,FE 同名 |

## Galgame 评分(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/galgame-rating/:id` | 无问题 | BE 用 query `galgameRatingId`,FE 同名 |
| `/galgame-rating/:id/comment` | 无问题 | BE 用 query `galgameRatingCommentId`,FE 同名 |

## Galgame Wiki 代理(7)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/galgame/:gid/links` | 无问题 | wiki 端 body 含 `link_id`,FE 已对齐 snake_case |
| `/galgame/:gid/aliases` | 无问题 | wiki 端 body 含 `alias`,FE 已对齐 |
| `/galgame/:gid/contributors/:id` | 无问题 | 路径删除,无 body |
| ~~`/galgame-tag/:id`~~ | 无问题 | 路径删除,支持 `?force=true` 二阶段(query 透传给 wiki) | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-official/:id`~~ | 无问题 | 同上 | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-engine/:id`~~ | 无问题 | 同上 | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)
| ~~`/galgame-series/:id`~~ | 无问题 | 路径删除,无 body | ← 已退役 wave 169(词表写路径/staff lane 随 wiki 退役撤除)

## 工具集(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/toolset/:id` | 无问题 | 路径删除;BE 同步清 S3 资源(`s3.Delete`) |
| `/toolset/:id/comment` | 无问题 | BE 用 query `commentId`,FE 同名 |
| `/toolset/:id/resource` | 无问题 | BE 用 query `toolsetResourceId`,FE 同名;BE 同步清 S3 对象 |

## 文档(Doc, admin)(3)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/doc/article` | 无问题 | BE 用 query `articleId`,FE 同名 |
| `/doc/category` | 无问题 | BE 用 query `categoryId`,FE 同名 |
| `/doc/tag` | 无问题 | BE 用 query `tagId`,FE 同名 |

## 网站(admin)(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/website/:domain` | 无问题 | BE 用 query `websiteId`,FE 同名(注意:domain 在 path 是为了人类可读,实际删除用 ID) |
| `/website-tag` | 无问题 | BE 用 query `tagId`,FE 同名 |

## 更新日志(admin)(2)

| 路径 | 状态 | 备注 |
|---|---|---|
| `/update/history` | 无问题 | BE 用 query `updateLogId`,FE 同名 |
| `/update/todo` | 无问题 | BE 用 query `todoId`,FE 同名 |

---

## DELETE 已修复问题清单(2 项)

| # | 端点 | 修复 |
|---|---|---|
| 1 | `/message/:id` | FE 清掉冗余 `?messageId=` query 字段(BE handler 只用 path) |
| 2 | `/website/:domain/comment` | FE 删除死的 `updateCommentSchema`(本不存在的 PUT 端点) |

---

## 检查方法论(摘录)

DELETE 端点主要核对:

1. **ID 位置**:BE 是 `c.Params(":id")`(path)、`c.Query("xId")`(query)、还是 body?FE 必须传到正确位置
2. **字段名**:query 名 / path 名必须双端完全一致
3. **是否带 query 透传**:wiki 代理删除(tag/official/engine)支持 `?force=true` 二阶段,proxy 必须透传 query
4. **副作用清理**:BE 是否在删除时同步清 S3 / 缓存 / 关联表(toolset/galgame resource 涉及)

`go build ./...` + `pnpm typecheck` 全程通过。
