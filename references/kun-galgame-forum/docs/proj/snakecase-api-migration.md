# 论坛 API 全量 snake_case 迁移（2026-07-01 起）

> 本仓自有工程笔记。**决策(2026-07-01,owner)**:论坛前后端契约统一为 snake_case,
> 与 infra / wiki / moyu 及未来「鲲 Galgame」App / 开发者平台对齐,消除跨仓命名分歧。
> 前端直接使用后端 snake_case 字段名。
>
> **状态:完成(2026-07-02,含一次关键 CR 返工)。** 全 17 域 request+response 均已
> snake_case;`apps/api` 全量扫描 0 个 camelCase json/query tag;go build + vet + test
> + nuxt typecheck 全绿。
>
> **⚠️ CR 返工记录(commit 1cbd84e4):** 首次宣布「100%」是**错的**——response 侧 OK,
> 但 **request 侧发送点大面积仍是 camelCase**(我用了个在 `rg -A` 输出上锚点错误的 grep 做
> 校验 → 假「干净」)。3-agent CR 查出 ~40+ 处:camel key 发到 snake Go 绑定 / snake Zod
> schema → 静默 400(validate:required)/ Zod 客户端拦截 / no-op。**vue-tsc 看不见**(发送对象
> 无类型)。教训:**任何 wire 改名迁移,发送点校验绝不能靠 grep,必须逐个把 `kunFetch`
> body/query 的 key 对到 Go 绑定**;发送对象藏在 5+ 种形态里(inline 显式 / 简写 `{ fooBar }`
> / `query: computed(()=>({}))` / 具名 `const body={…}`(常被 Zod 校验→静默拦截)/ reactive
> `query: pageData` 展开)。另:Go 有两种绑定,FE 要求相反——struct-tag(snake)vs 原始
> `c.Query("commentId")`(FE 必须发 camel);少数发送点因此**正确保留 camel**。
>
> **仍分离的独立项(非 snake 范围)**:PR `note`→`title`/`message` 真 bug(见
> `openapi-types-forum.md`,note/message 都是单词,与 snake 无关)+ WikiPRDetailResponse
> 别名到生成 spec —— 二者仍待做,是独立任务。
>
> 备注:此前评估倾向「不做全量,改用 BFF 适配层 + oapi-codegen 契约校验」(见
> `openapi-types-forum.md`)。owner 选择全量对齐;本计划据此**以安全为先**落地。

## 现状:混合命名(这正是要消除的分歧)

论坛既有约定是「model/DB 层 snake_case、DTO/API 层 camelCase」,但并不统一——
部分域(如 `update` 的响应体 `content_en_us` / `completed_time` / `user_id`)**已是
snake_case**。全量迁移 = 把仍是 camelCase 的 DTO 字段拉齐到 snake_case。

嵌入的共享 user 对象在 Go(各域各自的 `UserBrief`)与 FE(`KunUser`,`shared/user.d.ts`)
均为 `id` / `name` / `avatar` ——**无 camelCase,不会跨域连锁**。这是全量可分域推进的前提。

### 待迁移清单(camelCase json tag,按域;`request`=带 `validate` 的入参 DTO)

| 域 | 合计 | 响应(FE 读) | 请求(FE 发) |
|----|-----|------------|------------|
| galgame | 119 | 105 | 14 |
| topic | 61 | 44 | 17 |
| doc | 43 | 31 | 12 |
| user | 37 | 37 | 0 |
| toolset | 34 | 28 | 6 |
| website | 32 | 19 | 13 |
| activity | 31 | 31 | 0 |
| message | 14 | 12 | 2 |
| search | 12 | 12 | 0 |
| home | 11 | 11 | 0 |
| section | 7 | 7 | 0 |
| admin | 7 | 7 | 0 |
| friendlink | 5 | 3 | 2 |
| ranking | 3 | 3 | 0 |
| update | 2 | 0 | 2 |
| rss | 2 | 2 | 0 |
| image | 1 | 1 | 0 |
| 合计 | ~421 | ~335 | ~86 |

（`galgame` 含 wiki 代理层;见「特殊处理」。）

## 安全网与其缺口(决定了整套程序)

