# 论坛消费 galgame-wiki OpenAPI 契约（2026-07-01）

> 本仓自有工程笔记（**非** infra 镜像）。对应 infra `todos.md` 第 2 项
> 「OpenAPI → 类型化客户端」的论坛落地部分（该项曾标注「forum 仍未消费(其团队)」）。
>
> **🏁 全管线已随上游退役(2026-07-23,开放 API Phase 2 路线 B W5)**:galgame-wiki
> read/calendar spec 及其门户发布随桥面读退役删除,论坛的 Phase A 管线
> (gen 脚本 / 生成物 / `openapi-types.yml` 防漂移闸 / lint 排除)同波拆除——
> 生成物本就零 importer(Phase B 从未落地),纯管线移除零运行时影响。
> galgame 数据现经论坛 BFF 消费 infra 的 `/v1` 公开契约(真值=infra
> `docs/galgame_wiki/public-openapi.yaml`)。以下为历史记录。
>
> ~~当前状态:Phase A(生成 + 防漂移闸)已完成并提交;Phase B(替换手写类型)暂缓——
> 见下方「关键结论」:论坛架构与 moyu 不同,整体 Phase B 不适用,但排查中发现一个真 bug。~~

## 背景

infra 把 galgame-wiki 的**读端点**用 Huma 从 Go 代码导出为 OpenAPI 3.1 契约,发布到门户:

- `https://docs-kungal.nextmoe.dev/specs/galgame-wiki.openapi.yaml`（21 端点）
- `https://docs-kungal.nextmoe.dev/specs/galgame-wiki-calendar.openapi.yaml`（月历）

样板是 moyu（patch）:`openapi-typescript` 从发布的 spec 生成类型 + CI 防漂移闸,
再把手写 `shared/types` 改为生成式别名。moyu 落地即抓到一个真 bug（PR 线缆是
`title`+`message` 不是 `note`）。

## Phase A（已完成,提交 `dff3ddf1`）

纯管线,零消费方改动,不会影响运行时:

- `apps/web/package.json` 加两条脚本 `gen:types:galgame-{read,calendar}`
  （钉版 `openapi-typescript@7.13.0`,`pnpm dlx`,不入 lockfile）。
- 生成物提交进 `apps/web/shared/types/generated/galgame-{read,calendar}-api.ts`。
- eslint + prettier 排除该目录。
- `.github/workflows/openapi-types.yml`:重新生成 + `git diff --exit-code`。
  上游改字段 / 本地漏重生成 → CI 红,而非运行时炸。

本地已核实生成确定性(重跑 → diff 干净)、eslint 忽略生效、生成物零 importer。

## 关键结论:整体 Phase B 不适用于论坛

moyu **直接消费 wiki 的原始响应**(snake_case),所以它的手写类型能一一别名到 spec。
**论坛不同**:论坛后端在 `internal/galgame/` 把 wiki 响应**重映射为自有 camelCase DTO**,
并**把 `user_id` 在服务端解析成完整 `user` 对象**(`pkg/userclient` 批量水合)。

- 证据:`apps/api/internal/galgame/dto/wiki_proxy_dto.go`——`GalgameLink` / `GalgameRevision`
  / `GalgamePR` 都是 camelCase(`galgameId` / `baseRevision` / `completedTime`)且 `User UserBrief`。
- 服务端重映射:`apps/api/internal/galgame/service/wiki_service.go`（`wikiPRRow` →
  `dto.GalgamePR`,`userClient.Hydrate` 解析用户）。

因此前端 `apps/web/shared/types/galgame-{link,history,pr}.ts` 是**论坛自有的 camelCase 形状**,
**故意**不同于 snake_case 的 wiki spec。把它们别名到生成类型是**错的**。论坛这些手写类型**基本正确**,
不是漂移隐患,没有可机械别名的对象。

**唯一可别名的是一个「逐字透传」类型**(下节)。

## 排查中发现的真 bug:PR 读的是已废弃的 `note`

wiki 的 PR 人读描述字段是 **`title` + `message`**;`note` 是**修订(revision)**的字段,
PR 的单 `note` 列已废弃。权威出处 infra `apps/api/internal/platform/galgame/model/pr.go`:

> "Title + Message are the PR's human description … The retired single `note` column is
> left in the DB for historical rows; new PRs write title/message."

wiki 的 `PRResponse`(spec 与 infra HEAD)只序列化 `title`/`message`,**不含 `note`**。
但论坛两处都读 `note`:

| 层 | 位置 | 读取 | wiki 实际发送 | 效果 |
|----|------|------|---------------|------|
| Go PR 列表 | `wiki_service.go` `wikiPRRow.Note json:"note"` | `note` | `title`/`message`(无 `note`) | `GalgamePR.Note` 恒为 `""` |
| 前端 PR 详情 | `apps/web/app/components/galgame/pr/Info.vue:52` `detail.pr.note` | `note` | 逐字透传的 wiki 响应无 `note` | `undefined` |

结果:PR 的「说明」(`pr/Info.vue:118` 的 `v-if="pr.note"`)在**列表与详情 diff 视图都不渲染**——
每个 PR 的描述都是空白;`title` 论坛前端根本没取。这是 moyu 已修、论坛未修的同一处漂移。

**注意:未在部署环境实测确认**,但证据链(infra 模型注释 + moyu 已按 `message` 修好并上线 +
论坛读废弃字段)指向这是**线上真实 bug**。修前应对部署的 wiki 抽验一次。

## 建议的收口(自成一个小任务,非本次)

论坛自持前后端,改动内聚,**无需 infra 改动、无需数据库迁移**:

1. **Go 侧**:`wikiPRRow` 与 `dto.GalgamePR` 读 / 暴露 `title`+`message`(替换 `note`);
   `wiki_service.go` PR 列表映射同改。
2. **前端**:`WikiPRDetailResponse.pr` 与 `GalgamePR` 用 `title`/`message`;
   `pr/Info.vue` 改读 `detail.pr.message`(并可展示 `title`)。
3. **顺带做「唯一可别名」**:把逐字透传的 `WikiPRDetailResponse` 别名到生成的
   `PRDetailData` / `PRResponse`。**这样此类漂移会在编译期报错**(spec 无 `pr.note`,
   `detail.pr.note` 直接不过类型)——正是论坛消费 spec 的真正价值所在。
4. 验证:`go build ./...` + `nuxt typecheck`;对部署 wiki 抽验 PR 详情含 `title`/`message`。

## 附:发布 spec 相对 infra HEAD 可能偏旧

infra HEAD 的 `PRResponse` 含 `user_id` / `title` / `updated`;抽查发布 spec 时其
`PRResponse` 字段不完全一致(疑似 infra 改动后未重新发布)。若论坛要依赖生成类型的 PR 形状,
应先请 infra 重新发布 spec(其 `docs:sync` / spec 构建)。这属 infra 侧,非论坛。
