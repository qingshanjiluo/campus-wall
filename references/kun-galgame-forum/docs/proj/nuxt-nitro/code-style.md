# 代码风格和命名规范

## 格式化配置

**Prettier** (`.prettierrc`)：
- 单引号 `'`，无分号
- 行宽 80 字符，缩进 2 空格
- 无尾逗号
- 箭头函数参数始终加括号
- 启用 Tailwind CSS 插件（自动排序 class）

**ESLint** (`eslint.config.mjs`)：
- 允许 `console`
- 允许单词组件名
- 关闭 camelCase 强制（部分场景需要 snake_case）
- 关闭未使用变量警告

## 命名规范

### 文件命名

| 类型 | 风格 | 示例 |
|------|------|------|
| API 端点 | `kebab-case` + HTTP 方法后缀 | `index.get.ts`, `index.post.ts` |
| 路由参数 | 方括号 | `[gid]/resource/index.put.ts` |
| Vue 组件 | `PascalCase` | `TopicHeader.vue`, `GalgameResource.vue` |
| Composable | `use` 前缀 | `useTopic.ts`, `useGalgameResource.ts` |
| Store (持久化) | `usePersist*` 前缀 | `usePersistEditTopicStore` |
| Store (临时) | `useTemp*` 前缀 | `useTempGalgamePRStore` |
| 工具函数 | `camelCase` | `parseFileName.ts`, `canUserUpload.ts` |
| 常量 | `UPPER_SNAKE_CASE` | `KUN_RESOURCE_TYPE_CONST` |
| 常量 Map | `UPPER_SNAKE_CASE` + `_MAP` | `KUN_GALGAME_RESOURCE_TYPE_MAP` |
| Prisma Schema | `snake_case` | `galgame_resource`, `topic_reply` |
| 验证 Schema | `camelCase` + `Schema` 后缀 | `getGalgameSchema`, `createTopicSchema` |

### 变量命名

| 类型 | 风格 | 示例 |
|------|------|------|
| 普通变量/函数 | `camelCase` | `pageData`, `getTopics` |
| 布尔值 | `is/has/show` 前缀 | `isActive`, `hasPermission`, `showAlert` |
| 多语言键 | 固定格式 | `'en-us'`, `'ja-jp'`, `'zh-cn'`, `'zh-tw'` |
| 数据库字段 | `snake_case` | `user_id`, `vndb_id`, `name_en_us` |

### 多语言类型

所有需要国际化的字段遵循统一的 locale map 结构：

```typescript
type KunLanguage = {
  'en-us': string
  'ja-jp': string
  'zh-cn': string
  'zh-tw': string
}

// 创建空 locale map
const createEmptyLocaleMap = () => ({
  'en-us': '',
  'ja-jp': '',
  'zh-cn': '',
  'zh-tw': ''
})
```

## Vue 组件风格

### 基本结构

```vue
<script setup lang="ts">
// 1. 导入（类型、composable、组件）
import type { SomeType } from '~/shared/types/...'

// 2. 页面元数据（如需要）
definePageMeta({ middleware: ['auth', 'prevent'] })

// 3. SEO 设置
useKunSeoMeta({ title: '...', description: '...' })

// 4. Props 和 Emits
const props = withDefaults(defineProps<Props>(), {
  isActive: false
})
const emit = defineEmits<{
  update: [value: string]
}>()

// 5. 响应式状态
const pageData = reactive({
  page: 1,
  limit: 50
})

// 6. 数据获取
const { data, status } = await useFetch('/api/...', {
  method: 'GET',
  query: pageData,
  ...kungalgameResponseHandler
})

// 7. 计算属性和方法
const computedValue = computed(() => ...)

const handleClick = () => { ... }
</script>

<template>
  <div class="space-y-3">
    <!-- 使用 kebab-case 引用组件 -->
    <KunCard :is-transparent="false">
      <KunHeader name="标题" description="描述" />
    </KunCard>
  </div>
</template>
```

### 样式特点

- 全局使用 Tailwind CSS 4 实用类
- 组件级样式使用 `<style scoped>` 或内联 Tailwind class
- 深色主题通过 `@nuxtjs/color-mode` 管理，使用 `kun-dark-mode` / `kun-light-mode` class 前缀
- 页面过渡动画名为 `kun-page`，模式为 `out-in`

## TypeScript 风格

- 类型定义集中在 `shared/types/` 目录
- 使用 `interface` 定义对象结构
- Store 类型定义在 `app/store/types/`
- 使用 Zod 进行运行时类型验证
- 泛型广泛用于 Zod 解析函数

## 错误消息

- 面向用户的错误消息使用**中文**
- 业务逻辑错误码统一为 `233`
- 认证失效错误码统一为 `205`

## 常量定义模式

每个业务模块的常量文件通常包含以下结构：

```typescript
// 1. TypeScript 联合类型
export type KunGalgameResourceTypeOptions =
  | 'all' | 'game' | 'patch' | 'collection' | ...

// 2. 映射表（用于 UI 展示）
export const KUN_GALGAME_RESOURCE_TYPE_MAP: Record<string, string> = {
  all: '全部类型',
  game: '游戏本体',
  ...
}

// 3. 值数组（用于 Zod 验证）
export const KUN_RESOURCE_TYPE_CONST = ['game', 'patch', 'collection', ...]

// 4. Select 选项（用于下拉组件）
export const kunGalgameResourceTypeOptions: KunSelectOption[] = [
  { value: 'all', label: '全部类型' },
  ...
]
```
