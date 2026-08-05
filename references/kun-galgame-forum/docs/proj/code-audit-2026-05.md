# 代码审计问题清单（kungal / kun-galgame-forum）

> 本文档汇总外部跨仓审计（`kungal-docs/claude` + `kungal-docs/gpt`，审计日期 2026-05-30）
> 中**与本仓库相关**的问题，并已**逐条对照当前代码复核（2026-05-29）**。oauth / moyu
> 两仓的独立问题不在此列；跨仓契约问题只保留 kungal 这一侧。
>
> 复核方法：对每条 finding 打开其引用的文件与行号，确认描述与**当前代码**一致后才收录；
> 行号若漂移已修正为真实位置。被审计「证伪」的候选也复核过，列在 §6 以示「看过、非漏」。
>
> 列号格式 `file:line` 可点击。审计源编号（`F0xx` / `GPT-xx`）保留以便回溯 `kungal-docs`。
>
> 严重度以本次复核为准；claude 与 gpt 评级有分歧时在条目内标注两方意见。

## 速查矩阵

| 严重度 | 数量 | 说明 |
|---|---:|---|
| **MEDIUM** | 8 | 用户可见功能失效 / 数据静默丢失 / 完整性可被刷 / 越权时序 |
| **LOW** | 20 | 静默吞错、时区/cron 漂移、非原子计数、误导性注释、一致性瑕疵 |
| 已核实「非问题」 | 5 | 1 条 verified-negative + 4 条证伪（best-effort 设计 / 不可达 / 无返回值 / 责任在 oauth 侧）|

> 注：claude 原始评级里 F017/F021/F022/F031 为 MEDIUM，gpt 复核下调为 LOW；本文采用 gpt 的
> LOW（理由见各条）。F036 两方均「不计入 bug、仅人工确认项」，本文列在 LOW 的「潜在风险」。

## 目录

