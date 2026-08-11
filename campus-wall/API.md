# 校园墙 API 文档

基础路径：`/api`。除特别说明外，接口返回 JSON。

## 认证

- 需要登录的接口需在请求头携带：`Authorization: Bearer <token>`
- 登录后 Token 有效期 7 天。
- 错误统一格式：`{ "error": "错误描述" }`，附对应 HTTP 状态码。

---

## 认证 Auth (`/api/auth`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/register` | 注册 `{username, email, password}` | 否 |
| POST | `/login` | 登录 `{username, password}` → `{token, user}` | 否 |
| GET | `/me` | 当前用户信息 | 是 |
| PUT | `/me` | 更新资料 `{username, email, avatar, bio, mood, title}` | 是 |
| GET | `/user/<uid>` | 查看用户信息（含 stats） | 否 |
| POST | `/avatar` | 上传头像（multipart `file`） | 是 |
| POST | `/change-password` | 改密码 `{old_password, new_password}` | 是 |
| POST | `/forgot-password` | 找回密码 `{email}` | 否 |
| POST | `/reset-password` | 重置密码 `{token, new_password}` | 否 |

## 子站 Stations (`/api/stations`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `` | 列表。参数：`limit, offset, sort(newest/popular/posts), tag, category` | 否 |
| GET | `/categories` | 分类层级列表（含 `path/parent/name/depth`） | 否 |
| GET | `/<sid>` | 子站详情（含 `is_member/is_owner`） | 可选 |
| POST | `` | 创建 `{name, description, icon, tags[], category}` | 是 |
| PUT | `/<sid>` | 更新 `{name, description, icon, tags, cover, category, is_public, only_owner_posts, announcement}`（仅站长/管理员） | 是 |
| DELETE | `/<sid>` | 删除子站（仅站长/管理员） | 是 |
| POST | `/<sid>/join` | 加入子站 | 是 |
| POST | `/<sid>/leave` | 退出子站 | 是 |
| GET | `/<sid>/posts` | 子站帖子列表。参数：`limit, offset, sort` | 否 |
| GET | `/<sid>/members` | 成员列表 | 否 |
| DELETE | `/<sid>/members/<uid>` | 移除成员（站长/管理员） | 是 |
| POST | `/<sid>/transfer` | 转让站长 `{new_owner_id}` | 是 |
| GET | `/<sid>/stats` | 子站统计。参数：`days`（默认 7，发帖趋势） | 否 |
| POST | `/upload-cover` | 上传封面（multipart `file`）→ `{url}` | 是 |
| GET | `/search` | 搜索 `?q=keyword` | 否 |
| GET | `/mine` | 我加入的子站 | 是 |

## 帖子 Posts (`/api/posts`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `` | 列表。参数：`limit, offset, sort(newest/popular/comments), station_id, author_id, type(text/link/vote)` | 可选 |
| POST | `` | 创建 `{title, content, station_id, image, post_type, images[], extra, is_anonymous}` | 是 |
| GET | `/liked` | 我赞过的帖子 | 是 |
| GET | `/<pid>` | 帖子详情（自动 +1 浏览量） | 可选 |
| PUT | `/<pid>` | 编辑 `{title, content, image}`（作者/管理员；`is_pinned` 仅管理员） | 是 |
| DELETE | `/<pid>` | 删除（作者/管理员） | 是 |
| GET | `/<pid>/versions` | 编辑历史（作者/管理员） | 是 |
| POST | `/<pid>/like` | 点赞/取消 | 是 |
| GET | `/<pid>/comments` | 评论列表 | 否 |
| POST | `/<pid>/comments` | 发表评论 `{content, parent_id}` | 是 |
| POST | `/upload-image` | 上传图片（multipart `file`）→ `{url}` | 是 |
| DELETE | `/comments/<cid>` | 删除评论 | 是 |

> 帖子列表/详情会针对匿名帖自动隐藏作者身份（作者本人与管理员除外）。

## 社交 Social (`/api/social`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/follow/<uid>` | 关注/取关 | 是 |
| GET | `/followers/<uid>` | 粉丝列表 | 否 |
| GET | `/following/<uid>` | 关注列表 | 否 |
| GET | `/is-following/<uid>` | 是否已关注 | 是 |
| GET | `/notifications` | 通知列表 | 是 |
| POST | `/notifications/read` | 全部已读 | 是 |
| GET | `/notifications/unread-count` | 未读数 | 是 |
| POST | `/comment/<cid>/like` | 评论点赞 | 是 |

## 身份组 Identity (`/api/identity`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/groups` | 身份组列表 | 否 |
| POST | `/groups` | 创建 `{name, icon, color, description, min_level, is_default}`（管理员） | 是/管理员 |
| POST | `/assign` | 分配身份组 `{user_id, group_name}`（管理员） | 是/管理员 |
| GET | `/my` | 我的身份组 | 是 |

