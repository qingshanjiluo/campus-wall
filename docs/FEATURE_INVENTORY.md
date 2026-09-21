# CampusWall 校园墙 · 全功能与链路清单

> 生成方式：直接读代码/DB 核对（非凭记忆）。范围＝生产主线 Flask（`campus-wall/app`）+ 静态前端（`campus-wall/frontend`）+ 部署（`campus-wall/deploy`）。
> 权威口径：20 个蓝图 / 122 个 HTTP 端点 / 29 张数据表 / 25 个前端页面。
> 鉴权标记：**[A]**=需登录 token_required ｜ **[O]**=可选登录 optional_auth（影响脱敏/点赞态） ｜ **[Ad]**=管理员 ｜ 无标记=公开。

---

## 0. 架构链路（一次请求的完整路径）

```
浏览器 ──HTTPS──▶ Nginx :443/80
   ├─ 页面路由 (/, /waterfall, /world …)  ─▶ Flask pages_bp（静态 25 页，no-cache + ETag）
   ├─ /static/*  (js/css/img/vendor)      ─▶ Flask 静态（Nginx 1h proxy_cache）
   ├─ /static/uploads/*                   ─▶ 磁盘直出（/srv/uploads，30d immutable）
   └─ /api/*  ──proxy_pass(X-Real-IP)──▶ Gunicorn 4w×2t :8000 ─▶ Flask app factory
                                                   ├─ JWT 鉴权 / 限流 / 敏感词
                                                   └─ raw sqlite3（WAL，DATABASE_PATH 可指任意盘 / 单点可换 PG）
前端 JS 链（按 <script> 顺序，彼此用 window.* 桥接）：
   api → utils → icons → auth → modal → common → animations → app
   api.js=fetch 封装+JWT ｜ utils=escHtml/safeUrl/showToast/formatTime ｜ icons=lucide 自托管+emoji 映射
   auth=登录态/导航/弹窗入口 ｜ modal=编辑资料/改密/举报等模态 ｜ common=注入壳层(导航/页脚/广告位)+主题
   animations=进场动效 ｜ app=帖子卡片/瀑布流/投票/分享等全站业务逻辑
```

---

## 1. 功能模块总览

