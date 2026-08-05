# 待实现功能清单

> 最后更新: 2026-04-12
> 本文档列出所有尚未在 Go Fiber 后端实现的功能，供后续 Claude Code 会话参考。

---

## 1. Toolset 工具集模块 (17 端点)

**优先级: 高** | **复杂度: 5/5** | **依赖: S3 分片上传**

模型已定义: `internal/toolset/model/toolset.go` (143 行, 7 模型)

### 1.1 基础 CRUD (5 端点)

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /api/toolset | 公开 | 列表 (分页+多条件筛选: type/language/platform/version) |
| POST | /api/toolset | 需要 | 创建 (+3 萌萌点, 可含别名) |
| GET | /api/toolset/:id | 公开 | 详情 (含 Markdown→HTML, 评分分布, 评论预览) |
| PUT | /api/toolset/:id | 需要 | 编辑 (创建者 or role≥2) |
| DELETE | /api/toolset/:id | 需要 | 删除 (S3 资源清理, 萌萌点扣除 3+(资源数×3)) |

**列表特殊逻辑:**
- 按 type/language/platform/version 过滤，`'all'` 值跳过对应过滤
- `sortField`: `resource_update_time`, `created`, `view`
- 计算: 下载量总和 (`SUM(resource.download)`), 实用性平均分
- 排除 `status=1` (已删除)

**详情特殊逻辑:**
- Markdown → HTML 转换 (内容字段)
- 评分分布: `{ 1: 数量, 2: 数量, 3: 数量, 4: 数量, 5: 数量 }`
- 返回最新 5 条评论预览 (含嵌套回复)
- 浏览量 +1

**删除特殊逻辑:**
- 遍历所有 S3 类型资源，逐个调 `DeleteObjectCommand` 删除 S3 对象
- 扣除萌萌点: `3 + (资源数 × 3)`，余额不足不允许删除
- 事务超时 60 秒 (S3 操作耗时)

### 1.2 实用性评分 (2 端点)

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /api/toolset/:id/practicality | 可选 | 获取评分分布 + 当前用户评分 |
| PUT | /api/toolset/:id/practicality | 需要 | 提交/更新评分 (1-5, upsert) |

### 1.3 评论 (4 端点)

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /api/toolset/:id/comment | 公开 | 评论列表 (分页, 含嵌套回复) |
| POST | /api/toolset/:id/comment | 需要 | 发表评论 (支持 parent_id 回复, 消息通知) |
| PUT | /api/toolset/:id/comment | 需要 | 编辑评论 (作者 only) |
| DELETE | /api/toolset/:id/comment | 需要 | 删除评论 (作者 or 工具创建者 or role≥2) |

**评论是自引用树** — `parent_id` 指向父评论。回复时给父评论作者 +1 萌萌点 + 消息通知。

### 1.4 资源管理 (4 端点)

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | /api/toolset/:id/resource | 需要 | 创建资源 (支持 S3 和用户提供链接两种类型) |
| PUT | /api/toolset/:id/resource | 需要 | 编辑资源 (S3 类型只能改 password/note) |
| DELETE | /api/toolset/:id/resource | 需要 | 删除资源 (S3 类型清理对象, -3 萌萌点) |
| GET | /api/toolset/:id/resource/detail | 公开 | 资源详情 (下载量 +1) |

**创建资源逻辑:**
- 如果有 `salt`: 从 Redis 取上传缓存 → S3 类型资源, content=S3 key
- 无 `salt`: 用户提供链接类型资源
- 更新 `resource_update_time`
- 萌萌点 +3
- 添加创建者为贡献者 (去重)

### 1.5 文件上传 (4 端点)

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | /api/toolset/:id/upload/small | 需要 | 小文件上传初始化 (≤10MB, 返回预签名 URL) |
| POST | /api/toolset/:id/upload/large | 需要 | 大文件分片上传初始化 (>10MB, 返回 UploadId + 各分片预签名 URL) |
| POST | /api/toolset/:id/upload/complete | 需要 | 完成上传 (合并分片, 校验大小, 更新配额) |
| POST | /api/toolset/:id/upload/abort | 需要 | 中止上传 (AbortMultipartUpload, 清理缓存) |

