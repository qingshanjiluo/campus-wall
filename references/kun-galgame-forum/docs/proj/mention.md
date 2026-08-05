# Mention + Quote 设计文档

> 用现代 **@mention**(通知 + 跳转到用户)和 **quote 引用**(跳转到楼层)取代畸形的
> 多目标回复(`TopicReplyTarget` 的 per-target content)。退役旧模型,让回复正文
> (`TopicReply.Content`)成为唯一真相。

## 1. 背景与病根

一条回复的文字现在**散落**在 `TopicReply.Content` + N 个 `TopicReplyTarget.Content`
里——"这条回复说了什么"不是读一个字段能回答的,逼着列表 / 搜索 / 通知 / SEO 摘要
全去 JOIN 拼装,嵌套与代码都被这个准嵌套模型拖累。这是半年实践确认的反模式。

正解是把两个被揉在一起的概念拆开:

- **@mention** = 提到某个**用户**(通知 + 链到其主页)。
- **quote** = 引用某个**楼层/回复**(跳转 + 卡片预览)。

二者**正交**,都只产生 markdown,落到 `Content` 这一个字段。

## 2. 现有脚手架(半数已就绪)

| 部分 | 状态 |
|---|---|
| 通知类型 `mentioned` | ✅ `NotifyMentioned` 常量 + `message.type` 枚举 + 前端 i18n「提到了您！」——**从未触发** |
| 用户搜索 `/users/search` | ✅ 已实现(代理 OAuth);C6 契约原文即「用于 @ 补全,不缓存」 |
| 编辑器插件机制 | ✅ milkdown v7 + `tooltipFactory` + `pluginViewFactory`(现有 `Tooltip.vue` 即模板) |
| markdown 渲染先例 | ✅ `/image/<hash>` token 由 goldmark 扩展**在消毒前**解析成绝对 URL(因 UGCPolicy 剥相对 URL);`class` 已全局放行 |
| 通知发射钩子 | ✅ `notifier.Emit(tx, Spec{Kind, ...})` |

## 3. Token 格式(存储,markdown)

- **Mention**:`[@名字](kungal-user:<userID>)`(`@` 在链接文本内,否则 goldmark 会把 `@` 留在 `<a>` 外)
  - 存 **user.id**(C1 三库共享主键)。`名字` 仅为写入时快照 + 兜底;**渲染解析当前名**(Slack/Discord/GitHub 共识:改名只能靠存 id 解决)。
- **Quote**:`[#<floor>](kungal-reply:<replyID>)`
  - 存被引用回复的 **id** + **楼层**。渲染成带样式的引用卡片 + 跳转(决策 B)。

两者都是"自定义 scheme 的 markdown 链接",经 goldmark 扩展识别。

## 4. 渲染(`markdown.go`)

**沿用 `/image/<hash>` 的先例**:加一个 goldmark 扩展(类似 `lazyImageExtension`),在
AST 渲染期把 token 转成**消毒可存活**的 HTML(在 sanitize 之前):

- `kungal-user:<id>` → `<a class="kun-mention" data-uid="<id>" href="<siteBase>/user/<id>">@名字</a>`
  - 用**绝对 URL**(配置 site base,仿 `SetContentImageCDNBase`)绕过 UGCPolicy 剥相对 URL;`class`/`data-*` 由 `AllowAttrs("class").Globally()` + 需补 `data-uid` 放行。
- `kungal-reply:<id>` → `<span class="kun-quote" data-reply-id="<id>" data-floor="<floor>">#<floor></span>`
  - **无 href**,避免 URL 问题;由**前端 hydrate** 成卡片(懒加载预览 + 跳转,仿 lazy-image 的 data-attr + 前端激活)。

**名字解析(改名自动反映)= 服务端**:渲染输出带 `data-uid` + 写入时快照名;然后在
话题/回复 mapper 把 mention id **并进已有的** `userClient.Hydrate` 批次、渲染后用
`ResolveMentionNames(html, id→当前名)` 套一层替换链接文本。主路径(话题正文 + 回复 +
最佳答案)就 **~2 个文件**——mapper 本来就批量拉作者,mention 用户搭便车、近零额外网络。
选服务端而非客户端(Slack/Discord 式)的理由:① 和 kungal「渲染全在服务端」的架构一致
(它专门去掉了每次 SSR 跑的客户端 DOMPurify,图片 token 也是服务端解析);② SSR 正确
(SEO / 无 JS / 不闪旧名 / 不多发请求)。Slack/Discord 客户端解析靠本地已缓存全量用户,
SSR 网页论坛没这个前提。快照名作「用户已注销」兜底;`data-uid` 同时给将来客户端解析留口子
(galgame 评论 / 私信等次要内容按需补)。

**消毒**:`newSanitizePolicy` 需补 `AllowAttrs("data-uid").OnElements("a")` +
`AllowAttrs("data-reply-id","data-floor").OnElements("span")`。