| 域 | 模块 | 入口页面 | 主要蓝图 |
|---|---|---|---|
| 账户 | 注册/登录/资料/改密/找回 | 弹窗 + `/reset-password` | auth |
| 内容 | 帖子(text/image/vote/link/**expose**) | `/`, `/waterfall`, `/post/<id>` | posts |
| 社区 | 子站(版块)/成员/私密站/转让 | `/station/<id>`, `/create-station` | stations |
| 互动 | 点赞/收藏/关注/评论/通知 | 卡片、`/favorites`、`/notifications`、`/profile` | posts, social, favorites |
| 私信 | 一对一聊天 + 会话列表 + 未读 | `/messages` | dm |
| 经济 | 签到/金币/积分/商店/道具/流水 | `/checkin`, `/shop` | checkin, shop |
| 匹配 | 恋爱档案/表白连线/心愿任务 | `/romance` | romance |
| 匿名 | 树洞爆料(轻) | `/gossip` | gossip |
| **爆料台** | **恒匿名+先审后发(重)** | **`/expose`** | posts(post_type=expose) |
| **角色关系图** | **多人共建可视化图谱** | **`/world`** | world |
| **公开论坛** | **版块矩阵+全站热议** | **`/forum`** | stations, posts |
| 分发 | 推荐流/兴趣子站/智能搜索 | `/waterfall`(热), `/search` | recommend |
| 治理 | 敏感词三级 + 人工审核 + 举报 | `/admin` | posts, admin, reports |
| 后台 | 统计/用户/子站/帖子/身份组/商城/公告/广告位/日志 | `/admin` | admin |
| 站点 | 公告/看板娘/**广告位配置** | 注入于所有页 | announcements, kanban, site |
| 体验 | 深色模式/响应式/动效/懒加载 | 全站 | common, animations, icons |

---

## 2. 端到端链路（关键用户旅程）

### 2.1 注册 / 登录
`create.html/弹窗` → `POST /api/auth/register`(bcrypt 存哈希、用户名≥2/密码≥6 校验、重名 409) → 自动 `POST /api/auth/login` 发 JWT → 前端 `localStorage.token` → `api.js` 后续带 `Authorization: Bearer` → 导航 `updateNavRight()` 渲染头像/用户名/下拉。找回密码：`POST /api/auth/forgot-password`（**生产无 SMTP 返回 503**；开发态回显一次性 token）→ `/reset-password?token=` → `POST /api/auth/reset-password`（token 校验+改密+旧密码失效）。

### 2.2 发帖（含审核闸）
`/create` → 选子站/类型(vote 需≥2 选项、link 需合法 URL) → 可选多图上传(`POST /api/posts/upload-image` ×N，≤9 张) → `POST /api/posts` → 后端 `scan_text` 敏感词：命中 **block→400 拒** / **review→status=pending 进队列** / 否则 approved → 作者见「审核中」徽标（`GET /api/posts?author_id=me` 放开自己 pending/rejected），公开流（列表/详情/搜索/推荐）**只出 approved**；`pending/rejected` 对他人详情 404。编辑 `PUT /api/posts/<id>` 会**重扫**内容（可被重新打回 pending）。

### 2.3 投票 / 点赞 / 收藏 / 评论
- 投票：`POST /api/posts/<id>/vote {option_index}`，一人一票，`extra.counts/voters` 记账，重投 400；`shape_post` 展开 `vote_options/vote_counts/user_voted`。
- 点赞：`POST /api/posts/<id>/like` 返回 `{liked, likes_count}`（服务端真计数，前端不再猜）。评论点赞 `POST /api/social/comment/<cid>/like`。
- 收藏：`POST /api/favorites/<type>/<id>`（post/station 多态）→ `/favorites` 页。
- 评论：`GET/POST /api/posts/<id>/comments`（支持 `parent_id` 楼中楼）。
- 关注：`POST /api/social/follow/<uid>` + followers/following。

### 2.4 私信
`/profile/<id>` 或 `/messages?with=uid` → `POST /api/dm {to_user_id,content}`（敏感词 block 拒、不能发自己）→ 写 `dm_messages` + 触发通知 → `GET /api/dm/threads`(会话+未读数+最后一条) → `GET /api/dm/<peer>`(自动置已读) → `GET /api/dm/unread`(导航徽标)。双栏/移动单栏，8s 轮询。

### 2.5 签到 → 金币 → 商店 → 道具
`POST /api/checkin`（当日唯一，`do_checkin` 按 `10+min(streak-1,7)*5` 发币、写 `coin_transactions` 流水）→ `/shop` `GET /api/shop/coins` 余额、`GET /api/shop/items` 商品 → `POST /api/shop/buy`（扣币+生成 `shop_orders`）→ `POST /api/shop/use`（改名卡/置顶卡/匿名卡/称号/彩虹昵称，改 `users`/发通知）→ `GET /api/shop/transactions` 金币明细（收支流水）。

### 2.6 恋爱匹配
`/romance` `GET/POST /api/romance/profile`（建/看自己档案）→ `GET /api/romance/profiles`（异性列表，含 `match_score`）→ `GET /api/romance/profile/<uid>`（看他人）→ `POST /api/romance/link`（表白连线，可匿名，通知对方）→ `GET /api/romance/links?direction=to|from` → `POST/GET /api/romance/task`（恋爱心愿任务）。

### 2.7 角色关系图（多人共建，R6）
`/world` `GET /api/world/graph`（一次拉全部 active 节点+关系，公开可读）→ 力导向 SVG 渲染（拖拽/点选高亮/单向箭头双向虚线/搜索高亮/点空白取消）→ 登录用户 `POST /api/world/nodes`（角色名≤30、头像 emoji、人设、颜色）、`POST /api/world/relations`（from/to/label≤20/双向；自反关系 400）→ `PUT/DELETE /api/world/nodes/<id>`（**属主或管理员**，否则 403；删节点级联删关系）、`DELETE /api/world/relations/<id>`（属主/管理员）。多用户各自新增、实时汇入同一张共享图谱（"共同写入串联维护"）。

### 2.8 爆料台（恒匿名+先审后发，R6）
`/expose` `POST /api/posts {post_type:'expose', content, images[]}` → 后端**强制 is_anonymous=1 且 status=pending**（无子站时落首个公开子站；敏感词 block 仍拒）→ `GET /api/admin/review` 管理员见队列 → `POST /api/admin/review {action:'approve'}` 发布 → 公开流 `GET /api/posts?post_type=expose`；`shape_post` 对 expose **无条件**把作者名 mask 成「匿名同学」（**连作者本人/管理员都看不到举报人**，DB 仍留 author_id 供追责）→ 作者侧栏 `loadMyPending` 看自己待审。

### 2.9 公开论坛（R6）
`/forum` `GET /api/stations?limit=100` 版块矩阵（过滤私密站，含帖数/简介）+ `GET /api/posts?sort=newest|popular` 全站最新/最热热议（分页「加载更多」），点击回落 `/station/<id>` 与 `/post/<id>`。

### 2.10 推荐 / 兴趣 / 搜索
- 推荐流：`GET /api/recommend/posts`（热度+时间+`is_pinned`，仅 approved，支持 `station_id` 过滤——瀑布流"热门+我关注的子站"用）。
- 兴趣站：`GET /api/recommend/interests`（按用户发帖/点赞历史算 Top 子站）→ 瀑布流顶部快捷筛选栏。
- 智能搜索：`GET /api/recommend/search?q=`（全站帖+子站聚合）+ 独立 `GET /api/stations/search`；`/search` 页带历史（点击走 `data-h` 防注入）。

### 2.11 审核 / 举报 / 后台
`/admin`（`admin_required`）：仪表盘统计+趋势(`GET /api/admin/stats`、`/stats/series`)、用户管理(`PUT /api/admin/users/<uid>` 改身份组/角色/封禁)、子站、帖子、**帖子审核队列**(`GET/POST /api/admin/review` approve/reject)、身份组、商城上架、公告 CRUD、**广告位配置**(`PUT /api/admin/site/config`)、举报中心(`GET /api/reports`、`PUT /api/reports/<rid>` 处理)、操作日志(`admin_log`)。举报：用户侧 `POST /api/reports`。

### 2.12 站点配置 / 广告位 / 公告 / 看板娘
- 公告：`GET /api/announcements`（active）→ 前端公告条。
- 看板娘：`GET /api/kanban/message`（文案轮播气泡）。
- **广告位**：`GET /api/site/config`（`ad_enabled`/`ad_header`/`ad_footer`）→ `common.js.initAdSlots()` 在导航下/页脚上注入 `.ad-slot`（可开关/改文案，暗色适配）。

### 2.13 上传链路（帖子图/头像/封面共用底座）
`POST /api/posts/upload-image`（multipart，`file`）→ **PIL verify 防伪扩展名 + EXIF 剥离 + 长边压 1600px + webp 输出** → 落 `UPLOAD_FOLDER` → 返回 `/static/uploads/...` url → Nginx 磁盘直出。头像 `POST /api/auth/avatar`、封面 `POST /api/stations/upload-cover` 同底座。

---

## 3. 全端点清单（按蓝图）

### auth（/api/auth）
`POST /register` ｜ `POST /login` ｜ `GET /me`[A] ｜ `PUT /me`[A] ｜ `GET /user/<uid>` ｜ `POST /avatar`[A] ｜ `POST /change-password`[A] ｜ `POST /forgot-password` ｜ `POST /reset-password`

### stations（/api/stations）
`GET ''`[O] ｜ `GET /categories` ｜ `GET /<sid>` ｜ `POST ''`[A] ｜ `POST /<sid>/join`[A] ｜ `POST /<sid>/leave`[A] ｜ `GET /<sid>/posts`[O] ｜ `GET /by/<uid>` ｜ `GET /search` ｜ `GET /mine`[A] ｜ `POST /upload-cover`[A] ｜ `GET /<sid>/stats` ｜ `GET /<sid>/members` ｜ `PUT /<sid>`[A,属主/管] ｜ `DELETE /<sid>`[A,属主/管] ｜ `DELETE /<sid>/members/<uid>`[A,属主/管] ｜ `POST /<sid>/transfer`[A,属主]

### posts（/api/posts）
`GET /liked`[A] ｜ `GET ''`[O] ｜ `GET /<pid>/versions`[A] ｜ `GET /<pid>`[O] ｜ `POST ''`[A] ｜ `PUT /<pid>`[A,属主] ｜ `DELETE /<pid>`[A,属主/管] ｜ `POST /<pid>/like`[A] ｜ `POST /<pid>/vote`[A] ｜ `GET /<pid>/comments` ｜ `POST /<pid>/comments`[A] ｜ `POST /upload-image`[A] ｜ `DELETE /comments/<cid>`[A]

### social（/api/social）
`POST /follow/<uid>`[A] ｜ `GET /followers/<uid>` ｜ `GET /following/<uid>` ｜ `GET /is-following/<uid>`[A] ｜ `GET /notifications`[A] ｜ `POST /notifications/read`[A] ｜ `GET /notifications/unread-count`[A] ｜ `POST /comment/<cid>/like`[A]

### dm（/api/dm）
`POST ''`[A] ｜ `GET /threads`[A] ｜ `GET /unread`[A] ｜ `GET /<peer_id>`[A]

### identity（/api/identity）
`GET /groups` ｜ `POST /groups`[Ad] ｜ `POST /assign`[Ad] ｜ `GET /my`[A]

### checkin（/api/checkin）
`POST ''`[A] ｜ `GET /status`[A]

### shop（/api/shop）
`GET /items` ｜ `GET /items/<id>` ｜ `POST /buy`[A] ｜ `GET /orders`[A] ｜ `GET /coins`[A] ｜ `GET /transactions`[A] ｜ `POST /use`[A]

### romance（/api/romance）
`GET /profiles` ｜ `GET /profile`[A] ｜ `POST /profile`[A] ｜ `GET /profile/<uid>` ｜ `POST /link`[A] ｜ `GET /links`[A] ｜ `POST /task`[A] ｜ `GET /tasks`[A]

### gossip（/api/gossip）
`GET ''`[O] ｜ `POST ''`[A] ｜ `GET /<gid>` ｜ `POST /<gid>/like`[A] ｜ `POST /<gid>/comment`[A]

### trade（/api/trade）
`GET ''`[O] ｜ `POST ''`[A] ｜ `PUT /<tid>/status`[A,属主/管]

### kanban（/api/kanban）
`GET /message`

### recommend（/api/recommend）
`GET /posts`[O] ｜ `GET /interests`[A] ｜ `GET /search`[O]

### favorites（/api/favorites）
`POST /<type>/<id>`[A] ｜ `GET /status/<type>/<id>`[A] ｜ `GET ''`[A]

### reports（/api/reports）
`POST ''`[A] ｜ `GET ''`[Ad] ｜ `PUT /<rid>`[Ad]

### announcements（/api/announcements）
`GET ''`

### world（/api/world，R6）
`GET /graph`[O] ｜ `GET /nodes/<nid>` ｜ `POST /nodes`[A] ｜ `PUT /nodes/<nid>`[A,属主/管] ｜ `DELETE /nodes/<nid>`[A,属主/管] ｜ `POST /relations`[A] ｜ `DELETE /relations/<rid>`[A,属主/管]

### site（/api/site，R6）
`GET /config` ｜ `PUT /api/admin/site/config`[Ad]

### admin（/api/admin）
`GET /stats` ｜ `GET /stats/series` ｜ `GET /users` ｜ `PUT /users/<uid>` ｜ `GET /stations` ｜ `GET /posts` ｜ `POST /posts/<pid>/pin` ｜ `DELETE /posts/<pid>` ｜ `GET /logs` ｜ `POST /shop/items` ｜ `GET /identity-groups` ｜ `GET /announcements` ｜ `POST /announcements` ｜ `PUT /announcements/<aid>` ｜ `POST /announcements/<aid>/toggle` ｜ `DELETE /announcements/<aid>` ｜ `GET /review` ｜ `POST /review` ｜ `PUT /site/config`（均 [Ad]）

### pages（Flask 页面矩阵）
`/`（index）｜ `/post/<id>` `/station/<id>` `/profile/<id>`（前缀详情）｜ 干净路由见 §5 页面表 ｜ 未知路径 → `404.html`(HTTP 404)

---

## 4. 数据表（29，按域）

| 域 | 表 | 作用 |
|---|---|---|
| 用户 | `users`(+扩展列 coins/points/level/exp/checkin_streak/last_checkin/identity_group/title/mood) | 账户+经济+身份 |
| | `identity_groups` | 身份组（管理员可授徽章） |
| 内容 | `posts` | 帖子（`post_type` text/image/link/vote/**expose**，`images` JSON、`extra` JSON、`status` 审核态） |
| | `comments` / `post_versions` | 评论（楼中楼）/ 编辑历史 |
| | `likes` | 多态点赞（post/comment/gossip…） |
| | `favorites` | 多态收藏（post/station） |
| 社区 | `stations` / `station_members` | 子站（含私密/仅站长发帖）/ 成员角色 |
| | `follows` | 用户关注 |
| 分发 | （无独立表，算于 posts/likes/stations） | 推荐/兴趣/搜索 |
| 私信 | `dm_messages` | 私信（双索引 sender/receiver+未读） |
| 经济 | `checkins` | 签到记录（连续天数） |
| | `coin_transactions` | 金币流水（income/spend，明细页） |
| | `shop_items` / `shop_orders` | 商城商品 / 订单（道具） |
| 匹配 | `romance_profiles` / `romance_links` / `romance_tasks` | 恋爱档案 / 表白连线 / 心愿任务 |
| 匿名 | `gossip` / `gossip_comments` | 树洞（轻匿名爆料）/ 评论 |
| 交易 | `trade_posts` | 二手商品（关联 post，status 可售/预定/已售） |
| **关系图** | `character_nodes` / `character_relations` | 角色节点 / 关系边（多人共建，R6） |
| 治理 | `reports` | 举报 |
| | `admin_log` | 管理操作审计 |
| 站点 | `site_announcements` | 公告 |
| | `site_config` | 键值配置（广告位开关/文案，R6） |
| | `kanban_messages` | 看板娘文案 |
| | `notifications` | 通知（评论/点赞/关注/系统/私信/连线…） |
| | `user_settings` | 通知偏好/主题 |

---

## 5. 前端页面（25）

| 路由 | 文件 | 说明 |
|---|---|---|
| `/` | index | 首页：子站矩阵 + 公告 + 最新 |
| `/waterfall` | waterfall | 瀑布流（最新/热门 + 兴趣子站筛选） |
| `/forum` | forum | 公开论坛（版块矩阵 + 全站热议排序/分页）R6 |
| `/world` | world | 角色关系图（力导向可视化，多人共建）R6 |
| `/expose` | expose | 爆料台（恒匿名 + 先审后发 + 配图）R6 |
| `/trade` | trade | 二手市场（属主可改状态） |
| `/romance` | romance | 恋爱情报（档案/连线/任务） |
| `/gossip` | gossip | 树洞（轻匿名） |
| `/shop` | shop | 积分商城 + 金币明细 |
| `/checkin` | checkin | 签到（连续加成） |
| `/search` | search | 智能搜索 + 历史 |
| `/favorites` | favorites | 我的收藏 |
| `/messages` | messages | 私信（双栏/未读/深链） |
| `/notifications` | notifications | 通知中心 |
| `/create` | create | 发帖（含 vote/link/多图） |
| `/create-station` | create_station | 创建子站 |
| `/station/<id>` | station | 子站详情（成员/加入/设置/转让） |
| `/post/<id>` | post | 帖子详情（投票/多图/评论/分享/举报） |
| `/profile/<id>` | profile | 用户主页（资料/子站/关注/私信按钮） |
| `/admin` | admin | 管理后台（13 Tab） |
| `/reset-password` | reset_password | 重置密码 |
| `/about` `/terms` `/privacy` | 同名 | 关于/协议/隐私 |
| `/404` | 404 | 未找到页 |

导航栏（`common.js`，≤1024px 折叠汉堡）：首页 · 瀑布流 · 论坛 · 角色图 · 爆料 · 交易 · 恋爱 · 树洞 · 商城 · 搜索；右侧：主题切换 + 登录/注册 或 头像下拉（后台/发帖/退出）。

---

## 6. 安全 / 治理链路

- **JWT + bcrypt**：生产强制 `JWT_SECRET`（缺失拒启动）。
- **接口限流**（`RATELIMIT_ENABLED=1` 生产开，内存 storage）：register 8/h、login 12/min、发帖 20/min、上传 10–15/min、私信 30/min、树洞/交易 10/min；429 统一中文 JSON。
- **敏感词三级**（内置 50+，`SENSITIVE_WORDS_FILE` 热扩展）：block/review/approved，作用于发帖/编辑/评论/私信/树洞。
- **审核闸**：`posts.status` approved/pending/rejected；对外只放 approved，作者见自己三态，管理员全量；零迁移 ALTER + `idx_posts_status`。
- **匿名脱敏**：`shape_post` 在列表/详情/搜索/推荐全通道 mask 匿名作者；expose **恒匿名含管理员**。
- **越权防护**：帖子/子站/道具/交易状态/**角色图节点与关系**均校验属主/管理员（403）。
- **上传安全**：PIL 内容校验防伪扩展名 + EXIF 剥离 + 尺寸压缩。
- **前端注入防护**：全站 `escHtml/safeUrl`；`onclick` 走 `data-*` 属性传值（搜索历史）；图标属性转义。
- **CORS** 经 env 收紧。

---

## 7. 部署 / 运维链路

```
cp .env.example .env（填 JWT_SECRET/SECRET_KEY/CORS/SMTP/域名）
→ docker compose up -d --build （web: gunicorn + nginx + certbot 续期 + 每日备份 sidecar）
→ 健康检查 app 驱动 nginx 依赖；缺密钥 compose 直接拒启
备份：scripts/backup.py（sqlite3 在线一致快照 + uploads tar + 滚动 7 份）
发布：scripts/deploy.sh（拉码→build→滚动重启→冒烟）
无 Docker 备选：systemd/campuswall.service（ProtectSystem/PrivateTmp/降权）
验证：tools/e2e_live.ps1 -BaseUrl <域名> ；tools/js_gate.py（语法+ASCII+壳层守卫）
CI：.github/workflows/backend-ci.yml（干净 ubuntu 冷启生产 → 68 步 E2E → 路由矩阵 → JS 门）
```

---

## 8. 当前状态

- 质量门：live E2E **83 步全绿**；`js_gate`（外链+内联 `node --check` + PS ASCII + 壳层结构守卫 + 跨脚本引用中毒扫描）全绿。
- 已修高危：R5-B 引入的**全站登录失效**（跨脚本裸引用 ReferenceError）已修并加门禁（`ee6befe`）。
- 剩余（外部人工，非代码）：买服务器/域名 → `docker compose up` → 对线上域名跑 83 步 E2E → 改演示 `admin/admin123` 密码 → 浏览器双主题走查。
- 演示账号：`admin/admin123`（管理员）、`xiaohua/123456`（用户）；口令哈希漂移已按 seed 修复。

---

## 9. R8-R13 扩展模块（持续开发波次交付）

| 波次 | 模块 | 入口/端点 | 关键表 |
|---|---|---|---|
| R8-1 | **话题/标签 + 热搜榜**（发帖显式+`#`内联识别，归一化≤5个；近7天热度榜；话题聚合过滤） | `/search?topic=` ｜ `GET /api/topics/trending`、`GET /api/topics/<名>/posts`、admin topics CRUD | `topics` `post_topics` |
| R8-2 | **任务系统**：系统任务（自动发奖）/ 官方任务 / 积分悬赏（发布预扣-提交-审核-放款），名额与防重领 | `/tasks` ｜ `/api/tasks/*` + admin tasks/close/claims | `tasks` `task_claims` |
| R8-3 | **举报增强**：七分类枚举 + 证据图（≤4 张仅本站 uploads）+ admin 分类筛选/缩略图 | `POST/GET /api/reports` | `reports(+evidence)` |
| R8-4 | **数据导出**：用户 JSON 附件 + admin 用户/帖子 CSV（UTF-8 BOM） | `GET /api/auth/export`、`GET /api/admin/export/*.csv` | 只读聚合 |
| R9-1 | **多主题皮肤**：glass/galgame/minimal/cyberpunk × 明暗；导航调色板选择器；localStorage + 服务端双持久化 | `GET/PUT /api/user/settings` | `user_settings(+skin)` |
| R9-2 | **访客模式**：closed 时匿名访问内容流一律 401，admin 可配 | `site_config.visitor_mode`、`PUT /api/admin/site/config` | `site_config` |
| R10 | **聊天室**：多房间（公开/私有）/成员/@提及解析+通知/本人撤回软删/`after_id` 增量轮询 | `/chat` ｜ `/api/chat/*` | `chat_rooms` `chat_members` `chat_messages` |
| R11 | **活动系统**：发布（日期/名额/打卡金币）/报名（容量闸）/取消/到场打卡发奖 | `/events` ｜ `/api/events/*` + admin close | `events` `event_registrations` |
| R11 | **暗阁·悬赏问答**：提问预扣积分 → 他人回答 → 提问者采纳放款（或关闭退回）；被采纳者收通知 | `/qna` ｜ `/api/qna/*`（questions/answers/accept/close） | `bounty_questions` `bounty_answers` |
| R11 | **暗阁·付费可见**：pay_points 0-999，未购者全通道 30 字摘要；解锁买家扣分、作者实时收款 | `POST /api/posts/<id>/unlock` ｜ shape_post 统一锁 | `content_unlocks` `posts(+pay_points)` |
| R11 | **暗阁·积分推流**：作者 10 积分/天（1-7 天）推荐流置前（boosted 优先于热度） | `POST /api/posts/<id>/boost` | `posts(+boosted_until)` |
| R12 | **等级自动升级**：发帖+10/评论+3/被赞+2/签到+5 经验；规则 admin 可配；升级发金币 + 身份组自动授予 | `GET/PUT /api/admin/level-rules` | `level_rules` `identity_groups(+auto_assign,is_public)` |
| R12 | **AI 生态 v1**：可插拔适配器（规则引擎默认 + OpenAI 兼容可选）；AI 管理员（待审建议/一键应用/自动模式）；AI 用户机器人；AI 接管私信代回（50 积分/次开启） | `/api/admin/ai/*`、`PUT /api/user/settings{ai_reply}`、`app/utils/ai.py` | `level_rules` |
| R13 | **站点定制注入**：管理员 CSS/HTML/JS 三键（各≤20KB），前端隔离注入 | `GET /api/site/custom`、`PUT /api/admin/site/custom` | `site_config` |
| R13 | **插件钩子雏形**：注册表 + 4 挂载点（post_created/comment_created/user_registered/chat_message_sent），异常全隔离；内置 welcome_plugin；admin 启停 | `GET /api/admin/plugins`、`POST /api/admin/plugins/<名>/toggle` | `site_config.disabled_plugins` |

多主题皮肤选择器在导航调色板按钮；admin「广告位」Tab 已扩展为 广告 + 访客模式 + 站点定制 三区。
多站架构（独立域名/独立用户体系）按计划冻结，未实现。
