# 关键功能实现

## 1. 身份认证系统

### 双 Token 认证流程

```
客户端                    服务端                    Redis
  │                        │                        │
  │  POST /api/auth/login  │                        │
  │───────────────────────>│                        │
  │                        │  验证密码 (bcrypt)      │
  │                        │  生成 Access Token      │
  │                        │  生成 Refresh Token     │
  │                        │──── 存储 Refresh Token ─>│
  │  Set-Cookie: refresh   │                        │
  │<───────────────────────│                        │
  │                        │                        │
  │  请求 API (带 Cookie)   │                        │
  │───────────────────────>│                        │
  │                        │  getCookieTokenInfo()  │
  │                        │  jwt.verify()          │
  │                        │──── 检查 Token 有效性 ──>│
  │                        │<─── 返回结果 ───────────│
  │  返回数据              │                        │
  │<───────────────────────│                        │
```

- **Access Token**: 有效期 60 分钟
- **Refresh Token**: 有效期 30 天，Cookie 名 `kungalgame-moemoe-refresh-token`
- **Redis Key**: `refreshToken:{uid}`

### 客户端身份失效处理

`app/utils/responseHandler.ts` 中统一拦截 `code: 205`：
1. 重置用户 Store
2. 设置防重复跳转 Cookie（1 分钟有效）
3. 弹出错误提示
4. 跳转到 `/login`

### 路由守卫

```typescript
// app/middleware/auth.ts - 客户端路由中间件
export default defineNuxtRouteMiddleware((to, from) => {
  const { id } = usePersistUserStore()
  if (!id) {
    useMessage(10249, 'warn', 5000)
    return navigateTo('/login')
  }
})

// 页面使用:
definePageMeta({ middleware: ['auth', 'prevent'] })
```

## 2. 实时通信 (Socket.IO)

### 服务端插件

```
server/plugins/socket.io.ts
├── JWT 认证中间件（解析 Cookie Token）
├── 将 Payload 挂载到 socket 对象
└── 交由 handleSocketRequest() 处理事件

server/socket/
├── handleSocketRequest.ts  # 事件路由
├── service/                # 业务处理
└── socket.d.ts             # 类型定义
```

### 认证中间件

Socket.IO 连接时从 handshake 的 cookie 中提取 JWT Token，验证通过后将用户信息挂载到 socket 对象上，未认证连接将被拒绝。

## 3. 文件上传系统

### 双 S3 存储

项目使用两套 S3 兼容存储：

| 用途 | 环境变量前缀 | 场景 |
|------|-------------|------|
| 图床 | `KUN_VISUAL_NOVEL_IMAGE_BED_*` | 用户头像、Galgame 封面、文章图片 |
| 文件存储 | `KUN_VISUAL_NOVEL_S3_STORAGE_*` | 工具集资源文件上传 |

### 上传流程

```
客户端                        服务端                     S3
  │                            │                        │
  │  POST (FormData + file)    │                        │
  │───────────────────────────>│                        │
  │                            │ kunParseFormData()     │
  │                            │ checkBufferSize()     │
  │                            │ canUserUpload()       │
  │                            │──── uploadImageToS3 ──>│
  │                            │<─── 返回 URL ─────────│
  │  返回图片 URL              │                        │
  │<───────────────────────────│                        │
```

### 上传限制

- 每日上传图片计数限制（`daily_image_count`）
- 每日工具集上传计数限制（`daily_toolset_upload_count`）
- 文件大小检查（`checkBufferSize`）
- 每日计数通过定时任务 `reset-daily` 重置

## 4. Galgame 数据管理

### PR (Pull Request) 机制

Galgame 信息修改采用类似 Git PR 的协作模式：

1. 用户提交修改请求 (PR)
2. 系统记录修改历史 (`galgame_history`)
3. 管理员或原作者审核
4. 合并后更新 Galgame 数据
5. 贡献者列表更新 (`galgame_contributor`)

### 资源管理

每个 Galgame 可关联多个下载资源 (`galgame_resource`)，资源包含：
- 平台（Windows、macOS、Linux、Android 等）
- 语言
- 资源类型（游戏本体、补丁、语音包等）
- 下载链接和提供者信息

### 标签系统

标签通过中间表 `galgame_tag_relation` 关联，支持：
- 标签分类（content、sexual、technical）
- 剧透等级（`spoiler_level`: 0-无, 1-轻微, 2-严重）
- 标签别名（`galgame_tag_alias`）

## 5. 论坛讨论系统

### 三级内容结构

```
Topic (话题)
├── TopicReply[] (回复 - 一级)
│   └── TopicComment[] (评论 - 二级，针对回复的评论)
├── TopicLike / TopicDislike / TopicUpvote (互动)
├── TopicFavorite (收藏)
└── TopicPoll (投票)
    └── TopicPollVote[]
```

### 投票功能

话题可以附带投票（`topic_poll`），支持单选/多选，用户通过 `topic_poll_vote` 记录投票。

## 6. 积分系统 (Moemoepoint)

用户操作会影响 `moemoepoint` 积分：
- 新用户注册获得初始积分（默认 7）
- 发帖、回复、贡献资源等正向行为加分
- 被举报、违规等负向行为扣分
- 积分影响部分功能的使用权限

## 7. CDN 缓存清除

通过 Cloudflare API 实现缓存清除（`server/utils/purgeCache.ts`）：
- 内容更新时主动清除对应 URL 缓存
- 使用 `KUN_CF_CACHE_ZONE_ID` 和 `KUN_CF_CACHE_PURGE_API_TOKEN`

## 8. RSS 订阅

```
server/routes/rss/
└── 动态生成 RSS Feed
    ├── Galgame 更新订阅
    └── Topic 更新订阅
```

## 9. 主题系统

通过 `@nuxtjs/color-mode` 实现深色/浅色主题切换：

```typescript
// nuxt.config.ts
colorMode: {
  preference: 'system',        // 默认跟随系统
  fallback: 'light',           // 降级为浅色
  globalName: '__KUNGALGAME_COLOR_MODE__',
  classPrefix: 'kun-',         // CSS class 前缀
  classSuffix: '-mode',        // CSS class 后缀
  storageKey: 'kungalgame-color-mode'
}
```

用户还可通过 `usePersistSettingsStore` 自定义：
- 页面透明度
- 字体样式
- 背景图片及模糊/亮度
- 侧边栏折叠状态
- 内容限制级别 (SFW/NSFW)

## 10. 内容编辑器

### Markdown 编辑

使用 Milkdown 编辑器（基于 ProseMirror）+ CodeMirror 作为代码编辑器：
- 支持 Markdown 语法
- KaTeX 数学公式渲染
- 代码高亮
- 图片上传（直接粘贴/拖拽）

### 内容渲染

使用 remark/rehype 插件链处理 Markdown：
- `server/utils/remark/` - 自定义 remark 插件
- `server/utils/doc/` - 文档内容处理

## 11. 搜索功能

全文搜索通过 `/api/search` 端点实现，支持搜索：
- Galgame
- 话题
- 用户

## 12. 邮件验证

注册和密码重置使用邮件验证码：

```
server/utils/
├── sendVerificationCodeEmail.ts  # 发送验证码邮件
├── verifyVerificationCode.ts     # 验证验证码
└── generateRandomCode.ts         # 生成随机验证码
```

验证码通过 Redis 存储，设定有效期后自动过期。
