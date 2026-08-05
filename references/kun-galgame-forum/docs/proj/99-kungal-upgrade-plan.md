# Kungal 升级方案（与 wiki v1 升级配套）

> 状态：**待审定**。本文件是 `99-final-upgrade-plan.md`（galgame wiki 端升级方案）的下游配套文档。
> 适用范围：kungal 后端（`apps/api`，Go Fiber）+ Nuxt 前端（`apps/web`）。
> 自包含：阅读本文 + `99-final-upgrade-plan.md` + 本仓库代码即可完成实施，无需查阅任何外部项目。
>
> 实施前阅读次序：先读 `99-final-upgrade-plan.md` §1-§8（理解 wiki 契约变更）→ 再读本文 §1 对接定位 → §3-§5 三项主升级 → §6 内部改进 → §8 实施顺序。

---

## 1. 对接定位与原则

### 1.1 我们在三服务架构中的角色
- **galgame wiki**：元数据 SoT，拥有所有 schema、修订系统、apply-snapshot 写路径。
- **kungal**(`apps/api`)：HTTP 代理层 + 本站本地数据（评分/评论/资源等）+ Bearer token 转发。**不持有 galgame 元数据**，不复刻 wiki 的修订系统。
- **前端**(`apps/web`)：消费方，所有 galgame 元数据从 wiki 间接读取。

### 1.2 升级原则（每条决策都要套用）
1. **不复刻 wiki 的写路径**：所有 galgame/taxonomy 写入仍是"前端→kungal→wiki"逐字透传(`ProxyWriteWithToken`)。kungal 只做路由 + 透传 + 极少量响应增强（如 `rewriteBanners`）。
2. **wire 字段名严格对齐 wiki**：kungal `ProxyWrite` 不改字段名；前端 payload key 必须 1:1 匹配 wiki DTO。任何 wiki 改名等同前端改名（kungal 不做翻译层）。
3. **替换全部语义(presence semantics)是 wire 级强制约定**：任何含 ID 数组/对象数组的 PUT 字段（`tag_ids/official_ids/engine_ids/covers/screenshots/aliases(PR 端点)`），前端**必须完整预填+整集回传**；不传=保持不变。这条约定下游不可放宽不可收紧。
4. **breaking 与 additive 区分对待**：U1 是 breaking、与 wiki 同期发版；U2/U3 是 additive、可在 wiki 上线后跟进，不阻断。
5. **测试护栏先行**：每条 wire 契约变更必须配 wire-shape 单测，防止下次 wiki 改 shape 时静默失效。

### 1.3 wiki 升级对我们的影响速览

| Wiki 升级 | 性质 | 我们必改面 |
|---|---|---|
| **U1** `released` → `release_date`+`release_date_tba` | **BREAKING** | kungal DTO + 前端 type + 校验 schema + 显示/编辑/diff label-map |
| **U2** `galgame_cover[]` + `galgame_screenshot[]` + `effective_banner_hash`（`banner_image_hash` 过渡期保留） | additive，过渡期长 | kungal banner 解析扩展、前端编辑表单含图集编辑、详情页画廊、SnapshotDiff 处理数组字段、PR 全量回传 |
| **U3** 单张 `taxonomy_revision` + tag/official/engine/series 全 history+revert | additive | kungal 加 4 实体的 `revisions[/​:rev]` + `revert` 代理路由；前端 4 实体加 History UI（复用现有 galgame History 模式） |

---

## 2. 必读约定（下游不可违反）

### 2.1 wire 字段名（snake_case）一律与 wiki 对齐
- 新字段：`release_date`(`string|null`，`"YYYY-MM-DD"`)、`release_date_tba`(`bool`)、`covers`(`object[]`)、`screenshots`(`object[]`)、`effective_banner_hash`(`string` 派生只读)。
- 移除字段（PR5 之后）：`released`(string)、`banner`(legacy URL)、`banner_image_hash`(顶层)。

### 2.2 presence 全量替换（PUT/PR payload）
- **不传**该字段 → wiki 保持不变。
- **传数组（含空 `[]`）** → wiki 按它"清空旧关联→按此重建"。`[]` = 显式清空。
- 因此前端编辑表单**必须回传该 galgame 的当前全量** `tag_ids/official_ids/engine_ids/covers/screenshots`。**不要只回传"新增的那几个"**——会被当成"替换成只剩这几个"。
- 这条约定**已在 `pr/Footer.vue` + `Rewrite.vue` 水合中落实**(`tag/official/engine` 已完整预填)，U2 上线后需对 `covers/screenshots` 同款处理。

### 2.3 kungal 代理透传不动 body
- `internal/galgame/handler/wiki_handler.go` 的 `ProxyWriteWithToken` 用 `c.Body()` + `c.Get("Content-Type")` + `collectQuery(c)` 逐字透传。新增字段无需 kungal 改代码，自动透传。
- `service/wiki_service.go::ProxyWrite` 已支持 query 转发（用于 `?force=true` 两段式删除），同款机制处理未来其他 query。

