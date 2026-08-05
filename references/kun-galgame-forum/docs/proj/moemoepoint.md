# 萌萌点：用户限制 与 全部增减来源（2026-06-15）

> 本仓自有工程笔记（**非** infra 镜像）。把「萌萌点如何限制用户」和「萌萌点的每一条
> 增减来源」一次盘清。权威契约见只读镜像 `docs/oauth/06-moemoepoint.md`（infra 拥有，
> 改契约去 infra 改）；本文记录的是 **kungal 侧的具体业务规则与金额**，这些是本仓自己定的。

## 0. 架构定位（先理解这点，下面才不歧义）

- **单一真源在 OAuth**。每个用户**全站一个余额**（kungal / 摸鱼 / 贴纸共享），存在 OAuth
  的 `users.moemoepoint`。本仓 `kungal_user_state`（及 `users`）里的萌萌点是**缓存视图**，
  不是真源。
- **kungal 只会算 delta，不持有账本**。所有增减都走 s2s API：
  `POST /users/:id/moemoepoint`（OAuth Client Basic Auth + 铸币白名单 `moemoepoint_awarder=true`）。
  推送实现见 `internal/moemoepoint/pusher.go`，业务侧统一调 `moemoepoint.Award()`（异步、best-effort、
  不阻塞核心写）/ `moemoepoint.AwardSync()`（仅 wiki cron 用，要先确认推成功再推进游标）。
- **幂等键** = `<app>:<event>:<ref>`，如 `kungal:checkin:1207:2026-06-15`、`kungal:claim:42:1207`。
  两类键：可重放事件（签到、cron、claim）用**稳定键**；用户即时动作（点赞/收藏/推）用
  `KeyNonce(reason, ref)` 的**一次性 nonce**（避免重试被误判重复）。
- **余额允许为负**（OAuth 不做非负约束，保证回收/反转永不被挡）；`|delta| ≤ 1,000,000`。
- 本仓事务内先做余额门槛检查（`SELECT … FOR UPDATE` 锁 `kungal_user_state`），通过后才
  在同一事务里记账并触发推送。

OAuth 侧的 reason 枚举（共 8 个，4 个下游可用 / 4 个 OAuth 保留）：

| reason | 方向 | 谁能用 | kungal 用途 |
|---|---|---|---|
| `content_approved` | + | 下游 s2s | 发内容 / 被推 / 被采纳 / @回复 等正向产出 |
| `content_removed` | − | 下游 s2s | 删内容 / 付费分区扣费 / 推话题花费 等回收 |
| `daily_checkin` | + | 下游 s2s | 每日签到 |
| `liked` | + | 下游 s2s | 被点赞 / 被收藏（发给**内容拥有者**，不是点赞者） |
| `admin_grant` / `admin_deduct` | ± | **OAuth 保留**，s2s 禁用 | 管理员在统一账号后台手动发放/扣除 |
| `migration` | + | **OAuth 保留** | 一次性迁移起始值 |
| `register_gift` | + | **OAuth 保留** | 注册礼物（见 §1） |

---

## 1. 萌萌点的全部增减来源

> 金额常量见 `apps/api/internal/constants/moemoepoint.go`。点赞/收藏类奖励一律给
> **被赞者（内容拥有者）**，取消则等额回收。删除类**不退原奖励**，而是按规则**另扣**作者
> 一笔（防「发了删、删了发」刷分）。

### 1.1 注册礼物（OAuth 侧，唯一不在本仓触发的来源）

| 事件 | reason | 金额 | 幂等键 | 说明 |
|---|---|---|---|---|
| 新账号在 OAuth 注册成功 | `register_gift` | **+7** | `oauth:register_gift:<id>` | best-effort，不阻塞注册；note=「鲲给予你的第一份礼物」。本仓无法触发（保留 reason）。`infra .../auth_service.go:33,285` |

### 1.2 签到

| 事件 | reason | 金额 | 幂等键 | 说明 |
|---|---|---|---|---|
| 每日签到 `POST /user/checkin` | `daily_checkin` | **+0~7（随机 `rand.IntN(8)`）** | `kungal:checkin:<uid>:<YYYY-MM-DD>` | 原子「每天一次」闸门（`daily_check_in` 0→1）。可能抽到 0。`user_service.go:141` |

### 1.3 话题 topic

| 事件 | reason | 金额 | 给谁 | 可逆 |
|---|---|---|---|---|
| 发布话题（免费分区） | `content_approved` | **+3** (`RewardCreateTopic`) | 作者 | 无删除入口 |
| 发布话题（付费分区） | `content_removed` | **−10** (`CostConsumeSection`) | 作者 | — |
| 编辑话题切换分区 | `content_approved`/`content_removed` | **±13**（免↔付，按 footprint 差值） | 作者 | 是（切回反转） |
| 话题被点赞 / 取消 | `liked` | **+1 / −1** | 话题主 | 是 |
| 话题被收藏 / 取消 | `liked` | **+1 / −1**（收藏自己不计） | 话题主 | 是 |
| 话题被推（upvote） | `content_removed`（推者）/`content_approved`（主） | 推者 **−20** (`CostUpvoteSender`)，话题主 **+3** (`RewardUpvoteOwner`) | 双方 | 否（一次性） |
| 设为最佳回答 / 取消 | content_* | **+7 / −7**（字面量，非常量） | 被设的回复作者 | 是 | `topic_write_service.go:459` |