- **响应字段:`vue-tsc` 是安全网。** 改 FE 类型字段名 → typecheck 对每处过时的
  `.camelCase` 访问报红 → 逐一改。`vue-tsc -b` **会检查模板表达式**,故 `.vue` 模板里的
  访问也会报红。把「无界 find-replace」变成「编译器给的清单」。
- **缺口 ①:动态访问。** `obj[key]`、`any` 类型、`JSON.parse`、对松散类型 `v-for` 的字段
  访问,typecheck **抓不到** → 每域**人工审计**(grep 中括号访问 / `any` / `parse`)。
- **缺口 ②:Go json tag 改名不触发 `go build` 失败**(tag 是字符串)——Go 侧**没有编译器
  安全网**。请求 DTO(`validate`)同理:FE 不改发送字段名也不会编译错。故 Go 改名**必须与
  FE 成对**,靠 typecheck / 运行时验证,不能只靠 `go build`。大域建议补 golden-JSON 线缆测试。
- **部署原子性:论坛 FE+BE 同步部署。** 每个域的 Go+FE 改动**必须同一提交、同批部署**;
  **绝不**上线半迁移的域(否则运行时字段对不上 → 静默空值 / 提交失败)。

## 每域程序

1. Go DTO 该域 camelCase json tag → snake_case(响应 + 请求都改)。
2. FE `shared/types` 对应字段名同步改 snake_case。
3. `pnpm -F web typecheck` → 修掉每一处报红(脚本 + 模板;含复用组件,如 topic 卡片被
   home/search/user 复用——typecheck 会跨目录指出,不必预先精确分区)。
4. 审计该域组件 / store / composable 的动态访问(缺口 ①):`rg "\[\s*['\"]"`、`: any`、`JSON.parse`。
5. 改请求发送处(`kunFetch` body / 校验 schema)为 snake_case 字段名(缺口 ②)。
6. 验证:`go build ./...` + `pnpm -F web typecheck` 绿 + **目标运行时抽验**(打开该页、跑一次表单提交)。
7. **原子提交**(Go+FE 同一 commit)。

## 顺序(风险递增,先证程序、再碰耦合)

1. **热身叶子**:`image`(1)、`rss`(2)、`ranking`(3)、`section`(7)、`admin`(7)——响应为主、组件少。
2. **纯响应中等**(0 请求,typecheck 全程守护):`home`(11)、`search`(12)、`activity`(31)、`user`(37)。
3. **含请求中等**:`friendlink`、`update`、`message`、`toolset`、`website`、`doc`——多一步改请求发送处。
4. **大且耦合,最后做**:`topic`(61)、`galgame`(119)。galgame 同时收口 wiki 代理层。

## 重要修正:共享「卡片」类型跨域耦合(2026-07-01 迁移中发现)

分域推进在 `home`/`search` 边界失效:FE 有**跨域共享的卡片类型**,一个类型被**多个后端端点**生产,必须整组一起翻:

- **topic 卡片形状** = `HomeTopic`(home.ts)。`SearchResultTopic = HomeTopic`(home+search 共用同一张卡 `home/topic/Card.vue`)。`activity` / `user` 主页的话题列表大概率也复用同型。
- **`GalgameCard`**(galgame.ts)= `HomeGalgame`,且被 galgame 列表域(119 字段,排最后)生产。故 `home` 的 galgame 卡**不能独立迁移**——它属于 galgame 卡片单元。

**结论:剩余工作按「共享类型」而非「后端域」编组**:
1. **topic-card 单元**:`HomeTopic`(home)+ `search.TopicItem` + activity/user 主页话题 +(最终 topic 域),所有生产者一起翻。
2. **galgame-card 单元**:`GalgameCard` 的全部生产者(home `HomeGalgame` + galgame 列表 + search galgame + user 主页 galgame + …),随 galgame 域一起翻。
3. **各域独有的非卡片字段**(search reply/comment 的 topic_id/topic_title、home user-status、activity 专有、user 主页标量)按域单独翻。

`ranking` 的 galgame 项是独立的 `RankingGalgameItem`(已完成),**不**属于 `GalgameCard`,故不受影响。

## 重要修正 2:编辑表单 = 请求/响应纠缠(website 迁移中发现，2026-07-01)

