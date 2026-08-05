# 开发指南

## 环境准备

### 必需依赖

- Node.js (推荐 LTS)
- pnpm 10.17.1+
- PostgreSQL
- Redis

### 安装与启动

```bash
# 安装依赖
pnpm install

# 复制环境变量模板并配置
cp .env.example .env

# 生成 Prisma Client
pnpm prisma:generate

# 推送 Schema 到数据库
pnpm prisma:push

# 启动开发服务器 (http://127.0.0.1:1007)
pnpm dev
```

### 常用命令

| 命令 | 用途 |
|------|------|
| `pnpm dev` | 启动开发服务器 |
| `pnpm build` | 构建生产版本 |
| `pnpm build:limit` | 构建生产版本（8GB 内存限制） |
| `pnpm preview` | 预览构建结果 |
| `pnpm lint` | ESLint 检查 |
| `pnpm format` | Prettier 格式化 |
| `pnpm typecheck` | TypeScript 类型检查 |
| `pnpm prisma:generate` | 生成 Prisma Client |
| `pnpm prisma:push` | 推送 Schema 变更 |
| `pnpm prisma:pull` | 从数据库拉取 Schema |
| `pnpm prisma:studio` | 打开 Prisma Studio |
| `pnpm start` | PM2 启动生产服务 |
| `pnpm stop` | PM2 停止生产服务 |
| ~~`pnpm build:sitemap`~~ | 已移除：sitemap 由 `@nuxtjs/sitemap` 在运行时动态生成（`/sitemap.xml`，SFW 源端点 `server/api/__sitemap__/urls.ts`） |
| `pnpm build:friend` | 生成友链截图 |

## 新增 API 端点

### 步骤

1. **定义 Zod 验证 Schema** (`app/validations/`)

```typescript
// app/validations/example.ts
import { z } from 'zod'

export const getExampleSchema = z.object({
  page: z.coerce.number().min(1).max(9999999),
  limit: z.coerce.number().min(1).max(50)
})

export const createExampleSchema = z.object({
  title: z.string().min(1).max(200),
  content: z.string().min(1).max(10000)
})
```

2. **创建 API 端点** (`server/api/`)

```typescript
// server/api/example/index.get.ts
export default defineEventHandler(async (event) => {
  const input = kunParseGetQuery(event, getExampleSchema)
  if (typeof input === 'string') {
    return kunError(event, input)
  }

  const result = await prisma.example.findMany({
    take: input.limit,
    skip: (input.page - 1) * input.limit,
    orderBy: { created: 'desc' }
  })

  return result
})
```

```typescript
// server/api/example/index.post.ts
export default defineEventHandler(async (event) => {
  const input = await kunParsePostBody(event, createExampleSchema)
  if (typeof input === 'string') {
    return kunError(event, input)
  }

  const userInfo = await getCookieTokenInfo(event)
  if (!userInfo) {
    return kunError(event, '用户登录失效', 205)
  }

  return prisma.example.create({
    data: {
      title: input.title,
      content: input.content,
      user_id: userInfo.uid
    }
  })
})
```

3. **定义共享类型** (`shared/types/`)

```typescript
// shared/types/example.ts
export interface Example {
  id: number
  title: string
  content: string
  user: { id: number; name: string; avatar: string }
  created: string
}
```

4. **在前端调用**

```typescript
// 在页面或 composable 中
const { data } = await useFetch('/api/example', {
  method: 'GET',
  query: { page: 1, limit: 10 },
  ...kungalgameResponseHandler
})
```

## 新增页面

1. 在 `app/pages/` 下创建 Vue 文件（文件名即路由路径）
2. 设置页面元数据和 SEO
3. 按需添加路由中间件

```vue
<!-- app/pages/example/index.vue -->
<script setup lang="ts">
definePageMeta({
  middleware: ['auth']  // 如需登录
})

useKunSeoMeta({
  title: '示例页面',
  description: '这是示例页面'
})

const { data } = await useFetch('/api/example', {
  method: 'GET',
  query: { page: 1, limit: 10 },
  ...kungalgameResponseHandler
})
</script>

<template>
  <div class="container mx-auto space-y-4">
    <KunHeader name="示例" description="示例页面" />
    <!-- 页面内容 -->
  </div>
</template>
```

## 新增组件

组件放置在 `app/components/` 对应的业务目录下，Nuxt 自动导入，无需手动 import：

```
app/components/example/
├── ExampleCard.vue
├── ExampleList.vue
└── ExampleDetail.vue
```

## 新增 Composable

```typescript
// app/composables/example/useExample.ts
export const useExample = () => {
  const examples = useState<Example[]>('examples', () => [])
  const isLoading = ref(false)

  const fetchExamples = async (page: number) => {
    isLoading.value = true
    const result = await $fetch('/api/example', {
      method: 'GET',
      query: { page, limit: 10 },
      ...kungalgameResponseHandler
    })
    if (result) {
      examples.value.push(...result)
    }
    isLoading.value = false
  }

  return { examples, isLoading, fetchExamples }
}
```

## 新增数据库模型

1. 在 `prisma/schema/` 下创建 `.prisma` 文件
2. 运行 `pnpm prisma:generate` 生成 Client
3. 运行 `pnpm prisma:push` 推送到数据库

```prisma
// prisma/schema/example.prisma
model example {
  id      Int    @id @default(autoincrement())
  title   String @db.VarChar(200)
  content String @db.Text

  user_id Int
  user    user @relation(fields: [user_id], references: [id], onDelete: Cascade)

  created DateTime @default(now())
  updated DateTime @updatedAt
}
```

> 注意：新模型需要在 `user.prisma` 中添加反向关联字段。

## 新增 Store

```typescript
// app/store/modules/example.ts
export const usePersistExampleStore = defineStore(
  'KUNGalgameExample',
  () => {
    const draft = ref('')

    const resetDraft = () => {
      draft.value = ''
    }

    return { draft, resetDraft }
  },
  {
    persist: {
      storage: piniaPluginPersistedstate.localStorage()
    }
  }
)
```

记得在 `app/store/index.ts` 的 `kungalgameStoreReset()` 中添加重置逻辑。

## 生产部署

```bash
# 安装依赖
pnpm deploy:install

# 构建
pnpm deploy:build

# PM2 启动
pnpm start

# PM2 停止
pnpm stop
```

部署配置在 `ecosystem.config.cjs` 中定义。