---

## 3. U1 实施：`released` → `release_date` + `release_date_tba`（BREAKING）

### 3.1 影响面盘点（实施时按图索骥）

**kungal 后端**：
- `apps/api/internal/galgame/dto/galgame_dto.go` / `entity_dto.go` / `wiki_dto.go` / `resource_dto.go` / `rating_dto.go`：所有含 galgame 元数据的 DTO 中如有 `Released string` 字段 → 替换为 `ReleaseDate *string` + `ReleaseDateTBA bool`，JSON tag 用 snake_case。
- `apps/api/internal/galgame/client/client.go::GalgameBrief`：同上。
- `apps/api/internal/galgame/service/galgame_mapper.go::galgameDetailFromWiki` 及同包其他 mapper：删除 `Released:` 拷贝，新增两字段拷贝。

**前端**：
- `apps/web/shared/types/galgame.ts`：`GalgameDetail` / `GalgameCard` / 其它含元数据的接口中如有 `released` → 替换为 `release_date?: string | null` + `release_date_tba?: boolean`。
- `apps/web/app/validations/galgame.ts`：`createGalgameSchema` / `submitGalgameSchema` / `patchDraftSchema` / `updateGalgameSchema` 如校验过 `released` → 改两字段；否则若需要让前端表单可编辑，按下方 §3.2 加入。
- 校验语义：`release_date: z.string().refine(v=>v===''||/^\d{4}-\d{2}-\d{2}$/.test(v)).optional()`（与 `patchDraftSchema.vndb_id` 同款"空串或合法格式"）；`release_date_tba: z.boolean().optional()`。
- `apps/web/app/store/types/edit/galgame.ts::GalgameEditStoreTemp`：加 `releaseDate: string | null`（`""` 代表 unknown）+ `releaseDateTBA: boolean`。
- `apps/web/app/components/galgame/Rewrite.vue`：水合时从 `galgame.release_date` / `galgame.release_date_tba` 灌入。
- `apps/web/app/components/edit/galgame/Draft.vue`：构造 store 项时含两字段默认值。
- `apps/web/app/components/edit/galgame/Meta.vue`（创建/提交流程的"游戏基础信息"区）：新增"发售日期"`KunInput type="date"` + "未定(TBA)"`KunSwitch`。
- `apps/web/app/components/edit/galgame/pr/PullRequest.vue` 分级/元数据面板：同上。
- `apps/web/app/components/edit/galgame/pr/Footer.vue::handlePublishGalgamePR` 的 `data` 对象：加 `release_date: galgame.releaseDate`、`release_date_tba: galgame.releaseDateTBA`。
- `apps/web/app/components/edit/galgame/Footer.vue`（创建/submit Footer）：同上。
- 详情页 `apps/web/app/components/galgame/Info.vue` / `Header.vue`（如显示发售日）：渲染时优先 `release_date`，TBA=true 显示"未定"，否则空显示"未公布"。
- SnapshotDiff label map：`apps/web/app/constants/galgame.ts::KUN_GALGAME_RESOURCE_PULL_REQUEST_I18N_FIELD_MAP` 删 `released` 键、增 `release_date: '发售日期'`、`release_date_tba: '发售日 TBA'`。
- 旧 `released` 显示位（如 GalgameCard 角标、列表筛选）：全部替换。grep `"released"` / `\.released` 收尾。

### 3.2 决策点

| 项 | 决策 | 理由 |
|---|---|---|
| 前端是否暴露 release_date 编辑字段 | **暴露** | wiki 端可编辑、有 revision；前端不暴露 = 用户改不了 = 功能不完整 |
| TBA 与 release_date 互斥？ | **不互斥**（与 wiki 一致） | TBA=true 仍可有日期（"预计 2024 年某月"语义） |
| 历史数据兼容（前端容错） | **临时双读** | 过渡期 wiki 可能返回新+旧两套字段；显示层优先新字段，缺失时尝试 `released` 字符串解析。**仅过渡期保留 ≤2 个 release**，之后 grep 清掉 |

### 3.3 实施提醒
- **同期发版**：U1 是 wire breaking；kungal/前端必须与 wiki 同一个 release 上线，错峰会立刻 400/字段缺失。
- **本地开发数据库可能滞后**：迁移脚本在 wiki 端跑；本地拉到的 wiki 容器若是新版，旧 dump 数据需重跑迁移。

---

## 4. U2 实施：图片模型（covers / screenshots / effective_banner_hash）

### 4.1 wire 形态（必读，决定下面所有代码）

新 wire 字段（snake_case）：

```jsonc
// GET /galgame/:gid 响应里新增
"covers": [
  { "image_hash": "abcd...", "sort_order": 0, "sexual": 0, "violence": 0,
    "source": "", "source_key": "" }
],
"screenshots": [
  { "image_hash": "...", "sort_order": 1, "caption": "OP 立绘",
    "sexual": 0, "violence": 0, "source": "", "source_key": "" }
],
"effective_banner_hash": "abcd..."  // 派生只读：sort_order=0 的 cover.image_hash
```

