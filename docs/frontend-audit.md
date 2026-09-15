# 校园墙 CampusWall · 前端 ↔ Worker 后端联调审计报告

- 审计范围：`campus-wall/frontend/pages/*.html`（21 个页面）+ `campus-wall/frontend/static/js/*.js`（8 个脚本）+ `frontend/index.html` + `_redirects` / `_headers` / `functions/`
- 后端证据：`backend-worker/src/routes_auth.py / routes_stations.py / routes_posts.py / routes_social.py / routes_extended.py / entry.py / models.py / models_ext.py / uploads.py`
- 方法：扫描全部 ~90 处 `fetch / api.get/post/put/delete` 调用点，逐条与 `entry.Router` 注册的正则路由表比对；请求体字段与 handler 中 `data.get(...)` 比对；响应消费字段与 handler `jsonify(...)` 比对；页面 `<link>/<script>` 引用与 onclick 全局函数交叉核对；上传 URL 回传链路核对；innerHTML 拼接转义抽查。
- 结论：**未发现调用了不存在路由的"拼写错误"型 404，唯一例外是 `/api/posts/{pid}/vote`（不存在）**；但存在 1 个响应结构错配导致整页功能失效的 P0、多处对后端字段的幻觉引用、以及 3 类**存储型 XSS**属性注入口。线上主链路（登录/发帖/子站/帖子/评论/点赞/收藏/举报/签到/商城/树洞/交易/恋爱/搜索/管理后台）路由与字段绝大多数已核对通过。

---

## P0 · 阻塞上线

### P0-1 瀑布流页"最新"Tab 永远显示"暂无动态"（响应结构错配）
- 位置：`campus-wall/frontend/pages/waterfall.html:132-146`
- 问题：`newest` 分支请求 `/api/posts?limit=…&sort=newest` 后把响应**当数组直接消费**（`posts.length`、`posts.map`）。
- 后端证据：`routes_posts.py:76` `list_posts` 返回 `jsonify({'posts': posts, 'total': total})`（对象，非数组）；对比 `recommended_posts`（`routes_extended.py:427-433`）返回裸数组。
- 后果：默认视图 `posts.length === undefined → 判定为空 → 渲染"暂无动态"；点"热门"再切回或上拉加载时 `posts.map` 抛 `TypeError`（未处理 Promise 异常）。瀑布流为导航栏一级页面，等同整页瘫痪。
- 修复：与 index.html 一致改为 `data.posts`：`const list = Array.isArray(data) ? data : (data.posts || [])`（两个分支分别归一化即可）。

### P0-2 投票/链接帖：调用了不存在的 `/api/posts/{pid}/vote`，且 vote/link 字段后端整体丢弃
- 位置（三条链一起失效）：
  - `static/js/app.js:581-587` `submitVote()` → `POST /api/posts/{pid}/vote` —— **路由表中不存在**（`entry.py:54-60` 注册的全部 `ROUTES` 中无 `/vote`，将返回 404 `接口不存在`）。
  - `static/js/app.js:101-107` `handleCreatePost()` 发送 `link_url`、`vote_options` —— `routes_posts.py:110-150` 的 `create` 只读取 `title/content/station_id/image/images/is_anonymous/post_type`，两字段被静默丢弃（不落库、不报错）。
  - `pages/post.html:141` 与 `static/js/app.js:547-579` 渲染投票区读取 `p.vote_options / p.vote_counts / p.user_voted` —— 后端帖子行（`models.py:252-258` `POST_SELECT`）根本没有这些字段，投票 UI 永不出现。
- 后果：发布"链接/投票"类型帖是完整的假功能（用户填了链接和选项，帖子只剩正文）；`submitVote` 一旦可达必 404。
- 修复（二选一）：
  1. 后端补齐：`create/update` 将 `link_url/vote_options` 序列化进 `posts.extra`，`get_post/get_posts` 解析 `extra` 返回；新增 `POST /api/posts/<pid>/vote` + `post_votes` 表；
  2. 前端下线：暂时隐藏 `post_type=link/vote` 选项与 `renderVoteSection/submitVote`，避免用户踩坑。上线前至少做 2。

---

## P1 · 高优（显著功能损坏 / 安全，强烈建议上线前修）

### P1-1 全站登录/注册/发帖弹窗的提示 Toast 不可见（CampusUtils.showToast 与 CSS 冲突）
- 位置：`static/js/utils.js:14-35`（`CampusUtils.showToast` 实现）；`static/css/style.css:860-876`（`.toast` 基础样式 `opacity:0; transform:translateY(100px)`，仅 `.toast.show` 才可见）。
- 问题：utils 版用 `toast.style.cssText = ...` 覆盖内联样式，但**没有设置 `opacity`/`transform`，也不加 `.show` 类**；其引用的 `animation: toast-in/toast-out` 关键帧在 style.css 中**不存在**（全文 0 处）。
- 影响面：`auth.js:42/54/73/77/92/96/105`（登录失败/注册失败/退出提示）、`app.js:109/114/117/160/163/166`（发帖/建站校验与结果）等——**失败时用户完全无反馈**，以为按钮失灵。
- 证据：style.css 只有 `.toast` / `.toast.show`（860/873 行）；`@keyframes toast-in` 检索 0 命中。app.js 自己的全局 `showToast`（536-544 行，用 `.show` 类）是好的，但 `CampusUtils.showToast` 是闭包引用 utils 版本，二者不一致。
- 修复：把 `CampusUtils.showToast` 改为 `el.classList.add('show')` 方案（与 app.js 统一），或补 `toast-in/toast-out` 关键帧并在内联样式里写 `opacity:1;transform:translateX(-50%) translateY(0)`。

### P1-2 存储型 XSS（属性注入）：`station.icon/cover`、任意用户的 `author_avatar`、trade 的 `price/original_price/condition` 未转义直插 HTML
- 证据链（后端均为任意字符串可写）：
  - `PUT /api/stations/<sid>`（`routes_stations.py:197-211` + `models.py:212-219`）允许 `icon/cover` 任意字符串，任何注册用户可建 own station 后注入；
  - `PUT /api/auth/me`（`routes_auth.py:106` `update_me`）允许 `avatar` 任意字符串；
  - `POST /api/trade`（`routes_extended.py:369-395`）`price/original_price/condition` 直接取 body 不校验数值/枚举，SQLite REAL 亲和性对非数字文本原样存 TEXT。
- 未转义注入点（拼接进属性值，`"` 即可逃逸）：
  - `pages/index.html:261` `style="background-image:url('...s.cover...')"`、`:263` `data-lucide="${s.icon || 'school'}"`
  - `pages/search.html:82` `(s.icon || 'school')`、`:102` `(p.station_icon || 'school')`、`:97/116` `author_avatar`/`u.avatar`
  - `static/js/app.js:491` renderPostCard `src="${p.author_avatar || ...}"` —— **每个列表页都渲染**
  - `pages/post.html:91/131/186`（推荐位/正文/评论的 `author_avatar`）
  - `pages/station.html:271`（成员列表 `m.avatar`）
  - `pages/profile.html:88/242`、`pages/romance.html:72`（`avatar`）
  - `pages/trade.html:123/124/129`（`¥${t.price}`、`¥${t.original_price}`、`${condMap[t.condition] || t.condition}`）