`topic_write_service.go`：创建 144 / 切区 240 / 点赞 277,288 / 收藏 431,444 / 推 384,386 / 最佳回答 `SetBestAnswer`。

### 1.4 回复 reply / 评论 comment

| 事件 | reason | 金额 | 给谁 | 可逆 |
|---|---|---|---|---|
| 发回复 | `content_approved` | **+1** (`RewardReply`) | 每个被 @ 的用户 + 话题主 | 否 |
| 回复/评论被点赞 / 取消 | `liked` | **+1 / −1** | 作者 | 是 |
| @ 提及（评论里点到人） | `content_approved` | **+1** | 被回复者 | 否 |
| **作者自删回复** | `content_removed` | **−3 ×（评论数+点赞数+目标数+被目标数+1）** | 作者 | 否 |
| **作者自删评论** | `content_removed` | **−3 ×（点赞数+1）** | 作者 | 否 |
| 版主删回复/评论 | `content_removed` | **−3（平价）** | 作者 | 否 |

`reply_service.go`：发回复 207-217 / 点赞 381,394 / 删 344。`comment_service.go`：@ 76 / 点赞 180,190 / 删 246。
删除按互动量放大惩罚（防刷），版主删则只扣平价 3。

### 1.5 Galgame 词条

| 事件 | reason | 金额 | 幂等键 / 给谁 | 可逆 |
|---|---|---|---|---|
| 提交/认领 galgame | `content_approved` | **+3** (`RewardCreateGalgame`) | `kungal:claim:<gid>:<uid>` / 提交者 | 否 |
| wiki 审核通过（cron 同步） | `content_approved` | **+3** | `kungal:wiki_approved:<msgid>` / 消息里的 TargetUserID | 否 |
| galgame 被点赞 / 取消 | `liked` | **+1 / −1** | 词条主 | 是 |
| galgame 被收藏 / 取消 | `liked` | **+1 / −1** | 词条主 | 是 |
| PR 合并 | `content_approved` | **+1** (`RewardPRMerge`) | PR 提交者 | 否 |
| galgame 评论 @ 到人 | `content_approved` | **+1** | 被回复者 | 否 |
| galgame 评论被点赞 / 取消 | `liked` | **+1 / −1** | 评论作者 | 是 |

`galgame_service.go`：PR 合并 171 / 点赞 312 / 收藏 337。`submission_service.go:89`。`wiki_message_sync.go:175`（**AwardSync**）。`comment_service.go:330,509`。

### 1.6 Galgame 评分 rating（按简评字数分级）

| 事件 | reason | 金额 | 说明 |
|---|---|---|---|
| 发布评分 | `content_approved` | **+3 / +5 / +10** | `len(short_summary)`：<233→3，[233,666)→5，≥666→10（`RatingRewardLow/Medium/High`） |
| 编辑评分（跨档） | content_* | **新档−旧档**的差值（可正可负） | 例 150→250 字 = +5−3 = +2；500→700 = +5−10 = −5 |
| 删除评分 | `content_removed` | **−（删除时所属档位）** | 即 −3 / −5 / −10 |
| 评分被点赞 / 取消 | `liked` | **+1 / −1** | 评分作者 |

`rating_service.go`：创建 238 / 改 298,316 / 删 348 / 点赞 394。

### 1.7 Galgame 资源 / 工具集 toolset

| 事件 | reason | 金额 | 给谁 | 可逆 |
|---|---|---|---|---|
| 发布 galgame 资源 | `content_approved` | **+3** (`RewardCreateResource`) | 作者 | 否 |
| 删除 galgame 资源 | `content_removed` | **−（5 + 点赞数）** | 作者 | 否 |
| galgame 资源被点赞 / 取消 | `liked` | **+1 / −1** | 作者 | 是 |
| 创建工具集 | `content_approved` | **+3** (`RewardCreateToolset`) | 创建者 | 否 |
| 删除工具集 | `content_removed` | **−3** | 拥有者 | 否 |
| 工具集加资源 | `content_approved` | **+3** | 作者 | 否 |
| 删除工具集资源 | `content_removed` | **−3** | 作者 | 否 |

`galgame/.../resource_service.go`：建 254 / 删 336 / 点赞 390。`toolset/.../toolset_service.go`：建 139 / 删 318。`toolset/.../resource_service.go`：建 98 / 删 190。