「仅响应」范围在**带编辑表单的域**上会卡住:表单/请求负载类型(常是 Zod `z.infer`，如 `WebsiteData`)与响应类型(`WebsiteDetail`)字段同名，编辑流程靠 `{...rest}` 把响应对象摊进表单对象——两者都是 camelCase 时可行。一旦响应转 snake、表单仍 camel(请求侧,本轮不动),该 `...rest` 摊平就类型不符,需要写 snake→camel 适配器。

**决策:带编辑表单纠缠的域,其编辑/请求路径 defer 到「请求侧统一 pass」**(那时表单 Zod schema + Go 请求 DTO + 发送站点一起转 snake,适配器消失)。纯响应读取路径(列表/详情展示)照常本轮做。

- **website 整体 defer**(edit modal `WebsiteData` Zod 表单 + Operation 的 `...rest` 适配)——已 revert，等请求侧 pass。
- 其它带编辑表单的域(doc/toolset 的 admin CRUD、topic/galgame 编辑器)同理:只做纯展示读取路径,编辑/发送路径随请求侧 pass。

## 特殊处理

- **wiki 代理层(galgame 阶段一并做)**:论坛转 snake_case 后,wiki 代理 DTO 可**大幅减少重映射**
  (输出接近逐字透传的 snake_case),但**保留服务端 user 水合**(嵌入 snake_case 的 `id/name/avatar`
  user 对象,`userclient.Hydrate`)。同阶段:① 修 `note`→`title`/`message` 的真 bug
  (见 `openapi-types-forum.md`);② 把逐字透传的 `WikiPRDetailResponse` 别名到生成的
  `PRDetailData`,使此类漂移编译期报错。
- **已是 snake 的字段**(`content_en_us` 等 i18n、`update` 响应体)**不动**。
- **生成物防漂移(Phase A 已就位)**:`galgame-{read,calendar}-api.ts` + `openapi-types.yml` 闸不受影响。

## 验证与回归

- 每域:`go build ./...` + `pnpm -F web typecheck` 必绿。
- 运行时抽验清单(随域补充):列表页渲染、详情页、表单提交(请求字段)、分页、搜索。
- 大域(topic/galgame):补 Go golden-JSON 线缆测试,弥补「json tag 改名不触发 go build 失败」的缺口。

## 代码审查（CR，2026-07-01，commit 68754082）

对已完成部分做了彻底 CR（3 个专项 reviewer + 手工 Go-wire↔FE-type 对齐审计）。结论:**已迁移各域端到端对齐正确**(所有 Go 生产者均已同步 snake;calendar/search-galgame 路径已覆盖;omitempty/validate/gorm 列均未动;鉴权/可见性字段无 desync——NSFW 服务端靠 SFW cookie 强制,非展示字段)。修复项:

- **真·线上回归(已修)**:sitemap 对每个 /topic、/galgame URL 丢了 `<lastmod>`。`kunSitemapSources.ts` 用 `Record<string,unknown>` 建模行(vue-tsc 盲区,即「缺口①」),改名后 `r.statusUpdateTime`/`r.resourceUpdateTime` 静默变 undefined。已修字段名 + 用 `Pick<TopicCard|GalgameCard,…>` cast 重新挂上类型安全网(下次改名 → 编译期报错)。**审计域时务必扫 `apps/web/server`,不止 `apps/web/app`。**
- **潜在陷阱(已修)**:死 FE 请求类型(ChatMessageHistoryRequest.receiverId、MessageRequestData.sortField/sortOrder)被 FE 转换器误 snake(FE 侧无 validate: 信号)——已还原 camelCase(须与 query 绑定一致)。
- **一致性**:search targetUser→target_user(死字段,对齐已迁移的 search 域)。
- **注释里的过期 camelCase** 已扫为 snake;**并还原了 2 处 comment-fixer 越界**(误改到未迁移类型的字段 GalgameDetail.releaseDate、TopicDetail.coverImageMeta——其 Go 生产者仍 camel,保持 camel)。

**教训**:① 批量注释改名脚本会误伤同文件内未迁移类型的**字段声明**(不止注释),必须 diff 校验非注释行;② vue-tsc 盲区 = server/ + lib/ + `Record<string,unknown>`/`any` 动态访问,每域都要单独扫。

## 进度