## 5. 通知(接通 `NotifyMentioned`)

- 话题 / 回复 / 评论 **create + update** 后,正则扫 `Content` 的 `kungal-user:(\d+)`:
  - 去重 → 排除自己 → (更新时)排除上一版已含的 id(只通知新增的)。
  - 每个 `notifier.Emit(tx, Spec{Kind: NotifyMentioned, RecipientID: id, Link: <帖子链接>, ...})`。
- 限流:每帖 mention 上限(沿用 targets 的 10);i18n 已就绪。

## 6. 编辑器(milkdown)

- **Mention 插件**:仿 `Tooltip.vue`(`tooltipFactory` + `pluginViewFactory` + Vue 下拉)。
  `@` 触发 → debounce 查 `/users/search?keyword=`(**不缓存**)→ 上下键/点击选 → 插入
  mention 行内 **atom 节点**(携带 `{userId, name}`,仿 inline-katex / sticker 节点)。
- **Quote**:点某楼"引用"→ 插入 quote 节点(`{replyId, floor}`)→ 渲染成卡片。
- **remark 序列化/解析**:节点 ↔ `@[name](kungal-user:id)` / `[#floor](kungal-reply:id)`,
  **往返**(编辑旧帖时还原成芯片/卡片)。

## 7. 迁移(Path B —— 退役 `TopicReplyTarget`)

数据齐全:`TopicReplyTarget = {Content(笔记), ReplyID, TargetReplyID}`,目标楼层/用户
经 `TargetReplyID → TopicReply.Floor/UserID` 一次 JOIN 即得。

对每条有 target 的 `TopicReply`:

1. 读其 targets(笔记 + TargetReplyID),JOIN 取目标 `floor` / `userID`。
2. 每个 target 折成一段(并入 `Content`,上/下/顺序随意):
   ```
   > 回复 [@](kungal-user:<targetUserID>) [#<floor>](kungal-reply:<targetReplyID>)

   <该 target 的 Content 笔记>
   ```
   名字留空 → 渲染解析当前名(迁移脚本不必碰 OAuth)。
3. `UPDATE topic_reply.content`;迁完 `DROP TABLE topic_reply_target`(或先留一版回滚)。
4. **不发通知**(数据搬运,不能把半年旧 @ 重新轰一遍)。
5. 悬空目标(被 purge 置 NULL 的 `target_reply_id`)→ 只保留笔记,不写 quote 链接。

**警示(改用户内容,不可逆)**:迁移前**备份** `topic_reply.content` + 整张
`topic_reply_target`;脚本**幂等**(只处理仍有 target 的回复);先在**库副本 dry-run**
肉眼核对几条;按铁律出 `migrations/NNN_*.up.sql` 或 Go 一次性 job,并在最后明确告知
**在哪个库、用什么命令跑**。

## 8. 前端

- `prose.css` 加 `.kun-mention`(链接芯片)/ `.kun-quote`(引用卡片)样式。
- `KunContent` 挂载后 hydrate `.kun-quote`(懒加载预览 + 跳转,仿 lazy-image)。
- 通知展示已就绪(i18n)。
- **退役多目标 compose**:`reply/Panel` + `PanelBody` 改成**单编辑器 + @ + quote**,
  删掉 per-target tab、`replyDraft.targets`、`TopicReplyTarget` 相关读写。

## 9. 分期实施

1. **后端基础(全部完成)**:✅ 1a token 渲染 + 消毒放行;✅ 1b `NotifyMentioned` 接线
   (topic/reply create+update)+ **服务端名字解析**(话题正文 / 回复 / 最佳答案,见 §4)。
2. **编辑器(全部完成)**:✅ 2.0 milkdown 7.17.3 → **7.21.2 真升级**(前一次只改了
   `apps/web/package.json`,被 root `pnpm.overrides` 钉死在 7.17.3;改 overrides 后整树
   统一 7.21.2,typecheck + build 双绿)。✅ 2.1 mention 行内 atom 节点 + remark 往返
   (`[@name](kungal-user:id)` ↔ 芯片)。✅ 2.2 **@ 下拉**:后端 `GET /api/user/search`
   (代理 OAuth `/users/search`,userAuth,不缓存)+ `MentionDropdown.vue`(`slashFactory`
   + `SlashProvider`,`@` 触发、查询、上下键/Tab/Enter/点击选、插入节点)。✅ 2.3 **quote
   节点** + remark 往返(`[#floor](kungal-reply:id)` ↔ 芯片,`quotePlugin.ts`)+
   `insertQuoteCommand`(供 Phase 3 的「引用」按钮调用)。后端渲染 / 消毒在 1a 已就绪。
   全程登录态浏览器实测:@ 候选、插入芯片、quote 往返、改名反映均通过。