- 对照：同文件里对 `name/title/username/description` 都走了 `escHtml`，唯独这几类漏了。
- 修复：所有上述插值一律 `escHtml(...)`；trade 的 price 再叠一层 `Number(x)||0`；并建议后端在 `update_me/update_station/create_trade` 处加白名单校验（avatar/cover 以 `/static/uploads/` 或 http(s) 前缀强校验；condition ∈ 枚举；price 强制 float）。

### P1-3 `document.querySelector('.modal-overlay').remove()` 误删全局登录框，导致本会话登录弹窗永久失效
- 位置：`pages/station.html:212、222`（子站设置保存/转让成功后）；`pages/admin.html:342、419`（公告/商品创建成功后）；`pages/gossip.html:163`（树洞评论成功后）。
- 问题：动态弹窗是用 `document.body.appendChild` 追加的，而 `common.js:70-343` 早已在文档前部注入了 8 个隐藏的 `.modal-overlay`（`#loginModal` 等）。`querySelector('.modal-overlay')` 命中的是**第一个静态弹窗（loginModal）**：它被 remove，动态弹窗却不会关闭。`getElementById('loginModal')` 此后返回 null，`openModal('loginModal')` 静默失败——需要登录的入口全部"点了没反应"，只能刷新。
- 修复：统一用 `modal.remove()` / `closeModal(id)`；这三处应改为保存 `const modal = document.createElement(...)` 引用自身。

### P1-4 管理后台仪表盘"今日帖子/今日注册"显示 `undefined`
- 位置：`pages/admin.html:106-107` 读取 `s.today_posts`、`s.today_users`。
- 后端证据：`models_ext.py:454-462` `get_admin_stats()` 仅返回 `{users, stations, posts, comments, trend, coins}`。
- 修复：前端改用 `s.trend` 最后一格（`get_admin_stats_series` 已含逐日 users/posts/comments，取 `labels.length-1` 即今日），或后端补 `today_posts/today_users` 字段。