过渡期：`banner_image_hash` 顶层字段保留；终态(PR5)删除（同时新数据已迁入 `covers[sort_order=0]`）。

### 4.2 kungal 后端改动

#### 4.2.1 DTO 扩展（`apps/api/internal/galgame/dto/`）

`GalgameDetail` / `GalgameBrief`（及 wiki_dto.go 对应 wiki 解码结构）新增字段：

```go
EffectiveBannerHash string             `json:"effective_banner_hash"`
Covers              []GalgameCover     `json:"covers"`
Screenshots         []GalgameScreenshot `json:"screenshots"`

type GalgameCover struct {
    ImageHash string `json:"image_hash"`
    SortOrder int    `json:"sort_order"`
    Sexual    int    `json:"sexual"`
    Violence  int    `json:"violence"`
    Source    string `json:"source"`
    SourceKey string `json:"source_key"`
    BannerURL string `json:"banner_url,omitempty"` // 派生 CDN URL（由 rewriteBanners 填）
}

type GalgameScreenshot struct {
    // 同 Cover + Caption string `json:"caption"`
    // + BannerURL → 改名 ImageURL string `json:"image_url,omitempty"`
}
```

mapper（`service/galgame_mapper.go::galgameDetailFromWiki`）拷贝这些字段。

#### 4.2.2 关键扩展：`rewriteBanners` 升级为全 hash → URL 解析器

现状：`apps/api/internal/galgame/client/banner.go::rewriteBanners` 递归遍历 wiki 响应 JSON，对任何 `banner_image_hash` 非空且 `banner` 为空的对象，把 `banner` 填为 `{cdnBase}/{hh}/{hh}/{hash}.webp`（通过 `doRequest` 流转,所有 wiki 响应都过这一层）。

升级要做的：在同一个递归 walker 里**追加两类规则**：

1. **`covers` / `screenshots` 数组中每一项的 `image_hash`** → 派生填入 `banner_url`(cover)/`image_url`(screenshot)。
2. **顶层 `effective_banner_hash`** → 同时填一个 `effective_banner_url`(派生 URL)。

这样前端拿到的所有图片相关字段都是即用 URL，无需在多个组件里重复拼。

实现要点：
- 复用现有的 `bannerURLFromHash(cdnBase, hash)` 纯字符串函数。
- walker 命中 cover/screenshot 项的判定：对象同时含 `image_hash`(非空 string) → 注入 `banner_url`(用 key 区分 cover vs screenshot 需要看外层 key，简化方案：统一注入键名 `cdn_url`，前端按上下文用)。**推荐用统一 `cdn_url` 键**，避免 walker 还要知道父键名。
- 走过 `revision.snapshot` / `pr.snapshot` 时也命中（snapshot 内同款结构），自动让 diff 渲染时也有可用 URL。
- 同样使用 `json.Number` 保数字精度（已有）。
- 若 `cdnBase==""`(降级)整段跳过(已有)。
- **不破坏现有 banner_image_hash 行为**：现有规则与新增规则并行,过渡期同时工作。

实施文件：仅修 `apps/api/internal/galgame/client/banner.go` 一处；测试在同包 `banner_test.go` 加 case。

#### 4.2.3 路由 / handler 不动

`PUT /galgame/:gid` 和 `POST /galgame/:gid/prs` 的 body 是逐字透传，`covers`/`screenshots` 字段自动随 body 抵达 wiki。kungal **不加** body 解析/校验逻辑(否则就违反"不复刻 wiki 写路径"原则)。

### 4.3 前端改动

#### 4.3.1 类型

`apps/web/shared/types/galgame.ts`：
```ts
export interface GalgameCover {
  image_hash: string
  sort_order: number
  sexual: number
  violence: number
  source: string
  source_key: string
  cdn_url?: string        // kungal rewriteBanners 派生
}
export interface GalgameScreenshot extends GalgameCover {
  caption: string
}
export interface GalgameDetail {
  // ... 现有
  effective_banner_hash?: string
  effective_banner_url?: string
  covers: GalgameCover[]
  screenshots: GalgameScreenshot[]
  // banner_image_hash 字段过渡期保留(同 PR5 之前)
}
```

#### 4.3.2 编辑 store 与水合

`apps/web/app/store/types/edit/galgame.ts::GalgameEditStoreTemp` 加：
```ts
covers: GalgameCover[]
screenshots: GalgameScreenshot[]
```
`Rewrite.vue` 水合：`covers: [...galgame.covers]`、`screenshots: [...galgame.screenshots]`(**必须完整深拷贝**，否则编辑会改原对象)。

`Draft.vue` 构造时给 `covers: []` / `screenshots: []`（草稿 PATCH 不处理图集，与现 alias 同款）。

#### 4.3.3 编辑表单：新增图集编辑器