**小文件流程:**
1. 校验文件名 (必须 `.7z/.zip/.rar`)
2. 校验大小 ≤ 10MB
3. 校验用户上传配额 (`canUserUpload`)
4. 生成 S3 key: `toolset/{toolsetId}/{uid}_{base}_{salt}.{ext}`
5. 生成 PutObject 预签名 URL
6. 存储上传元数据到 Redis (`saveUploadSalt`)
7. 返回 `{ key, salt, url }`

**大文件流程:**
1. 校验大小 >10MB 且 ≤2GB
2. 校验配额
3. CreateMultipartUpload → 获取 UploadId
4. 计算分片数 = `ceil(filesize / CHUNK_SIZE)`
5. 为每个分片生成 UploadPart 预签名 URL
6. 存储元数据到 Redis
7. 返回 `{ key, salt, uploadId, partSize, urls: [{partNumber, url}] }`

**完成上传流程:**
1. 如果有 uploadId → CompleteMultipartUpload (合并分片)
2. HeadObject 校验实际文件大小 = 缓存记录的大小
3. 大小不匹配 → DeleteObject + 清理缓存 + 报错
4. 最终校验配额
5. 更新用户 `daily_toolset_upload_count`

**S3 相关常量:**
```
MAX_SMALL_FILE_SIZE = 10 * 1024 * 1024  (10MB)
MAX_LARGE_FILE_SIZE = 2 * 1024 * 1024 * 1024  (2GB)
LARGE_FILE_CHUNK_SIZE = 10 * 1024 * 1024  (10MB per part)
UPLOAD_TIMEOUT = 3600  (预签名 URL 1 小时过期)
```

**Redis 缓存结构 (upload salt):**
```
Key: toolset:upload:{salt}
Value: { key, type, salt, filesize, base, ext }
TTL: 与预签名 URL 同步
```

### 前端文件

需要迁移的前端文件 (~15 个):
```
pages/toolset/[id]/index.vue
components/toolset/comment/Comment.vue
components/toolset/comment/Panel.vue  (if exists)
components/toolset/resource/Upload.vue
components/toolset/resource/Item.vue
components/edit/toolset/ (创建/编辑页)
```

---

## 2. Image Upload 图片上传 (1 端点)

**优先级: 高** | **复杂度: 3/5** | **依赖: S3, 图片压缩库**

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | /api/image/topic | 需要 | 话题图片上传 |

**处理流程:**
1. 接收 multipart form data (图片文件)
2. 校验文件大小 ≤ 10MB
3. 图片处理:
   - 缩放: 最大 1920×1080, `fit: inside`, 不放大
   - 格式: 转为 WebP
   - 质量: 由 `KUN_VISUAL_NOVEL_IMAGE_COMPRESS_QUALITY` 控制
   - 压缩后大小校验: ≤ `KUN_VISUAL_NOVEL_IMAGE_COMPRESS_LIMIT`
4. 校验每日上传限制 (`daily_image_count < KUN_VISUAL_NOVEL_USER_DAILY_UPLOAD_IMAGE_LIMIT`)
5. 上传到 S3: `topic/user_{uid}/{filename}.webp`
6. 用户 `daily_image_count` +1
7. 返回图片 URL

**Go 图片压缩方案:**
- 使用 `github.com/disintegration/imaging` 缩放
- 使用 `golang.org/x/image/webp` 编码 (或 `github.com/chai2010/webp`)
- 或使用 cgo 绑定的 `libwebp`

**前端:** `components/kun/milkdown/plugins/upload/uploader.ts` — 已经调用 `/api/image/topic`

---

## 3. Activity 活动流 (2 端点)

**优先级: 中** | **复杂度: 3/5** | **依赖: 多表联合查询**

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /api/activity | 公开 | 按类型获取活动列表 (分页) |
| GET | /api/activity/timeline | 公开 | 时间线 (所有类型混合) |

**18 种活动类型:**
```
GALGAME_CREATION, GALGAME_RATING_CREATION, GALGAME_COMMENT_CREATION,
GALGAME_PR_CREATION, GALGAME_WEBSITE_CREATION,
GALGAME_RATING_COMMENT_CREATION, GALGAME_WEBSITE_COMMENT_CREATION,
GALGAME_RESOURCE_CREATION, TOOLSET_CREATION, TOOLSET_RESOURCE_CREATION,
TOOLSET_COMMENT_CREATION, TOPIC_CREATION, TOPIC_REPLY_CREATION,
TOPIC_COMMENT_CREATION, TODO_CREATION, UPDATE_LOG_CREATION,
MESSAGE_SOLUTION, MESSAGE_UPVOTE
```