### P1-5 `/create` 独立发帖页"上传图片"永远不会插入正文
- 位置：`static/js/app.js:287` `document.querySelector('#createPostForm [name="content"], #editPostForm [name="content"]')`。
- 问题：`pages/create.html:28` 的表单 id 是 `createPostPageForm`，选择器不匹配 → 上传成功、后端已存 D1（`routes_posts.py:265-272` 返回 `{url}`），但页面拿不到 textarea，图片 URL 无处落地，用户以为"传了图"实际没进帖。
- 修复：选择器补 `#createPostPageForm [name="content"]`；更稳妥的做法是记录触发上传的 input 所属 form。

### P1-6 移动端导航汉堡按钮 ReferenceError（`toggleNavMenu` 不是全局函数）
- 位置：`static/js/common.js:29`（注入的 `onclick="toggleNavMenu()"`）vs `common.js:409`（定义在 IIFE 内部，从未挂到 `window`）。
- 后果：窄屏点击菜单按钮报 `toggleNavMenu is not defined`，移动端无法打开导航（其余入口按钮除外）。
- 修复：`window.toggleNavMenu = toggleNavMenu;`（common.js 末尾，仿照 modal.js 的 window 导出）。

### P1-7 恋爱档案第二次保存返回 500（前端数组触发后端 UPDATE 分支未序列化）
- 位置：`pages/romance.html:218-224` `POST /api/romance/profile` 的 `hobbies` 为 JS 数组。
- 后端证据：`models_ext.py:225-244`：首次 INSERT 分支有 `json.dumps(hobbies)`（242 行），但已存在时的 UPDATE 分支（228-234 行）把 list 直接作为 D1 参数绑定 → 序列化失败 → `entry.py:70-74` 兜底 500。即用户"第二次保存档案必报错"。
- 修复（后端一行）：UPDATE 分支对 `hobbies` 同样 `json.dumps`；或前端改为发送逗号串并两端约定。属于联调必现 bug，列 P1。

---

## P2 · 中低优（体验 / 一致性 / 死代码 / 潜在风险）

### P2-1 `getStationIcon()` 把所有 lucide 图标名降级为 `school`
- 位置：`static/js/icons.js:281-285`；使用点：`app.js:364,498`、`post.html:96,136`、`station.html:137`、`profile.html:174`、`favorites.html:71`、`admin.html:164`、`waterfall.html:149`。
- 证据：`ICON_MAP`（icons.js:7-212）的 key 全是 emoji；而 stations.icon 实际存的是 lucide 名（`seed.py:41-69` `'heart'/'book-open'/'cat'/...`，create_station.html 下拉也是 lucide 名）。只有 `lucide:` 前缀和 `school` 恰好命中。
- 后果：帖子卡片/子站页/收藏/后台中除 school 外所有子站图标渲染成校徽。对比 `index.html:263` 直接用 `s.icon` 是对的。
- 修复：`getStationIcon = icon => ICON_MAP[icon] || (ICON_NAMES.has(icon) ? icon : 'school')`（或直接透传并让 lucide 自己处理未知名）。注：透传需与 P1-2 转义一并处理。

### P2-2 个人资料页 EXP 进度条恒为 0
- `pages/profile.html:102` 读 `u.experience`，后端 `models.py:68-80` `public_user` 字段名是 `exp`。

### P2-3 子站管理弹窗两处开关不回填/静默重置
- `pages/station.html:179/182`：`only_owner_posts` 未用 `s.only_owner_posts` 初始化；`:209` 每次保存都提交 `only_owner_posts: 0/1` → 打开设置直接点保存会把"仅站长可发帖"关掉。`tags` 超过 5 个被前端 slice(0,5) 静默截断（:207）。另外后端 `routes_stations.py:197-211` 对 `name` 无最少 2 字校验（create 有，edit 漏），配合表单 `name` 无 `required` 可把子站改名成空。

### P2-4 `profile` 页"子站"Tab 显示的是访问者自己的子站
- `pages/profile.html:164-165` 用 `/api/stations/mine`（`routes_stations.py:156-163`，取当前 token 用户），访问他人主页时看到的是"我加入的子站"，语义错误且泄露访问者隐私。后端暂无 `/api/stations/user/<uid>`，建议加路由或该 Tab 仅对自己显示。