新建 `apps/web/app/components/edit/galgame/pr/Covers.vue` + `Screenshots.vue`（与 `pr/Links.vue` 同款行编辑器风格）：
- 列表 + 每行：image 预览（点开 modal 上传/换图，复用 `EditGalgameBanner` 的 image_service 路径）、`sort_order` 数字（cover 用,screenshot 用拖拽？v1 简化为数字字段）、`sexual/violence` `KunSelect`（取值见下方决策表）、删除按钮。
- "新增"按钮 → 上传新图 → 取得 hash → push 一行。
- screenshot 多一个 `caption` `KunInput`。

挂载点：`apps/web/app/components/edit/galgame/pr/PullRequest.vue` 的"分级/封面/高级"标签页中,把现有单一 `<EditGalgameBanner/>` 替换/扩展为 cover 编辑器；新增"画廊"区段或独立标签页放 screenshot 编辑器。

#### 4.3.4 Footer payload

`pr/Footer.vue::handlePublishGalgamePR` 的 `data` 对象加：
```ts
covers: galgame.covers,            // presence 全量替换；已完整水合
screenshots: galgame.screenshots,
```
schema (`updateGalgameSchema`) 加对应字段校验（数组、每项最少 `image_hash`、`sort_order` int、`sexual/violence` 枚举范围）。

#### 4.3.5 详情页展示

- `apps/web/app/components/galgame/Header.vue` 渲染 banner：优先 `effective_banner_url`（kungal 派生），缺失 fallback `banner_image_hash` 经现有 banner 解析（过渡期），最后 fallback legacy `banner`。**写一个 `getEffectiveBanner(galgame)` util**（`apps/web/shared/utils/`），所有显示点统一调用，避免 fallback 链散落。
- 详情页新增"画廊"`apps/web/app/components/galgame/Gallery.vue`：渲染 `galgame.screenshots`(按 `sort_order` 排序)，点击放大；如为空隐藏。

#### 4.3.6 SnapshotDiff 数组字段渲染

`apps/web/app/components/galgame/SnapshotDiff.vue` 当前对 `tag_ids`/`links` 等数组字段是 `JSON.stringify` 后做字符 LCS——读不出"加了哪张图、改了哪张的 sort_order"。配合本次升级,**做结构化数组 diff**：

- 引入轻量依赖 `microdiff` (`pnpm add microdiff`) 或自实现 ~40 行的 `arrayDiffByKey(old, new, keyFn)` 工具。
- `SnapshotDiff.vue` 内部分类:
  - 标量(字符串/数字/布尔)：仍用 `useDiff` LCS。
  - **对象数组(covers/screenshots/links)**：按 `image_hash`/`name+link` 配键,产出 `+added / −removed / ~changed{field: old→new}`。
  - 标量数组(`tag_ids`/`aliases`)：按值集合差产出 `+added/−removed`。
- 字段标签`KUN_GALGAME_RESOURCE_PULL_REQUEST_I18N_FIELD_MAP` 加 `covers/screenshots` 键。

### 4.4 决策点

| 项 | 决策 | 理由 |
|---|---|---|
| 替换 banner 上传方案 | **替换为 cover 编辑器** | 旧 banner 是 cover[0] 的特例；保留旧入口反而混乱 |
| sexual/violence 取值 UI | **v1 仅提供 0(未评定)/1(低)/2(中)/3(高)** | wiki schema 是 `int16`，未定细则；UI 留 KunSelect 占位 + 文档注明"当前不影响展示门控,产品后续可启用" |
| 数据迁移期间双显示 banner | **`getEffectiveBanner` util 兼容三级 fallback** | `effective_banner_url > banner_image_hash > banner`；PR5 后只保第一级 |
| screenshot 拖拽排序 | **v1 不做** | 用数字 `sort_order` 字段；产品验证有用再做拖拽 |
| 是否在 kungal 校验 covers/screenshots payload | **不校验** | wiki 才是 SoT；kungal 校验会双重维护 |
| 引入 `microdiff` 依赖 | **优先自实现 `arrayDiffByKey`** | 仅 ~40 行，避免新依赖；如发现复杂度爆炸再切 microdiff |

### 4.5 PR5 收尾（wiki 删 `banner_image_hash` 列后）

- 删除 kungal DTO 中的 `BannerImageHash` 字段（同步删 wiki dto 解码）。
- 前端 `getEffectiveBanner` 移除 fallback 链中间档,仅保 `effective_banner_url`。
- 旧编辑入口/旧 schema 字段 grep 清理。

---

## 5. U3 实施：Taxonomy 修订（tag/official/engine/series 各自 history + revert）

### 5.1 新 wire 端点（wiki 提供）

