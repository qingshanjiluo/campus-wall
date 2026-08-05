# 数据库 Schema 主权

> 本文档说明 kungalgame 数据库 schema 的归属、FK 行为约定,以及 Go 代码层如何引用它们。

## TL;DR

- **真理之源**: `apps/api/migrations/*.sql` —— Go 仓库内显式 SQL migrations。
- **GORM AutoMigrate**: **不用**。Go API 启动不重建表,只通过 `pnpm migrate` 运行 SQL。
- **历史**: 老 Nitro+Prisma 时代的 schema 已 `pg_dump -s` 进 `000_baseline.up.sql`,后续增量在 `001-NNN_*.sql`。Prisma 已从仓库移除。

## FK ON DELETE 行为分布

总共 108 个 FK 约束。**默认行为 = CASCADE**(104 个),写代码时如果一个 FK 没在 model 里特别标注,就当 CASCADE 处理。剩 4 个特例如下,**对应 GORM model 必须有显式注释 + `constraint:OnDelete:X` 文档 tag**:

| 表.列 | 引用 | 行为 | 语义 | model 位置 |
|---|---|---|---|---|
| `galgame_rating.galgame_id` | `galgame(id)` | **RESTRICT** | 删 galgame 时,若仍有评分则拒绝。评分是用户创作内容,不应跟着 wiki 实体一起消失 | `internal/galgame/model/rating.go` |
| `galgame_website.category_id` | `galgame_website_category(id)` | **RESTRICT** | 删分类时,若仍有 website 则拒绝。分类是 taxonomy,必须先迁移/删除其下站点 | `internal/website/model/website.go` |
| `topic.best_answer_id` | `topic_reply(id)` | **SET NULL** | 删 reply 时,topic 的 best-answer 指针自动清空 | `internal/topic/model/topic.go` |
| `topic.pinned_reply_id` | `topic_reply(id)` | **SET NULL** | 删 reply 时,topic 的 pinned 指针自动清空 | `internal/topic/model/topic.go` |

## 在 Go 代码里依赖 FK CASCADE 时的规则

1. **DB FK CASCADE 由 PostgreSQL 直接执行**,Go 不需要在事务里手动删子行。但 GORM model 里同时也用 association(`hasMany` / `belongsTo`)时,GORM 也会尝试操作 —— 这种 model 在 kungalgame 里几乎不存在(我们都用扁平 `xxx_id int` 字段),所以无冲突。

2. **CASCADE 行为对依赖端 (denorm counter) 不可见**。例如 `galgame_website_comment.parent_id` 是 CASCADE,删 root 时 PG 自动删所有子评论。**但 `galgame_website.comment_count` 不会自动减** —— 这是 app 层 denorm,要手动 `AdjustCommentCount(-subtreeSize)`。已有先例:
   - `internal/galgame/service/comment_service.go:DeleteComment` 用 recursive CTE 算 subtreeSize
   - `internal/website/service/comment_service.go:DeleteComment` 同上

3. **PR 评审 / 新功能** 涉及删除主行时,一定要:
   - 查 `apps/api/migrations/000_baseline.up.sql` 里这张表的 ALTER TABLE ADD CONSTRAINT FOREIGN KEY 子句
   - 确认 `ON DELETE` 行为是 CASCADE / SET NULL / RESTRICT
   - 想清楚 denormalized counter 是否需要手动调整

4. **测试环境 schema** 由 `pnpm migrate` 自动建,等价于生产。不再依赖任何 Prisma artifact。

## 在 GORM model 里加 `constraint:OnDelete:X` 的约定

- **CASCADE**(默认行为)在 model 里**不显式标注**,减少噪音
- **SET NULL** 或 **RESTRICT** 必须显式标注:
  ```go
  XID *int `gorm:"column:x_id;constraint:OnDelete:SET NULL" json:"x_id"`
  YID  int `gorm:"column:y_id;constraint:OnDelete:RESTRICT" json:"y_id"`
  ```
- tag 本身只是文档 —— GORM 只在 AutoMigrate 时才解析它。本项目不跑 AutoMigrate,所以 tag 对运行时无效。**真正起作用的是 DB 里的 FK 约束**(由 `000_baseline.up.sql` 落到 schema)。

## 改 schema 的流程

1. 写新的 `NNN_*.up.sql` + `NNN_*.down.sql`(`NNN` 接着现有最大序号往后,如 `012_xxx.up.sql`)
2. `pnpm migrate` 应用到本地 dev DB,验证
3. 如果涉及非 CASCADE 的 FK 行为,在对应 GORM model 加注释 + `constraint:OnDelete:X` doc tag
4. **不要**修改 `000_baseline.up.sql` —— 那是历史快照,只能整体重生成(运维场景才需要)
5. PR 描述里说明对哪个表加了什么 FK,行为是什么

## 重生成 baseline 的场景

仅在以下情况需要:
- 完成一次大规模 schema 重构,觉得 001-NNN 累积太散乱
- 想 squash 历史 migrations

操作:
```bash
pg_dump -h <prod> -U <user> -d kungalgame -s --no-owner --no-acl --no-comments > /tmp/raw.sql
# 走 docs/proj/checks 里记录的同样 sanitize 脚本
# 替换 000_baseline.up.sql,并准备 `INSERT INTO _migrations ('000_baseline')` 的 prod 部署 runbook
```