### P2-5 同名全局函数互相覆盖（隐性炸弹 + 死代码）
- `openModal/closeModal/switchModal`：`modal.js:5-25`（用 `.active` 类）被 `app.js:28-71`（用 `.show` 类）覆盖。CSS 只有 `.modal-overlay.show`（`style.css:828`），`.active` 从未定义 → modal.js 内的 `openModal` 及其 ESC/遮罩关闭监听（modal.js:122-129/186-191）全部为死代码；`window.CampusModal.openModal`（modal.js:194）仍暴露 `'active'` 版——谁调用它就是"打开了个看不见的弹窗"。目前无页面直接调用，属高危陷阱。
- `handleReport`：`modal.js:152`（举报表单事件版）与 `admin.html:314`（处理举报版）同名，admin 页内联脚本后加载会覆盖前者——该页 reportModal 若被提交会以 `event` 当 `rid` 发请求。建议 admin 版改名 `handleReportAdmin`（与 `deletePostAdmin` 等命名一致）。
- `escHtml/showToast`：utils.js 与 app.js 各一份（app 版不转义 `'`，attribute 单引号语境有细微差异；showToast 差异见 P1-1）。
- `openForgotPassword/handleForgotPassword`：auth.js:82/111 与 app.js:195/200 重复，后者生效。

### P2-6 Lucide / Chart.js 依赖注入不完整
- 所有页面均只靠 `icons.js:220-232` 在 DOMContentLoaded 从 `unpkg lucide@latest` 动态加载（无 `<script>` 静态引入、无 SRI、版本漂移风险；unpkg 在部分校园网不可达时全站图标消失）。建议自托管固定版本（如 0.4xx UMD）并 `<link rel=preload>`。
- 部分渲染后未调 `lucide.createIcons()`：`trade.html:138`（商品卡/空态图标）、`romance.html:87/112/136`（三个 Tab 列表）、`shop.html:88/130`（商品与订单里的 coins/star 图标）——图标要等下一次别处的 createIcons 才替换，期间显示为空白 `<i>`。
- `checkin.html:160` 用 `class="icon icon-xs"`，`icons.css` 无 `.icon-xs`（0 命中）→ 日历对勾按 `.icon` 默认尺寸渲染。

### P2-7 上传链路核对结论（任务 3）✅ 基本可用，两处留意
- 链路：`uploadPostImage/upload_cover/auth/avatar`（multipart，字段名 `file` ✓ 与 `uploads.py:33` 一致）→ 后端返回 `/static/uploads/{posts|covers|avatars}/<key>`（`uploads.py:19,76`，**D1 分支同样返回该前缀**）→ 浏览器请求 `/static/uploads/...`：Pages Function `functions/static/uploads/[[path]].ts` 反代 → Worker `entry.py:60` `^/static/uploads/(?P<path>[A-Za-z0-9_./-]+)$` 命中（字符类含 `/`，支持子目录）→ `serve_upload`（`uploads.py:79-109`）取末段文件名作 key 查 D1 回传 base64 ✓。`/api/uploads/...`（entry.py:58）同源备用也通。post.html 渲染侧 `renderContent` 白名单 `^/static/|^/uploads/|^https?://`（app.js:450,458）覆盖 `/static/uploads/` ✓。**结论：闭环成立。**
- 留意①：`app.js:275` 前端限制 16MB，后端 `uploads.py:56` 限 5MB——大文件要等到上传完才报"文件过大"，建议前端同步为 5MB。
- 留意②：`_redirects:29` 的 `/* → 404` 兜底规则与 functions 的优先级在 Cloudflare Pages 是"3xx → Functions → 200 → 静态 → 404 兜底"，理论安全；**部署后建议实测 `GET /api/stations` 与 `GET /static/uploads/<某已存在key>` 未被 404 页吞掉**。

### P2-8 死链核对（任务 1d）✅ 全部命中
- `_redirects` 覆盖 `/ /post/* /station/* /profile/* /waterfall /trade /romance /gossip /shop /search /checkin /favorites /notifications /create /create-station /admin /reset-password /about /terms /privacy`；对全站 `href` / `location.href` 字面量扫描（约 30 个去重路径）无一越界；`forgot-password` 生成的 `origin + '/reset-password?token=…'` 亦命中。`reset_password.html` 用 `?token` 读取 ✓。
- 冗余：仓库根 `frontend/index.html` 与 `pages/index.html` **逐字节相同**（Compare 验证 IDENTICAL）——`/` 由 200 rewrite 指向 pages 版，根版仅多余，存在双份漂移风险，建议删除或改为 301 到 `/`。