### 1.8 管理员手动调整（OAuth 后台，不在 kungal）

| 事件 | reason | 金额 | 说明 |
|---|---|---|---|
| 管理员发放 | `admin_grant` | +N | 在统一账号后台用户页操作（`infra apps/web .../MoemoepointModal.vue`）；ref=`admin:<adminId>`，actor=管理员 id，可填 note |
| 管理员扣除 | `admin_deduct` | −N | 同上，delta<0 自动归为 admin_deduct |

kungal 后台**无此能力**（本地不存账号）；本仓 admin 用户页已加「去统一账号后台」入口。

---

## 2. 萌萌点对用户的限制 / 门槛

> 权威以**服务端**为准；前端检查只是体验提示。下表的「服务端」列是真正的闸门。

| # | 限制 | 公式 / 阈值 | 服务端位置 | 管理员豁免 |
|---|---|---|---|---|
| 1 | **每日发帖数上限** | `萌萌点/10 + 1` 个/天 | `topic_write_service.go:95` | 否 |
| 2 | **每日 galgame 提交数上限** | `萌萌点/10 + 1` 个/天（软限：wiki 不可达时放行） | `galgame_service.go:92` | 否 |
| 3 | **付费分区发帖门槛** | 需 `萌萌点 ≥ 10`，发帖扣 **10** | `topic_write_service.go:100` | 否 |
| 4 | **推话题门槛** | 需 `萌萌点 ≥ 20`，推者扣 **20** | `topic_write_service.go:371` | 否 |
| 5 | **每日工具集上传额度** | `100MB + 萌萌点 × 1MB` 字节/天 | `toolset/.../upload_service.go:11`（`if isAdmin { return nil }`） | **是**（仅留单文件 2GB 上限） |
| 6 | **自删回复/评论的余额门槛** | 需余额 ≥ 惩罚额（见 §1.4） | `reply_service.go:328` / `comment_service.go:231` | 部分（版主只扣平价 3） |
| 7 | **删 galgame 资源/评分的余额门槛** | 需余额 ≥ 惩罚额 | `resource_service.go` / `rating_service.go` | 否 |

要点：
- **萌萌点越多 → 每天能发的帖/词条越多、每天能传的文件越大**（线性放大）。0 点也能每天发 1 帖、传 100MB。
- 付费分区与推话题是**硬门槛 + 直接花费**；删除是**惩罚式花费**（且按互动量放大）。
- 改用户名曾经要 17 点（`CostChangeUsername = 17`），但**当前代码未使用**，已不收费。

---

## 3. 常量速查（`apps/api/internal/constants/moemoepoint.go`）

```
RewardCreateTopic/Galgame/Resource/Toolset = 3
RewardReply = 1        RewardPRMerge = 1
CostConsumeSection = 10   CostUpvoteSender = 20   RewardUpvoteOwner = 3
CostChangeUsername = 17   // 定义但未使用
RatingRewardLow/Medium/High = 3 / 5 / 10
RatingLenThresholdMedium/High = 233 / 666
最佳回答 = ±7（字面量，不在常量表）
注册礼物 = 7（在 infra：registerGiftPoints）
每日上传基数 = 100MB（config/upload.ts: USER_DAILY_UPLOAD_LIMIT），每点 +1MB
```

---

## 4. 已知不一致 / 注意事项（排查时留意）

1. ~~付费分区前后端不一致~~ **已修（2026-06-15）**：前端 `TOPIC_SECTION_CONSUME_MOEMOEPOINTS`
   补上 `g-other`，与后端 `TopicSectionConsume` = `{g-seeking, g-other, t-help}` 对齐。
2. ~~推话题前端阈值偏严~~ **已改（2026-06-15）**：推话题门槛/花费**前后端统一为 20**
   （此前服务端 `CostUpvoteSender=7` 当门槛+花费、前端 `< 50` 拦、文案各说各话）。现 `CostUpvoteSender=20`，
   前端 `< 20` 拦，所有文案（确认弹窗 / 不足提示 / 服务端报错）都说 20；被推者奖励仍为 +3。
3. **删除不退原奖励、而是另扣**：审计时不要把「删除扣分」当成「撤销创建奖励」——它是独立的
   `content_removed` 流水，金额规则也不同（资源删 = −(5+赞)，回复/评论删 = −3×互动量）。
4. **like/unlike 等用 nonce 幂等键**：同一内容反复赞/取消会产生多条流水（每次一条 ±1），
   这是有意的（净额正确），不是 bug。
5. **wiki 审核同步用 AwardSync（同步阻塞 cron 游标）**：OAuth 抖动会让该条消息重试；其余所有
   来源都是异步 best-effort，OAuth 挂了也不卡用户操作（萌萌点最终一致，余额回读 OAuth）。
6. 余额显示是缓存：要实时准确（如后台核对）应回读 OAuth，不要信本地 `users.moemoepoint`。