3. **前端**(进行中):✅ 3a `.kun-mention`/`.kun-quote` 渲染样式(`prose.css`,
   `.kun-prose` + `.kun-prose-compact` 双作用域)。✅ 3c **退役多目标 compose**:回复
   = 单 body 编辑器,`replyDraft` 去掉 `targets`,提交只发 `{content}`;后端
   create/update 停写 `TopicReplyTarget`(**读路径保留**,旧回复仍显示 targets 卡片到
   Phase 4)。**「引用」= 往编辑器内插入内联 `@作者 #楼层` 头**(WYSIWYG,编辑即所见):
   `footer/Reply.vue` 置 `tempReply.pendingQuote`(一次性信号,经 `BodyEditor`→
   `DualEditorProvider` 透传到 `Editor.vue`),编辑器在 **mounted**(空开)或 **watch**
   (已开追加)时,对**已聚焦的活动 view** 直接 dispatch 事务插 mention+quote 原子节点 +
   空 body 段并落光标(空文档 → `[头, 空 body]` 光标在 body;非空 → 段首前插头),插完
   `emit('quoteInserted')` 清信号;按 reply id 去重。**根因复盘**:早期用 `defaultValueCtx`
   带头挂载 + 挂载后追光标,与 Vue 响应式 / Teleport 挂载 / 输入法合成有时序竞争(光标看似
   在第 2 行、首字却跳回第 1 行,多 hook 还会多插空行)。改用 **@ 下拉菜单同款"活动 view 内
   dispatch"** 路径(其 IME 实测可靠)即根治。登录态实测:内联头 + 空 body 光标、输入法
   (`execCommand insertText`)正文落第 2 行不跳、去重、多引用置顶、发布 content 内联 token、
   取消重开不丢正文不重复均通过。✅ 3b `.kun-quote` hydrate(`useQuoteContent`
   委托监听 + `TopicQuotePreview` 卡片):点击 → 跳到该楼层(`[id^="<floor>."]` 锚点 +
   高亮);悬停 → 懒加载 `/topic/:tid/reply/detail` 预览卡(作者 + 楼层 + 摘要,按 id 缓存)。
   跨页跳转仍按 §10 延后(不在当前页 → 提示)。登录态注入实测:点击跳楼 + 悬停卡均通过。
   **触发架构(已定 A)**:`@`=用户;`#`=内容(话题/游戏,**未来阶段**,分类菜单);楼层引用
   走「引用」按钮。`kungal-<type>:<id>` token + node + `$remark` 模式可直接复用到
   `kungal-topic:`/`kungal-galgame:`,故 topic/galgame mention 是后续独立阶段,不阻塞当前。
4. **迁移(Phase 4)**:✅ 一次性 job `cmd/migrate-reply-targets`(默认 dry-run;
   `-dry-run=false` 才写)。把每条 `topic_reply_target` 折成
   `[@](kungal-user:id) [#floor](kungal-reply:id)\n\n<笔记>`(纯头,无 `>` 引用块)前置到作者回复正文,
   然后删该 target 行。**名字留空**靠 `ResolveMentionNames` 渲染期解析(不碰 OAuth);
   悬空目标只留笔记;**不发通知**。apply 前自动建 `topic_reply_target_backup` +
   `topic_reply_content_backup`(`IF NOT EXISTS`,重跑不覆盖,即回滚源);每条独立事务 →
   **幂等可续跑**。本地副本库实测:4517 回复迁移、备份齐(4541/4517)、重跑 no-op、空名
   token 渲染正常。✅ **收尾(prod 数据迁移 2026-06-21 后)**:删 `Target.vue` + mapper
   targets 读路径(33b4ed39);`DROP` 两张 backup 表;彻底退役——删 `TopicReplyTarget` 模型 +
   lifecycle、activity/search/user 三处 content 聚合查询、`cmd/migrate-reply-targets`,
   出 `migrations/028_drop_topic_reply_target`,**已于 2026-06-21 应用**(先 redeploy api,再 `migrate --only=028`;表 + 序列已 DROP,activity/search/user 三处改写查询线上 200 验证)。

   **prod 执行(用户操作,我不 push/不跑)**:推 master → CI 出 `kungal-tools` 镜像 →
   `docker compose -f docker-compose.prod.yml --profile jobs run --rm tools migrate-reply-targets`
   (dry-run 核对)→ 加 `-dry-run=false` 正式跑;库 = infra postgres 容器的 `kungalgame`。

## 10. 暂不做(后续单独排期)

- **跨页跳到具体楼层**:因回复懒加载分页,跳 `/topic/:id#k<floor>` 时该楼可能未加载。
  需 BE「定位楼层」端点(`GET /topic/:id/reply/:floor/context` 或 by-id 定位页码)+
  前端落地页按锚点滚动。`Target.vue` 的 `loadDetail` 兜底已暗示此缺口。quote 卡片先做
  **页内**跳转,跨页等此端点。