### P2-9 后端有、前端没接的孤儿路由（功能缺口，非错误）
- `PUT /api/trade/<tid>/status`（改"已售/预定"）：trade.html 列表渲染了状态徽章却没有任何入口改状态（`routes_extended.py:398-408`）。
- `GET /api/recommend/interests`、`GET /api/shop/transactions`、`GET/POST /api/identity/*`（身份组自助/展示）、`GET /api/romance/profile/<uid>`：前端 0 调用。
- `POST /api/posts/<pid>/like` 的 `liked` 计数依赖前端解析按钮文本（app.js:300,313 的 `textContent.replace(/\D/g)`），后端并未返回新计数——可顺手让 like 接口返回 `likes_count`。

### P2-10 文案/细节杂项
- `pages/checkin.html:67`"连签7天可达 45 金币"：后端公式 `10 + min(streak-1,7)*5`（models_ext.py:52-54）第 7 天是 40（第 8 天才 45）。
- `pages/profile.html:260` 存在游离的 `</script>` 闭合标签（HTML 无效，浏览器容错）。
- `pages/romance.html:198-201` `openSendLink` 里 prompt 的留言 `desc` 被丢弃，`sendRomanceLink` 内部重新 prompt 或直接空串。
- `pages/search.html:165` 历史记录 `onclick="useSearchHistory('…')"` 仅 escHtml（不转义 `'`），可被自带 `'` 的历史项注入——仅本地 localStorage 自 XSS，低危。
- `static/js/app.js:184-193` `saveProfile`、`auth.js:224` `window.currentUser = currentUser`（快照后永不再同步，恒为 null）：均为死代码/误导，建议清理。
- `static/js/app.js:362-366` 往 `<option>` 里塞 `<i data-lucide>`：浏览器不渲染 option 内 HTML，下拉里图标必然丢失（改成纯文本前缀或自定义下拉）。
- `functions/api/[[path]].ts:50-51` 上线前建议去掉 `X-Debug-Target / X-Debug-Upstream-Status` 响应头（暴露内部 API 域名）。
- 管理页每次操作后 `switchAdminTab(document.querySelector('#adminTabs .tag'), '<对应tab>')`（admin.html:311,317,342,349,356,420）传的是第一个 tag 元素 → 高亮永远回到"仪表盘"，内容却是操作后 Tab，视觉错位。
- `pages/admin.html:205/223`、`pages/shop.html:74`：`g.icon`/`item.icon` 原样插 innerHTML（仅管理员可写 → 管理员存给用户看的存储型注入仍需转义，严重度低于 P1-2 因写权限受限）。
- 各页 `<main id="page-main">` 与 common.js 注入的 `<main id="page-main">` 造成重复 ID（`common.js:44`）：footer/modals 会插在真实内容之前，目前靠 `main` 布局容错显示正常，建议注入模板去掉重复 main 或页面内容改为 JS 挂载。

---

## 每页静态资源与 Lucide 初始化核对（任务 2）

| 页面 | css(3) / js(8) 引用 | 文件存在 | lucide 初始化 | 备注 |
|---|---|---|---|---|
| index / 404 / about / terms / privacy / admin / checkin / create / create_station / favorites / gossip / notifications / post / profile / reset_password / romance / search / shop / station / trade / waterfall | variables.css + style.css + icons.css；api→utils→icons→auth→modal→common→animations→app 全 8 件 | ✅ 全部存在（含 default-avatar.svg、avatars/1-8.svg） | 由 icons.js 统一 lazy-load 并 createIcons ✅ 每页都有（含 404/about/terms/privacy） | station/admin 额外引 Chart.js CDN ✓；app.js 在 modal.js 之后加载决定了 P2-5 的覆盖方向 |

onclick/onsubmit 全局函数交叉核对：除 `toggleNavMenu`（P1-6）外，所有内联处理函数均可在其引用的 js 或页内 `<script>` 中找到定义（脚本化比对 21 页 0 缺失）。

---

## 修复优先级清单（上线 gate）
1. **P0-1** waterfall 响应解包（1 行）
2. **P0-2** 后端补 vote/link 或前端下线（小工作量）
3. **P1-2** 转义 avatar/cover/icon/trade 数值（约 15 处插值 + 后端校验）
4. **P1-1** 统一 showToast（utils 版加 `.show`）
5. **P1-3** 三处 `querySelector('.modal-overlay').remove()` 改 `modal.remove()`
6. **P1-4/5/6/7** today_* 字段、create.html 上传选择器、toggleNavMenu 导出、romance hobbies 序列化

> 本审计未修改任何源文件。全部行号以当前工作区文件为准。