**每种类型查询不同的表**, 统一返回:
```go
type ActivityItem struct {
    UniqueID  string    `json:"uniqueId"`
    Type      string    `json:"type"`
    Timestamp time.Time `json:"timestamp"`
    Actor     KunUser   `json:"actor"`
    Link      string    `json:"link"`
    Content   string    `json:"content"` // 前 100 字预览
}
```

**注意:**
- 部分类型 (galgame 相关) 需要查 wiki service
- SFW 过滤: galgame 按 `content_limit`, topic 按 `is_nsfw`
- Timeline: 混合所有类型按时间排序

**前端文件:**
```
pages/activity/index.vue
components/activity/Timeline.vue
```

---

## 4. Search 搜索 (1 端点)

**优先级: 中** | **复杂度: 3/5** | **依赖: Meilisearch (推荐) 或 PostgreSQL ILIKE**

| HTTP | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /api/search | 公开 | 统一搜索 |

**查询参数:**
- `keywords`: 搜索词 (空格分隔, 最长 107 字符)
- `type`: `topic` / `galgame` / `user` / `reply` / `comment`
- `page`, `limit` (最大 12)

**按类型搜索不同的表:**

| type | 搜索字段 | 返回 |
|------|---------|------|
| topic | title, content, category, tag | TopicCard |
| galgame | name_*, vndb_id, alias, tag, intro_* | GalgameCard (需调 wiki) |
| user | name | UserCard |
| reply | content | ReplyCard (含 topic 标题) |
| comment | content | CommentCard (含 topic 标题) |

**方案选择:**
- **方案 A (推荐):** Meilisearch — 独立搜索服务, 需要维护索引同步
- **方案 B (MVP):** PostgreSQL `ILIKE '%keyword%'` — 简单但性能差
- 当前 Nitro 代码用方案 B, 计划迁移到方案 A

**前端文件:**
```
components/search/Container.vue
```

---

## 5. 定时任务 (2 个任务)

**优先级: 中** | **复杂度: 2/5** | **依赖: `robfig/cron`**

### 5.1 每日重置 (0 0 * * *)

```go
// 每天午夜重置所有用户的每日计数器
UPDATE "user" SET
    daily_check_in = 0,
    daily_image_count = 0,
    daily_toolset_upload_count = 0
WHERE daily_check_in != 0
   OR daily_image_count != 0
   OR daily_toolset_upload_count != 0;
```

### 5.2 清理过期上传 (0 * * * *)

```go
// 每小时清理 Redis 中过期的 toolset 上传缓存
// 1. 扫描 Redis keys 匹配 "toolset:upload:*"
// 2. 对每个 key: 解析 UploadSaltCache, 提取 S3 key
// 3. DeleteObject 从 S3 删除孤立对象
// 4. 删除 Redis 缓存
```

**实现位置:** `internal/infrastructure/cron/cron.go`

**在 `app.go` 中启动:**
```go
import "github.com/robfig/cron/v3"

c := cron.New()
c.AddFunc("0 0 * * *", resetDailyTask(db))
c.AddFunc("0 * * * *", cleanupToolsetTask(db, s3, rdb))
c.Start()
// graceful shutdown: c.Stop()
```

---

## 6. WebSocket 实时消息 (4 事件)

**优先级: 低** | **复杂度: 4/5** | **依赖: `go-socket.io` 或 Fiber WebSocket**

### 事件

| 事件 | 方向 | 说明 |
|------|------|------|
| `private:join` | C→S | 加入私聊房间 (receiverUid) |
| `message:sending` | C→S | 发送消息 (receiverUid + content) |
| `message:recall` | C→S | 撤回消息 (messageId) |
| `private:leave` | C→S | 离开房间 |

### 认证

从 `kun_session` cookie 验证 Redis session, 提取 UserInfo。

### message:sending 逻辑

```
1. 校验: uid 存在, uid != receiverUid, content 长度 1-1007
2. 创建 chat_message 记录 (sender_id, receiver_id, content, chatroom_name)
3. 创建 chat_message_read_by 记录 (发送者自动已读)
4. 更新 chat_room.last_message_* 字段
5. emit 到发送者 + 房间
```

### message:recall 逻辑

