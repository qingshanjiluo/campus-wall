# API 端点开发模式

## 文件路由约定

API 端点位于 `server/api/` 目录下，使用 Nuxt 基于文件的路由系统。文件名中的 HTTP 方法后缀决定了请求方法：

```
server/api/galgame/index.get.ts       → GET    /api/galgame
server/api/galgame/index.post.ts      → POST   /api/galgame
server/api/galgame/[gid]/index.get.ts → GET    /api/galgame/:gid
server/api/galgame/[gid]/index.put.ts → PUT    /api/galgame/:gid
```

## 标准 API 端点结构

每个 API 端点遵循统一的处理模式：

```typescript
export default defineEventHandler(async (event) => {
  // 1. 输入验证
  const input = await kunParsePostBody(event, createGalgameSchema)
  if (typeof input === 'string') {
    return kunError(event, input)
  }

  // 2. 身份认证（如需要）
  const userInfo = await getCookieTokenInfo(event)
  if (!userInfo) {
    return kunError(event, '用户登录失效', 205)
  }

  // 3. 权限检查（如需要）
  if (userInfo.role <= 2) {
    return kunError(event, '您没有权限进行此操作')
  }

  // 4. 业务逻辑
  const result = await prisma.galgame.create({
    data: { ... }
  })

  // 5. 返回数据
  return result
})
```

## 输入验证工具

所有验证工具定义在 `server/utils/zod.ts`，返回类型为 `T | string`（成功返回数据，失败返回错误字符串）：

| 函数 | 用途 | HTTP 方法 |
|------|------|-----------|
| `kunParseGetQuery(event, schema)` | 解析查询参数 | GET |
| `kunParsePostBody(event, schema)` | 解析请求体 | POST |
| `kunParsePutBody(event, schema)` | 解析请求体 | PUT |
| `kunParseDeleteQuery(event, schema)` | 解析查询参数 | DELETE |
| `kunParseFormData(event, schema)` | 解析表单数据（含文件） | POST (multipart) |

### 使用示例

```typescript
// GET 请求 - 解析查询参数
const input = kunParseGetQuery(event, getGalgameSchema)
if (typeof input === 'string') {
  return kunError(event, input)
}
// input 此时是类型安全的

// POST 请求 - 解析请求体
const input = await kunParsePostBody(event, createGalgameSchema)
if (typeof input === 'string') {
  return kunError(event, input)
}
```

## 错误处理

### kunError 函数

```typescript
// server/utils/kunError.ts
kunError(event, message, code?, statusCode?)
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `message` | - | 错误消息（中文，面向用户） |
| `code` | `233` | 业务错误码 |
| `statusCode` | `233` | HTTP 状态码 |

### 错误码约定

| 错误码 | 含义 | 客户端处理 |
|--------|------|------------|
| `205` | 身份认证失效（Token 过期/无效） | 重置用户状态，跳转登录页 |
| `233` | 通用业务逻辑错误 | 弹出错误提示消息 |

## 身份认证

### Token 机制

项目使用双 Token 机制：
- **Access Token**: 有效期 60 分钟（目前主要通过 refresh token 验证）
- **Refresh Token**: 有效期 30 天，存储在 Cookie（`kungalgame-moemoe-refresh-token`）和 Redis 中

### 认证检查

```typescript
// server/utils/getCookieTokenInfo.ts
const userInfo = await getCookieTokenInfo(event)
// 返回 KUNGalgamePayload | null

// KUNGalgamePayload 结构:
interface KUNGalgamePayload {
  iss: string   // 签发者
  aud: string   // 受众
  uid: number   // 用户 ID
  name: string  // 用户名
  role: number  // 角色等级（整数，越大权限越高）
}
```

### 认证流程

1. 从 Cookie 获取 Refresh Token
2. 使用 JWT_SECRET 验证签名
3. 在 Redis 中检查 Token 是否有效（key: `refreshToken:{uid}`）
4. 返回 Payload 或 null

### 权限检查模式

```typescript
// 要求登录
if (!userInfo) {
  return kunError(event, '用户登录失效', 205)
}

// 要求管理员权限（role > 2）
if (userInfo.role <= 2) {
  return kunError(event, '您没有权限进行此操作')
}

// 要求是资源所有者
if (resource.user_id !== userInfo.uid) {
  return kunError(event, '您没有权限操作此资源')
}
```

## Zod 验证 Schema

验证 Schema 定义在 `app/validations/` 目录，按业务模块拆分：

```typescript
// app/validations/galgame.ts
import { z } from 'zod'

export const getGalgameSchema = z.object({
  page: z.coerce.number().min(1).max(9999999),
  limit: z.coerce.number().min(1).max(50),
  type: z.enum(['all', 'game', 'patch'])
})

export const createGalgameSchema = z.object({
  vndbId: z
    .string()
    .min(1)
    .max(10)
    .refine((val) => /^v\d+$/.test(val), {
      message: 'VNDB ID 格式不正确'
    }),
  name: z.object({
    'en-us': z.string().max(1000),
    'ja-jp': z.string().max(1000),
    'zh-cn': z.string().max(1000),
    'zh-tw': z.string().max(1000)
  })
})
```

### 查询参数预处理

由于 GET 查询参数都是字符串，使用 `z.coerce` 转换数字，使用 `z.preprocess` 处理数组：

```typescript
// 数字转换
z.coerce.number().min(1).max(50)

// 数组预处理（逗号分隔字符串 → 数组）
z.preprocess((v) => {
  if (Array.isArray(v)) return v
  if (typeof v === 'string') return v ? v.split(',') : []
  return []
}, z.array(z.string()).default([]))
```

## 数据库操作模式

### 基本查询

```typescript
const galgame = await prisma.galgame.findUnique({
  where: { id: input.gid },
  include: {
    user: {
      select: { id: true, name: true, avatar: true }
    },
    _count: {
      select: { like: true, comment: true }
    }
  }
})
```

### 事务操作

多步操作使用 Prisma 事务保证一致性：

```typescript
return prisma.$transaction(async (prisma) => {
  const galgame = await prisma.galgame.create({
    data: { ... }
  })
  await prisma.galgame_contributor.create({
    data: { galgame_id: galgame.id, user_id: userInfo.uid }
  })
  return galgame
}, { timeout: 60000 })
```

## 客户端请求模式

### 响应处理器

`app/utils/responseHandler.ts` 提供统一的错误拦截：

```typescript
// 在 useFetch 或 $fetch 中使用
const { data } = await useFetch('/api/galgame', {
  method: 'GET',
  query: pageData,
  ...kungalgameResponseHandler  // 自动处理 205/233 错误
})
```

处理逻辑：
- **code 205**: 清除用户状态，跳转到 `/login`，设置防重复跳转 Cookie
- **code 233**: 调用 `useMessage(message, 'error')` 显示错误提示
- **无响应**: 显示「网络请求失败，请稍后重试」

### Composable 中的请求

```typescript
export const useSomeFeature = () => {
  const result = await $fetch('/api/some-endpoint', {
    method: 'POST',
    body: { ... },
    ...kungalgameResponseHandler
  })
  return result
}
```