- [x] image — `size_bytes` / `variant_urls`（`UploadGalgameResult` + `uploadGalgameImage.ts` + spec）
- [x] rss — `TopicRSSItem` `user_id` / `user_name`（Nitro `topic.xml.ts`；galgame item 已用嵌入 user）
- [x] ranking — 响应 echo `sort_field`（`RankingUser/Topic/GalgameItem` + 3 组件）；`query:"sortField"` 请求参与 sort-option 配置的 `i.sortField` **保持不变**（本轮只动返回数据）
- [x] section — `SectionTopicItem`(like_count/reply_count/has_best_answer/is_nsfw_topic)+ `SectionStat`(topic_count/view_count/latest_topic);FE `section.ts`+`category.ts`+两个 Container.vue（含 /category 端点，自包含内联卡片）
- [x] admin — `UserContentStats` 7 字段（topic_comments…chat_messages）；FE `admin.ts` + `UserCard.vue`（含 `keyof` 配置数组，typecheck 守住动态 `stats[item.key]`）
- **topic-card 单元(完成)**：共享卡片形状 `HomeTopic`(home)+ `TopicItem`(search)+ `TopicCard`(topic) 一起翻 snake_case，含 `home/topic/Card.vue`(HomeTopicCard)、`topic/Card.vue`、`search/Reply|CommentCard.vue`。search `ReplyItem`/`CommentItem` 的 topic_id/topic_title 一并完成。`topic/Layout.vue` 复用 HomeTopicCard 的耦合随之消解。
- **galgame-card 单元(完成)**：共享卡片 `GalgameCard`(8 字段：content_limit/like_count/rating_count/is_on_forum/resource_update_time/release_date/release_date_tba/release_precision)。4 个 Go 生产者一起翻：`home.HomeGalgame`、`galgame.GalgameListCard`(search galgame 复用它)、`entity_dto` 卡片、`user.UserGalgameCard`。消费者：`galgame/card/Card.vue`(10 读)、`calendar/Month(List).vue`、`useGalgameReleaseToday.ts`。**排除**：`GalgameActivityData`(activity 自有形状 coverHash/ageLimit)、galgame detail/rating/resource embed(各自单元)、`wiki_dto` release_precision(本已 snake)。
- [x] home — HomeTopic + HomeGalgame 均完成；仅 HomeUserStatus(isCheckIn/hasNewMessage) 源在 user 域(auth_dto)→随 user
- [x] search — TopicItem/ReplyItem/CommentItem + galgame(复用 GalgameListCard)全部完成；UserItem 无 camelCase
- [x] activity — 全 activity_dto 响应结构(ActivityItem/TopicActivityData/GalgameActivityData/RatingInfo/…) + activity.ts；脚本化 json-tag/字段改名(acronym-aware)+ typecheck 驱动 ~15 卡片组件 + home/Container；68 读全绿
- [~] user — UserGalgameCard(主页 galgame 卡)已完成（galgame-card 单元）；auth_dto(当前用户/HomeUserStatus)、主页话题/评分/资源等其余待做
- [x] friendlink — model.FriendLink 响应 json tag(banner_image_hash/banner_url/sort_order）+ FE FriendLink 响应类型 + 展示读取(Container/Section)+ Modal 的 1 处 response→form 映射；FriendLinkInput 请求表单保持 camelCase(正确)
- [ ] update
- [x] message — chat_dto/message_dto 响应结构 + chat-message.ts/message.ts；转换器**跳过 validate: 请求行**(receiverId/messageId 请求参保持 camelCase，send 站点一致)；typecheck 驱动 message 组件 + 2 构造/页面 straggler
- [ ] toolset — 纯展示读取路径本轮可做；create/edit 表单随请求侧 pass
- [defer] website — edit 表单(WebsiteData Zod)请求/响应纠缠，整体 defer 到请求侧统一 pass（见「重要修正 2」）
- [ ] doc — 展示读取路径本轮可做；admin CRUD 表单随请求侧 pass
- [~] topic — `TopicCard`（列表卡片）已完成（topic-card 单元）；topic 详情/回复/评论/投票等其余字段待做
- [~] galgame — `GalgameListCard`/`entity_dto` 卡片已完成（galgame-card 单元）；galgame 详情(GalgameDetail)/评分/资源/评论 + wiki 代理收口 + note→message 修复 + WikiPRDetailResponse 别名 待做