```
POST   /galgame-{tag|official|engine|series}                     # 创建（已存在,不变）
PUT    /galgame-{tag|official|engine|series}                     # 更新（已存在,但现在落 taxonomy_revision）
DELETE /galgame-{tag|official|engine|series}/:id?force=...       # 两段式删除（已存在,但现在落 deleted revision + galgame_revision 给受影响 galgame）
GET    /galgame-{tag|official|engine|series}/:id/references      # 已存在
GET    /galgame-{tag|official|engine|series}/:id/revisions       # 新增：列表
GET    /galgame-{tag|official|engine|series}/:id/revisions/:rev  # 新增：单条快照
POST   /galgame-{tag|official|engine|series}/:id/revert          # 新增：回滚 {revision: N}
```

> **打开问题(实施前需与 wiki 团队确认)**：见本文 §10 #1。

### 5.2 kungal 路由

新增 `apps/api/internal/app/router.go`：

```go
// 公共 GET
for _, entity := range []string{"tag","official","engine","series"} {
    api.Get(fmt.Sprintf("/galgame-%s/:id/revisions", entity),       a.GalgameWikiHandler.ProxyGet)
    api.Get(fmt.Sprintf("/galgame-%s/:id/revisions/:rev", entity),  a.GalgameWikiHandler.ProxyGet)
}
// 写：revert 需 Bearer
for _, entity := range []string{"tag","official","engine","series"} {
    authed.Post(fmt.Sprintf("/galgame-%s/:id/revert", entity),
        a.GalgameWikiHandler.ProxyWriteWithToken("POST"))
}
```

**路径映射(已决)**: wiki 采用 `/galgame/<entity>/` 新命名空间(避免与 topic tag 等概念歧义)。需在 `client/path_mapper.go` 新增 **suffix-aware** 规则: 网关路径形如 `/galgame-<entity>/:id/(revisions|revisions/:rev|revert)` 且 `<entity>` ∈ {tag,official,engine,series} → 映射为 `/galgame/<entity>/:id/...`; 其它原有路径继续走 `/galgame-tag → /tag` 等单词映射。配套 `path_mapper_test.go` 必须含正例 3 条(revisions/revisions:rev/revert)+ 回归断言 2 条(`/galgame-tag/search` 仍 → `/tag/search`、`/galgame-tag/5` 仍 → `/tag/5`)。详见 §10 #1。

### 5.3 前端 UI

`apps/web/app/components/galgame/history/History.vue` 当前实现是 galgame 实体专用(从 `route.params.gid` 取 ID + `/galgame/:gid/history/all` 套件)。**重构为参数化通用组件**或**抽出共用钩子 `useEntityRevisionHistory(entity, id)`**,然后 4 个 taxonomy 实体的详情页各自挂一个 History 区段。

**推荐结构**：

```
app/components/galgame/revision/
├── List.vue         # 通用列表：列出 revisions[],每条懒加载 diff,创建者/admin 显示"回滚"
├── DiffPanel.vue    # 调 .../revisions/:rev/diff(或单 snapshot)拉数据 → 用 GalgameSnapshotDiff 渲染
└── useRevisionHistory.ts  # 组合式: entity 名 + id → fetch 列表 + 取单条 + revert 调用
```

挂载：
- `apps/web/app/components/galgame/tag/DetailContainer.vue` 末尾加 `<GalgameRevisionList entity="tag" :id="tagId" />`。
- `apps/web/app/pages/galgame-official/[id].vue` 同款。
- `apps/web/app/pages/galgame-engine/[id].vue` 同款。
- 系列(series)详情页同款(若有)。
- galgame 自身的 `History.vue` 改为复用此组件: `<GalgameRevisionList entity="galgame" :id="gid" />`。

权限：调用方传入 `canRevert: boolean`(创建者或 `role >= 2`)；通用组件根据它显示/隐藏回滚按钮。

### 5.4 SnapshotDiff 跨实体复用

taxonomy snapshot 形态：tag/official/engine 都是"标量 + aliases[]"; series 是"纯标量"。**`GalgameSnapshotDiff.vue` 当前接受 `changedKeys + oldSnap + newSnap`,与实体无关,可直接复用**。仅需补 label map：

`apps/web/app/constants/galgame.ts::KUN_GALGAME_RESOURCE_PULL_REQUEST_I18N_FIELD_MAP` 加 taxonomy 各实体的字段标签:
- `name: '名称'`(已存在用于 galgame name_zh_cn 等,如 key 冲突则用全限定 key 如 `tag.name`，**建议**改 map 结构为 `{[entity]: {[field]: label}}` 二级查找,但工作量大；v1 简化版:复用顶层 map+字段名足够区分,因为字段名集合在 taxonomy 与 galgame 间几乎不重合)。

### 5.5 决策点