- [1. MEDIUM 级问题（8）](#1-medium-级问题8)
- [2. LOW · 静默吞错簇（8）](#2-low--静默吞错簇8)
- [3. LOW · 积分 / 幂等（3）](#3-low--积分--幂等3)
- [4. LOW · 时序 / 时区 / cron（3）](#4-low--时序--时区--cron3)
- [5. LOW · 鉴权时序 / 其它（6）](#5-low--鉴权时序--其它6)
- [6. 已核实为「非问题」（5）](#6-已核实为非问题5)
- [7. 主题性结论与批量修法](#7-主题性结论与批量修法)
- [8. 与审计源 ID 对照](#8-与审计源-id-对照)

---

## 1. MEDIUM 级问题（8）

### M1 · `F018` — 评分接口对「从未被交互过的 galgame」会 500（缺 stub 懒创建）
- **位置**：`apps/api/internal/galgame/service/rating_service.go:224-231`
- **对照**：`comment_service.go:316-317`、`resource_service.go:221-222`、`repository/interaction_repo.go:47-48,70-71` 这四条本地写路径**都先** `tx.Clauses(clause.OnConflict{DoNothing:true}).Create(&model.GalgameLocal{ID: galgameID})` 懒建本地 stub，唯独 `CreateRating` 没有。
- **问题**：`galgame_rating.galgame_id` 是 `ON DELETE RESTRICT` 外键（`migrations/000_baseline.up.sql` `galgame_rating_galgame_id_fkey`），必须指向本地 `galgame` stub 行；而 stub 表是稀疏的（仅交互时创建，浏览 `IncrementView` 只 UPDATE 不插入）。对一个没人点赞/评论/上传过资源的 galgame 首次评分 → 外键冲突 → 事务回滚 → 返回笼统 `ErrInternal("创建评分失败")`（500）。
- **影响**：合法用户给冷门 galgame 评分得到不可解释的 500，评分没保存——核心功能间歇性失效。
- **修法**：`CreateRating` 在 INSERT 前补一行 stub 懒创建（与 comment/resource 路径完全一致；repo 已有 `CreateLocalStub`/`EnsureLocalStub` 助手）。
- **复核**：ACCURATE（两方均 MEDIUM）。

### M2 · `F023` — 私聊发送吞写库错误 + `max=1007` 超过 `varchar(1000)` → 静默丢消息
- **位置**：`apps/api/internal/message/service/chat_service.go:178-181`、`repository/chat_repo.go:238-244,247-253`、`dto/chat_dto.go:17`、`model/message.go:102`、`handler/chat_handler.go:73`
- **问题**：`SendChatMessage` 调 `InsertChatMessage` + `UpdateRoomLastMessage`，二者都是裸 `r.db.Exec(...)` 不检查 `.Error`、无返回值；service 直接 `return nil`，handler 恒回 200 `"发送成功"`。且 DTO `Content` 校验 `max=1007`，但列是 `varchar(1000)`——**1001–1007 字符的消息能过校验、写库溢出、错误被吞、消息丢失、用户却看到「发送成功」**。
- **影响**：已登录用户在「发送成功」假象下静默丢失聊天消息；也掩盖任何瞬时 DB 故障。
- **修法**：`InsertChatMessage`/`UpdateRoomLastMessage` 返回 error 并在 service 检查、失败回 `ErrInternal`；把 `chat_dto.go` 的 `max` 改为 1000 对齐列宽。
- **复核**：ACCURATE（两方均 MEDIUM）。

### M3 · `F019` — 话题点赞（upvote）可无限重复 → 刷 `upvote_count` + 反复给作者加分
- **位置**：`apps/api/internal/topic/service/topic_write_service.go:293-329`、`model/topic.go:124-128`、`repository/topic_repo.go:220`、`constants/moemoepoint.go:15-16`
- **问题**：`topic_upvote` 表**无 `(topic_id, user_id)` 唯一约束**（model 注释明示「allows duplicate upvotes」，实测只有 `topic_upvote_pkey`），且 `Upvote()` 写路径**不检查是否已 upvote**（存在 `HasUserUpvoted` 助手 `topic_repo.go:86-90`，但只在读路径 `topic_service.go:189` 用于显示 `isUpvoted`，写路径没调）。每次重复点赞都插一行、`upvote_count + 1`、扣发起者 `CostUpvoteSender=7`、给作者 `RewardUpvoteOwner=3`，且用 `KeyNonce`（非稳定键）无法去重。
- **影响**：任意登录用户可对非自己的话题无界刷 `upvote_count`（排序/置顶操纵，`upvote_time` 还会顶 `status_update_time`），并反复给指定作者加分；上限仅受攻击者自身余额限制。**非印钞**（发起者净亏 7、作者得 3），属完整性/刷量缺陷。
- **修法**：upvote 改为每人每帖一次——`(topic_id,user_id)` 唯一索引，或写前 `HasUserUpvoted` 拦截，或改用稳定幂等键 `kungal:topic_upvote:<tid>:<uid>`。
- **复核**：ACCURATE（细节：guard 助手存在但写路径未用；两方均 MEDIUM）。

### M4 · `F020` — 会话角色登录时定格，token 刷新不重算 → 降权后最长 7 天仍是管理员
- **位置**：`apps/api/internal/user/service/auth_service.go:97,110,121`、`internal/middleware/auth.go:260-269`、`internal/middleware/role.go:17`
- **问题**：`OAuthCallback` 只在登录时 `role := middleware.RoleFromOAuthRoles(oauthUser.Roles)` 算一次，写入 Redis 会话（7 天 TTL）。`refreshSession` 刷新时**只换 access/refresh token**、原样 re-marshal 同一个 `UserInfo`，从不重新 `FetchUserInfo`、也不重算 `Role`。`RequireRole`（admin 路由组 `RequireRole(3)`、版主组 `RequireRole(2)`）读的就是这个缓存角色。
- **影响**：OAuth 中枢把某人降权后，kungal 在其会话有效期内（最长 7 天，或主动登出前）仍当其为 admin/版主——中心化权限回收被本地缓存绕过。
- **修法**：刷新时重新拉 `/oauth/userinfo` 并重算 `Role` 再落库；或在 `RequireRole` 路由上用新 access token 的 roles claim 现算；或缩短会话 TTL。
- **复核**：ACCURATE（两方均 MEDIUM）。**相关**：`F032`（封禁滞后）窗口小得多——封禁会在刷新时被 `IsBanned`(10014) 捕获删会话，受 access TTL 约束（≈15min，刷新提前 30s）；真正长的滞后是这里的「角色从不重算」。

### M5 · `F003` — wiki 审核 +3 是「至多一次」：SETNX 去重守卫在发奖之前 → 崩溃窗口永久丢分
- **位置**：`apps/api/internal/galgame/service/wiki_message_sync.go:135`(SETNX)、`:175`(Award)、`internal/moemoepoint/pusher.go:79-87`(吞错)、`wiki_message_sync.go:72`(`processedGuardTTL=30d`)、`:113-115,124-125`(游标无条件前移)
- **问题**：处理每条消息时**先**取 `wiki:msg:processed:<id>` SETNX 守卫（30 天），**后**才 `moemoepoint.Award`；而 Award 是 fire-and-forget，失败只 `slog.Warn`。若 SETNX 成功后、发奖落地前进程崩溃或 OAuth 5xx，守卫已置 30 天、游标已前移、feed 基于 `since_id` 不再重投 → 这条 +3 永久丢失。稳定幂等键只能防重复发、**救不回丢失的发奖**。
- **影响**：投稿者在偶发 OAuth/重启时静默少拿 +3；kungal 归因的积分总额随时间低于 C3 账本，无告警。
- **缓解（已存在）**：键稳定 + `cmd/sync-moemoepoint` 带外重算可补；属「下游积分 best-effort」设计取舍。故定 MEDIUM 而非更高。
- **修法**：把 SETNX 守卫**移到 Award 成功之后**；或发奖失败时不前移该消息游标，靠稳定键防重让下轮重试。
- **复核**：ACCURATE（两方均 MEDIUM）。

### M6 · `F016` — wiki `applyMessage` 注释声称「同事务原子」，实际发奖在事务外且两处错误都被丢弃
- **位置**：`apps/api/internal/galgame/service/wiki_message_sync.go:155-156`(注释)、`:167-170`(事务闭包)、`:175`(Award 在闭包外)、`repository/galgame_repo.go:81-84`(`CreateLocalStub` 无返回值)
- **问题**：注释写「Award the submitter +3 atomically in the same tx so a partial failure rolls back both」，但实际 `moemoepoint.Award` 在 `Transaction(...)` 闭包**之外**；且 `Transaction(...)` 的返回 error 没接收、`CreateLocalStub` 本身 `tx.Clauses(OnConflict{DoNothing}).Create(...)` 不返回 error——**两处错误双重吞掉**。结果可能是「发了奖但没 stub 行」（galgame 不出现在 kungal 列表），或反之，且无日志。
- **影响**：审核通过的 galgame 在 kungal 列表中静默缺失（缺 stub），或发奖/落库不一致，难以发现对账。
- **修法**：接住并记录 `Transaction(...)` 返回值，让 `CreateLocalStub` 返回 `Create(...).Error`；真要原子可用 outbox 行同事务记录、提交后再推 oauth。
- **复核**：ACCURATE（两方均 MEDIUM）。**注**：发奖移到事务外是本季度「避免 I/O-in-tx」重构有意为之；遗留的是**注释未同步**（注释撒谎）+ stub 错误吞掉，修法应至少改正注释并让 stub 返错。

### M7 · `GPT-M02` — 工具集上传后端不按字节执行每日额度，大小不符只 warn，计数只 +1
- **位置**：`apps/api/internal/toolset/service/upload_service.go:210-221`、`apps/web/app/config/upload.ts:9`
- **问题**：后端只有单文件上限（50MB/2GB，`upload_service.go:78-79,123-124`），**没有按字节的每日额度校验**；`actualSize != entry.FileSize` 时只 `slog.Warn` 不拒绝（`:214-217`）；`daily_toolset_upload_count` 只 `+1`（按次计，非按量，`:220-221`）。前端有 `USER_DAILY_UPLOAD_LIMIT = 100MB`（`upload.ts:9`）但后端从不执行。
- **影响**：前端字节额度可被绕过（直接打 API），用户可超量上传，存储成本/滥用面扩大；声明的大小与实际不符也不拦。
- **修法**：后端按字节累加并校验每日额度（读 `daily_toolset_upload_count` 或新增字节累计列）；大小不符应拒绝而非仅 warn。
- **复核**：ACCURATE（gpt 补充项，MEDIUM）。

### M8 · `F032` — 下游封禁/角色回收依赖 token 刷新生效，无逐请求回查
- **位置**：`apps/api/internal/middleware/auth.go:123-126`(`refreshSkew=30s`)、`:146-152`(happy path 不查 oauth)、`:245`(`IsBanned` 只在刷新时)
- **问题**：每请求只读 Redis 会话直接放行，**唯一**重新评估封禁/角色的时机是 token 刷新；kungal 提前 30s 主动刷新，封禁在 access TTL（≈15min）内会被捕获。属设计内的「仅验签 + 刷新时回查」。
- **影响**：刚被封禁的用户在 ≤access-TTL 窗口内仍能做已认证写操作（发帖/评论/赚分/wiki 编辑）。窗口有界、非永久绕过。
- **修法**：对真正敏感的下游写操作加一层短 TTL 缓存的 oauth 封禁态回查；或缩短 access TTL。**与 M4 合看**：封禁窗口小（≈15min），角色降权窗口大（≈7 天，M4 才是重点）。
- **复核**：ACCURATE（cross 条目，此处只列 kungal 侧；MEDIUM）。

---

## 2. LOW · 静默吞错簇（8）

> 统一根因：repo 方法 `.Count()/.Exec()/.Delete()/.Scan()` 不接 `.Error`，service 无条件 `return nil`，handler 回 200。**统一修法**：让 repo 方法返回 error 并逐层上抛。

| ID | 位置 | 问题 | 复核 |
|---|---|---|---|
| `F021` | `internal/website/service/website_service.go:218-221`、`repository/website_repo.go:127-129`、`handler/website_handler.go:92-95` | 网站删除吞 GORM 错误，恒回「网站已删除」。子表 CASCADE 故 FK 拒绝少见，残留风险是**瞬时 DB 错误下的假成功** | 确认（claude MEDIUM→gpt LOW）|
| `F056` | `internal/galgame/service/comment_service.go:487-526` | `ToggleCommentLike` 整个事务内 `First/Create/Update/Delete` 全部丢弃错误、恒 `return nil`；对不存在的评论点赞也「成功」 | 确认 |
| `F057` | `internal/section/repository/section_repo.go:53-88` | `FindSectionTopics`/`FindCategoryStats` 签名无 error 路径，`Count/Find/Scan` 错误丢弃 → 200 空结果 | 确认 |
| `F058` | `internal/user/repository/content_repo.go:62,132,184,242,284,331,388` | 多处 `Count(&total)` 丢 `.Error`（成对的 `Scan().Error` 有检查）。**注**：仅 `content_repo`，`stats_repo` 已正确检查 | 确认（部分）|
| `F063` | `internal/admin/repository/overview_repo.go:33-37,42-49` | 管理后台总览 `CountTable`/`DailyCountsSince` 吞查询错误 → 假零 | 确认 |
| `F064` | `internal/message/repository/chat_repo.go:228-235` | 聊天已读回执在循环里每条消息一次 INSERT 往返，且错误忽略（可改单条多行 INSERT）| 确认 |
| `F065` | `internal/doc/repository/article_repo.go:96-123`、`category_repo.go:46-47,64-65`、`tag_repo.go:46-48` | doc 的**删除 + 部分更新**（标签关系、分类更新）丢弃 DB 错误。**注**：`Create` 与主 `Update` 已检查错误，非全部 | 确认（部分）|
| `F062` | `internal/message/service/chat_service.go:178-181`、`repository/chat_repo.go:143-157` | 聊天发送（消息 insert + room-last-message update）与私聊房创建是非原子的 fire-and-forget Exec，无事务（对比 website 路径有事务）| 确认 |

## 3. LOW · 积分 / 幂等（3）

| ID | 位置 | 问题 | 复核 |
|---|---|---|---|
| `F055` | `internal/galgame/service/interaction.go:27-29`、调用方 `rating_service.go:228`、`comment_service.go:330` | `AdjustMoemoepoint` → `moemoepoint.Award` 是异步 goroutine，从**事务闭包内部**触发；若外层事务回滚，发奖已发出 → 幻影加分（best-effort + 稳定/nonce 键兜底，影响有限）| 确认（实质成立）|
| `F061` | `internal/toolset/service/resource_service.go:202-203`、`toolset_service.go:137-138,315-316` | 工具集/资源 create(+3)/delete(-3) 发奖用 `KeyNonce`（`time.Now().UnixNano()` 非稳定键），once-per-event 语义无法重放去重 | 确认 |
| `F017` | `internal/galgame/service/submission_service.go:63-66`(注释)、`:83`(默认 handle)、`:86`(Award)、`:29`(`stateRepo` 未用) | Claim 注释承诺「+3 atomic with FOR UPDATE lock on kungal_user_state」，实际**无事务、无 FOR UPDATE**，`stateRepo` 字段从未被引用，`CreateLocalStub` 错误丢弃。正确性实际靠 wiki status=2 + 稳定键。属**误导性注释**（同 M6，本季度重构遗留）| 确认（claude MEDIUM→gpt LOW）|

## 4. LOW · 时序 / 时区 / cron（3）

| ID | 位置 | 问题 | 复核 |
|---|---|---|---|
| `F085` | `internal/infrastructure/cron/cron.go:21,24` | `cron.New()` 无 `cron.WithLocation`，`"0 0 * * *"` 按**进程本地时区**午夜触发；服务器跑 UTC 而用户在 CST/JST 时，每日签到/计数重置边界与用户日历日不对齐 | 确认 |
| `F084` | `internal/admin/service/overview_service.go:107`、`repository/overview_repo.go:45` | 日统计下界 `since := time.Now().AddDate(0,0,-days)` 是**未截断到 0 点**的墙钟；而桶按 `date_trunc('day',created)` → 最老一天只从当前时刻起算，少计 | 确认 |
| `F031` | `migrations/012_system_message_read_state.up.sql:52-57`、`cmd/migrate/main.go:25` | 012 按 `"user".id` 回填消息已读游标；若早于 oauth `migrate-users` 重映射 id 运行，游标指向旧 id，而 migrate-users 不重映射此表。仅靠运行顺序约定（默认 `--exclude` 排除 012 + 文档）保证 | 确认（claude MEDIUM→gpt LOW）|

## 5. LOW · 鉴权时序 / 其它（6）

| ID | 位置 | 问题 | 复核 |
|---|---|---|---|
| `GPT-L03` | `internal/middleware/auth.go:158-188`(OptionalAuth) vs `:123-144`(Auth) | `OptionalAuth` 无 `refreshSkew`/刷新逻辑，对 Bearer 转发的读路径（如 `GET /galgame/:gid` 转发 wiki）会转发**过期的** OAuth access token → 用户自己的草稿/私有读可能 401，直到一次强制 Auth 请求触发刷新。非安全升级（转发的是更弱的过期 token）| 确认 |
| `F060` | `internal/website/repository/website_repo.go:97-104` | 网站详情按 `url ILIKE '%'+domain+'%'` 子串匹配解析（GetDetail/Update 用此路径），子串碰撞会命中/改到**错误的网站**，破坏 url 作为标识符 | 确认 |
| `F088` | `apps/web/app/pages/message/system.vue:13`、`notice.vue:13`、`internal/message/service/message_service.go:75-106` | 同级消息页 sort 参数名不一致：`notice.vue` 发 `sortOrder`（后端消费），`system.vue` 发 `order`（`GetSystemMessages` 根本不读 → 该 `desc` 是死参数）| 确认 |
| `F077` | `internal/user/oauth/client.go:29-30,63-73` | kungal 把 oauth invalid-grant 归为 `15005`、invalid-client-secret 归为 `15008`（与 moyu 分类不一致）；信息性、跨仓一致性 | 确认 |
| `F036` | `apps/web/app/components/message/aside/System.vue:31` | 系统/管理广播用 `v-html="message.content['zh-cn']"` **无 sanitizer**（moyu 同类 sink 有 `sanitize()`）。**当前** `apps/api` 内**无任何 `INSERT INTO system_message`**（内容由 admin/外部服务写入），故是**潜在** stored-XSS，非当前可达。建议加 DOMPurify 做纵深防御 | 确认（潜在风险，两方均不计入 bug）|
| — | `apps/web` 其它 `v-html` 汇点：`galgame/SnapshotDiff.vue`、`unmoe/Log.vue`、`edit/topic/SpecialNotice.vue` | 同类未净化 sink，内容多为系统生成的 diff/日志；与 F036 同批一并审视净化策略 | 关联项 |

---

## 6. 已核实为「非问题」（5）

> 这些被审计「证伪」或判为 verified-negative，我复核后确认**不必修**，列出以免重复排查。

| ID | 位置 | 结论 |
|---|---|---|
| `F059` | `internal/middleware/auth.go:158-188` | **verified-negative（非 bug）**：`OptionalAuth` 对匿名请求正确地**不**当作已登录——无 token/缓存未命中/反序列化失败都 `c.Next()` 不挂身份 |
| `F005` | `internal/infrastructure/cron/cron.go` | **正确证伪**：kungal 确实不给 oauth 图床 hash 续期（refping），但这是**有意**的——oauth 侧自带 avatar/galgame image refping job 从自己库续期，下游不 ping 不致死链 |
| `F052` | `internal/moemoepoint/pusher.go:82-87` | **正确证伪**：发奖 goroutine 吞错+`slog.Warn` 是**有意 best-effort**（包注释明示「rarely-lost point is acceptable」），非静默 bug |
| `F053` | `internal/moemoepoint/pusher.go:74-78,101` | **正确证伪**：`delta` 绝对值被钳到 1,000,000，但本应用真实 delta 极小（±1/±3/+7），钳位**不可达**，无害 |
| `F080` | `internal/galgame/service/wiki_message_sync.go:175-177` | **正确证伪**：`Award` 是 fire-and-forget **无返回值**，根本没有 error 可丢弃（与 M5/M6 重叠）|

---

## 7. 主题性结论与批量修法

1. **静默吞错（本仓最大簇，§2 共 8 条 + 散见）**：大量 repo 方法不接 `.Error`、service 恒 `return nil`、handler 回 200。后果从「管理面板假零」到「聊天/网站/doc 假成功」。**统一修法**：repo 方法签名带 `error` 并逐层上抛；handler 把约束冲突映射 4xx、其它映射 5xx。
2. **积分 / 幂等时序**：best-effort 推送整体正确（C3 账本对账=0），但反复出现「守卫先于发奖」(M5)、「承诺的事务锁不存在」(F017)、「发奖在事务闭包内触发」(F055)、「once-per-event 用 nonce 键」(F061)。**修法**：稳定幂等键 + 守卫置于发奖成功之后 + 发奖放到事务提交之后。
3. **误导性注释（M6/F017，本季度重构遗留）**：wiki_sync 与 submission 的注释仍称「同事务原子 / FOR UPDATE 锁」，但代码已无。**最小修法**：同步注释，并让 `CreateLocalStub` 返错。
4. **下游角色/封禁滞后（M4/M8）**：降权最长滞后 7 天（角色从不重算），封禁滞后 ≈access TTL。**修法**：刷新时重算角色（M4 优先），敏感写路径回查 oauth 封禁态。
5. **时区 / cron（F085/F084）**：进程本地时区 vs 用户日历日。**修法**：cron `WithLocation(Asia/Shanghai)`，统计下界 `date_trunc` 到 0 点。
6. **校验与列宽/额度不一致（M2/M7）**：DTO `max=1007` > 列 `varchar(1000)`；前端字节额度后端不执行。**修法**：校验对齐底层约束，额度在后端执行。

## 8. 与审计源 ID 对照

- 源目录：`../kungal-docs/claude`（88 条全仓，本仓 23 + cross 4）、`../kungal-docs/gpt`（对比复核后最终裁决）。
- 本文收录 = kungal 仓 confirmed + cross 的 kungal 侧 + gpt 补充的 kungal 项（`GPT-M02`/`GPT-L03`）。
- 未收录：oauth / moyu 两仓的独立问题（如 `F001`/`F002`/`F004` 三条 HIGH 均不在本仓）。
- 严重度分歧：`F017`/`F021`/`F022`/`F031` claude=MEDIUM、gpt=LOW，本文取 LOW；`F003`/`F016`/`F018`/`F019`/`F020`/`F023` 两方一致 MEDIUM。
- 复核基准：当前代码（2026-05-29）。所有 file:line 已对照真实位置；行号漂移已修正。
