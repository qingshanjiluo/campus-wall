# 全站 SEO 元数据审计

> 本文档逐页记录 `apps/web/app/pages/` 下所有 74 个页面的 SEO 状态，包括
> `useKunSeoMeta` / `useKunDisableSeo` 调用、JSON-LD schema.org 结构化数据、
> NSFW 与失败兜底处理。SEO 配置直接影响搜索引擎收录质量，任何改动都需要在
> 此文档对照核对。
>
> 维护规约（重要）：新增页面或调整 SEO 元数据时，**必须同步更新此表**。
>
> 最后审计：2026-05-26（K-PR SEO sweep）

## 目录

- [0. 调用约定与背景](#0-调用约定与背景)
- [1. 公开列表 / 索引页（21 个）— 全部 SEO 开启，无 JSON-LD](#1-公开列表--索引页21-个-全部-seo-开启无-json-ld)
- [2. 公开详情页（12 个）— 含完整 SEO + 部分含 JSON-LD + NSFW 守护](#2-公开详情页12-个-含完整-seo--部分含-json-ld--nsfw-守护)
- [3. 分类型导航页（4 个）— 静态常量 SEO](#3-分类型导航页4-个-静态常量-seo)
- [4. 用户主页子标签（8 个）— 1 个允许索引，其余全禁](#4-用户主页子标签8-个-1-个允许索引info其余全禁-seo)
- [5. 消息中心（5 个 + 1 wrapper）](#5-消息中心5-个--1-wrapper-全为登录态但保留-seo-meta仅-tab-title)
- [6. 编辑流（10 个）](#6-编辑流10-个-全为登录态title-only-seo)
- [7. 管理后台 / 特殊后台页（5 个）](#7-管理后台--特殊后台页5-个-全部-noindex)
- [8. 辅助 / 转换 / 信息页（5 个）](#8-辅助--转换--信息页5-个)
- [9. 三个"容器 wrapper" 页面 — 故意没有 SEO](#9-三个容器-wrapper-页面--故意没有-seo)
- [10. 设计权衡 / 已知陷阱](#10-设计权衡--已知陷阱)
- [11. JSON-LD 详细规格](#11-json-ld-详细规格)
- [12. 维护清单](#12-维护清单开发者参考)

## 速查矩阵

| 维度 | 数量 / 范围 |
|---|---|
| 总页面数 | **74** |
| `useKunSeoMeta`（正向 SEO） | **57** |
| `useKunDisableSeo`（反向 SEO，noindex） | **24** 处调用，分布在 18+ 页面（含 NSFW/404 双分支） |
| 注入 JSON-LD schema.org | **5 个详情页**：`/galgame/:gid`, `/topic/:id`, `/galgame-rating/:id`, `/toolset/:id`, `/website/:domain` |
| 完全无 SEO 调用（wrapper） | **3**：`category.vue`, `ranking.vue`, `update.vue` |
| `middleware: 'auth'` 守护 | **18+**：message/edit/admin/setting 等 |

## 一页速览（按 SEO 类型分类）

| 状态 | 页面数 | 代表 |
|---|---|---|
| 完整 SEO + JSON-LD | 5 | `/galgame/:gid`, `/topic/:id`, `/galgame-rating/:id`, `/toolset/:id`, `/website/:domain` |
| 完整 SEO（无 JSON-LD） | 52 | 其余所有列表 / 详情 / 导航 / 公开页 |
| 始终 noindex | 13 | OAuth callback / admin setting/submissions/user / edit-rewrite / 7 个 user 子标签 / sexual tag |
| NSFW / 404 条件 noindex | 12 | 所有公开详情页（详见 §2）|
| 无 SEO 调用 | 3 | category/ranking/update wrapper（子页面已处理） |

---

## 0. 调用约定与背景

### 0.1 两个核心 composable

| 名字 | 行为 |
|---|---|
| `useKunSeoMeta({ title, description, ogImage, ... })` | 正向 SEO：设置 `<title>`、`<meta name="description">`、`og:*` 系列、可选 `article:*`。完整结构化曝光 |
| `useKunDisableSeo(title)` | 反向 SEO：设置 `<meta name="robots" content="noindex, nofollow">` + 清空 `description` / `themeColor` / `schemaOrg.host`。`title` 仍渲染到 `<title>` 标签，便于浏览器 tab 显示，但搜索引擎不会索引 |

**关键不变量**：**在同一页面对同一次渲染**，二者只能调用其一。如果先调
`useKunDisableSeo` 再调 `useKunSeoMeta`，后者会**覆盖** `description` 等字段
但 **不会覆盖** `robots` 头 —— 这是先前在 `galgame-resource/[id]` 引发的 bug
（NSFW 资源的 title/description/og:image 仍泄漏给爬虫）的根因，参见 §3 修复说明。

### 0.2 何时该禁用 SEO

按优先级从高到低：

1. **数据加载失败 / 404**：远端 fetch 返回 null 或 `'banned'` 等失败哨兵 →
   `useKunDisableSeo('未找到 XXX')`。**绝对禁止**让 `data.value?.field` 把
   `undefined` 渲染进 `<title>` 或 `<meta>`。
2. **NSFW 内容**：作品 `contentLimit === 'nsfw'`、话题 `isNSFW === true`、
   网站 `ageLimit === 'r18'`、标签 `category === 'sexual'` —— 一律 `noindex`，
   避免被搜索引擎归类为成人站点。
3. **被封禁内容**：`status === 1`（话题）、`'banned'`（用户/galgame）—— `noindex`，
   失败语义已经消失的资源不该继续被索引。
4. **管理后台 / 编辑表单 / 个人设置 / OAuth callback / 私信** 等
   "登录必经页"：业务上没有公开浏览价值，全部 `noindex`。

### 0.3 JSON-LD 适用范围

只在**有结构化数据可言**的详情页注入 schema.org `<script type="application/ld+json">`：

| 页面 | schema 类型 |
|---|---|
| `/galgame/:gid` | `VideoGame` |
| `/topic/:id` | `DiscussionForumPosting`（含 `acceptedAnswer: Comment` 当 `bestAnswer` 存在） |
| `/galgame-rating/:id` | `Review` |
| `/toolset/:id` | `SoftwareApplication`（含 `AggregateRating`）+ 可选 `DiscussionForumPosting`（当 commentCount > 0） |
| `/website/:domain` | `Article`（含 `about: WebSite`、`review: Review[]`） |

**NSFW 详情页一律不注入 JSON-LD**：当走入 NSFW 分支时，FE 直接走
`useKunDisableSeo` 并跳过 JSON-LD 注入。检查方法：`if (NSFW) { useKunDisableSeo } else { useHead + useKunSeoMeta }`。

---

## 1. 公开列表 / 索引页（21 个）— 全部 SEO 开启，无 JSON-LD

这些页面是 SEO 主入口，使用静态 `useKunSeoMeta` —— 列表内容随时间变化但
分类语义稳定，关键词写在标题/描述里供 Google 索引。

| 路径 | Title | Description（关键词主体） | 启用 SEO |
|---|---|---|:---:|
| `/`（`index.vue`） | `主页` | `kungal.description` | 是 |
| `/galgame` | `Galgame 资源 wiki` | Galgame 下载/Wiki/汉化/Windows/macOS/PC/Android/Linux 等 | 是 |
| `/galgame-resource` | `最新 Galgame 资源下载` | PC/Windows/手机/模拟器/KRKR/Tyranor 等 | 是 |
| `/galgame-rating` | `Galgame 评分列表` | 评分 / 短评 / 剧透等级 / 游玩状态 / 排序维度 | 是 |
| `/galgame-series` | `Galgame 系列` | 美少女万华镜/灰色/近月少女的礼仪/巧克力与香子兰/9-nine 等 | 是 |
| `/galgame-official` | `Galgame 会社 / 制作商 Wiki` | アリスソフト / Frontwing / HOOKSOFT / minori / CIRCUS / ゆずソフト 等 | 是 |
| `/galgame-engine` | `Galgame 引擎 Wiki` | 引擎列表 + 关联 Galgame | 是 |
| `/galgame-tag` | `Galgame 标签 Wiki` | 青梅竹马/幼驯染/高中生/萝莉/白毛 等 | 是 |
| `/website` | `Galgame 网站 Wiki` | 资源/社区/论坛/资讯/Wiki/Telegram 等仅免费、严禁付费 | 是 |
| `/toolset` | `Galgame 工具资源下载` | 模拟器 / 解包 / 补丁 / 汉化工具 | 是 |
| `/topic` | `话题列表` | Galgame 交流 / 资源 / 补丁 / 逆向 / 汉化 / 新作 / 技术 | 是 |
| `/resource` | `资源和求助话题列表` | Galgame 下载 + 技术求助话题 | 是 |
| `/doc` | `Galgame 帮助文档` | 发布 / 交流 / 资源 / 联系我们 | 是 |
| `/activity` | `动态时间线` | 全站话题、回复、Galgame 与社区动态 | 是 |
| `/activity/category` | `站内动态列表` | 分类动态 | 是 |
| `/ranking/galgame` | from `rankingPageMetaData['galgame']` | 同上 | 是 |
| `/ranking/topic` | from `rankingPageMetaData['topic']` | 同上 | 是 |
| `/ranking/user` | from `rankingPageMetaData['user']` | 同上 | 是 |
| `/search` | `搜索 Galgame` | 搜索 Galgame / 资源 / 话题 / 用户 / 回复 / 评论 | 是 |
| `/rss` | `Galgame 和话题订阅` | RSS 订阅入口 | 是 |
| `/friend-links` | `友情链接网站` | 友链列表 + `articleAuthor` 列友链 URL | 是 |

---

## 2. 公开详情页（12 个）— 含完整 SEO + 部分含 JSON-LD + NSFW 守护

详情页是项目 SEO 的核心，每一页都做了 **`if/else` 三分支**：
**(A) NSFW** → `useKunDisableSeo` + **不**注入 JSON-LD；
**(B) 失败 / 404** → `useKunDisableSeo`；
**(C) 正常** → `useKunSeoMeta` + 可选 JSON-LD。

### 2.1 `/galgame/:gid` — `pages/galgame/[gid]/index.vue`

| 分支 | 行为 |
|---|---|
| `data === 'banned'` | `useHead({ meta: [robots:noindex,nofollow] })` + `useKunSeoMeta({ title: '这个 Galgame 已被封禁' })`（注：见下方说明） |
| `contentLimit === 'nsfw'` + 匿名 + SFW cookie | `useKunDisableSeo('')`（空 title） + 模板显示 "确认显示" 拦截卡 |
| `contentLimit === 'nsfw'` + 登录 或 NSFW cookie | `useKunDisableSeo(title)`（保留 tab title） + 直接渲染 |
| SFW 正常 | `useKunSeoMeta({ title, description, ogImage, articleAuthor/PublishedTime/ModifiedTime })` + **JSON-LD `VideoGame`**（含 `publisher` 来自 official、`isPartOf: CreativeWorkSeries`、`genre/keywords` 来自 tag、`interactionStatistic` 来自 like/view、`author/contributor`） |
| 数据缺失 | `useKunDisableSeo('请求 Galgame 错误')` |

注意：`banned` 分支使用 `useHead` 手设 robots 而非 `useKunDisableSeo`，是历史路径。
新加分支统一用 `useKunDisableSeo`。

### 2.2 `/topic/:id` — `pages/topic/[id]/index.vue`

| 分支 | 行为 |
|---|---|
| `topic.isNSFW` + 匿名 + SFW cookie | `useKunDisableSeo('')` + `isShowTopic = false` → 模板显示 "确认显示" 拦截卡 |
| `topic.isNSFW` + 登录 或 NSFW cookie | `useKunDisableSeo(topic.title)`（仍 noindex） + 直接渲染 |
| 正常 | `useKunSeoMeta({ title, description, ogImage=getFirstImageSrc(contentHtml), ogType:'article', articleAuthor/PublishedTime/ModifiedTime })` |
| 数据缺失 / `'banned'` | `useKunDisableSeo('未找到此话题')` 或 `'话题已被封禁'` |
| **JSON-LD（始终注入，含 NSFW）** | `DiscussionForumPosting` — `headline / description / image / author / datePublished / dateModified / interactionStatistic[CommentAction, LikeAction, VoteAction] / commentCount / keywords` + **`acceptedAnswer: Comment`**（仅当 BE `topic.bestAnswer` 存在；详见 [bestAnswer 数据流](#52-topicbestanswer-数据流)） |

注：JSON-LD 即使 NSFW 也注入，**但 `robots: noindex` 会让爬虫忽略整页**，
所以等价无害。这是设计权衡（避免 isNSFW 分支需要重复构造 JSON-LD）。

### 2.3 `/galgame-resource/:id` — `pages/galgame-resource/[id]/index.vue`

| 分支 | 行为 |
|---|---|
| `data.galgame.contentLimit === 'nsfw'` | `useKunDisableSeo(titleBase)` — **不调用** `useKunSeoMeta`（曾因连写导致 SEO 泄漏，2026-05 修复） |
| 正常 | `useKunSeoMeta({ title: "${titleBase} ${typeLabel}资源下载", description: resource.note || "${type} · ${language} · ${platform} · ${size}", ogImage: getEffectiveBanner(galgame) })` |
| `data === 'not found'` / 缺失 | `useKunDisableSeo('未找到 Galgame 资源')` |
| JSON-LD | **未注入**（资源条目缺少独立 schema.org 实体语义；inherit galgame 详情已注入） |

### 2.4 `/galgame-rating/:id` — `pages/galgame-rating/[id].vue`

| 分支 | 行为 |
|---|---|
| `data.galgame.contentLimit === 'nsfw'` | `useKunDisableSeo("${user.name} 的评价")` — 跳过 JSON-LD |
| 正常 | `useKunSeoMeta({ title, description, ogImage=getEffectiveBanner(galgame), articleAuthor/PublishedTime/ModifiedTime })` + **JSON-LD `Review`** — `itemReviewed: VideoGame`（含 `isFamilyFriendly: ageLimit !== 'r18'`、`publisher` 来自 official、`isPartOf` 来自 series）、`reviewRating: Rating(1-10)`、`reviewBody: short_summary[0..250]`、`additionalProperty: PropertyValue[]`（艺术/故事/音乐/角色/路线/系统/声优/重玩） |
| 数据缺失 | `useKunDisableSeo('请求 Galgame 评分数据错误')` |

### 2.5 `/galgame-series/:id` — `pages/galgame-series/[id].vue`

| 分支 | 行为 |
|---|---|
| `data.isNSFW` | `useKunDisableSeo(data.name)` |
| 正常 | `useKunSeoMeta({ title: "${name} 系列下载资源", description })` |
| 数据缺失 | `useKunDisableSeo('未找到 Galgame 系列')` |
| JSON-LD | 未注入 |

### 2.6 `/galgame-official/:id` — `pages/galgame-official/[id].vue`

| 分支 | 行为 |
|---|---|
| 正常 | `useKunSeoMeta({ title: "${name} 会社", description: "${name}, 即 ${alias.join('|')}, 查看会社 ${name} 制作的所有 Galgame" })` |
| 数据缺失 | `useKunDisableSeo('未找到 Galgame 会社')` |
| JSON-LD | 未注入 |
| NSFW 处理 | **不需要** — official 自身是元数据，无 content_limit。所属 galgame 列表由 BE SFW filter 处理（详见 NSFW 协议文档） |

### 2.7 `/galgame-engine/:id` — `pages/galgame-engine/[id].vue`

| 分支 | 行为 |
|---|---|
| 正常 | `useKunSeoMeta({ title: "${name} 引擎", description: "查看所有使用 ${name} 引擎制作的 Galgame" })` |
| 数据缺失 | `useKunDisableSeo('未找到 Galgame 引擎')` |
| JSON-LD | 未注入 |
| NSFW 处理 | **不需要** — engine 是元数据 |

### 2.8 `/galgame-tag/:id` — `pages/galgame-tag/[id].vue`

| 分支 | 行为 |
|---|---|
| `data.category === 'sexual'` | `useKunDisableSeo("标签 ${name} 的 Galgame")` — sexual 类别 tag 自身就视为成人内容 |
| 其他 category | `useKunSeoMeta({ title: "标签 ${name} 的 Galgame", description: 当前页所有 galgame 名字逗号串 })` |
| 数据缺失 | `useKunDisableSeo('未找到 Galgame 标签')` |
| JSON-LD | 未注入 |

### 2.9 `/toolset/:id` — `pages/toolset/[id]/index.vue`

| 分支 | 行为 |
|---|---|
| 正常 | `useKunSeoMeta({ title: "${name} 资源下载", description, articleAuthor/PublishedTime/ModifiedTime })` + **JSON-LD `SoftwareApplication`** — `name / alternateName / applicationCategory / operatingSystem / softwareVersion / inLanguage / datePublished/Modified / author / sameAs(homepage[:5]) / interactionStatistic[WatchAction, DownloadAction] / aggregateRating(practicalityAvg/Count)` |
| 当 `commentCount > 0` 时**额外**注入 JSON-LD | `DiscussionForumPosting` — `headline / articleBody / datePublished/Modified / author / commentCount / comment: [...commentPreview]` |
| 数据缺失 | `useKunDisableSeo('未找到该工具资源')` |
| NSFW 处理 | **不需要** — toolset 是工具类资源（解包/汉化工具等），无 NSFW 语义 |

### 2.10 `/website/:domain` — `pages/website/[domain].vue`

| 分支 | 行为 |
|---|---|
| `data.ageLimit !== 'all'`（即 r18） | `useKunDisableSeo(data.name)` — 跳过 JSON-LD |
| `ageLimit === 'all'` | `useKunSeoMeta({ title, description, ogImage: icon, articlePublishedTime/ModifiedTime })` + **JSON-LD `Article`**（含 `about: WebSite`（`isFamilyFriendly: ageLimit !== 'r18'`）、`review: Review[]`（comment 列表映射）、`keywords: category + tags`） |
| 数据缺失 | `useKunDisableSeo('未找到该网站')` |

### 2.11 `/doc/:slug` — `pages/doc/[...slug].vue`

| 分支 | 行为 |
|---|---|
| 正常 | `useKunSeoMeta({ title, description, ogImage: banner, ogType: 'article', articleAuthor/PublishedTime/ModifiedTime })` |
| 数据缺失 | `useKunDisableSeo('未找到该文档')` |
| JSON-LD | 未注入（doc 是项目内置教程，没有进一步 schema 价值） |
| NSFW 处理 | **不需要** — 文档全是项目说明 |

### 2.12 `/user/:id` 容器 — `pages/user.vue`

| 分支 | 行为 |
|---|---|
| `data === 'banned'` | `useKunDisableSeo('该用户已被封禁')` |
| 正常 | `useKunSeoMeta({ title: data.name, description: data.bio })` |
| 数据缺失 | `useKunDisableSeo('未找到该用户')` |

**子页面** (`user/[id]/info.vue` 等) 见 §4。

---

## 3. 分类型导航页（4 个）— 静态常量 SEO

| 路径 | Title 来源 | Description 来源 | 启用 SEO |
|---|---|---|:---:|
| `/category/:name` | `KUN_TOPIC_CATEGORY[name]` | `KUN_CATEGORY_DESCRIPTION_MAP[name]` | 是 |
| `/section/:section` | `KUN_TOPIC_CATEGORY[section[0]] - KUN_TOPIC_SECTION[section]` | `KUN_TOPIC_SECTION_DESCRIPTION_MAP[section.toLowerCase()]` | 是 |
| `/website-category/:name` | `data.label` | `data.description` | 正常 / `useKunDisableSeo('未找到该网站分类')` 失败 |
| `/website-tag/:name` | `data.name` | `data.description` | 正常 / `useKunDisableSeo('未找到该网站标签')` 失败 |

注意：**`/category/:name` 与 `/section/:section` 已知小问题**：当 URL 参数命中不到
常量 map 时，title 会出现 `"undefined - undefined"` 字面值。属低优先级，因为
正常路径下用户不会到这个 URL。如要修：包一层 `if (KUN_TOPIC_CATEGORY[name]) {
useKunSeoMeta(...) } else { useKunDisableSeo('未找到该分类') }`。

---

## 4. 用户主页子标签（8 个）— 1 个允许索引（info），其余全禁 SEO

用户主页（`/user/:id/*` 嵌套页）每页都是该用户的个人活动子集（回复/评论/收藏/评分等）。
**没有公开 SEO 价值**（被搜索引擎索引会产生大量低质重复页），全部 `useKunDisableSeo`：

| 路径 | 标题（仅做 tab） | 备注 |
|---|---|---|
| `/user/:id/info` | `${user.name} 的主页`（例外） | **唯一例外**：profile 主页是值得索引的（用户名 + bio），用 `useKunSeoMeta`。同时设置 `canonical` 链接收敛 SEO 信号 |
| `/user/:id/topic/[type]` | 动态 | `useKunDisableSeo` |
| `/user/:id/reply/[type]` | 动态 | `useKunDisableSeo` |
| `/user/:id/comment/[type]` | 动态 | `useKunDisableSeo` |
| `/user/:id/galgame/[type]` | 动态 | `useKunDisableSeo` |
| `/user/:id/resource/[type]` | 动态 | `useKunDisableSeo` |
| `/user/:id/rating` | `${user.name} 的评分` | `useKunDisableSeo` |
| `/user/:id/setting` | `信息设置` | `useKunDisableSeo`，且 `middleware: 'auth'` |

---

## 5. 消息中心（5 个 + 1 wrapper）— 全为登录态，但保留 SEO meta（仅 tab title）

消息页都是登录后页面（`middleware: 'auth'`），爬虫无法访问。仍设 `useKunSeoMeta`
仅为**浏览器 tab 标题显示**，没有 robots noindex —— 因为 auth middleware 已经把
爬虫挡在外面：

| 路径 | Title | 备注 |
|---|---|---|
| `/message` | `我的消息` | 容器页 |
| `/message/notice` | `通知消息` | |
| `/message/system` | `系统消息` | |
| `/message/user/:id` | `私信` | |
| `/message/wiki` | `Wiki 通知` | |
| `/message.vue` | — | wrapper，子页面已处理 |

> 严格来说也可以全用 `useKunDisableSeo`（auth-gated 页面爬虫到不了），
> 但当前用 `useKunSeoMeta` 给 tab 设标题也无害。**保持现状**。

---

## 6. 编辑流（10 个）— 全为登录态，title-only SEO

提交 / 编辑表单类页面，登录态访问。一律 `useKunSeoMeta({ title: ... })` 仅设标题，
不写 description。理论上爬虫到不了（auth middleware），SEO 元数据只是浏览器 tab。
唯一启用 `useKunDisableSeo` 的是 `edit/toolset/rewrite.vue`（语义上是 rewrite，
内容本质是私人编辑稿，noindex 更保险）。

| 路径 | SEO 处理 | middleware |
|---|---|---|
| `/edit/topic` | `useKunSeoMeta({ title: '发布话题' })` | `auth` |
| `/edit/doc/create` | `useKunSeoMeta({ title: '创建文档' })` | `auth` |
| `/edit/doc/rewrite` | `useKunSeoMeta({ title: '重新编辑文档' })` | `auth` |
| `/edit/galgame/create` | `useKunSeoMeta({ title: '发布 Galgame' })` | `auth`, `prevent` |
| `/edit/galgame/publish` | `useKunSeoMeta({ title: '发布 Galgame' })` | `auth` |
| `/edit/galgame/rewrite` | `useKunSeoMeta({ title: '重新编辑 Galgame' })` | `auth`, `prevent` |
| `/edit/galgame/draft/:gid` | `useKunSeoMeta({ title: '编辑草稿' })` | `auth` |
| `/edit/galgame/mine` | `useKunSeoMeta({ title: '我的 Galgame 提交' })` | `auth` |
| `/edit/toolset/create` | `useKunSeoMeta({ title: '发布 Galgame 工具' })` | `auth`, `prevent` |
| `/edit/toolset/rewrite` | `useKunDisableSeo('重新编辑工具信息')` | `auth` |

---

## 7. 管理后台 / 特殊后台页（5 个）— 全部 noindex

| 路径 | SEO 处理 | 原因 |
|---|---|---|
| `/admin` | `useKunSeoMeta({ title: '管理系统', description: '世界上最强大美观的 Galgame 网站管理系统...' })` | 容器页保留 SEO meta 不致命，但实际进入需要 admin role |
| `/admin/overview` | `useKunSeoMeta({ title: '数据总览', ... })` | 同上 |
| `/admin/setting` | `useKunDisableSeo('网站设置')` | 私人后台 |
| `/admin/submissions` | `useKunDisableSeo('Galgame 审核')` | 私人后台 |
| `/admin/user` | `useKunDisableSeo('用户管理')` | 私人后台 |

> `/admin` 与 `/admin/overview` 当前保留正向 SEO meta，是因为整套 admin 板块
> 由权限 middleware 守护，爬虫访问会被 401/403 拦下，meta 是死信。
> **可优化**：可以改为 `useKunDisableSeo` 以严格闭环。低优先级。

---

## 8. 辅助 / 转换 / 信息页（5 个）

| 路径 | SEO 处理 | 备注 |
|---|---|---|
| `/auth/callback` | `useKunDisableSeo('OAuth 登录回调')` | OAuth 回调跳转页，临时页面，不该被索引 |
| `/unmoe` | `useKunSeoMeta({ title: '不萌记录', description })` | 公开违规墙，索引可接受 |
| `/report` | `useKunSeoMeta({ title: '匿名举报', description })` | 公开举报入口 |
| `/update/history` | `useKunSeoMeta({ title: '更新历史', description })` | 项目变更日志 |
| `/update/todo` | `useKunSeoMeta({ title: '待办列表', description })` | 待办公示 |

---

## 9. 三个"容器 wrapper" 页面 — **故意没有 SEO**

这些是 `<NuxtPage />` 套子，没有自己的内容；子页面已设 SEO：

| 路径 | 子页面 |
|---|---|
| `pages/category.vue` | `category/[name].vue` |
| `pages/ranking.vue` | `ranking/{galgame,topic,user}/index.vue` |
| `pages/update.vue` | `update/{history,todo}.vue` |

---

## 10. 设计权衡 / 已知陷阱

### 10.1 `useKunDisableSeo` 与 `useKunSeoMeta` 顺序

**`useKunSeoMeta` 在后会覆盖** `useKunDisableSeo` 的 title/description/ogImage，
**但 `robots: noindex` 不会被覆盖**（因为它由 `useHead({ meta })` 设置，而
 `useKunSeoMeta` 操作的是 `useSeoMeta` 的 OG 系列）。这意味着如果两个都调用：

- 爬虫看到 `robots: noindex` → 不索引（safe）
- 但 `<title>` / `<meta description>` / `og:image` 仍正常显示（leak）

部分爬虫（非 Google）可能忽略 robots 仍抓 meta。**所以一定要 if/else 分支**，
不能让两个 helper 同时生效。

**历史 bug**：`pages/galgame-resource/[id]/index.vue` 在 2026-05 之前是顺序调用
（先 `useKunDisableSeo` 再 `useKunSeoMeta`），NSFW 资源详情的 og:image
等仍泄漏。现在改成 `if (NSFW) { useKunDisableSeo } else { useKunSeoMeta }` 互斥。

### 10.2 `topic.bestAnswer` 数据流

BE `TopicDetail` 已嵌入 `bestAnswer?: TopicBestAnswerSummary`（id/floor/user/
contentMarkdown/contentHtml/created），由 BE service 通过 `topic.best_answer_id`
反查 `topic_reply` 并 hydrate identity 完成。FE topic 详情页将其映射为
`schema.org Comment` 类型作为 `acceptedAnswer`，触发 Google 论坛富搜索结果
（Q&A rich result）。

**字段从 wiki 后端重构后改名**：旧 shape `{ id, topicId, floor, user, created, edited }`
已废弃，新 shape `{ id, floor, user, contentMarkdown, contentHtml, created }`。
任何引用旧字段名的代码都会失败。检查命令：
```bash
grep -rn "topicId.*bestAnswer\|TopicBestAnswer\b" apps/web --include="*.vue" --include="*.ts"
```

### 10.3 NSFW 详情页 UX 矩阵（与 SEO 联动）

| 场景 | BE 响应 | FE 行为 | SEO meta |
|---|---|---|---|
| 匿名 + SFW cookie + 访问 NSFW galgame/topic | 返回完整数据 | `<KunCard>` 拦截 + 确认按钮 | `noindex` + 空 title |
| 匿名 + NSFW cookie | 返回完整数据 | 直接渲染 | `noindex` + 真实 title |
| 登录 + 任何 cookie | 返回完整数据 | 直接渲染 | `noindex` + 真实 title |
| 任何 + SFW galgame/topic | 返回完整数据 | 直接渲染 | 完整 SEO + JSON-LD |

**核心权衡**：BE 不 gate 详情（spec §16.2 "直接 URL 访问是有意为之"），FE
通过 cookie + login 决定 UX；SEO 通过 `useKunDisableSeo` 保证爬虫不索引 NSFW。

### 10.4 字段名漂移（每次后端重构都要核对此节）

随着 wiki / kungal BE 重构，FE 引用的字段名可能漂移。**已知历史漂移**：

| 旧字段 | 新字段 | 影响范围 |
|---|---|---|
| `galgame.released`（"YYYY-MM" 哨兵字符串） | `release_date` + `release_date_tba` | 详情页 / 列表 / 编辑器 |
| `banner_image_hash` | `effective_banner_hash`（PR5 退役） | 所有 banner 引用 |
| `topic.bestAnswer: TopicBestAnswer{topicId, floor, user, edited}` | `topic.bestAnswer: TopicBestAnswerSummary{floor, user, contentMarkdown, contentHtml, created}` | topic 详情 JSON-LD |
| `system_message.status: 'read'\|'unread'` | `system_message.isRead: boolean` | 通知中心 |
| `Search.SearchGalgames(isSFW)` | `Search.SearchGalgames(false)` —— 搜索强制 all | 搜索 BE 调用方 |

**审计命令**（任何 PR 后跑一次）：
```bash
# 旧字段残留扫描
grep -rn "\.released\b\|banner_image_hash\|bannerImageHash" apps/web --include="*.vue" --include="*.ts"
# 老 bestAnswer shape 残留
grep -rn "TopicBestAnswer\b" apps/web --include="*.vue" --include="*.ts"
```

---

## 11. JSON-LD 详细规格

只在以下 5 类页面注入。FE 通过 `useHead({ script: [{ id, type: 'application/ld+json', innerHTML }] })` 写入。

### 11.1 `/galgame/:gid` → `VideoGame`

```json
{
  "@context": "https://schema.org",
  "@type": "VideoGame",
  "name": "<游戏首选语言名>",
  "alternateName": ["<别名>", ...],
  "url": "<pageUrl>",
  "image": "<getEffectiveBanner(galgame)>",
  "description": "<markdown 前 175 字>",
  "inLanguage": "<originalLanguage>",
  "datePublished": "<created>",
  "dateModified": "<updated>",
  "publisher": [{"@type":"Organization","name":"<official.name>"}],
  "genre": ["<content 类别 tag.name>"],
  "keywords": "<technical 类别 tag.name>",
  "isPartOf": {
    "@type": "CreativeWorkSeries",
    "name": "<series.name>",
    "url": "/series/<series.id>"
  },
  "interactionStatistic": [
    {"@type":"InteractionCounter","interactionType":{"@type":"LikeAction"},"userInteractionCount":<likeCount>},
    {"@type":"InteractionCounter","interactionType":{"@type":"WatchAction"},"userInteractionCount":<view>}
  ],
  "author": {"@type":"Person","name":"<user.name>"},
  "contributor": [{"@type":"Person","name":"<contributor.name>"}, ...]
}
```

### 11.2 `/topic/:id` → `DiscussionForumPosting`

```json
{
  "@context": "https://schema.org",
  "@type": "DiscussionForumPosting",
  "mainEntityOfPage": "<topicUrl>",
  "headline": "<topic.title>",
  "description": "<contentMarkdown 前 233 字>",
  "image": "<getFirstImageSrc(contentHtml) | 默认 kungalgame.webp>",
  "author": {"@type":"Person","name":"<user.name>","url":"/user/<user.id>/info","image":"<user.avatar>"},
  "datePublished": "<created>",
  "dateModified": "<edited || created>",
  "interactionStatistic": [
    {"@type":"InteractionCounter","interactionType":{"@type":"CommentAction"},"userInteractionCount":<replyCount>},
    {"@type":"InteractionCounter","interactionType":{"@type":"LikeAction"},"userInteractionCount":<likeCount>},
    {"@type":"InteractionCounter","interactionType":{"@type":"VoteAction"},"userInteractionCount":<upvoteCount>}
  ],
  "commentCount": <replyCount>,
  "acceptedAnswer": {  // 仅当 BE topic.bestAnswer 存在
    "@type": "Comment",
    "text": "<bestAnswer.contentMarkdown 前 5000 字>",
    "datePublished": "<bestAnswer.created>",
    "url": "<topicUrl>#k<bestAnswer.floor>",
    "author": {"@type":"Person","name":"<bestAnswer.user.name>","url":"/user/<bestAnswer.user.id>/info","image":"<bestAnswer.user.avatar>"}
  },
  "keywords": "<section name + tag, 逗号分隔>"
}
```

### 11.3 `/galgame-rating/:id` → `Review`

```json
{
  "@context": "https://schema.org",
  "@type": "Review",
  "mainEntityOfPage": "<pageUrl>",
  "headline": "<user.name> 对 <galgameName> 的评价",
  "datePublished": "<created>",
  "dateModified": "<updated>",
  "author": {"@type":"Person","name":"<user.name>","url":"/user/<user.id>/info"},
  "publisher": {"@type":"Organization","name":"<kungal.title>","logo":{...}},
  "itemReviewed": {
    "@type": "VideoGame",
    "name": "<galgame name>",
    "url": "/galgame/<galgame.id>",
    "image": "<getEffectiveBanner(galgame)>",
    "inLanguage": "<originalLanguage>",
    "isFamilyFriendly": <ageLimit !== 'r18'>,
    "publisher": [{"@type":"Organization","name":"<official.name>"}],
    "isPartOf": {"@type":"CreativeWorkSeries","name":"<series.name>","url":"/series/<series.id>"}
  },
  "reviewRating": {"@type":"Rating","ratingValue":<overall>,"bestRating":10,"worstRating":1},
  "reviewBody": "<short_summary 前 250 字>",
  "interactionStatistic": [...],
  "additionalProperty": [
    {"@type":"PropertyValue","name":"艺术风格","value":<art>},
    {"@type":"PropertyValue","name":"故事情节","value":<story>},
    ...
  ]
}
```

### 11.4 `/toolset/:id` → `SoftwareApplication` (+ 可选 `DiscussionForumPosting`)

`SoftwareApplication`:
- `name` / `alternateName` / `url` / `description`
- `applicationCategory: <toolset.type>` / `operatingSystem: <mapped from platform>`
- `softwareVersion / inLanguage / datePublished/Modified`
- `author: Person` / `sameAs: homepage[:5]`
- `interactionStatistic: [WatchAction(view), DownloadAction(download)]`
- `aggregateRating: { ratingValue: practicalityAvg, ratingCount: practicalityCount, reviewCount: commentCount, bestRating: 5, worstRating: 1 }`（仅当 practicalityAvg && Count）

附加 `DiscussionForumPosting`（commentCount > 0）:
- `url / headline / articleBody / datePublished/Modified / author / commentCount`
- `comment: commentPreview[:n].map(...)`

### 11.5 `/website/:domain` → `Article` (含 about + reviews)

`Article`:
- `mainEntityOfPage / headline / description / image / datePublished/Modified`
- `author / publisher`
- `about: WebSite{ name, url, description, inLanguage, isFamilyFriendly, image: icon }`
- `keywords: category + tags 逗号串`
- `review: Review[]`（来自 website.comment 映射）

---

## 12. 维护清单（开发者参考）

每次新增页面时检查：

- [ ] 是否公开可访问？非公开（admin/auth）页面可考虑 `useKunDisableSeo`
- [ ] 远端 fetch 是否会失败？给所有失败路径加 `useKunDisableSeo('...')`
- [ ] 是否有 NSFW 可能性？`if/else` 分支处理，不要顺序调用
- [ ] 详情页是否值得 JSON-LD？参考 §6 五个示例
- [ ] 字段名是否对齐后端最新 DTO？参考 §10.4 命令扫描
- [ ] `useKunSeoMeta` 的 `data.value?.field` 在数据缺失时会渲染 `undefined` 吗？
  改成 `if (data.value) { useKunSeoMeta } else { useKunDisableSeo }`

每次后端重构（galgame_wiki 文档更新 / DTO 改动）：

- [ ] 跑 §10.4 字段漂移扫描命令
- [ ] 重新阅读本文档 §2 详情页表，核对每个 JSON-LD 引用字段还在
- [ ] 如发现字段重命名，本文档 §10.4 添加一行记录