| 项 | 决策 | 理由 |
|---|---|---|
| History 组件参数化 vs 复制 4 份 | **参数化** | 4 实体形态高度同构;复制 4 份是维护负债 |
| 是否给 series 加专门历史 UI | **同款一套** | wiki 已统一 taxonomy_revision; 不一致没意义 |
| Revert UI 是否提供 dry-run | **不提供** | wiki 端无 dry-run 端点; 我们前端二次确认+刷新已足 |
| Taxonomy 删除事件在哪展示 | **history 列表显示 `deleted` action** | wiki 已落 deleted revision; 同时若用户访问该实体详情页(已被删) 会 404; 列表入口可在用户主页"我的编辑"加 deleted 项展示 |
| **撤销删除 UI** | **v1 仅做"恢复实体"基础版** | wiki 已落 `affected_galgame_ids`; 但"批量恢复关联"UI 是大特性,v1 仅提供"revert deleted revision → 实体复活,关联手动恢复"提示,不做勾选回填 UI |

---

## 6. 横切内部改进（与 wiki 升级无关但顺手做）

### 6.1 SnapshotDiff 数组结构化 diff（P0,见 §4.3.6）
- 与 U2 关联紧密但**已决: K-PR4 单独串行,不与 K-PR2 合并**(见 §10 #5)。项目测试期 + 历史 PR 已清,无兼容压力; K-PR2 上线后 K-PR4 之前,新产生的含图集改动 PR 在 diff 区临时显示为 JSON 字符串 diff(可读性差但不错乱)。
- 工作量: ~80 行新 util(`arrayDiffByKey`) + ~30 行 `SnapshotDiff.vue` 改造 + 配套 vitest spec。
- 同 PR 顺带建立前端测试基础设施(vitest + @nuxt/test-utils + happy-dom)。

### 6.2 关键编辑链路单元测试补齐（P0）
当前编辑链路改动密集且零测试护栏，按本次升级风险面补：

| 测试位 | 覆盖点 |
|---|---|
| `apps/api/internal/galgame/client/banner_test.go` | 现有 case + `rewriteBanners` 对 covers/screenshots/effective_banner_hash 的注入 |
| `apps/api/internal/galgame/client/path_mapper_test.go` (新增) | `ToWikiPath` 对 taxonomy revision 路径映射 |
| `apps/web/app/components/galgame/SnapshotDiff.spec.ts` (新增) | 数组 diff 各种 case；空 changed_keys → KunNull |
| `apps/web/app/components/edit/galgame/pr/Footer.spec.ts` (新增) | direct 分支 PUT + reconcile alias/link；PR 分支 POST；payload 字段名 |
| `apps/web/app/components/galgame/Rewrite.spec.ts` (新增) | 完整水合(含 covers/screenshots/aliases) + ?type=pr 跳转 |
| `apps/web/shared/utils/getEffectiveBanner.spec.ts` (新增) | 三级 fallback 链 |

栈: Go 用现有 `testing`; 前端**引入 vitest**(Nuxt 与 vitest 深度集成,`@nuxt/test-utils` + `happy-dom`),配置工作归入 K-PR4 一并完成。详见 §10 #2。

### 6.3 res DTO 契约固化（P2,可缓）
近期踩过的坑(PR detail shape 从 `oldData/newData` 改为 `{pr.snapshot, changed_keys}` → 前端静默失效):wiki 改 wire shape 时 kungal 透传不报错,前端默默坏。

**轻量缓解**:
- 关键透传端点的响应在 kungal 用 schema 做"shape sanity check"(只检顶层关键字段存在,不做完整校验),失败 log warn 不阻断。
- 在 `apps/api/internal/galgame/client/banner_test.go` 同侧增加"wire shape regression"单测,用真实 wiki sample fixture 跑 walker。

**v1 不做,登记为后续小优化。**

### 6.4 显式不做（避免过度工程）

| 项 | 不做原因 |
|---|---|
| 复刻 wiki 修订系统到 kungal | 违反三服务分工; wiki 才是 SoT |
| 在 kungal 加字段级 RBAC | wiki 端已留扩展位; 真启用时下游按 wiki 返回的策略走 |
| 引入 microdiff 第三方依赖 | 自实现 `arrayDiffByKey` ~40 行足够; 不增依赖 |
| 多搜索引擎/搜索联想/趋势词 | 已用 Meilisearch; YAGNI |
| Bull/异步审核流水线 | wiki 自有审核队列; 三服务下不适配 |
| 多源数据获取(VNDB 外) | wiki 已统一同步; kungal 不参与 |
| 角色实体/作品关系/walkthrough | wiki 端未做; 跟随 wiki 节奏 |
| OG 图独立服务 | Nuxt 自有 SEO 方案够用 |
| 热度算法 | 产品决策, 不在本次升级 |
| 增量关系动作(add/remove 单 tag) | wiki 是 presence 全量语义; 前端用 §2.2 约定即可 |

---

## 7. 风险与回滚

### 7.1 风险

| 风险 | 概率 | 缓解 |
|---|---|---|
| U1 同期发版漏 grep 个角落显示 `released` | 中 | §3.1 影响面盘点 + grep `released\|\.released\|"released"` |
| U2 编辑表单 covers 未完整水合 → PUT 后丢图 | **高** | §2.2 约定 + Rewrite.vue 水合处加单测; 提交前二次确认弹窗已警示"整组替换" |
| U2 image_service 配额/上传失败的画廊场景 | 中 | 复用现有 banner 上传流程, image_service 同款错误码透传 |
| U3 路径映射不命中 → 404 | 中 | §10 #1 实施前与 wiki 确认 + `path_mapper_test.go` 覆盖 |
| SnapshotDiff 结构化 diff 引入 bug | 中 | 与 U2 同 PR + 配套 spec; 不破坏原 LCS 路径(标量仍走 LCS) |
| 过渡期 banner 三级 fallback 显示错版本 | 低 | `getEffectiveBanner` util 单测覆盖三级链 |

### 7.2 回滚策略

- U1: 不可回滚(breaking)。若 wiki 必须紧急回滚则 kungal/前端也回滚同一版本。
- U2: PR2 上线后若发现严重问题, 因 `banner_image_hash` 仍保留,可前端 `getEffectiveBanner` 降级仅用旧字段,等修复后再启用 covers 显示。编辑入口可单独 feature flag 关闭。
- U3: 仅新增端点 + 新增 UI 入口; 直接前端隐藏入口即"逻辑回滚",不影响其他功能。

---

## 8. 实施顺序（建议 PR 切分）

| PR | 范围 | 依赖 | 同期 wiki PR |
|---|---|---|---|
| **K-PR1** | U1: `released→release_date+tba` 全链路（DTO + 类型 + 校验 + 显示 + 编辑表单 + label map） | 无 | wiki PR1 同期发版 |
| **K-PR2** | U2.a: `rewriteBanners` 扩展为全 hash 解析；DTO/类型加 `covers/screenshots/effective_banner_hash`；详情页画廊只读展示；`getEffectiveBanner` util + fallback；`KUN_..._FIELD_MAP` 加键 | 无 | wiki PR2 同期或之后 |
| **K-PR3** | U2.b: 编辑表单图集编辑器(`pr/Covers.vue` + `Screenshots.vue`)；Footer payload 含 covers/screenshots；schema 校验；Rewrite 水合 | K-PR2 | wiki PR2 上线稳定后 |
| **K-PR4** | **SnapshotDiff 数组结构化 diff** + 关键链路单测(§6.1 + §6.2) | K-PR2 | 与 K-PR2/3 紧绑 |
| **K-PR5** | U3: 4 个 taxonomy 实体 `revisions[/​:rev]` + `revert` 代理路由；通用 `GalgameRevisionList` 组件 + 4 实体详情页挂载；`History.vue` 复用化 | 无 | wiki PR4 同期或之后 |
| **K-PR6（验证稳定后）** | U2.c: 删 `banner_image_hash` DTO 字段 + `getEffectiveBanner` fallback 链收尾 | K-PR3 上线 ≥ 2 周 | wiki PR5 同期 |

K-PR1 与 K-PR2/K-PR5 可并行; K-PR2 → K-PR3 → K-PR6 串行; K-PR4 与 K-PR2/3 紧绑。

---

## 9. 防回归测试清单

每条标"新增"对应 PR 必加。测试位见 §6.2。

### U1
- 新增：kungal mapper test: wiki sample with `release_date+tba` → DTO 正确填两字段
- 新增：前端 schema test: `release_date` 接受 `"" | "YYYY-MM-DD"`; 其他形态拒绝
- 新增：显示组件 test: TBA=true 显示"未定"；`release_date` 缺失显示"未公布"

### U2
- 新增：`rewriteBanners` test: covers 项注入 `cdn_url`; screenshots 项注入 `cdn_url`; `effective_banner_hash` 同时填 `effective_banner_url`
- 新增：`getEffectiveBanner` test: 三级 fallback
- 新增：编辑表单 test: covers 编辑 → Footer payload 包含 covers 完整数组; 不编辑 → 仍发送完整原数组(presence 约定)
- 新增：SnapshotDiff test: covers 对象数组 diff 显示 `+added/−removed/~changed{field}`

### U3
- 新增：`ToWikiPath` test: `/galgame-tag/:id/revisions` 等 4 实体 × 3 端点全部映射正确
- 新增：RevisionList 组件 test: 列表渲染 + 懒加载单条 + 回滚权限门控

### 横切
- 新增：Footer test: `direct` 分支 PUT + reconcileAliasesLinks; PR 分支 POST; covers/screenshots 都在两条路径上正确传递
- 新增：Rewrite 水合 test: 完整水合所有字段含 covers/screenshots

---

## 10. 已确认决策记录

实施前的 5 个待定项已逐条决议,固化如下(影响下面 §3-§9 的对应小节):

1. **wiki taxonomy revision 端点采用 `/galgame/<entity>/` 新命名空间**(避免 `tag` 与 topic.tag 等概念歧义):
   - 新端点形态:`GET /galgame/{tag|official|engine|series}/:id/revisions[/:rev]`、`POST /galgame/{...}/:id/revert`。
   - **现有 taxonomy 端点**(`/tag` `/official` `/engine` `/series` 单词形态,如 `/tag/search`、`PUT /tag` 等)**继续保留**,不迁移命名空间。
   - kungal `client/path_mapper.go` **不能改 `wikiPathPrefixes` 全局映射**(会破坏所有现有端点),而是**新增 suffix-aware 规则**:`ToWikiPath` 检测网关路径是否形如 `/galgame-<entity>/:id/(revisions|revisions/:rev|revert)` 且 `<entity>` ∈ {`tag`,`official`,`engine`,`series`},命中则映射为 `/galgame/<entity>/:id/...`(保留 `/galgame/` 命名空间);否则走原 `/galgame-tag → /tag` 等映射。具体写法:在 `path_mapper.go` 加 `taxonomyRevisionSuffixes := []string{"/revisions", "/revert"}` + 在匹配 entity 前缀后检测 `:id/<suffix>` 结构;命中则 `wikiPath = "/galgame/<entity>" + rest`,否则按现规则继续。
   - 配套测试 `client/path_mapper_test.go` 必须覆盖:`/galgame-tag/5/revisions` → `/galgame/tag/5/revisions`、`/galgame-tag/5/revisions/3` → `/galgame/tag/5/revisions/3`、`/galgame-tag/5/revert` → `/galgame/tag/5/revert`、且 `/galgame-tag/search` 仍 → `/tag/search`(回归断言)、`/galgame-tag/5` 仍 → `/tag/5`(回归断言)。

2. **前端引入 vitest**:Nuxt 与 vitest 有深度集成(`@nuxt/test-utils`),开箱即用。K-PR4 一并完成基础设施:`apps/web/package.json` 加 devDep `vitest`/`@vue/test-utils`/`@nuxt/test-utils`/`happy-dom`,新增 `apps/web/vitest.config.ts`(沿 Nuxt 官方推荐配置),`package.json` 加 `test`/`test:watch` 脚本。

3. **画廊 UI v1 简单版**:网格 + 点击放大(KunModal 内 KunImage 全屏)。不做 lightbox / 键盘左右切 / 缩放手势 / 视频。

4. **派生 URL 命名采用 cover/screenshot 对象内 `cdn_url`(通用) + 顶层 `effective_banner_url`(banner 语义专用)**:
   - cover 对象 / screenshot 对象内统一注入 `cdn_url`(因为 screenshot 无"banner"语义,用通用 key 不别扭)。
   - 顶层另注入一个 `effective_banner_url`(派生自 `effective_banner_hash`),命名贴近"封面"语义,前端单纯取头图直接用。
   - `apps/web/shared/utils/getEffectiveBanner.ts` 的 fallback 链相应变为:`galgame.effective_banner_url` → `galgame.banner_image_hash`(过渡期经现有 banner 解析) → `galgame.banner`(legacy)。

5. **K-PR4(数组结构化 diff)与 K-PR2 解耦,顺序串行而非合并**:项目目前**整体处于测试阶段、未上线**,且**所有历史 PR 已被清除**——不存在"老 diff 数据被新 diff 渲染错乱"的兼容压力。K-PR2 上线后,**新产生**的 PR/revision 若含 covers/screenshots 改动,在 K-PR4 之前会暂时被 `SnapshotDiff.vue` 用旧 LCS 路径渲染为 JSON 字符串 diff(可读性差但**不会错**)。这在测试阶段可接受。K-PR4 单独排期、规模更可控。

> 历史问题:`§10 打开问题` 节已逐条决议并固化到上述各节;此节保留为决策审计记录,不再有"待定"项。

---

## 11. 一句话总结

本次升级在保持"kungal 仅做透传 + 极少响应增强、wiki 是 SoT"的分工不变前提下做三件事:

1. **U1**: 全链路把 `released` 字段换成 `release_date date? + release_date_tba bool`,与 wiki 同期发版(breaking)。
2. **U2**: 扩展 `rewriteBanners` 把 wiki 新增的 covers/screenshots/effective_banner_hash 全部解析为可用 CDN URL; 前端加图集编辑表单 + 详情页画廊 + `getEffectiveBanner` 三级 fallback util(过渡期); 顺手把 SnapshotDiff 对象数组 diff 做掉(否则 covers/screenshots 改动在 diff 区是乱码)。
3. **U3**: kungal 加 4 个 taxonomy 实体的 `revisions[/​:rev]` + `revert` 代理路由; 把现有 galgame `History.vue` 重构为通用 `GalgameRevisionList`,4 个 taxonomy 详情页各挂一份。

**显式不做**: 复刻 wiki 修订系统、字段级 RBAC、多源同步、热度算法、多搜索引擎、增量关系动作端点、撤销删除批量恢复 UI 全功能版。这些要么是 wiki 端职责、要么是 YAGNI、要么是 v1 不必要的复杂度。

**关键护栏**: 每条 wire 契约变更配 wire-shape 单测; 编辑表单的 presence 全量替换约定靠 Rewrite.vue 完整水合 + 提交前二次确认弹窗双保险。