```
1. 校验: 消息存在, sender_id == uid, 未被撤回
2. 更新: is_recall=true, recall_time=now
3. 如果是最新消息: 更新 chat_room.last_message_content = "{name}撤回了一条消息"
4. emit 到发送者 + 房间
```

### 推荐方案

**`github.com/googollee/go-socket.io`** — 前端 Socket.IO 客户端无需改动。

---

## 7. Markdown 渲染 (goldmark)

**优先级: 低** | **复杂度: 4/5** | **依赖: goldmark + 自定义插件**

### 当前 Nitro 管线

```
remarkParse → remarkGfm → remarkFrontmatter → remarkMath
→ remarkRehype
→ rehypeSanitize → rehypeSlug → rehypePrism → rehypeKatex
→ rehypeKunH1ToH2 → rehypeKunLazyImage → rehypeKunCodeBlockWrapper
→ rehypeKunTableWrapper
→ rehypeStringify
→ 后处理: spoiler (||text||), video (kv:<url>)
```

### Go 目标管线

```go
import (
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"       // GFM
    mathjax "github.com/nicholasgasior/goldmark-mathjax"
    highlighting "github.com/yuin/goldmark-highlighting/v2"
)

md := goldmark.New(
    goldmark.WithExtensions(
        extension.GFM,
        mathjax.MathJax,
        highlighting.NewHighlighting(
            highlighting.WithStyle("monokai"),
        ),
    ),
)
```

### 需要实现的自定义 AST Walker

| 插件 | 功能 | 对应 Nitro |
|------|------|-----------|
| LazyImage | `<img>` 添加 `loading="lazy"` + `decoding="async"` | rehypeKunLazyImage |
| CodeBlockWrapper | `<pre>` 外包 `<div class="kun-code-container">` + 语言标签 + 复制按钮 | rehypeKunCodeBlockWrapper |
| H1ToH2 | `<h1>` → `<h2>` | rehypeKunH1ToH2 |
| TableWrapper | `<table>` 外包 `<div class="kun-table-container">` | rehypeKunTableWrapper |

### 后处理 (regex)

```go
// Spoiler: ||text|| → <span class="kun-spoiler ...">text</span>
re := regexp.MustCompile(`\|\|(.+?)\|\|`)
html = re.ReplaceAllString(html, `<span class="kun-spoiler">$1</span>`)

// Video: kv:<a href="url.mp4">...</a> → <video controls ...>
reVideo := regexp.MustCompile(`kv:<a href="([^"]+)"[^>]*>[^<]*</a>`)
html = reVideo.ReplaceAllString(html, `<video controls preload="metadata" src="$1"></video>`)
```

### 使用位置

- Topic 创建/编辑 → content → HTML
- Reply 创建/编辑 → content → HTML
- Toolset 详情 → description → HTML
- Activity 预览 → 纯文本截断 (去 HTML)

### 风险

渲染结果可能与 remark/rehype 有差异。建议对 100+ 篇真实文章做对比测试。

**实现位置:** `internal/infrastructure/markdown/markdown.go`

---

## 实施顺序建议

```
1. Toolset (17 端点)          ← 最后一个完整业务模块
   └── 含 S3 分片上传
2. Image Upload (1 端点)      ← 共享 S3 逻辑
3. 定时任务 (2 个)             ← 简单, robfig/cron
4. Activity (2 端点)          ← 多表查询
5. Search (1 端点)            ← Meilisearch 或 ILIKE
6. Markdown 渲染              ← goldmark + 自定义插件
7. WebSocket (4 事件)         ← go-socket.io
```

---

## 已完成模块参考

| 模块 | 端点数 | 位置 |
|------|--------|------|
| User + Auth | 20 | `internal/user/` |
| Topic (CRUD + Reply + Comment + Poll) | 28 | `internal/topic/` |
| Message | 6 | `internal/message/` |
| Admin | 6 | `internal/admin/` |
| Ranking + Section + Category | 5 | `internal/common/` |
| Doc | 12 | `internal/doc/` |
| Website | 14 | `internal/website/` |
| Update + Report + RSS | 8 | `internal/common/` |
| Galgame (代理 + 交互) | 47 | `internal/galgame/` |
| **总计** | **~146** | |

前端迁移: ~104 文件已从 `kungalgameResponseHandler` 迁移到 `kunFetch/useKunFetch`。
Toolset 前端 (~15 文件) 待迁移。