## 签到 Checkin (`/api/checkin`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `` | 执行签到 → `{streak, coins_earned, points_earned, total_coins, total_points}` | 是 |
| GET | `/status` | 签到状态 `{streak, coins, points, checked_in_today, history[]}` | 是 |

## 商城 Shop (`/api/shop`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/items` | 商品列表 | 否 |
| GET | `/items/<id>` | 商品详情 | 否 |
| POST | `/buy` | 购买 `{item_id, quantity}` | 是 |
| GET | `/orders` | 我的订单 | 是 |
| GET | `/coins` | 我的金币/积分 | 是 |
| GET | `/transactions` | 金币流水 | 是 |
| POST | `/use` | 使用道具 `{item_id, extra{new_name/post_id/title/count}}` | 是 |

## 恋爱情报 Romance (`/api/romance`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/profiles` | 档案列表 | 否 |
| GET | `/profile` | 我的档案 | 是 |
| POST | `/profile` | 保存/更新档案 | 是 |
| GET | `/profile/<uid>` | 查看他人档案 | 否 |
| POST | `/link` | 发送情报 `{to_user_id, link_type, description, is_anonymous}` | 是 |
| GET | `/links` | 我的情报（`?direction=to/from`） | 是 |
| POST | `/task` | 发布任务 `{title, description, task_type, reward_coins}` | 是 |
| GET | `/tasks` | 任务列表 | 否 |

## 树洞 Gossip (`/api/gossip`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `` | 列表。参数：`limit, offset, sort(newest/hot)` | 否 |
| POST | `` | 匿名发布 `{content, image}` | 是 |
| GET | `/<gid>` | 详情 | 否 |
| POST | `/<gid>/like` | 点赞 | 是 |
| POST | `/<gid>/comment` | 评论 | 是 |

## 交易 Trade (`/api/trade`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `` | 列表。参数：`limit, offset, category, q`（关键词搜索） | 否 |
| POST | `` | 发布 `{title, content, price, original_price, condition, category, contact, image}` | 是 |
| PUT | `/<tid>/status` | 更新状态 `{status}`（available/sold） | 是 |

## 推荐 Recommend (`/api/recommend`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/posts` | 推流推荐（热度+时间+兴趣） | 可选 |
| GET | `/interests` | 兴趣子站推荐 | 是 |
| GET | `/search` | 综合搜索 `?q=` → `{stations, posts, users}` | 可选 |

## 看板娘 Kanban (`/api/kanban`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `/message` | 随机看板娘消息 | 否 |

## 收藏 Favorites (`/api/favorites`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/<type>/<id>` | 收藏/取消（`type`: post/station） | 是 |
| GET | `/status/<type>/<id>` | 收藏状态 | 是 |
| GET | `` | 我的收藏。参数：`type`（post/station） | 是 |

## 举报 Reports (`/api/reports`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `` | 举报 `{target_type(post/comment/user/gossip/trade), target_id, reason, detail}` | 是 |
| GET | `` | 举报列表（管理员）。参数：`status`（pending） | 是/管理员 |
| PUT | `/<rid>` | 处理 `{status: approved/rejected, note}` | 是/管理员 |

## 公告 Announcements (`/api/announcements`)

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| GET | `` | 有效公告列表（前 5 条） | 否 |

### 公告管理（管理员，`/api/admin/announcements`）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `` | 全部公告 |
| POST | `` | 发布 `{title, content}` |
| PUT | `/<aid>` | 更新 `{title, content}` |
| POST | `/<aid>/toggle` | 显隐 `{active}` |
| DELETE | `/<aid>` | 删除 |

## 管理后台 Admin (`/api/admin`)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/stats` | 全站统计 |
| GET | `/stats/series` | 增长趋势。参数：`days`(7/14/30) → `{labels, users, posts, comments}` |
| GET | `/users` | 用户列表 |
| PUT | `/users/<uid>` | 更新用户 `{role, coins, points, level, identity_group}` |
| GET | `/stations` | 子站列表 |
| GET | `/posts` | 帖子列表 |
| POST | `/posts/<pid>/pin` | 置顶/取消 |
| DELETE | `/posts/<pid>` | 删除帖子 |
| GET | `/logs` | 操作日志 |
| POST | `/shop/items` | 创建商品 |
| GET | `/identity-groups` | 身份组列表 |

> 所有 `/api/admin/*` 接口需管理员权限。

---

## 分页约定

列表接口一般支持 `limit`（默认 20~50）与 `offset`（默认 0）参数，返回 `{posts: [...]}` 或直接数组（子站/交易等），请以实际响应为准。

## 时间格式

`created_at` 等字段为 ISO 时间字符串（`YYYY-MM-DD HH:MM:SS`），前端用本地时间解析展示。
